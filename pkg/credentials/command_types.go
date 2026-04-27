package credentials

import (
	"fmt"
	"time"
)

// CommandProviderConfig is the top-level YAML structure for a command-based provider.
type CommandProviderConfig struct {
	Provider CommandProviderSpec `yaml:"provider"`
}

// CommandProviderSpec defines a credential provider that uses external commands.
// Each YAML file in pkg/credentials/providers/ describes one provider.
//
// The Type field determines the provider implementation:
//   - "static" (default): CommandProvider — simple resolve/store/available
//   - "session": SessionCommandProvider — adds pre_check, refresh, TTL cache
type CommandProviderSpec struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	Type string `yaml:"type,omitempty"` // "static" (default) or "session"

	Resolve    CommandSpec `yaml:"resolve"`
	Store      CommandSpec `yaml:"store"`
	Available  CommandSpec `yaml:"available"`
	SetupGuide string     `yaml:"setup_guide,omitempty"`

	// Session-specific fields (only used when Type == "session")
	Session  *SessionConfig  `yaml:"session,omitempty"`
	PreCheck *CommandSpec     `yaml:"pre_check,omitempty"`
	Refresh  *RefreshSpec     `yaml:"refresh,omitempty"`
}

// SessionConfig holds session-specific configuration for session providers.
type SessionConfig struct {
	TTLStr   string `yaml:"ttl"`       // e.g., "8h", "24h"
	CacheKey string `yaml:"cache_key"` // template for cache key, e.g., "aws-{{.profile}}"

	// Derived (populated by Validate)
	TTL time.Duration `yaml:"-"`
}

// RefreshSpec extends CommandSpec with session-specific fields for re-authentication.
type RefreshSpec struct {
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args,omitempty"`
	Interactive bool     `yaml:"interactive,omitempty"` // attach stdin/stdout/stderr to terminal
	TimeoutStr  string   `yaml:"timeout,omitempty"`     // e.g., "120s", "300s"

	// Derived (populated by Validate)
	Timeout time.Duration `yaml:"-"`
}

// CommandSpec defines a single command operation.
type CommandSpec struct {
	Command    string   `yaml:"command"`              // e.g., "pass", "aws", "op"
	Args       []string `yaml:"args,omitempty"`       // supports {{.name}} and {{.value}} templates
	Parse      string   `yaml:"parse,omitempty"`      // "first_line" | "trim" | "json:.field"
	Input      string   `yaml:"input,omitempty"`      // stdin template (supports {{.value}})
	TimeoutStr string   `yaml:"timeout,omitempty"`    // e.g., "10s"

	// Derived (populated by Validate)
	Timeout time.Duration `yaml:"-"`
}

// IsSession returns true if this spec describes a session provider.
func (s *CommandProviderSpec) IsSession() bool {
	return s.Type == "session"
}

// Validate checks the CommandProviderSpec for required fields.
func (s *CommandProviderSpec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("provider.id is required")
	}
	if s.Name == "" {
		return fmt.Errorf("provider.name is required")
	}
	if s.Resolve.Command == "" {
		return fmt.Errorf("provider.resolve.command is required")
	}
	if s.Available.Command == "" {
		return fmt.Errorf("provider.available.command is required")
	}

	// Normalize type
	if s.Type == "" {
		s.Type = "static"
	}
	if s.Type != "static" && s.Type != "session" {
		return fmt.Errorf("unsupported provider type %q (supported: static, session)", s.Type)
	}

	// Parse timeouts for base commands
	specs := []*CommandSpec{&s.Resolve, &s.Store, &s.Available}
	for _, spec := range specs {
		if spec.TimeoutStr != "" {
			d, err := time.ParseDuration(spec.TimeoutStr)
			if err != nil {
				return fmt.Errorf("invalid timeout %q: %w", spec.TimeoutStr, err)
			}
			spec.Timeout = d
		} else {
			spec.Timeout = 10 * time.Second // default
		}
	}

	// Parse pre_check timeout
	if s.PreCheck != nil {
		if s.PreCheck.TimeoutStr != "" {
			d, err := time.ParseDuration(s.PreCheck.TimeoutStr)
			if err != nil {
				return fmt.Errorf("invalid pre_check timeout %q: %w", s.PreCheck.TimeoutStr, err)
			}
			s.PreCheck.Timeout = d
		} else {
			s.PreCheck.Timeout = 10 * time.Second
		}
	}

	// Validate session-specific fields
	if s.IsSession() {
		if s.Session == nil {
			return fmt.Errorf("provider.session is required for type=session")
		}
		if s.Session.TTLStr == "" {
			return fmt.Errorf("provider.session.ttl is required for type=session")
		}
		ttl, err := time.ParseDuration(s.Session.TTLStr)
		if err != nil {
			return fmt.Errorf("invalid session TTL %q: %w", s.Session.TTLStr, err)
		}
		s.Session.TTL = ttl

		if s.Session.CacheKey == "" {
			s.Session.CacheKey = s.ID // default: use provider ID as cache key
		}

		// Parse refresh timeout
		if s.Refresh != nil {
			if s.Refresh.TimeoutStr != "" {
				d, err := time.ParseDuration(s.Refresh.TimeoutStr)
				if err != nil {
					return fmt.Errorf("invalid refresh timeout %q: %w", s.Refresh.TimeoutStr, err)
				}
				s.Refresh.Timeout = d
			} else {
				s.Refresh.Timeout = 120 * time.Second // default: 2 min for interactive flows
			}
		}
	}

	// Validate parse mode
	if s.Resolve.Parse == "" {
		s.Resolve.Parse = "trim" // default
	}
	switch s.Resolve.Parse {
	case "trim", "first_line":
		// OK
	default:
		if len(s.Resolve.Parse) > 5 && s.Resolve.Parse[:5] == "json:" {
			// OK — json:.field format
		} else {
			return fmt.Errorf("unsupported parse mode %q (supported: trim, first_line, json:<path>)", s.Resolve.Parse)
		}
	}

	return nil
}
