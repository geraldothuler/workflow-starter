package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Fixtures ---

const testDefinitionYAML = `
id: test-pipeline
type: pipeline
name: Test Pipeline
version: "1.0"

inputs:
  - name: narrative
    description: Input narrative
    required: true
  - name: provider
    description: LLM provider
    required: false
    default: claude

steps:
  - name: step_one
    engine: test/engine-a
    description: First step
    output: .workflow/step1.json

  - name: step_optional
    engine: test/engine-b
    description: Optional step
    output: .workflow/step2.json
    optional: true

  - name: step_checkpoint
    engine: test/engine-c
    description: Step with checkpoint
    human_checkpoint: true

artefacts:
  - name: result
    format: result.json
    destination: docs/workflow/test/

chain:
  from: []
  to: [review]
`

const testDocumentaryYAML = `
id: test-doc
type: documentary
name: Test Documentary

inputs:
  - name: symptom
    required: true

artefacts:
  - name: savepoint
    format: savepoint.md
    destination: docs/workflow/test/
`

func writeFixture(t *testing.T, dir, useCaseID, content string) string {
	t.Helper()
	ucDir := filepath.Join(dir, "use-cases", useCaseID)
	if err := os.MkdirAll(ucDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ucDir, "definition.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// mockTerminal captures prompts and returns pre-set responses.
type mockTerminal struct {
	responses    []bool
	idx          int
	log          []string
	askResponses map[string]string
	canAsk       bool
}

func (m *mockTerminal) Confirm(prompt string) bool {
	m.log = append(m.log, "confirm: "+prompt)
	if m.idx >= len(m.responses) {
		return false
	}
	resp := m.responses[m.idx]
	m.idx++
	return resp
}

func (m *mockTerminal) Printf(format string, args ...any) {
	m.log = append(m.log, fmt.Sprintf(format, args...))
}

func (m *mockTerminal) Ask(prompt string, whyAsk string) string {
	m.log = append(m.log, "ask: "+prompt)
	if m.askResponses != nil {
		if val, ok := m.askResponses[prompt]; ok {
			return val
		}
	}
	return ""
}

func (m *mockTerminal) CanAsk() bool { return m.canAsk }

// testRegistry returns an engine registry with controllable stubs.
func testRegistry(executed *[]string) map[string]StepExecutor {
	mkExec := func(name string) StepExecutor {
		return func(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
			*executed = append(*executed, name+":"+step.Name)
			return StepResult{StepName: step.Name}
		}
	}
	return map[string]StepExecutor{
		"test/engine-a": mkExec("a"),
		"test/engine-b": mkExec("b"),
		"test/engine-c": mkExec("c"),
	}
}

// --- LoadDefinition ---

func TestLoadDefinition_Pipeline(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)

	def, err := LoadDefinition(home, "test-pipeline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ID != "test-pipeline" {
		t.Errorf("expected id=test-pipeline, got %q", def.ID)
	}
	if !def.IsPipeline() {
		t.Error("expected IsPipeline()=true")
	}
	if len(def.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(def.Steps))
	}
	if len(def.RequiredInputs()) != 1 || def.RequiredInputs()[0].Name != "narrative" {
		t.Errorf("expected 1 required input (narrative), got %v", def.RequiredInputs())
	}
}

func TestLoadDefinition_Documentary(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-doc", testDocumentaryYAML)

	def, err := LoadDefinition(home, "test-doc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.IsPipeline() {
		t.Error("documentary type should not be a pipeline")
	}
}

func TestLoadDefinition_NotFound(t *testing.T) {
	_, err := LoadDefinition(t.TempDir(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent use-case")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the use-case id: %v", err)
	}
}

func TestLoadDefinition_InvalidYAML(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "bad", ": invalid: [yaml")
	_, err := LoadDefinition(home, "bad")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadDefinition_MissingID(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "no-id", "type: pipeline\nname: Missing ID\n")
	_, err := LoadDefinition(home, "no-id")
	if err == nil {
		t.Error("expected error for definition without id")
	}
}

