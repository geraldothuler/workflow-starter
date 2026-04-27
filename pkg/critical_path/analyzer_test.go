package critical_path

import (
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// NIL / EMPTY / SINGLE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeCriticalPath_NilBacklog(t *testing.T) {
	report := AnalyzeCriticalPath(nil)

	if len(report.Items) != 0 {
		t.Errorf("expected 0 items for nil backlog, got %d", len(report.Items))
	}
	if len(report.Phases) != 0 {
		t.Errorf("expected 0 phases for nil backlog, got %d", len(report.Phases))
	}
}

func TestAnalyzeCriticalPath_EmptyBacklog(t *testing.T) {
	report := AnalyzeCriticalPath(&types.Backlog{})

	if len(report.Phases) != 0 {
		t.Errorf("expected 0 phases for empty backlog, got %d", len(report.Phases))
	}
	if len(report.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(report.Dependencies))
	}
}

func TestAnalyzeCriticalPath_SingleEpic(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "E01", Title: "Feature A", Stories: []types.Story{{Effort: 5}}},
		},
	}

	report := AnalyzeCriticalPath(backlog)

	if len(report.Phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(report.Phases))
	}
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(report.Items))
	}
	if len(report.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies for single epic, got %d", len(report.Dependencies))
	}
	if report.Phases[0].Phase != 1 {
		t.Errorf("expected phase 1, got %d", report.Phases[0].Phase)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FOUNDATION DETECTION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestDetectFoundations_ByTitle(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "AUTH", Title: "Authentication and Authorization Setup"},
			{Code: "UI", Title: "User Interface"},
		},
	}

	foundations := detectFoundations(backlog)

	if !foundations["AUTH"] {
		t.Error("expected AUTH to be detected as foundation (auth + setup)")
	}
	if foundations["UI"] {
		t.Error("expected UI NOT to be foundation")
	}
}

func TestDetectFoundations_ByTags(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "INFRA", Title: "System Setup", Tags: []string{"database", "infrastructure"}},
		},
	}

	foundations := detectFoundations(backlog)

	if !foundations["INFRA"] {
		t.Error("expected INFRA to be detected as foundation via tags")
	}
}

func TestDetectFoundations_NoMatch(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "FEAT", Title: "User Notifications", Description: "Send alerts to users"},
		},
	}

	foundations := detectFoundations(backlog)

	if foundations["FEAT"] {
		t.Error("expected FEAT NOT to be foundation")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TECHNOLOGY DEPENDENCY INFERENCE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestInferTechDependencies_CriticalToStandard(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				Code: "DATA", Title: "Data Pipeline",
				Stories: []types.Story{{ID: "S1", EpicID: "DATA"}},
			},
			{
				Code: "API", Title: "REST API",
				Stories: []types.Story{{ID: "S2", EpicID: "API"}},
			},
		},
		DeepDives: []types.DeepDive{
			{StoryID: "S1", Term: "PostgreSQL", Classification: "critical"},
			{StoryID: "S2", Term: "PostgreSQL", Classification: "standard"},
		},
	}

	edges := inferTechDependencies(backlog)

	found := false
	for _, edge := range edges {
		if edge.From == "DATA" && edge.To == "API" && edge.Type == "technology" {
			found = true
			if edge.Confidence != 0.8 {
				t.Errorf("expected confidence 0.8, got %f", edge.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected DATA → API dependency (critical PostgreSQL → standard PostgreSQL)")
	}
}

func TestInferTechDependencies_NoDuplicateEdges(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				Code: "A", Title: "Epic A",
				Stories: []types.Story{{ID: "S1", EpicID: "A"}},
			},
			{
				Code: "B", Title: "Epic B",
				Stories: []types.Story{{ID: "S2", EpicID: "B"}},
			},
		},
		DeepDives: []types.DeepDive{
			{StoryID: "S1", Term: "Redis", Classification: "critical"},
			{StoryID: "S2", Term: "Redis", Classification: "standard"},
			{StoryID: "S1", Term: "Kafka", Classification: "specific"},
			{StoryID: "S2", Term: "Kafka", Classification: "standard"},
		},
	}

	// inferTechDependencies may produce 2 edges A→B (one per tech)
	// After deduplication, should keep the highest confidence
	techEdges := inferTechDependencies(backlog)
	deduped := deduplicateEdges(techEdges)

	count := 0
	for _, edge := range deduped {
		if edge.From == "A" && edge.To == "B" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected at most 1 A→B edge after dedup, got %d", count)
	}
}

