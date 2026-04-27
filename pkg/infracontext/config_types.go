package infracontext

import (
	"fmt"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
)

// InfraProviderConfig is the top-level YAML wrapper.
type InfraProviderConfig struct {
	Provider InfraProviderSpec `yaml:"provider"`
}

// InfraProviderSpec is the declarative provider definition.
type InfraProviderSpec struct {
	ID          string             `yaml:"id"`
	Name        string             `yaml:"name"`
	Description string             `yaml:"description,omitempty"`
	Transport   InfraTransportSpec `yaml:"transport"`
	Auth        credentials.AuthContract `yaml:"auth"`
	Defaults    InfraDefaultsSpec  `yaml:"defaults"`
	FetchSteps  []InfraFetchStep   `yaml:"fetch_steps"`
}

// InfraTransportSpec configures how to communicate with the provider.
type InfraTransportSpec struct {
	Primary  string         `yaml:"primary"`            // "cli", "mcp", or "http"
	Fallback string         `yaml:"fallback,omitempty"` // fallback transport

	CLI  *InfraCLISpec  `yaml:"cli,omitempty"`
	MCP  *InfraMCPSpec  `yaml:"mcp,omitempty"`
	HTTP *InfraHTTPSpec `yaml:"http,omitempty"`
}

// InfraCLISpec configures a CLI-based transport (e.g., kubectl).
type InfraCLISpec struct {
	Command        string   `yaml:"command"`
	TimeoutStr     string   `yaml:"timeout,omitempty"`
	AvailableCheck []string `yaml:"available_check,omitempty"`

	// Derived
	Timeout time.Duration `yaml:"-"`
}

// InfraMCPSpec configures an MCP-based transport.
type InfraMCPSpec struct {
	Command          string   `yaml:"command"`
	Args             []string `yaml:"args,omitempty"`
	TimeoutStr       string   `yaml:"timeout,omitempty"`
	StartupTimeoutStr string  `yaml:"startup_timeout,omitempty"`

	// Derived
	Timeout        time.Duration `yaml:"-"`
	StartupTimeout time.Duration `yaml:"-"`
}

// InfraHTTPSpec configures an HTTP-based transport (e.g., Datadog, Prometheus, Confluent Cloud).
// Auth is handled via headers or auth_type/auth_value (Go templates expanded with resolved credentials).
type InfraHTTPSpec struct {
	BaseURL    string            `yaml:"base_url"`
	AuthType   string            `yaml:"auth_type,omitempty"`   // "basic", "bearer", or "" (auth via headers)
	AuthValue  string            `yaml:"auth_value,omitempty"`  // Go template (e.g., "{{.API_KEY}}:{{.API_SECRET}}")
	Headers    map[string]string `yaml:"headers,omitempty"`
	TimeoutStr string            `yaml:"timeout,omitempty"`

	// Derived
	Timeout time.Duration `yaml:"-"`
}

// InfraDefaultsSpec defines default values for fetch operations.
type InfraDefaultsSpec struct {
	Namespace     string   `yaml:"namespace,omitempty"`
	TTLStr        string   `yaml:"ttl,omitempty"`
	ResourceTypes []string `yaml:"resource_types,omitempty"`

	// Derived
	TTL time.Duration `yaml:"-"`
}

// InfraProvidesSpec defines how to extract values from a step's response
// for use in subsequent for_each steps.
type InfraProvidesSpec struct {
	SourcePath string            `yaml:"source_path"`             // dot-path to items array
	Field      string            `yaml:"field,omitempty"`         // single field: extracts []string
	Fields     map[string]string `yaml:"fields,omitempty"`        // multi-field: extracts []map[string]string
}

// InfraFetchStep defines one step in the fetch pipeline.
type InfraFetchStep struct {
	ID         string            `yaml:"id"`
	CLICommand string            `yaml:"cli_command,omitempty"`
	CLIArgs    []string          `yaml:"cli_args,omitempty"`    // explicit args list (each template-expanded, preserves spaces)
	MCPTool    string            `yaml:"mcp_tool,omitempty"`
	MCPArgs    map[string]any    `yaml:"mcp_args,omitempty"`
	Optional   bool              `yaml:"optional,omitempty"`
	ParseMode  string            `yaml:"parse_mode,omitempty"` // "json" (default) or "text"
	Mapping    InfraMapping      `yaml:"mapping"`

	// HTTP-specific fields
	HTTPMethod  string               `yaml:"http_method,omitempty"`
	HTTPPath    string               `yaml:"http_path,omitempty"`    // Go template
	HTTPParams  map[string]string    `yaml:"http_params,omitempty"` // query params
	HTTPHeaders map[string]string    `yaml:"http_headers,omitempty"`
	HTTPBody    string               `yaml:"http_body,omitempty"`
	Pagination  *InfraPaginationSpec `yaml:"pagination,omitempty"`

	// Step-chaining: provides values for subsequent for_each steps
	Provides    map[string]*InfraProvidesSpec `yaml:"provides,omitempty"`
	ForEach     string                        `yaml:"for_each,omitempty"`      // references a provides name
	HTTPBaseURL string                        `yaml:"http_base_url,omitempty"` // per-step base URL override (Go template)
}

