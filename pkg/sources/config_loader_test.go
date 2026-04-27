package sources

import (
	"os"
	"path/filepath"
	"testing"
)

// embeddedSourceCount is the number of real (non-placeholder) embedded sources.
// Updated when new YAML configs are added to pkg/sources/config/.
const embeddedSourceCount = 2 // figma + miro

// --- LoadConfigSources ---

func TestLoadConfigSources_EmbeddedOnly(t *testing.T) {
	// Load only embedded configs (no project dir)
	sources, err := LoadConfigSources("", nil)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	if len(sources) != embeddedSourceCount {
		t.Errorf("Expected %d embedded sources, got %d", embeddedSourceCount, len(sources))
	}

	// Verify figma and miro are present
	names := make(map[string]bool)
	for _, s := range sources {
		names[s.Name()] = true
	}
	if !names["figma"] {
		t.Error("Expected 'figma' embedded source")
	}
	if !names["miro"] {
		t.Error("Expected 'miro' embedded source")
	}
}

func TestLoadConfigSources_ProjectOverride(t *testing.T) {
	// Create temp project dir with override
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	// Write a valid source YAML
	sourceYAML := `source:
  id: test-override
  name: "Test Override"
  url_patterns:
    - "testoverride\\.com/"
  url_parser:
    regex: "testoverride\\.com/item/([^/]+)"
    captures:
      item_id: 1
  transport:
    type: mcp
    command: "test-cmd"
  auth:
    env_var: "TEST_TOKEN"
    setup_guide: "Set TEST_TOKEN"
  fetch_steps:
    - tool: "get_data"
  markdown:
    mode: walker
`
	if err := os.WriteFile(filepath.Join(overrideDir, "test.yml"), []byte(sourceYAML), 0644); err != nil {
		t.Fatalf("Failed to write override: %v", err)
	}

	sources, err := LoadConfigSources(tmpDir, nil)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	// Should have embedded + 1 override
	if len(sources) != embeddedSourceCount+1 {
		t.Fatalf("Expected %d sources (embedded + 1 override), got %d", embeddedSourceCount+1, len(sources))
	}

	// Verify override is present
	hasOverride := false
	for _, s := range sources {
		if s.Name() == "test-override" {
			hasOverride = true
		}
	}
	if !hasOverride {
		t.Error("Expected 'test-override' source")
	}
}

func TestLoadConfigSources_OverrideReplacesEmbedded(t *testing.T) {
	// Verify that a project override with the same ID replaces the embedded one.
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	// Override figma with different setup_guide
	overrideYAML := `source:
  id: figma
  name: "Figma"
  url_patterns:
    - "figma\\.com/(file|design)/"
  url_parser:
    regex: "figma\\.com/(?:file|design)/([^/]+)"
    captures:
      file_key: 1
  transport:
    type: mcp
    command: "npx"
  auth:
    env_var: "FIGMA_ACCESS_TOKEN"
    setup_guide: "CUSTOM OVERRIDE GUIDE"
  fetch_steps:
    - tool: "get_file"
      args: { file_key: "{{.file_key}}" }
  markdown:
    mode: walker
`
	if err := os.WriteFile(filepath.Join(overrideDir, "figma.yml"), []byte(overrideYAML), 0644); err != nil {
		t.Fatalf("Failed to write override: %v", err)
	}

	sources, err := LoadConfigSources(tmpDir, nil)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	// Same count — override replaces, doesn't add
	if len(sources) != embeddedSourceCount {
		t.Fatalf("Expected %d sources (override replaces, not adds), got %d", embeddedSourceCount, len(sources))
	}

	// Verify the figma source has the custom guide
	for _, s := range sources {
		if s.Name() == "figma" {
			if !contains(s.SetupGuide(), "CUSTOM OVERRIDE GUIDE") {
				t.Errorf("Expected overridden setup guide, got: %s", s.SetupGuide())
			}
			return
		}
	}
	t.Error("Expected to find figma source")
}

func TestLoadConfigSources_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(overrideDir, "bad.yml"), []byte("not: valid: yaml: [[["), 0644); err != nil {
		t.Fatalf("Failed to write bad YAML: %v", err)
	}

	_, err := LoadConfigSources(tmpDir, nil)
	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}
}

