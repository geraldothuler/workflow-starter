package doccheck

import (
	_ "embed"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/test_frontier.yml
var testFrontierYAML []byte

// FrontierRules holds the full set of test frontier classifications.
type FrontierRules struct {
	Frontiers []Frontier `yaml:"frontiers"`
}

// Frontier defines a single integration test frontier.
type Frontier struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Detect   []string `yaml:"detect"`
	Required []string `yaml:"required"`
	Cost     string   `yaml:"cost"`
}

// FrontierResult is the evaluation output for a single file.
type FrontierResult struct {
	FrontierID    string
	FrontierName  string
	RequiredTests []string
	Cost          string
}

// LoadFrontierRules parses the embedded test_frontier.yml config.
func LoadFrontierRules() (*FrontierRules, error) {
	var rules FrontierRules
	if err := yaml.Unmarshal(testFrontierYAML, &rules); err != nil {
		return nil, err
	}
	return &rules, nil
}

// EvalFrontier checks which frontier a file belongs to based on its content.
// Returns the first matching frontier (most specific, ordered by YAML position),
// or nil if no frontier matches.
func EvalFrontier(filePath string, content string, rules *FrontierRules) *FrontierResult {
	for _, f := range rules.Frontiers {
		for _, pattern := range f.Detect {
			if strings.Contains(content, pattern) {
				return &FrontierResult{
					FrontierID:    f.ID,
					FrontierName:  f.Name,
					RequiredTests: f.Required,
					Cost:          f.Cost,
				}
			}
		}
	}
	return nil
}
