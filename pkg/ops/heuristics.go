package ops

import (
	"embed"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/heuristics/*.yml
var heuristicsFS embed.FS

// HeuristicRule describes one evaluable rule loaded from YAML.
type HeuristicRule struct {
	Name       string          `yaml:"name"`
	Conditions []HeuristicCond `yaml:"conditions"`
	Operator   string          `yaml:"operator"` // "AND" | "OR" (default: AND)
	Status     string          `yaml:"status"`   // "warn" | "critical"
	Signal     string          `yaml:"signal"`
	Actions    []string        `yaml:"actions"`
}

// HeuristicCond is a single threshold comparison against the data map.
type HeuristicCond struct {
	Field string  `yaml:"field"`
	Op    string  `yaml:"op"`    // ">=" | ">" | "<" | "<=" | "==" | "!="
	Value float64 `yaml:"value"`
}

type heuristicsFile struct {
	Heuristics []HeuristicRule `yaml:"heuristics"`
}

// loadHeuristics reads config/heuristics/<name>.yml via embed.FS.
// Returns nil slice on error (safe degradation — no false positives).
func loadHeuristics(name string) []HeuristicRule {
	data, err := heuristicsFS.ReadFile("config/heuristics/" + name + ".yml")
	if err != nil {
		return nil
	}
	var h heuristicsFile
	if err := yaml.Unmarshal(data, &h); err != nil {
		return nil
	}
	return h.Heuristics
}

// EvalHeuristics applies rules against data fields in order.
// Returns the first matched rule's status/signal/actions, or "ok"/""/nil if none match.
func EvalHeuristics(data map[string]any, rules []HeuristicRule) (status, signal string, actions []string) {
	for _, rule := range rules {
		if matchesRule(data, rule) {
			return rule.Status, applySignalTemplate(rule.Signal, data), rule.Actions
		}
	}
	return "ok", "", nil
}

// matchesRule checks whether a rule's conditions are satisfied by data.
func matchesRule(data map[string]any, rule HeuristicRule) bool {
	op := strings.ToUpper(rule.Operator)
	if op == "" {
		op = "AND"
	}
	for _, cond := range rule.Conditions {
		result := evalCond(data, cond)
		if op == "OR" && result {
			return true
		}
		if op == "AND" && !result {
			return false
		}
	}
	return op == "AND"
}

// evalCond evaluates a single condition against the data map.
func evalCond(data map[string]any, cond HeuristicCond) bool {
	raw, ok := data[cond.Field]
	if !ok {
		return false
	}
	var v float64
	switch x := raw.(type) {
	case float64:
		v = x
	case int:
		v = float64(x)
	case int64:
		v = float64(x)
	case int32:
		v = float64(x)
	case string:
		var err error
		v, err = strconv.ParseFloat(x, 64)
		if err != nil {
			return false
		}
	default:
		return false
	}
	switch cond.Op {
	case ">=":
		return v >= cond.Value
	case ">":
		return v > cond.Value
	case "<=":
		return v <= cond.Value
	case "<":
		return v < cond.Value
	case "==":
		return v == cond.Value
	case "!=":
		return v != cond.Value
	default:
		return false
	}
}

// applySignalTemplate interpolates {field} placeholders using values from data.
func applySignalTemplate(tmpl string, data map[string]any) string {
	result := tmpl
	for k, v := range data {
		placeholder := "{" + k + "}"
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
		}
	}
	return result
}
