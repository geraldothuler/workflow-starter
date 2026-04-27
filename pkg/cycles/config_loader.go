package cycles

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed config/*.yml
var embeddedCycleConfigs embed.FS

// CycleConfig holds the YAML-driven cycle detection configuration.
type CycleConfig struct {
	Cycle struct {
		Threshold int            `yaml:"threshold"`
		Signals   []SignalConfig `yaml:"signals"`
		Savepoint SavepointConfig `yaml:"savepoint"`
	} `yaml:"cycle"`
}

// SignalConfig describes a single signal in the cycle rules YAML.
type SignalConfig struct {
	Name             string `yaml:"name"`
	Weight           int    `yaml:"weight"`
	Command          string `yaml:"command,omitempty"`
	Description      string `yaml:"description"`
	ThresholdMinutes int    `yaml:"threshold_minutes,omitempty"`
}

// SavepointConfig holds savepoint output settings.
type SavepointConfig struct {
	Dir    string `yaml:"dir"`
	Format string `yaml:"format"`
}

// LoadCycleConfig loads cycle rules using a two-stage approach:
// Stage 1: embedded defaults from config/cycle_rules.yml
// Stage 2: project override from <repoPath>/.workflow/cycle_rules.yml (if exists)
func LoadCycleConfig(repoPath string) (*CycleConfig, error) {
	// Stage 1: embedded defaults
	data, err := embeddedCycleConfigs.ReadFile("config/cycle_rules.yml")
	if err != nil {
		return nil, fmt.Errorf("embedded cycle_rules.yml not found: %w", err)
	}

	var cfg CycleConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid embedded cycle_rules.yml: %w", err)
	}

	// Stage 2: project override (merge)
	if repoPath != "" {
		overridePath := filepath.Join(repoPath, ".workflow", "cycle_rules.yml")
		if overrideData, err := os.ReadFile(overridePath); err == nil {
			var override CycleConfig
			if err := yaml.Unmarshal(overrideData, &override); err == nil {
				mergeCycleConfig(&cfg, &override)
			}
		}
	}

	return &cfg, nil
}

// mergeCycleConfig merges override values into base. Override wins per-key.
func mergeCycleConfig(base, override *CycleConfig) {
	if override.Cycle.Threshold > 0 {
		base.Cycle.Threshold = override.Cycle.Threshold
	}
	if len(override.Cycle.Signals) > 0 {
		base.Cycle.Signals = override.Cycle.Signals
	}
	if override.Cycle.Savepoint.Dir != "" {
		base.Cycle.Savepoint.Dir = override.Cycle.Savepoint.Dir
	}
	if override.Cycle.Savepoint.Format != "" {
		base.Cycle.Savepoint.Format = override.Cycle.Savepoint.Format
	}
}
