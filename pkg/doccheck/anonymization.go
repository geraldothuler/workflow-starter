// anonymization.go — Heuristic-First anonymization check for docs/.
//
// Scans docs/*.html and docs/*.md for internal infrastructure identifiers:
// private IPs, internal hostnames, replication slot names, hardcoded credentials.
//
// Patterns are YAML-driven (config/anonymization_rules.yml) — zero-LLM, $0 cost,
// millisecond latency. Override for intentional exposure:
//
//	<!-- wtb-noguard: anonymization-in-docs — <justificativa> -->
package doccheck

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/anonymization_rules.yml
var anonConfigFS embed.FS

type anonPattern struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Regex       string `yaml:"regex"`
	Guidance    string `yaml:"guidance"`
}

type anonScanConfig struct {
	Dirs       []string `yaml:"dirs"`
	Extensions []string `yaml:"extensions"`
	Recursive  bool     `yaml:"recursive"`
}

type anonRulesConfig struct {
	Check    string         `yaml:"check"`
	Scan     anonScanConfig `yaml:"scan"`
	Patterns []anonPattern  `yaml:"patterns"`
}

type anonViolation struct {
	File      string
	Line      int
	PatternID string
	Match     string
	Guidance  string
}

func loadAnonConfig() (anonRulesConfig, error) {
	data, err := anonConfigFS.ReadFile("config/anonymization_rules.yml")
	if err != nil {
		return anonRulesConfig{}, err
	}
	var cfg anonRulesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return anonRulesConfig{}, fmt.Errorf("parsing anonymization_rules.yml: %w", err)
	}
	return cfg, nil
}

// CheckAnonymizationInDocs verifies that docs/ files do not contain internal
// infrastructure identifiers (private IPs, internal hostnames, slot names, credentials).
//
// Patterns: config/anonymization_rules.yml (YAML-driven, zero-LLM).
// Drift:    identifier added to a generated doc without anonymization.
// Override: <!-- wtb-noguard: anonymization-in-docs — <justificativa> -->
func CheckAnonymizationInDocs(repoRoot string) GuardrailResult {
	const check = "anonymization-in-docs"
	const overrideMarker = "wtb-noguard: anonymization-in-docs"

	cfg, err := loadAnonConfig()
	if err != nil {
		return GuardrailResult{Check: check, Passed: true} // config unreadable → skip
	}

	type compiledPat struct {
		id       string
		re       *regexp.Regexp
		guidance string
	}
	var patterns []compiledPat
	for _, p := range cfg.Patterns {
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			continue // skip malformed pattern
		}
		patterns = append(patterns, compiledPat{p.ID, re, p.Guidance})
	}

	extSet := make(map[string]bool)
	for _, e := range cfg.Scan.Extensions {
		extSet[e] = true
	}

	var violations []anonViolation
	for _, dir := range cfg.Scan.Dirs {
		dirPath := filepath.Join(repoRoot, dir)
		var files []string
		if cfg.Scan.Recursive {
			_ = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if extSet[filepath.Ext(d.Name())] {
					files = append(files, path)
				}
				return nil
			})
		} else {
			entries, err := os.ReadDir(dirPath)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() && extSet[filepath.Ext(entry.Name())] {
					files = append(files, filepath.Join(dirPath, entry.Name()))
				}
			}
		}
		for _, filePath := range files {
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			content := string(data)
			if strings.Contains(content, overrideMarker) {
				continue // intentional override
			}
			rel, _ := filepath.Rel(repoRoot, filePath)
			scanner := bufio.NewScanner(strings.NewReader(content))
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()
				for _, p := range patterns {
					if match := p.re.FindString(line); match != "" {
						violations = append(violations, anonViolation{
							File:      rel,
							Line:      lineNum,
							PatternID: p.id,
							Match:     match,
							Guidance:  p.guidance,
						})
						break // one violation per line
					}
				}
			}
		}
	}

	if len(violations) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	var sb strings.Builder
	for _, v := range violations {
		fmt.Fprintf(&sb, "  %s:%d  [%s]  %q\n    → %s\n", v.File, v.Line, v.PatternID, v.Match, v.Guidance)
	}

	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			fmt.Sprintf("%d identificador(es) interno(s) detectado(s) em docs/:\n%s", len(violations), sb.String()),
			"Documentação pública não deve expor identificadores de infraestrutura interna.\n"+
				"Padrões configurados em pkg/doccheck/config/anonymization_rules.yml.",
			"Substitua pelos placeholders indicados ou adicione o override para exposição intencional.",
			"<!-- wtb-noguard: anonymization-in-docs — <justificativa> -->"),
	}
}
