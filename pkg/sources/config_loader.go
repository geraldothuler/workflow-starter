package sources

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EMBEDDED DEFAULTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

//go:embed config/*.yml
var defaultSourceConfigs embed.FS

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// LOADER
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// LoadConfigSources loads YAML-driven source configurations.
//
// Loading order:
//  1. Embedded defaults from pkg/sources/config/*.yml
//  2. Project overrides from <projectDir>/.workflow/sources/*.yml
//
// Override behavior: If a project config has the same source.id as an
// embedded config, the project config fully replaces the embedded one.
// Sources with id starting with "_" are skipped (placeholder files).
func LoadConfigSources(projectDir string, factory MCPClientFactory, credResolver ...*credentials.Resolver) ([]*ConfigSource, error) {
	specs := make(map[string]SourceSpec) // id -> spec

	// 1. Load embedded defaults
	entries, err := defaultSourceConfigs.ReadDir("config")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded source configs: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := defaultSourceConfigs.ReadFile("config/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded config %s: %w", entry.Name(), err)
		}
		spec, err := parseSourceConfig(data, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedded config %s: %w", entry.Name(), err)
		}
		if spec != nil {
			specs[spec.ID] = *spec
		}
	}

	// 2. Load project overrides
	if projectDir != "" {
		overrideDir := filepath.Join(projectDir, ".workflow", "sources")
		if _, err := os.Stat(overrideDir); err == nil {
			overrideFiles, err := os.ReadDir(overrideDir)
			if err != nil {
				return nil, fmt.Errorf("failed to read override dir %s: %w", overrideDir, err)
			}
			for _, entry := range overrideFiles {
				if entry.IsDir() || !isYAMLFile(entry.Name()) {
					continue
				}
				data, err := os.ReadFile(filepath.Join(overrideDir, entry.Name()))
				if err != nil {
					return nil, fmt.Errorf("failed to read override %s: %w", entry.Name(), err)
				}
				spec, err := parseSourceConfig(data, entry.Name())
				if err != nil {
					return nil, fmt.Errorf("failed to parse override %s: %w", entry.Name(), err)
				}
				if spec != nil {
					// Full replace by ID
					specs[spec.ID] = *spec
				}
			}
		}
	}

	// 3. Build ConfigSource instances
	var resolver *credentials.Resolver
	if len(credResolver) > 0 {
		resolver = credResolver[0]
	}

	var sources []*ConfigSource
	for _, spec := range specs {
		cs, err := NewConfigSource(spec, factory, resolver)
		if err != nil {
			return nil, fmt.Errorf("failed to create ConfigSource for %q: %w", spec.ID, err)
		}
		sources = append(sources, cs)
	}

	return sources, nil
}

// parseSourceConfig parses a YAML file into a SourceSpec.
// Returns nil (not error) for placeholder configs (id starts with "_").
func parseSourceConfig(data []byte, filename string) (*SourceSpec, error) {
	var config SourceConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}

	spec := &config.Source

	// Skip placeholders
	if strings.HasPrefix(spec.ID, "_") {
		return nil, nil
	}

	// Validate
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("validation error in %s: %w", filename, err)
	}

	return spec, nil
}

// isYAMLFile checks if a filename has a YAML extension.
func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yml" || ext == ".yaml"
}
