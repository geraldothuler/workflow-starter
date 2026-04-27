package memory

import (
	"os"
	"path/filepath"
	"testing"
)

// ── TopicMap ──────────────────────────────────────────────────────────────

func TestTopicMap_LoadDefault(t *testing.T) {
	tm, err := LoadTopicMap(t.TempDir()) // no custom file → falls back to embedded
	if err != nil {
		t.Fatalf("LoadTopicMap: %v", err)
	}
	if len(tm.Topics) == 0 {
		t.Error("expected non-empty topic map from embedded default")
	}
}

func TestTopicMap_CustomOverride(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "memory")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "topics:\n  mytest: docs/some/file.md\n"
	if err := os.WriteFile(filepath.Join(dir, "topic-map.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tm, err := LoadTopicMap(root)
	if err != nil {
		t.Fatalf("LoadTopicMap with custom file: %v", err)
	}
	if tm.Topics["mytest"] != "docs/some/file.md" {
		t.Errorf("custom topic not loaded, got %q", tm.Topics["mytest"])
	}
}

func TestTopicMap_Resolve_UnknownTopic(t *testing.T) {
	tm, _ := LoadTopicMap(t.TempDir())
	_, err := tm.Resolve(t.TempDir(), "nonexistent-topic-xyz")
	if err == nil {
		t.Error("expected error for unknown topic")
	}
}

func TestTopicMap_EmbeddedHasExpectedTopics(t *testing.T) {
	tm, _ := LoadTopicMap(t.TempDir())
	required := []string{"webhook", "airbyte", "datadog", "kotlin", "heuristics", "code-review"}
	for _, topic := range required {
		if _, ok := tm.Topics[topic]; !ok {
			t.Errorf("embedded topic-map.yml missing required topic %q", topic)
		}
	}
}

// ── ContextStore ──────────────────────────────────────────────────────────

func TestStore_SetAndGet(t *testing.T) {
	root := t.TempDir()
	s, _ := LoadStore(root)

	s.Set("my_key", "42", "threshold", "webhook", "test entry")
	e, ok := s.Get("my_key")
	if !ok {
		t.Fatal("Get returned false after Set")
	}
	if e.Value != "42" || e.Type != "threshold" || e.Topic != "webhook" {
		t.Errorf("unexpected entry: %+v", e)
	}
}

func TestStore_SetUpdatesExisting(t *testing.T) {
	root := t.TempDir()
	s, _ := LoadStore(root)

	s.Set("k", "v1", "fact", "", "")
	s.Set("k", "v2", "fact", "webhook", "updated")
	e, _ := s.Get("k")
	if e.Value != "v2" {
		t.Errorf("expected v2, got %q", e.Value)
	}
	if e.Topic != "webhook" {
		t.Errorf("expected topic updated to webhook, got %q", e.Topic)
	}
	all := s.FilterByTopic("")
	if len(all) != 1 {
		t.Errorf("expected 1 entry (no duplicate), got %d", len(all))
	}
}

func TestStore_SaveAndReload(t *testing.T) {
	root := t.TempDir()
	s, _ := LoadStore(root)
	s.Set("threshold_mi", "3200", "threshold", "webhook", "TM memory limit")

	if err := s.Save(root); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := LoadStore(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	e, ok := s2.Get("threshold_mi")
	if !ok {
		t.Fatal("entry not found after reload")
	}
	if e.Value != "3200" {
		t.Errorf("unexpected value: %q", e.Value)
	}
}

func TestStore_FilterByTopic(t *testing.T) {
	root := t.TempDir()
	s, _ := LoadStore(root)
	s.Set("k1", "v1", "fact", "webhook", "")
	s.Set("k2", "v2", "fact", "airbyte", "")
	s.Set("k3", "v3", "fact", "webhook", "")

	filtered := s.FilterByTopic("webhook")
	if len(filtered) != 2 {
		t.Errorf("expected 2 webhook entries, got %d", len(filtered))
	}
}

func TestStore_EmptyStore(t *testing.T) {
	root := t.TempDir()
	s, err := LoadStore(root)
	if err != nil {
		t.Fatalf("LoadStore on empty dir: %v", err)
	}
	all := s.FilterByTopic("")
	if len(all) != 0 {
		t.Errorf("expected empty store, got %d entries", len(all))
	}
}

func TestStore_PersistsAcrossReload(t *testing.T) {
	root := t.TempDir()
	s, _ := LoadStore(root)
	s.Set("k", "v", "fact", "test", "desc")

	s2, err := LoadStore(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	e, ok := s2.Get("k")
	if !ok {
		t.Fatal("entry not found after reload from same DB")
	}
	if e.Value != "v" {
		t.Errorf("unexpected value: %q", e.Value)
	}
}

// ── Router ────────────────────────────────────────────────────────────────

func TestRoute_CredentialGoesToKeychain(t *testing.T) {
	cases := []string{
		"Datadog API key para produção",
		"slack bot token",
		"webhook client_id para frota ABC",
		"Airbyte connection_id fusca CDC",
	}
	for _, c := range cases {
		r := Route(c)
		if r.Dest != DestKeychain {
			t.Errorf("Route(%q) = %v, want DestKeychain", c, r.Dest)
		}
	}
}

func TestRoute_NumericFactGoesToContextJSON(t *testing.T) {
	// Must have a numeric value to route to ContextJSON
	cases := []string{
		"webhook-builder TM memory limit é 3200Mi",
		"checkpoint interval safe default 5000ms",
		"fusca-api timeout 30000ms",
		"cerberus replica count 2",
	}
	for _, c := range cases {
		r := Route(c)
		if r.Dest != DestContextJSON {
			t.Errorf("Route(%q) = %v, want DestContextJSON", c, r.Dest)
		}
	}
}

func TestRoute_NarrativeGoesToTopicFile(t *testing.T) {
	cases := []string{
		"Flink checkpoint interval agressivo causa cascata de backpressure",
		"Sherlock-driver lê fusca PostgreSQL diretamente via Slick",
		"HikariCP keepaliveTime deve usar lowercase a",
	}
	for _, c := range cases {
		r := Route(c)
		if r.Dest != DestTopicFile {
			t.Errorf("Route(%q) = %v, want DestTopicFile", c, r.Dest)
		}
		if r.TopicFile == "" {
			t.Errorf("Route(%q): TopicFile is empty", c)
		}
	}
}

func TestRoute_FlinkMapsToHeuristicsFile(t *testing.T) {
	// No explicit number → narrative heuristic → topic file
	r := Route("Flink checkpoint interval agressivo causa risco de cascata")
	if r.TopicFile != "memory/heuristics-ops.md" {
		t.Errorf("expected heuristics-ops.md, got %q", r.TopicFile)
	}
}

func TestRoute_KeySuggestionIsSnakeCase(t *testing.T) {
	r := Route("webhook builder TM memory limit 3200Mi")
	key := r.KeySuggestion
	for _, c := range key {
		if c != '_' && !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') {
			t.Errorf("key %q contains non-snake_case char %q", key, c)
		}
	}
}

func TestToSnakeKey_Basic(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"webhook builder memory limit", "webhook_builder_memory_limit"},
		{"Airbyte connection ID fusca CDC", "airbyte_connection_id_fusca_cdc"},
	}
	for _, c := range cases {
		got := toSnakeKey(c.input)
		if got != c.want {
			t.Errorf("toSnakeKey(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
