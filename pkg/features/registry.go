package features

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Registry mantém todas as features disponíveis
type Registry struct {
	Features map[string]Feature `yaml:"features"`
}

// Feature define uma funcionalidade do Workflow Platform
type Feature struct {
	Name         string           `yaml:"name"`
	Description  string           `yaml:"description"`
	CLI          CLISpec          `yaml:"cli"`
	Claude       ClaudeSpec       `yaml:"claude"`
	Architecture ArchitectureSpec `yaml:"architecture"`
	Behavior     BehaviorSpec     `yaml:"behavior"`
}

// ArchitectureSpec documenta os patterns arquiteturais usados na composição da feature
type ArchitectureSpec struct {
	Patterns []ArchPatternRef `yaml:"patterns"`
}

// ArchPatternRef referencia um pattern com justificativa em linguagem natural
type ArchPatternRef struct {
	ID  string `yaml:"id"`
	Why string `yaml:"why"`
}

// CLISpec define como feature é exposta no CLI
type CLISpec struct {
	Command  string      `yaml:"command"`
	Inputs   []InputSpec `yaml:"inputs"`
	Examples []string    `yaml:"examples"`
}

// InputSpec define um input do CLI
type InputSpec struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	Choices     []string `yaml:"choices"`
	Description string   `yaml:"description"`
}

// ClaudeSpec define como feature é usada no Claude.ai
type ClaudeSpec struct {
	Skills          []string `yaml:"skills"`
	Patterns        []string `yaml:"patterns"`
	TriggerKeywords []string `yaml:"trigger_keywords"`
}

// BehaviorSpec define comportamento da feature
type BehaviorSpec struct {
	SystemPromptTemplate string `yaml:"system_prompt_template"`
	Interactive          bool   `yaml:"interactive"`
	OutputFormat         string `yaml:"output_format"`
}

// LoadRegistry carrega registro de features do FEATURES.yml
func LoadRegistry(basePath string) (*Registry, error) {
	featuresPath := filepath.Join(basePath, "features", "FEATURES.yml")

	data, err := os.ReadFile(featuresPath)
	if err != nil {
		return nil, err
	}

	var registry Registry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	return &registry, nil
}

// GetFeature retorna feature por nome
func (r *Registry) GetFeature(name string) (*Feature, bool) {
	feature, ok := r.Features[name]
	if !ok {
		return nil, false
	}
	return &feature, true
}

// GetFeatureByCommand busca feature pelo comando CLI
// Aceita match parcial: "backlog" encontra "backlog generate"
func (r *Registry) GetFeatureByCommand(command string) *Feature {
	command = strings.TrimSpace(strings.ToLower(command))
	for _, f := range r.Features {
		cliCmd := strings.ToLower(f.CLI.Command)
		if cliCmd == command || strings.HasPrefix(cliCmd, command+" ") || strings.HasPrefix(command, cliCmd) {
			return &f
		}
	}
	return nil
}

// GetSkillsForCommand retorna skills associados a um comando
func (r *Registry) GetSkillsForCommand(command string) []string {
	feature := r.GetFeatureByCommand(command)
	if feature == nil {
		return nil
	}
	return feature.Claude.Skills
}

// GetPatternsForCommand retorna patterns associados a um comando
func (r *Registry) GetPatternsForCommand(command string) []string {
	feature := r.GetFeatureByCommand(command)
	if feature == nil {
		return nil
	}
	return feature.Claude.Patterns
}

// ListFeatures retorna todas as features
func (r *Registry) ListFeatures() []Feature {
	features := make([]Feature, 0, len(r.Features))
	for _, f := range r.Features {
		features = append(features, f)
	}
	return features
}

// AllArchitecturePatternIDs retorna todos os pattern IDs referenciados em architecture.patterns
// Deduplica automaticamente.
func (r *Registry) AllArchitecturePatternIDs() []string {
	seen := make(map[string]bool)
	var ids []string
	for _, f := range r.Features {
		for _, p := range f.Architecture.Patterns {
			if !seen[p.ID] {
				seen[p.ID] = true
				ids = append(ids, p.ID)
			}
		}
	}
	return ids
}

// AllClaudePatternIDs retorna todos os pattern IDs referenciados em claude.patterns
func (r *Registry) AllClaudePatternIDs() []string {
	seen := make(map[string]bool)
	var ids []string
	for _, f := range r.Features {
		for _, p := range f.Claude.Patterns {
			if !seen[p] {
				seen[p] = true
				ids = append(ids, p)
			}
		}
	}
	return ids
}
