package infracontext

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed schema/tech_mapping.yml
var embeddedTechMapping embed.FS

// TechMappingConfig is the YAML structure for tech_mapping.yml.
type TechMappingConfig struct {
	TechMapping map[string]string `yaml:"tech_mapping"`
}

// TechMapper maps container image names to canonical technology names.
type TechMapper struct {
	mapping map[string]string // image prefix -> tech name
}

// NewTechMapper creates a TechMapper from embedded + optional project overrides.
func NewTechMapper(projectDir string) (*TechMapper, error) {
	mapping := make(map[string]string)

	// Stage 1: Load embedded tech mapping
	data, err := embeddedTechMapping.ReadFile("schema/tech_mapping.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded tech_mapping.yml: %w", err)
	}

	var cfg TechMappingConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse embedded tech_mapping.yml: %w", err)
	}
	for k, v := range cfg.TechMapping {
		mapping[k] = v
	}

	// Stage 2: Load project overrides
	if projectDir != "" {
		overridePath := filepath.Join(projectDir, ".workflow", "infra-providers", "tech_mapping.yml")
		if overrideData, err := os.ReadFile(overridePath); err == nil {
			var overrideCfg TechMappingConfig
			if err := yaml.Unmarshal(overrideData, &overrideCfg); err == nil {
				for k, v := range overrideCfg.TechMapping {
					mapping[k] = v
				}
			}
		}
	}

	return &TechMapper{mapping: mapping}, nil
}

// NewTechMapperFromMap creates a TechMapper from a pre-built map (for testing).
func NewTechMapperFromMap(m map[string]string) *TechMapper {
	return &TechMapper{mapping: m}
}

// ExtractTechFromImage maps a container image to its canonical tech name.
// e.g., "docker.io/library/postgres:15" -> "PostgreSQL"
func (tm *TechMapper) ExtractTechFromImage(image string) string {
	if image == "" {
		return ""
	}

	// Extract the image name (last segment before tag)
	// "docker.io/library/postgres:15" -> "postgres"
	// "redis:7.2" -> "redis"
	// "bitnami/kafka:3.6" -> "kafka"
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	name := strings.Split(last, ":")[0]
	name = strings.ToLower(name)

	// Direct match
	if tech, ok := tm.mapping[name]; ok {
		return tech
	}

	// Prefix match (e.g., "postgres" matches "postgresql-15")
	for prefix, tech := range tm.mapping {
		if strings.HasPrefix(name, prefix) {
			return tech
		}
	}

	return ""
}

// Mapping returns a copy of the internal mapping.
func (tm *TechMapper) Mapping() map[string]string {
	result := make(map[string]string, len(tm.mapping))
	for k, v := range tm.mapping {
		result[k] = v
	}
	return result
}
