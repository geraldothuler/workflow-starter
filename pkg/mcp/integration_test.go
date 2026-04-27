package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// ── helpers ─────────────────────────────────────────────────────────────────

// keysOf returns the top-level keys of a map (for debugging assertions).
func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// callToolReq builds a CallToolRequest with the given arguments map.
func callToolReq(args map[string]any) mcplib.CallToolRequest {
	return mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Arguments: args,
		},
	}
}

// resultText extracts the text content from a CallToolResult.
func resultText(t *testing.T, res *mcplib.CallToolResult) string {
	t.Helper()
	if res == nil {
		t.Fatal("result is nil")
	}
	if len(res.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := res.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

// seedUseCaseDefinition creates a minimal definition.yml for testing.
func seedUseCaseDefinition(t *testing.T, workflowHome, id string) {
	t.Helper()
	dir := filepath.Join(workflowHome, "use-cases", id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	def := `id: ` + id + `
type: documentary
name: Test ` + id + `
version: "1.0"
description: Test use-case for integration tests
primitives: []
triggers: []
inputs: []
steps: []
artefacts: []
chain:
  from: []
  to: []
`
	if err := os.WriteFile(filepath.Join(dir, "definition.yml"), []byte(def), 0644); err != nil {
		t.Fatal(err)
	}
}

// seedTemplate creates a minimal template.md for scaffold tests.
func seedTemplate(t *testing.T, workflowHome, wfType string) {
	t.Helper()
	dir := filepath.Join(workflowHome, "use-cases", wfType)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	tmpl := `# {{.Type}} — {{.Context}} (YYYY-MM-DD)

**ID:** NNN
**Date:** YYYY-MM-DD
**Type:** {{.Type}}
`
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(tmpl), 0644); err != nil {
		t.Fatal(err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// WORKFLOW tools
// ═══════════════════════════════════════════════════════════════════════════

func TestWorkflowRun_InvalidUseCase(t *testing.T) {
	workflowHome := t.TempDir()
	repoPath := t.TempDir()

	// Create use-cases directory (empty)
	if err := os.MkdirAll(filepath.Join(workflowHome, "use-cases"), 0755); err != nil {
		t.Fatal(err)
	}

	handler := handleWorkflowRun(workflowHome, repoPath)
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"use_case": "nonexistent-workflow",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for invalid use-case")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", text)
	}
}

func TestWorkflowRun_MissingUseCaseArg(t *testing.T) {
	workflowHome := t.TempDir()
	repoPath := t.TempDir()

	handler := handleWorkflowRun(workflowHome, repoPath)
	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for missing use_case")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "use_case is required") {
		t.Errorf("expected 'use_case is required', got: %s", text)
	}
}

func TestWorkflowListUseCases_ReturnsKnownUseCases(t *testing.T) {
	workflowHome := t.TempDir()

	seedUseCaseDefinition(t, workflowHome, "test-backlog")
	seedUseCaseDefinition(t, workflowHome, "test-investigation")

	handler := handleWorkflowListUseCases(workflowHome)
	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)

	// Parse JSON result
	var summaries []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(text), &summaries); err != nil {
		t.Fatalf("failed to parse JSON: %v\n%s", err, text)
	}

	if len(summaries) != 2 {
		t.Errorf("expected 2 use-cases, got %d", len(summaries))
	}

	ids := map[string]bool{}
	for _, s := range summaries {
		ids[s.ID] = true
	}
	if !ids["test-backlog"] {
		t.Error("missing use-case: test-backlog")
	}
	if !ids["test-investigation"] {
		t.Error("missing use-case: test-investigation")
	}
}

