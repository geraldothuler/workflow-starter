package ops

import (
	"strings"
	"testing"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// planID
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestPlanID_Format(t *testing.T) {
	id := planID()
	if !strings.HasPrefix(id, "plan-") {
		t.Errorf("planID should start with 'plan-', got %q", id)
	}
	// format: plan-YYYYMMDD-HHMMSS → 20 chars total
	if len(id) != 20 {
		t.Errorf("expected 20 chars (plan-YYYYMMDD-HHMMSS), got len=%d: %q", len(id), id)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ListTemplates
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestListTemplates_Returns8(t *testing.T) {
	templates := ListTemplates()
	if len(templates) != 10 {
		t.Errorf("expected 10 templates, got %d", len(templates))
	}
}

// TestListTemplates_YAMLTemplatesLoaded verifies that YAML-driven templates
// are loaded from config/plans/*.yml via embed.FS at init time.
func TestListTemplates_YAMLTemplatesLoaded(t *testing.T) {
	yamlIDs := []string{"deploy-forward", "fleet-migration-step"}
	templates := ListTemplates()
	byID := map[string]bool{}
	for _, tmpl := range templates {
		byID[tmpl.ID] = true
	}
	for _, id := range yamlIDs {
		if !byID[id] {
			t.Errorf("YAML template %q not loaded from config/plans/", id)
		}
	}
}

func TestListTemplates_KnownIDs(t *testing.T) {
	expectedIDs := []string{"health-check", "lock-contention", "kafka-recovery", "deploy-rollback", "deploy-forward", "fleet-migration-step"}
	templates := ListTemplates()
	byID := map[string]bool{}
	for _, tmpl := range templates {
		byID[tmpl.ID] = true
	}
	for _, id := range expectedIDs {
		if !byID[id] {
			t.Errorf("missing expected template ID: %q", id)
		}
	}
}

func TestListTemplates_RiskLevels(t *testing.T) {
	templates := ListTemplates()
	for _, tmpl := range templates {
		if tmpl.Risk != "low" && tmpl.Risk != "medium" && tmpl.Risk != "high" {
			t.Errorf("template %q has invalid risk %q", tmpl.ID, tmpl.Risk)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// NewPlanFromTemplate
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestNewPlanFromTemplate_HealthCheck(t *testing.T) {
	vars := map[string]string{
		"kubectl-context":     "cobli-prod",
		"namespace":           "fusca",
		"db-host":             "mydb.internal",
		"db-user":             "app",
		"ssm-path":            "/prod/db/password",
		"deployment":          "fusca-api-app",
		"consumer-deployment": "fusca-cdc",
		"kafka-topic":         "orders",
	}
	plan, ok := NewPlanFromTemplate("health-check", "test context", vars)
	if !ok {
		t.Fatal("expected template to be found")
	}
	b := plan.Plan
	if b.Risk != "low" {
		t.Errorf("expected low risk, got %q", b.Risk)
	}
	if b.ID == "" {
		t.Error("expected non-empty plan ID")
	}
	if b.CreatedAt == "" {
		t.Error("expected non-empty CreatedAt")
	}
	if b.Context != "test context" {
		t.Errorf("expected context 'test context', got %q", b.Context)
	}
	if len(b.Steps) == 0 {
		t.Error("expected non-empty steps")
	}
}

func TestNewPlanFromTemplate_LockContention(t *testing.T) {
	vars := map[string]string{
		"kubectl-context": "cobli-prod",
		"namespace":       "fusca",
		"db-host":         "mydb.internal",
		"db-user":         "app",
		"ssm-path":        "/prod/db/password",
		"deployment":      "fusca-api-app",
	}
	plan, ok := NewPlanFromTemplate("lock-contention", "", vars)
	if !ok {
		t.Fatal("expected template to be found")
	}
	b := plan.Plan
	if b.Risk != "high" {
		t.Errorf("expected high risk, got %q", b.Risk)
	}
	if len(b.ApprovalRequired) == 0 {
		t.Error("expected lock-contention to have approval_required steps")
	}
	// Step 5 requires human approval
	found := false
	for _, id := range b.ApprovalRequired {
		if id == 5 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected step 5 in approval_required, got %v", b.ApprovalRequired)
	}
}

func TestNewPlanFromTemplate_KafkaRecovery(t *testing.T) {
	vars := map[string]string{
		"kubectl-context":     "cobli-prod",
		"namespace":           "fusca",
		"consumer-deployment": "fusca-cdc",
		"kafka-topic":         "orders",
		"deployment":          "fusca-cdc",
	}
	plan, ok := NewPlanFromTemplate("kafka-recovery", "", vars)
	if !ok {
		t.Fatal("expected template to be found")
	}
	if plan.Plan.Risk != "high" {
		t.Errorf("expected high risk, got %q", plan.Plan.Risk)
	}
}

func TestNewPlanFromTemplate_DeployRollback(t *testing.T) {
	vars := map[string]string{
		"kubectl-context": "cobli-prod",
		"namespace":       "fusca",
		"deployment":      "fusca-api-app",
	}
	plan, ok := NewPlanFromTemplate("deploy-rollback", "", vars)
	if !ok {
		t.Fatal("expected template to be found")
	}
	// Step 3 (rollback) should require human approval
	found := false
	for _, id := range plan.Plan.ApprovalRequired {
		if id == 3 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected step 3 in approval_required, got %v", plan.Plan.ApprovalRequired)
	}
}

func TestNewPlanFromTemplate_NotFound(t *testing.T) {
	_, ok := NewPlanFromTemplate("nonexistent-template", "", nil)
	if ok {
		t.Error("expected false for nonexistent template")
	}
}

// TestNewPlanFromTemplate_DeployForward verifies the YAML-loaded deploy-forward template.
func TestNewPlanFromTemplate_DeployForward(t *testing.T) {
	vars := map[string]string{
		"kubectl-context": "cobli-prod",
		"namespace":       "organization",
		"deployment":      "webhook-sender",
	}
	plan, ok := NewPlanFromTemplate("deploy-forward", "SS-2272 HMAC deploy", vars)
	if !ok {
		t.Fatal("expected YAML template deploy-forward to be loaded")
	}
	b := plan.Plan
	if b.Risk != "medium" {
		t.Errorf("expected medium risk, got %q", b.Risk)
	}
	if len(b.Steps) != 5 {
		t.Errorf("expected 5 steps, got %d", len(b.Steps))
	}
	approvalSet := map[int]bool{}
	for _, id := range b.ApprovalRequired {
		approvalSet[id] = true
	}
	for _, want := range []int{3, 5} {
		if !approvalSet[want] {
			t.Errorf("expected step %d in approval_required, got %v", want, b.ApprovalRequired)
		}
	}
	// Rollback must be present on human steps
	if b.Steps[2].Rollback == "" {
		t.Error("expected rollback on deploy step (3)")
	}
	if b.Steps[4].Rollback == "" {
		t.Error("expected rollback on monitoring step (5)")
	}
	// Variable interpolation: deployment name must appear in step 3 tool
	if !strings.Contains(b.Steps[2].Tool, "webhook-sender") {
		t.Errorf("expected deployment var interpolated in step 3 tool: %q", b.Steps[2].Tool)
	}
}

// TestNewPlanFromTemplate_FleetMigrationStep verifies the YAML-loaded fleet-migration-step template.
func TestNewPlanFromTemplate_FleetMigrationStep(t *testing.T) {
	vars := map[string]string{
		"kubectl-context":   "cobli-prod",
		"namespace":         "organization",
		"fleet-id":          "299b65a7-b275-455f-903c-9daefbe5fa67",
		"deployment":        "webhook-sender",
		"legacy-deployment": "alexstrasza-webhook",
	}
	plan, ok := NewPlanFromTemplate("fleet-migration-step", "SS-2276 fleet migration", vars)
	if !ok {
		t.Fatal("expected YAML template fleet-migration-step to be loaded")
	}
	b := plan.Plan
	if b.Risk != "medium" {
		t.Errorf("expected medium risk, got %q", b.Risk)
	}
	if len(b.Steps) != 6 {
		t.Errorf("expected 6 steps, got %d", len(b.Steps))
	}
	approvalSet := map[int]bool{}
	for _, id := range b.ApprovalRequired {
		approvalSet[id] = true
	}
	for _, want := range []int{3, 4, 6} {
		if !approvalSet[want] {
			t.Errorf("expected step %d in approval_required, got %v", want, b.ApprovalRequired)
		}
	}
	// Rollback must be present on all human gate steps
	for _, idx := range []int{2, 3, 5} {
		if b.Steps[idx].Rollback == "" {
			t.Errorf("expected rollback on step %d", idx+1)
		}
	}
	// Hyphen→underscore normalisation: fleet-id must resolve in step 3 tool
	if !strings.Contains(b.Steps[2].Tool, "299b65a7-b275-455f-903c-9daefbe5fa67") {
		t.Errorf("expected fleet-id interpolated in step 3 tool: %q", b.Steps[2].Tool)
	}
	if !strings.Contains(b.Steps[2].Tool, "alexstrasza-webhook") {
		t.Errorf("expected legacy-deployment in step 3 tool: %q", b.Steps[2].Tool)
	}
	if !strings.Contains(b.Steps[3].Tool, "webhook-sender") {
		t.Errorf("expected deployment in step 4 tool: %q", b.Steps[3].Tool)
	}
}

func TestNewPlanFromTemplate_StepsNotEmpty(t *testing.T) {
	for _, tmplID := range []string{"health-check", "lock-contention", "kafka-recovery", "deploy-rollback", "deploy-forward", "fleet-migration-step"} {
		t.Run(tmplID, func(t *testing.T) {
			plan, ok := NewPlanFromTemplate(tmplID, "", map[string]string{})
			if !ok {
				t.Fatal("template not found")
			}
			if len(plan.Plan.Steps) == 0 {
				t.Errorf("expected non-empty steps for %q", tmplID)
			}
			// First step should always be auto (auth check)
			if plan.Plan.Steps[0].Owner != "auto" {
				t.Errorf("expected first step owner=auto, got %q", plan.Plan.Steps[0].Owner)
			}
		})
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// NewBlankPlan
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestNewBlankPlan_Fields(t *testing.T) {
	plan := NewBlankPlan("My scenario", "some context", "medium")
	b := plan.Plan
	if b.Scenario != "My scenario" {
		t.Errorf("expected scenario 'My scenario', got %q", b.Scenario)
	}
	if b.Context != "some context" {
		t.Errorf("expected context 'some context', got %q", b.Context)
	}
	if b.Risk != "medium" {
		t.Errorf("expected risk 'medium', got %q", b.Risk)
	}
	if b.ID == "" {
		t.Error("expected non-empty ID")
	}
	if len(b.Steps) < 2 {
		t.Errorf("expected at least 2 placeholder steps, got %d", len(b.Steps))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// MarshalPlan / UnmarshalPlan roundtrip
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestMarshalUnmarshalPlan_Roundtrip(t *testing.T) {
	original := Plan{
		Plan: PlanBody{
			ID:        "plan-20260222-091000",
			Scenario:  "Test Scenario",
			Context:   "test context",
			Risk:      "high",
			CreatedAt: "2026-02-22T09:10:00Z",
			Steps: []PlanStep{
				{ID: 1, Tool: "wtb ops auth --profile cobli-tech", Purpose: "check auth", Owner: "auto"},
				{ID: 2, Tool: "# manual step here", Purpose: "do manually", Owner: "human", Rollback: "# no rollback"},
			},
			ApprovalRequired: []int{2},
		},
	}

	data, err := MarshalPlan(original)
	if err != nil {
		t.Fatalf("MarshalPlan error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty YAML output")
	}

	decoded, err := UnmarshalPlan(data)
	if err != nil {
		t.Fatalf("UnmarshalPlan error: %v", err)
	}

	b := decoded.Plan
	if b.ID != original.Plan.ID {
		t.Errorf("ID mismatch: got %q, want %q", b.ID, original.Plan.ID)
	}
	if b.Risk != original.Plan.Risk {
		t.Errorf("Risk mismatch: got %q, want %q", b.Risk, original.Plan.Risk)
	}
	if len(b.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(b.Steps))
	}
	if b.Steps[1].Rollback != "# no rollback" {
		t.Errorf("Rollback not preserved: %q", b.Steps[1].Rollback)
	}
	if len(b.ApprovalRequired) != 1 || b.ApprovalRequired[0] != 2 {
		t.Errorf("ApprovalRequired not preserved: %v", b.ApprovalRequired)
	}
}

func TestUnmarshalPlan_InvalidYAML(t *testing.T) {
	_, err := UnmarshalPlan([]byte("not: valid: yaml: ["))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SubstituteVars
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestSubstituteVars_ReplacesPlaceholders(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			Steps: []PlanStep{
				{ID: 1, Tool: "kubectl rollout undo deployment/<name> -n <namespace>", Owner: "human",
					Rollback: "kubectl rollout undo deployment/<name> -n <namespace> # undo"},
			},
		},
	}
	vars := map[string]string{
		"name":      "fusca-api-app",
		"namespace": "fusca",
	}
	result := SubstituteVars(plan, vars)
	step := result.Plan.Steps[0]
	if strings.Contains(step.Tool, "<name>") {
		t.Errorf("placeholder <name> not replaced in Tool: %q", step.Tool)
	}
	if strings.Contains(step.Tool, "<namespace>") {
		t.Errorf("placeholder <namespace> not replaced in Tool: %q", step.Tool)
	}
	if !strings.Contains(step.Tool, "fusca-api-app") {
		t.Errorf("expected fusca-api-app in Tool: %q", step.Tool)
	}
	if strings.Contains(step.Rollback, "<name>") {
		t.Errorf("placeholder <name> not replaced in Rollback: %q", step.Rollback)
	}
}

func TestSubstituteVars_NoPlaceholders(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			Steps: []PlanStep{
				{ID: 1, Tool: "wtb ops auth --profile cobli-tech", Owner: "auto"},
			},
		},
	}
	result := SubstituteVars(plan, map[string]string{"name": "something"})
	if result.Plan.Steps[0].Tool != "wtb ops auth --profile cobli-tech" {
		t.Errorf("tool should be unchanged: %q", result.Plan.Steps[0].Tool)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PlanSummary
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestPlanSummary_ContainsKeyFields(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:        "plan-20260222-091000",
			Scenario:  "Lock Contention Recovery",
			Context:   "16 sessions blocked",
			Risk:      "high",
			CreatedAt: "2026-02-22T09:10:00Z",
			Steps: []PlanStep{
				{ID: 1, Tool: "wtb ops auth", Purpose: "check auth", Owner: "auto"},
				{ID: 2, Tool: "# SELECT pg_terminate_backend(123)", Purpose: "kill blocker", Owner: "human", Rollback: "# irreversible"},
			},
			ApprovalRequired: []int{2},
		},
	}

	summary := PlanSummary(plan)

	for _, expected := range []string{
		"plan-20260222-091000",
		"Lock Contention Recovery",
		"16 sessions blocked",
		"high",
		"check auth",
		"kill blocker",
		"🔴", // high risk icon
		"🤖", // auto owner
		"👤", // human owner
	} {
		if !strings.Contains(summary, expected) {
			t.Errorf("expected %q in summary:\n%s", expected, summary)
		}
	}
}

func TestPlanSummary_RiskIcons(t *testing.T) {
	cases := []struct {
		risk string
		icon string
	}{
		{"low", "🟢"},
		{"medium", "🟡"},
		{"high", "🔴"},
		{"unknown", "⚪"},
	}
	for _, tc := range cases {
		t.Run(tc.risk, func(t *testing.T) {
			plan := Plan{Plan: PlanBody{ID: "x", Risk: tc.risk, Steps: []PlanStep{}}}
			summary := PlanSummary(plan)
			if !strings.Contains(summary, tc.icon) {
				t.Errorf("expected icon %q for risk %q in:\n%s", tc.icon, tc.risk, summary)
			}
		})
	}
}

func TestPlanSummary_ProceedIfAndAbortIf(t *testing.T) {
	plan := Plan{
		Plan: PlanBody{
			ID:   "x",
			Risk: "low",
			Steps: []PlanStep{
				{ID: 1, Tool: "wtb ops db health", Purpose: "check", Owner: "auto",
					ProceedIf: "data.locks > 0", AbortIf: "data.total_conn > 100"},
			},
		},
	}
	summary := PlanSummary(plan)
	if !strings.Contains(summary, "proceed_if") {
		t.Errorf("expected proceed_if in summary: %s", summary)
	}
	if !strings.Contains(summary, "abort_if") {
		t.Errorf("expected abort_if in summary: %s", summary)
	}
}
