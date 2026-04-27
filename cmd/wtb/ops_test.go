package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpsCmd_SubcommandTree verifies the wtb ops subcommand tree is wired correctly.
func TestOpsCmd_SubcommandTree(t *testing.T) {
	cmd := newOpsCmd()

	wantSubs := []string{"probe", "db-health", "k8s-status", "kafka-status", "logs-analyze", "plan"}
	found := map[string]bool{}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	for _, name := range wantSubs {
		if !found[name] {
			t.Errorf("wtb ops missing subcommand %q", name)
		}
	}
}

// TestOpsPlanCmd_SubcommandTree verifies plan sub-subcommands.
func TestOpsPlanCmd_SubcommandTree(t *testing.T) {
	opsCmd := newOpsCmd()

	var planCmd interface{ Commands() []*interface{} }
	_ = planCmd

	wantPlanSubs := []string{"new", "show", "execute"}
	for _, sub := range opsCmd.Commands() {
		if sub.Name() != "plan" {
			continue
		}
		found := map[string]bool{}
		for _, planSub := range sub.Commands() {
			found[planSub.Name()] = true
		}
		for _, name := range wantPlanSubs {
			if !found[name] {
				t.Errorf("wtb ops plan missing subcommand %q", name)
			}
		}
		return
	}
	t.Error("wtb ops missing 'plan' subcommand")
}

// TestOpsPlanNewCmd_NoScenarioAndNoTemplate_ReturnsError verifies validation.
func TestOpsPlanNewCmd_NoScenarioAndNoTemplate_ReturnsError(t *testing.T) {
	cmd := newOpsPlanNewCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when neither --scenario nor --template is provided")
	}
}

// TestOpsPlanShowCmd_NoPlanFile_ReturnsError verifies file-not-found handling.
func TestOpsPlanShowCmd_NoPlanFile_ReturnsError(t *testing.T) {
	cmd := newOpsPlanShowCmd()
	cmd.SetArgs([]string{"--plan", "/nonexistent/plan.md"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when plan file does not exist")
	}
}

// TestOpsPlanExecuteCmd_NoPlanFile_ReturnsError verifies file-not-found handling.
func TestOpsPlanExecuteCmd_NoPlanFile_ReturnsError(t *testing.T) {
	cmd := newOpsPlanExecuteCmd()
	cmd.SetArgs([]string{"--plan", "/nonexistent/plan.md"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when plan file does not exist")
	}
}

// TestOpsPlanExecuteCmd_DryRun_ValidPlan verifies dry-run executes without error.
func TestOpsPlanExecuteCmd_DryRun_ValidPlan(t *testing.T) {
	// Create a minimal valid plan YAML in a temp file.
	planYAML := `plan:
  id: test-plan
  scenario: test
  risk: low
  created_at: "2026-02-23T00:00:00Z"
  steps:
    - id: 1
      tool: "echo hello"
      purpose: test step
      owner: auto
`
	tmp := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(tmp, []byte(planYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newOpsPlanExecuteCmd()
	cmd.SetArgs([]string{"--plan", tmp, "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error on dry-run with valid plan: %v", err)
	}
}
