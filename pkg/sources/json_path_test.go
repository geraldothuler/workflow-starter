package sources

import (
	"testing"
)

func TestExtractPath(t *testing.T) {
	data := map[string]any{
		"name": "Test File",
		"version": "1.0",
		"nested": map[string]any{
			"level1": map[string]any{
				"level2": "deep value",
				"number": 42.0,
			},
			"list": []any{"a", "b", "c"},
		},
		"empty": "",
		"null_val": nil,
	}

	tests := []struct {
		name     string
		data     any
		path     string
		expected any
	}{
		{"root path dot", data, ".", data},
		{"root path empty", data, "", data},
		{"top-level string", data, "name", "Test File"},
		{"top-level version", data, "version", "1.0"},
		{"nested one level", data, "nested.level1", map[string]any{"level2": "deep value", "number": 42.0}},
		{"nested two levels", data, "nested.level1.level2", "deep value"},
		{"nested number", data, "nested.level1.number", 42.0},
		{"nested list", data, "nested.list", []any{"a", "b", "c"}},
		{"empty string value", data, "empty", ""},
		{"nil value", data, "null_val", nil},
		{"missing key", data, "nonexistent", nil},
		{"missing nested", data, "nested.missing", nil},
		{"deep missing", data, "nested.level1.missing", nil},
		{"path through non-map", data, "name.sub", nil},
		{"nil data", nil, "name", nil},
		{"whitespace path", data, "  name  ", "Test File"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPath(tt.data, tt.path)

			if tt.expected == nil {
				if got != nil {
					t.Errorf("ExtractPath(%q) = %v, want nil", tt.path, got)
				}
				return
			}

			// For map comparison, check type match
			if expectedMap, ok := tt.expected.(map[string]any); ok {
				gotMap, ok := got.(map[string]any)
				if !ok {
					t.Fatalf("ExtractPath(%q) type = %T, want map[string]any", tt.path, got)
				}
				if len(gotMap) != len(expectedMap) {
					t.Errorf("ExtractPath(%q) map len = %d, want %d", tt.path, len(gotMap), len(expectedMap))
				}
				return
			}

			// For slice comparison, check type match
			if expectedSlice, ok := tt.expected.([]any); ok {
				gotSlice, ok := got.([]any)
				if !ok {
					t.Fatalf("ExtractPath(%q) type = %T, want []any", tt.path, got)
				}
				if len(gotSlice) != len(expectedSlice) {
					t.Errorf("ExtractPath(%q) slice len = %d, want %d", tt.path, len(gotSlice), len(expectedSlice))
				}
				return
			}

			if got != tt.expected {
				t.Errorf("ExtractPath(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	data := map[string]any{
		"name":   "Test",
		"count":  42.0,
		"active": true,
		"nested": map[string]any{"key": "value"},
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"string value", "name", "Test"},
		{"number value", "count", "42"},
		{"bool value", "active", "true"},
		{"missing key", "missing", ""},
		{"nil data", "anything", ""},
		{"nested string", "nested.key", "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d any = data
			if tt.name == "nil data" {
				d = nil
			}
			got := ExtractString(d, tt.path)
			if got != tt.expected {
				t.Errorf("ExtractString(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestExtractMap(t *testing.T) {
	data := map[string]any{
		"config": map[string]any{
			"key": "value",
		},
		"name": "not a map",
	}

	tests := []struct {
		name    string
		path    string
		wantNil bool
	}{
		{"valid map", "config", false},
		{"string not map", "name", true},
		{"missing", "missing", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMap(data, tt.path)
			if tt.wantNil && got != nil {
				t.Errorf("ExtractMap(%q) = %v, want nil", tt.path, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("ExtractMap(%q) = nil, want map", tt.path)
			}
		})
	}
}

func TestExtractSlice(t *testing.T) {
	data := map[string]any{
		"items": []any{"a", "b"},
		"name":  "not a slice",
	}

	tests := []struct {
		name    string
		path    string
		wantNil bool
	}{
		{"valid slice", "items", false},
		{"string not slice", "name", true},
		{"missing", "missing", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSlice(data, tt.path)
			if tt.wantNil && got != nil {
				t.Errorf("ExtractSlice(%q) = %v, want nil", tt.path, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("ExtractSlice(%q) = nil, want slice", tt.path)
			}
		})
	}
}
