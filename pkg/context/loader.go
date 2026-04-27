package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/features"
	"github.com/Cobliteam/workflow-toolkit/pkg/logging"

	"gopkg.in/yaml.v3"
)

// contentSanitizer filters credentials from loaded documents before they enter the system prompt.
// Defense-in-depth: the SecurityCheckpoint also scans, but filtering at the source is better.
var contentSanitizer = logging.NewSanitizer(true)

// ContextLoader carrega contexto dinâmico para o CLI
type ContextLoader struct {
	basePath string
}

// Context representa o contexto completo para o Claude
type Context struct {
	ClaudeContext string   // CLAUDE.md content
	Skills        []Skill  // Skills relevantes
	Patterns      []Pattern // Patterns aplicáveis
	ActiveDocs    []string // Documentos ACTIVE/IN_PROGRESS
}

// Skill representa um skill disponível
type Skill struct {
	Name        string
	Description string
	Content     string
	Triggers    SkillTriggers
}

// SkillTriggers define quando skill é relevante
type SkillTriggers struct {
	Keywords []string `yaml:"keywords"`
	Tasks    []string `yaml:"tasks"`
}

// Pattern representa um pattern reutilizável
type Pattern struct {
	Name        string
	Description string
	Content     string
}

// StatusDoc representa documento no STATUS.yml
type StatusDoc struct {
	Status string `yaml:"status"`
	File   string `yaml:"file"`
	Files  []string `yaml:"files"`
}

// NewContextLoader cria novo loader
func NewContextLoader(basePath string) *ContextLoader {
	return &ContextLoader{basePath: basePath}
}

// LoadForCommand carrega contexto apropriado para um comando
func (cl *ContextLoader) LoadForCommand(command string, userInput string) (*Context, error) {
	ctx := &Context{}
	
	// 1. Sempre carregar CLAUDE.md (base)
	claudeContent, err := cl.loadClaudeContext()
	if err != nil {
		return nil, fmt.Errorf("failed to load CLAUDE.md: %w", err)
	}
	ctx.ClaudeContext = claudeContent
	
	// 2. Carregar documentos ACTIVE/IN_PROGRESS
	activeDocs, err := cl.loadActiveDocs()
	if err != nil {
		// Não-fatal, apenas log warning
		fmt.Fprintf(os.Stderr, "Warning: failed to load active docs: %v\n", err)
	}
	ctx.ActiveDocs = activeDocs
	
	// 3. Carregar skills relevantes para o comando
	skills, err := cl.loadRelevantSkills(command, userInput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load skills: %v\n", err)
	}
	ctx.Skills = skills
	
	// 4. Carregar patterns aplicáveis
	patterns, err := cl.loadApplicablePatterns(command, userInput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load patterns: %v\n", err)
	}
	ctx.Patterns = patterns
	
	return ctx, nil
}

// loadClaudeContext carrega CLAUDE.md
func (cl *ContextLoader) loadClaudeContext() (string, error) {
	claudePath := filepath.Join(cl.basePath, "CLAUDE.md")
	content, err := os.ReadFile(claudePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// loadActiveDocs carrega docs com status ACTIVE ou IN_PROGRESS
func (cl *ContextLoader) loadActiveDocs() ([]string, error) {
	statusPath := filepath.Join(cl.basePath, "docs", "STATUS.yml")
	
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, err
	}
	
	var status struct {
		Research     map[string]StatusDoc `yaml:"research"`
		Planning     map[string]StatusDoc `yaml:"planning"`
		Savepoints   map[string]StatusDoc `yaml:"savepoints"`
		Architecture map[string]StatusDoc `yaml:"architecture"`
		Compliance   map[string]StatusDoc `yaml:"compliance"`
		Guides       map[string]StatusDoc `yaml:"guides"`
	}

	if err := yaml.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	var docs []string

	// Helper para processar docs
	processMap := func(m map[string]StatusDoc) {
		for _, doc := range m {
			if doc.Status == "ACTIVE" || doc.Status == "IN_PROGRESS" {
				// Carregar arquivo(s)
				if doc.File != "" {
					content, err := cl.loadFile(doc.File)
					if err == nil {
						docs = append(docs, content)
					}
				}
				for _, file := range doc.Files {
					content, err := cl.loadFile(file)
					if err == nil {
						docs = append(docs, content)
					}
				}
			}
		}
	}

	processMap(status.Research)
	processMap(status.Planning)
	processMap(status.Savepoints)
	processMap(status.Architecture)
	processMap(status.Compliance)
	processMap(status.Guides)
	
	return docs, nil
}

// loadRelevantSkills carrega skills baseado no comando e input
func (cl *ContextLoader) loadRelevantSkills(command string, userInput string) ([]Skill, error) {
	statusPath := filepath.Join(cl.basePath, "docs", "STATUS.yml")
	
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, err
	}
	
	var status struct {
		Skills map[string]struct {
			Description string `yaml:"description"`
			File        string `yaml:"file"`
			Triggers    struct {
				Keywords []string `yaml:"keywords"`
				Tasks    []string `yaml:"tasks"`
			} `yaml:"triggers"`
			AutoLoad bool `yaml:"auto_load"`
		} `yaml:"skills"`
	}
	
	if err := yaml.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	
	var skills []Skill

	// Resolver skills via Feature Registry (FEATURES.yml)
	// Isso elimina o commandTasks hardcoded
	featureSkills := make(map[string]bool)
	registry, regErr := features.LoadRegistry(cl.basePath)
	if regErr == nil {
		for _, skillName := range registry.GetSkillsForCommand(command) {
			featureSkills[skillName] = true
		}
	}

	combinedText := strings.ToLower(command + " " + userInput)

	for name, skillData := range status.Skills {
		if !skillData.AutoLoad {
			continue // Pular skills não auto-load (auxiliares)
		}

		// Verificar se skill é relevante
		relevant := false

		// Check Feature Registry (primary source)
		if featureSkills[name] {
			relevant = true
		}

		// Check task triggers from STATUS.yml (fallback)
		for _, trigger := range skillData.Triggers.Tasks {
			if strings.Contains(combinedText, strings.ToLower(trigger)) {
				relevant = true
				break
			}
		}

		// Check keywords
		for _, keyword := range skillData.Triggers.Keywords {
			if strings.Contains(combinedText, strings.ToLower(keyword)) {
				relevant = true
				break
			}
		}

		if relevant {
			content, err := cl.loadFile(skillData.File)
			if err != nil {
				continue
			}

			skills = append(skills, Skill{
				Name:        name,
				Description: skillData.Description,
				Content:     content,
			})
		}
	}
	
	return skills, nil
}

