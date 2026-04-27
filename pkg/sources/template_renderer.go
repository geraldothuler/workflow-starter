package sources

import (
	"fmt"
	"strings"
	"text/template"
)

// RenderTemplate renders a Go text/template with the given data.
// Used when markdown.mode is "template" for custom conversion.
func RenderTemplate(tmplStr string, data map[string]any) (string, error) {
	if tmplStr == "" {
		return "", fmt.Errorf("template string is empty")
	}

	tmpl, err := template.New("markdown").Funcs(templateFuncs()).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderTitle renders the title template with extracted variables.
func RenderTitle(titleTemplate string, vars map[string]string) (string, error) {
	if titleTemplate == "" {
		return "", nil
	}

	// Convert map[string]string to map[string]any for template
	data := make(map[string]any, len(vars))
	for k, v := range vars {
		data[k] = v
	}

	tmpl, err := template.New("title").Parse(titleTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse title template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute title template: %w", err)
	}

	return buf.String(), nil
}

// templateFuncs provides useful template functions for markdown rendering.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"join":    strings.Join,
		"upper":   strings.ToUpper,
		"lower":   strings.ToLower,
		"trim":    strings.TrimSpace,
		"replace": strings.ReplaceAll,
		"indent": func(spaces int, s string) string {
			prefix := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line != "" {
					lines[i] = prefix + line
				}
			}
			return strings.Join(lines, "\n")
		},
	}
}
