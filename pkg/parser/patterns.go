package parser

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ParseGoldenPaths extrai patterns de golden-paths.md
func ParseGoldenPaths(filepath string) (*types.GoldenPath, error) {
	file, err := os.Open(filepath)
	if err != nil {
		// Se arquivo não existir, retornar estrutura vazia (não é erro)
		return &types.GoldenPath{
			Patterns: make(map[string]types.Pattern),
		}, nil
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler golden paths: %w", err)
	}

	return parsePatterns(string(content), "GP")
}

// ParseTeamPatterns extrai patterns de team-patterns.md
func ParseTeamPatterns(filepath string) (*types.TeamPatterns, error) {
	file, err := os.Open(filepath)
	if err != nil {
		// Se arquivo não existir, retornar estrutura vazia (não é erro)
		return &types.TeamPatterns{
			Patterns: make(map[string]types.Pattern),
		}, nil
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler team patterns: %w", err)
	}

	tp := &types.TeamPatterns{
		Patterns: make(map[string]types.Pattern),
	}
	
	gp, err := parsePatterns(string(content), "TP")
	if err != nil {
		return nil, err
	}
	
	tp.Patterns = gp.Patterns
	return tp, nil
}

// parsePatterns extrai patterns de markdown estruturado
func parsePatterns(content string, prefix string) (*types.GoldenPath, error) {
	gp := &types.GoldenPath{
		Patterns: make(map[string]types.Pattern),
	}

	// Dividir em seções (## título)
	sections := strings.Split(content, "\n## ")
	patternCount := 1

	for _, section := range sections {
		if section == "" || !strings.Contains(section, "\n") {
			continue
		}

		lines := strings.Split(section, "\n")
		if len(lines) < 2 {
			continue
		}

		// Primeira linha é o título
		title := strings.TrimSpace(lines[0])
		
		// Pular se for apenas o header do arquivo
		if strings.HasPrefix(title, "#") {
			continue
		}

		pattern := types.Pattern{
			ID:          fmt.Sprintf("%s-%03d", prefix, patternCount),
			Name:        title,
			Description: "",
			When:        "",
			How:         "",
			Decisions:   []string{},
			Validated:   "",
		}

		// Extrair seções do pattern
		var currentSection string
		var currentContent []string

		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])

			// Detectar sub-seções
			if strings.HasPrefix(line, "### Pattern:") {
				if currentSection != "" {
					saveSection(&pattern, currentSection, currentContent)
				}
				currentSection = "pattern_name"
				currentContent = []string{strings.TrimPrefix(line, "### Pattern:")}
			} else if strings.HasPrefix(line, "**Quando usar") || strings.Contains(line, "Quando usar:") {
				if currentSection != "" {
					saveSection(&pattern, currentSection, currentContent)
				}
				currentSection = "when"
				currentContent = []string{}
			} else if strings.HasPrefix(line, "**Implementação validada") || strings.Contains(line, "Implementação:") {
				if currentSection != "" {
					saveSection(&pattern, currentSection, currentContent)
				}
				currentSection = "how"
				currentContent = []string{}
			} else if strings.HasPrefix(line, "**Decisão") || strings.Contains(line, "Decisão:") {
				if currentSection != "" {
					saveSection(&pattern, currentSection, currentContent)
				}
				currentSection = "decisions"
				currentContent = []string{}
			} else if strings.HasPrefix(line, "**Validado em") || strings.Contains(line, "Validado:") {
				if currentSection != "" {
					saveSection(&pattern, currentSection, currentContent)
				}
				currentSection = "validated"
				currentContent = []string{strings.TrimPrefix(line, "**Validado em:**")}
			} else if line != "" && line != "---" {
				currentContent = append(currentContent, line)
			}
		}

		// Salvar última seção
		if currentSection != "" {
			saveSection(&pattern, currentSection, currentContent)
		}

		// Adicionar ao mapa se tiver conteúdo mínimo
		if pattern.Name != "" {
			gp.Patterns[pattern.ID] = pattern
			patternCount++
		}
	}

	return gp, nil
}

// saveSection salva conteúdo na seção apropriada do pattern
func saveSection(pattern *types.Pattern, section string, content []string) {
	text := strings.TrimSpace(strings.Join(content, " "))
	
	switch section {
	case "pattern_name":
		if pattern.Name == "" {
			pattern.Name = text
		}
	case "when":
		pattern.When = text
	case "how":
		pattern.How = text
	case "decisions":
		// Dividir por bullets ou linhas
		for _, line := range content {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line != "" && !strings.HasPrefix(line, "**") {
				pattern.Decisions = append(pattern.Decisions, line)
			}
		}
	case "validated":
		pattern.Validated = text
	}
}

// ParseGoldenPathsFromContent extrai patterns a partir de conteúdo em memória
func ParseGoldenPathsFromContent(content string) (*types.GoldenPath, error) {
	return parsePatterns(content, "GP")
}

// ParseTeamPatternsFromContent extrai team patterns a partir de conteúdo em memória
func ParseTeamPatternsFromContent(content string) (*types.TeamPatterns, error) {
	gp, err := parsePatterns(content, "TP")
	if err != nil {
		return nil, err
	}
	return &types.TeamPatterns{Patterns: gp.Patterns}, nil
}

// DetectPatternReferences detecta menções a patterns em texto
func DetectPatternReferences(text string, gp *types.GoldenPath, tp *types.TeamPatterns) []string {
	references := []string{}
	seenMap := make(map[string]bool)
	
	textLower := strings.ToLower(text)

	// Buscar por IDs explícitos (GP-001, TP-003, etc)
	idRegex := regexp.MustCompile(`(GP|TP)-\d{3}`)
	matches := idRegex.FindAllString(strings.ToUpper(text), -1)
	for _, match := range matches {
		if !seenMap[match] {
			references = append(references, match)
			seenMap[match] = true
		}
	}

	// Buscar por nomes de patterns
	if gp != nil {
		for id, pattern := range gp.Patterns {
			nameLower := strings.ToLower(pattern.Name)
			if strings.Contains(textLower, nameLower) && !seenMap[id] {
				references = append(references, id)
				seenMap[id] = true
			}
		}
	}

	if tp != nil {
		for id, pattern := range tp.Patterns {
			nameLower := strings.ToLower(pattern.Name)
			if strings.Contains(textLower, nameLower) && !seenMap[id] {
				references = append(references, id)
				seenMap[id] = true
			}
		}
	}

	return references
}