// loadApplicablePatterns carrega patterns baseado no contexto
func (cl *ContextLoader) loadApplicablePatterns(command string, userInput string) ([]Pattern, error) {
	statusPath := filepath.Join(cl.basePath, "docs", "STATUS.yml")
	
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, err
	}
	
	var status struct {
		Patterns map[string]struct {
			Description string   `yaml:"description"`
			File        string   `yaml:"file"`
			WhenToLoad  []string `yaml:"when_to_load"`
		} `yaml:"patterns"`
	}
	
	if err := yaml.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	
	var patterns []Pattern
	combinedText := strings.ToLower(command + " " + userInput)

	// Resolver patterns via Feature Registry (FEATURES.yml)
	featurePatterns := make(map[string]bool)
	registry, regErr := features.LoadRegistry(cl.basePath)
	if regErr == nil {
		for _, patternName := range registry.GetPatternsForCommand(command) {
			featurePatterns[patternName] = true
		}
	}

	for name, patternData := range status.Patterns {
		// Verificar se pattern é aplicável
		applicable := false

		// Check Feature Registry (primary source)
		if featurePatterns[name] {
			applicable = true
		}

		// Check when_to_load conditions (fallback)
		for _, condition := range patternData.WhenToLoad {
			if strings.Contains(combinedText, strings.ToLower(condition)) {
				applicable = true
				break
			}
		}
		
		if applicable {
			content, err := cl.loadFile(patternData.File)
			if err != nil {
				continue
			}
			
			patterns = append(patterns, Pattern{
				Name:        name,
				Description: patternData.Description,
				Content:     content,
			})
		}
	}
	
	return patterns, nil
}

// loadFile carrega arquivo do filesystem e sanitiza credenciais antes de retornar.
// SECURITY: All loaded content may end up in the LLM system prompt, so credentials
// must be removed at the source. This is defense-in-depth — the SecurityCheckpoint
// also scans, but filtering here prevents credentials from even entering the prompt builder.
func (cl *ContextLoader) loadFile(relativePath string) (string, error) {
	fullPath := filepath.Join(cl.basePath, relativePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	// Sanitize credentials from loaded content before it enters the system prompt
	return contentSanitizer.Sanitize(string(content)), nil
}

// BuildSystemPrompt constrói prompt do sistema com todo contexto
func (ctx *Context) BuildSystemPrompt() string {
	var sb strings.Builder
	
	// 1. CLAUDE.md (contexto base)
	sb.WriteString("# Context: Workflow Platform\n\n")
	sb.WriteString(ctx.ClaudeContext)
	sb.WriteString("\n\n")
	
	// 2. Active docs
	if len(ctx.ActiveDocs) > 0 {
		sb.WriteString("# Active Documentation\n\n")
		for _, doc := range ctx.ActiveDocs {
			sb.WriteString(doc)
			sb.WriteString("\n\n---\n\n")
		}
	}
	
	// 3. Skills
	if len(ctx.Skills) > 0 {
		sb.WriteString("# Relevant Skills\n\n")
		sb.WriteString("The following skills contain best practices you should follow:\n\n")
		for _, skill := range ctx.Skills {
			sb.WriteString(fmt.Sprintf("## Skill: %s\n\n", skill.Name))
			sb.WriteString(fmt.Sprintf("*%s*\n\n", skill.Description))
			sb.WriteString(skill.Content)
			sb.WriteString("\n\n---\n\n")
		}
	}
	
	// 4. Patterns
	if len(ctx.Patterns) > 0 {
		sb.WriteString("# Applicable Patterns\n\n")
		sb.WriteString("The following patterns should be considered:\n\n")
		for _, pattern := range ctx.Patterns {
			sb.WriteString(fmt.Sprintf("## Pattern: %s\n\n", pattern.Name))
			sb.WriteString(fmt.Sprintf("*%s*\n\n", pattern.Description))
			sb.WriteString(pattern.Content)
			sb.WriteString("\n\n---\n\n")
		}
	}
	
	return sb.String()
}

// Summary retorna resumo do contexto carregado (para debug)
func (ctx *Context) Summary() string {
	return fmt.Sprintf(
		"Context loaded: %d skills, %d patterns, %d active docs",
		len(ctx.Skills),
		len(ctx.Patterns),
		len(ctx.ActiveDocs),
	)
}
