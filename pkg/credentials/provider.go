// Package credentials provides pluggable credential resolution for Workflow Platform.
//
// Architecture:
//
//	credentials.Resolver chains multiple Provider implementations.
//	Providers are tried in order until one resolves the credential.
//
//	Native providers (Go code):
//	  - env: reads os.Getenv() — default, always available
//	  - encrypted-file: AES-256-GCM storage in .workflow/secrets/
//
//	Command providers (YAML-driven, zero Go code):
//	  - pass, keyring-macos, keyring-linux, aws-ssm, 1password
//	  - Each is 1 YAML file in pkg/credentials/providers/*.yml
//	  - Override/extend: .workflow/credential-providers/*.yml
//
// YAML auth contract (sources/exporters):
//
//	auth:
//	  credentials:
//	    - name: "FIGMA_ACCESS_TOKEN"
//	      required: true
//	  resolve_order: ["env", "keyring", "encrypted-file"]
//
// Backward compatible: old `auth.env_var` field is auto-migrated via Normalize().
package credentials

import (
	"context"
	"time"
)

// Provider resolves credentials by name.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Name returns the provider identifier (e.g., "env", "keyring", "pass").
	Name() string

	// Resolve attempts to fetch the credential by name.
	// Returns ErrNotFound if the credential is not available in this provider.
	Resolve(ctx context.Context, name string) (*Credential, error)

	// Store saves a credential. Not all providers support this.
	// Returns ErrUnsupported if the provider is read-only.
	Store(ctx context.Context, name, value string) error

	// Available returns true if this provider is usable on the current system.
	// For example, keyring checks for OS keychain; pass checks for gpg.
	Available() bool
}

// Credential represents a resolved secret value.
type Credential struct {
	Name      string     // e.g., "FIGMA_ACCESS_TOKEN"
	Value     string     // the secret value
	Source    string     // which provider resolved it: "env", "keyring", etc.
	ExpiresAt *time.Time // optional: when this credential expires (session providers)
}

// IsExpired returns true if the credential has an expiration time that has passed.
func (c *Credential) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*c.ExpiresAt)
}

// CredentialSpec describes a single credential requirement (from YAML auth contract).
type CredentialSpec struct {
	Name        string `yaml:"name"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description,omitempty"`
	ValidateRe  string `yaml:"validate_regex,omitempty"` // optional format check regex
}

// AuthContract is the YAML auth section for sources and exporters.
// Supports both the new multi-credential format and the legacy single env_var format.
type AuthContract struct {
	// New format
	Credentials  []CredentialSpec `yaml:"credentials,omitempty"`
	ResolveOrder []string         `yaml:"resolve_order,omitempty"` // provider IDs in order
	SetupGuide   string           `yaml:"setup_guide,omitempty"`

	// Legacy fields (backward compat — auto-migrated by Normalize)
	EnvVar  string   `yaml:"env_var,omitempty"`  // single env var (old sources)
	EnvVars []string `yaml:"env_vars,omitempty"` // multiple env vars (old exporters)
}

// Normalize migrates legacy auth formats to the new credential-based format.
// After calling Normalize, Credentials is guaranteed to be populated.
// This is idempotent — calling it multiple times is safe.
func (a *AuthContract) Normalize() {
	if len(a.Credentials) > 0 {
		return // already in new format
	}

	// Migrate single env_var
	if a.EnvVar != "" {
		a.Credentials = []CredentialSpec{
			{Name: a.EnvVar, Required: true},
		}
		return
	}

	// Migrate multiple env_vars
	if len(a.EnvVars) > 0 {
		for _, envVar := range a.EnvVars {
			a.Credentials = append(a.Credentials, CredentialSpec{
				Name:     envVar,
				Required: true,
			})
		}
		return
	}
}

// RequiredNames returns the names of all required credentials.
func (a *AuthContract) RequiredNames() []string {
	a.Normalize()
	var names []string
	for _, c := range a.Credentials {
		if c.Required {
			names = append(names, c.Name)
		}
	}
	return names
}

// AllNames returns the names of all declared credentials.
func (a *AuthContract) AllNames() []string {
	a.Normalize()
	var names []string
	for _, c := range a.Credentials {
		names = append(names, c.Name)
	}
	return names
}
