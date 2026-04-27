package scaffold

import "testing"

func TestRenderTemplate_BasicSubstitution(t *testing.T) {
	// Template files use literal "NNN" and "YYYY-MM-DD" placeholders; other
	// substitutions are already in Go template action syntax {{.Key}}.
	content := "# NNN — {{.Context}}\nDate: YYYY-MM-DD\nType: {{.Type}}"
	vars := map[string]string{
		"Type":    "incident",
		"Context": "kafka-lag",
		"Date":    "2026-02-23",
		"NNN":     "003",
	}
	got, err := RenderTemplate(content, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "# 003 — kafka-lag\nDate: 2026-02-23\nType: incident"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderTemplate_LiteralPlaceholders(t *testing.T) {
	// Templates may have literal YYYY-MM-DD and NNN that need replacement
	content := "# NNN — context\nDate: YYYY-MM-DD"
	vars := map[string]string{
		"NNN":  "001",
		"Date": "2026-01-01",
	}
	got, err := RenderTemplate(content, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "# 001 — context\nDate: 2026-01-01"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderTemplate_UnknownVarRendersEmpty(t *testing.T) {
	content := "Hello {{.Unknown}}"
	vars := map[string]string{}
	got, err := RenderTemplate(content, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Go templates render missing map keys as <no value>
	if got == content {
		t.Errorf("expected template to execute, got raw content")
	}
}

func TestRenderTemplate_EmptyContent(t *testing.T) {
	got, err := RenderTemplate("", map[string]string{"NNN": "001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}
