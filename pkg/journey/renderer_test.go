package journey

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- CLI Rendering ---

func TestRenderer_CLI_RenderStep_ActiveJourney(t *testing.T) {
	r := NewRenderer(ModeCLI)
	def := miniJourney()
	e := newTestEngine(t)
	state, next, _ := e.Start("spec-builder")

	output := r.RenderStep(e.definitions["spec-builder"], state, next)

	expectedContains := []string{
		"Build a Specification",     // Title
		"Phase 1/6",                 // Phase progress
		"Project Context",           // Phase title
		"What does the system do?",  // Question prompt
		"Required",                  // Required indicator
	}

	for _, expected := range expectedContains {
		if !strings.Contains(output, expected) {
			t.Errorf("CLI output missing %q.\nOutput:\n%s", expected, output)
		}
	}

	// Should NOT contain completion markers
	if strings.Contains(output, "Complete ✅") {
		t.Error("active step should not show completion")
	}

	_ = def // avoid unused
}

func TestRenderer_CLI_RenderStep_SelectQuestion(t *testing.T) {
	r := NewRenderer(ModeCLI)
	e := newTestEngine(t)
	state, _, _ := e.Start("spec-builder")

	// Advance to scale phase (question with select type)
	state, _, _ = e.Next(state.ID, "PM tool")
	state, _, _ = e.Next(state.ID, "Improve productivity")
	state, next, _ := e.Next(state.ID, "Product managers")

	output := r.RenderStep(e.definitions["spec-builder"], state, next)

	// Should show numbered options
	if !strings.Contains(output, "[1]") {
		t.Error("select question should show numbered options")
	}
	if !strings.Contains(output, "< 100") {
		t.Errorf("should contain first option.\nOutput:\n%s", output)
	}
}

func TestRenderer_CLI_RenderStep_WhyAsk(t *testing.T) {
	r := NewRenderer(ModeCLI)
	e := newTestEngine(t)
	state, next, _ := e.Start("spec-builder")

	output := r.RenderStep(e.definitions["spec-builder"], state, next)

	// Should show Socratic context (WhyAsk)
	if !strings.Contains(output, "💡") {
		t.Error("CLI output should show WhyAsk with 💡 icon")
	}
	if !strings.Contains(output, "Understanding the core purpose") {
		t.Errorf("missing WhyAsk content.\nOutput:\n%s", output)
	}
}

func TestRenderer_CLI_RenderStep_CompletedJourney(t *testing.T) {
	r := NewRenderer(ModeCLI)
	e := newTestEngine(t)

	e.Register(&JourneyDefinition{
		Name:  "mini",
		Title: "Mini Journey",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase 1", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Q?", Type: "text", Required: true, WhyAsk: "Because"},
				},
			},
		},
	})

	state, _, _ := e.Start("mini")
	state, next, _ := e.Next(state.ID, "Done")

	output := r.RenderStep(e.definitions["mini"], state, next)

	expectedContains := []string{
		"Complete ✅",
		"1 responses",
		"1 phases",
	}

	for _, expected := range expectedContains {
		if !strings.Contains(output, expected) {
			t.Errorf("CLI complete output missing %q.\nOutput:\n%s", expected, output)
		}
	}
}

func TestRenderer_CLI_RenderList(t *testing.T) {
	r := NewRenderer(ModeCLI)
	e := newTestEngine(t)

	output := r.RenderList(e.ListJourneys())

	if !strings.Contains(output, "Available Journeys") {
		t.Error("list should contain header")
	}

	// Should contain all 4 journey names
	expectedNames := []string{"spec-builder", "backlog-refiner", "tech-advisor", "deep-dive-explorer"}
	for _, name := range expectedNames {
		if !strings.Contains(output, name) {
			t.Errorf("list missing journey %q.\nOutput:\n%s", name, output)
		}
	}

	// Should show phase and question counts
	if !strings.Contains(output, "Phases:") {
		t.Error("list should show phase counts")
	}
	if !strings.Contains(output, "Questions:") {
		t.Error("list should show question counts")
	}
}

func TestRenderer_CLI_ProgressBar(t *testing.T) {
	r := NewRenderer(ModeCLI)

	tests := []struct {
		current, total int
		expectContains string
	}{
		{1, 4, "25%"},
		{2, 4, "50%"},
		{3, 4, "75%"},
		{4, 4, "100%"},
		{1, 6, "16%"},
	}

	for _, tt := range tests {
		bar := r.progressBar(tt.current, tt.total)
		if !strings.Contains(bar, tt.expectContains) {
			t.Errorf("progressBar(%d, %d): expected %q, got %q", tt.current, tt.total, tt.expectContains, bar)
		}
		if !strings.Contains(bar, "█") {
			t.Errorf("progressBar should contain filled blocks: %q", bar)
		}
	}
}

func TestRenderer_CLI_ProgressBar_EdgeCases(t *testing.T) {
	r := NewRenderer(ModeCLI)

	// Zero total
	bar := r.progressBar(0, 0)
	if bar != "" {
		t.Errorf("expected empty string for zero total, got %q", bar)
	}
}

// --- JSON/MCP Rendering ---

