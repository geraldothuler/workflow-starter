package ops

import (
	"bytes"
	"strings"
	"testing"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// firstLine / truncate
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestFirstLine_WithNewline(t *testing.T) {
	got := firstLine("first line\nsecond line\nthird")
	if got != "first line" {
		t.Errorf("expected 'first line', got %q", got)
	}
}

func TestFirstLine_NoNewline(t *testing.T) {
	got := firstLine("single line")
	if got != "single line" {
		t.Errorf("expected 'single line', got %q", got)
	}
}

func TestFirstLine_Empty(t *testing.T) {
	got := firstLine("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTruncate_BelowLimit(t *testing.T) {
	got := truncate("short", 100)
	if got != "short" {
		t.Errorf("expected unchanged short string, got %q", got)
	}
}

func TestTruncate_ExactLimit(t *testing.T) {
	s := strings.Repeat("a", 10)
	got := truncate(s, 10)
	if got != s {
		t.Errorf("expected exact string at limit, got %q", got)
	}
}

func TestTruncate_AboveLimit(t *testing.T) {
	s := strings.Repeat("a", 20)
	got := truncate(s, 10)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected '…' suffix, got %q", got)
	}
	// Content before ellipsis should be exactly 10 bytes
	content := strings.TrimSuffix(got, "…")
	if len(content) != 10 {
		t.Errorf("expected 10 bytes before ellipsis, got %d: %q", len(content), got)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// buildExecutionResult
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBuildExecutionResult_AllOK(t *testing.T) {
	b := PlanBody{ID: "plan-001", Scenario: "Test"}
	results := []StepResult{
		{StepID: 1, Status: "ok"},
		{StepID: 2, Status: "ok"},
	}
	r := buildExecutionResult(b, results, false, "")
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
	if r.Cost != "zero-llm" {
		t.Errorf("expected zero-llm cost")
	}
	if r.Data["ok"] != 2 {
		t.Errorf("expected ok=2, got %v", r.Data["ok"])
	}
	if r.Data["skipped"] != 0 {
		t.Errorf("expected skipped=0, got %v", r.Data["skipped"])
	}
}

func TestBuildExecutionResult_WithSkipped(t *testing.T) {
	b := PlanBody{ID: "plan-001", Scenario: "Test"}
	results := []StepResult{
		{StepID: 1, Status: "ok"},
		{StepID: 2, Status: "skipped", Skipped: true},
	}
	r := buildExecutionResult(b, results, false, "")
	if r.Status != "ok" {
		t.Errorf("expected ok when only skipped, got %q", r.Status)
	}
	if r.Data["skipped"] != 1 {
		t.Errorf("expected skipped=1, got %v", r.Data["skipped"])
	}
}

func TestBuildExecutionResult_Aborted(t *testing.T) {
	b := PlanBody{ID: "plan-001", Scenario: "Test"}
	results := []StepResult{
		{StepID: 1, Status: "ok"},
		{StepID: 2, Status: "error"},
	}
	r := buildExecutionResult(b, results, true, "step 2 failed: connection refused")
	if r.Status != "critical" {
		t.Errorf("expected critical for aborted plan, got %q", r.Status)
	}
	if !strings.Contains(r.Signal, "abortado") {
		t.Errorf("expected 'abortado' in signal: %q", r.Signal)
	}
}

func TestBuildExecutionResult_Warn(t *testing.T) {
	b := PlanBody{ID: "plan-001", Scenario: "Test"}
	results := []StepResult{
		{StepID: 1, Status: "ok"},
		{StepID: 2, Status: "error"}, // not aborted, just failed
	}
	r := buildExecutionResult(b, results, false, "")
	if r.Status != "warn" {
		t.Errorf("expected warn for failed-but-not-aborted plan, got %q", r.Status)
	}
	if !strings.Contains(r.Signal, "falha") {
		t.Errorf("expected 'falha' in signal: %q", r.Signal)
	}
}

func TestBuildExecutionResult_DataFields(t *testing.T) {
	b := PlanBody{ID: "plan-20260222-091000", Scenario: "Health Check"}
	results := []StepResult{
		{StepID: 1, Status: "ok"},
		{StepID: 2, Status: "warn"},
		{StepID: 3, Status: "skipped", Skipped: true},
	}
	r := buildExecutionResult(b, results, false, "")
	if r.Data["plan_id"] != "plan-20260222-091000" {
		t.Errorf("expected plan_id in data, got %v", r.Data["plan_id"])
	}
	if r.Data["scenario"] != "Health Check" {
		t.Errorf("expected scenario in data, got %v", r.Data["scenario"])
	}
	if r.Data["total"] != 3 {
		t.Errorf("expected total=3, got %v", r.Data["total"])
	}
	if r.Data["ok"] != 2 { // ok + warn both count as ok
		t.Errorf("expected ok=2 (ok+warn), got %v", r.Data["ok"])
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// executeAutoStep
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExecuteAutoStep_CommentSkipped(t *testing.T) {
	step := PlanStep{ID: 1, Tool: "# SELECT pg_terminate_backend(123)", Owner: "auto"}
	sr := executeAutoStep(step, "wtb")
	if sr.Status != "skipped" {
		t.Errorf("expected skipped for comment step, got %q", sr.Status)
	}
	if !sr.Skipped {
		t.Error("expected Skipped=true")
	}
}

func TestExecuteAutoStep_CommentWithLeadingSpace(t *testing.T) {
	step := PlanStep{ID: 1, Tool: "  # SELECT something", Owner: "auto"}
	sr := executeAutoStep(step, "wtb")
	if sr.Status != "skipped" {
		t.Errorf("expected skipped for comment with leading spaces, got %q", sr.Status)
	}
}

func TestExecuteAutoStep_EmptyToolError(t *testing.T) {
	step := PlanStep{ID: 1, Tool: "   ", Owner: "auto"}
	sr := executeAutoStep(step, "wtb")
	if sr.Status != "error" {
		t.Errorf("expected error for empty tool, got %q", sr.Status)
	}
}

func TestExecuteAutoStep_NonWtbCommand(t *testing.T) {
	// "echo ok" should succeed via sh -c
	step := PlanStep{ID: 1, Tool: "echo ok", Owner: "auto"}
	sr := executeAutoStep(step, "wtb")
	if sr.Status != "ok" {
		t.Errorf("expected ok for 'echo ok', got %q (signal: %s)", sr.Status, sr.Signal)
	}
	if !strings.Contains(sr.Signal, "ok") {
		t.Errorf("expected 'ok' in signal, got %q", sr.Signal)
	}
}

func TestExecuteAutoStep_FailingCommand(t *testing.T) {
	step := PlanStep{ID: 1, Tool: "false", Owner: "auto"}
	sr := executeAutoStep(step, "wtb")
	if sr.Status != "error" {
		t.Errorf("expected error for failing command, got %q", sr.Status)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ExecutePlan — dry-run
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExecutePlan_DryRun_AllSkipped(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:       "plan-test-001",
			Scenario: "Dry Run Test",
			Risk:     "low",
			Steps: []PlanStep{
				{ID: 1, Tool: "wtb ops auth --profile cobli-tech", Purpose: "check auth", Owner: "auto"},
				{ID: 2, Tool: "# manual step", Purpose: "manual", Owner: "human"},
			},
		},
	}

	var out bytes.Buffer
	cfg := PlanExecuteConfig{
		DryRun: true,
		Stdout: &out,
		Stdin:  strings.NewReader(""),
	}

	r := ExecutePlan(plan, cfg)

	if r.Status != "ok" {
		t.Errorf("expected ok for dry-run, got %q (signal: %s)", r.Status, r.Signal)
	}

	steps, ok := r.Data["steps"].([]StepResult)
	if !ok {
		t.Fatalf("expected steps in data")
	}
	for _, sr := range steps {
		if sr.Status != "skipped" || !sr.Skipped {
			t.Errorf("expected all steps skipped in dry-run, got status=%q skipped=%v for step %d",
				sr.Status, sr.Skipped, sr.StepID)
		}
		if sr.Signal != "dry-run" {
			t.Errorf("expected signal='dry-run', got %q", sr.Signal)
		}
	}

	output := out.String()
	if !strings.Contains(output, "plan-test-001") {
		t.Errorf("expected plan ID in output: %q", output)
	}
	if !strings.Contains(output, "dry-run") {
		t.Errorf("expected 'dry-run' in output: %q", output)
	}
}

func TestExecutePlan_DryRun_PrintsAllSteps(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:   "plan-test-002",
			Risk: "high",
			Steps: []PlanStep{
				{ID: 1, Tool: "wtb ops auth", Purpose: "auth check", Owner: "auto"},
				{ID: 2, Tool: "wtb ops db health ...", Purpose: "db check", Owner: "auto"},
				{ID: 3, Tool: "# kill root blocker", Purpose: "manual kill", Owner: "human"},
			},
		},
	}

	var out bytes.Buffer
	cfg := PlanExecuteConfig{DryRun: true, Stdout: &out, Stdin: strings.NewReader("")}
	r := ExecutePlan(plan, cfg)

	output := out.String()
	for i := 1; i <= 3; i++ {
		if !strings.Contains(output, "Step") {
			t.Errorf("expected step information in output")
		}
	}

	if r.Data["total"] != 3 {
		t.Errorf("expected total=3, got %v", r.Data["total"])
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ExecutePlan — auto steps execution
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExecutePlan_AutoSteps_SuccessfulCommand(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:   "plan-auto-001",
			Risk: "low",
			Steps: []PlanStep{
				{ID: 1, Tool: "echo hello-wtb", Purpose: "echo test", Owner: "auto"},
			},
		},
	}

	var out bytes.Buffer
	cfg := PlanExecuteConfig{
		DryRun:     false,
		BinaryPath: "echo",
		Stdout:     &out,
		Stdin:      strings.NewReader(""),
	}

	r := ExecutePlan(plan, cfg)
	if r.Status != "ok" {
		t.Errorf("expected ok for successful echo, got %q (signal: %s)", r.Status, r.Signal)
	}

	steps, _ := r.Data["steps"].([]StepResult)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(steps))
	}
	if steps[0].Status != "ok" {
		t.Errorf("expected step ok, got %q", steps[0].Status)
	}
	if steps[0].Duration == "" {
		t.Error("expected non-empty duration")
	}
	if steps[0].StartedAt == "" {
		t.Error("expected non-empty started_at")
	}
}

func TestExecutePlan_AutoSteps_FailureAbortsPlan(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:   "plan-abort-001",
			Risk: "high",
			Steps: []PlanStep{
				{ID: 1, Tool: "false", Purpose: "will fail", Owner: "auto"},
				{ID: 2, Tool: "echo should-not-run", Purpose: "should be skipped", Owner: "auto"},
			},
		},
	}

	var out bytes.Buffer
	cfg := PlanExecuteConfig{
		DryRun: false,
		Stdout: &out,
		Stdin:  strings.NewReader(""),
	}

	r := ExecutePlan(plan, cfg)
	if r.Status != "critical" {
		t.Errorf("expected critical for aborted plan, got %q", r.Status)
	}

	steps, _ := r.Data["steps"].([]StepResult)
	if len(steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(steps))
	}
	if steps[0].Status != "error" {
		t.Errorf("expected step 1 error, got %q", steps[0].Status)
	}
	if steps[1].Status != "skipped" {
		t.Errorf("expected step 2 skipped (aborted), got %q", steps[1].Status)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ExecutePlan — human steps
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExecutePlan_HumanStep_EnterConfirms(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:   "plan-human-001",
			Risk: "high",
			Steps: []PlanStep{
				{ID: 1, Tool: "# manual command here", Purpose: "do manually", Owner: "human"},
			},
		},
	}

	var out bytes.Buffer
	// Simulates pressing Enter (empty line)
	cfg := PlanExecuteConfig{
		DryRun: false,
		Stdout: &out,
		Stdin:  strings.NewReader("\n"),
	}

	r := ExecutePlan(plan, cfg)
	steps, _ := r.Data["steps"].([]StepResult)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != "ok" {
		t.Errorf("expected ok when operator confirms with Enter, got %q", steps[0].Status)
	}
	if steps[0].Owner != "human" {
		t.Errorf("expected owner=human, got %q", steps[0].Owner)
	}
}

