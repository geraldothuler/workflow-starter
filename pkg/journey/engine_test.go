package journey

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Helpers ---

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(t.TempDir())
}

func fixedTime() time.Time {
	return time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)
}

// --- NewEngine ---

func TestNewEngine_RegistersBuiltinJourneys(t *testing.T) {
	e := newTestEngine(t)

	journeys := e.ListJourneys()
	if len(journeys) != 4 {
		t.Errorf("expected 4 built-in journeys, got %d", len(journeys))
	}

	expectedNames := map[string]bool{
		"spec-builder":      false,
		"backlog-refiner":   false,
		"tech-advisor":      false,
		"deep-dive-explorer": false,
	}

	for _, j := range journeys {
		if _, ok := expectedNames[j.Name]; ok {
			expectedNames[j.Name] = true
		} else {
			t.Errorf("unexpected journey: %s", j.Name)
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing built-in journey: %s", name)
		}
	}
}

func TestNewEngine_GetDefinition(t *testing.T) {
	e := newTestEngine(t)

	def, ok := e.GetDefinition("spec-builder")
	if !ok {
		t.Fatal("expected to find spec-builder")
	}
	if def.Title != "Build a Specification" {
		t.Errorf("unexpected title: %q", def.Title)
	}

	_, ok = e.GetDefinition("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent journey")
	}
}

// --- Register ---

func TestEngine_RegisterCustomJourney(t *testing.T) {
	e := newTestEngine(t)

	custom := &JourneyDefinition{
		Name:        "custom-journey",
		Title:       "Custom Journey",
		Description: "A custom journey for testing",
		Phases: []Phase{
			{
				ID: "phase1", Title: "Phase 1", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Question 1?", Type: "text", Required: true},
				},
			},
		},
	}

	e.Register(custom)

	def, ok := e.GetDefinition("custom-journey")
	if !ok {
		t.Fatal("expected to find custom-journey after registration")
	}
	if def.Title != "Custom Journey" {
		t.Errorf("unexpected title: %q", def.Title)
	}

	// Should now have 5 journeys
	if len(e.ListJourneys()) != 5 {
		t.Errorf("expected 5 journeys, got %d", len(e.ListJourneys()))
	}
}

// --- Start ---

func TestEngine_Start_ReturnsFirstQuestion(t *testing.T) {
	e := newTestEngine(t)

	state, next, err := e.Start("spec-builder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// State checks
	if state.JourneyName != "spec-builder" {
		t.Errorf("journey name: expected 'spec-builder', got %q", state.JourneyName)
	}
	if state.Status != "active" {
		t.Errorf("status: expected 'active', got %q", state.Status)
	}
	if state.Phase != 0 {
		t.Errorf("phase: expected 0, got %d", state.Phase)
	}
	if state.Step != 0 {
		t.Errorf("step: expected 0, got %d", state.Step)
	}
	if len(state.Responses) != 0 {
		t.Errorf("responses: expected 0, got %d", len(state.Responses))
	}
	if state.Context == nil {
		t.Error("context should be initialized")
	}

	// NextStep checks
	if next.Done {
		t.Error("expected Done=false for first step")
	}
	if next.PhaseTitle != "Project Context" {
		t.Errorf("phase title: expected 'Project Context', got %q", next.PhaseTitle)
	}
	if next.PhaseNum != 1 {
		t.Errorf("phase num: expected 1, got %d", next.PhaseNum)
	}
	if next.TotalPhases != 6 {
		t.Errorf("total phases: expected 6, got %d", next.TotalPhases)
	}
	if next.Question == nil {
		t.Fatal("expected a question, got nil")
	}
	if next.Question.ID != "what" {
		t.Errorf("question ID: expected 'what', got %q", next.Question.ID)
	}
	if next.Question.Type != "multiline" {
		t.Errorf("question type: expected 'multiline', got %q", next.Question.Type)
	}
}

func TestEngine_Start_NotFound(t *testing.T) {
	e := newTestEngine(t)

	_, _, err := e.Start("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent journey")
	}
}