// InfraMapping defines how to map raw data to InfraContext fields.
type InfraMapping struct {
	Topology *InfraMappingSection `yaml:"topology,omitempty"`
	Health   *InfraMappingSection `yaml:"health,omitempty"`
	Metrics  *InfraMappingSection `yaml:"metrics,omitempty"`
	Alerts   *InfraMappingSection `yaml:"alerts,omitempty"`
}

// InfraMappingSection maps raw data to a specific InfraContext section.
type InfraMappingSection struct {
	SourcePath string                   `yaml:"source_path,omitempty"`
	Transform  string                   `yaml:"transform,omitempty"`
	Args       map[string]any           `yaml:"args,omitempty"`
	Each       map[string]any           `yaml:"each,omitempty"`
}

// InfraPaginationSpec configures declarative pagination for HTTP fetch steps.
type InfraPaginationSpec struct {
	Style         string `yaml:"style"`                    // "cursor" | "page" | "offset"
	CursorParam   string `yaml:"cursor_param,omitempty"`
	CursorPath    string `yaml:"cursor_path,omitempty"`
	PageParam     string `yaml:"page_param,omitempty"`
	PageSizeParam string `yaml:"page_size_param,omitempty"`
	PageSize      int    `yaml:"page_size,omitempty"`
	OffsetParam   string `yaml:"offset_param,omitempty"`
	LimitParam    string `yaml:"limit_param,omitempty"`
	Limit         int    `yaml:"limit,omitempty"`
	TotalPath     string `yaml:"total_path,omitempty"`
	ResultsPath   string `yaml:"results_path,omitempty"`
	MaxPages      int    `yaml:"max_pages,omitempty"`
}

// Validate checks the provider spec for required fields and parses durations.
func (s *InfraProviderSpec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("provider.id is required")
	}
	if s.Name == "" {
		return fmt.Errorf("provider.name is required")
	}
	if err := s.Transport.Validate(); err != nil {
		return fmt.Errorf("provider.transport: %w", err)
	}
	if err := s.Defaults.Validate(); err != nil {
		return fmt.Errorf("provider.defaults: %w", err)
	}
	for i, step := range s.FetchSteps {
		if step.ID == "" {
			return fmt.Errorf("provider.fetch_steps[%d].id is required", i)
		}
		if step.CLICommand != "" && len(step.CLIArgs) > 0 {
			return fmt.Errorf("provider.fetch_steps[%d]: cli_command and cli_args are mutually exclusive", i)
		}
	}
	return nil
}

// Validate checks the transport spec.
func (t *InfraTransportSpec) Validate() error {
	if t.Primary == "" {
		return fmt.Errorf("primary is required")
	}
	if t.Primary != "cli" && t.Primary != "mcp" && t.Primary != "http" {
		return fmt.Errorf("primary must be 'cli', 'mcp', or 'http', got %q", t.Primary)
	}
	if t.CLI != nil {
		if err := t.CLI.Validate(); err != nil {
			return fmt.Errorf("cli: %w", err)
		}
	}
	if t.MCP != nil {
		if err := t.MCP.Validate(); err != nil {
			return fmt.Errorf("mcp: %w", err)
		}
	}
	if t.HTTP != nil {
		if err := t.HTTP.Validate(); err != nil {
			return fmt.Errorf("http: %w", err)
		}
	}
	return nil
}

// Validate checks the CLI spec and parses durations.
func (c *InfraCLISpec) Validate() error {
	if c.Command == "" {
		return fmt.Errorf("command is required")
	}
	if c.TimeoutStr != "" {
		d, err := time.ParseDuration(c.TimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", c.TimeoutStr, err)
		}
		c.Timeout = d
	} else {
		c.Timeout = 30 * time.Second
	}
	return nil
}

// Validate checks the MCP spec and parses durations.
func (m *InfraMCPSpec) Validate() error {
	if m.Command == "" {
		return fmt.Errorf("command is required")
	}
	if m.TimeoutStr != "" {
		d, err := time.ParseDuration(m.TimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", m.TimeoutStr, err)
		}
		m.Timeout = d
	} else {
		m.Timeout = 60 * time.Second
	}
	if m.StartupTimeoutStr != "" {
		d, err := time.ParseDuration(m.StartupTimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid startup_timeout %q: %w", m.StartupTimeoutStr, err)
		}
		m.StartupTimeout = d
	} else {
		m.StartupTimeout = 15 * time.Second
	}
	return nil
}

// Validate checks the HTTP spec and parses durations.
func (h *InfraHTTPSpec) Validate() error {
	if h.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if h.TimeoutStr != "" {
		d, err := time.ParseDuration(h.TimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", h.TimeoutStr, err)
		}
		h.Timeout = d
	} else {
		h.Timeout = 30 * time.Second
	}
	return nil
}

// Validate checks defaults and parses durations.
func (d *InfraDefaultsSpec) Validate() error {
	if d.TTLStr != "" {
		dur, err := time.ParseDuration(d.TTLStr)
		if err != nil {
			return fmt.Errorf("invalid ttl %q: %w", d.TTLStr, err)
		}
		d.TTL = dur
	} else {
		d.TTL = 5 * time.Minute
	}
	if d.Namespace == "" {
		d.Namespace = "default"
	}
	return nil
}
