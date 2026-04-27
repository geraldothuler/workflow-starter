package sources

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// --- Helper: build a minimal valid SourceSpec for testing ---

func testSourceSpec() SourceSpec {
	return SourceSpec{
		ID:          "test-source",
		Name:        "Test Source",
		URLPatterns: []string{`testsource\.com/item/`},
		URLParser: URLParserSpec{
			Regex:    `testsource\.com/item/([^/]+)`,
			Captures: map[string]int{"item_id": 1},
		},
		Transport: TransportSpec{
			Type:    "mcp",
			Command: "test-cmd",
			Timeout: 30_000_000_000, // 30s as nanoseconds
		},
		Auth: AuthSpec{
			EnvVar:     "TEST_SOURCE_TOKEN",
			SetupGuide: "Set TEST_SOURCE_TOKEN to your token.",
		},
		FetchSteps: []FetchStep{
			{
				Tool:    "get_item",
				Args:    map[string]any{"id": "{{.item_id}}"},
				Extract: map[string]string{"title": "name"},
				StoreAs: "item_data",
			},
		},
		Markdown: MarkdownSpec{
			Mode: "walker",
			Walker: &WalkerSpec{
				SourceKey:   "item_data",
				MaxDepth:    4,
				HeadingKeys: []string{"name"},
				SkipKeys:    []string{"id"},
			},
		},
	}
}

// --- NewConfigSource ---

func TestNewConfigSource_ValidSpec(t *testing.T) {
	spec := testSourceSpec()
	mock := &MockMCPFactory{Session: NewMockMCPSession(nil)}

	cs, err := NewConfigSource(spec, mock)
	if err != nil {
		t.Fatalf("NewConfigSource failed: %v", err)
	}
	if cs == nil {
		t.Fatal("Expected non-nil ConfigSource")
	}
}

func TestNewConfigSource_InvalidURLPattern(t *testing.T) {
	spec := testSourceSpec()
	spec.URLPatterns = []string{"[invalid"}

	_, err := NewConfigSource(spec, nil)
	if err == nil {
		t.Fatal("Expected error for invalid URL pattern")
	}
	if got := err.Error(); !contains(got, "invalid url_pattern") {
		t.Errorf("Expected 'invalid url_pattern' in error, got: %s", got)
	}
}

func TestNewConfigSource_InvalidParserRegex(t *testing.T) {
	spec := testSourceSpec()
	spec.URLParser.Regex = "[invalid"

	_, err := NewConfigSource(spec, nil)
	if err == nil {
		t.Fatal("Expected error for invalid parser regex")
	}
	if got := err.Error(); !contains(got, "invalid url_parser.regex") {
		t.Errorf("Expected 'invalid url_parser.regex' in error, got: %s", got)
	}
}

func TestNewConfigSource_NilFactory_UsesDefault(t *testing.T) {
	spec := testSourceSpec()
	cs, err := NewConfigSource(spec, nil)
	if err != nil {
		t.Fatalf("NewConfigSource failed: %v", err)
	}
	if cs.mcpFactory == nil {
		t.Fatal("Expected default factory when nil provided")
	}
}

// --- Name ---

func TestConfigSource_Name(t *testing.T) {
	spec := testSourceSpec()
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	if got := cs.Name(); got != "test-source" {
		t.Errorf("Name() = %q, want %q", got, "test-source")
	}
}

// --- CanHandle ---

