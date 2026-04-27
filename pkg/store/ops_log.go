package store

import (
	"fmt"
	"strings"
)

// TrendSignal evaluates YAML trend rules against the log and returns
// any signals that fired, ready to print. Empty slice = no rules matched.
func TrendSignal(path string) ([]string, error) {
	cfg, err := LoadStoreConfig()
	if err != nil {
		return nil, err
	}
	return evalRules(path, cfg.TrendRules), nil
}

func evalRules(path string, rules []TrendRule) []string {
	var signals []string
	for _, rule := range rules {
		recent := QueryTrend(path, rule.Probe, rule.Window)
		if len(recent) < rule.Window {
			continue
		}
		allMatch := true
		for _, r := range recent[:rule.Window] {
			if r.Status != rule.ConsecutiveStatus {
				allMatch = false
				break
			}
		}
		if allMatch {
			msg := strings.ReplaceAll(rule.Signal, "{n}", fmt.Sprintf("%d", rule.Window))
			signals = append(signals, fmt.Sprintf("[%s] %s", rule.EscalateTo, msg))
		}
	}
	return signals
}
