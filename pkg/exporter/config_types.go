package exporter

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
)

// ExporterConfig is the top-level YAML structure for a YAML-driven exporter.
type ExporterConfig struct {
	Exporter ExporterSpec `yaml:"exporter"`
}

// ExporterSpec defines a complete exporter declaratively.
// Each YAML file in pkg/exporter/config/ describes one target tool.
type ExporterSpec struct {
	ID          string                   `yaml:"id"`
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description,omitempty"`
	Auth        credentials.AuthContract `yaml:"auth"`
	Transport   ExportTransportSpec      `yaml:"transport"`
	Setup       *SetupSpec               `yaml:"setup,omitempty"`
	Push        PushSpec                 `yaml:"push"`
}

// ExportTransportSpec defines the HTTP transport configuration.
type ExportTransportSpec struct {
	Type       string            `yaml:"type"`                  // "http"
	BaseURL    string            `yaml:"base_url"`              // Go template with credential vars
	AuthType   string            `yaml:"auth_type"`             // "basic" | "bearer"
	AuthValue  string            `yaml:"auth_value"`            // Go template
	Headers    map[string]string `yaml:"headers,omitempty"`
	TimeoutStr string            `yaml:"timeout,omitempty"`     // e.g., "30s"

	// Derived fields (populated by Validate)
	Timeout time.Duration `yaml:"-"`
}

// SetupSpec defines how to discover/select the target project.
type SetupSpec struct {
	ListProjects *APICallSpec `yaml:"list_projects,omitempty"`
}

// PushSpec defines how to push epics and stories.
type PushSpec struct {
	Epic  APICallSpec `yaml:"epic"`
	Story APICallSpec `yaml:"story"`
}

// APICallSpec defines a single API call template.
type APICallSpec struct {
	Method  string            `yaml:"method"`              // GET, POST, PUT, PATCH
	Path    string            `yaml:"path"`                // URL path template
	Body    string            `yaml:"body,omitempty"`      // JSON body Go template
	Extract map[string]string `yaml:"extract,omitempty"`   // response field name -> dot.path
	Headers map[string]string `yaml:"headers,omitempty"`   // per-call extra headers
}

// --- Validation ---

// Validate checks the ExporterSpec for required fields and parses durations.
func (s *ExporterSpec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("exporter.id is required")
	}
	if s.Name == "" {
		return fmt.Errorf("exporter.name is required")
	}
	if err := s.Transport.Validate(); err != nil {
		return fmt.Errorf("exporter.transport: %w", err)
	}
	if err := s.Push.Validate(); err != nil {
		return fmt.Errorf("exporter.push: %w", err)
	}
	return nil
}

// Validate checks the ExportTransportSpec and parses duration strings.
func (t *ExportTransportSpec) Validate() error {
	if t.Type == "" {
		return fmt.Errorf("type is required")
	}
	if t.Type != "http" {
		return fmt.Errorf("unsupported transport type: %q (supported: http)", t.Type)
	}
	if t.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	authType := strings.ToLower(t.AuthType)
	if authType != "basic" && authType != "bearer" {
		return fmt.Errorf("auth_type must be 'basic' or 'bearer', got %q", t.AuthType)
	}
	if t.AuthValue == "" {
		return fmt.Errorf("auth_value is required")
	}

	// Parse timeout
	if t.TimeoutStr != "" {
		var err error
		t.Timeout, err = time.ParseDuration(t.TimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", t.TimeoutStr, err)
		}
	} else {
		t.Timeout = 30 * time.Second
	}

	return nil
}

// Validate checks the PushSpec for required fields.
func (p *PushSpec) Validate() error {
	if err := p.Epic.Validate("epic"); err != nil {
		return fmt.Errorf("epic: %w", err)
	}
	if err := p.Story.Validate("story"); err != nil {
		return fmt.Errorf("story: %w", err)
	}
	return nil
}

// Validate checks the APICallSpec for required fields.
func (a *APICallSpec) Validate(name string) error {
	method := strings.ToUpper(a.Method)
	if method == "" {
		return fmt.Errorf("method is required")
	}
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true}
	if !validMethods[method] {
		return fmt.Errorf("unsupported method %q (supported: GET, POST, PUT, PATCH)", a.Method)
	}
	if a.Path == "" {
		return fmt.Errorf("path is required")
	}
	return nil
}
