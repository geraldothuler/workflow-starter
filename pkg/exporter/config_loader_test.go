package exporter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigExporters_Embedded(t *testing.T) {
	exporters, err := LoadConfigExporters("", nil)
	if err != nil {
		t.Fatalf("LoadConfigExporters error: %v", err)
	}

	// Should have 3 embedded exporters: jira, linear, azure-devops
	if len(exporters) != 3 {
		t.Errorf("expected 3 embedded exporters, got %d", len(exporters))
	}

	// Check that all expected IDs exist
	ids := make(map[string]bool)
	for _, e := range exporters {
		ids[e.Name()] = true
	}
	for _, expected := range []string{"jira", "linear", "azure-devops"} {
		if !ids[expected] {
			t.Errorf("missing expected exporter: %s", expected)
		}
	}
}

func TestLoadConfigExporters_ProjectOverride(t *testing.T) {
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "exporters")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create override YAML that replaces jira
	overrideYAML := `
exporter:
  id: jira
  name: "Custom Jira"
  description: "Custom Jira override"
  auth:
    credentials:
      - name: "CUSTOM_TOKEN"
        required: true
  transport:
    type: http
    base_url: "https://custom.atlassian.net/rest/api/3"
    auth_type: bearer
    auth_value: "{{.CUSTOM_TOKEN}}"
    timeout: "60s"
  push:
    epic:
      method: POST
      path: "/issue"
      body: "{}"
    story:
      method: POST
      path: "/issue"
      body: "{}"
`
	if err := os.WriteFile(filepath.Join(overrideDir, "jira.yml"), []byte(overrideYAML), 0644); err != nil {
		t.Fatal(err)
	}

	exporters, err := LoadConfigExporters(tmpDir, nil)
	if err != nil {
		t.Fatalf("LoadConfigExporters error: %v", err)
	}

	// Should still have 3 (override replaces, doesn't add)
	if len(exporters) != 3 {
		t.Errorf("expected 3 exporters after override, got %d", len(exporters))
	}

	// Find jira exporter and check it was overridden
	for _, e := range exporters {
		if e.Name() == "jira" {
			ce := e
			if ce.config.Description != "Custom Jira override" {
				t.Errorf("jira description = %q, want 'Custom Jira override'", ce.config.Description)
			}
			return
		}
	}
	t.Error("jira exporter not found after override")
}

func TestLoadConfigExporters_PlaceholderSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "exporters")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Placeholder config (id starts with "_")
	placeholderYAML := `
exporter:
  id: _template
  name: "Template"
  transport:
    type: http
    base_url: "http://example.com"
    auth_type: bearer
    auth_value: "tok"
  push:
    epic:
      method: POST
      path: "/"
      body: "{}"
    story:
      method: POST
      path: "/"
      body: "{}"
`
	if err := os.WriteFile(filepath.Join(overrideDir, "_template.yml"), []byte(placeholderYAML), 0644); err != nil {
		t.Fatal(err)
	}

	exporters, err := LoadConfigExporters(tmpDir, nil)
	if err != nil {
		t.Fatalf("LoadConfigExporters error: %v", err)
	}

	// Placeholder should be skipped; only embedded 3 remain
	for _, e := range exporters {
		if e.Name() == "_template" {
			t.Error("placeholder exporter should have been skipped")
		}
	}
}

func TestParseExporterConfig_InvalidYAML(t *testing.T) {
	_, err := parseExporterConfig([]byte("{{invalid yaml"), "bad.yml")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseExporterConfig_ValidationError(t *testing.T) {
	// Missing required fields
	yaml := `
exporter:
  id: "test"
  name: ""
  transport:
    type: http
    base_url: "http://example.com"
    auth_type: bearer
    auth_value: "tok"
  push:
    epic:
      method: POST
      path: "/"
    story:
      method: POST
      path: "/"
`
	_, err := parseExporterConfig([]byte(yaml), "test.yml")
	if err == nil {
		t.Error("expected validation error for empty name")
	}
}

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"config.yml", true},
		{"config.yaml", true},
		{"config.YML", true},
		{"config.YAML", true},
		{"config.json", false},
		{"config.go", false},
		{"README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isYAMLFile(tt.name)
			if got != tt.want {
				t.Errorf("isYAMLFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
