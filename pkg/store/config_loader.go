package store

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed config/store_rules.yml
var defaultStoreRulesYAML []byte

// StoreConfig holds YAML-driven second-order heuristics.
type StoreConfig struct {
	TrendRules []TrendRule `yaml:"trend_rules"`
}

// TrendRule fires when the last Window probe executions all share ConsecutiveStatus.
type TrendRule struct {
	Probe             string `yaml:"probe"`
	Window            int    `yaml:"window"`
	ConsecutiveStatus string `yaml:"consecutive_status"`
	EscalateTo        string `yaml:"escalate_to"`
	Signal            string `yaml:"signal"` // {n} replaced with Window value
}

// LoadStoreConfig loads the embedded store rules.
func LoadStoreConfig() (StoreConfig, error) {
	var cfg StoreConfig
	err := yaml.Unmarshal(defaultStoreRulesYAML, &cfg)
	return cfg, err
}
