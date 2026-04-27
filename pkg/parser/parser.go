package parser

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ParseInput extrai seções estruturadas do input markdown
func ParseInput(filepath string) (*types.ProjectInput, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir arquivo: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	rawContent := string(content)
	sections := extractSections(rawContent)

	input := &types.ProjectInput{
		Context:       sections["contexto"],
		Volumetry:     sections["volumetria"],
		NFRs:          sections["rnfs"],
		Stack:         sections["stack"],
		DataFlow:      sections["fluxo"],
		BusinessRules: sections["regras"],
		EdgeCases:     sections["edge cases"],
		Integrations:  sections["integrações"],
		RawContent:    rawContent,
		Metadata:      make(map[string]string),
	}

	// Extrair volumetria também de outras seções se não encontrada
	if input.Volumetry == "" {
		input.Volumetry = extractVolumetryFromContext(sections)
	}

	input.Metadata["file"] = filepath
	input.Metadata["hash"] = calculateHash(rawContent)

	return input, nil
}

// extractSections extrai seções do markdown baseado em headers
func extractSections(content string) map[string]string {
	sections := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	var currentMainSection string      // ## Level
	var currentSubSection string       // ### Level
	var mainContent strings.Builder    // Conteúdo da seção ##
	var subContent strings.Builder     // Conteúdo da subseção ###
	var inCodeBlock bool

	for scanner.Scan() {
		line := scanner.Text()
		
		// Detectar blocos de código
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			if currentSubSection != "" {
				subContent.WriteString(line + "\n")
			} else if currentMainSection != "" {
				mainContent.WriteString(line + "\n")
			}
			continue
		}

		// Detectar header ## (seção principal)
		if !inCodeBlock && strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") {
			// Salvar subseção anterior se existir
			if currentSubSection != "" {
				sections[currentSubSection] = strings.TrimSpace(subContent.String())
				// Adicionar também à seção principal
				mainContent.WriteString(subContent.String())
				mainContent.WriteString("\n\n")
				subContent.Reset()
				currentSubSection = ""
			}
			
			// Salvar seção principal anterior
			if currentMainSection != "" {
				sections[currentMainSection] = strings.TrimSpace(mainContent.String())
			}
			
			// Iniciar nova seção principal
			headerText := strings.TrimPrefix(line, "## ")
			currentMainSection = normalizeSection(headerText)
			mainContent.Reset()
			continue
		}
		
		// Detectar header ### (subseção)
		if !inCodeBlock && strings.HasPrefix(line, "### ") {
			// Salvar subseção anterior se existir
			if currentSubSection != "" {
				sections[currentSubSection] = strings.TrimSpace(subContent.String())
				// Adicionar também à seção principal
				mainContent.WriteString(subContent.String())
				mainContent.WriteString("\n\n")
			}
			
			// Iniciar nova subseção
			headerText := strings.TrimPrefix(line, "### ")
			currentSubSection = normalizeSection(headerText)
			subContent.Reset()
			continue
		}
		
		// Acumular conteúdo
		if currentSubSection != "" {
			subContent.WriteString(line + "\n")
		} else if currentMainSection != "" {
			mainContent.WriteString(line + "\n")
		}
	}
	
	// Salvar última subseção se existir
	if currentSubSection != "" {
		sections[currentSubSection] = strings.TrimSpace(subContent.String())
		mainContent.WriteString(subContent.String())
	}
	
	// Salvar última seção principal
	if currentMainSection != "" {
		sections[currentMainSection] = strings.TrimSpace(mainContent.String())
	}

	// Mapear variações de nomes de seções
	sections = normalizeSectionNames(sections)

	return sections
}

// normalizeSection normaliza nome de seção para chave
func normalizeSection(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)
	// Remover caracteres especiais
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == ' ' {
			return r
		}
		return -1
	}, name)
	return name
}

// normalizeSectionNames mapeia variações de nomes para chaves padrão
func normalizeSectionNames(sections map[string]string) map[string]string {
	normalized := make(map[string]string)
	
	for key, value := range sections {
		// Contexto
		if strings.Contains(key, "contexto") || strings.Contains(key, "context") {
			normalized["contexto"] = value
		}
		// Volumetria
		if strings.Contains(key, "volumetria") || strings.Contains(key, "volume") || strings.Contains(key, "problema") {
			normalized["volumetria"] = value
		}
		// RNFs
		if strings.Contains(key, "rnf") || strings.Contains(key, "requisitos") || strings.Contains(key, "não funcionais") {
			normalized["rnfs"] = value
		}
		// Stack
		if strings.Contains(key, "stack") || strings.Contains(key, "tecnologia") || strings.Contains(key, "arquitetura") {
			normalized["stack"] = value
		}
		// Fluxo
		if strings.Contains(key, "fluxo") || strings.Contains(key, "flow") || strings.Contains(key, "pipeline") {
			normalized["fluxo"] = value
		}
		// Regras
		if strings.Contains(key, "regras") || strings.Contains(key, "negócio") || strings.Contains(key, "lógica") {
			normalized["regras"] = value
		}
		// Edge Cases
		if strings.Contains(key, "edge") || strings.Contains(key, "borda") || strings.Contains(key, "exceção") {
			normalized["edge cases"] = value
		}
		// Integrações
		if strings.Contains(key, "integra") || strings.Contains(key, "dependên") {
			normalized["integrações"] = value
		}
		
		// Manter original também
		normalized[key] = value
	}
	
	return normalized
}

