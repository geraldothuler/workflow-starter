package ops

import (
	"testing"
)

func TestCheckMonteCarlo_MissingCredentials(t *testing.T) {
	r := CheckMonteCarlo(MonteCarloConfig{})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckMonteCarlo_BreachToday(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{
		"data": {
			"getCustomRuleExecutionAnalytics": [
				{"date": "2026-03-05", "runs": 12, "passes": 3, "breaches": 9},
				{"date": "2026-03-06", "runs": 6, "passes": 0, "breaches": 6}
			]
		}
	}`)

	r := CheckMonteCarlo(MonteCarloConfig{
		APIKey: "key", APIToken: "tok",
		Vars: map[string]string{"RuleUUID": "3296a0a8-3941-4779-bd53-145eda2520b9"},
	})
	if r.Status != "critical" {
		t.Errorf("expected critical, got %q", r.Status)
	}
}

func TestCheckMonteCarlo_NoBreaches(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{
		"data": {
			"getCustomRuleExecutionAnalytics": [
				{"date": "2026-03-05", "runs": 12, "passes": 12, "breaches": 0},
				{"date": "2026-03-06", "runs": 6, "passes": 6, "breaches": 0}
			]
		}
	}`)

	r := CheckMonteCarlo(MonteCarloConfig{
		APIKey: "key", APIToken: "tok",
		Vars: map[string]string{"RuleUUID": "3296a0a8-3941-4779-bd53-145eda2520b9"},
	})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

func TestCheckMonteCarlo_API500(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(500, `error`)
	r := CheckMonteCarlo(MonteCarloConfig{
		APIKey: "key", APIToken: "tok",
		Vars: map[string]string{"RuleUUID": "3296a0a8-3941-4779-bd53-145eda2520b9"},
	})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckMonteCarlo_UnstableRule(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	// Passou hoje mas teve muitos breaches recentes
	httpDo = mockHTTPResponse(200, `{
		"data": {
			"getCustomRuleExecutionAnalytics": [
				{"date": "2026-03-04", "runs": 12, "passes": 4, "breaches": 8},
				{"date": "2026-03-05", "runs": 12, "passes": 4, "breaches": 8},
				{"date": "2026-03-06", "runs": 6, "passes": 6, "breaches": 0}
			]
		}
	}`)

	r := CheckMonteCarlo(MonteCarloConfig{
		APIKey: "key", APIToken: "tok",
		Vars: map[string]string{"RuleUUID": "3296a0a8-3941-4779-bd53-145eda2520b9"},
	})
	if r.Status != "warn" {
		t.Errorf("expected warn, got %q", r.Status)
	}
}

// ── Path extractor unit tests ──────────────────────────────────────────────────

func TestMCExtractNumeric_LastElement(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"breaches": float64(9)},
			map[string]any{"breaches": float64(6)},
		},
	}
	v, ok := mcExtractNumeric(data, "results[-1].breaches")
	if !ok || v != 6 {
		t.Errorf("expected 6, got %v (ok=%v)", v, ok)
	}
}

func TestMCExtractNumeric_Sum(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"breaches": float64(9)},
			map[string]any{"breaches": float64(6)},
			map[string]any{"breaches": float64(0)},
		},
	}
	v, ok := mcExtractNumeric(data, "results[*].breaches|sum")
	if !ok || v != 15 {
		t.Errorf("expected 15, got %v (ok=%v)", v, ok)
	}
}
