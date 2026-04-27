package exporter

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
var defaultExporterConfigs embed.FS

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// LOADER
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// LoadConfigExporters loads YAML-driven exporter configurations.
//
// Loading order:
//  1. Embedded defaults from pkg/exporter/config/*.yml
//  2. Project overrides from <projectDir>/.workflow/exporters/*.yml
//
// Override behavior: If a project config has the same exporter.id as an
// embedded config, the project config fully replaces the embedded one.
// Exporters with id starting with "_" are skipped (placeholder files).
func LoadConfigExporters(projectDir string, credResolver *credentials.Resolver) ([]*ConfigExporter, error) {
	specs := make(map[string]ExporterSpec) // id -> spec

	// 1. Load embedded defaults
	entries, err := defaultExporterConfigs.ReadDir("config")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded exporter configs: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := defaultExporterConfigs.ReadFile("config/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded config %s: %w", entry.Name(), err)
		}
		spec, err := parseExporterConfig(data, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedded config %s: %w", entry.Name(), err)
		}
		if spec != nil {
			specs[spec.ID] = *spec
		}
	}

	// 2. Load project overrides
	if projectDir != "" {
		overrideDir := filepath.Join(projectDir, ".workflow", "exporters")
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
				spec, err := parseExporterConfig(data, entry.Name())
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

	// 3. Build ConfigExporter instances
	var exporters []*ConfigExporter
	for _, spec := range specs {
		ce, err := NewConfigExporter(spec, credResolver)
		if err != nil {
			return nil, fmt.Errorf("failed to create ConfigExporter for %q: %w", spec.ID, err)
		}
		exporters = append(exporters, ce)
	}

	return exporters, nil
}

// parseExporterConfig parses a YAML file into an ExporterSpec.
// Returns nil (not error) for placeholder configs (id starts with "_").
func parseExporterConfig(data []byte, filename string) (*ExporterSpec, error) {
	var config ExporterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}

	spec := &config.Exporter

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
