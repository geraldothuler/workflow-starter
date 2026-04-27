package playbook

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/*.yml
var defaultPlaybooks embed.FS

// LoadPlaybookConfigs loads playbook configurations from embedded defaults
// and project overrides. Override replaces embedded config by playbook ID.
func LoadPlaybookConfigs(projectDir string) (map[string]*PlaybookSpec, error) {
	specs := make(map[string]*PlaybookSpec)

	// Stage 1: Embedded configs
	entries, err := defaultPlaybooks.ReadDir("config")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded playbook configs: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := defaultPlaybooks.ReadFile("config/" + entry.Name())
		if err != nil {
			continue
		}
		spec, err := parsePlaybookConfig(data, entry.Name())
		if err != nil {
			continue
		}
		if spec != nil {
			specs[spec.ID] = spec
		}
	}

	// Stage 2: Project overrides
	if projectDir != "" {
		overrideDir := filepath.Join(projectDir, ".workflow", "playbooks")
		overrideEntries, err := os.ReadDir(overrideDir)
		if err == nil {
			for _, entry := range overrideEntries {
				if entry.IsDir() || !isYAMLFile(entry.Name()) {
					continue
				}
				data, err := os.ReadFile(filepath.Join(overrideDir, entry.Name()))
				if err != nil {
					continue
				}
				spec, err := parsePlaybookConfig(data, entry.Name())
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

// LoadPlaybook loads a single playbook by ID.
func LoadPlaybook(playbookID string, projectDir string) (*PlaybookSpec, error) {
	specs, err := LoadPlaybookConfigs(projectDir)
	if err != nil {
		return nil, err
	}

	spec, ok := specs[playbookID]
	if !ok {
		return nil, fmt.Errorf("playbook %q not found", playbookID)
	}
	return spec, nil
}

func parsePlaybookConfig(data []byte, filename string) (*PlaybookSpec, error) {
	var cfg PlaybookConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	spec := &cfg.Playbook
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid playbook %s: %w", filename, err)
	}

	return spec, nil
}

func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yml" || ext == ".yaml"
}