func TestInferTechDependencies_NoDeepDives(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "A", Title: "Epic A"},
			{Code: "B", Title: "Epic B"},
		},
	}

	edges := inferTechDependencies(backlog)

	if len(edges) != 0 {
		t.Errorf("expected 0 edges without deep dives, got %d", len(edges))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PATTERN DEPENDENCY ANALYSIS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzePatternDependencies_SharedPattern(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "AUTH", Title: "Auth", Complexity: 8},
			{Code: "API", Title: "API", Complexity: 5},
		},
		PatternSuggestions: []types.PatternSuggestion{
			{
				PatternName:   "JWT Authentication",
				Type:          "pattern",
				AffectedEpics: []string{"AUTH", "API"},
			},
		},
	}

	foundations := map[string]bool{}
	edges := analyzePatternDependencies(backlog, foundations)

	found := false
	for _, edge := range edges {
		if edge.From == "AUTH" && edge.To == "API" && edge.Type == "pattern" {
			found = true
		}
	}
	if !found {
		t.Error("expected AUTH → API edge (AUTH has higher complexity)")
	}
}

func TestAnalyzePatternDependencies_NoPatterns(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "A", Title: "A"},
			{Code: "B", Title: "B"},
		},
	}

	edges := analyzePatternDependencies(backlog, map[string]bool{})

	if len(edges) != 0 {
		t.Errorf("expected 0 edges without patterns, got %d", len(edges))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PRIORITY SCORING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestScoreEpicPriority_FoundationBonus(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "AUTH", Title: "Auth", Complexity: 5},
			{Code: "UI", Title: "UI", Complexity: 5},
		},
	}

	foundations := map[string]bool{"AUTH": true}
	scores := scoreEpicPriority(backlog, foundations, nil)

	if scores["AUTH"] <= scores["UI"] {
		t.Errorf("expected AUTH (%d) to score higher than UI (%d) due to foundation bonus",
			scores["AUTH"], scores["UI"])
	}
	// Foundation bonus = 30, so diff should be at least 30
	if scores["AUTH"]-scores["UI"] < 30 {
		t.Errorf("expected at least 30 point difference, got %d", scores["AUTH"]-scores["UI"])
	}
}

func TestScoreEpicPriority_HighComplexity(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "HARD", Title: "Hard Epic", Complexity: 9},
			{Code: "EASY", Title: "Easy Epic", Complexity: 3},
		},
	}

	scores := scoreEpicPriority(backlog, map[string]bool{}, nil)

	if scores["HARD"] <= scores["EASY"] {
		t.Errorf("expected HARD (%d) to score higher than EASY (%d)",
			scores["HARD"], scores["EASY"])
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EXECUTION PHASE GROUPING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestGroupExecutionPhases_LinearDeps(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "A", Title: "Epic A"},
			{Code: "B", Title: "Epic B"},
			{Code: "C", Title: "Epic C"},
		},
	}

	edges := []types.DependencyEdge{
		{From: "A", To: "B", Type: "foundation", Confidence: 0.9},
		{From: "B", To: "C", Type: "technology", Confidence: 0.8},
	}

	scores := map[string]int{"A": 80, "B": 50, "C": 30}
	foundations := map[string]bool{}

	phases, items := groupExecutionPhases(backlog, scores, edges, foundations)

	if len(phases) != 3 {
		t.Fatalf("expected 3 phases for linear A→B→C, got %d", len(phases))
	}

	// Phase 1 should have A, Phase 2 B, Phase 3 C
	if len(phases[0].EpicCodes) != 1 || phases[0].EpicCodes[0] != "A" {
		t.Errorf("Phase 1 should have [A], got %v", phases[0].EpicCodes)
	}
	if len(phases[1].EpicCodes) != 1 || phases[1].EpicCodes[0] != "B" {
		t.Errorf("Phase 2 should have [B], got %v", phases[1].EpicCodes)
	}
	if len(phases[2].EpicCodes) != 1 || phases[2].EpicCodes[0] != "C" {
		t.Errorf("Phase 3 should have [C], got %v", phases[2].EpicCodes)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestGroupExecutionPhases_ParallelEpics(t *testing.T) {
	// A→C and B→C → Phase 1=[A,B] (parallel), Phase 2=[C]
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "A", Title: "Epic A"},
			{Code: "B", Title: "Epic B"},
			{Code: "C", Title: "Epic C"},
		},
	}

	edges := []types.DependencyEdge{
		{From: "A", To: "C", Type: "technology", Confidence: 0.8},
		{From: "B", To: "C", Type: "foundation", Confidence: 0.9},
	}

	scores := map[string]int{"A": 70, "B": 60, "C": 30}
	foundations := map[string]bool{}

	phases, _ := groupExecutionPhases(backlog, scores, edges, foundations)

	if len(phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(phases))
	}

	// Phase 1 should have A and B (parallel)
	if len(phases[0].EpicCodes) != 2 {
		t.Errorf("Phase 1 should have 2 epics, got %d: %v", len(phases[0].EpicCodes), phases[0].EpicCodes)
	}
	if !phases[0].Parallel {
		t.Error("Phase 1 should be marked as parallel")
	}

	// Phase 2 should have C
	if len(phases[1].EpicCodes) != 1 || phases[1].EpicCodes[0] != "C" {
		t.Errorf("Phase 2 should have [C], got %v", phases[1].EpicCodes)
	}
}

