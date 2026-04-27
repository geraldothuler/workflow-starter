package ops

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// humanBytes
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(1.5 * 1024 * 1024), "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{int64(10.5 * 1024 * 1024 * 1024), "10.5 GB"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := humanBytes(tc.input)
			if got != tc.want {
				t.Errorf("humanBytes(%d) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// extractFirstJSON
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExtractFirstJSON_FoundOnFirstLine(t *testing.T) {
	output := `{"locks":3,"long_tx":0,"idle_in_tx":0,"total_conn":12,"active_conn":5,"db_size_bytes":1048576,"wal_lag_bytes":0,"tables":null}`
	got := extractFirstJSON(output)
	if got == "" {
		t.Fatal("expected JSON, got empty string")
	}
	if !strings.HasPrefix(got, "{") {
		t.Errorf("expected JSON object, got %q", got)
	}
}

func TestExtractFirstJSON_SkipsNonJSON(t *testing.T) {
	output := "pod/wtb-db-health-1234 created\n{\"locks\":0,\"long_tx\":0,\"idle_in_tx\":0,\"total_conn\":5,\"active_conn\":1,\"db_size_bytes\":512,\"wal_lag_bytes\":0,\"tables\":null}\npod deleted"
	got := extractFirstJSON(output)
	if got == "" {
		t.Fatal("expected JSON to be found after non-JSON lines")
	}
	if !strings.Contains(got, "locks") {
		t.Errorf("unexpected JSON content: %q", got)
	}
}

func TestExtractFirstJSON_NoJSON(t *testing.T) {
	output := "error: connection refused\nno json here"
	got := extractFirstJSON(output)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractFirstJSON_EmptyOutput(t *testing.T) {
	got := extractFirstJSON("")
	if got != "" {
		t.Errorf("expected empty string for empty output")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// evaluateDBHealth
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestEvaluateDBHealth_Healthy(t *testing.T) {
	m := dbMetrics{
		Locks:       0,
		LongTX:      0,
		IdleInTX:    2,
		TotalConn:   20,
		ActiveConn:  5,
		DBSizeBytes: 512 * 1024 * 1024,
		WALLagBytes: 0,
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q (signal: %s)", r.Status, r.Signal)
	}
	if r.Cost != "zero-llm" {
		t.Errorf("expected zero-llm cost")
	}
	if !strings.Contains(r.Signal, "saudável") {
		t.Errorf("expected 'saudável' in signal, got %q", r.Signal)
	}
}

func TestEvaluateDBHealth_CriticalLocks(t *testing.T) {
	m := dbMetrics{
		Locks:       int64(criticalLockCount + 1),
		TotalConn:   50,
		ActiveConn:  10,
		DBSizeBytes: 1024,
		WALLagBytes: 0,
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Status != "critical" {
		t.Errorf("expected critical for locks > threshold, got %q", r.Status)
	}
	if !strings.Contains(r.Signal, "CRÍTICO") {
		t.Errorf("expected CRÍTICO in signal, got %q", r.Signal)
	}
	if len(r.Actions) == 0 {
		t.Error("expected at least one action for lock contention")
	}
}

func TestEvaluateDBHealth_LockCountAtThreshold(t *testing.T) {
	// Exactly at threshold should NOT be critical (> not >=)
	m := dbMetrics{
		Locks:       int64(criticalLockCount),
		TotalConn:   10,
		ActiveConn:  3,
		DBSizeBytes: 1024,
		WALLagBytes: 0,
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Status == "critical" {
		t.Errorf("locks at threshold should not be critical, got %q", r.Status)
	}
}

func TestEvaluateDBHealth_WarnLongTX(t *testing.T) {
	m := dbMetrics{
		Locks:       0,
		LongTX:      2,
		TotalConn:   10,
		ActiveConn:  2,
		DBSizeBytes: 1024,
		WALLagBytes: 0,
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Status != "warn" {
		t.Errorf("expected warn for long transactions, got %q", r.Status)
	}
	if !strings.Contains(r.Signal, "atenção") {
		t.Errorf("expected 'atenção' in signal, got %q", r.Signal)
	}
}

func TestEvaluateDBHealth_CriticalWALLag(t *testing.T) {
	m := dbMetrics{
		Locks:       0,
		TotalConn:   10,
		ActiveConn:  2,
		DBSizeBytes: 1024,
		WALLagBytes: criticalWALLagBytes + 1,
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Status != "critical" {
		t.Errorf("expected critical for WAL lag > 10GB, got %q", r.Status)
	}
}

func TestEvaluateDBHealth_WarnWALLag(t *testing.T) {
	m := dbMetrics{
		Locks:       0,
		TotalConn:   10,
		ActiveConn:  2,
		DBSizeBytes: 1024,
		WALLagBytes: warnWALLagBytes + 1,
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Status != "warn" {
		t.Errorf("expected warn for WAL lag > 1GB, got %q", r.Status)
	}
}

func TestEvaluateDBHealth_WarnDeadTuples(t *testing.T) {
	m := dbMetrics{
		Locks:       0,
		TotalConn:   10,
		ActiveConn:  2,
		DBSizeBytes: 1024,
		WALLagBytes: 0,
		Tables: []tableMetrics{
			{Name: "events", LiveTuples: 1000, DeadTuples: 200}, // 20% > 10% threshold
		},
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Status != "warn" {
		t.Errorf("expected warn for dead tuples > threshold, got %q", r.Status)
	}
	found := false
	for _, a := range r.Actions {
		if strings.Contains(a, "events") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected VACUUM action for table 'events', actions: %v", r.Actions)
	}
}

func TestEvaluateDBHealth_DeadTuplesBelowThreshold(t *testing.T) {
	m := dbMetrics{
		Locks:       0,
		TotalConn:   10,
		ActiveConn:  2,
		DBSizeBytes: 1024,
		WALLagBytes: 0,
		Tables: []tableMetrics{
			{Name: "events", LiveTuples: 1000, DeadTuples: 50}, // 5% < 10% threshold
		},
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Status == "warn" {
		t.Errorf("dead tuples below threshold should not warn, got %q", r.Status)
	}
}

func TestEvaluateDBHealth_DataFields(t *testing.T) {
	m := dbMetrics{
		Locks:       3,
		LongTX:      0,
		TotalConn:   25,
		ActiveConn:  8,
		DBSizeBytes: 2 * 1024 * 1024 * 1024,
		WALLagBytes: 0,
	}
	r := evaluateDBHealth("mydb.internal", m)
	if r.Data["host"] != "mydb.internal" {
		t.Errorf("expected host in data, got %v", r.Data["host"])
	}
	if r.Data["locks"] != int64(3) {
		t.Errorf("expected locks=3 in data, got %v", r.Data["locks"])
	}
	if r.Data["db_size"] != "2.0 GB" {
		t.Errorf("unexpected db_size: %v", r.Data["db_size"])
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// resolvePassword
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestResolvePassword_DirectPassword(t *testing.T) {
	cfg := DBHealthConfig{DBPassword: "secret123"}
	pwd, err := resolvePassword(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pwd != "secret123" {
		t.Errorf("expected direct password, got %q", pwd)
	}
}

func TestResolvePassword_NeitherPasswordNorSSM(t *testing.T) {
	cfg := DBHealthConfig{}
	_, err := resolvePassword(cfg)
	if err == nil {
		t.Error("expected error when no password source provided")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// buildDBSignal
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBuildDBSignal_Healthy(t *testing.T) {
	m := dbMetrics{Locks: 0, LongTX: 0, ActiveConn: 5, TotalConn: 20}
	sig := buildDBSignal("mydb", "ok", m, nil, nil)
	if !strings.Contains(sig, "mydb") {
		t.Errorf("signal missing host: %q", sig)
	}
	if !strings.Contains(sig, "saudável") {
		t.Errorf("expected 'saudável' in ok signal: %q", sig)
	}
}

func TestBuildDBSignal_Critical(t *testing.T) {
	m := dbMetrics{Locks: 10, LongTX: 0, ActiveConn: 5, TotalConn: 20}
	sig := buildDBSignal("mydb", "critical", m, []string{"10 lock waits"}, nil)
	if !strings.Contains(sig, "CRÍTICO") {
		t.Errorf("expected CRÍTICO in critical signal: %q", sig)
	}
	if !strings.Contains(sig, "lock waits") {
		t.Errorf("expected lock detail in signal: %q", sig)
	}
}

func TestBuildDBSignal_Warn(t *testing.T) {
	m := dbMetrics{LongTX: 2, ActiveConn: 3, TotalConn: 15}
	sig := buildDBSignal("mydb", "warn", m, nil, []string{"2 tx longas"})
	if !strings.Contains(sig, "atenção") {
		t.Errorf("expected 'atenção' in warn signal: %q", sig)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// F002: ephemeral Secret helpers
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestPodSecretOverrides_ValidJSON(t *testing.T) {
	json, err := podSecretOverrides("wtb-db-health-1234", "wtb-db-health-1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(json, `"envFrom"`) {
		t.Errorf("expected envFrom in overrides JSON: %s", json)
	}
	if !strings.Contains(json, `"secretRef"`) {
		t.Errorf("expected secretRef in overrides JSON: %s", json)
	}
	if !strings.Contains(json, `wtb-db-health-1234`) {
		t.Errorf("expected secret name in overrides JSON: %s", json)
	}
	if strings.Contains(json, "PGPASSWORD") {
		t.Errorf("password must NOT appear in overrides JSON: %s", json)
	}
}

func TestCreateEphemeralSecret_PassesCorrectArgs(t *testing.T) {
	var capturedArgs []string
	orig := shellExec
	defer func() { shellExec = orig }()
	shellExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return nil, nil
	}

	cfg := DBHealthConfig{Namespace: "org", KubectlContext: "prod"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := createEphemeralSecret(ctx, cfg, "wtb-db-health-42", "s3cr3t"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedArgs) == 0 || capturedArgs[0] != "create" {
		t.Errorf("expected 'create' as first arg, got %v", capturedArgs)
	}
	// Verify PGPASSWORD value is carried in --from-literal
	found := false
	for _, a := range capturedArgs {
		if strings.Contains(a, "PGPASSWORD=s3cr3t") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --from-literal=PGPASSWORD=s3cr3t in args: %v", capturedArgs)
	}
	// Verify namespace and context are passed
	nsFound, ctxFound := false, false
	for i, a := range capturedArgs {
		if a == "-n" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "org" {
			nsFound = true
		}
		if a == "--context" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "prod" {
			ctxFound = true
		}
	}
	if !nsFound {
		t.Errorf("expected -n org in args: %v", capturedArgs)
	}
	if !ctxFound {
		t.Errorf("expected --context prod in args: %v", capturedArgs)
	}
}

func TestCreateEphemeralSecret_ReturnsError(t *testing.T) {
	orig := shellExec
	defer func() { shellExec = orig }()
	shellExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("RBAC denied")
	}

	cfg := DBHealthConfig{Namespace: "org", KubectlContext: "prod"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := createEphemeralSecret(ctx, cfg, "wtb-db-health-99", "pw")
	if err == nil {
		t.Error("expected error from shellExec failure")
	}
}

func TestDeleteEphemeralSecret_PassesIgnoreNotFound(t *testing.T) {
	var capturedArgs []string
	orig := shellExec
	defer func() { shellExec = orig }()
	shellExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return nil, nil
	}

	cfg := DBHealthConfig{Namespace: "org", KubectlContext: "prod"}
	deleteEphemeralSecret(cfg, "wtb-db-health-42")

	found := false
	for _, a := range capturedArgs {
		if a == "--ignore-not-found" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --ignore-not-found in delete args: %v", capturedArgs)
	}
}

func TestFetchDBMetrics_SecretPath_NoPasswordInPodArgs(t *testing.T) {
	calls := 0
	orig := shellExec
	defer func() { shellExec = orig }()

	metricsJSON := `{"locks":0,"long_tx":0,"idle_in_tx":0,"total_conn":10,"active_conn":2,"db_size_bytes":1048576,"wal_lag_bytes":0,"tables":null}`
	var podArgs []string
	shellExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		calls++
		switch calls {
		case 1: // createEphemeralSecret
			return nil, nil
		case 2: // kubectl run pod
			podArgs = args
			return []byte(metricsJSON), nil
		default: // deleteEphemeralSecret
			return nil, nil
		}
	}

	cfg := DBHealthConfig{
		Namespace:      "org",
		KubectlContext: "prod",
		DBHost:         "db.internal",
		DBPort:         "5432",
		DBUser:         "app",
		DBName:         "mydb",
	}

	_, err := fetchDBMetrics(cfg, "supersecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// PGPASSWORD must NOT appear as a direct --env arg in the pod run call
	for _, a := range podArgs {
		if strings.Contains(a, "PGPASSWORD=supersecret") {
			t.Errorf("PGPASSWORD must not appear in pod run args when Secret path is used: %v", podArgs)
		}
	}

	// --overrides must be present
	found := false
	for _, a := range podArgs {
		if a == "--overrides" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --overrides in pod run args: %v", podArgs)
	}
}

func TestFetchDBMetrics_FallbackPath_PasswordInPodArgs(t *testing.T) {
	calls := 0
	orig := shellExec
	defer func() { shellExec = orig }()

	metricsJSON := `{"locks":0,"long_tx":0,"idle_in_tx":0,"total_conn":10,"active_conn":2,"db_size_bytes":1048576,"wal_lag_bytes":0,"tables":null}`
	var podArgs []string
	shellExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		calls++
		switch calls {
		case 1: // createEphemeralSecret fails → RBAC denied
			return nil, fmt.Errorf("forbidden")
		case 2: // kubectl run pod (fallback path)
			podArgs = args
			return []byte(metricsJSON), nil
		default:
			return nil, nil
		}
	}

	cfg := DBHealthConfig{
		Namespace:      "org",
		KubectlContext: "prod",
		DBHost:         "db.internal",
		DBPort:         "5432",
		DBUser:         "app",
		DBName:         "mydb",
	}

	_, err := fetchDBMetrics(cfg, "supersecret")
	if err != nil {
		t.Fatalf("unexpected error on fallback path: %v", err)
	}

	// Fallback: PGPASSWORD must appear as --env
	found := false
	for _, a := range podArgs {
		if strings.Contains(a, "PGPASSWORD=supersecret") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PGPASSWORD in fallback pod run args: %v", podArgs)
	}

	// --overrides must NOT be present in fallback
	for _, a := range podArgs {
		if a == "--overrides" {
			t.Errorf("--overrides must not appear in fallback pod run args: %v", podArgs)
		}
	}
}
