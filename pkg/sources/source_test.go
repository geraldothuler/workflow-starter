package sources

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIsURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"https notion", "https://www.notion.so/Page-abc123", true},
		{"https plain", "https://example.com/page", true},
		{"http url", "http://example.com/page", true},
		{"file path", "project-input.md", false},
		{"relative path", "./docs/spec.md", false},
		{"absolute path", "/home/user/spec.md", false},
		{"empty string", "", false},
		{"just text", "not a url", false},
		{"with spaces", "  https://example.com  ", true},
		{"ftp url", "ftp://files.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsURL(tt.input)
			if got != tt.expected {
				t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRegistry_Detect(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name       string
		url        string
		wantSource string
		wantNil    bool
	}{
		{"notion url", "https://www.notion.so/Page-abc123def456abc123def456abc123de", "notion", false},
		{"notion without www", "https://notion.so/Page-abc123def456abc123def456abc123de", "notion", false},
		{"google url", "https://www.google.com/search", "", true},
		{"empty string", "", "", true},
		{"file path", "spec.md", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := registry.Detect(tt.url)
			if tt.wantNil {
				if source != nil {
					t.Errorf("Registry.Detect(%q) = %v, want nil", tt.url, source.Name())
				}
				return
			}
			if source == nil {
				t.Fatalf("Registry.Detect(%q) = nil, want %q", tt.url, tt.wantSource)
			}
			if source.Name() != tt.wantSource {
				t.Errorf("Registry.Detect(%q).Name() = %q, want %q", tt.url, source.Name(), tt.wantSource)
			}
		})
	}
}

