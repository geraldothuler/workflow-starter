// Package runner loads use-case definitions and executes their step pipelines.
package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UseCaseDefinition is the parsed content of a use-cases/<type>/definition.yml.
type UseCaseDefinition struct {
	ID          string         `yaml:"id"`
	Type        string         `yaml:"type"` // documentary | pipeline | agent
	Name        string         `yaml:"name"`
	Version     string         `yaml:"version"`
	Description string         `yaml:"description"`
	Primitives  []string       `yaml:"primitives"`
	Triggers    []string       `yaml:"triggers"`
	Inputs      []InputSpec    `yaml:"inputs"`
	Steps       []StepSpec     `yaml:"steps"`
	Artefacts   []ArtefactSpec `yaml:"artefacts"`
	Chain       ChainSpec      `yaml:"chain"`
	Naming      string         `yaml:"naming_convention"`
	NNNPadding  int            `yaml:"nnn_padding"`
	Agent       AgentSpec      `yaml:"agent"`
}

// AgentSpec is the agent: block in an agent-type use-case definition.
// It describes the Claude Code sub-agent to spawn when the use-case is invoked.
type AgentSpec struct {
	SubagentType        string `yaml:"subagent_type"`
	Background          bool   `yaml:"background"`
	DescriptionTemplate string `yaml:"description_template"`
	PromptTemplate      string `yaml:"prompt_template"`
}

// InputSpec describes a required or optional input for the use-case.
type InputSpec struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	WhyAsk      string   `yaml:"why_ask"`  // Socratic message explaining why this input matters
	Resolve     []string `yaml:"resolve"`  // Resolution chain: [session, chain, helm, repo, ask]
}

// CommandSpec describes a single command with dependency and criticality metadata.
type CommandSpec struct {
	Name       string   `yaml:"name"`
	Critical   bool     `yaml:"critical,omitempty"`
	BlocksOn   []string `yaml:"blocks_on,omitempty"`   // statuses that block dependents: [error, critical]
	DependsOn  []string `yaml:"depends_on,omitempty"`  // commands that must succeed first
	SkipSignal string   `yaml:"skip_signal,omitempty"` // message template when skipped
}

// StepSpec describes a single execution step in a pipeline use-case.
type StepSpec struct {
	Name            string            `yaml:"name"`
	Engine          string            `yaml:"engine"`
	Command         string            `yaml:"command"`        // single command (use AllCommands to access)
	Commands        []string          `yaml:"commands"`       // multi-command list (use AllCommands to access)
	CommandSpecs    []CommandSpec     `yaml:"command_specs,omitempty"` // rich command metadata (fallback for AllCommands)
	InputMapping    map[string]string `yaml:"input_mapping"`  // YAML-driven input defaults/aliases (${key:-default})
	Description     string            `yaml:"description"`
	Output          string            `yaml:"output"`
	Optional        bool              `yaml:"optional"`
	HumanCheckpoint bool              `yaml:"human_checkpoint"`
}

// AllCommands returns the unified list of commands: Command (if set) takes priority,
// then Commands slice, then names extracted from CommandSpecs as fallback.
// Engines should always use AllCommands instead of reading Command or Commands directly.
func (s StepSpec) AllCommands() []string {
	if s.Command != "" {
		return []string{s.Command}
	}
	if len(s.Commands) > 0 {
		return s.Commands
	}
	// Fallback: extract names from CommandSpecs
	names := make([]string, len(s.CommandSpecs))
	for i, cs := range s.CommandSpecs {
		names[i] = cs.Name
	}
	return names
}

// ArtefactSpec declares where output files are written.
type ArtefactSpec struct {
	Name        string `yaml:"name"`
	Format      string `yaml:"format"`
	Destination string `yaml:"destination"`
}

// ChainSpec declares upstream and downstream use-case links.
type ChainSpec struct {
	From []string `yaml:"from"`
	To   []string `yaml:"to"`
}

// IsPipeline returns true for use-cases that have executable steps.
func (d *UseCaseDefinition) IsPipeline() bool {
	return d.Type == "pipeline" && len(d.Steps) > 0
}

// IsAgent returns true for use-cases that spawn a Claude Code sub-agent.
func (d *UseCaseDefinition) IsAgent() bool {
	return d.Type == "agent"
}

// RequiredInputs returns only the inputs marked required: true.
func (d *UseCaseDefinition) RequiredInputs() []InputSpec {
	var req []InputSpec
	for _, in := range d.Inputs {
		if in.Required {
			req = append(req, in)
		}
	}
	return req
}

// LoadDefinition reads and parses use-cases/<id>/definition.yml from workflowHome.
func LoadDefinition(workflowHome, useCaseID string) (*UseCaseDefinition, error) {
	path := filepath.Join(workflowHome, "use-cases", useCaseID, "definition.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("definition not found for use-case %q (looked at %s): %w", useCaseID, path, err)
	}

	var def UseCaseDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("invalid definition.yml for %q: %w", useCaseID, err)
	}

	if def.ID == "" {
		return nil, fmt.Errorf("definition.yml for %q is missing required field 'id'", useCaseID)
	}

	return &def, nil
}

// ListUseCases returns all use-case IDs found under workflowHome/use-cases/.
func ListUseCases(workflowHome string) ([]string, error) {
	dir := filepath.Join(workflowHome, "use-cases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read use-cases directory: %w", err)
	}

	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		defPath := filepath.Join(dir, e.Name(), "definition.yml")
		if _, err := os.Stat(defPath); err == nil {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}
