package sources

import (
	"strings"
	"testing"
)

func TestJSONWalker_SimpleObject(t *testing.T) {
	data := map[string]any{
		"name": "Test Component",
		"type": "FRAME",
	}

	walker := NewJSONWalker(nil) // default config
	got := walker.Walk(data)

	if !strings.Contains(got, "Test Component") {
		t.Error("missing heading from name key")
	}
	if !strings.Contains(got, "**type:** FRAME") {
		t.Error("missing type key-value pair")
	}
}

func TestJSONWalker_NestedFigmaLike(t *testing.T) {
	data := map[string]any{
		"name": "Landing Page",
		"document": map[string]any{
			"children": []any{
				map[string]any{
					"name": "Frame 1",
					"type": "FRAME",
					"children": []any{
						map[string]any{
							"name":       "Header",
							"type":       "TEXT",
							"characters": "Welcome to our app",
						},
						map[string]any{
							"name": "Button",
							"type": "COMPONENT",
						},
					},
				},
			},
		},
	}

	spec := &WalkerSpec{
		MaxDepth:    6,
		HeadingKeys: []string{"name"},
		SkipKeys:    []string{"id"},
		ValueKeys:   []string{"type"},
		CodeKeys:    []string{"characters"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(data)

	// Check headings
	if !strings.Contains(got, "Frame 1") {
		t.Error("missing Frame 1 heading")
	}
	if !strings.Contains(got, "Header") {
		t.Error("missing Header heading")
	}
	if !strings.Contains(got, "Button") {
		t.Error("missing Button heading")
	}

	// Check type values
	if !strings.Contains(got, "**type:** FRAME") {
		t.Error("missing FRAME type")
	}
	if !strings.Contains(got, "**type:** TEXT") {
		t.Error("missing TEXT type")
	}
	if !strings.Contains(got, "**type:** COMPONENT") {
		t.Error("missing COMPONENT type")
	}

	// Check code block for characters
	if !strings.Contains(got, "```\nWelcome to our app\n```") {
		t.Error("missing code block for characters")
	}
}

func TestJSONWalker_MiroLike(t *testing.T) {
	data := map[string]any{
		"name":        "Project Board",
		"description": "Sprint planning board",
		"items": []any{
			map[string]any{
				"title": "User Authentication",
				"type":  "sticky_note",
			},
			map[string]any{
				"title": "API Gateway",
				"type":  "sticky_note",
			},
		},
	}

	spec := &WalkerSpec{
		HeadingKeys: []string{"name", "title"},
		ValueKeys:   []string{"type", "description"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(data)

	if !strings.Contains(got, "User Authentication") {
		t.Error("missing User Authentication")
	}
	if !strings.Contains(got, "API Gateway") {
		t.Error("missing API Gateway")
	}
	if !strings.Contains(got, "**type:** sticky_note") {
		t.Error("missing sticky_note type")
	}
}

func TestJSONWalker_SkipKeys(t *testing.T) {
	data := map[string]any{
		"name":       "Component",
		"id":         "12345",
		"pluginData": map[string]any{"internal": true},
	}

	spec := &WalkerSpec{
		HeadingKeys: []string{"name"},
		SkipKeys:    []string{"id", "pluginData"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(data)

	if strings.Contains(got, "12345") {
		t.Error("should skip id key")
	}
	if strings.Contains(got, "pluginData") {
		t.Error("should skip pluginData key")
	}
	if !strings.Contains(got, "Component") {
		t.Error("heading should still appear")
	}
}

func TestJSONWalker_PrimitiveArray(t *testing.T) {
	data := map[string]any{
		"name": "Tags Container",
		"tags": []any{"frontend", "react", "typescript"},
	}

	spec := &WalkerSpec{
		HeadingKeys: []string{"name"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(data)

	if !strings.Contains(got, "- frontend") {
		t.Error("missing frontend bullet")
	}
	if !strings.Contains(got, "- react") {
		t.Error("missing react bullet")
	}
	if !strings.Contains(got, "- typescript") {
		t.Error("missing typescript bullet")
	}
}

func TestJSONWalker_MaxDepth(t *testing.T) {
	// Create deeply nested structure
	deep := map[string]any{
		"name": "Level 0",
		"child": map[string]any{
			"name": "Level 1",
			"child": map[string]any{
				"name": "Level 2",
				"child": map[string]any{
					"name": "Level 3",
				},
			},
		},
	}

	spec := &WalkerSpec{
		MaxDepth:    2, // Only go 2 levels deep
		HeadingKeys: []string{"name"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(deep)

	if !strings.Contains(got, "Level 1") {
		t.Error("Level 1 should appear (depth 1)")
	}
	if !strings.Contains(got, "Level 2") {
		t.Error("Level 2 should appear (depth 2)")
	}
	if strings.Contains(got, "Level 3") {
		t.Error("Level 3 should NOT appear (depth 3 > max 2)")
	}
}

func TestJSONWalker_EmptyData(t *testing.T) {
	walker := NewJSONWalker(nil)

	tests := []struct {
		name string
		data any
	}{
		{"nil data", nil},
		{"empty map", map[string]any{}},
		{"empty array", []any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := walker.Walk(tt.data)
			// Should not panic, may produce empty or minimal output
			_ = got
		})
	}
}

func TestJSONWalker_ListKeys(t *testing.T) {
	data := map[string]any{
		"name": "Container",
		"children": []any{
			map[string]any{
				"name": "Child A",
				"type": "TEXT",
			},
			map[string]any{
				"name": "Child B",
				"type": "FRAME",
			},
		},
	}

	spec := &WalkerSpec{
		HeadingKeys: []string{"name"},
		ListKeys:    []string{"children"},
		ValueKeys:   []string{"type"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(data)

	if !strings.Contains(got, "Child A") {
		t.Error("missing Child A from list_keys traversal")
	}
	if !strings.Contains(got, "Child B") {
		t.Error("missing Child B from list_keys traversal")
	}
}

func TestJSONWalker_CodeKeys(t *testing.T) {
	data := map[string]any{
		"name":       "Text Node",
		"characters": "Hello, World!",
		"css":        "font-size: 16px;\ncolor: red;",
	}

	spec := &WalkerSpec{
		HeadingKeys: []string{"name"},
		CodeKeys:    []string{"characters", "css"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(data)

	if !strings.Contains(got, "```\nHello, World!\n```") {
		t.Error("missing characters code block")
	}
	if !strings.Contains(got, "```\nfont-size: 16px;\ncolor: red;\n```") {
		t.Error("missing css code block")
	}
}

func TestJSONWalker_DefaultConfig(t *testing.T) {
	walker := NewJSONWalker(nil)
	cfg := walker.config

	if cfg.MaxDepth != 6 {
		t.Errorf("default MaxDepth = %d, want 6", cfg.MaxDepth)
	}
	if !cfg.HeadingKeys["name"] {
		t.Error("default HeadingKeys should include 'name'")
	}
	if !cfg.HeadingKeys["title"] {
		t.Error("default HeadingKeys should include 'title'")
	}
	if !cfg.SkipKeys["id"] {
		t.Error("default SkipKeys should include 'id'")
	}
	if !cfg.ValueKeys["type"] {
		t.Error("default ValueKeys should include 'type'")
	}
}

func TestJSONWalker_HeadingDepthLevels(t *testing.T) {
	data := map[string]any{
		"children": []any{
			map[string]any{
				"name": "Level1",
				"children": []any{
					map[string]any{
						"name": "Level2",
						"children": []any{
							map[string]any{
								"name": "Level3",
							},
						},
					},
				},
			},
		},
	}

	spec := &WalkerSpec{
		MaxDepth:    6,
		HeadingKeys: []string{"name"},
		ListKeys:    []string{"children"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(data)

	// Level1 at depth 1 → ## (H2)
	if !strings.Contains(got, "## Level1") {
		t.Error("Level1 should be H2")
	}
	// Level2 at depth 2 → ### (H3)
	if !strings.Contains(got, "### Level2") {
		t.Error("Level2 should be H3")
	}
	// Level3 at depth 3 → #### (H4)
	if !strings.Contains(got, "#### Level3") {
		t.Error("Level3 should be H4")
	}
}

func TestJSONWalker_BoolAndNumberValues(t *testing.T) {
	data := map[string]any{
		"name":    "Widget",
		"visible": true,
		"opacity": 0.5,
		"count":   3.0,
	}

	spec := &WalkerSpec{
		HeadingKeys: []string{"name"},
		ValueKeys:   []string{"visible", "opacity", "count"},
	}

	walker := NewJSONWalker(spec)
	got := walker.Walk(data)

	if !strings.Contains(got, "**visible:** true") {
		t.Error("missing bool value")
	}
	if !strings.Contains(got, "**opacity:** 0.5") {
		t.Error("missing float value")
	}
}
