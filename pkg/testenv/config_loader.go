package testenv

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed config/*.yml
var configFS embed.FS

// LoadTestEnvConfig loads the embedded global config.
func LoadTestEnvConfig() (*TestEnvConfig, error) {
	data, err := configFS.ReadFile("config/test_env.yml")
	if err != nil {
		return nil, fmt.Errorf("reading embedded test_env.yml: %w", err)
	}
	var cfg TestEnvConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing test_env.yml: %w", err)
	}
	return &cfg, nil
}

// LoadRepoTestEnvConfig loads .workflow/testenv.yml from the given repo root.
// Returns an empty config (no error) if the file doesn't exist.
func LoadRepoTestEnvConfig(repoRoot string) (*RepoTestEnvConfig, error) {
	path := filepath.Join(repoRoot, ".workflow", "testenv.yml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &RepoTestEnvConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg RepoTestEnvConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}
