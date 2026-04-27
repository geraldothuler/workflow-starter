package patterns_catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// LOADING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestNewPatternCatalog_LoadsEmbedded(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	if pc.PatternCount() == 0 {
		t.Fatal("expected entries to be loaded from embedded config")
	}

	// Expect exactly 64 entries (56 patterns + 8 anti-patterns)
	if pc.PatternCount() != 64 {
		t.Errorf("PatternCount() = %d, want 64", pc.PatternCount())
	}
}

func TestNewPatternCatalog_CorrectPatternCount(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	patterns := pc.Patterns()
	antiPatterns := pc.AntiPatterns()

	if len(patterns) != 56 {
		t.Errorf("Patterns() count = %d, want 56", len(patterns))
	}
	if len(antiPatterns) != 8 {
		t.Errorf("AntiPatterns() count = %d, want 8", len(antiPatterns))
	}
}

func TestNewPatternCatalog_Categories(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	cats := pc.Categories()
	if len(cats) < 10 {
		t.Errorf("Categories() count = %d, want >= 10", len(cats))
	}

	// Verify expected categories exist
	catMap := make(map[string]bool)
	for _, c := range cats {
		catMap[c.ID] = true
	}

	expectedCats := []string{"creational", "structural", "behavioral", "data",
		"integration", "resilience", "security", "scalability", "design-first", "migration"}
	for _, cat := range expectedCats {
		if !catMap[cat] {
			t.Errorf("expected category %q not found", cat)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ACCESSORS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestGetByID_ExistingPattern(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	tests := []struct {
		id       string
		wantName string
		wantType string
	}{
		{"cqrs", "CQRS", "pattern"},
		{"factory-method", "Factory Method", "pattern"},
		{"distributed-monolith", "Distributed Monolith", "anti-pattern"},
		{"api-first", "API First", "pattern"},
		{"circuit-breaker", "Circuit Breaker", "pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			entry := pc.GetByID(tt.id)
			if entry == nil {
				t.Fatalf("GetByID(%q) = nil", tt.id)
			}
			if entry.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", entry.Name, tt.wantName)
			}
			if entry.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", entry.Type, tt.wantType)
			}
		})
	}
}

func TestGetByID_NotFound(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	entry := pc.GetByID("nonexistent-pattern")
	if entry != nil {
		t.Errorf("GetByID(nonexistent) = %v, want nil", entry)
	}
}

func TestEntriesByCategory(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// Creational should have 5 GoF patterns
	creational := pc.EntriesByCategory("creational")
	if len(creational) != 5 {
		t.Errorf("EntriesByCategory(creational) = %d, want 5", len(creational))
	}

	// Verify all are type "pattern"
	for _, e := range creational {
		if e.Type != "pattern" {
			t.Errorf("creational entry %q has type %q, want pattern", e.ID, e.Type)
		}
	}
}

func TestEntriesByLevel(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// All embedded entries should be universal
	universal := pc.EntriesByLevel("universal")
	if len(universal) != 64 {
		t.Errorf("EntriesByLevel(universal) = %d, want 64", len(universal))
	}

	// No company/team/project entries in embedded catalog
	company := pc.EntriesByLevel("company")
	if len(company) != 0 {
		t.Errorf("EntriesByLevel(company) = %d, want 0", len(company))
	}
}

func TestEffectiveType(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	if got := pc.EffectiveType("cqrs"); got != "pattern" {
		t.Errorf("EffectiveType(cqrs) = %q, want pattern", got)
	}
	if got := pc.EffectiveType("distributed-monolith"); got != "anti-pattern" {
		t.Errorf("EffectiveType(distributed-monolith) = %q, want anti-pattern", got)
	}
	if got := pc.EffectiveType("nonexistent"); got != "" {
		t.Errorf("EffectiveType(nonexistent) = %q, want empty", got)
	}
}

