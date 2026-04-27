package playbook

import (
	"strings"
	"testing"
	"time"
)

func TestRenderMarkdown_EmptyReport(t *testing.T) {
	report := &InvestigationReport{
		PlaybookID:    "test",
		PlaybookTitle: "Test Report",
		StartedAt:     time.Now(),
		CompletedAt:   time.Now(),
		Duration:      100 * time.Millisecond,
		Summary:       "No issues found. All checks passed.",
	}
	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test Report",
	}

	md := RenderMarkdown(report, spec)
	if !strings.Contains(md, "# Test Report") {
		t.Error("should contain title")
	}
	if !strings.Contains(md, "No issues found") {
		t.Error("should contain summary")
	}
	if !strings.Contains(md, "Executive Summary") {
		t.Error("should contain Executive Summary section")
	}
}

func TestRenderMarkdown_WithFindings(t *testing.T) {
	report := &InvestigationReport{
		PlaybookID:    "test",
		PlaybookTitle: "CDC Investigation",
		StartedAt:     time.Now(),
		CompletedAt:   time.Now(),
		Duration:      2 * time.Second,
		StepsExecuted: 2,
		Summary:       "Found 2 issue(s): 1 critical, 1 warning",
		Findings: []Finding{
			{
				ID:             "f1",
				StepID:         "s1",
				AnalyzerName:   "analyze_inactive_slots",
				Severity:       SeverityCritical,
				Title:          "Inactive slot: debezium_slot",
				Detail:         "Replication slot is inactive",
				Evidence:       "slot=debezium_slot, status=unhealthy",
				Recommendation: "Check consumer process",
			},
			{
				ID:             "f2",
				StepID:         "s1",
				AnalyzerName:   "analyze_wal_lag",
				Severity:       SeverityWarning,
				Title:          "WAL lag on sender1",
				Detail:         "200MB replay lag",
				Evidence:       "sender=sender1, lag=200MB",
				Recommendation: "Investigate throughput",
			},
		},
		StepResults: []StepResult{
			{StepID: "s1", Title: "Check slots", Provider: "postgresql", Status: StepStatusSuccess, Findings: []string{"f1", "f2"}, Duration: time.Second},
			{StepID: "s2", Title: "Check Kafka", Provider: "kafka", Status: StepStatusSkipped, Duration: 0},
		},
	}

	md := RenderMarkdown(report, &PlaybookSpec{ID: "test", Title: "CDC Investigation"})

	// Should have findings section
	if !strings.Contains(md, "## Findings") {
		t.Error("should contain Findings section")
	}
	if !strings.Contains(md, "Inactive slot") {
		t.Error("should contain finding title")
	}
	if !strings.Contains(md, "**Evidence:**") {
		t.Error("should contain evidence")
	}
	if !strings.Contains(md, "**Recommendation:**") {
		t.Error("should contain recommendation")
	}
	// Severity headers
	if !strings.Contains(md, "### Critical") {
		t.Error("should have Critical section")
	}
	if !strings.Contains(md, "### Warning") {
		t.Error("should have Warning section")
	}
	// Recommendations section
	if !strings.Contains(md, "## Recommendations") {
		t.Error("should contain Recommendations section")
	}
}

func TestRenderMarkdown_WithCausalChain(t *testing.T) {
	report := &InvestigationReport{
		PlaybookID:    "test",
		PlaybookTitle: "Test",
		StartedAt:     time.Now(),
		CompletedAt:   time.Now(),
		Duration:      time.Second,
		StepsExecuted: 1,
		Summary:       "Found issues",
		Findings: []Finding{
			{ID: "f1", AnalyzerName: "a1", Severity: SeverityCritical, Title: "Cause"},
			{ID: "f2", AnalyzerName: "a2", Severity: SeverityCritical, Title: "Effect"},
		},
		CausalChain: []CausalLink{
			{From: "f1", To: "f2", Reasoning: "Cause leads to effect"},
		},
	}

	md := RenderMarkdown(report, &PlaybookSpec{ID: "test", Title: "Test"})

	if !strings.Contains(md, "## Causal Chain") {
		t.Error("should contain Causal Chain section")
	}
	if !strings.Contains(md, "Cause leads to effect") {
		t.Error("should contain reasoning")
	}
}

func TestRenderMarkdown_StepTable(t *testing.T) {
	report := &InvestigationReport{
		PlaybookID:    "test",
		PlaybookTitle: "Test",
		StartedAt:     time.Now(),
		CompletedAt:   time.Now(),
		Duration:      time.Second,
		Summary:       "OK",
		StepResults: []StepResult{
			{StepID: "s1", Title: "Check A", Provider: "postgresql", Status: StepStatusSuccess, Findings: []string{"f1"}, Duration: 500 * time.Millisecond},
			{StepID: "s2", Title: "Check B", Provider: "kafka", Status: StepStatusSkipped, Error: "not available", Duration: 0},
		},
	}

	md := RenderMarkdown(report, &PlaybookSpec{ID: "test", Title: "Test"})

	if !strings.Contains(md, "## Investigation Steps") {
		t.Error("should contain Investigation Steps section")
	}
	if !strings.Contains(md, "| Check A |") {
		t.Error("should contain step A row")
	}
	if !strings.Contains(md, "| Check B |") {
		t.Error("should contain step B row")
	}
	if !strings.Contains(md, "skipped") {
		t.Error("should show skipped status")
	}
}

func TestRenderMarkdown_TitleTemplate(t *testing.T) {
	report := &InvestigationReport{
		PlaybookID:    "test",
		PlaybookTitle: "Default Title",
		StartedAt:     time.Now(),
		CompletedAt:   time.Now(),
		Duration:      time.Second,
		Summary:       "OK",
	}
	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Default Title",
		Report: ReportSpec{
			TitleTemplate: "Custom Investigation Title",
		},
	}

	md := RenderMarkdown(report, spec)
	if !strings.Contains(md, "# Custom Investigation Title") {
		t.Error("should use title_template from spec")
	}
}
