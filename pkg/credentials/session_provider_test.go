package credentials

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func newTestSessionSpec() CommandProviderSpec {
	return CommandProviderSpec{
		ID:   "test-session",
		Name: "Test Session Provider",
		Type: "session",
		Session: &SessionConfig{
			TTLStr:   "1h",
			TTL:      1 * time.Hour,
			CacheKey: "test-{{.name}}",
		},
		PreCheck: &CommandSpec{
			Command: "gh",
			Args:    []string{"auth", "status"},
			Timeout: 5 * time.Second,
		},
		Refresh: &RefreshSpec{
			Command:     "gh",
			Args:        []string{"auth", "login", "--web"},
			Interactive: false, // non-interactive for tests
			Timeout:     10 * time.Second,
		},
		Resolve: CommandSpec{
			Command: "gh",
			Args:    []string{"auth", "token"},
			Parse:   "trim",
			Timeout: 5 * time.Second,
		},
		Available: CommandSpec{
			Command: "gh",
			Args:    []string{"--version"},
			Timeout: 5 * time.Second,
		},
	}
}

func TestSessionProvider_Name(t *testing.T) {
	p := NewSessionCommandProvider(newTestSessionSpec(), nil)
	if p.Name() != "test-session" {
		t.Errorf("expected 'test-session', got %q", p.Name())
	}
}

func TestSessionProvider_Resolve_CacheMiss_PreCheckPass(t *testing.T) {
	// Pre-check passes (session alive), resolve succeeds
	spec := newTestSessionSpec()
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount <= 1 {
			// pre_check: success (exit 0)
			return mockExecCommand(0, "", "")(ctx, name, args...)
		}
		// resolve: return token
		return mockExecCommand(0, "ghp_test123\n", "")(ctx, name, args...)
	}

	cred, err := p.Resolve(context.Background(), "GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "ghp_test123" {
		t.Errorf("expected 'ghp_test123', got %q", cred.Value)
	}
	if cred.Source != "test-session" {
		t.Errorf("expected source 'test-session', got %q", cred.Source)
	}
	if cred.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestSessionProvider_Resolve_CacheHit(t *testing.T) {
	spec := newTestSessionSpec()
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	// Pre-populate cache
	cache.Set("test-GITHUB_TOKEN", &Credential{
		Name:   "GITHUB_TOKEN",
		Value:  "cached-token",
		Source: "test-session",
	}, 1*time.Hour)

	// execCommand should NOT be called (cache hit)
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		t.Fatal("execCommand should not be called on cache hit")
		return nil
	}

	cred, err := p.Resolve(context.Background(), "GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "cached-token" {
		t.Errorf("expected 'cached-token', got %q", cred.Value)
	}
}

func TestSessionProvider_Resolve_CacheExpired_RefreshNeeded(t *testing.T) {
	spec := newTestSessionSpec()
	cache := NewSessionCache()
	now := time.Now()
	cache.nowFunc = func() time.Time { return now }

	p := NewSessionCommandProvider(spec, cache)

	// Pre-populate cache with entry that will expire
	cache.Set("test-TOKEN", &Credential{Value: "old"}, 30*time.Minute)

	// Advance time past TTL
	cache.nowFunc = func() time.Time { return now.Add(1 * time.Hour) }

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// pre_check: fail (session expired)
			return mockExecCommand(1, "", "not authenticated")(ctx, name, args...)
		}
		if callCount == 2 {
			// refresh: success
			return mockExecCommand(0, "", "")(ctx, name, args...)
		}
		// resolve: return new token
		return mockExecCommand(0, "new-token\n", "")(ctx, name, args...)
	}

	cred, err := p.Resolve(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "new-token" {
		t.Errorf("expected 'new-token', got %q", cred.Value)
	}
	if callCount != 3 {
		t.Errorf("expected 3 commands (pre_check, refresh, resolve), got %d", callCount)
	}
}

