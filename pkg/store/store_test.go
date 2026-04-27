package store

import (
	"path/filepath"
	"testing"
)

func tmpLog(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "ops-log.db")
}

// --- Append + ReadAll ---

func TestAppend_CreatesFile(t *testing.T) {
	path := tmpLog(t)
	if err := Append(path, "db-health", "ok", "WAL 12MB", "fusca"); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	records, err := ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Probe != "db-health" || records[0].Status != "ok" {
		t.Errorf("unexpected record: %+v", records[0])
	}
}

func TestAppend_MultipleRecords(t *testing.T) {
	path := tmpLog(t)
	probes := []struct{ probe, status string }{
		{"db-health", "ok"},
		{"airbyte", "warn"},
		{"db-health", "warn"},
	}
	for _, p := range probes {
		if err := Append(path, p.probe, p.status, "signal", "repo"); err != nil {
			t.Fatal(err)
		}
	}
	records, _ := ReadAll(path)
	if len(records) != 3 {
		t.Fatalf("expected 3, got %d", len(records))
	}
}

func TestReadAll_MissingFile(t *testing.T) {
	records, err := ReadAll("/nonexistent/path/ops-log.db")
	if err != nil {
		t.Errorf("expected nil error for missing file, got %v", err)
	}
	if records != nil {
		t.Errorf("expected nil records for missing file")
	}
}

// --- QueryTrend ---

func TestQueryTrend_FiltersByProbe(t *testing.T) {
	path := tmpLog(t)
	Append(path, "db-health", "ok", "s1", "r")
	Append(path, "airbyte", "warn", "s2", "r")
	Append(path, "db-health", "warn", "s3", "r")

	results := QueryTrend(path, "db-health", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 db-health records, got %d", len(results))
	}
	// newest first
	if results[0].Status != "warn" {
		t.Errorf("expected newest first (warn), got %s", results[0].Status)
	}
}

func TestQueryTrend_RespectsLimit(t *testing.T) {
	path := tmpLog(t)
	for i := 0; i < 10; i++ {
		Append(path, "db-health", "ok", "signal", "repo")
	}
	results := QueryTrend(path, "db-health", 3)
	if len(results) != 3 {
		t.Fatalf("expected 3, got %d", len(results))
	}
}

func TestQueryTrend_EmptyProbeReturnsAll(t *testing.T) {
	path := tmpLog(t)
	Append(path, "db-health", "ok", "s", "r")
	Append(path, "airbyte", "warn", "s", "r")
	results := QueryTrend(path, "", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
}

func TestQueryTrend_MissingFile(t *testing.T) {
	results := QueryTrend("/nonexistent/path/ops-log.db", "db-health", 10)
	if results != nil {
		t.Errorf("expected nil for missing file, got %v", results)
	}
}

// --- evalRules (second-order heuristics) ---

func TestEvalRules_ConsecutiveWarnFires(t *testing.T) {
	path := tmpLog(t)
	Append(path, "db-health", "warn", "WAL 400MB", "fusca")
	Append(path, "db-health", "warn", "WAL 450MB", "fusca")
	Append(path, "db-health", "warn", "WAL 503MB", "fusca")

	rules := []TrendRule{
		{Probe: "db-health", Window: 3, ConsecutiveStatus: "warn", EscalateTo: "critical",
			Signal: "db-health: {n} execuções consecutivas em warn"},
	}
	signals := evalRules(path, rules)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %v", len(signals), signals)
	}
	if signals[0] != "[critical] db-health: 3 execuções consecutivas em warn" {
		t.Errorf("unexpected signal: %q", signals[0])
	}
}

func TestEvalRules_NotEnoughHistory(t *testing.T) {
	path := tmpLog(t)
	Append(path, "db-health", "warn", "WAL 400MB", "fusca")
	Append(path, "db-health", "warn", "WAL 450MB", "fusca")
	// only 2, window = 3

	rules := []TrendRule{
		{Probe: "db-health", Window: 3, ConsecutiveStatus: "warn", EscalateTo: "critical",
			Signal: "signal"},
	}
	signals := evalRules(path, rules)
	if len(signals) != 0 {
		t.Errorf("expected no signal (insufficient history), got %v", signals)
	}
}

func TestEvalRules_InterruptedSequence(t *testing.T) {
	path := tmpLog(t)
	Append(path, "db-health", "warn", "s", "r")
	Append(path, "db-health", "ok", "s", "r") // breaks the streak
	Append(path, "db-health", "warn", "s", "r")

	rules := []TrendRule{
		{Probe: "db-health", Window: 3, ConsecutiveStatus: "warn", EscalateTo: "critical", Signal: "s"},
	}
	signals := evalRules(path, rules)
	if len(signals) != 0 {
		t.Errorf("expected no signal (streak broken by ok), got %v", signals)
	}
}

func TestEvalRules_MixedProbes_OnlyTargetFires(t *testing.T) {
	path := tmpLog(t)
	Append(path, "airbyte", "warn", "s", "r")
	Append(path, "db-health", "warn", "s", "r")
	Append(path, "db-health", "warn", "s", "r")
	Append(path, "db-health", "warn", "s", "r")

	rules := []TrendRule{
		{Probe: "db-health", Window: 3, ConsecutiveStatus: "warn", EscalateTo: "critical", Signal: "db-health {n}"},
		{Probe: "airbyte", Window: 3, ConsecutiveStatus: "warn", EscalateTo: "warn", Signal: "airbyte {n}"},
	}
	signals := evalRules(path, rules)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (only db-health), got %d: %v", len(signals), signals)
	}
}

// --- LoadStoreConfig ---

func TestLoadStoreConfig_Embedded(t *testing.T) {
	cfg, err := LoadStoreConfig()
	if err != nil {
		t.Fatalf("LoadStoreConfig failed: %v", err)
	}
	if len(cfg.TrendRules) == 0 {
		t.Error("expected at least one trend rule in embedded config")
	}
	for i, r := range cfg.TrendRules {
		if r.Probe == "" || r.Window == 0 || r.ConsecutiveStatus == "" {
			t.Errorf("rule[%d] missing required fields: %+v", i, r)
		}
	}
}

// --- Schema validation ---

func TestSchema_IDAutoincrement(t *testing.T) {
	path := tmpLog(t)
	for i := 0; i < 3; i++ {
		if err := Append(path, "k8s-status", "ok", "signal", "repo"); err != nil {
			t.Fatalf("Append %d failed: %v", i, err)
		}
	}
	records, _ := ReadAll(path)
	if len(records) != 3 {
		t.Fatalf("expected 3, got %d", len(records))
	}
	// All records should have valid timestamps
	for _, r := range records {
		if r.Ts == "" {
			t.Errorf("record missing ts: %+v", r)
		}
	}
}

func TestSchema_IndexedQueryPerformance(t *testing.T) {
	path := tmpLog(t)
	// Insert mixed probes to verify index-based filtering works correctly
	for i := 0; i < 20; i++ {
		Append(path, "db-health", "ok", "s", "r")
		Append(path, "airbyte", "warn", "s", "r")
	}
	results := QueryTrend(path, "db-health", 5)
	if len(results) != 5 {
		t.Fatalf("expected 5 db-health records via index, got %d", len(results))
	}
	for _, r := range results {
		if r.Probe != "db-health" {
			t.Errorf("index filter returned wrong probe: %s", r.Probe)
		}
	}
}
