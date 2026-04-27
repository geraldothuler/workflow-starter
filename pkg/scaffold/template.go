package scaffold

import (
	"strings"
	"text/template"
)

// RenderTemplate renders a workflow template with the given variables.
// It first replaces common literal placeholders (YYYY-MM-DD, NNN) with
// Go template syntax, then executes the template with vars as data.
// On parse or execution error, falls back to simple string replacement.
func RenderTemplate(content string, vars map[string]string) (string, error) {
	content = strings.ReplaceAll(content, "YYYY-MM-DD", "{{.Date}}")
	content = strings.ReplaceAll(content, "NNN", "{{.NNN}}")

	tmpl, err := template.New("artefact").Parse(content)
	if err != nil {
		// Template parse failed (e.g. stray {{ in content) — fall back to literal replace.
		result := content
		for k, v := range vars {
			result = strings.ReplaceAll(result, "{{."+k+"}}", v)
		}
		return result, nil
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return content, nil // fallback: return raw
	}
	return buf.String(), nil
}