func TestSessionProvider_Resolve_NoPreCheck(t *testing.T) {
	spec := newTestSessionSpec()
	spec.PreCheck = nil // no pre_check configured

	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// refresh (always needed without pre_check)
			return mockExecCommand(0, "", "")(ctx, name, args...)
		}
		// resolve
		return mockExecCommand(0, "token-value\n", "")(ctx, name, args...)
	}

	cred, err := p.Resolve(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "token-value" {
		t.Errorf("expected 'token-value', got %q", cred.Value)
	}
}

func TestSessionProvider_Resolve_NoRefresh(t *testing.T) {
	spec := newTestSessionSpec()
	spec.Refresh = nil // no refresh configured

	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// pre_check: fail
			return mockExecCommand(1, "", "expired")(ctx, name, args...)
		}
		// resolve (no refresh available, just try resolve anyway)
		return mockExecCommand(0, "direct-token\n", "")(ctx, name, args...)
	}

	cred, err := p.Resolve(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "direct-token" {
		t.Errorf("expected 'direct-token', got %q", cred.Value)
	}
}

func TestSessionProvider_Resolve_RefreshFailed(t *testing.T) {
	spec := newTestSessionSpec()
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// pre_check: fail
			return mockExecCommand(1, "", "expired")(ctx, name, args...)
		}
		// refresh: fail
		return mockExecCommand(1, "", "authentication error")(ctx, name, args...)
	}

	_, err := p.Resolve(context.Background(), "TOKEN")
	if err == nil {
		t.Fatal("expected error when refresh fails")
	}
	if !errors.Is(err, ErrRefreshFailed) {
		t.Errorf("expected ErrRefreshFailed, got %v", err)
	}
}

func TestSessionProvider_Resolve_ResolveNotFound(t *testing.T) {
	spec := newTestSessionSpec()
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// pre_check: pass
			return mockExecCommand(0, "", "")(ctx, name, args...)
		}
		// resolve: fail (not found)
		return mockExecCommand(1, "", "not found")(ctx, name, args...)
	}

	_, err := p.Resolve(context.Background(), "TOKEN")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionProvider_Resolve_EmptyOutput(t *testing.T) {
	spec := newTestSessionSpec()
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// pre_check: pass
			return mockExecCommand(0, "", "")(ctx, name, args...)
		}
		// resolve: empty output
		return mockExecCommand(0, "", "")(ctx, name, args...)
	}

	_, err := p.Resolve(context.Background(), "TOKEN")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for empty output, got %v", err)
	}
}

func TestSessionProvider_Store(t *testing.T) {
	spec := newTestSessionSpec()
	spec.Store = CommandSpec{
		Command: "gh",
		Args:    []string{"secret", "set", "{{.name}}"},
		Input:   "{{.value}}",
		Timeout: 5 * time.Second,
	}

	p := NewSessionCommandProvider(spec, nil)
	p.execCommand = mockExecCommand(0, "", "")

	err := p.Store(context.Background(), "MY_SECRET", "value123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionProvider_Store_Unsupported(t *testing.T) {
	spec := newTestSessionSpec()
	spec.Store = CommandSpec{} // no store
	p := NewSessionCommandProvider(spec, nil)

	err := p.Store(context.Background(), "TOKEN", "value")
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("expected ErrUnsupported, got %v", err)
	}
}

func TestSessionProvider_Available(t *testing.T) {
	spec := newTestSessionSpec()
	p := NewSessionCommandProvider(spec, nil)
	p.execCommand = mockExecCommand(0, "gh version 2.0", "")

	if !p.Available() {
		t.Error("expected Available() = true")
	}
}

func TestSessionProvider_Available_False(t *testing.T) {
	spec := newTestSessionSpec()
	p := NewSessionCommandProvider(spec, nil)
	p.execCommand = mockExecCommand(1, "", "command not found")

	if p.Available() {
		t.Error("expected Available() = false")
	}
}

func TestSessionProvider_InvalidateCache(t *testing.T) {
	spec := newTestSessionSpec()
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	cache.Set("test-TOKEN", &Credential{Value: "cached"}, 1*time.Hour)

	p.InvalidateCache("TOKEN")

	if cache.Get("test-TOKEN") != nil {
		t.Error("expected cache to be invalidated")
	}
}

