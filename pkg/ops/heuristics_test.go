package ops

import "testing"

// 1. AND conditions — ambas verdadeiras → match
func TestEvalHeuristics_AndBothTrue(t *testing.T) {
	rules := []HeuristicRule{
		{
			Name: "test_rule",
			Conditions: []HeuristicCond{
				{Field: "count", Op: ">=", Value: 1},
				{Field: "attempts", Op: ">", Value: 2},
			},
			Operator: "AND",
			Status:   "warn",
			Signal:   "test signal",
		},
	}
	data := map[string]any{"count": 2, "attempts": 3}
	status, signal, _ := EvalHeuristics(data, rules)
	if status != "warn" {
		t.Errorf("expected warn, got %q", status)
	}
	if signal != "test signal" {
		t.Errorf("expected 'test signal', got %q", signal)
	}
}

// 2. AND conditions — uma condição falsa → no match
func TestEvalHeuristics_AndOneFalse(t *testing.T) {
	rules := []HeuristicRule{
		{
			Name: "test_rule",
			Conditions: []HeuristicCond{
				{Field: "count", Op: ">=", Value: 1},
				{Field: "attempts", Op: ">", Value: 2},
			},
			Operator: "AND",
			Status:   "warn",
		},
	}
	data := map[string]any{"count": 2, "attempts": 1} // attempts=1, não > 2
	status, _, _ := EvalHeuristics(data, rules)
	if status != "ok" {
		t.Errorf("expected ok (one false condition), got %q", status)
	}
}

// 3. OR conditions — uma verdadeira → match
func TestEvalHeuristics_OrOneTrue(t *testing.T) {
	rules := []HeuristicRule{
		{
			Name: "test_rule",
			Conditions: []HeuristicCond{
				{Field: "count", Op: ">=", Value: 10}, // false (count=2)
				{Field: "attempts", Op: ">", Value: 2}, // true (attempts=3)
			},
			Operator: "OR",
			Status:   "critical",
		},
	}
	data := map[string]any{"count": 2, "attempts": 3}
	status, _, _ := EvalHeuristics(data, rules)
	if status != "critical" {
		t.Errorf("expected critical (OR one true), got %q", status)
	}
}

// 4. Template interpolation → signal com valores corretos
func TestEvalHeuristics_SignalTemplate(t *testing.T) {
	rules := []HeuristicRule{
		{
			Name:       "test_rule",
			Conditions: []HeuristicCond{{Field: "count", Op: ">=", Value: 1}},
			Status:     "warn",
			Signal:     "found {count} items in {namespace}",
		},
	}
	data := map[string]any{"count": 5, "namespace": "data-platform"}
	_, signal, _ := EvalHeuristics(data, rules)
	if signal != "found 5 items in data-platform" {
		t.Errorf("unexpected signal: %q", signal)
	}
}

// 5. No rules match → status ok, signal vazio, actions nil
func TestEvalHeuristics_NoMatch(t *testing.T) {
	rules := []HeuristicRule{
		{
			Name:       "test_rule",
			Conditions: []HeuristicCond{{Field: "count", Op: ">=", Value: 10}},
			Status:     "warn",
		},
	}
	data := map[string]any{"count": 2}
	status, signal, actions := EvalHeuristics(data, rules)
	if status != "ok" {
		t.Errorf("expected ok, got %q", status)
	}
	if signal != "" {
		t.Errorf("expected empty signal, got %q", signal)
	}
	if actions != nil {
		t.Errorf("expected nil actions, got %v", actions)
	}
}
