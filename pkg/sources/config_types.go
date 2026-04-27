package sources

import (
	"fmt"
	"strings"
	"time"
)

// SourceConfig is the top-level YAML structure for a YAML-driven source.
type SourceConfig struct {
	Source SourceSpec `yaml:"source"`
}

// SourceSpec defines a complete external source declaratively.
// Each YAML file in pkg/sources/config/ describes one source.
type SourceSpec struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	URLPatterns []string `yaml:"url_patterns"`

	URLParser  URLParserSpec `yaml:"url_parser"`
	Transport  TransportSpec `yaml:"transport"`
	Auth       AuthSpec      `yaml:"auth"`
	FetchSteps []FetchStep   `yaml:"fetch_steps"`
	Markdown   MarkdownSpec  `yaml:"markdown"`
}

// URLParserSpec defines how to extract variables from a URL.
type URLParserSpec struct {
	Regex    string         `yaml:"regex"`
	Captures map[string]int `yaml:"captures"` // name -> regex group index
}

// TransportSpec defines the MCP transport configuration.
type TransportSpec struct {
	Type       string            `yaml:"type"`                // "mcp"
	Command    string            `yaml:"command"`             // e.g., "npx"
	Args       []string          `yaml:"args,omitempty"`      // e.g., ["-y", "@anthropic-ai/figma-mcp"]
	Env        map[string]string `yaml:"env,omitempty"`       // supports ${VAR} expansion
	TimeoutStr string            `yaml:"timeout,omitempty"`   // e.g., "30s"
	StartupStr string            `yaml:"startup_timeout,omitempty"` // e.g., "15s"

	// Derived fields (populated by Validate)
	Timeout        time.Duration `yaml:"-"`
	StartupTimeout time.Duration `yaml:"-"`
}

// AuthSpec defines authentication requirements.
type AuthSpec struct {
	EnvVar     string `yaml:"env_var"`
	SetupGuide string `yaml:"setup_guide"`
}

// FetchStep defines a single step in the fetch pipeline.
type FetchStep struct {
	Tool    string            `yaml:"tool"`
	Args    map[string]any    `yaml:"args,omitempty"`
	Extract map[string]string `yaml:"extract,omitempty"` // field name -> dot.path
	StoreAs string            `yaml:"store_as,omitempty"`
}

// MarkdownSpec defines how to convert fetched data to markdown.
type MarkdownSpec struct {
	Title    string      `yaml:"title"`    // Go template, e.g., "{{.title}}"
	Mode     string      `yaml:"mode"`     // "walker" | "template"
	Walker   *WalkerSpec `yaml:"walker,omitempty"`
	Template string      `yaml:"template,omitempty"` // Go text/template source
}

// WalkerSpec tunes the generic JSON-to-markdown walker.
type WalkerSpec struct {
	SourceKey   string   `yaml:"source_key,omitempty"`   // which stored result to walk
	MaxDepth    int      `yaml:"max_depth,omitempty"`    // max JSON nesting depth
	HeadingKeys []string `yaml:"heading_keys,omitempty"` // keys that become headings
	ListKeys    []string `yaml:"list_keys,omitempty"`    // keys rendered as lists
	SkipKeys    []string `yaml:"skip_keys,omitempty"`    // keys to omit entirely
	ValueKeys   []string `yaml:"value_keys,omitempty"`   // keys as key-value pairs
	CodeKeys    []string `yaml:"code_keys,omitempty"`    // keys as code blocks
}

// --- Validation ---

// Validate checks the SourceSpec for required fields and parses duration strings.
func (s *SourceSpec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("source.id is required")
	}
	if s.Name == "" {
		return fmt.Errorf("source.name is required")
	}
	if len(s.URLPatterns) == 0 {
		return fmt.Errorf("source.url_patterns requires at least one pattern")
	}
	if s.URLParser.Regex == "" {
		return fmt.Errorf("source.url_parser.regex is required")
	}
	if len(s.URLParser.Captures) == 0 {
		return fmt.Errorf("source.url_parser.captures requires at least one capture")
	}
	if err := s.Transport.Validate(); err != nil {
		return fmt.Errorf("source.transport: %w", err)
	}
	if s.Auth.EnvVar == "" {
		return fmt.Errorf("source.auth.env_var is required")
	}
	if len(s.FetchSteps) == 0 {
		return fmt.Errorf("source.fetch_steps requires at least one step")
	}
	for i, step := range s.FetchSteps {
		if step.Tool == "" {
			return fmt.Errorf("source.fetch_steps[%d].tool is required", i)
		}
	}
	if err := s.Markdown.Validate(); err != nil {
		return fmt.Errorf("source.markdown: %w", err)
	}
	return nil
}

// Validate checks the TransportSpec and parses duration strings.
func (t *TransportSpec) Validate() error {
	if t.Type == "" {
		return fmt.Errorf("type is required")
	}
	if t.Type != "mcp" {
		return fmt.Errorf("unsupported transport type: %q (supported: mcp)", t.Type)
	}
	if t.Command == "" {
		return fmt.Errorf("command is required for mcp transport")
	}

	// Parse timeout strings
	var err error
	if t.TimeoutStr != "" {
		t.Timeout, err = time.ParseDuration(t.TimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", t.TimeoutStr, err)
		}
	} else {
		t.Timeout = 30 * time.Second
	}

	if t.StartupStr != "" {
		t.StartupTimeout, err = time.ParseDuration(t.StartupStr)
		if err != nil {
			return fmt.Errorf("invalid startup_timeout %q: %w", t.StartupStr, err)
		}
	} else {
		t.StartupTimeout = 10 * time.Second
	}

	return nil
}

// Validate checks the MarkdownSpec.
func (m *MarkdownSpec) Validate() error {
	mode := strings.ToLower(m.Mode)
	if mode != "walker" && mode != "template" {
		return fmt.Errorf("mode must be 'walker' or 'template', got %q", m.Mode)
	}
	if mode == "template" && m.Template == "" {
		return fmt.Errorf("template is required when mode is 'template'")
	}
	return nil
}

// DefaultWalkerConfig returns sensible defaults for the JSON walker.
func DefaultWalkerConfig() *WalkerSpec {
	return &WalkerSpec{
		MaxDepth:    6,
		HeadingKeys: []string{"name", "title", "label"},
		ListKeys:    nil, // auto-detect arrays of objects
		SkipKeys:    []string{"id"},
		ValueKeys:   []string{"type"},
		CodeKeys:    nil,
	}
}

// Merged returns a WalkerSpec with defaults filled in for zero-value fields.
func (w *WalkerSpec) Merged() *WalkerSpec {
	defaults := DefaultWalkerConfig()
	result := &WalkerSpec{
		SourceKey:   w.SourceKey,
		MaxDepth:    w.MaxDepth,
		HeadingKeys: w.HeadingKeys,
		ListKeys:    w.ListKeys,
		SkipKeys:    w.SkipKeys,
		ValueKeys:   w.ValueKeys,
		CodeKeys:    w.CodeKeys,
	}
	if result.MaxDepth == 0 {
		result.MaxDepth = defaults.MaxDepth
	}
	if len(result.HeadingKeys) == 0 {
		result.HeadingKeys = defaults.HeadingKeys
	}
	if len(result.SkipKeys) == 0 {
		result.SkipKeys = defaults.SkipKeys
	}
	if len(result.ValueKeys) == 0 {
		result.ValueKeys = defaults.ValueKeys
	}
	return result
}