func TestConfigSource_CanHandle(t *testing.T) {
	spec := testSourceSpec()
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	tests := []struct {
		url  string
		want bool
	}{
		{"https://testsource.com/item/abc123", true},
		{"https://TESTSOURCE.COM/item/xyz", true}, // case-insensitive
		{"https://other.com/item/abc123", false},
		{"https://testsource.com/other/abc123", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := cs.CanHandle(tt.url); got != tt.want {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestConfigSource_CanHandle_MultiplePatterns(t *testing.T) {
	spec := testSourceSpec()
	spec.URLPatterns = []string{
		`testsource\.com/item/`,
		`testsource\.com/design/`,
	}
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	if !cs.CanHandle("https://testsource.com/design/abc") {
		t.Error("Expected CanHandle to match second pattern")
	}
}

// --- SetupGuide ---

func TestConfigSource_SetupGuide(t *testing.T) {
	spec := testSourceSpec()
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	if got := cs.SetupGuide(); got != "Set TEST_SOURCE_TOKEN to your token." {
		t.Errorf("SetupGuide() = %q, want setup guide text", got)
	}
}

// --- parseURL ---

func TestConfigSource_ParseURL(t *testing.T) {
	spec := testSourceSpec()
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	vars := cs.parseURL("https://testsource.com/item/abc123")
	if vars == nil {
		t.Fatal("Expected non-nil vars")
	}
	if got := vars["item_id"]; got != "abc123" {
		t.Errorf("vars[item_id] = %q, want %q", got, "abc123")
	}
}

func TestConfigSource_ParseURL_NoMatch(t *testing.T) {
	spec := testSourceSpec()
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	vars := cs.parseURL("https://other.com/page/xyz")
	if vars != nil {
		t.Errorf("Expected nil vars for non-matching URL, got %v", vars)
	}
}

func TestConfigSource_ParseURL_MultipleCaptures(t *testing.T) {
	spec := testSourceSpec()
	spec.URLParser.Regex = `testsource\.com/item/([^/]+)/version/([^/]+)`
	spec.URLParser.Captures = map[string]int{
		"item_id":    1,
		"version_id": 2,
	}
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	vars := cs.parseURL("https://testsource.com/item/abc/version/v2")
	if vars == nil {
		t.Fatal("Expected non-nil vars")
	}
	if got := vars["item_id"]; got != "abc" {
		t.Errorf("item_id = %q, want %q", got, "abc")
	}
	if got := vars["version_id"]; got != "v2" {
		t.Errorf("version_id = %q, want %q", got, "v2")
	}
}

// --- Fetch: auth check ---

func TestConfigSource_Fetch_MissingAuth(t *testing.T) {
	spec := testSourceSpec()
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	// Ensure env var is NOT set
	os.Unsetenv("TEST_SOURCE_TOKEN")

	_, err := cs.Fetch("https://testsource.com/item/abc123")
	if err == nil {
		t.Fatal("Expected error for missing auth token")
	}
	if !contains(err.Error(), "token") {
		t.Errorf("Expected error to mention 'token', got: %s", err.Error())
	}
}

// --- Fetch: URL parse failure ---

func TestConfigSource_Fetch_URLParseFailure(t *testing.T) {
	spec := testSourceSpec()
	cs, _ := NewConfigSource(spec, &MockMCPFactory{Session: NewMockMCPSession(nil)})

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	_, err := cs.Fetch("https://other.com/invalid-url")
	if err == nil {
		t.Fatal("Expected error for URL parse failure")
	}
	if !contains(err.Error(), "extrair parâmetros") {
		t.Errorf("Expected URL parse error, got: %s", err.Error())
	}
}

// --- Fetch: MCP connect failure ---

func TestConfigSource_Fetch_MCPConnectFailure(t *testing.T) {
	spec := testSourceSpec()
	factory := &MockMCPFactory{Err: fmt.Errorf("connection refused")}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	_, err := cs.Fetch("https://testsource.com/item/abc123")
	if err == nil {
		t.Fatal("Expected error for MCP connect failure")
	}
	if !contains(err.Error(), "conectar MCP") {
		t.Errorf("Expected MCP connection error, got: %s", err.Error())
	}
}

// --- Fetch: tool call failure ---

func TestConfigSource_Fetch_ToolCallFailure(t *testing.T) {
	spec := testSourceSpec()
	mockSession := NewMockMCPSession(nil)
	mockSession.Errors["get_item"] = fmt.Errorf("tool not found")
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	_, err := cs.Fetch("https://testsource.com/item/abc123")
	if err == nil {
		t.Fatal("Expected error for tool call failure")
	}
	if !contains(err.Error(), "fetch_step[0]") {
		t.Errorf("Expected step error, got: %s", err.Error())
	}
}

// --- Fetch: complete pipeline (walker mode) ---

func TestConfigSource_Fetch_WalkerMode(t *testing.T) {
	spec := testSourceSpec()

	// Canned MCP response for get_item
	itemData := map[string]any{
		"name":        "Design System",
		"type":        "CANVAS",
		"id":          "0:1",
		"description": "Main design system file",
	}
	itemJSON, _ := json.Marshal(itemData)

	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": itemJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	result, err := cs.Fetch("https://testsource.com/item/abc123")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Verify title extracted from "name" field
	if result.Title != "Design System" {
		t.Errorf("Title = %q, want %q", result.Title, "Design System")
	}

	// Verify H1 title prepended
	if !contains(result.Content, "# Design System") {
		t.Errorf("Content should start with H1 title, got:\n%s", result.Content)
	}

	// Verify walker rendered key-value for type
	if !contains(result.Content, "**type:** CANVAS") {
		t.Errorf("Content should contain type value, got:\n%s", result.Content)
	}

	// Verify "id" was skipped by walker
	if contains(result.Content, "**id:**") {
		t.Errorf("Content should NOT contain skipped key 'id', got:\n%s", result.Content)
	}

	// Verify metadata
	if result.Source != "test-source" {
		t.Errorf("Source = %q, want %q", result.Source, "test-source")
	}
	if result.URL != "https://testsource.com/item/abc123" {
		t.Errorf("URL = %q, want original URL", result.URL)
	}

	// Verify session was closed
	if !mockSession.IsClosed() {
		t.Error("Expected MCP session to be closed after Fetch")
	}
}

// --- Fetch: template mode ---

func TestConfigSource_Fetch_TemplateMode(t *testing.T) {
	spec := testSourceSpec()
	spec.Markdown.Mode = "template"
	spec.Markdown.Template = `## {{.title}}

Content from {{.item_id}}.

**Description:** {{ index ._result "description" }}
`
	spec.Markdown.Walker = nil
	spec.FetchSteps[0].StoreAs = "" // don't need store_as for template mode

	itemData := map[string]any{
		"name":        "My Item",
		"description": "A great item",
	}
	itemJSON, _ := json.Marshal(itemData)

	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": itemJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	result, err := cs.Fetch("https://testsource.com/item/abc123")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Template should render with extracted title
	if !contains(result.Content, "## My Item") {
		t.Errorf("Content should contain template heading, got:\n%s", result.Content)
	}
	// Template should have access to URL vars
	if !contains(result.Content, "Content from abc123") {
		t.Errorf("Content should contain item_id, got:\n%s", result.Content)
	}
	// Template should have access to _result
	if !contains(result.Content, "A great item") {
		t.Errorf("Content should contain description from _result, got:\n%s", result.Content)
	}
}

// --- Fetch: multi-step pipeline ---

func TestConfigSource_Fetch_MultiStep(t *testing.T) {
	spec := testSourceSpec()
	spec.FetchSteps = []FetchStep{
		{
			Tool:    "get_metadata",
			Args:    map[string]any{"id": "{{.item_id}}"},
			Extract: map[string]string{"title": "name", "version": "version"},
		},
		{
			Tool:    "get_content",
			Args:    map[string]any{"id": "{{.item_id}}", "version": "{{.version}}"},
			Extract: map[string]string{"content": "."},
			StoreAs: "item_data",
		},
	}

	metadataResp := map[string]any{
		"name":    "Multi-Step Item",
		"version": "v3",
	}
	contentResp := map[string]any{
		"name":  "Content Node",
		"type":  "DOCUMENT",
		"pages": []any{"Page 1", "Page 2"},
	}

	metaJSON, _ := json.Marshal(metadataResp)
	contentJSON, _ := json.Marshal(contentResp)

	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_metadata": metaJSON,
		"get_content":  contentJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	result, err := cs.Fetch("https://testsource.com/item/xyz789")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Title should come from first step extraction
	if result.Title != "Multi-Step Item" {
		t.Errorf("Title = %q, want %q", result.Title, "Multi-Step Item")
	}

	// Verify both tools were called
	if len(mockSession.Calls) != 2 {
		t.Fatalf("Expected 2 tool calls, got %d", len(mockSession.Calls))
	}

	// First call: get_metadata
	if mockSession.Calls[0].Name != "get_metadata" {
		t.Errorf("First call = %q, want get_metadata", mockSession.Calls[0].Name)
	}
	if mockSession.Calls[0].Args["id"] != "xyz789" {
		t.Errorf("First call id = %v, want xyz789", mockSession.Calls[0].Args["id"])
	}

	// Second call: get_content with version from first step
	if mockSession.Calls[1].Name != "get_content" {
		t.Errorf("Second call = %q, want get_content", mockSession.Calls[1].Name)
	}
	if mockSession.Calls[1].Args["version"] != "v3" {
		t.Errorf("Second call version = %v, want v3", mockSession.Calls[1].Args["version"])
	}
}

// --- Fetch: JSON parse failure ---

func TestConfigSource_Fetch_InvalidJSONResponse(t *testing.T) {
	spec := testSourceSpec()

	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": json.RawMessage(`{not valid json`),
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	_, err := cs.Fetch("https://testsource.com/item/abc123")
	if err == nil {
		t.Fatal("Expected error for invalid JSON response")
	}
	if !contains(err.Error(), "JSON parse failed") {
		t.Errorf("Expected JSON parse error, got: %s", err.Error())
	}
}

// --- Fetch: unknown markdown mode ---

func TestConfigSource_Fetch_UnknownMarkdownMode(t *testing.T) {
	spec := testSourceSpec()
	spec.Markdown.Mode = "unknown"

	itemJSON, _ := json.Marshal(map[string]any{"name": "Test"})
	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": itemJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	_, err := cs.Fetch("https://testsource.com/item/abc123")
	if err == nil {
		t.Fatal("Expected error for unknown markdown mode")
	}
	if !contains(err.Error(), "unknown markdown mode") {
		t.Errorf("Expected unknown mode error, got: %s", err.Error())
	}
}

// --- Fetch: walker source_key not found ---

func TestConfigSource_Fetch_WalkerMissingSourceKey(t *testing.T) {
	spec := testSourceSpec()
	spec.Markdown.Walker.SourceKey = "nonexistent"
	// Don't store any result with that key
	spec.FetchSteps[0].StoreAs = "other_key"

	itemJSON, _ := json.Marshal(map[string]any{"name": "Test"})
	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": itemJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	_, err := cs.Fetch("https://testsource.com/item/abc123")
	if err == nil {
		t.Fatal("Expected error for missing source_key")
	}
	if !contains(err.Error(), "source_key") {
		t.Errorf("Expected source_key error, got: %s", err.Error())
	}
}

// --- Fetch: walker uses lastResult when no source_key ---

func TestConfigSource_Fetch_WalkerLastResult(t *testing.T) {
	spec := testSourceSpec()
	spec.Markdown.Walker.SourceKey = "" // use last result

	itemData := map[string]any{
		"name":   "Direct Result",
		"status": "active",
	}
	itemJSON, _ := json.Marshal(itemData)

	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": itemJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	result, err := cs.Fetch("https://testsource.com/item/abc123")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should use lastResult directly
	if !contains(result.Content, "Direct Result") {
		t.Errorf("Content should contain walker output from lastResult, got:\n%s", result.Content)
	}
}

// --- Fetch: title not extracted ---

func TestConfigSource_Fetch_NoTitle(t *testing.T) {
	spec := testSourceSpec()
	// Change extraction to not extract title
	spec.FetchSteps[0].Extract = map[string]string{"status": "status"}

	itemData := map[string]any{
		"status": "active",
		"data":   "some data",
	}
	itemJSON, _ := json.Marshal(itemData)

	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": itemJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	result, err := cs.Fetch("https://testsource.com/item/abc123")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// No title, so no H1 header
	if result.Title != "" {
		t.Errorf("Title = %q, want empty", result.Title)
	}
	if contains(result.Content, "# ") && !contains(result.Content, "## ") {
		t.Errorf("Content should not have H1 when no title, got:\n%s", result.Content)
	}
}

// --- Fetch: extract with dot-path from step ---

func TestConfigSource_Fetch_ExtractDotPath(t *testing.T) {
	spec := testSourceSpec()
	spec.FetchSteps[0].Extract = map[string]string{
		"title":  "metadata.title",
		"author": "metadata.author",
	}

	itemData := map[string]any{
		"metadata": map[string]any{
			"title":  "Nested Title",
			"author": "Test Author",
		},
		"content": "hello",
	}
	itemJSON, _ := json.Marshal(itemData)

	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": itemJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	result, err := cs.Fetch("https://testsource.com/item/abc123")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if result.Title != "Nested Title" {
		t.Errorf("Title = %q, want %q (from nested dot-path)", result.Title, "Nested Title")
	}
}

// --- countStructure ---

func TestCountStructure(t *testing.T) {
	tests := []struct {
		name string
		data any
		want int
	}{
		{
			name: "nil",
			data: nil,
			want: 0,
		},
		{
			name: "string",
			data: "hello",
			want: 0,
		},
		{
			name: "simple object",
			data: map[string]any{"name": "test", "type": "obj"},
			want: 1, // 1 map
		},
		{
			name: "nested object",
			data: map[string]any{
				"name": "test",
				"child": map[string]any{
					"type": "inner",
				},
			},
			want: 2, // outer + inner
		},
		{
			name: "array of objects",
			data: []any{
				map[string]any{"name": "a"},
				map[string]any{"name": "b"},
			},
			want: 2, // 2 maps
		},
		{
			name: "empty map",
			data: map[string]any{},
			want: 1,
		},
		{
			name: "empty array",
			data: []any{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countStructure(tt.data); got != tt.want {
				t.Errorf("countStructure() = %d, want %d", got, tt.want)
			}
		})
	}
}

// --- Fetch: metadata source_type ---

func TestConfigSource_Fetch_Metadata(t *testing.T) {
	spec := testSourceSpec()

	itemJSON, _ := json.Marshal(map[string]any{"name": "Test Item"})
	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_item": itemJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("TEST_SOURCE_TOKEN", "test-token")
	defer os.Unsetenv("TEST_SOURCE_TOKEN")

	result, err := cs.Fetch("https://testsource.com/item/abc123")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if result.Metadata["source_type"] != "mcp" {
		t.Errorf("Metadata source_type = %q, want %q", result.Metadata["source_type"], "mcp")
	}
	if result.BlockCount < 0 {
		t.Errorf("BlockCount should be >= 0, got %d", result.BlockCount)
	}
}

// --- Figma-like end-to-end test ---

func TestConfigSource_FigmaLikeE2E(t *testing.T) {
	spec := SourceSpec{
		ID:          "figma",
		Name:        "Figma",
		URLPatterns: []string{`figma\.com/(file|design)/`},
		URLParser: URLParserSpec{
			Regex:    `figma\.com/(?:file|design)/([^/]+)`,
			Captures: map[string]int{"file_key": 1},
		},
		Transport: TransportSpec{
			Type:    "mcp",
			Command: "npx",
			Args:    []string{"-y", "@anthropic-ai/figma-mcp"},
			Timeout: 30_000_000_000,
		},
		Auth: AuthSpec{
			EnvVar:     "FIGMA_ACCESS_TOKEN",
			SetupGuide: "Get a Personal Access Token from figma.com/developers/api",
		},
		FetchSteps: []FetchStep{
			{
				Tool:    "get_metadata",
				Args:    map[string]any{"file_key": "{{.file_key}}"},
				Extract: map[string]string{"title": "name", "last_modified": "lastModified"},
			},
			{
				Tool:    "get_file",
				Args:    map[string]any{"file_key": "{{.file_key}}"},
				StoreAs: "file_data",
			},
		},
		Markdown: MarkdownSpec{
			Mode: "walker",
			Walker: &WalkerSpec{
				SourceKey:   "file_data",
				MaxDepth:    6,
				HeadingKeys: []string{"name", "label"},
				SkipKeys:    []string{"id", "pluginData", "exportSettings"},
				CodeKeys:    []string{"characters", "css"},
				ValueKeys:   []string{"type", "visible"},
			},
		},
	}

	// Canned Figma-like responses
	metadataResp := map[string]any{
		"name":         "My Design System",
		"lastModified": "2026-02-20T10:00:00Z",
		"version":      "1234567",
	}
	fileResp := map[string]any{
		"name": "My Design System",
		"document": map[string]any{
			"name": "Document",
			"type": "DOCUMENT",
			"children": []any{
				map[string]any{
					"name": "Page 1",
					"type": "CANVAS",
					"children": []any{
						map[string]any{
							"name":       "Button",
							"type":       "COMPONENT",
							"visible":    true,
							"characters": "Click me",
						},
					},
				},
			},
		},
	}

	metaJSON, _ := json.Marshal(metadataResp)
	fileJSON, _ := json.Marshal(fileResp)

	mockSession := NewMockMCPSession(map[string]json.RawMessage{
		"get_metadata": metaJSON,
		"get_file":     fileJSON,
	})
	factory := &MockMCPFactory{Session: mockSession}
	cs, _ := NewConfigSource(spec, factory)

	os.Setenv("FIGMA_ACCESS_TOKEN", "fig_test_token")
	defer os.Unsetenv("FIGMA_ACCESS_TOKEN")

	result, err := cs.Fetch("https://www.figma.com/design/ABC123xyz/My-Design-System")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Title from first step
	if result.Title != "My Design System" {
		t.Errorf("Title = %q, want %q", result.Title, "My Design System")
	}

	// Content should have H1 title
	if !contains(result.Content, "# My Design System") {
		t.Errorf("Content should start with H1 title")
	}

	// Content should contain walker output
	if !contains(result.Content, "Page 1") {
		t.Errorf("Content should contain 'Page 1' from walker")
	}
	if !contains(result.Content, "**type:** COMPONENT") {
		t.Errorf("Content should contain value_keys like type")
	}

	// Code keys should render as code blocks
	if !contains(result.Content, "```\nClick me\n```") {
		t.Errorf("Content should contain characters as code block, got:\n%s", result.Content)
	}

	// Verify source metadata
	if result.Source != "figma" {
		t.Errorf("Source = %q, want %q", result.Source, "figma")
	}
	if result.Metadata["source_type"] != "mcp" {
		t.Errorf("source_type = %q, want %q", result.Metadata["source_type"], "mcp")
	}

	// Verify tool calls
	if len(mockSession.Calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d", len(mockSession.Calls))
	}
	if mockSession.Calls[0].Args["file_key"] != "ABC123xyz" {
		t.Errorf("First call file_key = %v, want ABC123xyz", mockSession.Calls[0].Args["file_key"])
	}
}

// Note: contains() helper is defined in source_test.go
