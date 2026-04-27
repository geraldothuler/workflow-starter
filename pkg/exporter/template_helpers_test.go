package exporter

import (
	"testing"
)

func TestBase64(t *testing.T) {
	tests := []struct {
		name   string
		parts  []string
		want   string
	}{
		{"user:pass", []string{"user", "pass"}, "dXNlcjpwYXNz"},
		{"empty:token", []string{"", "mytoken"}, "Om15dG9rZW4="},
		{"single", []string{"onlyone"}, "b25seW9uZQ=="},
		{"email:token", []string{"user@example.com", "tok123"}, "dXNlckBleGFtcGxlLmNvbTp0b2sxMjM="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tplBase64(tt.parts...)
			if got != tt.want {
				t.Errorf("tplBase64(%v) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestJSONEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello", "hello"},
		{"quotes", `say "hello"`, `say \"hello\"`},
		{"newline", "line1\nline2", `line1\nline2`},
		{"tab", "col1\tcol2", `col1\tcol2`},
		{"backslash", `path\to\file`, `path\\to\\file`},
		{"empty", "", ""},
		{"unicode", "hello 🚀", "hello 🚀"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tplJSONEscape(tt.input)
			if got != tt.want {
				t.Errorf("tplJSONEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestJSONArray(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  string
	}{
		{"nil", nil, "[]"},
		{"empty", []string{}, "[]"},
		{"single", []string{"tag1"}, `["tag1"]`},
		{"multiple", []string{"go", "rust", "python"}, `["go","rust","python"]`},
		{"special", []string{`say "hi"`, "a&b"}, `["say \"hi\"","a\u0026b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tplJSONArray(tt.items)
			if got != tt.want {
				t.Errorf("tplJSONArray(%v) = %q, want %q", tt.items, got, tt.want)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		sep   string
		want  string
	}{
		{"comma", []string{"a", "b", "c"}, ", ", "a, b, c"},
		{"semicolon", []string{"tag1", "tag2"}, "; ", "tag1; tag2"},
		{"empty", []string{}, ", ", ""},
		{"single", []string{"only"}, ", ", "only"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tplJoin(tt.items, tt.sep)
			if got != tt.want {
				t.Errorf("tplJoin(%v, %q) = %q, want %q", tt.items, tt.sep, got, tt.want)
			}
		})
	}
}

func TestADFText(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "Hello world"},
		{"multi_paragraph", "Paragraph 1\n\nParagraph 2"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tplADFText(tt.input)
			if got == "" {
				t.Error("tplADFText returned empty string")
			}
			// Should always contain doc type
			if !contains(got, `"type":"doc"`) {
				t.Errorf("tplADFText(%q) missing doc type: %s", tt.input, got)
			}
			if !contains(got, `"version":1`) {
				t.Errorf("tplADFText(%q) missing version: %s", tt.input, got)
			}
		})
	}
}

func TestDefault(t *testing.T) {
	tests := []struct {
		value, fallback, want string
	}{
		{"val", "fb", "val"},
		{"", "fb", "fb"},
	}

	for _, tt := range tests {
		got := tplDefault(tt.value, tt.fallback)
		if got != tt.want {
			t.Errorf("tplDefault(%q, %q) = %q, want %q", tt.value, tt.fallback, got, tt.want)
		}
	}
}

func TestExpandTemplate(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data any
		want string
	}{
		{
			"simple",
			"Hello {{.name}}",
			map[string]any{"name": "World"},
			"Hello World",
		},
		{
			"base64_func",
			"{{base64 .user .pass}}",
			map[string]any{"user": "admin", "pass": "secret"},
			"YWRtaW46c2VjcmV0",
		},
		{
			"json_escape",
			`"{{jsonEscape .text}}"`,
			map[string]any{"text": `say "hello"`},
			`"say \"hello\""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandTemplate(tt.name, tt.tmpl, tt.data)
			if err != nil {
				t.Fatalf("expandTemplate error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expandTemplate = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandTemplate_Error(t *testing.T) {
	// Invalid template syntax should error
	_, err := expandTemplate("bad", "{{.foo | nonexistentFunc}}", map[string]any{"foo": "bar"})
	if err == nil {
		t.Error("expected error for invalid template function")
	}
}

func TestExtractField(t *testing.T) {
	data := map[string]any{
		"key":    "PROJ-123",
		"id":     float64(12345),
		"nested": map[string]any{"deep": map[string]any{"value": "found"}},
		"data": map[string]any{
			"projectCreate": map[string]any{
				"project": map[string]any{
					"id": "uuid-123",
				},
			},
		},
	}

	tests := []struct {
		path string
		want string
	}{
		{"key", "PROJ-123"},
		{"id", "12345"},
		{"nested.deep.value", "found"},
		{"data.projectCreate.project.id", "uuid-123"},
		{"nonexistent", ""},
		{"nested.nonexistent.path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractField(data, tt.path)
			if got != tt.want {
				t.Errorf("extractField(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
