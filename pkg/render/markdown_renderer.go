package render

import (
	"bytes"
	"strings"
	"text/template"
)

// RenderMarkdown renders LensData as a Markdown document using the embedded template.
func RenderMarkdown(data *LensData) (string, error) {
	tmplData, err := templatesFS.ReadFile("templates/backlog.md.tmpl")
	if err != nil {
		return "", err
	}

	funcMap := template.FuncMap{
		"gt": func(a, b int) bool { return a > b },
		"join": func(items []string, sep string) string {
			return strings.Join(items, sep)
		},
	}

	t, err := template.New("backlog.md").Funcs(funcMap).Parse(string(tmplData))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