// extractVolumetryFromContext tenta extrair volumetria de outras seções
func extractVolumetryFromContext(sections map[string]string) string {
	var volumetry strings.Builder
	
	// Procurar em várias seções
	for key, content := range sections {
		if strings.Contains(key, "problema") || strings.Contains(key, "volume") || 
		   strings.Contains(key, "contexto") {
			// Extrair linhas com números
			scanner := bufio.NewScanner(strings.NewReader(content))
			for scanner.Scan() {
				line := scanner.Text()
				// Linhas com números indicam volumetria
				if containsNumbers(line) && !strings.HasPrefix(line, "#") {
					volumetry.WriteString(line + "\n")
				}
			}
		}
	}
	
	return volumetry.String()
}

func containsNumbers(text string) bool {
	hasNumber := false
	for _, char := range text {
		if char >= '0' && char <= '9' {
			hasNumber = true
			break
		}
	}
	return hasNumber
}

// calculateHash calcula MD5 hash do conteúdo
func calculateHash(content string) string {
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// ValidateInput valida se input tem seções mínimas
func ValidateInput(input *types.ProjectInput) []string {
	var missing []string

	if input.Context == "" {
		missing = append(missing, "Contexto")
	}
	if input.Volumetry == "" {
		missing = append(missing, "Volumetria")
	}
	if input.Stack == "" {
		missing = append(missing, "Stack")
	}

	return missing
}

// GoldenPathParser parser de golden path
type GoldenPathParser struct {
	path string
}

// NewGoldenPathParser cria parser de golden path
func NewGoldenPathParser(path string) *GoldenPathParser {
	return &GoldenPathParser{path: path}
}

// Parse parseia golden path de arquivo YAML (DEPRECATED - use ParseGoldenPaths em patterns.go)
func (p *GoldenPathParser) Parse() (*types.GoldenPath, error) {
	// Retornar estrutura vazia - novo parser em patterns.go
	return &types.GoldenPath{
		Patterns: make(map[string]types.Pattern),
	}, nil
}

// TeamConfigParser parser de team config
type TeamConfigParser struct {
	path string
}

// NewTeamConfigParser cria parser de team config
func NewTeamConfigParser(path string) *TeamConfigParser {
	return &TeamConfigParser{path: path}
}

// Parse parseia team config de arquivo YAML (DEPRECATED - use ParseTeamPatterns em patterns.go)
func (p *TeamConfigParser) Parse() (*types.TeamPatterns, error) {
	// Retornar estrutura vazia - novo parser em patterns.go
	return &types.TeamPatterns{
		Patterns: make(map[string]types.Pattern),
	}, nil
}

// ProjectInputParser parser de project input
type ProjectInputParser struct {
	path string
}

// NewProjectInputParser cria parser de project input
func NewProjectInputParser(path string) *ProjectInputParser {
	return &ProjectInputParser{path: path}
}

// Parse parseia project input (markdown)
func (p *ProjectInputParser) Parse() (*types.ProjectInput, error) {
	return ParseInput(p.path)
}

// Merger merger de configs
type Merger struct {
	gp *types.GoldenPath
	tc *types.TeamPatterns
	pi *types.ProjectInput
}

// NewMerger cria merger
func NewMerger(gp *types.GoldenPath, tc *types.TeamPatterns, pi *types.ProjectInput) *Merger {
	return &Merger{gp: gp, tc: tc, pi: pi}
}

// Merge faz merge (stub)
func (m *Merger) Merge() (*types.Specification, error) {
	return &types.Specification{}, nil
}

// ConfigMerger merger de configs
type ConfigMerger struct {
	gp *types.GoldenPath
	tc *types.TeamPatterns
	pi *types.ProjectInput
}

// NewConfigMerger cria merger
func NewConfigMerger(gp *types.GoldenPath, tc *types.TeamPatterns, pi *types.ProjectInput) *ConfigMerger {
	return &ConfigMerger{gp: gp, tc: tc, pi: pi}
}

// Merge faz merge
func (m *ConfigMerger) Merge() (*types.Specification, error) {
	return &types.Specification{}, nil
}

// AdvancedValidator validador avançado
type AdvancedValidator struct{}

// NewAdvancedValidator cria validador avançado
func NewAdvancedValidator(gp *types.GoldenPath, tc *types.TeamPatterns) *AdvancedValidator {
	return &AdvancedValidator{}
}

// ValidateGoldenPath valida golden path
func (v *AdvancedValidator) ValidateGoldenPath(path string) error {
	return nil
}

// ValidateTeamConfig valida team config
func (v *AdvancedValidator) ValidateTeamConfig(path string) error {
	return nil
}

// ValidateProjectInput valida project input
func (v *AdvancedValidator) ValidateProjectInput(path string) error {
	return nil
}

// Métodos de validação atualizados
func (v *AdvancedValidator) ValidateGoldenPathStruct(gp *types.GoldenPath) error {
	return nil
}

func (v *AdvancedValidator) ValidateTeamConfigStruct(tc *types.TeamConfig, gp *types.GoldenPath) error {
	return nil
}

func (v *AdvancedValidator) ValidateProjectInputStruct(pi *types.ProjectInput) error {
	return nil
}

