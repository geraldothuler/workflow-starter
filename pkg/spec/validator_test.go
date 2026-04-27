package spec

import (
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator(&types.GoldenPath{})
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestValidate_CompleteSpec(t *testing.T) {
	v := NewValidator(&types.GoldenPath{})
	input := &types.ProjectInput{
		Context:   "Plataforma de gestão de frota IoT",
		Volumetry: "500k dispositivos, 10M eventos/dia",
		NFRs:      "P99 < 200ms, 99.9% uptime",
		Stack:     "Go + Kafka + ScyllaDB",
	}

	result := v.Validate(input)
	if !result.Valid {
		t.Error("expected valid for complete spec")
	}
	if result.Score != 100 {
		t.Errorf("expected score 100, got %d", result.Score)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

func TestValidate_MissingContext(t *testing.T) {
	v := NewValidator(&types.GoldenPath{})
	input := &types.ProjectInput{
		Volumetry: "500k",
		NFRs:      "P99 200ms",
		Stack:     "Go",
	}

	result := v.Validate(input)
	if result.Valid {
		t.Error("expected invalid without context")
	}
	if result.Score >= 100 {
		t.Errorf("expected score < 100, got %d", result.Score)
	}
}

func TestValidate_MissingAll(t *testing.T) {
	v := NewValidator(&types.GoldenPath{})
	input := &types.ProjectInput{}

	result := v.Validate(input)
	if result.Valid {
		t.Error("expected invalid")
	}
	// Missing context(-20), volumetry(-20), stack(-20), NFRs(-10 warning) = 100-70=30
	if result.Score != 30 {
		t.Errorf("expected score 30, got %d", result.Score)
	}
	if len(result.Errors) < 3 {
		t.Errorf("expected at least 3 errors, got %d", len(result.Errors))
	}
}

func TestValidate_MissingNFRs_Warning(t *testing.T) {
	v := NewValidator(&types.GoldenPath{})
	input := &types.ProjectInput{
		Context:   "Something",
		Volumetry: "100k",
		Stack:     "Go",
	}

	result := v.Validate(input)
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for missing NFRs")
	}
}

func TestNewGenerator(t *testing.T) {
	spec := &types.Specification{
		Epics: []types.Epic{{Title: "E1"}},
	}
	g := NewGenerator(spec, &types.GoldenPath{}, &types.TeamPatterns{}, &types.ProjectInput{})
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
}

func TestGenerate(t *testing.T) {
	spec := &types.Specification{
		Epics: []types.Epic{{Title: "Test Epic"}},
	}
	g := NewGenerator(spec, &types.GoldenPath{}, &types.TeamPatterns{}, &types.ProjectInput{})

	result, err := g.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Epics) != 1 || result.Epics[0].Title != "Test Epic" {
		t.Error("expected spec returned as-is")
	}
}

func TestGenerate_NilMerged(t *testing.T) {
	g := NewGenerator(nil, &types.GoldenPath{}, &types.TeamPatterns{}, &types.ProjectInput{})

	_, err := g.Generate()
	if err == nil {
		t.Error("expected error for nil merged spec")
	}
}

func TestGenerate_EnrichesStackDecisions(t *testing.T) {
	spec := &types.Specification{
		Epics: []types.Epic{{Title: "E1"}},
	}
	pi := &types.ProjectInput{
		Stack: "Go + Kafka",
	}
	g := NewGenerator(spec, &types.GoldenPath{}, &types.TeamPatterns{}, pi)

	result, err := g.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StackDecisions == nil {
		t.Error("expected stack decisions to be enriched from input")
	}
	if result.StackDecisions["stack"] != "Go + Kafka" {
		t.Errorf("expected stack 'Go + Kafka', got %v", result.StackDecisions["stack"])
	}
}

func TestGenerate_PreservesExistingStackDecisions(t *testing.T) {
	spec := &types.Specification{
		StackDecisions: map[string]interface{}{
			"backend": "Go",
		},
	}
	pi := &types.ProjectInput{Stack: "Python"}
	g := NewGenerator(spec, &types.GoldenPath{}, &types.TeamPatterns{}, pi)

	result, err := g.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StackDecisions["backend"] != "Go" {
		t.Error("should preserve existing stack decisions")
	}
}

func TestReport_CompleteSpec(t *testing.T) {
	spec := &types.Specification{
		Epics:          []types.Epic{{Title: "E1"}, {Title: "E2"}},
		StackDecisions: map[string]interface{}{"backend": "Go"},
	}
	pi := &types.ProjectInput{
		Context:       "IoT platform",
		Volumetry:     "500k devices",
		NFRs:          "P99 < 200ms",
		Stack:         "Go",
		BusinessRules: "Rule 1",
	}
	g := NewGenerator(spec, &types.GoldenPath{}, &types.TeamPatterns{}, pi)

	report := g.Report()
	if report.Score < 80 {
		t.Errorf("expected high score for complete spec, got %d", report.Score)
	}
	if len(report.Sections) < 3 {
		t.Errorf("expected at least 3 sections, got %d", len(report.Sections))
	}
}

func TestReport_EmptySpec(t *testing.T) {
	spec := &types.Specification{}
	g := NewGenerator(spec, &types.GoldenPath{}, &types.TeamPatterns{}, &types.ProjectInput{})

	report := g.Report()
	if report.Score >= 80 {
		t.Errorf("expected low score for empty spec, got %d", report.Score)
	}
	if len(report.Recommendations) == 0 {
		t.Error("expected recommendations for empty spec")
	}
}

func TestReport_NilMerged(t *testing.T) {
	g := NewGenerator(nil, &types.GoldenPath{}, &types.TeamPatterns{}, &types.ProjectInput{})

	report := g.Report()
	if report.Score != 0 {
		t.Errorf("expected score 0 for nil spec, got %d", report.Score)
	}
}

func TestReport_WithGaps(t *testing.T) {
	spec := &types.Specification{
		Epics: []types.Epic{{Title: "E1"}},
		Gaps: []types.Gap{
			{Type: "missing_nfr", Severity: "high"},
			{Type: "missing_stack", Severity: "medium"},
		},
		StackDecisions: map[string]interface{}{"backend": "Go"},
	}
	pi := &types.ProjectInput{
		Context:   "System",
		Volumetry: "100k",
		NFRs:      "P99 200ms",
		Stack:     "Go",
	}
	g := NewGenerator(spec, &types.GoldenPath{}, &types.TeamPatterns{}, pi)

	report := g.Report()
	foundGapRec := false
	for _, rec := range report.Recommendations {
		if strings.Contains(rec, "gaps") {
			foundGapRec = true
		}
	}
	if !foundGapRec {
		t.Error("expected recommendation about pending gaps")
	}
}

func TestFormatReport(t *testing.T) {
	report := &SpecReport{
		Score: 75,
		Sections: []SectionScore{
			{Name: "Epics", Score: 100, Items: 3},
			{Name: "Gaps", Score: 60, Items: 2},
		},
		Recommendations: []string{"Resolver gaps", "Adicionar stack"},
	}

	output := report.FormatReport()

	checks := []string{
		"Score Geral:** 75/100",
		"| Epics | 100/100 | 3 |",
		"| Gaps | 60/100 | 2 |",
		"- Resolver gaps",
		"- Adicionar stack",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected report to contain %q", check)
		}
	}
}

func TestEvaluateInput(t *testing.T) {
	// Complete input
	complete := &types.ProjectInput{
		Context:       "IoT",
		Volumetry:     "500k",
		NFRs:          "P99 200ms",
		Stack:         "Go",
		BusinessRules: "Rule 1",
	}
	if evaluateInput(complete) != 100 {
		t.Errorf("expected 100 for complete input, got %d", evaluateInput(complete))
	}

	// Empty input
	empty := &types.ProjectInput{}
	score := evaluateInput(empty)
	if score != 0 {
		t.Errorf("expected 0 for empty input, got %d", score)
	}
}
