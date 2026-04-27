package credentials

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResolverConfig_EmbeddedDefaults(t *testing.T) {
	config, err := LoadResolverConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if len(config.DefaultOrder) == 0 {
		t.Error("expected non-empty default order")
	}
	// Should contain at least "env"
	found := false
	for _, p := range config.DefaultOrder {
		if p == "env" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'env' in default order")
	}
}

func TestLoadResolverConfig_ProjectOverride(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".workflow")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	overrideYAML := `
default_order:
  - encrypted-file
  - env
verbose: true
`
	if err := os.WriteFile(filepath.Join(configDir, "credentials.yml"), []byte(overrideYAML), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadResolverConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(config.DefaultOrder) != 2 {
		t.Fatalf("expected 2 items in override order, got %d", len(config.DefaultOrder))
	}
	if config.DefaultOrder[0] != "encrypted-file" {
		t.Errorf("expected first item 'encrypted-file', got %q", config.DefaultOrder[0])
	}
	if !config.Verbose {
		t.Error("expected verbose=true from override")
	}
}

func TestLoadCommandProviders_EmbeddedProviders(t *testing.T) {
	providers, err := LoadCommandProviders("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) == 0 {
		t.Fatal("expected at least one embedded provider")
	}

	// Check known provider IDs
	ids := make(map[string]bool)
	for _, p := range providers {
		ids[p.Name()] = true
	}

	expected := []string{"pass", "keyring-macos", "keyring-linux", "aws-ssm", "1password"}
	for _, id := range expected {
		if !ids[id] {
			t.Errorf("expected embedded provider %q, not found", id)
		}
	}
}

func TestLoadCommandProviders_ProjectOverride(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "credential-providers")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Override pass with different resolve command
	customYAML := `
provider:
  id: pass
  name: "Custom Pass"

  resolve:
    command: "gopass"
    args: ["show", "-o", "wtb/{{.name}}"]
    parse: "trim"

  store:
    command: "gopass"
    args: ["insert", "-f", "wtb/{{.name}}"]
    input: "{{.value}}"

  available:
    command: "gopass"
    args: ["--version"]
`
	if err := os.WriteFile(filepath.Join(overrideDir, "pass.yml"), []byte(customYAML), 0644); err != nil {
		t.Fatal(err)
	}

	providers, err := LoadCommandProviders(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the pass provider — should be the overridden one
	var passProvider Provider
	for _, p := range providers {
		if p.Name() == "pass" {
			passProvider = p
			break
		}
	}

	if passProvider == nil {
		t.Fatal("expected 'pass' provider after override")
	}
	// Verify it's a CommandProvider with overridden config
	cp, ok := passProvider.(*CommandProvider)
	if !ok {
		t.Fatalf("expected *CommandProvider, got %T", passProvider)
	}
	if cp.config.Name != "Custom Pass" {
		t.Errorf("expected overridden name 'Custom Pass', got %q", cp.config.Name)
	}
	if cp.config.Resolve.Command != "gopass" {
		t.Errorf("expected overridden command 'gopass', got %q", cp.config.Resolve.Command)
	}
}

func TestLoadCommandProviders_CustomProvider(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "credential-providers")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Add a completely custom provider
	customYAML := `
provider:
  id: vault
  name: "HashiCorp Vault"

  resolve:
    command: "vault"
    args: ["kv", "get", "-field=value", "secret/wtb/{{.name}}"]
    parse: "trim"

  store:
    command: "vault"
    args: ["kv", "put", "secret/wtb/{{.name}}", "value={{.value}}"]

  available:
    command: "vault"
    args: ["status"]

  setup_guide: "Install vault: https://www.vaultproject.io/"
`
	if err := os.WriteFile(filepath.Join(overrideDir, "vault.yml"), []byte(customYAML), 0644); err != nil {
		t.Fatal(err)
	}

	providers, err := LoadCommandProviders(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have all embedded providers + custom vault
	ids := make(map[string]bool)
	for _, p := range providers {
		ids[p.Name()] = true
	}

	if !ids["vault"] {
		t.Error("expected custom 'vault' provider")
	}
	if !ids["pass"] {
		t.Error("expected embedded 'pass' provider to still exist")
	}
}

func TestLoadCommandProviders_SkipsPlaceholders(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "credential-providers")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	placeholderYAML := `
provider:
  id: _placeholder
  name: "Placeholder"

  resolve:
    command: "echo"
    args: ["placeholder"]

  available:
    command: "echo"
    args: ["ok"]
`
	if err := os.WriteFile(filepath.Join(overrideDir, "_placeholder.yml"), []byte(placeholderYAML), 0644); err != nil {
		t.Fatal(err)
	}

	providers, err := LoadCommandProviders(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range providers {
		if p.Name() == "_placeholder" {
			t.Error("placeholder provider should have been skipped")
		}
	}
}

func TestLoadCommandProviders_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "credential-providers")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	invalidYAML := `not: valid: yaml: {{{`
	if err := os.WriteFile(filepath.Join(overrideDir, "invalid.yml"), []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadCommandProviders(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadCommandProviders_NoOverrideDir(t *testing.T) {
	dir := t.TempDir()
	// No .workflow/credential-providers dir — should just use embedded

	providers, err := LoadCommandProviders(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) == 0 {
		t.Fatal("expected at least one embedded provider")
	}
}

func TestNewFullResolver(t *testing.T) {
	dir := t.TempDir()

	resolver, err := NewFullResolver(dir, "test-master-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	providers := resolver.Providers()
	if len(providers) < 2 {
		t.Errorf("expected at least 2 providers (env + encrypted-file), got %d", len(providers))
	}

	// Should contain native providers
	providerSet := make(map[string]bool)
	for _, p := range providers {
		providerSet[p] = true
	}
	if !providerSet["env"] {
		t.Error("expected 'env' provider")
	}
	if !providerSet["encrypted-file"] {
		t.Error("expected 'encrypted-file' provider")
	}

	// Should contain command providers
	if !providerSet["pass"] {
		t.Error("expected 'pass' command provider")
	}
}

func TestNewFullResolver_NoMasterKey(t *testing.T) {
	dir := t.TempDir()

	resolver, err := NewFullResolver(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// encrypted-file should be registered but not available
	available := resolver.AvailableProviders()
	for _, p := range available {
		if p == "encrypted-file" {
			t.Error("encrypted-file should not be available without master key")
		}
	}
}
