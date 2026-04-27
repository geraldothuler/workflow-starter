package exporter

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// templateFuncMap returns the custom template functions available in YAML body templates.
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"base64":     tplBase64,
		"jsonEscape": tplJSONEscape,
		"jsonArray":  tplJSONArray,
		"join":       tplJoin,
		"adfText":    tplADFText,
		"printf":     fmt.Sprintf,
		"default":    tplDefault,
	}
}

// tplBase64 encodes username:password as base64 for Basic auth.
// Usage: {{base64 .user .token}}
func tplBase64(parts ...string) string {
	joined := strings.Join(parts, ":")
	return base64.StdEncoding.EncodeToString([]byte(joined))
}

// tplJSONEscape escapes a string for safe embedding in JSON.
// Usage: {{jsonEscape .epic.Description}}
func tplJSONEscape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	// json.Marshal wraps in quotes — strip them for embedding in a larger JSON string
	return string(b[1 : len(b)-1])
}

// tplJSONArray converts a string slice to a JSON array.
// Usage: {{jsonArray .epic.Tags}}
func tplJSONArray(items []string) string {
	if items == nil {
		return "[]"
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// tplJoin joins a string slice with a separator.
// Usage: {{join .story.Tags ", "}}
func tplJoin(items []string, sep string) string {
	return strings.Join(items, sep)
}

// tplADFText converts plain text to Atlassian Document Format (ADF) JSON.
// ADF is required by Jira Cloud REST API v3 for description fields.
// Usage: {{adfText .epic.Description}}
func tplADFText(text string) string {
	// Split text into paragraphs
	paragraphs := strings.Split(text, "\n\n")
	content := make([]map[string]any, 0, len(paragraphs))

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		content = append(content, map[string]any{
			"type": "paragraph",
			"content": []map[string]any{
				{
					"type": "text",
					"text": para,
				},
			},
		})
	}

	doc := map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}

	b, err := json.Marshal(doc)
	if err != nil {
		return `{"type":"doc","version":1,"content":[]}`
	}
	return string(b)
}

// tplDefault returns the fallback value if the input is empty.
// Usage: {{default .field "N/A"}}
func tplDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// expandTemplate executes a Go text/template string with the given data and custom funcs.
func expandTemplate(name, tmpl string, data any) (string, error) {
	t, err := template.New(name).Funcs(templateFuncMap()).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// extractField extracts a value from a JSON response using dot-path notation.
// Supports nested paths like "data.projectCreate.project.id".
func extractField(data any, path string) string {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		default:
			return ""
		}
	}

	switch v := current.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case json.Number:
		return v.String()
	default:
		if current == nil {
			return ""
		}
		return fmt.Sprintf("%v", current)
	}
}
