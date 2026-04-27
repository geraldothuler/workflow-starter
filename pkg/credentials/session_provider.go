package credentials

import (
	"context"
	"fmt"
	"os/exec"
)

// SessionCommandProvider extends CommandProvider with session management:
// pre_check (validate existing session), refresh (re-authenticate), and TTL caching.
//
// Resolution flow:
//  1. TTL cache check → if valid, return cached
//  2. pre_check       → if exit 0, skip to step 4 (session still alive)
//  3. refresh         → interactive: true → attach terminal → browser/MFA/prompt
//  4. resolve         → execute command, parse output
//  5. cache result    → store in memory with TTL expiration
//  6. return Credential{Value, Source, ExpiresAt}
type SessionCommandProvider struct {
	config CommandProviderSpec
	cache  *SessionCache
	runner *InteractiveRunner

	// execCommand is the function used to create commands.
	// Defaults to exec.CommandContext; override in tests.
	execCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewSessionCommandProvider creates a session-aware credential provider.
func NewSessionCommandProvider(config CommandProviderSpec, cache *SessionCache) *SessionCommandProvider {
	if cache == nil {
		cache = NewSessionCache()
	}
	return &SessionCommandProvider{
		config:      config,
		cache:       cache,
		runner:      NewInteractiveRunner(),
		execCommand: exec.CommandContext,
	}
}

// Name returns the provider identifier from config.
func (p *SessionCommandProvider) Name() string { return p.config.ID }

// Resolve implements the session credential resolution flow.
func (p *SessionCommandProvider) Resolve(ctx context.Context, name string) (*Credential, error) {
	cacheKey := p.buildCacheKey(name)

	// Step 1: TTL cache check
	if cached := p.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	// Step 2: Pre-check (is session still alive?)
	needsRefresh := true
	if p.config.PreCheck != nil {
		if err := p.runPreCheck(ctx, name); err == nil {
			needsRefresh = false // session is still valid
		}
	}

	// Step 3: Refresh if needed
	if needsRefresh && p.config.Refresh != nil {
		if err := p.runRefresh(ctx); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRefreshFailed, err)
		}
	}

	// Step 4: Resolve the credential value
	spec := p.config.Resolve
	output, err := RunCommand(ctx, p.execCommand, spec, map[string]string{"name": name})
	if err != nil {
		return nil, err
	}

	value, err := parseOutput(output, spec.Parse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if value == "" {
		return nil, ErrNotFound
	}

	cred := &Credential{
		Name:   name,
		Value:  value,
		Source: p.config.ID,
	}

	// Step 5: Cache with TTL
	if p.config.Session != nil && p.config.Session.TTL > 0 {
		p.cache.Set(cacheKey, cred, p.config.Session.TTL)
	}

	return cred, nil
}

// Store delegates to the underlying command spec (same as CommandProvider).
func (p *SessionCommandProvider) Store(ctx context.Context, name, value string) error {
	spec := p.config.Store
	if spec.Command == "" {
		return ErrUnsupported
	}

	vars := map[string]string{"name": name, "value": value}
	_, err := RunCommand(ctx, p.execCommand, spec, vars)
	if err != nil {
		return fmt.Errorf("store failed: %w", err)
	}
	return nil
}

// Available checks if the external tool is installed.
func (p *SessionCommandProvider) Available() bool {
	spec := p.config.Available
	ctx, cancel := context.WithTimeout(context.Background(), spec.Timeout)
	defer cancel()

	cmd := p.execCommand(ctx, spec.Command, spec.Args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run() == nil
}

// InvalidateCache removes the cached session for a specific credential name.
func (p *SessionCommandProvider) InvalidateCache(name string) {
	p.cache.Invalidate(p.buildCacheKey(name))
}

// InvalidateAllCache clears all cached sessions for this provider.
func (p *SessionCommandProvider) InvalidateAllCache() {
	p.cache.InvalidateAll()
}

// --- internal ---

// buildCacheKey creates the cache key for a credential.
// Uses the session.cache_key template if available, otherwise falls back to "providerID:name".
func (p *SessionCommandProvider) buildCacheKey(name string) string {
	if p.config.Session != nil && p.config.Session.CacheKey != "" {
		key, err := expandTemplate(p.config.Session.CacheKey, map[string]string{"name": name})
		if err == nil && key != "" {
			return key
		}
	}
	return p.config.ID + ":" + name
}

// runPreCheck executes the pre_check command to verify if an existing session is still valid.
// Returns nil if session is valid (exit code 0), error otherwise.
func (p *SessionCommandProvider) runPreCheck(ctx context.Context, name string) error {
	spec := *p.config.PreCheck

	args, err := expandArgs(spec.Args, map[string]string{"name": name})
	if err != nil {
		return fmt.Errorf("pre_check args expansion failed: %w", err)
	}

	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 10_000_000_000 // 10s in nanoseconds as Duration
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := p.execCommand(ctx, spec.Command, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run()
}

// runRefresh executes the refresh command to re-authenticate.
// If refresh.interactive is true, uses InteractiveRunner to attach terminal.
func (p *SessionCommandProvider) runRefresh(ctx context.Context) error {
	refresh := p.config.Refresh

	timeout := refresh.Timeout
	if timeout == 0 {
		timeout = 120_000_000_000 // 120s default
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args, err := expandArgs(refresh.Args, nil)
	if err != nil {
		return fmt.Errorf("refresh args expansion failed: %w", err)
	}

	if refresh.Interactive {
		// Attach terminal for SSO/MFA flows
		return p.runner.Run(ctx, refresh.Command, args)
	}

	// Non-interactive refresh
	cmd := p.execCommand(ctx, refresh.Command, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run()
}
