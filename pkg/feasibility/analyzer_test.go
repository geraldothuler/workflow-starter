package feasibility

import (
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// COMPLEXITY-EFFORT MISMATCH
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeFeasibility_ComplexityMismatch_High(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				Code: "E01", Title: "Complex Epic", Complexity: 9,
				Stories: []types.Story{
					{Effort: 1}, {Effort: 1}, {Effort: 2},
				},
			},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	found := false
	for _, item := range report.Items {
		if item.Category == "complexity" && strings.Contains(item.Title, "Underestimated") {
			found = true
			if item.Severity != "high" {
				t.Errorf("expected severity high, got %q", item.Severity)
			}
		}
	}
	if !found {
		t.Error("expected complexity-effort mismatch for high complexity + low effort")
	}
}

func TestAnalyzeFeasibility_ComplexityMismatch_None(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				Code: "E01", Title: "Balanced Epic", Complexity: 7,
				Stories: []types.Story{
					{Effort: 5}, {Effort: 3}, {Effort: 5},
				},
			},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	for _, item := range report.Items {
		if item.Category == "complexity" && strings.Contains(item.Title, "E01") {
			t.Errorf("should not flag balanced epic: %s", item.Title)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CRITICAL TECHNOLOGY CONCENTRATION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeFeasibility_CriticalTechConcentration(t *testing.T) {
	backlog := &types.Backlog{
		DeepDives: []types.DeepDive{
			{Term: "Kubernetes", Classification: "critical"},
			{Term: "Kafka", Classification: "critical"},
			{Term: "Flink", Classification: "critical"},
			{Term: "Cassandra", Classification: "critical"},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	found := false
	for _, item := range report.Items {
		if item.Category == "technology" && item.Severity == "critical" {
			found = true
			if !strings.Contains(item.Description, "4 critical") {
				t.Errorf("description should mention 4 critical techs: %s", item.Description)
			}
		}
	}
	if !found {
		t.Error("expected critical tech concentration item for 4 critical technologies")
	}
}

func TestAnalyzeFeasibility_CriticalTechLow(t *testing.T) {
	backlog := &types.Backlog{
		DeepDives: []types.DeepDive{
			{Term: "PostgreSQL", Classification: "critical"},
			{Term: "React", Classification: "standard"},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	for _, item := range report.Items {
		if item.Category == "technology" && item.Severity == "critical" {
			t.Errorf("should not flag critical for 1 critical tech: %s", item.Title)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// REQUIREMENT COMPLETENESS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeFeasibility_RequirementGaps(t *testing.T) {
	gaps := []types.Gap{
		{Type: "nfr_missing", Severity: "high", Message: "NFR not defined"},
		{Type: "security_missing", Severity: "high", Message: "Security not addressed"},
		{Type: "scale_missing", Severity: "high", Message: "Scale undefined"},
	}

	report := AnalyzeFeasibility(&types.Backlog{}, gaps)

	found := false
	for _, item := range report.Items {
		if item.Category == "requirements" && item.Severity == "critical" {
			found = true
			if !strings.Contains(item.Description, "3 high-severity") {
				t.Errorf("should mention 3 high-severity gaps: %s", item.Description)
			}
		}
	}
	if !found {
		t.Error("expected critical requirement gap item for 3 high-severity gaps")
	}
}

func TestAnalyzeFeasibility_RequirementGaps_None(t *testing.T) {
	report := AnalyzeFeasibility(&types.Backlog{}, nil)

	for _, item := range report.Items {
		if item.Category == "requirements" {
			t.Errorf("should not flag requirements with nil gaps: %s", item.Title)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ARCHITECTURE RISK
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeFeasibility_ArchitectureRisk(t *testing.T) {
	backlog := &types.Backlog{
		PatternSuggestions: []types.PatternSuggestion{
			{PatternID: "distributed-monolith", Type: "anti-pattern", Confidence: 0.8},
			{PatternID: "god-class", Type: "anti-pattern", Confidence: 0.7},
		},
		CoherenceIssues: []types.CoherenceIssue{
			{ID: "COH-001", Severity: "high", Title: "Missing companion"},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	antiPatternFound := false
	coherenceFound := false
	for _, item := range report.Items {
		if item.Category == "architecture" && strings.Contains(item.Title, "anti-pattern") {
			antiPatternFound = true
			if item.Severity != "high" {
				t.Errorf("expected high severity for 2 anti-patterns, got %q", item.Severity)
			}
		}
		if item.Category == "architecture" && strings.Contains(item.Title, "coherence") {
			coherenceFound = true
		}
	}
	if !antiPatternFound {
		t.Error("expected anti-pattern architecture risk item")
	}
	if !coherenceFound {
		t.Error("expected coherence architecture risk item")
	}
}

func TestAnalyzeFeasibility_ArchitectureRisk_Clean(t *testing.T) {
	backlog := &types.Backlog{
		PatternSuggestions: []types.PatternSuggestion{
			{PatternID: "cqrs", Type: "pattern", Confidence: 0.8},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	for _, item := range report.Items {
		if item.Category == "architecture" {
			t.Errorf("should not flag architecture risk for clean patterns: %s", item.Title)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SCHEDULE RISK
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeFeasibility_ScheduleRisk(t *testing.T) {
	// >40% stories ≥5 effort
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				Code: "E01", Title: "Big Epic",
				Stories: []types.Story{
					{Effort: 8}, {Effort: 5}, {Effort: 5}, {Effort: 2}, {Effort: 1},
				},
			},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	found := false
	for _, item := range report.Items {
		if item.Category == "schedule" && strings.Contains(item.Title, "large stories") {
			found = true
			if item.Severity != "high" {
				t.Errorf("expected high severity for >40%% large stories, got %q", item.Severity)
			}
		}
	}
	if !found {
		t.Error("expected schedule risk for high concentration of large stories")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EDGE CASES & SCORING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeFeasibility_EmptyBacklog(t *testing.T) {
	report := AnalyzeFeasibility(&types.Backlog{}, nil)

	if report.Score != 100 {
		t.Errorf("empty backlog should have score 100, got %d", report.Score)
	}
	if len(report.Items) != 0 {
		t.Errorf("empty backlog should have 0 items, got %d", len(report.Items))
	}
}

func TestAnalyzeFeasibility_NilBacklog(t *testing.T) {
	report := AnalyzeFeasibility(nil, nil)

	if report.Score != 100 {
		t.Errorf("nil backlog should have score 100, got %d", report.Score)
	}
}

func TestAnalyzeFeasibility_ScoreCalculation(t *testing.T) {
	// 1 critical (-25) + 1 high (-15) = score 60
	backlog := &types.Backlog{
		DeepDives: []types.DeepDive{
			{Term: "Kubernetes", Classification: "critical"},
			{Term: "Kafka", Classification: "critical"},
			{Term: "Flink", Classification: "critical"},
			{Term: "Cassandra", Classification: "critical"},
		},
		CoherenceIssues: []types.CoherenceIssue{
			{ID: "COH-001", Severity: "high", Title: "Missing companion"},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	// Should have at least 2 items
	if len(report.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(report.Items))
	}

	// Score should be < 100
	if report.Score >= 100 {
		t.Errorf("expected score < 100 with risks, got %d", report.Score)
	}

	// Score should be >= 0
	if report.Score < 0 {
		t.Errorf("score should not be negative, got %d", report.Score)
	}
}

func TestAnalyzeFeasibility_Ordering(t *testing.T) {
	// Mix of severities → critical should come first
	backlog := &types.Backlog{
		DeepDives: []types.DeepDive{
			{Term: "K8s", Classification: "critical"},
			{Term: "Kafka", Classification: "critical"},
			{Term: "Flink", Classification: "critical"},
			{Term: "Cassandra", Classification: "critical"},
		},
		Epics: []types.Epic{
			{
				Code: "E01", Title: "Simple Epic", Complexity: 2,
				Stories: []types.Story{
					{Effort: 8}, {Effort: 5},
				},
			},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	if len(report.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(report.Items))
	}

	// First item should be critical
	if report.Items[0].Severity != "critical" {
		t.Errorf("first item should be critical, got %q", report.Items[0].Severity)
	}

	// Verify ordering: severity should be non-decreasing
	for i := 1; i < len(report.Items); i++ {
		prev := severityOrder[report.Items[i-1].Severity]
		curr := severityOrder[report.Items[i].Severity]
		if prev > curr {
			t.Errorf("item %d (severity %q) should not come after item %d (severity %q)",
				i, report.Items[i].Severity, i-1, report.Items[i-1].Severity)
		}
	}
}

func TestAnalyzeFeasibility_IDsSequential(t *testing.T) {
	backlog := &types.Backlog{
		DeepDives: []types.DeepDive{
			{Term: "K8s", Classification: "critical"},
			{Term: "Kafka", Classification: "critical"},
			{Term: "Flink", Classification: "critical"},
			{Term: "Cassandra", Classification: "critical"},
		},
		PatternSuggestions: []types.PatternSuggestion{
			{PatternID: "god-class", Type: "anti-pattern", Confidence: 0.8},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	if len(report.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(report.Items))
	}

	// Check sequential IDs
	if report.Items[0].ID != "FSB-001" {
		t.Errorf("first item ID should be FSB-001, got %q", report.Items[0].ID)
	}
	if report.Items[1].ID != "FSB-002" {
		t.Errorf("second item ID should be FSB-002, got %q", report.Items[1].ID)
	}
}

func TestAnalyzeFeasibility_Summary(t *testing.T) {
	backlog := &types.Backlog{
		DeepDives: []types.DeepDive{
			{Term: "K8s", Classification: "critical"},
			{Term: "Kafka", Classification: "critical"},
			{Term: "Flink", Classification: "critical"},
			{Term: "Cassandra", Classification: "critical"},
		},
	}

	report := AnalyzeFeasibility(backlog, nil)

	if report.Summary == "" {
		t.Error("summary should not be empty when there are items")
	}
	if !strings.Contains(report.Summary, "risk(s) found") {
		t.Errorf("summary should contain 'risk(s) found': %s", report.Summary)
	}
}

func TestBuildSummary_Empty(t *testing.T) {
	s := buildSummary(nil)
	if s != "No risks found" {
		t.Errorf("expected 'No risks found', got %q", s)
	}
}
