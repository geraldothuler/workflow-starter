package credentials

import (
	"context"
	"os"
)

// EnvProvider resolves credentials from environment variables.
// This is the default provider — always available, zero config.
type EnvProvider struct{}

// NewEnvProvider creates an environment variable credential provider.
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{}
}

// Name returns "env".
func (p *EnvProvider) Name() string { return "env" }

// Resolve returns the value of the environment variable named `name`.
// Returns ErrNotFound if the variable is not set or empty.
func (p *EnvProvider) Resolve(_ context.Context, name string) (*Credential, error) {
	value := os.Getenv(name)
	if value == "" {
		return nil, ErrNotFound
	}
	return &Credential{
		Name:   name,
		Value:  value,
		Source: "env",
	}, nil
}

// Store sets the environment variable for the current process session only.
// The value is NOT persisted across restarts.
func (p *EnvProvider) Store(_ context.Context, name, value string) error {
	return os.Setenv(name, value)
}

// Available always returns true — environment variables are always accessible.
func (p *EnvProvider) Available() bool { return true }
