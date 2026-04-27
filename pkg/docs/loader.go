package docs

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadUseCases reads all use-cases/*/definition.yml under repoRoot
// and returns the parsed definitions sorted by ID.
func LoadUseCases(repoRoot string) ([]UseCaseDef, error) {
	pattern := filepath.Join(repoRoot, "use-cases", "*", "definition.yml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing use-cases: %w", err)
	}

	var defs []UseCaseDef
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var def UseCaseDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		if def.ID != "" {
			defs = append(defs, def)
		}
	}
	return defs, nil
}