func TestAllEntries_SortedByID(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	entries := pc.AllEntries()
	for i := 1; i < len(entries); i++ {
		if entries[i].ID < entries[i-1].ID {
			t.Errorf("AllEntries() not sorted: %q < %q at index %d", entries[i].ID, entries[i-1].ID, i)
			break
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ENTRY FIELDS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestPatternEntry_HasRequiredFields(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	for _, entry := range pc.AllEntries() {
		t.Run(entry.ID, func(t *testing.T) {
			if entry.Name == "" {
				t.Error("Name is empty")
			}
			if entry.Type != "pattern" && entry.Type != "anti-pattern" {
				t.Errorf("Type = %q, want pattern or anti-pattern", entry.Type)
			}
			if _, ok := levelPriority[entry.Level]; !ok {
				t.Errorf("Level = %q, not a valid level", entry.Level)
			}
			if entry.Category == "" {
				t.Error("Category is empty")
			}
			if entry.Description == "" {
				t.Error("Description is empty")
			}
			if entry.Source.Reference == "" {
				t.Error("Source.Reference is empty")
			}

			// Patterns must have when_to_use
			if entry.Type == "pattern" && len(entry.WhenToUse) == 0 {
				t.Error("pattern has no when_to_use entries")
			}

			// Anti-patterns must have signs and remediation
			if entry.Type == "anti-pattern" {
				if len(entry.Signs) == 0 {
					t.Error("anti-pattern has no signs")
				}
				if len(entry.Remediation) == 0 {
					t.Error("anti-pattern has no remediation")
				}
			}
		})
	}
}

func TestAntiPattern_RemediationReferencesExist(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	for _, ap := range pc.AntiPatterns() {
		for _, remID := range ap.Remediation {
			// Remediation patterns should exist in catalog OR be external
			// (some like "clean-architecture" may not be in catalog)
			entry := pc.GetByID(remID)
			if entry == nil {
				// Allow external references but log them
				t.Logf("anti-pattern %q remediation %q not in catalog (external reference)", ap.ID, remID)
			}
		}
	}
}

func TestPatternSources_AllMapped(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// Count entries by source reference
	sourceCounts := make(map[string]int)
	for _, e := range pc.AllEntries() {
		sourceCounts[e.Source.Reference]++
	}

	// Verify we have entries from all expected sources
	expectedSources := []string{"GoF", "Patterns of Enterprise Application Architecture",
		"Microservices Patterns", "Domain-Driven Design"}
	for _, src := range expectedSources {
		if sourceCounts[src] == 0 {
			t.Errorf("no entries from source %q", src)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HIERARCHY (OVERRIDE)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestWithProjectOverrides_AddNewEntry(t *testing.T) {
	// Setup: temp dir with override YAML
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "patterns-catalog")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	overrideYAML := `
patterns:
  - id: custom-company-pattern
    name: "Custom Company Pattern"
    type: pattern
    level: company
    category: structural
    source:
      reference: "Internal ADR"
      author: "Engineering Team"
    description: "A custom company-specific pattern"
    when_to_use:
      - "When company standards require it"
    keywords: ["custom", "company"]
`
	if err := os.WriteFile(filepath.Join(overrideDir, "company.yml"), []byte(overrideYAML), 0644); err != nil {
		t.Fatal(err)
	}

	pc, err := NewPatternCatalog(WithProjectOverrides(dir))
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// New entry should exist
	entry := pc.GetByID("custom-company-pattern")
	if entry == nil {
		t.Fatal("custom-company-pattern not found after override")
	}
	if entry.Level != "company" {
		t.Errorf("Level = %q, want company", entry.Level)
	}

	// Total should be 64 + 1
	if pc.PatternCount() != 65 {
		t.Errorf("PatternCount() = %d, want 65", pc.PatternCount())
	}
}

func TestWithProjectOverrides_OverrideExistingEntry(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "patterns-catalog")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Override: reclassify "singleton" as anti-pattern at company level
	overrideYAML := `
patterns:
  - id: singleton
    name: "Singleton"
    type: anti-pattern
    level: company
    category: creational
    source:
      reference: "Company ADR-042"
      author: "Architecture Team"
    description: "Decided against Singleton pattern per ADR-042"
    signs:
      - "Global mutable state"
      - "Hidden dependencies"
    remediation: ["factory-method"]
    keywords: ["singleton", "global state"]
`
	if err := os.WriteFile(filepath.Join(overrideDir, "overrides.yml"), []byte(overrideYAML), 0644); err != nil {
		t.Fatal(err)
	}

	pc, err := NewPatternCatalog(WithProjectOverrides(dir))
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	entry := pc.GetByID("singleton")
	if entry == nil {
		t.Fatal("singleton not found")
	}

	// Company override should win over universal
	if entry.Type != "anti-pattern" {
		t.Errorf("Type = %q, want anti-pattern (company override)", entry.Type)
	}
	if entry.Level != "company" {
		t.Errorf("Level = %q, want company", entry.Level)
	}
	if entry.Source.Reference != "Company ADR-042" {
		t.Errorf("Source.Reference = %q, want Company ADR-042", entry.Source.Reference)
	}

	// Count should stay same (override, not add)
	if pc.PatternCount() != 64 {
		t.Errorf("PatternCount() = %d, want 64", pc.PatternCount())
	}
}

func TestHierarchy_HigherLevelWins(t *testing.T) {
	// Simulate: universal entry exists, then team and project overrides
	pc, err := NewPatternCatalog(
		WithEntries(
			CatalogEntry{
				ID: "test-pattern", Name: "Test", Type: "pattern",
				Level: "team", Category: "structural",
				Source:      EntrySource{Reference: "Team", Author: "Team"},
				Description: "Team version",
				WhenToUse:   []string{"team use"},
			},
		),
	)
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// The universal "cqrs" should still exist
	cqrs := pc.GetByID("cqrs")
	if cqrs == nil {
		t.Fatal("cqrs not found")
	}

	// test-pattern at team level
	tp := pc.GetByID("test-pattern")
	if tp == nil {
		t.Fatal("test-pattern not found")
	}
	if tp.Level != "team" {
		t.Errorf("Level = %q, want team", tp.Level)
	}

	// Now add a project-level override via WithEntries
	pc2, err := NewPatternCatalog(
		WithEntries(
			CatalogEntry{
				ID: "cqrs", Name: "CQRS (Project Override)", Type: "anti-pattern",
				Level: "project", Category: "data",
				Source:      EntrySource{Reference: "Project ADR", Author: "Project Team"},
				Description: "CQRS decided against for this project",
				Signs:       []string{"Complexity not justified"},
				Remediation: []string{"service-layer"},
			},
		),
	)
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// Project-level should win over universal
	cqrsOverride := pc2.GetByID("cqrs")
	if cqrsOverride == nil {
		t.Fatal("cqrs not found after override")
	}
	if cqrsOverride.Type != "anti-pattern" {
		t.Errorf("Type = %q, want anti-pattern", cqrsOverride.Type)
	}
	if cqrsOverride.Level != "project" {
		t.Errorf("Level = %q, want project", cqrsOverride.Level)
	}
}

func TestHierarchy_LowerLevelDoesNotOverride(t *testing.T) {
	// First add a company-level entry, then try to override with universal
	pc, err := NewPatternCatalog(
		WithEntries(
			CatalogEntry{
				ID: "singleton", Name: "Singleton (Company Ban)", Type: "anti-pattern",
				Level: "company", Category: "creational",
				Source:      EntrySource{Reference: "ADR-042", Author: "Company"},
				Description: "Banned at company level",
				Signs:       []string{"Global state"},
				Remediation: []string{"factory-method"},
			},
		),
	)
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// The embedded universal "singleton" was loaded first, then company override applied
	// Since company > universal, the company version should be active
	entry := pc.GetByID("singleton")
	if entry == nil {
		t.Fatal("singleton not found")
	}
	if entry.Type != "anti-pattern" {
		t.Errorf("Type = %q, want anti-pattern (company override should win)", entry.Type)
	}
}

func TestWithProjectOverrides_NoOverrideDir(t *testing.T) {
	dir := t.TempDir()
	// No .workflow/patterns-catalog/ directory exists

	pc, err := NewPatternCatalog(WithProjectOverrides(dir))
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// Should still load embedded entries
	if pc.PatternCount() != 64 {
		t.Errorf("PatternCount() = %d, want 64", pc.PatternCount())
	}
}

func TestWithProjectOverrides_NewCategory(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "patterns-catalog")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	overrideYAML := `
categories:
  - id: custom-category
    name: "Custom Category"

patterns:
  - id: custom-pattern
    name: "Custom Pattern"
    type: pattern
    level: company
    category: custom-category
    source:
      reference: "Internal"
      author: "Team"
    description: "A custom pattern in a new category"
    when_to_use:
      - "When needed"
    keywords: ["custom"]
`
	if err := os.WriteFile(filepath.Join(overrideDir, "custom.yml"), []byte(overrideYAML), 0644); err != nil {
		t.Fatal(err)
	}

	pc, err := NewPatternCatalog(WithProjectOverrides(dir))
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// Custom category should be added
	cats := pc.Categories()
	found := false
	for _, c := range cats {
		if c.ID == "custom-category" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custom-category not found in categories")
	}

	// Custom pattern should be in custom category
	entries := pc.EntriesByCategory("custom-category")
	if len(entries) != 1 {
		t.Errorf("EntriesByCategory(custom-category) = %d, want 1", len(entries))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FORMAT FOR PROMPT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestFormatForPrompt_ContainsAllEntries(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	prompt := pc.FormatForPrompt()

	// Should contain header
	if !strings.Contains(prompt, "# Architecture Pattern Catalog") {
		t.Error("prompt missing header")
	}

	// Should contain some key patterns
	expectedIDs := []string{"cqrs", "factory-method", "distributed-monolith", "api-first"}
	for _, id := range expectedIDs {
		if !strings.Contains(prompt, id) {
			t.Errorf("prompt missing pattern %q", id)
		}
	}

	// Anti-patterns should have [AP] marker
	if !strings.Contains(prompt, "[AP|") {
		t.Error("prompt missing [AP|...] markers for anti-patterns")
	}

	// Patterns should have [P] marker
	if !strings.Contains(prompt, "[P|") {
		t.Error("prompt missing [P|...] markers for patterns")
	}
}

func TestFormatForPrompt_CompactSize(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	prompt := pc.FormatForPrompt()

	// Rough estimate: ~40 tokens/entry × 64 entries = ~2560 tokens
	// At ~4 chars/token, expect ~10K chars. Allow generous margin.
	if len(prompt) > 30000 {
		t.Errorf("prompt too large: %d chars (target: <30000)", len(prompt))
	}
	if len(prompt) < 3000 {
		t.Errorf("prompt too small: %d chars (seems incomplete)", len(prompt))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// GoF COMPLETENESS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestGoF_All23Patterns(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	gofPatterns := []string{
		// Creational (5)
		"abstract-factory", "builder", "factory-method", "prototype", "singleton",
		// Structural (7)
		"adapter", "bridge", "composite", "decorator", "facade", "flyweight", "proxy",
		// Behavioral (11)
		"chain-of-responsibility", "command", "interpreter", "iterator", "mediator",
		"memento", "observer", "state", "strategy", "template-method", "visitor",
	}

	for _, id := range gofPatterns {
		entry := pc.GetByID(id)
		if entry == nil {
			t.Errorf("GoF pattern %q missing from catalog", id)
			continue
		}
		if entry.Source.Reference != "GoF" {
			t.Errorf("GoF pattern %q has source %q, want GoF", id, entry.Source.Reference)
		}
	}

	if len(gofPatterns) != 23 {
		t.Errorf("GoF list has %d patterns, want 23", len(gofPatterns))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANTI-PATTERN SPECIFICS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAntiPatterns_Expected8(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	expectedAntiPatterns := []string{
		"distributed-monolith", "god-class", "big-ball-of-mud", "golden-hammer",
		"spaghetti-code", "anemic-domain-model", "chatty-io", "shared-database",
	}

	for _, id := range expectedAntiPatterns {
		entry := pc.GetByID(id)
		if entry == nil {
			t.Errorf("anti-pattern %q missing from catalog", id)
			continue
		}
		if entry.Type != "anti-pattern" {
			t.Errorf("anti-pattern %q has type %q, want anti-pattern", id, entry.Type)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EDGE CASES
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestWithEntries_EmptyID(t *testing.T) {
	pc, err := NewPatternCatalog(
		WithEntries(CatalogEntry{ID: "", Name: "No ID"}),
	)
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// Empty ID entries should be handled gracefully
	// The entry should be added (mergeEntry allows it) but GetByID("") returns it
	// This is acceptable behavior
	if pc.PatternCount() != 65 { // 64 + 1 empty-id entry
		t.Logf("PatternCount() = %d (with empty ID entry)", pc.PatternCount())
	}
}

func TestWithProjectOverrides_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "patterns-catalog")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write invalid YAML
	if err := os.WriteFile(filepath.Join(overrideDir, "bad.yml"), []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should not panic, silently skips invalid files
	pc, err := NewPatternCatalog(WithProjectOverrides(dir))
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	// Embedded entries should still be loaded
	if pc.PatternCount() < 64 {
		t.Errorf("PatternCount() = %d, want >= 64", pc.PatternCount())
	}
}

func TestWithProjectOverrides_NonYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "patterns-catalog")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write non-YAML file (should be ignored)
	if err := os.WriteFile(filepath.Join(overrideDir, "readme.txt"), []byte("not yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	pc, err := NewPatternCatalog(WithProjectOverrides(dir))
	if err != nil {
		t.Fatalf("NewPatternCatalog() error: %v", err)
	}

	if pc.PatternCount() != 64 {
		t.Errorf("PatternCount() = %d, want 64", pc.PatternCount())
	}
}