func TestExecutePlan_HumanStep_SkipInput(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:   "plan-human-002",
			Risk: "high",
			Steps: []PlanStep{
				{ID: 1, Tool: "# manual command here", Purpose: "do manually", Owner: "human"},
			},
		},
	}

	var out bytes.Buffer
	// Simulates typing "s" to skip
	cfg := PlanExecuteConfig{
		DryRun: false,
		Stdout: &out,
		Stdin:  strings.NewReader("s\n"),
	}

	r := ExecutePlan(plan, cfg)
	steps, _ := r.Data["steps"].([]StepResult)
	if steps[0].Status != "skipped" {
		t.Errorf("expected skipped when operator types 's', got %q", steps[0].Status)
	}
	if !steps[0].Skipped {
		t.Error("expected Skipped=true")
	}
}

func TestExecutePlan_HumanStep_ShowsRollback(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:   "plan-human-003",
			Risk: "high",
			Steps: []PlanStep{
				{ID: 1, Tool: "# kill process", Purpose: "kill",
					Owner: "human", Rollback: "# cannot undo"},
			},
		},
	}

	var out bytes.Buffer
	cfg := PlanExecuteConfig{
		DryRun: false,
		Stdout: &out,
		Stdin:  strings.NewReader("\n"),
	}

	ExecutePlan(plan, cfg)
	output := out.String()
	if !strings.Contains(output, "cannot undo") {
		t.Errorf("expected rollback info in output: %q", output)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ExecutePlan — plan header output
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExecutePlan_PrintsHeader(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:       "plan-header-test",
			Scenario: "My Scenario",
			Context:  "some context",
			Risk:     "medium",
			Steps:    []PlanStep{},
		},
	}

	var out bytes.Buffer
	cfg := PlanExecuteConfig{DryRun: true, Stdout: &out, Stdin: strings.NewReader("")}
	ExecutePlan(plan, cfg)

	output := out.String()
	if !strings.Contains(output, "plan-header-test") {
		t.Errorf("expected plan ID in header: %q", output)
	}
	if !strings.Contains(output, "My Scenario") {
		t.Errorf("expected scenario in header: %q", output)
	}
	if !strings.Contains(output, "some context") {
		t.Errorf("expected context in header: %q", output)
	}
}
