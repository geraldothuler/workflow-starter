package runner

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SessionResolver reads values from ~/.workflow/session.yml.
type SessionResolver struct{}

func (r *SessionResolver) Name() string { return "session" }

func (r *SessionResolver) Resolve(spec InputSpec, inputs RunInputs, ctx ResolveContext) (string, bool) {
	if ctx.PersonalDir == "" {
		return "", false
	}

	sourceFile := "session.yml"
	if ctx.Config != nil {
		if s, ok := ctx.Config.Resolve.Strategies["session"]; ok && s.SourceFile != "" {
			sourceFile = s.SourceFile
		}
	}

	path := filepath.Join(ctx.PersonalDir, sourceFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var session map[string]string
	if err := yaml.Unmarshal(data, &session); err != nil {
		return "", false
	}

	val, ok := session[spec.Name]
	return val, ok && val != ""
}
