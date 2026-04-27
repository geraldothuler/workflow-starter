package infracontext

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/*.yml
var defaultConfigs embed.FS

// LoadProviderConfigs loads provider configurations from embedded defaults
// and project overrides. Override replaces embedded config by provider ID.
func LoadProviderConfigs(projectDir string) (map[string]*InfraProviderSpec, error) {
	specs := make(map[string]*InfraProviderSpec)

	// Stage 1: Embedded configs
	entries, err := defaultConfigs.ReadDir("config")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded configs: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := defaultConfigs.ReadFile("config/" + entry.Name())
		if err != nil {
			continue
		}
		spec, err := parseProviderConfig(data, entry.Name())
		if err != nil {
			continue // skip invalid configs silently
		}
		if spec != nil && !strings.HasPrefix(spec.ID, "_") {
			specs[spec.ID] = spec
		}
	}

	// Stage 2: Project overrides
	if projectDir != "" {
		overrideDir := filepath.Join(projectDir, ".workflow", "infra-providers")
		overrideEntries, err := os.ReadDir(overrideDir)
		if err == nil {
			for _, entry := range overrideEntries {
				if entry.IsDir() || !isYAMLFile(entry.Name()) {
					continue
				}
				// Skip tech_mapping.yml — loaded separately by TechMapper
				if entry.Name() == "tech_mapping.yml" || entry.Name() == "tech_mapping.yaml" {
					continue
				}
				data, err := os.ReadFile(filepath.Join(overrideDir, entry.Name()))
				if err != nil {
					continue
				}
				spec, err := parseProviderConfig(data, entry.Name())
				if err != nil {
					continue
				}
				if spec != nil {
					specs[spec.ID] = spec // full replace by ID
				}
			}
		}
	}

	return specs, nil
}

// LoadProviderConfig loads a single provider config by ID.
func LoadProviderConfig(providerID string, projectDir string) (*InfraProviderSpec, error) {
	specs, err := LoadProviderConfigs(projectDir)
	if err != nil {
		return nil, err
	}

	spec, ok := specs[providerID]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", providerID)
	}
	return spec, nil
}

func parseProviderConfig(data []byte, filename string) (*InfraProviderSpec, error) {
	var cfg InfraProviderConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	spec := &cfg.Provider
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", filename, err)
	}

	return spec, nil
}

func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yml" || ext == ".yaml"
}