func TestEngine_Start_EmptyPhases(t *testing.T) {
	e := newTestEngine(t)
	e.Register(&JourneyDefinition{
		Name:   "empty",
		Title:  "Empty Journey",
		Phases: []Phase{},
	})

	_, _, err := e.Start("empty")
	if err == nil {
		t.Fatal("expected error for journey with no phases")
	}
}

func TestEngine_Start_PersistsState(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewEngine(tmpDir)

	state, _, err := e.Start("spec-builder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists on disk
	path := filepath.Join(tmpDir, state.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("state file not found: %v", err)
	}

	var loaded JourneyState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if loaded.JourneyName != "spec-builder" {
		t.Errorf("persisted journey name: expected 'spec-builder', got %q", loaded.JourneyName)
	}
	if loaded.Status != "active" {
		t.Errorf("persisted status: expected 'active', got %q", loaded.Status)
	}
}

func TestEngine_Start_UniqueSessionIDs(t *testing.T) {
	e := newTestEngine(t)
	// Use incrementing time to guarantee unique IDs
	counter := int64(0)
	e.nowFunc = func() time.Time {
		counter++
		return fixedTime().Add(time.Duration(counter) * time.Millisecond)
	}

	state1, _, _ := e.Start("spec-builder")
	state2, _, _ := e.Start("spec-builder")

	if state1.ID == state2.ID {
		t.Errorf("expected unique session IDs, got %q and %q", state1.ID, state2.ID)
	}
}

// --- Next ---

func TestEngine_Next_AdvancesWithinPhase(t *testing.T) {
	e := newTestEngine(t)

	state, _, err := e.Start("spec-builder")
	if err != nil {
		t.Fatalf("start error: %v", err)
	}

	// Answer first question ("what")
	state, next, err := e.Next(state.ID, "A project management tool")
	if err != nil {
		t.Fatalf("next error: %v", err)
	}

	// Should be on question 2 of phase 1 ("why")
	if next.Done {
		t.Error("should not be done after first answer")
	}
	if next.Question.ID != "why" {
		t.Errorf("expected question 'why', got %q", next.Question.ID)
	}
	if next.PhaseNum != 1 {
		t.Errorf("should still be phase 1, got %d", next.PhaseNum)
	}

	// Check response was recorded
	if len(state.Responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(state.Responses))
	}
	if state.Responses[0].Answer != "A project management tool" {
		t.Errorf("answer not recorded correctly: %q", state.Responses[0].Answer)
	}
	if state.Context["what"] != "A project management tool" {
		t.Errorf("context not updated: %q", state.Context["what"])
	}
}

func TestEngine_Next_AdvancesPhase(t *testing.T) {
	e := newTestEngine(t)

	state, _, _ := e.Start("spec-builder")

	// Answer all 3 questions of phase 1 (context: what, why, who)
	state, _, _ = e.Next(state.ID, "A PM tool")
	state, _, _ = e.Next(state.ID, "Improve productivity")
	state, next, _ := e.Next(state.ID, "Product managers")

	// Should now be on phase 2 (Scale Estimation)
	if next.Done {
		t.Error("should not be done")
	}
	if next.PhaseNum != 2 {
		t.Errorf("expected phase 2, got %d", next.PhaseNum)
	}
	if next.PhaseTitle != "Scale Estimation" {
		t.Errorf("expected 'Scale Estimation', got %q", next.PhaseTitle)
	}
	if next.Question.ID != "users_volume" {
		t.Errorf("expected 'users_volume', got %q", next.Question.ID)
	}

	// Check all 3 responses recorded
	if len(state.Responses) != 3 {
		t.Errorf("expected 3 responses, got %d", len(state.Responses))
	}
}

