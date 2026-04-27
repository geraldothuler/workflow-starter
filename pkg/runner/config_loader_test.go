package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResolveConfig_Embedded(t *testing.T) {
	cfg, err := LoadResolveConfig("")
	if err != nil {
		t.Fatalf("failed to load embedded config: %v", err)
	}
	if len(cfg.Resolve.DefaultChain) == 0 {
		t.Error("expected non-empty default chain")
	}
	if _, ok := cfg.Resolve.Strategies["session"]; !ok {
		t.Error("expected session strategy in embedded config")
	}
	if _, ok := cfg.Resolve.Strategies["helm"]; !ok {
		t.Error("expected helm strategy in embedded config")
	}
}

func TestLoadResolveConfig_ProjectOverride(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow")
	os.MkdirAll(overrideDir, 0755)

	override := `
resolve:
  default_chain: [ask]
  strategies:
    custom:
      description: "custom strategy"
`
	os.WriteFile(filepath.Join(overrideDir, "resolve_config.yml"), []byte(override), 0644)

	cfg, err := LoadResolveConfig(dir)
	if err != nil {
		t.Fatalf("failed to load config with override: %v", err)
	}

	// Override chain should win
	if len(cfg.Resolve.DefaultChain) != 1 || cfg.Resolve.DefaultChain[0] != "ask" {
		t.Errorf("expected override chain [ask], got %v", cfg.Resolve.DefaultChain)
	}

	// Custom strategy should be merged
	if _, ok := cfg.Resolve.Strategies["custom"]; !ok {
		t.Error("expected custom strategy from override")
	}

	// Original strategies should still be present
	if _, ok := cfg.Resolve.Strategies["session"]; !ok {
		t.Error("expected session strategy preserved from embedded")
	}
}
