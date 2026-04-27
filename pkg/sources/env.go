package sources

import (
	"os"
	"strings"
	"text/template"
)

// expandEnvVars replaces ${VAR_NAME} and $VAR patterns with environment variable values.
// Uses Go's os.Expand which handles both ${VAR} and $VAR syntax.
func expandEnvVars(s string) string {
	return os.Expand(s, os.Getenv)
}

// expandEnvMap expands environment variables in all values of a map.
func expandEnvMap(env map[string]string) map[string]string {
	if len(env) == 0 {
		return env
	}
	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = expandEnvVars(v)
	}
	return result
}

// expandTemplateArgs replaces {{.key}} patterns in step args with variable values.
// Non-string values are passed through unchanged.
func expandTemplateArgs(args map[string]any, vars map[string]string) (map[string]any, error) {
	if len(args) == 0 {
		return args, nil
	}

	result := make(map[string]any, len(args))
	for k, v := range args {
		switch val := v.(type) {
		case string:
			if !strings.Contains(val, "{{") {
				// No template syntax, pass through
				result[k] = val
				continue
			}
			tmpl, err := template.New("arg").Parse(val)
			if err != nil {
				return nil, err
			}
			var buf strings.Builder
			if err := tmpl.Execute(&buf, vars); err != nil {
				return nil, err
			}
			result[k] = buf.String()
		default:
			result[k] = v
		}
	}
	return result, nil
}