// --- ListUseCases ---

func TestListUseCases(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, home, "uc-a", testDefinitionYAML)
	writeFixture(t, home, "uc-b", testDefinitionYAML)
	// directory without definition.yml should be ignored
	if err := os.MkdirAll(filepath.Join(home, "use-cases", "no-def"), 0755); err != nil {
		t.Fatal(err)
	}

	ids, err := ListUseCases(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 use-cases, got %d: %v", len(ids), ids)
	}
}

// --- Runner.ValidateInputs ---

func TestRunner_ValidateInputs_Missing(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")
	r := New(def, testRegistry(new([]string)), RunOptions{DryRun: true})

	err := r.ValidateInputs(RunInputs{}) // narrative is required
	if err == nil {
		t.Error("expected validation error for missing required input")
	}
	if !strings.Contains(err.Error(), "narrative") {
		t.Errorf("error should mention missing field: %v", err)
	}
}

func TestRunner_ValidateInputs_OK(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")
	r := New(def, testRegistry(new([]string)), RunOptions{DryRun: true})

	if err := r.ValidateInputs(RunInputs{"narrative": "some text"}); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

// --- Runner.Run dry-run ---

func TestRunner_Run_DryRun(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	term := &mockTerminal{}
	r := New(def, testRegistry(new([]string)), RunOptions{DryRun: true}).WithTerminal(term)

	results, err := r.Run(RunInputs{"narrative": "hello"})
	if err != nil {
		t.Fatalf("unexpected error in dry-run: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 step results, got %d", len(results))
	}
	// step_optional should be skipped in dry-run
	if !results[1].Skipped {
		t.Error("optional step should be skipped in dry-run")
	}
}

// --- Runner.Run — documentary use-case error ---

func TestRunner_Run_Documentary_Error(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-doc", testDocumentaryYAML)
	def, _ := LoadDefinition(home, "test-doc")
	r := New(def, DefaultRegistry(), RunOptions{DryRun: true})

	_, err := r.Run(RunInputs{"symptom": "high latency"})
	if err == nil {
		t.Error("expected error for documentary use-case (no steps)")
	}
	if !strings.Contains(err.Error(), "wtb new") {
		t.Errorf("error should suggest 'wtb new': %v", err)
	}
}

// --- Runner.Run — optional step control ---

func TestRunner_Run_OptionalStep_AutoSkip(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	var executed []string
	r := New(def, testRegistry(&executed), RunOptions{AutoSkip: true}).
		WithTerminal(&mockTerminal{})

	_, err := r.Run(RunInputs{"narrative": "hello"})
	if err != nil {
		// step_checkpoint has no terminal response → aborts; that's fine for this test
		if !strings.Contains(err.Error(), "aborted") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	// optional step must not be in executed list
	for _, e := range executed {
		if strings.Contains(e, "step_optional") {
			t.Error("optional step should have been skipped")
		}
	}
}

func TestRunner_Run_OptionalStep_Confirmed(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	var executed []string
	// Confirm optional step (true), then confirm checkpoint (true)
	term := &mockTerminal{responses: []bool{true, true}}
	r := New(def, testRegistry(&executed), RunOptions{}).WithTerminal(term)

	_, err := r.Run(RunInputs{"narrative": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(executed) != 3 {
		t.Errorf("expected 3 steps executed, got %d: %v", len(executed), executed)
	}
}

// --- Runner.Run — human checkpoint abort ---

func TestRunner_Run_Checkpoint_Abort(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	var executed []string
	// Auto-skip optional, deny checkpoint
	term := &mockTerminal{responses: []bool{false}}
	r := New(def, testRegistry(&executed), RunOptions{AutoSkip: true}).WithTerminal(term)

	_, err := r.Run(RunInputs{"narrative": "hello"})
	if err == nil {
		t.Error("expected error when checkpoint is denied")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected abort error, got: %v", err)
	}
	// Only step_one should have executed
	if len(executed) != 1 || !strings.HasSuffix(executed[0], ":step_one") {
		t.Errorf("expected only step_one executed, got: %v", executed)
	}
}

// --- Runner.Run — missing engine ---

func TestRunner_Run_MissingEngine(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	// Empty registry — no engines registered
	r := New(def, map[string]StepExecutor{}, RunOptions{}).
		WithTerminal(&mockTerminal{responses: []bool{true, true}})

	_, err := r.Run(RunInputs{"narrative": "hello"})
	if err == nil {
		t.Error("expected error for unregistered engine")
	}
	if !strings.Contains(err.Error(), "no engine registered") {
		t.Errorf("expected 'no engine registered' error, got: %v", err)
	}
}

// --- RunInputs.Get with default ---

func TestRunInputs_Get_Default(t *testing.T) {
	spec := InputSpec{Name: "provider", Default: "claude"}
	inputs := RunInputs{}

	if got := inputs.Get(spec); got != "claude" {
		t.Errorf("expected default 'claude', got %q", got)
	}
}

func TestRunInputs_Get_Override(t *testing.T) {
	spec := InputSpec{Name: "provider", Default: "claude"}
	inputs := RunInputs{"provider": "gemini"}

	if got := inputs.Get(spec); got != "gemini" {
		t.Errorf("expected override 'gemini', got %q", got)
	}
}

// --- CollectInputs ---

func TestCollectInputs_FlagWins(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	cfg := &ResolveConfig{}
	cfg.Resolve.DefaultChain = []string{"ask"}
	cfg.Resolve.Strategies = map[string]StrategyConfig{}

	reg := DefaultResolverRegistry(cfg)
	term := &mockTerminal{canAsk: true}
	r := New(def, testRegistry(new([]string)), RunOptions{DryRun: true}).
		WithTerminal(term).WithResolvers(reg)

	// narrative provided via flag -> should not ask
	resolved, err := r.CollectInputs(RunInputs{"narrative": "from-flag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved["narrative"] != "from-flag" {
		t.Errorf("expected flag value, got %q", resolved["narrative"])
	}
}

func TestCollectInputs_FallsToAsk(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	cfg := &ResolveConfig{}
	cfg.Resolve.DefaultChain = []string{"ask"}
	cfg.Resolve.Strategies = map[string]StrategyConfig{}

	reg := DefaultResolverRegistry(cfg)
	term := &mockTerminal{
		canAsk: true,
		askResponses: map[string]string{
			"Input narrative": "from-ask",
		},
	}
	r := New(def, testRegistry(new([]string)), RunOptions{DryRun: true}).
		WithTerminal(term).WithResolvers(reg)

	resolved, err := r.CollectInputs(RunInputs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved["narrative"] != "from-ask" {
		t.Errorf("expected ask value, got %q", resolved["narrative"])
	}
}

func TestCollectInputs_NonInteractiveFails(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	cfg := &ResolveConfig{}
	cfg.Resolve.DefaultChain = []string{"ask"}
	cfg.Resolve.Strategies = map[string]StrategyConfig{}

	reg := DefaultResolverRegistry(cfg)
	term := &mockTerminal{canAsk: false}
	r := New(def, testRegistry(new([]string)), RunOptions{DryRun: true}).
		WithTerminal(term).WithResolvers(reg)

	_, err := r.CollectInputs(RunInputs{})
	if err == nil {
		t.Error("expected error when non-interactive and required input missing")
	}
}

func TestCollectInputs_BackwardCompat(t *testing.T) {
	home := writeFixture(t, t.TempDir(), "test-pipeline", testDefinitionYAML)
	def, _ := LoadDefinition(home, "test-pipeline")

	// No resolvers set -> Run should use ValidateInputs
	r := New(def, testRegistry(new([]string)), RunOptions{DryRun: true}).
		WithTerminal(&mockTerminal{})

	_, err := r.Run(RunInputs{}) // narrative missing
	if err == nil {
		t.Error("expected validation error without resolvers")
	}
}