func TestLoadConfigSources_InvalidSpec(t *testing.T) {
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	// Valid YAML but missing required fields
	invalidYAML := `source:
  id: "missing-fields"
  name: "Missing Fields"
`
	if err := os.WriteFile(filepath.Join(overrideDir, "invalid.yml"), []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	_, err := LoadConfigSources(tmpDir, nil)
	if err == nil {
		t.Fatal("Expected error for invalid spec (missing required fields)")
	}
}

func TestLoadConfigSources_NoOverrideDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create .workflow/sources/

	sources, err := LoadConfigSources(tmpDir, nil)
	if err != nil {
		t.Fatalf("LoadConfigSources should succeed with no override dir: %v", err)
	}

	// Should have only embedded sources
	if len(sources) != embeddedSourceCount {
		t.Errorf("Expected %d embedded sources, got %d", embeddedSourceCount, len(sources))
	}
}

func TestLoadConfigSources_SkipsNonYAML(t *testing.T) {
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	// Write a .txt file that should be ignored
	if err := os.WriteFile(filepath.Join(overrideDir, "notes.txt"), []byte("not a config"), 0644); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	// Write a valid yaml
	sourceYAML := `source:
  id: real-source
  name: "Real Source"
  url_patterns:
    - "real\\.com/"
  url_parser:
    regex: "real\\.com/([^/]+)"
    captures:
      key: 1
  transport:
    type: mcp
    command: "test-cmd"
  auth:
    env_var: "REAL_TOKEN"
  fetch_steps:
    - tool: "get"
  markdown:
    mode: walker
`
	if err := os.WriteFile(filepath.Join(overrideDir, "real.yml"), []byte(sourceYAML), 0644); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	sources, err := LoadConfigSources(tmpDir, nil)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	// Embedded + 1 override (txt ignored)
	if len(sources) != embeddedSourceCount+1 {
		t.Fatalf("Expected %d sources (txt should be skipped), got %d", embeddedSourceCount+1, len(sources))
	}
}

func TestLoadConfigSources_PlaceholderSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	// Write a placeholder source (id starts with _)
	placeholderYAML := `source:
  id: "_test-placeholder"
  name: "Placeholder"
  url_patterns:
    - "^$"
  url_parser:
    regex: "^$"
    captures:
      _: 0
  transport:
    type: mcp
    command: "echo"
  auth:
    env_var: "_TOKEN"
  fetch_steps:
    - tool: "noop"
  markdown:
    mode: walker
`
	if err := os.WriteFile(filepath.Join(overrideDir, "placeholder.yml"), []byte(placeholderYAML), 0644); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	sources, err := LoadConfigSources(tmpDir, nil)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	// Only embedded (placeholder override was skipped)
	if len(sources) != embeddedSourceCount {
		t.Errorf("Expected %d sources (placeholders should be skipped), got %d", embeddedSourceCount, len(sources))
	}
}

func TestLoadConfigSources_YAMLExtension(t *testing.T) {
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	// Write a .yaml file (not .yml)
	sourceYAML := `source:
  id: yaml-ext
  name: "YAML Extension"
  url_patterns:
    - "yamlext\\.com/"
  url_parser:
    regex: "yamlext\\.com/([^/]+)"
    captures:
      key: 1
  transport:
    type: mcp
    command: "test-cmd"
  auth:
    env_var: "YAML_TOKEN"
  fetch_steps:
    - tool: "get"
  markdown:
    mode: walker
`
	if err := os.WriteFile(filepath.Join(overrideDir, "source.yaml"), []byte(sourceYAML), 0644); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	sources, err := LoadConfigSources(tmpDir, nil)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	// Embedded + 1 override
	if len(sources) != embeddedSourceCount+1 {
		t.Fatalf("Expected %d sources (.yaml extension should be recognized), got %d", embeddedSourceCount+1, len(sources))
	}
}

// --- Embedded YAML configs validation ---

