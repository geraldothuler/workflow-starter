// Package exporter provides YAML-driven export to external project management tools.
//
// Architecture mirrors pkg/sources/ but for the inverse operation:
//   - Sources: External Tool → fetch → Markdown → Workflow Platform
//   - Exporters: Workflow Platform → Backlog → template → HTTP → External Tool
//
// New exporter = 1 YAML file in pkg/exporter/config/. Zero Go code.
// Override per project: .workflow/exporters/*.yml (full replace by ID).
package exporter

import (
	"context"
	"log"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// Exporter pushes a backlog to an external project management tool.
type Exporter interface {
	// Name returns the exporter identifier (e.g., "jira", "linear").
	Name() string

	// Push sends the backlog to the external tool.
	// Returns a PushResult with details of what was created.
	Push(ctx context.Context, backlog *types.Backlog, opts PushOptions) (*PushResult, error)

	// SetupGuide returns human-readable setup instructions.
	// Displayed when auth is missing or misconfigured.
	SetupGuide() string
}

// PushOptions configures the push behavior.
type PushOptions struct {
	ProjectKey string // target project key/identifier (e.g., "MYPROJ" for Jira)
	DryRun     bool   // if true, show what would be created without pushing
	Verbose    bool   // detailed output
}

// PushResult contains the outcome of a push operation.
type PushResult struct {
	Target        string       `json:"target"`         // exporter id
	ProjectKey    string       `json:"project_key"`
	EpicsPushed   int          `json:"epics_pushed"`
	StoriesPushed int          `json:"stories_pushed"`
	Items         []PushedItem `json:"items"`
	DryRun        bool         `json:"dry_run"`
	Errors        []string     `json:"errors,omitempty"` // non-fatal errors
}

// PushedItem represents a single item pushed to the external tool.
type PushedItem struct {
	Type      string `json:"type"`                // "epic" | "story"
	LocalID   string `json:"local_id"`            // workflow ID
	RemoteKey string `json:"remote_key,omitempty"` // e.g., "PROJ-123" (empty for dry-run)
	Title     string `json:"title"`
	ParentKey string `json:"parent_key,omitempty"` // story's epic remote key
	Error     string `json:"error,omitempty"`      // per-item error (non-fatal)
}

// --- Registry ---

// Registry holds all registered exporters.
type Registry struct {
	exporters    map[string]Exporter
	credResolver *credentials.Resolver
}

// RegistryOption configures a Registry during creation.
type RegistryOption func(*Registry)

// WithCredentialResolver sets the credential resolver for all exporters.
func WithCredentialResolver(resolver *credentials.Resolver) RegistryOption {
	return func(r *Registry) {
		r.credResolver = resolver
	}
}

// WithProjectDir loads YAML-driven exporter configs from
// <dir>/.workflow/exporters/*.yml as overrides for embedded defaults.
func WithProjectDir(dir string) RegistryOption {
	return func(r *Registry) {
		exporters, err := LoadConfigExporters(dir, r.credResolver)
		if err != nil {
			log.Printf("Warning: failed to load YAML exporters from %s: %v", dir, err)
			return
		}
		for _, e := range exporters {
			r.exporters[e.Name()] = e
		}
	}
}

// NewRegistry creates a registry with all embedded exporter configs.
func NewRegistry(opts ...RegistryOption) *Registry {
	r := &Registry{
		exporters: make(map[string]Exporter),
	}

	// Apply options first to capture credResolver
	for _, opt := range opts {
		opt(r)
	}

	// Load embedded YAML-driven exporters (jira, linear, azure-devops)
	exporters, err := LoadConfigExporters("", r.credResolver)
	if err != nil {
		log.Printf("Warning: failed to load embedded exporter configs: %v", err)
	} else {
		for _, e := range exporters {
			r.exporters[e.Name()] = e
		}
	}

	return r
}

// Get returns the exporter with the given ID, or nil if not found.
func (r *Registry) Get(id string) Exporter {
	return r.exporters[id]
}

// List returns all registered exporters.
func (r *Registry) List() []Exporter {
	result := make([]Exporter, 0, len(r.exporters))
	for _, e := range r.exporters {
		result = append(result, e)
	}
	return result
}