func TestWorkflowListUseCases_EmptyDir(t *testing.T) {
	workflowHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workflowHome, "use-cases"), 0755); err != nil {
		t.Fatal(err)
	}

	handler := handleWorkflowListUseCases(workflowHome)
	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)
	if text != "null" && text != "[]" {
		t.Errorf("expected null or [] for empty dir, got: %s", text)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// STATUS tools
// ═══════════════════════════════════════════════════════════════════════════

func TestWorkflowStatus_ReturnsStatus(t *testing.T) {
	workflowHome := t.TempDir()

	handler := handleWorkflowStatus(workflowHome)
	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)

	// Should always include these status lines
	required := []string{"security_contract:", "active_premises:", "sessions_1on1:", "personal_dir:", "platform_dir:"}
	for _, key := range required {
		if !strings.Contains(text, key) {
			t.Errorf("expected status to contain %q, got:\n%s", key, text)
		}
	}

	if !strings.Contains(text, workflowHome) {
		t.Errorf("expected status to contain workflowHome path %q", workflowHome)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// OPS tools
// ═══════════════════════════════════════════════════════════════════════════

func TestOpsProbe_GracefulSkip(t *testing.T) {
	handler := handleOpsProbe()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"namespace": "nonexistent-ns",
		"profile":   "nonexistent-profile",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	// ops_probe should NOT panic or return a Go error even when infra
	// is not available. It returns structured JSON with per-check status.
	if res.IsError {
		t.Fatalf("ops_probe should not return IsError when infra is unavailable, got: %s", resultText(t, res))
	}

	text := resultText(t, res)

	// Should contain all four probe keys
	var results map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("expected valid JSON, got: %s", text)
	}

	for _, key := range []string{"auth", "k8s", "db", "kafka"} {
		if _, ok := results[key]; !ok {
			t.Errorf("probe result missing key %q", key)
		}
	}
}

func TestOpsLogsAnalyze_WithFixtureLog(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	logContent := `2026-02-23T10:00:00Z ERROR panic: runtime error: index out of range
goroutine 1 [running]:
main.main()
2026-02-23T10:00:01Z WARN OOMKilled: container exceeded memory limit
2026-02-23T10:00:02Z ERROR slow query detected: SELECT * FROM orders took 45s
2026-02-23T10:00:03Z INFO kafka consumer lag detected: topic=events group=processor lag=50000
`
	if err := os.WriteFile(logFile, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	handler := handleOpsLogsAnalyze()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"logs_file": logFile,
		"patterns":  []any{"all"},
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)

	// Verify the result is valid JSON (OpsResult)
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("expected valid JSON from logs_analyze, got: %s", text)
	}

	// OpsResult should have a status field
	if _, ok := result["status"]; !ok {
		t.Error("logs_analyze result missing 'status' field")
	}
}

func TestOpsLogsAnalyze_MissingFile(t *testing.T) {
	handler := handleOpsLogsAnalyze()
	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for missing logs_file")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "logs_file is required") {
		t.Errorf("expected 'logs_file is required', got: %s", text)
	}
}

func TestOpsPlanNew_CreatesPlan(t *testing.T) {
	handler := handleOpsPlanNew()

	// Test blank plan creation
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"scenario":   "db connection pool exhaustion",
		"namespace":  "production",
		"deployment": "api-server",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)

	// Should be valid JSON Plan
	// Note: Plan struct uses yaml tags, so json.Marshal uses Go field names
	// (capitalized). The top-level key is "Plan" and inner fields like
	// "Scenario" are also capitalized.
	var plan map[string]any
	if err := json.Unmarshal([]byte(text), &plan); err != nil {
		t.Fatalf("expected valid JSON plan, got: %s", text)
	}

	planBody, ok := plan["Plan"].(map[string]any)
	if !ok {
		t.Fatalf("plan JSON missing 'Plan' key, keys: %v", keysOf(plan))
	}

	if scenario, ok := planBody["Scenario"].(string); !ok || scenario != "db connection pool exhaustion" {
		t.Errorf("plan scenario mismatch, got: %v", planBody["Scenario"])
	}
}

