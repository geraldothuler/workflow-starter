package patterns_catalog

import (
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// MISSING COMPANION PATTERNS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeCoherence_MissingCompanion(t *testing.T) {
	// CQRS suggested but event-sourcing absent
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "cqrs", PatternName: "CQRS", Type: "pattern", Confidence: 0.8,
			Reasoning: "System needs separate read/write models"},
	}

	issues := AnalyzeCoherence(suggestions, pc)

	// Should find at least 1 missing companion (event-sourcing or domain-events)
	found := false
	for _, issue := range issues {
		if issue.Type == "missing-companion" && containsSubstring(issue.Description, "event-sourcing") {
			found = true
			if issue.Severity != "medium" && issue.Severity != "high" {
				t.Errorf("expected severity medium or high, got %q", issue.Severity)
			}
			break
		}
	}
	if !found {
		t.Error("expected missing-companion issue for CQRS → event-sourcing")
	}
}

func TestAnalyzeCoherence_MissingCompanion_AllPresent(t *testing.T) {
	// CQRS + event-sourcing + domain-events all present → no missing companion
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "cqrs", PatternName: "CQRS", Type: "pattern", Confidence: 0.8,
			Reasoning: "Separate read/write models"},
		{PatternID: "event-sourcing", PatternName: "Event Sourcing", Type: "pattern", Confidence: 0.7,
			Reasoning: "Store events for audit trail"},
		{PatternID: "domain-events", PatternName: "Domain Events", Type: "pattern", Confidence: 0.7,
			Reasoning: "Decouple domain logic"},
	}

	issues := AnalyzeCoherence(suggestions, pc)

	for _, issue := range issues {
		if issue.Type == "missing-companion" && strings.Contains(issue.PatternIDs[0], "cqrs") {
			t.Errorf("unexpected missing-companion issue for CQRS when all companions present: %s", issue.Description)
		}
	}
}

