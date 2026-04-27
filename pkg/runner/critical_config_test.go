package runner

import (
	"testing"
)

func TestLoadCriticalCommandsConfig_Embedded(t *testing.T) {
	cfg, err := LoadCriticalCommandsConfig()
	if err != nil {
		t.Fatalf("failed to load embedded critical commands config: %v", err)
	}
	if len(cfg.Commands) == 0 {
		t.Error("expected non-empty commands list")
	}

	// Verify auth is marked critical
	found := false
	for _, cmd := range cfg.Commands {
		if cmd.Name == "auth" {
			found = true
			if !cmd.Critical {
				t.Error("expected auth to be critical")
			}
			if len(cmd.BlocksOn) == 0 {
				t.Error("expected auth to have blocks_on")
			}
		}
	}
	if !found {
		t.Error("expected auth command in config")
	}
}

func TestMergedCommandSpec_EmbeddedDefault(t *testing.T) {
	cfg := &CriticalCommandsConfig{
		Commands: []CommandSpec{
			{Name: "auth", Critical: true, BlocksOn: []string{"error"}},
			{Name: "db-health", DependsOn: []string{"auth"}},
		},
	}

	spec := MergedCommandSpec("auth", cfg, nil)
	if spec == nil {
		t.Fatal("expected non-nil spec for auth")
	}
	if !spec.Critical {
		t.Error("expected critical=true from embedded")
	}
}

func TestMergedCommandSpec_StepOverride(t *testing.T) {
	cfg := &CriticalCommandsConfig{
		Commands: []CommandSpec{
			{Name: "auth", Critical: true, BlocksOn: []string{"error"}},
		},
	}

	stepSpecs := []CommandSpec{
		{Name: "auth", Critical: false}, // Override: not critical
	}

	spec := MergedCommandSpec("auth", cfg, stepSpecs)
	if spec == nil {
		t.Fatal("expected non-nil spec for auth")
	}
	if spec.Critical {
		t.Error("expected critical=false from step override")
	}
}

func TestMergedCommandSpec_Unknown(t *testing.T) {
	cfg := &CriticalCommandsConfig{
		Commands: []CommandSpec{
			{Name: "auth", Critical: true},
		},
	}

	spec := MergedCommandSpec("unknown-cmd", cfg, nil)
	if spec != nil {
		t.Error("expected nil for unknown command")
	}
}

func TestShouldSkip_DependencyFailed(t *testing.T) {
	spec := &CommandSpec{
		Name:      "db-health",
		DependsOn: []string{"auth"},
	}

	cmdStatuses := map[string]string{"auth": "error"}

	if !shouldSkip(spec, cmdStatuses) {
		t.Error("expected skip when dependency has error status")
	}
}

func TestShouldSkip_DependencyCritical(t *testing.T) {
	spec := &CommandSpec{
		Name:      "db-health",
		DependsOn: []string{"auth"},
	}

	cmdStatuses := map[string]string{"auth": "critical"}

	if !shouldSkip(spec, cmdStatuses) {
		t.Error("expected skip when dependency has critical status")
	}
}

func TestShouldSkip_DependencySkipped(t *testing.T) {
	spec := &CommandSpec{
		Name:      "db-health",
		DependsOn: []string{"auth"},
	}

	cmdStatuses := map[string]string{"auth": "skipped"}

	if !shouldSkip(spec, cmdStatuses) {
		t.Error("expected skip when dependency has skipped status")
	}
}

func TestShouldSkip_NoDependency(t *testing.T) {
	spec := &CommandSpec{
		Name: "k8s-status",
	}

	cmdStatuses := map[string]string{"auth": "error"}

	if shouldSkip(spec, cmdStatuses) {
		t.Error("expected no skip when command has no dependencies")
	}
}

func TestShouldSkip_DependencyOk(t *testing.T) {
	spec := &CommandSpec{
		Name:      "db-health",
		DependsOn: []string{"auth"},
	}

	cmdStatuses := map[string]string{"auth": "ok"}

	if shouldSkip(spec, cmdStatuses) {
		t.Error("expected no skip when dependency is ok")
	}
}

func TestShouldSkip_NilSpec(t *testing.T) {
	cmdStatuses := map[string]string{"auth": "error"}

	if shouldSkip(nil, cmdStatuses) {
		t.Error("expected no skip for nil spec")
	}
}

func TestShouldSkip_DependencyNotYetRun(t *testing.T) {
	spec := &CommandSpec{
		Name:      "db-health",
		DependsOn: []string{"auth"},
	}

	cmdStatuses := map[string]string{} // auth hasn't run yet

	if shouldSkip(spec, cmdStatuses) {
		t.Error("expected no skip when dependency hasn't run yet")
	}
}

func TestRenderSkipSignal_WithTemplate(t *testing.T) {
	spec := &CommandSpec{
		Name:       "db-health",
		DependsOn:  []string{"auth"},
		SkipSignal: "skipped (dependency: {dep} returned {status})",
	}
	cmdStatuses := map[string]string{"auth": "error"}

	got := renderSkipSignal(spec.SkipSignal, spec, cmdStatuses)
	want := "skipped (dependency: auth returned error)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderSkipSignal_DefaultTemplate(t *testing.T) {
	spec := &CommandSpec{
		Name:      "db-health",
		DependsOn: []string{"auth"},
	}
	cmdStatuses := map[string]string{"auth": "critical"}

	got := renderSkipSignal("", spec, cmdStatuses)
	want := "skipped (dependency failed)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderSkipSignal_NoFailedDep(t *testing.T) {
	spec := &CommandSpec{
		Name:      "db-health",
		DependsOn: []string{"auth"},
	}
	cmdStatuses := map[string]string{"auth": "ok"}

	got := renderSkipSignal("template {dep}", spec, cmdStatuses)
	// No failed dep found, returns template as-is
	if got != "template {dep}" {
		t.Errorf("expected template as-is when no dep failed, got %q", got)
	}
}