func TestEngine_Next_CompletesJourney(t *testing.T) {
	e := newTestEngine(t)

	// Use a small journey for easier completion
	e.Register(&JourneyDefinition{
		Name:  "mini",
		Title: "Mini Journey",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase 1", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Question 1?", Type: "text", Required: true},
				},
			},
			{
				ID: "p2", Title: "Phase 2", Order: 2,
				Questions: []Question{
					{ID: "q2", Prompt: "Question 2?", Type: "text", Required: true},
				},
			},
		},
	})

	state, _, _ := e.Start("mini")
	state, _, _ = e.Next(state.ID, "Answer 1")
	state, next, err := e.Next(state.ID, "Answer 2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be done
	if !next.Done {
		t.Error("expected journey to be completed")
	}
	if state.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", state.Status)
	}
	if next.Summary == "" {
		t.Error("expected a summary for completed journey")
	}
	if next.TotalPhases != 2 {
		t.Errorf("expected 2 total phases, got %d", next.TotalPhases)
	}
}

func TestEngine_Next_SessionNotFound(t *testing.T) {
	e := newTestEngine(t)

	_, _, err := e.Next("nonexistent-session", "answer")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestEngine_Next_AlreadyCompleted(t *testing.T) {
	e := newTestEngine(t)

	e.Register(&JourneyDefinition{
		Name:  "tiny",
		Title: "Tiny",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase 1", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Q?", Type: "text", Required: true},
				},
			},
		},
	})

	state, _, _ := e.Start("tiny")
	e.Next(state.ID, "Done")

	// Try to answer again
	_, next, err := e.Next(state.ID, "Extra answer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.Done {
		t.Error("expected Done=true for already completed journey")
	}
}

// --- Status ---

func TestEngine_Status_ActiveJourney(t *testing.T) {
	e := newTestEngine(t)

	state, _, _ := e.Start("spec-builder")
	e.Next(state.ID, "Answer 1")

	_, next, err := e.Status(state.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if next.Done {
		t.Error("expected active journey")
	}
	if next.Question.ID != "why" {
		t.Errorf("expected current question 'why', got %q", next.Question.ID)
	}
	if next.PhaseNum != 1 {
		t.Errorf("expected phase 1, got %d", next.PhaseNum)
	}
}

func TestEngine_Status_CompletedJourney(t *testing.T) {
	e := newTestEngine(t)

	e.Register(&JourneyDefinition{
		Name:  "one-q",
		Title: "One Question",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase 1", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Q?", Type: "text", Required: true},
				},
			},
		},
	})

	state, _, _ := e.Start("one-q")
	e.Next(state.ID, "Done")

	_, next, err := e.Status(state.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.Done {
		t.Error("expected completed journey")
	}
	if next.Summary == "" {
		t.Error("expected summary for completed journey")
	}
}

func TestEngine_Status_SessionNotFound(t *testing.T) {
	e := newTestEngine(t)

	_, _, err := e.Status("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

// --- buildSummary ---

func TestEngine_BuildSummary_ContainsAllAnswers(t *testing.T) {
	e := newTestEngine(t)

	e.Register(&JourneyDefinition{
		Name:  "summary-test",
		Title: "Summary Test",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase One", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "First question?", Type: "text", Required: true},
					{ID: "q2", Prompt: "Second question?", Type: "text", Required: true},
				},
			},
		},
	})

	state, _, _ := e.Start("summary-test")
	state, _, _ = e.Next(state.ID, "First answer")
	_, next, _ := e.Next(state.ID, "Second answer")

	if !next.Done {
		t.Fatal("expected journey to be done")
	}

	summary := next.Summary
	expectedContains := []string{
		"Summary Test", "Phase One",
		"First question?", "First answer",
		"Second question?", "Second answer",
	}

	for _, expected := range expectedContains {
		if !contains(summary, expected) {
			t.Errorf("summary missing %q.\nSummary:\n%s", expected, summary)
		}
	}
}

// --- buildInsights ---

