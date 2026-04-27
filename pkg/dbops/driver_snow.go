package dbops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// QuerySnowflake executes a SQL query via snowsql subprocess using SSO
// (externalbrowser authenticator). On first call it opens a browser for login;
// subsequent calls reuse the cached session token.
func QuerySnowflake(account, user, role, warehouse, database, schema, query string) ([]map[string]any, error) {
	// Write query to temp file to avoid shell quoting issues
	tmp, err := os.CreateTemp("", "wtb-snow-*.sql")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())

	sql := strings.TrimRight(strings.TrimSpace(query), ";") + ";"
	if _, err := tmp.WriteString(sql); err != nil {
		return nil, err
	}
	tmp.Close()

	args := []string{
		"-a", account,
		"-u", user,
		"--authenticator", "externalbrowser",
		"-o", "output_format=json",
		"-o", "timing=false",
		"-o", "friendly=false",
		"-f", tmp.Name(),
	}
	if role != "" {
		args = append(args, "--rolename", role)
	}
	if warehouse != "" {
		args = append(args, "--warehouse", warehouse)
	}
	if database != "" {
		args = append(args, "--dbname", database)
	}
	if schema != "" {
		args = append(args, "--schemaname", schema)
	}

	var stdout strings.Builder
	cmd := exec.Command("snowsql", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr // SSO browser prompts go to stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("snowsql: %w", err)
	}

	return parseSnowflakeOutput(stdout.String())
}

// parseSnowflakeOutput parses snowsql JSON output from stdout.
// snowsql -o output_format=json emits one JSON array per result set.
func parseSnowflakeOutput(output string) ([]map[string]any, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return []map[string]any{}, nil
	}

	// Try direct JSON array
	var rows []map[string]any
	if err := json.Unmarshal([]byte(trimmed), &rows); err == nil {
		return rows, nil
	}

	// Try wrapped {"data": [...]}
	var wrapped struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(trimmed), &wrapped); err == nil {
		return wrapped.Data, nil
	}

	// snowsql may emit banners before the JSON — find the first '[' or '{'
	start := strings.IndexAny(trimmed, "[{")
	if start > 0 {
		return parseSnowflakeOutput(trimmed[start:])
	}

	preview := trimmed
	if len(preview) > 300 {
		preview = preview[:300]
	}
	return nil, fmt.Errorf("could not parse snowsql output:\n%s", preview)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