func TestLoadConfigSources_EmbeddedFigmaConfig(t *testing.T) {
	sources, err := LoadConfigSources("", nil)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	var figma *ConfigSource
	for _, s := range sources {
		if s.Name() == "figma" {
			figma = s
			break
		}
	}
	if figma == nil {
		t.Fatal("Expected to find figma source")
	}

	// Test URL matching
	if !figma.CanHandle("https://www.figma.com/design/ABC123/My-Design") {
		t.Error("Figma should handle figma.com/design/ URLs")
	}
	if !figma.CanHandle("https://www.figma.com/file/XYZ789/Another") {
		t.Error("Figma should handle figma.com/file/ URLs")
	}
	if figma.CanHandle("https://figma.com/community/file/123") {
		t.Error("Figma should NOT handle community URLs")
	}

	// Test URL parsing
	vars := figma.parseURL("https://www.figma.com/design/ABC123xyz/My-Design-System")
	if vars == nil {
		t.Fatal("Expected vars from URL parsing")
	}
	if vars["file_key"] != "ABC123xyz" {
		t.Errorf("file_key = %q, want %q", vars["file_key"], "ABC123xyz")
	}

	// Test setup guide
	guide := figma.SetupGuide()
	if !contains(guide, "FIGMA_ACCESS_TOKEN") {
		t.Error("Setup guide should mention FIGMA_ACCESS_TOKEN")
	}
}

func TestLoadConfigSources_EmbeddedMiroConfig(t *testing.T) {
	sources, err := LoadConfigSources("", nil)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	var miro *ConfigSource
	for _, s := range sources {
		if s.Name() == "miro" {
			miro = s
			break
		}
	}
	if miro == nil {
		t.Fatal("Expected to find miro source")
	}

	// Test URL matching
	if !miro.CanHandle("https://miro.com/app/board/uXjVN123abc=/") {
		t.Error("Miro should handle miro.com/app/board/ URLs")
	}
	if miro.CanHandle("https://miro.com/marketplace/") {
		t.Error("Miro should NOT handle marketplace URLs")
	}

	// Test URL parsing
	vars := miro.parseURL("https://miro.com/app/board/uXjVN123abc=/")
	if vars == nil {
		t.Fatal("Expected vars from URL parsing")
	}
	if vars["board_id"] != "uXjVN123abc=" {
		t.Errorf("board_id = %q, want %q", vars["board_id"], "uXjVN123abc=")
	}

	// Test setup guide
	guide := miro.SetupGuide()
	if !contains(guide, "MIRO_API_TOKEN") {
		t.Error("Setup guide should mention MIRO_API_TOKEN")
	}
}

// --- parseSourceConfig ---

func TestParseSourceConfig_Valid(t *testing.T) {
	data := []byte(`source:
  id: valid
  name: "Valid"
  url_patterns:
    - "valid\\.com/"
  url_parser:
    regex: "valid\\.com/([^/]+)"
    captures:
      key: 1
  transport:
    type: mcp
    command: "test-cmd"
  auth:
    env_var: "VALID_TOKEN"
  fetch_steps:
    - tool: "get"
  markdown:
    mode: walker
`)
	spec, err := parseSourceConfig(data, "test.yml")
	if err != nil {
		t.Fatalf("parseSourceConfig failed: %v", err)
	}
	if spec == nil {
		t.Fatal("Expected non-nil spec")
	}
	if spec.ID != "valid" {
		t.Errorf("ID = %q, want %q", spec.ID, "valid")
	}
}

func TestParseSourceConfig_Placeholder(t *testing.T) {
	data := []byte(`source:
  id: "_placeholder"
  name: "Skip me"
  url_patterns: ["^$"]
  url_parser:
    regex: "^$"
    captures:
      _: 0
  transport:
    type: mcp
    command: echo
  auth:
    env_var: "_TOKEN"
  fetch_steps:
    - tool: "noop"
  markdown:
    mode: walker
`)
	spec, err := parseSourceConfig(data, "placeholder.yml")
	if err != nil {
		t.Fatalf("parseSourceConfig should not error for placeholder: %v", err)
	}
	if spec != nil {
		t.Errorf("Expected nil spec for placeholder, got %+v", spec)
	}
}

// --- isYAMLFile ---

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"figma.yml", true},
		{"miro.yaml", true},
		{"FIGMA.YML", true},
		{"source.YAML", true},
		{"notes.txt", false},
		{"config.json", false},
		{"readme.md", false},
		{".gitkeep", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isYAMLFile(tt.name); got != tt.want {
				t.Errorf("isYAMLFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