func TestOpsPlanNew_WithTemplate(t *testing.T) {
	handler := handleOpsPlanNew()

	res, err := handler(context.Background(), callToolReq(map[string]any{
		"scenario":    "api server rollback needed",
		"template_id": "deploy-rollback",
		"namespace":   "production",
		"deployment":  "api-server",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)

	var plan map[string]any
	if err := json.Unmarshal([]byte(text), &plan); err != nil {
		t.Fatalf("expected valid JSON plan, got: %s", text)
	}

	planBody, ok := plan["Plan"].(map[string]any)
	if !ok {
		t.Fatalf("plan JSON missing 'Plan' key, keys: %v", keysOf(plan))
	}

	// Template-based plan should have steps
	steps, ok := planBody["Steps"].([]any)
	if !ok || len(steps) == 0 {
		t.Error("expected template plan to have steps")
	}
}

func TestOpsPlanNew_InvalidTemplate(t *testing.T) {
	handler := handleOpsPlanNew()

	res, err := handler(context.Background(), callToolReq(map[string]any{
		"scenario":    "test",
		"template_id": "nonexistent-template",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for invalid template_id")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", text)
	}
}

func TestOpsPlanNew_MissingScenario(t *testing.T) {
	handler := handleOpsPlanNew()

	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for missing scenario")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "scenario is required") {
		t.Errorf("expected 'scenario is required', got: %s", text)
	}
}

func TestOpsPlanShow_ValidPlan(t *testing.T) {
	// ops_plan_show expects YAML input (ops.UnmarshalPlan uses yaml.Unmarshal).
	// Construct a valid YAML plan directly.
	planYAML := `plan:
  id: plan-test-001
  scenario: test scenario for show
  risk: medium
  created_at: "2026-02-23T10:00:00Z"
  steps:
    - id: 1
      tool: "echo hello"
      purpose: "test step"
      owner: auto
`
	handlerShow := handleOpsPlanShow()
	res, err := handlerShow(context.Background(), callToolReq(map[string]any{
		"plan_json": planYAML,
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)
	if !strings.Contains(text, "test scenario for show") {
		t.Errorf("plan summary should contain scenario, got: %s", text)
	}
	if !strings.Contains(text, "plan-test-001") {
		t.Errorf("plan summary should contain plan ID, got: %s", text)
	}
}

func TestOpsPlanShow_InvalidJSON(t *testing.T) {
	handler := handleOpsPlanShow()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"plan_json": "not valid json",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for invalid JSON")
	}
}

func TestOpsDbHealth_GracefulWhenNoInfra(t *testing.T) {
	handler := handleOpsDbHealth()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"namespace": "nonexistent",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	// Should not panic, should return valid JSON OpsResult
	if res.IsError {
		t.Fatalf("ops_db_health should not return IsError, got: %s", resultText(t, res))
	}

	text := resultText(t, res)
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s", text)
	}
}

func TestOpsK8sStatus_GracefulWhenNoInfra(t *testing.T) {
	handler := handleOpsK8sStatus()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"namespace":  "nonexistent",
		"deployment": "nonexistent",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("ops_k8s_status should not return IsError, got: %s", resultText(t, res))
	}

	text := resultText(t, res)
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s", text)
	}
}

func TestOpsKafkaStatus_GracefulWhenNoInfra(t *testing.T) {
	handler := handleOpsKafkaStatus()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"namespace":  "nonexistent",
		"deployment": "nonexistent",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("ops_kafka_status should not return IsError, got: %s", resultText(t, res))
	}

	text := resultText(t, res)
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s", text)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// PLAYBOOK tools
// ═══════════════════════════════════════════════════════════════════════════

func TestPlaybookList_ReturnsAvailablePlaybooks(t *testing.T) {
	// Use the real workflow home for embedded playbooks
	workflowHome := t.TempDir()

	handler := handlePlaybookList(workflowHome)
	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)

	// Should be valid JSON array
	var summaries []map[string]any
	if err := json.Unmarshal([]byte(text), &summaries); err != nil {
		t.Fatalf("expected valid JSON array, got: %s", text)
	}

	// Embedded playbooks should have at least one entry
	if len(summaries) == 0 {
		t.Error("expected at least one embedded playbook")
	}

	// Each entry should have id and title
	for i, s := range summaries {
		if _, ok := s["id"]; !ok {
			t.Errorf("playbook[%d] missing 'id'", i)
		}
		if _, ok := s["title"]; !ok {
			t.Errorf("playbook[%d] missing 'title'", i)
		}
	}
}

func TestPlaybookRun_InvalidID(t *testing.T) {
	workflowHome := t.TempDir()

	handler := handlePlaybookRun(workflowHome)
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"playbook_id": "totally-nonexistent-playbook-xyz",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for invalid playbook_id")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", text)
	}
}

func TestPlaybookRun_MissingID(t *testing.T) {
	workflowHome := t.TempDir()

	handler := handlePlaybookRun(workflowHome)
	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for missing playbook_id")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "playbook_id is required") {
		t.Errorf("expected 'playbook_id is required', got: %s", text)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SCAFFOLD tools
// ═══════════════════════════════════════════════════════════════════════════

func TestWorkflowNew_CreatesArtefact(t *testing.T) {
	workflowHome := t.TempDir()
	repoPath := t.TempDir()

	seedTemplate(t, workflowHome, "incident")

	handler := handleWorkflowNew(workflowHome, repoPath)
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"type":      "incident",
		"context":   "db-deadlock",
		"repo_path": repoPath,
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)
	if !strings.Contains(text, "created:") {
		t.Errorf("expected 'created:' in output, got: %s", text)
	}
	if !strings.Contains(text, "incident") {
		t.Errorf("expected 'incident' in output, got: %s", text)
	}
	if !strings.Contains(text, "db-deadlock") {
		t.Errorf("expected 'db-deadlock' in output, got: %s", text)
	}

	// Verify the file was actually created
	typeDir := filepath.Join(repoPath, "docs", "workflow", "incident")
	entries, err := os.ReadDir(typeDir)
	if err != nil {
		t.Fatalf("failed to read type dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected artefact file to exist in type dir")
	}
}

