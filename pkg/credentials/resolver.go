package credentials

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ResolverConfig holds global credential resolution settings.
type ResolverConfig struct {
	DefaultOrder []string `yaml:"default_order"` // e.g., ["env", "keyring", "encrypted-file"]
	Verbose      bool     `yaml:"verbose"`
}

// DefaultResolverConfig returns sensible defaults.
func DefaultResolverConfig() *ResolverConfig {
	return &ResolverConfig{
		DefaultOrder: []string{"env"},
		Verbose:      false,
	}
}

// Resolver chains multiple providers and resolves credentials in order.
type Resolver struct {
	providers map[string]Provider // id -> provider
	ordered   []Provider          // providers in registration order
	config    *ResolverConfig
}

// NewResolver creates a resolver with the given providers.
// Providers are registered by their Name(). If config is nil, DefaultResolverConfig is used.
func NewResolver(config *ResolverConfig, providers ...Provider) *Resolver {
	if config == nil {
		config = DefaultResolverConfig()
	}

	providerMap := make(map[string]Provider, len(providers))
	for _, p := range providers {
		providerMap[p.Name()] = p
	}

	return &Resolver{
		providers: providerMap,
		ordered:   providers,
		config:    config,
	}
}

// Resolve tries providers in resolveOrder until one succeeds.
// If resolveOrder is nil/empty, uses the config's DefaultOrder.
// If DefaultOrder doesn't match any registered provider, falls back to all registered providers.
func (r *Resolver) Resolve(ctx context.Context, name string, resolveOrder []string) (*Credential, error) {
	if len(resolveOrder) == 0 {
		resolveOrder = r.config.DefaultOrder
	}

	// Check if any provider in the order is actually registered
	hasMatch := false
	for _, providerID := range resolveOrder {
		if _, ok := r.providers[providerID]; ok {
			hasMatch = true
			break
		}
	}

	// If no providers in the order are registered, try all registered providers
	if !hasMatch {
		return r.resolveAllProviders(ctx, name)
	}

	// Try providers in specified order
	var lastErr error
	for _, providerID := range resolveOrder {
		p, ok := r.providers[providerID]
		if !ok {
			continue // provider not registered, skip
		}
		if !p.Available() {
			continue // provider not available on this system
		}

		cred, err := p.Resolve(ctx, name)
		if err == nil {
			return cred, nil
		}
		if !errors.Is(err, ErrNotFound) {
			lastErr = fmt.Errorf("provider %q: %w", providerID, err)
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNotFound
}

// resolveAllProviders tries every registered provider in registration order.
// Used as fallback when the specified resolve order doesn't match any registered providers.
func (r *Resolver) resolveAllProviders(ctx context.Context, name string) (*Credential, error) {
	var lastErr error
	for _, p := range r.ordered {
		if !p.Available() {
			continue
		}

		cred, err := p.Resolve(ctx, name)
		if err == nil {
			return cred, nil
		}
		if !errors.Is(err, ErrNotFound) {
			lastErr = fmt.Errorf("provider %q: %w", p.Name(), err)
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNotFound
}

// ResolveAll resolves all credentials declared in an AuthContract.
// Returns a map of credential name -> Credential for all resolved credentials.
// Returns an error listing all missing required credentials.
func (r *Resolver) ResolveAll(ctx context.Context, contract AuthContract) (map[string]*Credential, error) {
	contract.Normalize()

	resolveOrder := contract.ResolveOrder
	result := make(map[string]*Credential, len(contract.Credentials))
	var missing []string

	for _, spec := range contract.Credentials {
		cred, err := r.Resolve(ctx, spec.Name, resolveOrder)
		if err != nil {
			if spec.Required && errors.Is(err, ErrNotFound) {
				missing = append(missing, spec.Name)
				continue
			}
			if spec.Required && !errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("failed to resolve required credential %q: %w", spec.Name, err)
			}
			continue // optional credential, skip
		}
		result[spec.Name] = cred
	}

	if len(missing) > 0 {
		guide := r.buildSetupGuide(contract, missing)
		return nil, fmt.Errorf("%w: %s\n\n%s", ErrMissingRequired, strings.Join(missing, ", "), guide)
	}

	return result, nil
}

// Store saves a credential using the specified provider.
func (r *Resolver) Store(ctx context.Context, name, value, providerID string) error {
	p, ok := r.providers[providerID]
	if !ok {
		return fmt.Errorf("provider %q not registered", providerID)
	}
	if !p.Available() {
		return fmt.Errorf("provider %q not available on this system", providerID)
	}
	return p.Store(ctx, name, value)
}

// Providers returns all registered provider names.
func (r *Resolver) Providers() []string {
	names := make([]string, 0, len(r.ordered))
	for _, p := range r.ordered {
		names = append(names, p.Name())
	}
	return names
}

// AvailableProviders returns names of providers that are available on this system.
func (r *Resolver) AvailableProviders() []string {
	var names []string
	for _, p := range r.ordered {
		if p.Available() {
			names = append(names, p.Name())
		}
	}
	return names
}

// buildSetupGuide generates a contextual guide for missing credentials.
func (r *Resolver) buildSetupGuide(contract AuthContract, missing []string) string {
	var b strings.Builder

	for _, name := range missing {
		fmt.Fprintf(&b, "  %s", name)
		// Find description from contract
		for _, spec := range contract.Credentials {
			if spec.Name == name && spec.Description != "" {
				fmt.Fprintf(&b, " — %s", spec.Description)
				break
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\nOptions to configure:\n\n")

	available := r.AvailableProviders()
	for i, providerID := range available {
		p := r.providers[providerID]
		switch providerID {
		case "env":
			fmt.Fprintf(&b, "  %d. Environment variable (simplest):\n", i+1)
			for _, name := range missing {
				fmt.Fprintf(&b, "     export %s='your-value-here'\n", name)
			}
		case "encrypted-file":
			fmt.Fprintf(&b, "  %d. Encrypted file (portable, secure):\n", i+1)
			for _, name := range missing {
				fmt.Fprintf(&b, "     wtb credentials store %s --provider encrypted-file\n", name)
			}
		default:
			fmt.Fprintf(&b, "  %d. %s:\n", i+1, p.Name())
			for _, name := range missing {
				fmt.Fprintf(&b, "     wtb credentials store %s --provider %s\n", name, providerID)
			}
		}
		b.WriteString("\n")
	}

	if contract.SetupGuide != "" {
		b.WriteString("Setup guide:\n")
		b.WriteString(contract.SetupGuide)
		b.WriteString("\n")
	}

	return b.String()
}
