package doccheck

import (
	_ "embed"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/test_gate.yml
var testGateYML []byte

// TestGateRules holds the parsed test gate configuration.
type TestGateRules struct {
	TestGate struct {
		Steps                 []string                         `yaml:"steps"`
		PackageClassification map[string]PackageClassification `yaml:"package_classification"`
		Rules                 []string                         `yaml:"rules"`
	} `yaml:"test_gate"`
}

// PackageClassification defines a testability tier.
type PackageClassification struct {
	Description    string   `yaml:"description"`
	CoverageTarget int      `yaml:"coverage_target"`
	Patterns       []string `yaml:"patterns"`
}

// TestGap represents a package that may need test coverage.
type TestGap struct {
	Package        string
	Classification string
	Rule           string
}

// LoadTestGateRules loads the test gate rules from the embedded YAML.
func LoadTestGateRules() (*TestGateRules, error) {
	var rules TestGateRules
	if err := yaml.Unmarshal(testGateYML, &rules); err != nil {
		return nil, err
	}
	return &rules, nil
}

// ClassifyPackage returns "testable", "partial", or "io_limited" for a given
// package path. If no pattern matches, returns "testable" as the default.
func ClassifyPackage(pkgPath string, rules *TestGateRules) string {
	for classification, cfg := range rules.TestGate.PackageClassification {
		for _, pattern := range cfg.Patterns {
			if strings.Contains(pkgPath, pattern) {
				return classification
			}
		}
	}
	return "testable"
}

// CheckTestGate examines changed file paths and returns gaps — packages that
// may need test coverage based on their classification.
func CheckTestGate(changedFiles []string, rules *TestGateRules) []TestGap {
	seen := make(map[string]bool)
	var gaps []TestGap

	for _, file := range changedFiles {
		// Skip test files themselves
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		// Skip non-Go files
		if !strings.HasSuffix(file, ".go") {
			continue
		}

		classification := ClassifyPackage(file, rules)
		// io_limited packages don't generate gaps
		if classification == "io_limited" {
			continue
		}

		// Deduplicate by package directory
		pkgDir := file
		if idx := strings.LastIndex(file, "/"); idx >= 0 {
			pkgDir = file[:idx+1]
		}
		if seen[pkgDir] {
			continue
		}
		seen[pkgDir] = true

		rule := "Nova funcao publica → teste obrigatorio"
		if classification == "partial" {
			rule = "Mix de logica testavel + I/O interativo — meta 50-70%"
		}

		gaps = append(gaps, TestGap{
			Package:        pkgDir,
			Classification: classification,
			Rule:           rule,
		})
	}
	return gaps
}