func TestSessionProvider_InvalidateAllCache(t *testing.T) {
	spec := newTestSessionSpec()
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	cache.Set("key1", &Credential{Value: "v1"}, 1*time.Hour)
	cache.Set("key2", &Credential{Value: "v2"}, 1*time.Hour)

	p.InvalidateAllCache()

	if cache.Size() != 0 {
		t.Error("expected cache to be fully invalidated")
	}
}

func TestSessionProvider_CacheKeyTemplate(t *testing.T) {
	spec := newTestSessionSpec()
	spec.Session.CacheKey = "github-{{.name}}"
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return mockExecCommand(0, "", "")(ctx, name, args...) // pre_check pass
		}
		return mockExecCommand(0, "token123\n", "")(ctx, name, args...) // resolve
	}

	_, err := p.Resolve(context.Background(), "MY_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache was populated with the template key
	if cache.Get("github-MY_TOKEN") == nil {
		t.Error("expected cache entry with key 'github-MY_TOKEN'")
	}
}

func TestSessionProvider_DefaultCacheKey(t *testing.T) {
	spec := newTestSessionSpec()
	spec.Session.CacheKey = "" // empty = fallback to providerID:name
	cache := NewSessionCache()
	p := NewSessionCommandProvider(spec, cache)

	callCount := 0
	p.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return mockExecCommand(0, "", "")(ctx, name, args...)
		}
		return mockExecCommand(0, "tok\n", "")(ctx, name, args...)
	}

	_, err := p.Resolve(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cache.Get("test-session:TOKEN") == nil {
		t.Error("expected cache entry with key 'test-session:TOKEN'")
	}
}

// --- Validation tests for session spec ---

func TestCommandProviderSpec_Validate_SessionType(t *testing.T) {
	t.Run("valid session spec", func(t *testing.T) {
		spec := newTestSessionSpec()
		if err := spec.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.Session.TTL != 1*time.Hour {
			t.Errorf("expected TTL 1h, got %v", spec.Session.TTL)
		}
	})

	t.Run("session without session config", func(t *testing.T) {
		spec := newTestSessionSpec()
		spec.Session = nil
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for session type without session config")
		}
	})

	t.Run("session without TTL", func(t *testing.T) {
		spec := newTestSessionSpec()
		spec.Session.TTLStr = ""
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for session type without TTL")
		}
	})

	t.Run("invalid TTL format", func(t *testing.T) {
		spec := newTestSessionSpec()
		spec.Session.TTLStr = "invalid"
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for invalid TTL format")
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		spec := newTestSessionSpec()
		spec.Type = "invalid"
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for invalid type")
		}
	})

	t.Run("session with refresh timeout", func(t *testing.T) {
		spec := newTestSessionSpec()
		spec.Refresh.TimeoutStr = "5m"
		if err := spec.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.Refresh.Timeout != 5*time.Minute {
			t.Errorf("expected refresh timeout 5m, got %v", spec.Refresh.Timeout)
		}
	})

	t.Run("default cache_key", func(t *testing.T) {
		spec := newTestSessionSpec()
		spec.Session.CacheKey = ""
		if err := spec.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.Session.CacheKey != "test-session" {
			t.Errorf("expected default cache_key 'test-session', got %q", spec.Session.CacheKey)
		}
	})

	t.Run("IsSession helper", func(t *testing.T) {
		spec := newTestSessionSpec()
		if !spec.IsSession() {
			t.Error("expected IsSession() = true for type=session")
		}

		staticSpec := newTestSpec()
		if staticSpec.IsSession() {
			t.Error("expected IsSession() = false for static spec")
		}
	})
}

// --- Config loader: session type detection ---

func TestLoadCommandProviders_SessionTypeDetection(t *testing.T) {
	providers, err := LoadCommandProviders("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// github-cli should be a SessionCommandProvider
	for _, p := range providers {
		if p.Name() == "github-cli" {
			if _, ok := p.(*SessionCommandProvider); !ok {
				t.Errorf("expected github-cli to be *SessionCommandProvider, got %T", p)
			}
			return
		}
	}
	t.Error("expected github-cli provider to be loaded")
}