func TestRenderer_MCP_RenderStep_ActiveJourney(t *testing.T) {
	r := NewRenderer(ModeMCP)
	e := newTestEngine(t)
	state, next, _ := e.Start("spec-builder")

	output := r.RenderStep(e.definitions["spec-builder"], state, next)

	var result MCPStepOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput:\n%s", err, output)
	}

	if result.SessionID != state.ID {
		t.Errorf("session_id: expected %q, got %q", state.ID, result.SessionID)
	}
	if result.JourneyName != "spec-builder" {
		t.Errorf("journey_name: expected 'spec-builder', got %q", result.JourneyName)
	}
	if result.Status != "active" {
		t.Errorf("status: expected 'active', got %q", result.Status)
	}
	if result.NextAction != "answer" {
		t.Errorf("next_action: expected 'answer', got %q", result.NextAction)
	}

	// Progress
	if result.Progress.Phase != 1 {
		t.Errorf("progress.phase: expected 1, got %d", result.Progress.Phase)
	}
	if result.Progress.TotalPhases != 6 {
		t.Errorf("progress.total_phases: expected 6, got %d", result.Progress.TotalPhases)
	}
	if result.Progress.PhaseTitle != "Project Context" {
		t.Errorf("progress.phase_title: expected 'Project Context', got %q", result.Progress.PhaseTitle)
	}

	// Question
	if result.Question == nil {
		t.Fatal("expected question, got nil")
	}
	if result.Question.ID != "what" {
		t.Errorf("question.id: expected 'what', got %q", result.Question.ID)
	}
	if result.Question.Type != "multiline" {
		t.Errorf("question.type: expected 'multiline', got %q", result.Question.Type)
	}
	if result.Question.Required != true {
		t.Error("question.required: expected true")
	}
}

func TestRenderer_MCP_RenderStep_CompletedJourney(t *testing.T) {
	r := NewRenderer(ModeMCP)
	e := newTestEngine(t)

	e.Register(&JourneyDefinition{
		Name:  "mini",
		Title: "Mini",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase 1", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Q?", Type: "text", Required: true, WhyAsk: "Because"},
				},
			},
		},
	})

	state, _, _ := e.Start("mini")
	state, next, _ := e.Next(state.ID, "Done")

	output := r.RenderStep(e.definitions["mini"], state, next)

	var result MCPStepOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("status: expected 'completed', got %q", result.Status)
	}
	if result.NextAction != "done" {
		t.Errorf("next_action: expected 'done', got %q", result.NextAction)
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if result.Progress.Percent != 100 {
		t.Errorf("progress.percent: expected 100, got %d", result.Progress.Percent)
	}
	if result.Question != nil {
		t.Error("completed journey should not have a question")
	}
}

func TestRenderer_MCP_RenderStep_ContextIncluded(t *testing.T) {
	r := NewRenderer(ModeMCP)
	e := newTestEngine(t)

	state, _, _ := e.Start("spec-builder")
	state, next, _ := e.Next(state.ID, "A PM tool")

	output := r.RenderStep(e.definitions["spec-builder"], state, next)

	var result MCPStepOutput
	json.Unmarshal([]byte(output), &result)

	if result.Context["what"] != "A PM tool" {
		t.Errorf("context should include previous answers: %v", result.Context)
	}
}

func TestRenderer_MCP_RenderStep_SelectOptions(t *testing.T) {
	r := NewRenderer(ModeMCP)
	e := newTestEngine(t)

	state, _, _ := e.Start("spec-builder")
	// Advance to scale phase
	state, _, _ = e.Next(state.ID, "Tool")
	state, _, _ = e.Next(state.ID, "Need")
	state, next, _ := e.Next(state.ID, "Users")

	output := r.RenderStep(e.definitions["spec-builder"], state, next)

	var result MCPStepOutput
	json.Unmarshal([]byte(output), &result)

	if result.Question == nil {
		t.Fatal("expected question")
	}
	if len(result.Question.Options) == 0 {
		t.Error("select question should have options in MCP output")
	}
}

func TestRenderer_MCP_RenderList(t *testing.T) {
	r := NewRenderer(ModeMCP)
	e := newTestEngine(t)

	output := r.RenderList(e.ListJourneys())

	var result MCPJourneyListOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput:\n%s", err, output)
	}

	if result.Count != 4 {
		t.Errorf("count: expected 4, got %d", result.Count)
	}
	if len(result.Journeys) != 4 {
		t.Errorf("journeys length: expected 4, got %d", len(result.Journeys))
	}

	// Check structure of each journey
	for _, j := range result.Journeys {
		if j.Name == "" {
			t.Error("journey name should not be empty")
		}
		if j.Title == "" {
			t.Error("journey title should not be empty")
		}
		if j.PhaseCount == 0 {
			t.Errorf("journey %s: phase_count should be > 0", j.Name)
		}
		if j.QuestionCount == 0 {
			t.Errorf("journey %s: question_count should be > 0", j.Name)
		}
		if len(j.PhaseNames) != j.PhaseCount {
			t.Errorf("journey %s: phase_names count (%d) != phase_count (%d)", j.Name, len(j.PhaseNames), j.PhaseCount)
		}
	}
}

// --- API Mode (same as MCP) ---

func TestRenderer_API_SameAsMCP(t *testing.T) {
	mcp := NewRenderer(ModeMCP)
	api := NewRenderer(ModeAPI)
	e := newTestEngine(t)
	state, next, _ := e.Start("spec-builder")

	mcpOutput := mcp.RenderStep(e.definitions["spec-builder"], state, next)
	apiOutput := api.RenderStep(e.definitions["spec-builder"], state, next)

	if mcpOutput != apiOutput {
		t.Error("MCP and API modes should produce identical output")
	}
}

// --- Helpers ---

func miniJourney() *JourneyDefinition {
	return &JourneyDefinition{
		Name:  "mini",
		Title: "Mini",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase 1", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Q?", Type: "text", Required: true, WhyAsk: "Because"},
				},
			},
		},
	}
}
