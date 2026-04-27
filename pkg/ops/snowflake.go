package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SnowflakeConfig holds Snowflake connection settings for snowsql CLI.
type SnowflakeConfig struct {
	Account   string
	User      string
	Password  string // F001: passed as SNOWSQL_PWD env var to subprocess, never as CLI arg
	Warehouse string
	Database  string
	Schema    string
	Query     string
}

// WarehouseCostConfig extends SnowflakeConfig for warehouse cost analysis (Discovery 008-C).
type WarehouseCostConfig struct {
	SnowflakeConfig
	GapRealMin float64 // optional: gap between syncs in minutes for heuristic signal
}

// CheckSnowflake runs a Snowflake query via snowsql and evaluates results.
func CheckSnowflake(cfg SnowflakeConfig) OpsResult {
	if cfg.Account == "" || cfg.User == "" || cfg.Query == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "Snowflake: missing account, user, or query",
			Actions: []string{"set --input snowflake-account=..., snowflake-user=..., snowflake-query=..."},
			Cost:    "zero-llm",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{
		"-a", cfg.Account,
		"-u", cfg.User,
		"-q", cfg.Query,
		"-o", "output_format=tsv",
		"-o", "friendly=false",
	}
	if cfg.Warehouse != "" {
		args = append(args, "-w", cfg.Warehouse)
	}
	if cfg.Database != "" {
		args = append(args, "-d", cfg.Database)
	}
	if cfg.Schema != "" {
		args = append(args, "-s", cfg.Schema)
	}

	start := time.Now()
	// F001: pass password via SNOWSQL_PWD env var, not as CLI arg.
	// If cfg.Password is empty, SNOWSQL_PWD must already be set in the caller's environment.
	var output []byte
	var err error
	if cfg.Password != "" {
		output, err = shellExecEnv(ctx, []string{"SNOWSQL_PWD=" + cfg.Password}, "snowsql", args...)
	} else {
		output, err = shellExec(ctx, "snowsql", args...)
	}
	elapsed := time.Since(start)

	return evaluateSnowflake(output, err, elapsed)
}

