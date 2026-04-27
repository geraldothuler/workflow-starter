package ops

import (
	"context"
	"strings"
	"testing"
)

// ── F001: password via env var ────────────────────────────────────────────────

func TestCheckSnowflake_PasswordViaEnvVar(t *testing.T) {
	var capturedEnv []string
	orig := shellExecEnv
	defer func() { shellExecEnv = orig }()

	shellExecEnv = func(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
		capturedEnv = env
		return []byte("col1\nval1"), nil
	}

	CheckSnowflake(SnowflakeConfig{
		Account: "acct", User: "user", Password: "s3cr3t", Query: "SELECT 1",
	})

	if len(capturedEnv) == 0 {
		t.Fatal("expected shellExecEnv to be called with env vars")
	}
	found := false
	for _, e := range capturedEnv {
		if e == "SNOWSQL_PWD=s3cr3t" {
			found = true
		}
	}
	if !found {
		t.Errorf("SNOWSQL_PWD not set in env; got %v", capturedEnv)
	}
}

func TestCheckSnowflake_NoPasswordUsesShellExec(t *testing.T) {
	envCalled := false
	origEnv := shellExecEnv
	defer func() { shellExecEnv = origEnv }()
	shellExecEnv = func(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
		envCalled = true
		return nil, nil
	}

	origExec := shellExec
	defer func() { shellExec = origExec }()
	shellExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("col\nval"), nil
	}

	CheckSnowflake(SnowflakeConfig{Account: "a", User: "u", Query: "SELECT 1"})

	if envCalled {
		t.Error("shellExecEnv should not be called when Password is empty")
	}
}

// ── parseShowWarehousesJSON ───────────────────────────────────────────────────

func TestParseShowWarehousesJSON_Basic(t *testing.T) {
	raw := []byte(`[{"name":"MY_WH","state":"SUSPENDED","size":"X-Small","auto_suspend":300}]`)
	sec, state, size, err := parseShowWarehousesJSON(raw, "MY_WH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec != 300 {
		t.Errorf("auto_suspend: want 300, got %d", sec)
	}
	if state != "SUSPENDED" {
		t.Errorf("state: want SUSPENDED, got %q", state)
	}
	if size != "X-Small" {
		t.Errorf("size: want X-Small, got %q", size)
	}
}

func TestParseShowWarehousesJSON_WithBanner(t *testing.T) {
	// snowsql may prepend banner lines before the JSON array
	raw := []byte("Snowflake - CLI\nVersion 1.2.3\n[{\"name\":\"WH\",\"state\":\"RUNNING\",\"size\":\"Small\",\"auto_suspend\":\"600\"}]")
	sec, _, _, err := parseShowWarehousesJSON(raw, "WH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec != 600 {
		t.Errorf("want 600, got %d", sec)
	}
}

func TestParseShowWarehousesJSON_NotFound(t *testing.T) {
	raw := []byte(`[]`)
	_, _, _, err := parseShowWarehousesJSON(raw, "MISSING")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestParseShowWarehousesJSON_NoArray(t *testing.T) {
	_, _, _, err := parseShowWarehousesJSON([]byte("no json here"), "WH")
	if err == nil {
		t.Error("expected error for missing JSON array")
	}
}

// ── evalGapHeuristic ─────────────────────────────────────────────────────────

func TestEvalGapHeuristic_NoGap(t *testing.T) {
	status, signal, actions := evalGapHeuristic(0, 10, 600, "WH")
	if status != "ok" {
		t.Errorf("want ok, got %q", status)
	}
	if !strings.Contains(signal, "forneça") {
		t.Errorf("expected hint about --gap-real-min, got %q", signal)
	}
	if len(actions) == 0 {
		t.Error("expected action hint")
	}
}

func TestEvalGapHeuristic_GapExceedsAutoSuspend(t *testing.T) {
	// gap 6.6min > auto_suspend 5min → ok
	status, signal, actions := evalGapHeuristic(6.6, 5, 300, "WH")
	if status != "ok" {
		t.Errorf("want ok, got %q", status)
	}
	if !strings.Contains(signal, "suspende entre syncs") {
		t.Errorf("unexpected signal: %q", signal)
	}
	if len(actions) != 0 {
		t.Errorf("expected no actions, got %v", actions)
	}
}

func TestEvalGapHeuristic_WarehouseAlwaysOn(t *testing.T) {
	// gap 6.6min < auto_suspend 10min → warn always-on
	status, signal, actions := evalGapHeuristic(6.6, 10, 600, "DATA_PLATFORM_AIRBYTE_WH")
	if status != "warn" {
		t.Errorf("want warn, got %q", status)
	}
	if !strings.Contains(signal, "always-on") {
		t.Errorf("unexpected signal: %q", signal)
	}
	if len(actions) == 0 {
		t.Error("expected ALTER WAREHOUSE action")
	}
	if !strings.Contains(actions[0], "ALTER WAREHOUSE") {
		t.Errorf("expected ALTER WAREHOUSE in action, got %q", actions[0])
	}
}

func TestEvalGapHeuristic_AlterRecommendedBelow80Pct(t *testing.T) {
	// gap 6.6min → recommended auto_suspend = 6.6*60*0.8 = 316s → < gap
	_, _, actions := evalGapHeuristic(6.6, 10, 600, "WH")
	if len(actions) == 0 {
		t.Fatal("expected action")
	}
	if !strings.Contains(actions[0], "316") && !strings.Contains(actions[0], "317") {
		t.Errorf("expected ~316s recommendation, got %q", actions[0])
	}
}

// ── CheckSnowflakeWarehouseCost: missing config ───────────────────────────────

func TestCheckSnowflakeWarehouseCost_MissingConfig(t *testing.T) {
	r := CheckSnowflakeWarehouseCost(WarehouseCostConfig{})
	if r.Status != "error" {
		t.Errorf("want error, got %q", r.Status)
	}
}
