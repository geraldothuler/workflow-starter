package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// HelmResolver parses Helm production values for namespace/service mappings.
type HelmResolver struct{}

func (r *HelmResolver) Name() string { return "helm" }

func (r *HelmResolver) Resolve(spec InputSpec, inputs RunInputs, ctx ResolveContext) (string, bool) {
	if ctx.RepoPath == "" || ctx.Config == nil {
		return "", false
	}

	strategy, ok := ctx.Config.Resolve.Strategies["helm"]
	if !ok {
		return "", false
	}

	yamlPaths, ok := strategy.FieldMappings[spec.Name]
	if !ok || len(yamlPaths) == 0 {
		return "", false
	}

	for _, pattern := range strategy.SearchFiles {
		matches, err := filepath.Glob(filepath.Join(ctx.RepoPath, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			data, err := os.ReadFile(match)
			if err != nil {
				continue
			}

			var values map[string]any
			if err := yaml.Unmarshal(data, &values); err != nil {
				continue
			}

			for _, path := range yamlPaths {
				if val := walkYAMLPath(values, path); val != "" {
					return val, true
				}
			}
		}
	}

	return "", false
}

// walkYAMLPath traverses a nested YAML map using a dot-separated path.
// Returns the string value at the path, or empty string if not found.
func walkYAMLPath(data map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var current any = data

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}

	switch v := current.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case float64:
		if v == float64(int(v)) {
			return fmt.Sprintf("%d", int(v))
		}
		return fmt.Sprintf("%v", v)
	default:
		return ""
	}
}
