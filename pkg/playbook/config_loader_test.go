package playbook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPlaybookConfigs_Embedded(t *testing.T) {
	specs, err := LoadPlaybookConfigs("")
	if err != nil {
		t.Fatalf("failed to load configs: %v", err)
	}

	if len(specs) < 1 {
		t.Fatalf("expected at least 1 playbook config, got %d", len(specs))
	}

	cdc, ok := specs["cdc-replication-check"]
	if !ok {
		t.Fatal("expected cdc-replication-check playbook")
	}

	if cdc.Title != "CDC Replication Health Investigation" {
		t.Errorf("title = %q", cdc.Title)
	}
	if len(cdc.RequiredProviders) != 1 {
		t.Errorf("expected 1 required provider, got %d", len(cdc.RequiredProviders))
	}
	if cdc.RequiredProviders[0].ID != "postgresql" {
		t.Errorf("required provider = %q, want postgresql", cdc.RequiredProviders[0].ID)
	}
	if len(cdc.OptionalProviders) != 2 {
		t.Errorf("expected 2 optional providers, got %d", len(cdc.OptionalProviders))
	}
	if len(cdc.Steps) != 5 {
		t.Errorf("expected 5 steps, got %d", len(cdc.Steps))
	}

	// Verify step IDs
	stepIDs := make(map[string]bool)
	for _, step := range cdc.Steps {
		stepIDs[step.ID] = true
	}
	for _, expected := range []string{
		"check_replication_slots", "check_pg_connections",
		"check_kafka_connectors", "check_kafka_consumers", "check_pipeline_pods",
	} {
		if !stepIDs[expected] {
			t.Errorf("missing step: %s", expected)
		}
	}

	// Verify optional steps
	for _, step := range cdc.Steps {
		switch step.ID {
		case "check_pg_connections", "check_kafka_connectors",
			"check_kafka_consumers", "check_pipeline_pods":
			if !step.Optional {
				t.Errorf("step %q should be optional", step.ID)
			}
		case "check_replication_slots":
			if step.Optional {
				t.Errorf("step %q should not be optional", step.ID)
			}
		}
	}
}

func TestLoadPlaybookConfigs_Override(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "playbooks")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	override := []byte(`playbook:
  id: cdc-replication-check
  title: "Custom CDC Check"
  steps:
    - id: custom_step
      provider: postgresql
      analyzers:
        - name: analyze_inactive_slots
`)
	if err := os.WriteFile(filepath.Join(overrideDir, "cdc-custom.yml"), override, 0644); err != nil {
		t.Fatal(err)
	}

	specs, err := LoadPlaybookConfigs(dir)
	if err != nil {
		t.Fatalf("failed to load configs: %v", err)
	}

	cdc, ok := specs["cdc-replication-check"]
	if !ok {
		t.Fatal("expected cdc-replication-check playbook")
	}
	if cdc.Title != "Custom CDC Check" {
		t.Errorf("title = %q, want Custom CDC Check", cdc.Title)
	}
	if len(cdc.Steps) != 1 {
		t.Errorf("expected 1 step (override), got %d", len(cdc.Steps))
	}
}

func TestLoadPlaybook_ByID(t *testing.T) {
	spec, err := LoadPlaybook("cdc-replication-check", "")
	if err != nil {
		t.Fatalf("failed to load playbook: %v", err)
	}
	if spec.ID != "cdc-replication-check" {
		t.Errorf("id = %q", spec.ID)
	}
}

func TestLoadPlaybook_NotFound(t *testing.T) {
	_, err := LoadPlaybook("nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent playbook")
	}
}

func TestLoadPlaybookConfigs_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "playbooks")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write invalid YAML - should be skipped, not cause error
	if err := os.WriteFile(filepath.Join(overrideDir, "bad.yml"), []byte("{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	specs, err := LoadPlaybookConfigs(dir)
	if err != nil {
		t.Fatalf("invalid YAML should be skipped, got error: %v", err)
	}

	// Should still have the embedded playbook
	if _, ok := specs["cdc-replication-check"]; !ok {
		t.Error("expected embedded playbook to still be loaded")
	}
}
