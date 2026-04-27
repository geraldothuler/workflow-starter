package sources

import (
	"log"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
)

// Source defines the interface for fetching content from external sources.
// Each implementation converts a URL into markdown text that can be fed
// into the existing extract pipeline.
type Source interface {
	// Name returns the source identifier (e.g., "notion", "confluence")
	Name() string

	// CanHandle returns true if this source can handle the given URL
	CanHandle(url string) bool

	// Fetch retrieves the content at the URL and returns it as markdown text.
	Fetch(url string) (*FetchResult, error)

	// SetupGuide returns a human-readable setup guide for configuring this source.
	// Displayed when auth is missing or misconfigured.
	SetupGuide() string
}

// FetchResult contains the fetched content plus metadata
type FetchResult struct {
	Content    string            `json:"content"`     // Markdown content
	Title      string            `json:"title"`       // Page title
	URL        string            `json:"url"`         // Original URL
	Source     string            `json:"source"`      // Source name (e.g., "notion")
	BlockCount int               `json:"block_count"` // Number of blocks converted
	Metadata   map[string]string `json:"metadata"`    // Additional metadata
}

// Registry holds all registered sources
type Registry struct {
	sources      []Source
	credResolver *credentials.Resolver // shared credential resolver
}

// RegistryOption configures a Registry during creation.
type RegistryOption func(*Registry)

// WithCredentialResolver sets the credential resolver for all sources.
func WithCredentialResolver(resolver *credentials.Resolver) RegistryOption {
	return func(r *Registry) {
		r.credResolver = resolver
	}
}

// WithProjectDir loads YAML-driven source configs from
// <dir>/.workflow/sources/*.yml as overrides for embedded defaults.
func WithProjectDir(dir string) RegistryOption {
	return func(r *Registry) {
		configSources, err := LoadConfigSources(dir, nil, r.credResolver)
		if err != nil {
			log.Printf("Warning: failed to load YAML sources from %s: %v", dir, err)
			return
		}
		for _, cs := range configSources {
			r.sources = append(r.sources, cs)
		}
	}
}

// WithMCPFactory sets a custom MCPClientFactory for YAML-driven sources.
// Useful for testing with mock MCP sessions.
func WithMCPFactory(factory MCPClientFactory) RegistryOption {
	return func(r *Registry) {
		configSources, err := LoadConfigSources("", factory, r.credResolver)
		if err != nil {
			log.Printf("Warning: failed to load YAML sources with custom factory: %v", err)
			return
		}
		for _, cs := range configSources {
			r.sources = append(r.sources, cs)
		}
	}
}

// NewRegistry creates a registry with all built-in sources.
// Sources are checked in registration order (first match wins).
// Use RegistryOptions to add YAML-driven sources or project overrides.
func NewRegistry(opts ...RegistryOption) *Registry {
	r := &Registry{}

	// Apply options first to capture credResolver before loading sources
	for _, opt := range opts {
		opt(r)
	}

	// Register built-in sources
	// Notion uses lazy auth — token check happens in Fetch(), not here.
	r.sources = append(r.sources, &NotionSource{})

	// Load embedded YAML-driven sources (figma, miro, etc.)
	configSources, err := LoadConfigSources("", nil, r.credResolver)
	if err != nil {
		log.Printf("Warning: failed to load embedded source configs: %v", err)
	} else {
		for _, cs := range configSources {
			r.sources = append(r.sources, cs)
		}
	}

	return r
}

// Detect finds the appropriate source for a URL, or nil if none match.
func (r *Registry) Detect(url string) Source {
	for _, s := range r.sources {
		if s.CanHandle(url) {
			return s
		}
	}
	return nil
}

// Sources returns all registered sources.
func (r *Registry) Sources() []Source {
	return r.sources
}

// IsURL returns true if the input looks like a URL rather than a file path.
func IsURL(input string) bool {
	input = strings.TrimSpace(input)
	return strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")
}
