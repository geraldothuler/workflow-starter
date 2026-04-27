package ops

import (
	"embed"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/investigation/*.yml
var investigationFS embed.FS

// InvestigationStrategy captures when and how to apply an investigation approach.
// Encodes successful patterns (prefer, steps) and failed anti-patterns (avoid)
// learned from real incidents. LLMUsage declares the target token consumption.
type InvestigationStrategy struct {
	Name        string                `yaml:"name"`
	Description string                `yaml:"description"`
	Triggers    []StrategyCond        `yaml:"triggers"`
	Operator    string                `yaml:"operator"`  // "AND" | "OR" (default: AND)
	LLMUsage    string                `yaml:"llm_usage"` // "none" | "minimal" | "required"
	Prefer      StrategyApproach      `yaml:"prefer"`
	Avoid       []StrategyAntipattern `yaml:"avoid"`
	Steps       []string              `yaml:"steps"`
	Evidence    string                `yaml:"evidence"`
}

// StrategyCond is a trigger condition supporting both numeric (value) and
// string (string_value) comparisons against the investigation context map.
type StrategyCond struct {
	Field       string  `yaml:"field"`
	Op          string  `yaml:"op"`                      // ">=" | ">" | "<" | "<=" | "==" | "!="
	Value       float64 `yaml:"value,omitempty"`
	StringValue string  `yaml:"string_value,omitempty"`
}

// StrategyApproach describes the recommended tool and execution pattern.
type StrategyApproach struct {
	Tool    string `yaml:"tool"`
	Pattern string `yaml:"pattern"`
}

// StrategyAntipattern records a tool/approach that should be avoided and why.
type StrategyAntipattern struct {
	Tool   string `yaml:"tool"`
	Reason string `yaml:"reason"`
}

type strategiesFile struct {
	Strategies []InvestigationStrategy `yaml:"strategies"`
}

// LoadStrategies reads config/investigation/<name>.yml via embed.FS.
// Returns nil on error (safe degradation).
func LoadStrategies(name string) []InvestigationStrategy {
	data, err := investigationFS.ReadFile("config/investigation/" + name + ".yml")
	if err != nil {
		return nil
	}
	var f strategiesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil
	}
	return f.Strategies
}

// SelectStrategies returns all strategies whose triggers match context data,
// sorted by llm_usage ascending (none → minimal → required).
func SelectStrategies(data map[string]any, strategies []InvestigationStrategy) []InvestigationStrategy {
	var matched []InvestigationStrategy
	for _, s := range strategies {
		if strategyMatches(data, s) {
			matched = append(matched, s)
		}
	}
	sortByLLMUsage(matched)
	return matched
}

func strategyMatches(data map[string]any, s InvestigationStrategy) bool {
	op := strings.ToUpper(s.Operator)
	if op == "" {
		op = "AND"
	}
	for _, cond := range s.Triggers {
		result := evalStrategyCond(data, cond)
		if op == "OR" && result {
			return true
		}
		if op == "AND" && !result {
			return false
		}
	}
	return op == "AND"
}

// evalStrategyCond evaluates a single trigger condition.
// Uses StringValue for string equality/inequality, Value for numeric comparisons.
func evalStrategyCond(data map[string]any, cond StrategyCond) bool {
	raw, ok := data[cond.Field]
	if !ok {
		return false
	}

	// String comparison path
	if cond.StringValue != "" {
		var s string
		switch x := raw.(type) {
		case string:
			s = x
		default:
			s = fmt.Sprintf("%v", raw)
		}
		switch cond.Op {
		case "==":
			return s == cond.StringValue
		case "!=":
			return s != cond.StringValue
		default:
			return false
		}
	}

	// Numeric comparison path
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
	}
	return false
}

// llmRank maps llm_usage values to sort priority (lower = better).
var llmRank = map[string]int{"none": 0, "minimal": 1, "required": 2, "": 3}

func sortByLLMUsage(ss []InvestigationStrategy) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && llmRank[ss[j].LLMUsage] < llmRank[ss[j-1].LLMUsage]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
