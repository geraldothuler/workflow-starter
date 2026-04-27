package credentials

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestEnvProvider_Name(t *testing.T) {
	p := NewEnvProvider()
	if p.Name() != "env" {
		t.Errorf("expected name 'env', got %q", p.Name())
	}
}

func TestEnvProvider_Available(t *testing.T) {
	p := NewEnvProvider()
	if !p.Available() {
		t.Error("env provider should always be available")
	}
}

func TestEnvProvider_Resolve_Found(t *testing.T) {
	p := NewEnvProvider()
	ctx := context.Background()

	const envKey = "WTB_TEST_CRED_RESOLVE_FOUND"
	os.Setenv(envKey, "secret-value-123")
	defer os.Unsetenv(envKey)

	cred, err := p.Resolve(ctx, envKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Name != envKey {
		t.Errorf("expected name %q, got %q", envKey, cred.Name)
	}
	if cred.Value != "secret-value-123" {
		t.Errorf("expected value 'secret-value-123', got %q", cred.Value)
	}
	if cred.Source != "env" {
		t.Errorf("expected source 'env', got %q", cred.Source)
	}
}

func TestEnvProvider_Resolve_NotFound(t *testing.T) {
	p := NewEnvProvider()
	ctx := context.Background()

	const envKey = "WTB_TEST_CRED_NONEXISTENT_KEY_XYZ"
	os.Unsetenv(envKey)

	_, err := p.Resolve(ctx, envKey)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEnvProvider_Resolve_EmptyValue(t *testing.T) {
	p := NewEnvProvider()
	ctx := context.Background()

	const envKey = "WTB_TEST_CRED_EMPTY_VALUE"
	os.Setenv(envKey, "")
	defer os.Unsetenv(envKey)

	_, err := p.Resolve(ctx, envKey)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for empty value, got %v", err)
	}
}

func TestEnvProvider_Store(t *testing.T) {
	p := NewEnvProvider()
	ctx := context.Background()

	const envKey = "WTB_TEST_CRED_STORE"
	defer os.Unsetenv(envKey)

	if err := p.Store(ctx, envKey, "stored-value"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it can be resolved
	cred, err := p.Resolve(ctx, envKey)
	if err != nil {
		t.Fatalf("unexpected error on resolve: %v", err)
	}
	if cred.Value != "stored-value" {
		t.Errorf("expected 'stored-value', got %q", cred.Value)
	}
}
