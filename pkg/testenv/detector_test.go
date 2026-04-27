package testenv

import (
	"testing"
)

func TestLoadTestEnvConfig(t *testing.T) {
	cfg, err := LoadTestEnvConfig()
	if err != nil {
		t.Fatalf("LoadTestEnvConfig: %v", err)
	}
	if len(cfg.Runtimes) == 0 {
		t.Error("expected at least one runtime in config")
	}
	if len(cfg.Compose.Discovery) == 0 {
		t.Error("expected compose discovery paths")
	}
	if cfg.Health.TimeoutSeconds == 0 {
		t.Error("expected non-zero health timeout")
	}
}

func TestLoadTestEnvConfig_RuntimeNames(t *testing.T) {
	cfg, _ := LoadTestEnvConfig()
	names := map[string]bool{}
	for _, r := range cfg.Runtimes {
		names[r.Name] = true
	}
	for _, want := range []string{"docker-desktop", "colima"} {
		if !names[want] {
			t.Errorf("expected runtime %q in config", want)
		}
	}
}

func TestDetectRuntime_NoCrash(t *testing.T) {
	cfg, _ := LoadTestEnvConfig()
	// Just verify it doesn't panic — result depends on environment
	_ = DetectRuntime(cfg)
}

func TestDiscoverCompose_NotFound(t *testing.T) {
	cfg, _ := LoadTestEnvConfig()
	result := DiscoverCompose(t.TempDir(), cfg, "")
	if result != "" {
		t.Errorf("expected empty result for temp dir, got %q", result)
	}
}

func TestDiscoverCompose_Override(t *testing.T) {
	cfg, _ := LoadTestEnvConfig()
	// Override with non-existent path returns ""
	result := DiscoverCompose(t.TempDir(), cfg, "nonexistent-compose.yml")
	if result != "" {
		t.Errorf("expected empty result for missing override, got %q", result)
	}
}

func TestLoadRepoTestEnvConfig_Missing(t *testing.T) {
	cfg, err := LoadRepoTestEnvConfig(t.TempDir())
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(cfg.Services) != 0 {
		t.Error("expected empty services for missing config")
	}
}

func TestRuntimeInstallInstructions(t *testing.T) {
	cfg, _ := LoadTestEnvConfig()
	lines := RuntimeInstallInstructions(cfg)
	if len(lines) < 2 {
		t.Errorf("expected at least 2 instruction lines, got %d", len(lines))
	}
	for _, r := range cfg.Runtimes {
		found := false
		for _, l := range lines {
			if contains(l, r.InstallURL) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("install URL for %q not found in instructions", r.Name)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