func TestWorkflowNew_InvalidType(t *testing.T) {
	workflowHome := t.TempDir()
	repoPath := t.TempDir()

	handler := handleWorkflowNew(workflowHome, repoPath)
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"type":    "invalid-type",
		"context": "test",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for invalid type")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "invalid type") {
		t.Errorf("expected 'invalid type' in error, got: %s", text)
	}
}

func TestWorkflowNew_MissingType(t *testing.T) {
	workflowHome := t.TempDir()
	repoPath := t.TempDir()

	handler := handleWorkflowNew(workflowHome, repoPath)
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"context": "test",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for missing type")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "type is required") {
		t.Errorf("expected 'type is required', got: %s", text)
	}
}

func TestWorkflowNew_MissingContext(t *testing.T) {
	workflowHome := t.TempDir()
	repoPath := t.TempDir()

	handler := handleWorkflowNew(workflowHome, repoPath)
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"type": "incident",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for missing context")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "context is required") {
		t.Errorf("expected 'context is required', got: %s", text)
	}
}

func TestWorkflowIndex_EmptyWorkflowDir(t *testing.T) {
	repoPath := t.TempDir()

	// Create an empty docs/workflow/ directory
	workflowRoot := filepath.Join(repoPath, "docs", "workflow")
	if err := os.MkdirAll(workflowRoot, 0755); err != nil {
		t.Fatal(err)
	}

	handler := handleWorkflowIndex()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"repo_path": repoPath,
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)
	if !strings.Contains(text, "updated:") {
		t.Errorf("expected 'updated:' in output, got: %s", text)
	}

	// Master INDEX.md should be created
	masterIndex := filepath.Join(workflowRoot, "INDEX.md")
	if _, err := os.Stat(masterIndex); os.IsNotExist(err) {
		t.Error("expected INDEX.md to be created")
	}
}

func TestWorkflowIndex_WithTypeDir(t *testing.T) {
	repoPath := t.TempDir()

	// Create docs/workflow/incident/ with a sample artefact
	incidentDir := filepath.Join(repoPath, "docs", "workflow", "incident")
	if err := os.MkdirAll(incidentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incidentDir, "001-test-2026-02-23.md"), []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := handleWorkflowIndex()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"repo_path": repoPath,
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)
	if !strings.Contains(text, "incident/INDEX.md") {
		t.Errorf("expected 'incident/INDEX.md' in updated list, got: %s", text)
	}
}

func TestWorkflowIndex_NoDocsDir(t *testing.T) {
	repoPath := t.TempDir()

	handler := handleWorkflowIndex()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"repo_path": repoPath,
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for missing docs/workflow/")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", text)
	}
}

func TestWorkflowIndex_MissingRepoPath(t *testing.T) {
	handler := handleWorkflowIndex()
	res, err := handler(context.Background(), callToolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for missing repo_path")
	}
}

func TestWorkflowList_EmptyRepo(t *testing.T) {
	repoPath := t.TempDir()

	// Create minimal workflow structure
	if err := os.MkdirAll(filepath.Join(repoPath, "docs", "workflow"), 0755); err != nil {
		t.Fatal(err)
	}

	handler := handleWorkflowList()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"repo_path": repoPath,
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)
	// Empty repo should return null or []
	if text != "null" && text != "[]" {
		t.Errorf("expected null or [] for empty repo, got: %s", text)
	}
}

func TestWorkflowList_WithArtefacts(t *testing.T) {
	repoPath := t.TempDir()

	// Create incident artefact
	incidentDir := filepath.Join(repoPath, "docs", "workflow", "incident")
	if err := os.MkdirAll(incidentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incidentDir, "001-test-2026-02-23.md"), []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := handleWorkflowList()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"repo_path": repoPath,
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}

	text := resultText(t, res)

	var results []map[string]any
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("expected valid JSON array, got: %s", text)
	}

	if len(results) == 0 {
		t.Error("expected at least one artefact entry")
	}

	found := false
	for _, r := range results {
		if r["type"] == "incident" {
			found = true
		}
	}
	if !found {
		t.Error("expected incident type in results")
	}
}

func TestWorkflowList_InvalidType(t *testing.T) {
	repoPath := t.TempDir()

	handler := handleWorkflowList()
	res, err := handler(context.Background(), callToolReq(map[string]any{
		"repo_path": repoPath,
		"type":      "invalid-type",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if !res.IsError {
		t.Error("expected IsError=true for invalid type filter")
	}
}
