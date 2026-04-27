package runner

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed config/*.yml
var embeddedRunnerConfigs embed.FS

// ResolveConfig holds the YAML-driven input resolution configuration.
type ResolveConfig struct {
	Resolve struct {
		DefaultChain []string                  `yaml:"default_chain"`
		Strategies   map[string]StrategyConfig `yaml:"strategies"`
	} `yaml:"resolve"`
}

// StrategyConfig describes a single resolution strategy.
type StrategyConfig struct {
	Description   string              `yaml:"description"`
	SourceFile    string              `yaml:"source_file,omitempty"`
	ArtifactDir   string              `yaml:"artifact_dir,omitempty"`
	SearchFields  map[string][]string `yaml:"search_fields,omitempty"`
	SearchFiles   []string            `yaml:"search_files,omitempty"`
	FieldMappings map[string][]string `yaml:"field_mappings,omitempty"`
}

// LoadResolveConfig loads the resolve configuration using a two-stage approach:
// Stage 1: embedded defaults from config/resolve_config.yml
// Stage 2: project override from <repoPath>/.workflow/resolve_config.yml (if exists)
func LoadResolveConfig(repoPath string) (*ResolveConfig, error) {
	// Stage 1: embedded defaults
	data, err := embeddedRunnerConfigs.ReadFile("config/resolve_config.yml")
	if err != nil {
		return nil, fmt.Errorf("embedded resolve_config.yml not found: %w", err)
	}

	var cfg ResolveConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid embedded resolve_config.yml: %w", err)
	}

	// Stage 2: project override (merge)
	if repoPath != "" {
		overridePath := filepath.Join(repoPath, ".workflow", "resolve_config.yml")
		if overrideData, err := os.ReadFile(overridePath); err == nil {
			var override ResolveConfig
			if err := yaml.Unmarshal(overrideData, &override); err == nil {
				mergeResolveConfig(&cfg, &override)
			}
		}
	}

	return &cfg, nil
}

// mergeResolveConfig merges override values into base. Override wins per-key.
func mergeResolveConfig(base, override *ResolveConfig) {
	if len(override.Resolve.DefaultChain) > 0 {
		base.Resolve.DefaultChain = override.Resolve.DefaultChain
	}
	for name, strategy := range override.Resolve.Strategies {
		base.Resolve.Strategies[name] = strategy
	}
}
