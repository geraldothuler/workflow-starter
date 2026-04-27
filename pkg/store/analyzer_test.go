package store

import (
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

func TestAnalyzeHistory_WithMock(t *testing.T) {
	mock, err := llm.NewMockProviderFromConfig()
	if err != nil {
		t.Fatalf("NewMockProviderFromConfig: %v", err)
	}

	path := tmpLog(t)
	Append(path, "db-health", "warn", "WAL 400MB", "fusca")
	Append(path, "db-health", "warn", "WAL 450MB", "fusca")
	Append(path, "db-health", "warn", "WAL 503MB", "fusca")

	result, err := AnalyzeHistory(path, mock)
	if err != nil {
		t.Fatalf("AnalyzeHistory: %v", err)
	}
	if len(result.SuggestedRules) == 0 {
		t.Error("expected at least one suggested rule")
	}
	if len(result.Patterns) == 0 {
		t.Error("expected at least one pattern")
	}
	if result.Confidence == 0 {
		t.Error("expected non-zero confidence")
	}
	if result.DataPoints == 0 {
		t.Error("expected non-zero data_points")
	}
}

func TestAnalyzeHistory_EmptyDB(t *testing.T) {
	mock, _ := llm.NewMockProviderFromConfig()
	path := tmpLog(t)

	result, err := AnalyzeHistory(path, mock)
	if err != nil {
		t.Fatalf("AnalyzeHistory on empty DB: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for empty DB")
	}
	// No LLM call for empty DB — returns zero-value AnalysisResult
	if len(result.SuggestedRules) != 0 {
		t.Errorf("expected no rules for empty DB, got %d", len(result.SuggestedRules))
	}
}

func TestAnalyzeHistory_SuggestedRulesMatchTrendRuleFormat(t *testing.T) {
	mock, _ := llm.NewMockProviderFromConfig()
	path := tmpLog(t)
	Append(path, "airbyte", "warn", "sync slow", "data-platform")

	result, err := AnalyzeHistory(path, mock)
	if err != nil {
		t.Fatalf("AnalyzeHistory: %v", err)
	}
	for i, sr := range result.SuggestedRules {
		if sr.Probe == "" {
			t.Errorf("rule[%d] missing probe", i)
		}
		if sr.Window == 0 {
			t.Errorf("rule[%d] missing window", i)
		}
		if sr.ConsecutiveStatus == "" {
			t.Errorf("rule[%d] missing consecutive_status", i)
		}
		tr := sr.ToTrendRule()
		if tr.Probe != sr.Probe || tr.Window != sr.Window {
			t.Errorf("rule[%d] ToTrendRule mismatch", i)
		}
	}
}

func TestSuggestedRule_ToTrendRule_DropsRationale(t *testing.T) {
	sr := SuggestedRule{
		Probe: "db-health", Window: 3,
		ConsecutiveStatus: "warn", EscalateTo: "critical",
		Signal: "test signal", Rationale: "test rationale",
	}
	tr := sr.ToTrendRule()
	if tr.Probe != sr.Probe || tr.Window != sr.Window || tr.Signal != sr.Signal {
		t.Errorf("ToTrendRule field mismatch: %+v", tr)
	}
	// TrendRule has no Rationale — compilation enforces this
}

func TestBuildAnalysisPrompt_ContainsSugerirRegras(t *testing.T) {
	records := []Record{
		{Ts: "2026-02-25T00:00:00Z", Probe: "db-health", Status: "warn", Signal: "WAL 400MB", Repo: "fusca"},
	}
	prompt := buildAnalysisPrompt(records)
	if !strings.Contains(prompt, "sugerir_regras") {
		t.Error("prompt missing 'sugerir_regras' keyword — mock router will not route to heuristic_analysis")
	}
	if !strings.Contains(prompt, "db-health") {
		t.Error("prompt missing record data")
	}
	if !strings.Contains(prompt, "suggested_rules") {
		t.Error("prompt missing output format hint")
	}
}

func TestBuildAnalysisPrompt_TruncatesTimestamp(t *testing.T) {
	records := []Record{
		{Ts: "2026-02-25T12:34:56Z", Probe: "airbyte", Status: "ok", Signal: "syncs ok", Repo: "r"},
	}
	prompt := buildAnalysisPrompt(records)
	// Timestamp truncated to 16 chars: "2026-02-25T12:34"
	if !strings.Contains(prompt, "2026-02-25T12:34") {
		t.Error("expected truncated timestamp in prompt")
	}
}
