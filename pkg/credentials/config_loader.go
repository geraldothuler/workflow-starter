package credentials

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EMBEDDED DEFAULTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

//go:embed providers/*.yml
var defaultProviderConfigs embed.FS

//go:embed config/defaults.yml
var defaultResolverConfigFS embed.FS

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CONFIG LOADER
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// LoadResolverConfig loads the resolver config from embedded defaults + project override.
// Project override: <projectDir>/.workflow/credentials.yml (full replace if present).
func LoadResolverConfig(projectDir string) (*ResolverConfig, error) {
	// 1. Load embedded defaults
	data, err := defaultResolverConfigFS.ReadFile("config/defaults.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded resolver config: %w", err)
	}

	config := &ResolverConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse embedded resolver config: %w", err)
	}

	// 2. Override with project config if present
	if projectDir != "" {
		overridePath := filepath.Join(projectDir, ".workflow", "credentials.yml")
		if overrideData, err := os.ReadFile(overridePath); err == nil {
			override := &ResolverConfig{}
			if err := yaml.Unmarshal(overrideData, override); err != nil {
				return nil, fmt.Errorf("failed to parse project credentials config %s: %w", overridePath, err)
			}
			// Full replace — project config takes precedence
			config = override
		}
	}

	return config, nil
}

// LoadCommandProviders loads YAML-driven command providers.
//
// Loading order:
//  1. Embedded defaults from pkg/credentials/providers/*.yml
//  2. Project overrides from <projectDir>/.workflow/credential-providers/*.yml
//
// Override behavior: If a project provider has the same provider.id as an
// embedded one, the project config fully replaces the embedded one.
//
// Provider type detection:
//   - type: "static" (default) → CommandProvider
//   - type: "session"          → SessionCommandProvider (with pre_check, refresh, TTL cache)
func LoadCommandProviders(projectDir string) ([]Provider, error) {
	specs := make(map[string]CommandProviderSpec) // id -> spec

	// 1. Load embedded defaults
	entries, err := defaultProviderConfigs.ReadDir("providers")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded provider configs: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := defaultProviderConfigs.ReadFile("providers/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded provider %s: %w", entry.Name(), err)
		}
		spec, err := parseProviderConfig(data, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedded provider %s: %w", entry.Name(), err)
		}
		if spec != nil {
			specs[spec.ID] = *spec
		}
	}

	// 2. Load project overrides
	if projectDir != "" {
		overrideDir := filepath.Join(projectDir, ".workflow", "credential-providers")
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
				spec, err := parseProviderConfig(data, entry.Name())
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

	// 3. Build Provider instances based on type
	// Session providers share a single SessionCache for efficiency.
	sessionCache := NewSessionCache()
	var providers []Provider
	for _, spec := range specs {
		if spec.IsSession() {
			providers = append(providers, NewSessionCommandProvider(spec, sessionCache))
		} else {
			providers = append(providers, NewCommandProvider(spec))
		}
	}

	return providers, nil
}

// NewFullResolver creates a fully configured Resolver with all available providers.
// This is the main entry point for production use.
//
// It loads:
//  1. ResolverConfig from embedded defaults + project override
//  2. Native providers (env, encrypted-file)
//  3. YAML-driven command providers from embedded + project override
//
// The secretsDir is typically <projectDir>/.workflow/secrets.
// The masterKey is used for the encrypted-file provider (empty = disabled).
func NewFullResolver(projectDir, masterKey string) (*Resolver, error) {
	// Load config
	config, err := LoadResolverConfig(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load resolver config: %w", err)
	}

	// Native providers
	nativeProviders := []Provider{
		NewEnvProvider(),
	}

	secretsDir := filepath.Join(projectDir, ".workflow", "secrets")
	encProvider := NewEncryptedFileProvider(secretsDir, masterKey)
	nativeProviders = append(nativeProviders, encProvider)

	// YAML-driven command providers (returns []Provider — static or session)
	cmdProviders, err := LoadCommandProviders(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load command providers: %w", err)
	}

	// Combine all providers
	allProviders := make([]Provider, 0, len(nativeProviders)+len(cmdProviders))
	allProviders = append(allProviders, nativeProviders...)
	allProviders = append(allProviders, cmdProviders...)

	return NewResolver(config, allProviders...), nil
}

// --- helpers ---

func parseProviderConfig(data []byte, filename string) (*CommandProviderSpec, error) {
	var config CommandProviderConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}

	spec := &config.Provider

	// Skip placeholders
	if strings.HasPrefix(spec.ID, "_") {
		return nil, nil
	}

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("validation error in %s: %w", filename, err)
	}

	return spec, nil
}

func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yml" || ext == ".yaml"
}