func TestEngine_BuildInsights_ShortAnswerWarning(t *testing.T) {
	e := newTestEngine(t)

	e.Register(&JourneyDefinition{
		Name:  "insight-test",
		Title: "Insight Test",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Q1?", Type: "text", Required: true},
					{ID: "q2", Prompt: "Q2?", Type: "text", Required: true},
					{ID: "q3", Prompt: "Q3?", Type: "text", Required: true},
				},
			},
		},
	})

	state, _, _ := e.Start("insight-test")
	state, _, _ = e.Next(state.ID, "ok")       // Short
	state, _, _ = e.Next(state.ID, "yes")      // Short
	_, next, _ := e.Next(state.ID, "sure")     // Short

	if !next.Done {
		t.Fatal("expected done")
	}

	// Should have insight about short answers
	foundShortWarning := false
	for _, insight := range next.Insights {
		if contains(insight, "more detail") {
			foundShortWarning = true
		}
	}
	if !foundShortWarning {
		t.Errorf("expected short answer warning insight, got: %v", next.Insights)
	}
}

func TestEngine_BuildInsights_CountsResponses(t *testing.T) {
	e := newTestEngine(t)

	e.Register(&JourneyDefinition{
		Name:  "count-test",
		Title: "Count Test",
		Phases: []Phase{
			{
				ID: "p1", Title: "Phase", Order: 1,
				Questions: []Question{
					{ID: "q1", Prompt: "Q?", Type: "text", Required: true},
				},
			},
		},
	})

	state, _, _ := e.Start("count-test")
	_, next, _ := e.Next(state.ID, "A thorough and detailed answer here")

	foundCount := false
	for _, insight := range next.Insights {
		if contains(insight, "1 questions answered") {
			foundCount = true
		}
	}
	if !foundCount {
		t.Errorf("expected question count insight, got: %v", next.Insights)
	}
}

// --- State Persistence ---

func TestEngine_StatePersistence_AcrossEngineInstances(t *testing.T) {
	tmpDir := t.TempDir()

	// Engine 1: Start a journey
	e1 := NewEngine(tmpDir)
	state, _, _ := e1.Start("spec-builder")
	e1.Next(state.ID, "A project management tool")

	// Engine 2: New instance, same state dir
	e2 := NewEngine(tmpDir)
	loadedState, next, err := e2.Status(state.ID)
	if err != nil {
		t.Fatalf("failed to load state from new engine: %v", err)
	}

	if loadedState.JourneyName != "spec-builder" {
		t.Errorf("expected 'spec-builder', got %q", loadedState.JourneyName)
	}
	if len(loadedState.Responses) != 1 {
		t.Errorf("expected 1 response, got %d", len(loadedState.Responses))
	}
	if next.Question.ID != "why" {
		t.Errorf("expected current question 'why', got %q", next.Question.ID)
	}
}