// CheckSnowflakeWarehouseCost queries warehouse configuration and 24h metering history,
// then applies a gap heuristic: does gap_real_min > auto_suspend_min?
// F001: password passed via SNOWSQL_PWD env var, never as CLI arg.
func CheckSnowflakeWarehouseCost(cfg WarehouseCostConfig) OpsResult {
	if cfg.Account == "" || cfg.User == "" || cfg.Warehouse == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "Snowflake warehouse-cost: missing account, user, or warehouse",
			Actions: []string{"set snowflake-account, snowflake-user, snowflake-warehouse"},
			Cost:    "zero-llm",
		}
	}

	autoSuspendSec, state, size, err := queryWarehouseConfig(cfg.SnowflakeConfig)
	if err != nil {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("Snowflake warehouse-cost: SHOW WAREHOUSES failed: %v", err),
			Actions: []string{"verify SNOWSQL_PWD is set, check warehouse name and permissions"},
			Cost:    "zero-llm",
		}
	}

	creditsDay, _ := queryWarehouseMetering(cfg.SnowflakeConfig) // non-critical

	autoSuspendMin := float64(autoSuspendSec) / 60.0
	costMonthEst := creditsDay * 30 * 2.0 // X-Small ~$2/credit; 0 if metering unavailable

	status, suspendSignal, actions := evalGapHeuristic(cfg.GapRealMin, autoSuspendMin, autoSuspendSec, cfg.Warehouse)

	signal := fmt.Sprintf("Warehouse %s (%s/%s): AUTO_SUSPEND %ds — %s",
		cfg.Warehouse, size, state, autoSuspendSec, suspendSignal)
	if creditsDay > 0 {
		signal += fmt.Sprintf(" | %.1f créditos/dia → ~$%.0f/mês", creditsDay, costMonthEst)
	}

	return OpsResult{
		Status: status,
		Signal: signal,
		Data: map[string]any{
			"warehouse":          cfg.Warehouse,
			"state":              state,
			"size":               size,
			"auto_suspend_sec":   autoSuspendSec,
			"auto_suspend_min":   autoSuspendMin,
			"credits_day":        creditsDay,
			"cost_month_est_usd": costMonthEst,
			"gap_real_min":       cfg.GapRealMin,
		},
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// queryWarehouseConfig runs SHOW WAREHOUSES and extracts auto_suspend, state and size.
func queryWarehouseConfig(cfg SnowflakeConfig) (autoSuspendSec int, state, size string, err error) {
	query := fmt.Sprintf("SHOW WAREHOUSES LIKE '%s';", cfg.Warehouse)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{
		"-a", cfg.Account, "-u", cfg.User,
		"-q", query,
		"-o", "output_format=json",
		"-o", "friendly=false",
	}

	var out []byte
	if cfg.Password != "" {
		out, err = shellExecEnv(ctx, []string{"SNOWSQL_PWD=" + cfg.Password}, "snowsql", args...)
	} else {
		out, err = shellExec(ctx, "snowsql", args...)
	}
	if err != nil {
		return 0, "", "", err
	}

	return parseShowWarehousesJSON(out, cfg.Warehouse)
}

// parseShowWarehousesJSON parses snowsql JSON output from SHOW WAREHOUSES.
// snowsql may prefix the JSON array with banner lines; we locate the first '['.
func parseShowWarehousesJSON(data []byte, warehouseName string) (autoSuspendSec int, state, size string, err error) {
	raw := string(data)
	start := strings.Index(raw, "[")
	if start == -1 {
		return 0, "", "", fmt.Errorf("no JSON array in SHOW WAREHOUSES output")
	}

	var rows []map[string]any
	if err = json.Unmarshal([]byte(raw[start:]), &rows); err != nil {
		return 0, "", "", fmt.Errorf("failed to parse SHOW WAREHOUSES JSON: %w", err)
	}
	if len(rows) == 0 {
		return 0, "", "", fmt.Errorf("warehouse %q not found", warehouseName)
	}

	row := rows[0]
	switch v := row["auto_suspend"].(type) {
	case float64:
		autoSuspendSec = int(v)
	case string:
		autoSuspendSec, _ = strconv.Atoi(v)
	}
	state = fmt.Sprintf("%v", row["state"])
	size = fmt.Sprintf("%v", row["size"])
	return autoSuspendSec, state, size, nil
}

// queryWarehouseMetering returns total credits used in the last 24h.
// Uses INFORMATION_SCHEMA — no ACCOUNTADMIN required. Returns 0 on any error.
func queryWarehouseMetering(cfg SnowflakeConfig) (creditsDay float64, err error) {
	query := fmt.Sprintf(
		"SELECT ROUND(SUM(CREDITS_USED),2) FROM TABLE(INFORMATION_SCHEMA.WAREHOUSE_METERING_HISTORY("+
			"DATE_RANGE_START=>DATEADD(HOUR,-24,CURRENT_TIMESTAMP()),"+
			"DATE_RANGE_END=>CURRENT_TIMESTAMP())) WHERE WAREHOUSE_NAME='%s';",
		cfg.Warehouse,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{
		"-a", cfg.Account, "-u", cfg.User,
		"-q", query,
		"-o", "output_format=tsv",
		"-o", "friendly=false",
	}
	if cfg.Warehouse != "" {
		args = append(args, "-w", cfg.Warehouse)
	}

	var out []byte
	if cfg.Password != "" {
		out, err = shellExecEnv(ctx, []string{"SNOWSQL_PWD=" + cfg.Password}, "snowsql", args...)
	} else {
		out, err = shellExec(ctx, "snowsql", args...)
	}
	if err != nil {
		return 0, err
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "|") ||
			strings.HasPrefix(line, "ROUND") || strings.HasPrefix(line, "round") {
			continue
		}
		if v, parseErr := strconv.ParseFloat(line, 64); parseErr == nil {
			return v, nil
		}
	}
	return 0, nil
}

// evalGapHeuristic returns status, signal and suggested actions based on gap vs auto_suspend.
func evalGapHeuristic(gapRealMin, autoSuspendMin float64, autoSuspendSec int, warehouse string) (status, signal string, actions []string) {
	if gapRealMin <= 0 {
		return "ok",
			fmt.Sprintf("AUTO_SUSPEND %ds (%.0fmin) — forneça --gap-real-min para análise de suspensão", autoSuspendSec, autoSuspendMin),
			[]string{"run with --gap-real-min=<schedule_min - avg_duration_min>"}
	}
	if gapRealMin > autoSuspendMin {
		return "ok",
			fmt.Sprintf("gap %.1fmin > AUTO_SUSPEND %.0fmin → suspende entre syncs ✓", gapRealMin, autoSuspendMin),
			nil
	}
	recommended := int(gapRealMin * 60 * 0.8)
	return "warn",
		fmt.Sprintf("gap %.1fmin ≤ AUTO_SUSPEND %.0fmin → warehouse always-on", gapRealMin, autoSuspendMin),
		[]string{fmt.Sprintf("ALTER WAREHOUSE %s SET AUTO_SUSPEND = %d;", warehouse, recommended)}
}

func evaluateSnowflake(output []byte, err error, elapsed time.Duration) OpsResult {
	if err != nil {
		actions := []string{"check credentials and network"}
		if strings.Contains(err.Error(), "executable file not found") {
			actions = []string{
				"install snowsql: https://developers.snowflake.com/snowsql/ or `brew install --cask snowflake-snowsql`",
				"add to PATH: export PATH=$PATH:/Applications/SnowSQL.app/Contents/MacOS",
			}
		}
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("Snowflake: snowsql failed: %v", err),
			Data:    map[string]any{"output": string(output)},
			Actions: actions,
			Cost:    "zero-llm",
		}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	rowCount := len(lines)
	if rowCount > 0 && (lines[0] == "" || strings.HasPrefix(lines[0], "+-")) {
		rowCount = 0
		for _, line := range lines {
			if line != "" && !strings.HasPrefix(line, "+-") && !strings.HasPrefix(line, "|") {
				rowCount++
			}
		}
	}

	data := map[string]any{
		"row_count":    rowCount,
		"elapsed_ms":   elapsed.Milliseconds(),
		"output_bytes": len(output),
	}

	hStatus, hSignal, hActions := EvalHeuristics(data, loadHeuristics("snowflake"))
	signal := fmt.Sprintf("Snowflake: query returned %d rows in %s", rowCount, elapsed.Round(time.Millisecond))
	if hSignal != "" {
		signal += " | " + hSignal
	}
	return OpsResult{
		Status:  hStatus,
		Signal:  signal,
		Data:    data,
		Actions: hActions,
		Cost:    "zero-llm",
	}
}
