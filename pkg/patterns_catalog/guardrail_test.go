package patterns_catalog

import (
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HIERARCHY ENFORCEMENT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestApplyHierarchy_NoConflicts(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "cqrs", PatternName: "CQRS", Type: "pattern", Level: "universal", Confidence: 0.8},
		{PatternID: "facade", PatternName: "Facade", Type: "pattern", Level: "universal", Confidence: 0.7},
	}

	result := ApplyHierarchy(suggestions, pc)
	if len(result) != 2 {
		t.Fatalf("got %d results, want 2", len(result))
	}

	for _, s := range result {
		if s.BlockedBy != "" {
			t.Errorf("suggestion %q should not be blocked: %s", s.PatternID, s.BlockedBy)
		}
	}
}

func TestApplyHierarchy_CompanyOverrideBlocksUniversal(t *testing.T) {
	// Simulate: company reclassifies "singleton" as anti-pattern
	pc, err := NewPatternCatalog(
		WithEntries(CatalogEntry{
			ID: "singleton", Name: "Singleton", Type: "anti-pattern",
			Level: "company", Category: "creational",
			Source:      EntrySource{Reference: "ADR-042", Author: "Architecture Team"},
			Description: "Singleton banned per ADR",
			Signs:       []string{"Global state"},
			Remediation: []string{"factory-method"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	// LLM suggests singleton as pattern (based on universal catalog)
	suggestions := []types.PatternSuggestion{
		{PatternID: "singleton", PatternName: "Singleton", Type: "pattern", Level: "universal", Confidence: 0.75},
	}

	result := ApplyHierarchy(suggestions, pc)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}

	if result[0].BlockedBy == "" {
		t.Error("singleton should be blocked (company override reclassified as anti-pattern)")
	}
}

func TestApplyHierarchy_ProjectOverrideBlocksTeam(t *testing.T) {
	pc, err := NewPatternCatalog(
		WithEntries(
			CatalogEntry{
				ID: "active-record", Name: "Active Record", Type: "anti-pattern",
				Level: "project", Category: "structural",
				Source:      EntrySource{Reference: "Project Decision", Author: "Tech Lead"},
				Description: "Active Record banned for this project",
				Signs:       []string{"Direct DB coupling"},
				Remediation: []string{"repository", "data-mapper"},
			},
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	suggestions := []types.PatternSuggestion{
		{PatternID: "active-record", PatternName: "Active Record", Type: "pattern", Level: "universal", Confidence: 0.8},
	}

	result := ApplyHierarchy(suggestions, pc)
	if result[0].BlockedBy == "" {
		t.Error("active-record should be blocked (project override)")
	}
}

func TestApplyHierarchy_AntiPatternEnrichedWithRemediation(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	// Anti-pattern suggestion without remediation — should be enriched
	suggestions := []types.PatternSuggestion{
		{PatternID: "god-class", PatternName: "God Class", Type: "anti-pattern", Level: "universal", Confidence: 0.7},
	}

	result := ApplyHierarchy(suggestions, pc)
	if len(result[0].Remediation) == 0 {
		t.Error("god-class remediation should be enriched from catalog")
	}
}

func TestApplyHierarchy_PreservesExistingRemediation(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	// Anti-pattern with existing remediation — should NOT be overwritten
	suggestions := []types.PatternSuggestion{
		{
			PatternID:   "god-class",
			PatternName: "God Class",
			Type:        "anti-pattern",
			Level:       "universal",
			Confidence:  0.7,
			Remediation: []string{"custom-remediation"},
		},
	}

	result := ApplyHierarchy(suggestions, pc)
	if len(result[0].Remediation) != 1 || result[0].Remediation[0] != "custom-remediation" {
		t.Error("existing remediation should be preserved")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CONFLICT DETECTION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestDetectConflict_NoConflict(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	suggestion := types.PatternSuggestion{
		PatternID: "cqrs", Type: "pattern", Level: "universal",
	}

	conflict := detectConflict(suggestion, pc)
	if conflict != "" {
		t.Errorf("expected no conflict, got: %s", conflict)
	}
}

func TestDetectConflict_TypeMismatch(t *testing.T) {
	pc, err := NewPatternCatalog(
		WithEntries(CatalogEntry{
			ID: "testcontainers", Name: "TestContainers", Type: "anti-pattern",
			Level: "company", Category: "structural",
			Source:      EntrySource{Reference: "ADR-057", Author: "Engineering Team"},
			Description: "Banned per ADR",
			Signs:       []string{"Using TestContainers"},
			Remediation: []string{"company-integration-test-pods"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	suggestion := types.PatternSuggestion{
		PatternID: "testcontainers", Type: "pattern", Level: "universal",
	}

	conflict := detectConflict(suggestion, pc)
	if conflict == "" {
		t.Error("expected conflict (company says anti-pattern, suggestion says pattern)")
	}
}

func TestDetectConflict_UnknownPattern(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	suggestion := types.PatternSuggestion{
		PatternID: "unknown-pattern", Type: "pattern", Level: "universal",
	}

	conflict := detectConflict(suggestion, pc)
	if conflict != "" {
		t.Errorf("expected no conflict for unknown pattern, got: %s", conflict)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CONFLICT REPORT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestGenerateConflictReport(t *testing.T) {
	suggestions := []types.PatternSuggestion{
		{PatternID: "cqrs", Confidence: 0.8},
		{PatternID: "singleton", Confidence: 0.7, BlockedBy: "Banned per ADR-042"},
		{PatternID: "facade", Confidence: 0.6},
	}

	report := GenerateConflictReport(suggestions)

	if report.TotalInput != 3 {
		t.Errorf("TotalInput = %d, want 3", report.TotalInput)
	}
	if len(report.Clean) != 2 {
		t.Errorf("Clean = %d, want 2", len(report.Clean))
	}
	if len(report.Blocked) != 1 {
		t.Errorf("Blocked = %d, want 1", len(report.Blocked))
	}
}

func TestConflictReport_FormatReport(t *testing.T) {
	report := ConflictReport{
		TotalInput: 3,
		Clean: []types.PatternSuggestion{
			{PatternID: "cqrs"},
		},
		Blocked: []types.PatternSuggestion{
			{PatternID: "singleton", PatternName: "Singleton", BlockedBy: "Banned per ADR-042", Remediation: []string{"factory-method"}},
		},
	}

	formatted := report.FormatReport()
	if formatted == "" {
		t.Error("FormatReport() returned empty")
	}

	expectedStrings := []string{"3 total", "1 clean", "1 blocked", "Singleton", "ADR-042", "factory-method"}
	for _, s := range expectedStrings {
		if !contains(formatted, s) {
			t.Errorf("FormatReport() missing %q", s)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
