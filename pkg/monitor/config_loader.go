package monitor

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed config/*.yml
var embeddedMonitorConfigs embed.FS

// SlackMonitorConfig is the top-level YAML structure for slack_monitor.yml.
type SlackMonitorConfig struct {
	SchemaVersion       int                     `yaml:"schema_version"`
	PollIntervalSeconds int                     `yaml:"poll_interval_seconds"`
	Channels            []string                `yaml:"channels"`
	Signals             map[string]SignalConfig `yaml:"signals"`
}

// SignalConfig holds score threshold and keywords for a signal level (p0, p1, etc.).
type SignalConfig struct {
	Score    int      `yaml:"score"`
	Keywords []string `yaml:"keywords"`
}

// LoadSlackMonitorConfig loads slack_monitor.yml using a 3-stage override chain:
//  1. Embedded defaults (config/slack_monitor.yml)
//  2. Personal override: ~/.workflow/slack_monitor.yml
//  3. Repo override: <repoPath>/.workflow/slack_monitor.yml
//
// Each stage is a merge — partial overrides never break earlier defaults.
func LoadSlackMonitorConfig(repoPath string) (*SlackMonitorConfig, error) {
	// Stage 1: embedded defaults
	data, err := embeddedMonitorConfigs.ReadFile("config/slack_monitor.yml")
	if err != nil {
		return nil, fmt.Errorf("embedded slack_monitor.yml not found: %w", err)
	}

	var cfg SlackMonitorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid embedded slack_monitor.yml: %w", err)
	}

	// Stage 2: personal override (~/.workflow/slack_monitor.yml)
	if home, err := os.UserHomeDir(); err == nil {
		personalPath := filepath.Join(home, ".workflow", "slack_monitor.yml")
		if overrideData, err := os.ReadFile(personalPath); err == nil {
			var override SlackMonitorConfig
			if err := yaml.Unmarshal(overrideData, &override); err == nil {
				mergeSlackMonitorConfig(&cfg, &override)
			}
		}
	}

	// Stage 3: repo override (<repoPath>/.workflow/slack_monitor.yml)
	if repoPath != "" {
		repoOverridePath := filepath.Join(repoPath, ".workflow", "slack_monitor.yml")
		if overrideData, err := os.ReadFile(repoOverridePath); err == nil {
			var override SlackMonitorConfig
			if err := yaml.Unmarshal(overrideData, &override); err == nil {
				mergeSlackMonitorConfig(&cfg, &override)
			}
		}
	}

	return &cfg, nil
}

// WritePersonalOverride persists the config to ~/.workflow/slack_monitor.yml.
// Used by `wtb monitor slack keywords add/remove` to save personal keyword changes.
func WritePersonalOverride(cfg *SlackMonitorConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".workflow")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create ~/.workflow/: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	dest := filepath.Join(dir, "slack_monitor.yml")
	return os.WriteFile(dest, data, 0600)
}

// mergeSlackMonitorConfig applies override values into base (additive merge).
// Non-zero/non-empty override fields win; missing fields keep the base value.
func mergeSlackMonitorConfig(base, override *SlackMonitorConfig) {
	if override.PollIntervalSeconds > 0 {
		base.PollIntervalSeconds = override.PollIntervalSeconds
	}
	if len(override.Channels) > 0 {
		base.Channels = override.Channels
	}
	if len(override.Signals) > 0 {
		if base.Signals == nil {
			base.Signals = make(map[string]SignalConfig)
		}
		for level, sig := range override.Signals {
			base.Signals[level] = sig
		}
	}
}