func TestAnalyzeCoherence_MissingCompanion_HighSeverity(t *testing.T) {
	// Pattern with >2 missing companions → high severity
	pc, err := NewPatternCatalog(
		WithEntries(CatalogEntry{
			ID: "test-pattern", Name: "Test Pattern", Type: "pattern",
			Level: "universal", Category: "structural",
			Source:          EntrySource{Reference: "Test", Author: "Test"},
			Description:     "Test pattern",
			RelatedPatterns: []string{"comp-a", "comp-b", "comp-c"},
			WhenToUse:       []string{"When testing coherence analysis"},
			Keywords:        []string{"test"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "test-pattern", PatternName: "Test Pattern", Type: "pattern", Confidence: 0.8,
			Reasoning: "When testing coherence analysis"},
	}

	issues := AnalyzeCoherence(suggestions, pc)

	found := false
	for _, issue := range issues {
		if issue.Type == "missing-companion" && strings.Contains(issue.Description, "3 related pattern") {
			found = true
			if issue.Severity != "high" {
				t.Errorf("expected severity high for >2 missing, got %q", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected high-severity missing-companion issue for >2 missing companions")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANTI-PATTERN CONTRADICTIONS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeCoherence_AntiPatternContradiction(t *testing.T) {
	// distributed-monolith detected but no remediation pattern suggested
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "distributed-monolith", PatternName: "Distributed Monolith",
			Type: "anti-pattern", Confidence: 0.75,
			Reasoning: "Services are tightly coupled"},
	}

	issues := AnalyzeCoherence(suggestions, pc)

	found := false
	for _, issue := range issues {
		if issue.Type == "anti-pattern-contradiction" {
			found = true
			if issue.Severity != "high" {
				t.Errorf("expected severity high, got %q", issue.Severity)
			}
			if !containsSubstring(issue.Description, "distributed-monolith") &&
				!containsSubstring(issue.Description, "Distributed Monolith") {
				t.Errorf("description should mention the anti-pattern: %s", issue.Description)
			}
			break
		}
	}
	if !found {
		t.Error("expected anti-pattern-contradiction issue for distributed-monolith without remediation")
	}
}

func TestAnalyzeCoherence_AntiPatternWithRemediation(t *testing.T) {
	// distributed-monolith detected AND bounded-context suggested → no contradiction
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "distributed-monolith", PatternName: "Distributed Monolith",
			Type: "anti-pattern", Confidence: 0.75,
			Reasoning: "Services are tightly coupled"},
		{PatternID: "bounded-context", PatternName: "Bounded Context",
			Type: "pattern", Confidence: 0.8,
			Reasoning: "Clear service boundaries"},
	}

	issues := AnalyzeCoherence(suggestions, pc)

	for _, issue := range issues {
		if issue.Type == "anti-pattern-contradiction" &&
			strings.Contains(issue.Description, "Distributed Monolith") {
			t.Errorf("should not have anti-pattern-contradiction when remediation is present: %s", issue.Description)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// WHEN-TO-USE MISMATCH
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeCoherence_WhenToUseMismatch(t *testing.T) {
	// Pattern with when_to_use keywords that don't match reasoning
	pc, err := NewPatternCatalog(
		WithEntries(CatalogEntry{
			ID: "mismatch-test", Name: "Mismatch Test", Type: "pattern",
			Level: "universal", Category: "structural",
			Source:      EntrySource{Reference: "Test", Author: "Test"},
			Description: "Test pattern for mismatch",
			WhenToUse: []string{
				"When you need distributed transactions across microservices",
				"When eventual consistency is acceptable",
			},
			Keywords: []string{"saga", "compensation"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Reasoning has nothing to do with distributed transactions or eventual consistency
	suggestions := []types.PatternSuggestion{
		{PatternID: "mismatch-test", PatternName: "Mismatch Test", Type: "pattern",
			Confidence: 0.6, Reasoning: "Simple CRUD application with basic forms"},
	}

	issues := AnalyzeCoherence(suggestions, pc)

	found := false
	for _, issue := range issues {
		if issue.Type == "when-to-use-mismatch" {
			found = true
			if issue.Severity != "low" {
				t.Errorf("expected severity low, got %q", issue.Severity)
			}
		}
	}
	if !found {
		t.Error("expected when-to-use-mismatch issue")
	}
}

func TestAnalyzeCoherence_WhenToUseMatch(t *testing.T) {
	// Pattern with matching keywords → no mismatch
	pc, err := NewPatternCatalog(
		WithEntries(CatalogEntry{
			ID: "match-test", Name: "Match Test", Type: "pattern",
			Level: "universal", Category: "structural",
			Source:      EntrySource{Reference: "Test", Author: "Test"},
			Description: "Test pattern for match",
			WhenToUse: []string{
				"When you need distributed transactions across microservices",
				"When eventual consistency is acceptable",
			},
			Keywords: []string{"saga", "compensation"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Reasoning matches "distributed transactions"
	suggestions := []types.PatternSuggestion{
		{PatternID: "match-test", PatternName: "Match Test", Type: "pattern",
			Confidence: 0.8, Reasoning: "Need distributed transactions for order processing"},
	}

	issues := AnalyzeCoherence(suggestions, pc)

	for _, issue := range issues {
		if issue.Type == "when-to-use-mismatch" && issue.PatternIDs[0] == "match-test" {
			t.Errorf("should not have when-to-use-mismatch when keywords match: %s", issue.Description)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EDGE CASES & ORDERING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeCoherence_BlockedIgnored(t *testing.T) {
	// Blocked suggestions should be excluded from analysis
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "cqrs", PatternName: "CQRS", Type: "pattern", Confidence: 0.8,
			Reasoning: "Separate read/write", BlockedBy: "Banned per ADR-042"},
	}

	issues := AnalyzeCoherence(suggestions, pc)

	for _, issue := range issues {
		if issue.Type == "missing-companion" && strings.Contains(issue.Description, "CQRS") {
			t.Errorf("blocked suggestion should be ignored: %s", issue.Description)
		}
	}
}

func TestAnalyzeCoherence_EmptyInput(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	// Empty suggestions
	issues := AnalyzeCoherence(nil, pc)
	if issues != nil {
		t.Errorf("expected nil for empty suggestions, got %d issues", len(issues))
	}

	// Empty catalog
	issues = AnalyzeCoherence([]types.PatternSuggestion{{PatternID: "x"}}, nil)
	if issues != nil {
		t.Errorf("expected nil for nil catalog, got %d issues", len(issues))
	}
}

func TestAnalyzeCoherence_SeverityOrdering(t *testing.T) {
	// Mix of severities → high should come first
	pc, err := NewPatternCatalog(
		WithEntries(CatalogEntry{
			ID: "mismatch-order", Name: "Mismatch Order", Type: "pattern",
			Level: "universal", Category: "structural",
			Source:      EntrySource{Reference: "Test", Author: "Test"},
			Description: "Test pattern",
			WhenToUse: []string{
				"When you need specialized caching strategies",
				"When performance optimization requires custom eviction",
			},
			Keywords: []string{"cache"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		// Anti-pattern without remediation → high severity
		{PatternID: "distributed-monolith", PatternName: "Distributed Monolith",
			Type: "anti-pattern", Confidence: 0.7, Reasoning: "Coupled services"},
		// Pattern with when_to_use mismatch → low severity
		{PatternID: "mismatch-order", PatternName: "Mismatch Order",
			Type: "pattern", Confidence: 0.6, Reasoning: "Simple logging"},
	}

	issues := AnalyzeCoherence(suggestions, pc)
	if len(issues) < 2 {
		t.Fatalf("expected at least 2 issues, got %d", len(issues))
	}

	// First issue should be high severity
	if issues[0].Severity != "high" {
		t.Errorf("first issue should be high severity, got %q", issues[0].Severity)
	}

	// Verify ordering: each issue severity <= next issue severity
	for i := 1; i < len(issues); i++ {
		prev := severityOrder[issues[i-1].Severity]
		curr := severityOrder[issues[i].Severity]
		if prev > curr {
			t.Errorf("issue %d (severity %q) should come before issue %d (severity %q)",
				i, issues[i].Severity, i-1, issues[i-1].Severity)
		}
	}
}

func TestAnalyzeCoherence_IDsSequential(t *testing.T) {
	// Multiple issues should have sequential IDs
	pc, err := NewPatternCatalog(
		WithEntries(CatalogEntry{
			ID: "seq-test", Name: "Seq Test", Type: "pattern",
			Level: "universal", Category: "structural",
			Source:          EntrySource{Reference: "Test", Author: "Test"},
			Description:     "Test pattern",
			RelatedPatterns: []string{"nonexistent-a", "nonexistent-b", "nonexistent-c"},
			WhenToUse: []string{
				"When specialized quantum computing is required",
				"When neural network compilation is needed",
			},
			Keywords: []string{"quantum"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "distributed-monolith", PatternName: "Distributed Monolith",
			Type: "anti-pattern", Confidence: 0.7, Reasoning: "Coupled"},
		{PatternID: "seq-test", PatternName: "Seq Test",
			Type: "pattern", Confidence: 0.6, Reasoning: "Simple logging"},
	}

	issues := AnalyzeCoherence(suggestions, pc)
	if len(issues) < 2 {
		t.Fatalf("expected at least 2 issues, got %d", len(issues))
	}

	// Check sequential IDs
	for i, issue := range issues {
		expectedID := strings.Replace("COH-00X", "X", string(rune('1'+i)), 1)
		_ = expectedID // simple check below
		if !strings.HasPrefix(issue.ID, "COH-") {
			t.Errorf("issue %d ID should start with COH-, got %q", i, issue.ID)
		}
	}

	// Verify IDs are COH-001, COH-002, ...
	if issues[0].ID != "COH-001" {
		t.Errorf("first issue ID should be COH-001, got %q", issues[0].ID)
	}
	if issues[1].ID != "COH-002" {
		t.Errorf("second issue ID should be COH-002, got %q", issues[1].ID)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TEXT MATCHING HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExtractSignificantWords(t *testing.T) {
	tests := []struct {
		name     string
		phrase   string
		minWords int // minimum expected significant words
	}{
		{"basic", "When you need distributed transactions", 2},
		{"with stop words", "the and for with that are", 0},
		{"short words filtered", "a be do it", 0},
		{"mixed", "When eventual consistency is acceptable", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			words := extractSignificantWords(tt.phrase)
			if len(words) < tt.minWords {
				t.Errorf("extractSignificantWords(%q) = %d words, want >= %d: %v",
					tt.phrase, len(words), tt.minWords, words)
			}
		})
	}
}

func TestContainsAnyWord(t *testing.T) {
	tests := []struct {
		text  string
		words []string
		want  bool
	}{
		{"distributed transactions across services", []string{"distributed", "saga"}, true},
		{"simple crud application", []string{"distributed", "saga"}, false},
		{"", []string{"word"}, false},
		{"some text", nil, false},
		{"some text", []string{}, false},
	}

	for _, tt := range tests {
		got := containsAnyWord(tt.text, tt.words)
		if got != tt.want {
			t.Errorf("containsAnyWord(%q, %v) = %v, want %v", tt.text, tt.words, got, tt.want)
		}
	}
}