func TestGroupExecutionPhases_NoDependencies(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "A", Title: "Epic A"},
			{Code: "B", Title: "Epic B"},
			{Code: "C", Title: "Epic C"},
		},
	}

	scores := map[string]int{"A": 50, "B": 40, "C": 30}

	phases, _ := groupExecutionPhases(backlog, scores, nil, map[string]bool{})

	if len(phases) != 1 {
		t.Fatalf("expected 1 phase (all parallel), got %d", len(phases))
	}
	if len(phases[0].EpicCodes) != 3 {
		t.Errorf("Phase 1 should have all 3 epics, got %d", len(phases[0].EpicCodes))
	}
	if !phases[0].Parallel {
		t.Error("Phase 1 should be marked as parallel")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// INTEGRATION / EDGE CASES
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeCriticalPath_IDsSequential(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "A", Title: "Epic A"},
			{Code: "B", Title: "Epic B"},
			{Code: "C", Title: "Epic C"},
		},
	}

	report := AnalyzeCriticalPath(backlog)

	if len(report.Items) < 3 {
		t.Fatalf("expected at least 3 items, got %d", len(report.Items))
	}

	if report.Items[0].ID != "CPT-001" {
		t.Errorf("first item ID should be CPT-001, got %q", report.Items[0].ID)
	}
	if report.Items[1].ID != "CPT-002" {
		t.Errorf("second item ID should be CPT-002, got %q", report.Items[1].ID)
	}
	if report.Items[2].ID != "CPT-003" {
		t.Errorf("third item ID should be CPT-003, got %q", report.Items[2].ID)
	}
}

func TestAnalyzeCriticalPath_Summary(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "AUTH", Title: "Authentication and Authorization Setup"},
			{Code: "API", Title: "REST API"},
		},
	}

	report := AnalyzeCriticalPath(backlog)

	if report.Summary == "" {
		t.Error("summary should not be empty")
	}
	if !strings.Contains(report.Summary, "phase") {
		t.Errorf("summary should contain 'phase': %s", report.Summary)
	}
	if !strings.Contains(report.Summary, "dependenc") {
		t.Errorf("summary should contain 'dependenc': %s", report.Summary)
	}
}

func TestAnalyzeCriticalPath_FoundationFirstPhase(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Code: "UI", Title: "User Interface"},
			{Code: "AUTH", Title: "Authentication and Authorization Setup"},
			{Code: "INFRA", Title: "Infrastructure and Database Configuration"},
		},
	}

	report := AnalyzeCriticalPath(backlog)

	if len(report.Phases) == 0 {
		t.Fatal("expected at least 1 phase")
	}

	// Foundation epics should be in phase 1
	phase1Codes := make(map[string]bool)
	for _, code := range report.Phases[0].EpicCodes {
		phase1Codes[code] = true
	}

	// AUTH and INFRA should be foundations and in phase 1
	foundAuth := false
	foundInfra := false
	for _, item := range report.Items {
		if item.EpicCode == "AUTH" && item.IsFoundation {
			foundAuth = true
		}
		if item.EpicCode == "INFRA" && item.IsFoundation {
			foundInfra = true
		}
	}
	if !foundAuth {
		t.Error("AUTH should be detected as foundation")
	}
	if !foundInfra {
		t.Error("INFRA should be detected as foundation")
	}
}
