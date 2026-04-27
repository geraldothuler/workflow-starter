package render

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_BasicOutput(t *testing.T) {
	data := &LensData{
		Meta: MetaData{
			Title:    "Test Backlog",
			Subtitle: "Test subtitle",
		},
		KPIs: []KPI{
			{Label: "Epicos", Value: 2, Sub: "total"},
		},
		Epics: map[string]EpicLens{
			"E1": {
				ID: "E1", Code: "E1", Title: "Epic One",
				Summary:  "First epic",
				Priority: "high",
				Stories: []StoryLens{
					{Code: "E1.1", Title: "Story One", Effort: 3, RiskLabel: "low"},
				},
			},
		},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(md, "# Test Backlog") {
		t.Error("expected title in markdown")
	}
	if !strings.Contains(md, "Epic One") {
		t.Error("expected epic title in markdown")
	}
	if !strings.Contains(md, "Story One") {
		t.Error("expected story title in markdown")
	}
	if !strings.Contains(md, "Epicos") {
		t.Error("expected KPI in markdown")
	}
}

func TestRenderMarkdown_WithDeepDives(t *testing.T) {
	data := &LensData{
		Meta: MetaData{Title: "Test"},
		DeepDives: map[string]DeepDiveLens{
			"PostgreSQL": {
				Term:           "PostgreSQL",
				WhatIs:         "A relational database",
				WhyHere:        "Used for data storage",
				Classification: "standard",
				Scope:          "global",
				Patterns:       []string{"Connection pooling"},
				Decisions:      []string{"Use pgx driver"},
			},
		},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(md, "Tech Refs") {
		t.Error("expected Tech Refs section")
	}
	if !strings.Contains(md, "PostgreSQL") {
		t.Error("expected PostgreSQL in deep dives")
	}
	if !strings.Contains(md, "Connection pooling") {
		t.Error("expected pattern in deep dives")
	}
}

func TestRenderMarkdown_WithMilestones(t *testing.T) {
	data := &LensData{
		Meta: MetaData{Title: "Test"},
		Milestones: []Milestone{
			{
				Title:        "MVP",
				TotalSPs:     20,
				DaysEstimate: 10,
				ValueProp:    "Core features",
				EpicIDs:      []string{"E1", "E2"},
			},
		},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(md, "Milestones") {
		t.Error("expected Milestones section")
	}
	if !strings.Contains(md, "MVP") {
		t.Error("expected milestone title")
	}
	if !strings.Contains(md, "Core features") {
		t.Error("expected value prop in milestone")
	}
}

func TestRenderMarkdown_WithEffortPerEpic(t *testing.T) {
	data := &LensData{
		Meta: MetaData{Title: "Effort Test"},
		Epics: map[string]EpicLens{
			"E1": {ID: "E1", Code: "E1", Title: "Auth Module", Stories: []StoryLens{
				{Code: "E1.1", Title: "Login", Effort: 3, RiskLabel: "low"},
				{Code: "E1.2", Title: "OAuth", Effort: 5, RiskLabel: "medium"},
			}},
			"E2": {ID: "E2", Code: "E2", Title: "Payment Gateway", Stories: []StoryLens{
				{Code: "E2.1", Title: "Stripe Integration", Effort: 8, RiskLabel: "high"},
			}},
		},
		Effort: EffortSummary{
			TotalStories:   3,
			TotalSPs:       16,
			TotalDays:       8,
			OptimisticDays:  8,
			RealisticDays:  12,
			Velocity:       20,
			ByEpic: map[string]EpicEffort{
				"E1": {EpicID: "E1", Stories: 2, SPs: 8, Days: 4, Percentage: 50.0},
				"E2": {EpicID: "E2", Stories: 1, SPs: 8, Days: 4, Percentage: 50.0},
			},
		},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have the per-epic table header
	if !strings.Contains(md, "Esforco por Epico") {
		t.Error("expected 'Esforco por Epico' sub-section")
	}

	// Should have epic titles in the table
	if !strings.Contains(md, "Auth Module") {
		t.Error("expected epic title 'Auth Module' in effort table")
	}
	if !strings.Contains(md, "Payment Gateway") {
		t.Error("expected epic title 'Payment Gateway' in effort table")
	}

	// Should have percentage
	if !strings.Contains(md, "50%") {
		t.Error("expected percentage '50%' in effort table")
	}
}

func TestRenderMarkdown_EffortPerEpicAbsentWhenEmpty(t *testing.T) {
	data := &LensData{
		Meta:   MetaData{Title: "No Effort"},
		Epics:  map[string]EpicLens{},
		Effort: EffortSummary{ByEpic: map[string]EpicEffort{}},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(md, "Esforco por Epico") {
		t.Error("should NOT contain 'Esforco por Epico' when no effort data")
	}
}

func TestRenderMarkdown_WithCriticalPathReport(t *testing.T) {
	data := &LensData{
		Meta:  MetaData{Title: "Critical Path Test"},
		Epics: map[string]EpicLens{},
		CriticalPathReport: &CriticalPathReportLens{
			Summary: "Foundation epics must be completed first",
			Phases: []ExecutionPhaseLens{
				{Phase: 1, EpicCodes: []string{"E1"}, Parallel: false, TotalEffort: 10, Reasoning: "Foundation phase"},
				{Phase: 2, EpicCodes: []string{"E2", "E3"}, Parallel: true, TotalEffort: 16, Reasoning: "Parallel development"},
			},
			Items: []CriticalPathItemLens{
				{
					EpicCode:     "E1",
					EpicTitle:    "Core Infrastructure",
					Phase:        1,
					Priority:     1,
					Reasoning:    "Provides base for all other epics",
					IsFoundation: true,
					DependsOn:    []string{},
				},
				{
					EpicCode:  "E2",
					EpicTitle: "User Management",
					Phase:     2,
					Priority:  2,
					Reasoning: "Depends on core infrastructure",
					DependsOn: []string{"E1"},
				},
			},
			Dependencies: []DependencyEdgeLens{
				{From: "E1", To: "E2", Type: "structural", Confidence: 90.0, Reasoning: "E2 uses E1 services"},
			},
		},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(md, "Caminho Critico") {
		t.Error("expected 'Caminho Critico' section")
	}
	if !strings.Contains(md, "Foundation epics must be completed first") {
		t.Error("expected critical path summary")
	}
	if !strings.Contains(md, "Fases de Execucao") {
		t.Error("expected 'Fases de Execucao' sub-section")
	}
	if !strings.Contains(md, "Foundation phase") {
		t.Error("expected phase reasoning")
	}
	if !strings.Contains(md, "Parallel development") {
		t.Error("expected parallel phase reasoning")
	}
	if !strings.Contains(md, "Core Infrastructure") {
		t.Error("expected critical path item epic title")
	}
	if !strings.Contains(md, "Fundacao") {
		t.Error("expected foundation marker for E1")
	}
	if !strings.Contains(md, "Depende de: E1") {
		t.Error("expected dependency reference for E2")
	}
	if !strings.Contains(md, "Dependencias Inferidas") {
		t.Error("expected 'Dependencias Inferidas' sub-section")
	}
	if !strings.Contains(md, "90%") {
		t.Error("expected confidence percentage in dependencies table")
	}
}

func TestRenderMarkdown_CriticalPathAbsentWhenNil(t *testing.T) {
	data := &LensData{
		Meta:               MetaData{Title: "No Critical Path"},
		Epics:              map[string]EpicLens{},
		CriticalPathReport: nil,
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(md, "Caminho Critico") {
		t.Error("should NOT contain 'Caminho Critico' when report is nil")
	}
}

func TestRenderMarkdown_WithPatternSuggestions(t *testing.T) {
	data := &LensData{
		Meta:  MetaData{Title: "Patterns Test"},
		Epics: map[string]EpicLens{},
		PatternSuggestions: []PatternSuggestionLens{
			{
				PatternID:     "CQRS",
				PatternName:   "CQRS",
				Type:          "architectural",
				Confidence:    0.85,
				Reasoning:     "Separate read and write models for scalability",
				AffectedEpics: []string{"E1", "E3"},
				Category:      "design",
			},
			{
				PatternID:   "circuit-breaker",
				PatternName: "Circuit Breaker",
				Type:        "resilience",
				Confidence:  0.70,
				Reasoning:   "Protect against cascading failures in microservices",
				Category:    "infrastructure",
			},
		},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(md, "Sugestoes de Patterns") {
		t.Error("expected 'Sugestoes de Patterns' section")
	}
	if !strings.Contains(md, "CQRS") {
		t.Error("expected pattern name 'CQRS'")
	}
	if !strings.Contains(md, "Circuit Breaker") {
		t.Error("expected pattern name 'Circuit Breaker'")
	}
	if !strings.Contains(md, "Separate read and write models") {
		t.Error("expected CQRS reasoning")
	}
	if !strings.Contains(md, "Epicos afetados: E1, E3") {
		t.Error("expected affected epics for CQRS")
	}
	// Circuit Breaker has no affected epics, so that sub-line should be absent for it
	if !strings.Contains(md, "`resilience`") {
		t.Error("expected pattern type 'resilience'")
	}
}

func TestRenderMarkdown_PatternSuggestionsAbsentWhenEmpty(t *testing.T) {
	data := &LensData{
		Meta:               MetaData{Title: "No Patterns"},
		Epics:              map[string]EpicLens{},
		PatternSuggestions: []PatternSuggestionLens{},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(md, "Sugestoes de Patterns") {
		t.Error("should NOT contain 'Sugestoes de Patterns' when list is empty")
	}
}

func TestRenderMarkdown_EmptyData(t *testing.T) {
	data := &LensData{
		Meta:      MetaData{Title: "Empty"},
		Epics:     map[string]EpicLens{},
		DeepDives: map[string]DeepDiveLens{},
	}

	md, err := RenderMarkdown(data)
	if err != nil {
		t.Fatalf("unexpected error on empty data: %v", err)
	}

	if !strings.Contains(md, "# Empty") {
		t.Error("expected title even on empty data")
	}
}
