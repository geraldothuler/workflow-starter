package techref

import (
	"strings"
	"testing"
)

func TestLoadTemplateStore_ValidYAML(t *testing.T) {
	yaml := `templates:
  PostgreSQL:
    what_is: "A database"
    why_here: "Used in {{.EpicTitle}}"
    configuration:
      - "pool size"
    patterns:
      - "Repository Pattern"
    decisions:
      - "Use pgx"
`
	ts, err := loadTemplateStore([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts.Templates) != 1 {
		t.Errorf("expected 1 template, got %d", len(ts.Templates))
	}
}

func TestLoadTemplateStore_InvalidYAML(t *testing.T) {
	_, err := loadTemplateStore([]byte("not: valid: yaml: ["))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestTemplateStore_Find_ExactAndLowercase(t *testing.T) {
	yaml := `templates:
  PostgreSQL:
    what_is: "A database"
    why_here: "Used here"
    configuration: []
    patterns: []
    decisions: []
`
	ts, err := loadTemplateStore([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exact match
	tmpl, ok := ts.find("PostgreSQL")
	if !ok || tmpl == nil {
		t.Error("expected to find PostgreSQL with exact case")
	}

	// Lowercase match
	tmpl, ok = ts.find("postgresql")
	if !ok || tmpl == nil {
		t.Error("expected to find PostgreSQL with lowercase")
	}

	// Not found
	_, ok = ts.find("UnknownTechXYZ")
	if ok {
		t.Error("expected not to find UnknownTechXYZ")
	}
}

func TestGenerateFromTemplate_KnownTech(t *testing.T) {
	reg := DefaultRegistry()

	vars := TemplateVars{
		EpicTitle:      "Auth System",
		EpicID:         "E1",
		ProjectContext: "Banking App",
		Scope:          "epic",
	}

	dive, ok := GenerateFromTemplate(reg, "PostgreSQL", "epic", vars)
	if !ok {
		t.Fatal("expected template match for PostgreSQL")
	}
	if dive.Term != "PostgreSQL" {
		t.Errorf("expected Term=PostgreSQL, got %q", dive.Term)
	}
	if dive.WhatIs == "" {
		t.Error("expected non-empty WhatIs")
	}
	if dive.WhyHere == "" {
		t.Error("expected non-empty WhyHere")
	}
	if dive.Scope != "epic" {
		t.Errorf("expected Scope=epic, got %q", dive.Scope)
	}
	if dive.Source.Source != "template" {
		t.Errorf("expected Source.Source=template, got %q", dive.Source.Source)
	}
}

func TestGenerateFromTemplate_UnknownTech_ReturnsFalse(t *testing.T) {
	reg := DefaultRegistry()
	vars := TemplateVars{EpicTitle: "Test"}
	_, ok := GenerateFromTemplate(reg, "UnknownTechXYZ", "epic", vars)
	if ok {
		t.Error("expected false for unknown tech")
	}
}

func TestGenerateFromTemplate_CaseInsensitive(t *testing.T) {
	reg := DefaultRegistry()
	vars := TemplateVars{EpicTitle: "Test"}

	_, ok := GenerateFromTemplate(reg, "postgresql", "epic", vars)
	if !ok {
		t.Error("expected case-insensitive match for 'postgresql'")
	}
}

func TestGenerateFromTemplate_CanonicalFormFallback(t *testing.T) {
	reg := DefaultRegistry()
	vars := TemplateVars{EpicTitle: "Test"}

	// "postgres" should normalize to "PostgreSQL" via canonical forms
	canonical := reg.NormalizeToCanonical("postgres")
	if canonical == "postgres" {
		t.Skip("no canonical mapping for 'postgres' in this registry")
	}

	_, ok := GenerateFromTemplate(reg, "postgres", "epic", vars)
	if !ok {
		t.Errorf("expected canonical fallback to find template (canonical=%q)", canonical)
	}
}

func TestRenderField_WithVars(t *testing.T) {
	result := renderField("Hello {{.EpicTitle}} in {{.Scope}}", TemplateVars{
		EpicTitle: "Auth",
		Scope:     "epic",
	})
	if result != "Hello Auth in epic" {
		t.Errorf("unexpected render result: %q", result)
	}
}

func TestRenderField_InvalidTemplate_ReturnsFallback(t *testing.T) {
	raw := "Hello {{.Invalid"
	result := renderField(raw, TemplateVars{})
	if result != raw {
		t.Errorf("expected raw string back on error, got %q", result)
	}
}

func TestGenerateFromTemplate_TemplateVarsRendered(t *testing.T) {
	reg := DefaultRegistry()
	vars := TemplateVars{
		EpicTitle:      "Payment Gateway",
		EpicID:         "E3",
		ProjectContext: "E-commerce",
	}

	dive, ok := GenerateFromTemplate(reg, "PostgreSQL", "epic", vars)
	if !ok {
		t.Fatal("expected template match")
	}

	// Check that template variables were rendered (not raw {{.EpicTitle}})
	if strings.Contains(dive.WhyHere, "{{.EpicTitle}}") {
		t.Error("template variable {{.EpicTitle}} was not rendered")
	}
	if !strings.Contains(dive.WhyHere, "Payment Gateway") {
		t.Error("expected rendered EpicTitle in WhyHere")
	}
}

func TestTemplateCount_AtLeast35(t *testing.T) {
	reg := DefaultRegistry()
	if reg.TemplateStore() == nil {
		t.Fatal("template store is nil")
	}
	count := len(reg.TemplateStore().Templates)
	if count < 35 {
		t.Errorf("expected at least 35 templates, got %d", count)
	}
	t.Logf("template count: %d", count)
}

func TestGenerateFromTemplate_AllTemplatesRender(t *testing.T) {
	reg := DefaultRegistry()
	if reg.TemplateStore() == nil {
		t.Fatal("template store is nil")
	}

	vars := TemplateVars{
		EpicTitle:      "Test Epic",
		EpicID:         "E99",
		StoryTitle:     "Test Story",
		StoryID:        "S99",
		ProjectContext: "Test Project",
		Scope:          "epic",
	}

	for name := range reg.TemplateStore().Templates {
		t.Run(name, func(t *testing.T) {
			dive, ok := GenerateFromTemplate(reg, name, "epic", vars)
			if !ok {
				t.Fatalf("template %q failed to generate", name)
			}
			if dive.Term != name {
				t.Errorf("expected Term=%q, got %q", name, dive.Term)
			}
			if dive.WhatIs == "" {
				t.Errorf("template %q: WhatIs is empty", name)
			}
			if dive.WhyHere == "" {
				t.Errorf("template %q: WhyHere is empty", name)
			}
			if strings.Contains(dive.WhatIs, "{{") {
				t.Errorf("template %q: WhatIs contains unrendered template variable", name)
			}
			if strings.Contains(dive.WhyHere, "{{") {
				t.Errorf("template %q: WhyHere contains unrendered template variable", name)
			}
			if dive.Source.Source != "template" {
				t.Errorf("template %q: expected Source.Source=template, got %q", name, dive.Source.Source)
			}
		})
	}
}
