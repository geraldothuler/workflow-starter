// Package playbook provides a YAML-driven engine for orchestrating infrastructure
// investigations. Playbooks declare steps (provider fetches + heuristic analyzers),
// build causal chains from findings, and render markdown reports.
package playbook

import (
	"fmt"
	"time"
)

// PlaybookConfig is the top-level YAML wrapper.
type PlaybookConfig struct {
	Playbook PlaybookSpec `yaml:"playbook"`
}

// PlaybookSpec declares the playbook structure: providers, steps, and report format.
type PlaybookSpec struct {
	ID                string        `yaml:"id"`
	Title             string        `yaml:"title"`
	Description       string        `yaml:"description,omitempty"`
	Version           string        `yaml:"version,omitempty"`
	Tags              []string      `yaml:"tags,omitempty"`
	RequiredProviders []ProviderRef `yaml:"required_providers"`
	OptionalProviders []ProviderRef `yaml:"optional_providers,omitempty"`
	Steps             []PlaybookStep `yaml:"steps"`
	Report            ReportSpec    `yaml:"report,omitempty"`
}

// ProviderRef references an infrastructure provider by ID.
type ProviderRef struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description,omitempty"`
}

// PlaybookStep declares a single investigation step.
type PlaybookStep struct {
	ID            string        `yaml:"id"`
	Title         string        `yaml:"title"`
	Provider      string        `yaml:"provider"`
	ResourceTypes []string      `yaml:"resource_types,omitempty"`
	Optional      bool          `yaml:"optional,omitempty"`
	Analyzers     []AnalyzerRef `yaml:"analyzers"`
}

// AnalyzerRef references a registered analyzer function with optional args.
type AnalyzerRef struct {
	Name string         `yaml:"name"`
	Args map[string]any `yaml:"args,omitempty"`
}

// ReportSpec configures the output report.
type ReportSpec struct {
	TitleTemplate string        `yaml:"title_template,omitempty"`
	Sections      []SectionSpec `yaml:"sections,omitempty"`
}

// SectionSpec declares a report section.
type SectionSpec struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
}

// Validate checks that the playbook spec has all required fields.
func (s *PlaybookSpec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("playbook id is required")
	}
	if s.Title == "" {
		return fmt.Errorf("playbook title is required")
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("playbook must have at least one step")
	}
	for i, step := range s.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d: id is required", i)
		}
		if step.Provider == "" {
			return fmt.Errorf("step %q: provider is required", step.ID)
		}
		if len(step.Analyzers) == 0 {
			return fmt.Errorf("step %q: at least one analyzer is required", step.ID)
		}
	}
	return nil
}

// Severity constants for findings.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Finding represents a single issue detected by an analyzer.
type Finding struct {
	ID             string    `json:"id"`
	StepID         string    `json:"step_id"`
	AnalyzerName   string    `json:"analyzer_name"`
	Severity       string    `json:"severity"`
	Title          string    `json:"title"`
	Detail         string    `json:"detail"`
	Evidence       string    `json:"evidence,omitempty"`
	Recommendation string    `json:"recommendation,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// CausalLink connects a cause finding to an effect finding.
type CausalLink struct {
	From      string `json:"from"`      // Finding ID (cause)
	To        string `json:"to"`        // Finding ID (effect)
	Reasoning string `json:"reasoning"`
}

// InvestigationReport is the complete output of a playbook execution.
type InvestigationReport struct {
	PlaybookID    string        `json:"playbook_id"`
	PlaybookTitle string        `json:"playbook_title"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   time.Time     `json:"completed_at"`
	Duration      time.Duration `json:"duration"`
	StepsExecuted int           `json:"steps_executed"`
	StepsSkipped  int           `json:"steps_skipped"`
	StepResults   []StepResult  `json:"step_results"`
	Findings      []Finding     `json:"findings"`
	CausalChain   []CausalLink  `json:"causal_chain"`
	Summary       string        `json:"summary"`
	Markdown      string        `json:"markdown,omitempty"`
}

// StepResult captures the outcome of a single playbook step.
type StepResult struct {
	StepID   string        `json:"step_id"`
	Title    string        `json:"title"`
	Provider string        `json:"provider"`
	Status   string        `json:"status"` // "success", "skipped", "error"
	Error    string        `json:"error,omitempty"`
	Findings []string      `json:"findings"` // Finding IDs
	Duration time.Duration `json:"duration"`
}

// Step status constants.
const (
	StepStatusSuccess = "success"
	StepStatusSkipped = "skipped"
	StepStatusError   = "error"
)

// ExecuteOptions configures a playbook execution.
type ExecuteOptions struct {
	Namespace   string
	KubeContext string
	Verbose     bool
}