func TestEngine_StatePersistence_ContinueFromNewEngine(t *testing.T) {
	tmpDir := t.TempDir()

	// Engine 1: Start and answer first question
	e1 := NewEngine(tmpDir)
	state, _, _ := e1.Start("backlog-refiner")
	e1.Next(state.ID, "E1: Backend API")

	// Engine 2: Continue from same session
	e2 := NewEngine(tmpDir)
	_, next, err := e2.Next(state.ID, "Too broad scope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be on phase 2 (Dependencies)
	if next.PhaseTitle != "Dependencies" {
		t.Errorf("expected 'Dependencies', got %q", next.PhaseTitle)
	}
}

// --- Save/Load Error Handling ---

func TestEngine_Start_SaveError(t *testing.T) {
	e := newTestEngine(t)
	e.mkdirAll = func(path string, perm os.FileMode) error {
		return fmt.Errorf("disk full")
	}

	_, _, err := e.Start("spec-builder")
	if err == nil {
		t.Fatal("expected error when save fails")
	}
}

func TestEngine_Next_SaveError(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewEngine(tmpDir)

	state, _, _ := e.Start("spec-builder")

	// Now make writes fail
	e.writeFile = func(name string, data []byte, perm os.FileMode) error {
		return fmt.Errorf("write failed")
	}

	_, _, err := e.Next(state.ID, "answer")
	if err == nil {
		t.Fatal("expected error when save fails during next")
	}
}

func TestEngine_Next_LoadError_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewEngine(tmpDir)

	// Write invalid JSON
	os.MkdirAll(tmpDir, 0755)
	os.WriteFile(filepath.Join(tmpDir, "bad-session.json"), []byte("{invalid"), 0644)

	_, _, err := e.Next("bad-session", "answer")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Full Journey Walkthroughs ---

func TestEngine_FullWalkthrough_BacklogRefiner(t *testing.T) {
	e := newTestEngine(t)

	state, next, err := e.Start("backlog-refiner")
	if err != nil {
		t.Fatalf("start error: %v", err)
	}

	// Phase 1: Scope Review (2 questions)
	if next.PhaseTitle != "Scope Review" {
		t.Fatalf("expected 'Scope Review', got %q", next.PhaseTitle)
	}

	state, next, _ = e.Next(state.ID, "E1: User Authentication")
	if next.Question.ID != "scope_concerns" {
		t.Fatalf("expected 'scope_concerns', got %q", next.Question.ID)
	}

	state, next, _ = e.Next(state.ID, "OAuth scope might be too broad")

	// Phase 2: Dependencies (2 questions)
	if next.PhaseTitle != "Dependencies" {
		t.Fatalf("expected 'Dependencies', got %q", next.PhaseTitle)
	}

	state, next, _ = e.Next(state.ID, "Auth must complete before profile")
	state, next, _ = e.Next(state.ID, "External: Auth0 team")

	// Phase 3: Acceptance Criteria (2 questions)
	if next.PhaseTitle != "Acceptance Criteria" {
		t.Fatalf("expected 'Acceptance Criteria', got %q", next.PhaseTitle)
	}

	state, next, _ = e.Next(state.ID, "S3: Password reset")
	_, next, _ = e.Next(state.ID, "Expired tokens, concurrent logins")

	// Should be done
	if !next.Done {
		t.Fatal("expected journey to be completed")
	}
	if next.TotalPhases != 3 {
		t.Errorf("expected 3 phases, got %d", next.TotalPhases)
	}
}

func TestEngine_FullWalkthrough_TechAdvisor(t *testing.T) {
	e := newTestEngine(t)

	state, _, _ := e.Start("tech-advisor")

	// Phase 1: Team Profile (2 questions)
	state, _, _ = e.Next(state.ID, "4-8 developers")
	state, _, _ = e.Next(state.ID, "Go, TypeScript, React, PostgreSQL")

	// Phase 2: Technical Constraints (2 questions)
	state, _, _ = e.Next(state.ID, "AWS")
	state, _, _ = e.Next(state.ID, "Greenfield - no existing stack")

	// Phase 3: Technical Priorities (2 questions)
	state, _, _ = e.Next(state.ID, "Developer productivity")
	_, next, _ := e.Next(state.ID, "No vendor lock-in")

	if !next.Done {
		t.Fatal("expected tech-advisor to be completed")
	}
}

func TestEngine_FullWalkthrough_DeepDiveExplorer(t *testing.T) {
	e := newTestEngine(t)

	state, _, _ := e.Start("deep-dive-explorer")

	// Phase 1: Focus Area (2 questions)
	state, _, _ = e.Next(state.ID, "PostgreSQL")
	state, _, _ = e.Next(state.ID, "Choosing between PostgreSQL and MongoDB")

	// Phase 2: Exploration Depth (2 questions)
	state, _, _ = e.Next(state.ID, "Performance trade-offs")
	_, next, _ := e.Next(state.ID, "Intermediate")

	if !next.Done {
		t.Fatal("expected deep-dive-explorer to be completed")
	}
}

// --- Journey Definitions Validation ---

func TestBuiltinJourneys_AllHaveValidStructure(t *testing.T) {
	journeys := BuiltinJourneys()

	for _, j := range journeys {
		t.Run(j.Name, func(t *testing.T) {
			if j.Name == "" {
				t.Error("journey name is empty")
			}
			if j.Title == "" {
				t.Error("journey title is empty")
			}
			if j.Description == "" {
				t.Error("journey description is empty")
			}
			if len(j.Phases) == 0 {
				t.Error("journey has no phases")
			}

			for i, phase := range j.Phases {
				if phase.ID == "" {
					t.Errorf("phase %d: ID is empty", i)
				}
				if phase.Title == "" {
					t.Errorf("phase %d: Title is empty", i)
				}
				if phase.Order != i+1 {
					t.Errorf("phase %d: Order=%d, expected %d", i, phase.Order, i+1)
				}
				if len(phase.Questions) == 0 {
					t.Errorf("phase %d (%s): has no questions", i, phase.ID)
				}

				for j, q := range phase.Questions {
					if q.ID == "" {
						t.Errorf("phase %d, question %d: ID is empty", i, j)
					}
					if q.Prompt == "" {
						t.Errorf("phase %d, question %d: Prompt is empty", i, j)
					}
					if q.Type == "" {
						t.Errorf("phase %d, question %d: Type is empty", i, j)
					}

					validTypes := map[string]bool{"text": true, "multiline": true, "select": true, "confirm": true}
					if !validTypes[q.Type] {
						t.Errorf("phase %d, question %d: invalid type %q", i, j, q.Type)
					}

					if q.Type == "select" && len(q.Options) == 0 {
						t.Errorf("phase %d, question %d: select type has no options", i, j)
					}

					if q.WhyAsk == "" {
						t.Errorf("phase %d, question %d: WhyAsk is empty (Socratic context required)", i, j)
					}
				}
			}
		})
	}
}

func TestSpecBuilderJourney_Has6Phases(t *testing.T) {
	journeys := BuiltinJourneys()
	var specBuilder *JourneyDefinition
	for _, j := range journeys {
		if j.Name == "spec-builder" {
			specBuilder = j
			break
		}
	}

	if specBuilder == nil {
		t.Fatal("spec-builder not found")
	}

	if len(specBuilder.Phases) != 6 {
		t.Errorf("spec-builder: expected 6 phases, got %d", len(specBuilder.Phases))
	}

	expectedPhaseIDs := []string{"context", "scale", "features", "integrations", "product", "nfrs"}
	for i, expected := range expectedPhaseIDs {
		if specBuilder.Phases[i].ID != expected {
			t.Errorf("phase %d: expected ID %q, got %q", i, expected, specBuilder.Phases[i].ID)
		}
	}
}

// --- Time tracking ---

func TestEngine_TimestampsAreRecorded(t *testing.T) {
	e := newTestEngine(t)
	now := fixedTime()
	e.nowFunc = func() time.Time { return now }

	state, _, _ := e.Start("spec-builder")

	if !state.CreatedAt.Equal(now) {
		t.Errorf("created_at: expected %v, got %v", now, state.CreatedAt)
	}
	if !state.UpdatedAt.Equal(now) {
		t.Errorf("updated_at: expected %v, got %v", now, state.UpdatedAt)
	}

	// Advance time for next step
	later := now.Add(5 * time.Minute)
	e.nowFunc = func() time.Time { return later }

	state, _, _ = e.Next(state.ID, "Answer")

	if !state.UpdatedAt.Equal(later) {
		t.Errorf("updated_at after next: expected %v, got %v", later, state.UpdatedAt)
	}
	if state.Responses[0].Timestamp != later {
		t.Errorf("response timestamp: expected %v, got %v", later, state.Responses[0].Timestamp)
	}
}

// --- Helper ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
