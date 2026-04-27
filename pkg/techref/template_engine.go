package techref

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
	"gopkg.in/yaml.v3"
)

// TemplateVars holds variables available for template rendering.
type TemplateVars struct {
	EpicTitle      string
	EpicID         string
	StoryTitle     string
	StoryID        string
	ProjectContext string
	Scope          string
}

// DeepDiveTemplate represents a single technology template.
type DeepDiveTemplate struct {
	WhatIs        string   `yaml:"what_is"`
	WhyHere       string   `yaml:"why_here"`
	Configuration []string `yaml:"configuration"`
	Patterns      []string `yaml:"patterns"`
	Decisions     []string `yaml:"decisions"`
}

// templateStore holds all loaded templates indexed by technology name.
type templateStore struct {
	Templates map[string]DeepDiveTemplate `yaml:"templates"`
	// lowerIndex maps lowercase name → original key for case-insensitive lookup.
	lowerIndex map[string]string
}

// loadTemplateStore parses YAML bytes into a templateStore.
func loadTemplateStore(data []byte) (*templateStore, error) {
	var ts templateStore
	if err := yaml.Unmarshal(data, &ts); err != nil {
		return nil, err
	}
	ts.lowerIndex = make(map[string]string, len(ts.Templates))
	for key := range ts.Templates {
		ts.lowerIndex[strings.ToLower(key)] = key
	}
	return &ts, nil
}

// find looks up a template by name (case-insensitive).
func (ts *templateStore) find(term string) (*DeepDiveTemplate, bool) {
	lower := strings.ToLower(strings.TrimSpace(term))
	if originalKey, ok := ts.lowerIndex[lower]; ok {
		tmpl := ts.Templates[originalKey]
		return &tmpl, true
	}
	return nil, false
}

// loadTemplates reads deep_dive_templates.yml from the embedded config FS.
func (r *TechRegistry) loadTemplates() {
	data, err := defaultConfigs.ReadFile("config/deep_dive_templates.yml")
	if err != nil {
		return // Templates are optional — silent skip.
	}
	ts, err := loadTemplateStore(data)
	if err != nil {
		return
	}
	r.templateStore = ts
}

// TemplateStore returns the loaded template store (may be nil).
func (r *TechRegistry) TemplateStore() *templateStore {
	return r.templateStore
}

// GenerateFromTemplate tries to produce a DeepDive from a pre-defined template.
// Returns (dive, true) on success, or (zero, false) if no template matches.
func GenerateFromTemplate(reg *TechRegistry, term, scope string, vars TemplateVars) (types.DeepDive, bool) {
	if reg == nil || reg.templateStore == nil {
		return types.DeepDive{}, false
	}

	// Direct lookup (case-insensitive).
	tmpl, ok := reg.templateStore.find(term)
	if !ok {
		// Try canonical form fallback.
		canonical := reg.NormalizeToCanonical(term)
		if canonical != term {
			tmpl, ok = reg.templateStore.find(canonical)
		}
	}
	if !ok {
		return types.DeepDive{}, false
	}

	dive := types.DeepDive{
		Term:          term,
		WhatIs:        renderField(tmpl.WhatIs, vars),
		WhyHere:       renderField(tmpl.WhyHere, vars),
		Configuration: renderField(strings.Join(tmpl.Configuration, "\n"), vars),
		Patterns:      tmpl.Patterns,
		Decisions:     tmpl.Decisions,
		Scope:         scope,
		Source:        types.Tag{Source: "template"},
	}
	return dive, true
}

// renderField renders a Go text/template string with vars.
// On any error, returns the raw string as-is (safe fallback).
func renderField(tmplStr string, vars TemplateVars) string {
	t, err := template.New("field").Parse(tmplStr)
	if err != nil {
		return tmplStr
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return tmplStr
	}
	return buf.String()
}