func TestNotionSource_CanHandle(t *testing.T) {
	ns := &NotionSource{}

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"standard notion url", "https://www.notion.so/workspace/My-Page-abc123", true},
		{"notion without www", "https://notion.so/My-Page-abc123", true},
		{"notion http", "http://notion.so/My-Page-abc123", true},
		{"google url", "https://www.google.com", false},
		{"github url", "https://github.com/org/repo", false},
		{"empty", "", false},
		{"file path", "notion.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ns.CanHandle(tt.url)
			if got != tt.expected {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}

func TestNotionSource_SetupGuide(t *testing.T) {
	ns := &NotionSource{}
	guide := ns.SetupGuide()

	// Verify it contains key setup steps
	mustContain := []string{
		"notion.so/my-integrations",
		"New integration",
		"NOTION_API_TOKEN",
		"Connections",
	}

	for _, s := range mustContain {
		if !contains(guide, s) {
			t.Errorf("SetupGuide() missing %q", s)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		containsSubstring(haystack, needle)
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- Registry with RegistryOptions ---

func TestRegistry_WithProjectDir_DetectsYAMLSources(t *testing.T) {
	// Create a temporary project with a YAML source config
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	// Write a Figma-like YAML source config
	figmaYAML := `source:
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
    args: ["-y", "@anthropic-ai/figma-mcp"]
  auth:
    env_var: "FIGMA_ACCESS_TOKEN"
    setup_guide: "Get a token from figma.com/developers/api"
  fetch_steps:
    - tool: "get_file"
      args: { file_key: "{{.file_key}}" }
  markdown:
    mode: walker
`
	if err := os.WriteFile(filepath.Join(overrideDir, "figma.yml"), []byte(figmaYAML), 0644); err != nil {
		t.Fatalf("Failed to write figma.yml: %v", err)
	}

	// Create registry with project dir
	registry := NewRegistry(WithProjectDir(tmpDir))

	// Should detect Figma URLs
	figmaSource := registry.Detect("https://www.figma.com/design/ABC123/My-Design")
	if figmaSource == nil {
		t.Fatal("Expected to detect Figma source")
	}
	if figmaSource.Name() != "figma" {
		t.Errorf("Source name = %q, want %q", figmaSource.Name(), "figma")
	}

	// Should still detect Notion URLs
	notionSource := registry.Detect("https://notion.so/Page-abc123def456abc123def456abc123de")
	if notionSource == nil {
		t.Fatal("Expected to still detect Notion source")
	}
	if notionSource.Name() != "notion" {
		t.Errorf("Source name = %q, want %q", notionSource.Name(), "notion")
	}

	// Unknown URLs should return nil
	unknown := registry.Detect("https://google.com/search")
	if unknown != nil {
		t.Errorf("Expected nil for unknown URL, got %q", unknown.Name())
	}
}

func TestRegistry_WithProjectDir_MiroDetection(t *testing.T) {
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	miroYAML := `source:
  id: miro
  name: "Miro"
  url_patterns:
    - "miro\\.com/app/board/"
  url_parser:
    regex: "miro\\.com/app/board/([^/]+)"
    captures:
      board_id: 1
  transport:
    type: mcp
    command: "npx"
    args: ["-y", "@anthropic-ai/miro-mcp"]
  auth:
    env_var: "MIRO_API_TOKEN"
    setup_guide: "Get a token from miro.com/developers"
  fetch_steps:
    - tool: "miro_get_board"
      args: { board_id: "{{.board_id}}" }
  markdown:
    mode: walker
`
	if err := os.WriteFile(filepath.Join(overrideDir, "miro.yml"), []byte(miroYAML), 0644); err != nil {
		t.Fatalf("Failed to write miro.yml: %v", err)
	}

	registry := NewRegistry(WithProjectDir(tmpDir))

	miroSource := registry.Detect("https://miro.com/app/board/uXjVN123abc=/")
	if miroSource == nil {
		t.Fatal("Expected to detect Miro source")
	}
	if miroSource.Name() != "miro" {
		t.Errorf("Source name = %q, want %q", miroSource.Name(), "miro")
	}
}

func TestRegistry_Sources_ListsAll(t *testing.T) {
	registry := NewRegistry()
	allSources := registry.Sources()

	// At minimum, should have Notion (built-in)
	if len(allSources) < 1 {
		t.Fatalf("Expected at least 1 source (Notion), got %d", len(allSources))
	}

	hasNotion := false
	for _, s := range allSources {
		if s.Name() == "notion" {
			hasNotion = true
		}
	}
	if !hasNotion {
		t.Error("Expected Notion source in registry")
	}
}

func TestRegistry_WithProjectDir_FetchPipeline(t *testing.T) {
	// Full pipeline test: detect + fetch via YAML source with mock MCP
	tmpDir := t.TempDir()
	overrideDir := filepath.Join(tmpDir, ".workflow", "sources")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("Failed to create override dir: %v", err)
	}

	customYAML := `source:
  id: custom-tool
  name: "Custom Tool"
  url_patterns:
    - "customtool\\.io/project/"
  url_parser:
    regex: "customtool\\.io/project/([^/]+)"
    captures:
      project_id: 1
  transport:
    type: mcp
    command: "test-cmd"
  auth:
    env_var: "CUSTOM_TOOL_TOKEN"
    setup_guide: "Set CUSTOM_TOOL_TOKEN"
  fetch_steps:
    - tool: "get_project"
      args: { id: "{{.project_id}}" }
      extract:
        title: "name"
      store_as: "project_data"
  markdown:
    mode: walker
    walker:
      source_key: "project_data"
      heading_keys: ["name"]
      skip_keys: ["id"]
`
	if err := os.WriteFile(filepath.Join(overrideDir, "custom.yml"), []byte(customYAML), 0644); err != nil {
		t.Fatalf("Failed to write custom.yml: %v", err)
	}

	// Create mock factory
	projectData := map[string]any{
		"name":   "My Custom Project",
		"id":     "123",
		"status": "active",
	}
	projectJSON, _ := json.Marshal(projectData)
	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_project": projectJSON,
	})
	mockFactory := &MockMCPFactory{Session: mockSession}

	// Create source directly via LoadConfigSources with mock
	configSources, err := LoadConfigSources(tmpDir, mockFactory)
	if err != nil {
		t.Fatalf("LoadConfigSources failed: %v", err)
	}

	// Find our custom source among all loaded sources (embedded + overrides)
	var cs *ConfigSource
	for _, s := range configSources {
		if s.Name() == "custom-tool" {
			cs = s
			break
		}
	}
	if cs == nil {
		t.Fatal("Expected to find 'custom-tool' config source")
	}
	if !cs.CanHandle("https://customtool.io/project/proj-456") {
		t.Fatal("Expected source to handle URL")
	}

	// Set auth
	os.Setenv("CUSTOM_TOOL_TOKEN", "test-token")
	defer os.Unsetenv("CUSTOM_TOOL_TOKEN")

	result, err := cs.Fetch("https://customtool.io/project/proj-456")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if result.Title != "My Custom Project" {
		t.Errorf("Title = %q, want %q", result.Title, "My Custom Project")
	}
	if result.Source != "custom-tool" {
		t.Errorf("Source = %q, want %q", result.Source, "custom-tool")
	}
	if !contains(result.Content, "# My Custom Project") {
		t.Errorf("Content should contain H1 title")
	}
}
