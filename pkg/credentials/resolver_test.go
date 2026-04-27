package credentials

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestResolver_Resolve_SingleProvider(t *testing.T) {
	mock := NewMockProvider(map[string]string{
		"MY_TOKEN": "secret123",
	})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	cred, err := resolver.Resolve(ctx, "MY_TOKEN", []string{"mock"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "secret123" {
		t.Errorf("expected 'secret123', got %q", cred.Value)
	}
	if cred.Source != "mock" {
		t.Errorf("expected source 'mock', got %q", cred.Source)
	}
}

func TestResolver_Resolve_FallbackChain(t *testing.T) {
	// First provider doesn't have it, second does
	first := &MockProvider{ProviderName: "first", Credentials: map[string]string{}, AvailableVal: true}
	second := &MockProvider{ProviderName: "second", Credentials: map[string]string{"TOKEN": "from-second"}, AvailableVal: true}

	resolver := NewResolver(nil, first, second)
	ctx := context.Background()

	cred, err := resolver.Resolve(ctx, "TOKEN", []string{"first", "second"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != "second" {
		t.Errorf("expected source 'second', got %q", cred.Source)
	}
	if cred.Value != "from-second" {
		t.Errorf("expected 'from-second', got %q", cred.Value)
	}
}

func TestResolver_Resolve_FirstWins(t *testing.T) {
	// Both providers have it, first wins
	first := &MockProvider{ProviderName: "first", Credentials: map[string]string{"TOKEN": "from-first"}, AvailableVal: true}
	second := &MockProvider{ProviderName: "second", Credentials: map[string]string{"TOKEN": "from-second"}, AvailableVal: true}

	resolver := NewResolver(nil, first, second)
	ctx := context.Background()

	cred, err := resolver.Resolve(ctx, "TOKEN", []string{"first", "second"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != "first" {
		t.Errorf("expected source 'first', got %q", cred.Source)
	}
}

func TestResolver_Resolve_NotFound(t *testing.T) {
	mock := NewMockProvider(map[string]string{})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, "NONEXISTENT", []string{"mock"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolver_Resolve_SkipsUnavailable(t *testing.T) {
	unavailable := &MockProvider{ProviderName: "unavail", Credentials: map[string]string{"TOKEN": "nope"}, AvailableVal: false}
	available := &MockProvider{ProviderName: "avail", Credentials: map[string]string{"TOKEN": "yes"}, AvailableVal: true}

	resolver := NewResolver(nil, unavailable, available)
	ctx := context.Background()

	cred, err := resolver.Resolve(ctx, "TOKEN", []string{"unavail", "avail"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != "avail" {
		t.Errorf("expected source 'avail', got %q", cred.Source)
	}
}

func TestResolver_Resolve_UsesDefaultOrder(t *testing.T) {
	config := &ResolverConfig{
		DefaultOrder: []string{"secondary"},
	}
	primary := &MockProvider{ProviderName: "primary", Credentials: map[string]string{"TOKEN": "from-primary"}, AvailableVal: true}
	secondary := &MockProvider{ProviderName: "secondary", Credentials: map[string]string{"TOKEN": "from-secondary"}, AvailableVal: true}

	resolver := NewResolver(config, primary, secondary)
	ctx := context.Background()

	// Pass nil resolveOrder → uses DefaultOrder → only tries "secondary"
	cred, err := resolver.Resolve(ctx, "TOKEN", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != "secondary" {
		t.Errorf("expected source 'secondary', got %q", cred.Source)
	}
}

func TestResolver_ResolveAll_AllFound(t *testing.T) {
	mock := NewMockProvider(map[string]string{
		"TOKEN_A": "val-a",
		"TOKEN_B": "val-b",
	})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	contract := AuthContract{
		Credentials: []CredentialSpec{
			{Name: "TOKEN_A", Required: true},
			{Name: "TOKEN_B", Required: true},
		},
		ResolveOrder: []string{"mock"},
	}

	result, err := resolver.ResolveAll(ctx, contract)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result["TOKEN_A"].Value != "val-a" {
		t.Errorf("expected TOKEN_A='val-a', got %q", result["TOKEN_A"].Value)
	}
}

func TestResolver_ResolveAll_MissingRequired(t *testing.T) {
	mock := NewMockProvider(map[string]string{
		"TOKEN_A": "val-a",
	})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	contract := AuthContract{
		Credentials: []CredentialSpec{
			{Name: "TOKEN_A", Required: true},
			{Name: "TOKEN_B", Required: true, Description: "B token"},
		},
		ResolveOrder: []string{"mock"},
	}

	_, err := resolver.ResolveAll(ctx, contract)
	if err == nil {
		t.Fatal("expected error for missing required credential")
	}
	if !errors.Is(err, ErrMissingRequired) {
		t.Errorf("expected ErrMissingRequired, got %v", err)
	}
	if !strings.Contains(err.Error(), "TOKEN_B") {
		t.Errorf("expected error to mention TOKEN_B, got: %s", err.Error())
	}
}

func TestResolver_ResolveAll_OptionalMissing(t *testing.T) {
	mock := NewMockProvider(map[string]string{
		"TOKEN_A": "val-a",
	})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	contract := AuthContract{
		Credentials: []CredentialSpec{
			{Name: "TOKEN_A", Required: true},
			{Name: "OPTIONAL_TOKEN", Required: false},
		},
		ResolveOrder: []string{"mock"},
	}

	result, err := resolver.ResolveAll(ctx, contract)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 result (optional missing), got %d", len(result))
	}
}

func TestResolver_ResolveAll_LegacyEnvVar(t *testing.T) {
	mock := NewMockProvider(map[string]string{
		"FIGMA_ACCESS_TOKEN": "figd_test",
	})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	// Legacy format
	contract := AuthContract{
		EnvVar: "FIGMA_ACCESS_TOKEN",
	}

	result, err := resolver.ResolveAll(ctx, contract)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["FIGMA_ACCESS_TOKEN"].Value != "figd_test" {
		t.Errorf("expected 'figd_test', got %q", result["FIGMA_ACCESS_TOKEN"].Value)
	}
}

func TestResolver_ResolveAll_LegacyEnvVars(t *testing.T) {
	mock := NewMockProvider(map[string]string{
		"JIRA_API_TOKEN": "jira-xxx",
		"JIRA_BASE_URL":  "https://test.atlassian.net",
	})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	// Legacy multi env_vars format
	contract := AuthContract{
		EnvVars: []string{"JIRA_API_TOKEN", "JIRA_BASE_URL"},
	}

	result, err := resolver.ResolveAll(ctx, contract)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
}

func TestResolver_Store(t *testing.T) {
	mock := NewMockProvider(map[string]string{})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	if err := resolver.Store(ctx, "NEW_TOKEN", "value", "mock"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.StoreCalls) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(mock.StoreCalls))
	}
	if mock.StoreCalls[0].Name != "NEW_TOKEN" {
		t.Errorf("expected name 'NEW_TOKEN', got %q", mock.StoreCalls[0].Name)
	}

	// Should now be resolvable
	cred, err := resolver.Resolve(ctx, "NEW_TOKEN", []string{"mock"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "value" {
		t.Errorf("expected 'value', got %q", cred.Value)
	}
}

func TestResolver_Store_UnknownProvider(t *testing.T) {
	resolver := NewResolver(nil)
	ctx := context.Background()

	err := resolver.Store(ctx, "TOKEN", "value", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestResolver_Providers(t *testing.T) {
	p1 := &MockProvider{ProviderName: "alpha", AvailableVal: true}
	p2 := &MockProvider{ProviderName: "beta", AvailableVal: false}

	resolver := NewResolver(nil, p1, p2)

	providers := resolver.Providers()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	available := resolver.AvailableProviders()
	if len(available) != 1 {
		t.Fatalf("expected 1 available provider, got %d", len(available))
	}
	if available[0] != "alpha" {
		t.Errorf("expected 'alpha', got %q", available[0])
	}
}

func TestResolver_SetupGuide_ContainsMissingNames(t *testing.T) {
	mock := NewMockProvider(map[string]string{})
	resolver := NewResolver(nil, mock)
	ctx := context.Background()

	contract := AuthContract{
		Credentials: []CredentialSpec{
			{Name: "SECRET_TOKEN", Required: true, Description: "A very secret token"},
		},
		ResolveOrder: []string{"mock"},
		SetupGuide:   "Visit example.com to get your token",
	}

	_, err := resolver.ResolveAll(ctx, contract)
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "SECRET_TOKEN") {
		t.Errorf("expected error to contain credential name, got: %s", errStr)
	}
	if !strings.Contains(errStr, "example.com") {
		t.Errorf("expected error to contain setup guide, got: %s", errStr)
	}
}
