package runner

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// CriticalCommandsConfig holds the embedded critical commands configuration.
type CriticalCommandsConfig struct {
	Commands []CommandSpec `yaml:"critical_commands"`
}

// LoadCriticalCommandsConfig loads the embedded critical_commands.yml.
func LoadCriticalCommandsConfig() (*CriticalCommandsConfig, error) {
	data, err := embeddedRunnerConfigs.ReadFile("config/critical_commands.yml")
	if err != nil {
		return nil, fmt.Errorf("embedded critical_commands.yml not found: %w", err)
	}

	var cfg CriticalCommandsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid embedded critical_commands.yml: %w", err)
	}

	return &cfg, nil
}

// MergedCommandSpec returns the command spec for a given command name,
// merging embedded defaults with step-level overrides. Step override wins.
func MergedCommandSpec(name string, embedded *CriticalCommandsConfig, stepSpecs []CommandSpec) *CommandSpec {
	// Check step-level override first
	for _, s := range stepSpecs {
		if s.Name == name {
			return &s
		}
	}

	// Fall back to embedded config
	if embedded != nil {
		for _, s := range embedded.Commands {
			if s.Name == name {
				return &s
			}
		}
	}

	return nil
}

// shouldSkip returns true if a command should be skipped because a dependency
// reported a blocking status.
func shouldSkip(spec *CommandSpec, cmdStatuses map[string]string) bool {
	if spec == nil || len(spec.DependsOn) == 0 {
		return false
	}

	for _, dep := range spec.DependsOn {
		depStatus, ok := cmdStatuses[dep]
		if !ok {
			continue // dependency hasn't run yet -- don't skip
		}

		// If the dependency's status is a blocking one, skip this command
		if depStatus == "error" || depStatus == "critical" || depStatus == "skipped" {
			return true
		}
	}

	return false
}

// renderSkipSignal renders the skip signal template with actual dependency info.
func renderSkipSignal(template string, spec *CommandSpec, cmdStatuses map[string]string) string {
	if template == "" {
		template = "skipped (dependency failed)"
	}

	// Find the first failed dependency
	for _, dep := range spec.DependsOn {
		if status, ok := cmdStatuses[dep]; ok {
			if status == "error" || status == "critical" || status == "skipped" {
				result := strings.ReplaceAll(template, "{dep}", dep)
				result = strings.ReplaceAll(result, "{status}", status)
				return result
			}
		}
	}

	return template
}
