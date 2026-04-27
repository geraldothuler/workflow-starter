package techref

import (
	"fmt"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// TechRelevance representa o nível de relevância de uma tecnologia
type TechRelevance string

const (
	// TRIVIAL - Termo óbvio, não precisa deep dive (HTTP, JSON, etc)
	TRIVIAL TechRelevance = "trivial"

	// STANDARD - Tecnologia comum do épico, deep dive no nível do épico
	STANDARD TechRelevance = "standard"

	// SPECIFIC - Uso único em apenas 1 história, deep dive específico
	SPECIFIC TechRelevance = "specific"

	// CRITICAL - Tecnologia core do projeto, deep dive detalhado no épico
	CRITICAL TechRelevance = "critical"
)

// TechClassification contém o resultado da classificação
type TechClassification struct {
	Term      string
	Relevance TechRelevance
	Reason    string // Explicação da classificação
	Scope     string // "epic" ou "story"
	EpicID    string // ID do épico (se aplicável)
	StoryID   string // ID da história (se aplicável)
}

// ClassifierConfig permite customizar critérios de classificação
type ClassifierConfig struct {
	// CoreTechnologies - lista de techs core do projeto (sempre CRITICAL)
	CoreTechnologies []string

	// SpecificKeywords - keywords que indicam uso específico
	SpecificKeywords []string

	// EnableContextAwareness - usar contexto para refinar classificação
	EnableContextAwareness bool
}

// DefaultClassifierConfig retorna configuração padrão (registry-based)
func DefaultClassifierConfig() ClassifierConfig {
	reg := DefaultRegistry()
	return ClassifierConfig{
		CoreTechnologies:      []string{},
		SpecificKeywords:      reg.SpecificKeywords(),
		EnableContextAwareness: true,
	}
}

// ClassifyTech classifica uma tecnologia dentro de um épico
func ClassifyTech(tech string, epic types.Epic, config ClassifierConfig) TechClassification {
	return ClassifyTechWithRegistry(DefaultRegistry(), tech, epic, config)
}

// ClassifyTechWithRegistry classifica usando registry configurável
func ClassifyTechWithRegistry(reg *TechRegistry, tech string, epic types.Epic, config ClassifierConfig) TechClassification {
	if reg == nil {
		reg = DefaultRegistry()
	}

	// 1. É trivial?
	if reg.IsTrivial(tech) {
		return TechClassification{
			Term:      tech,
			Relevance: TRIVIAL,
			Reason:    "Termo trivial (conceito básico)",
			Scope:     "none",
		}
	}

	// 2. É tecnologia core?
	if isCoreTech(tech, config.CoreTechnologies) {
		return TechClassification{
			Term:      tech,
			Relevance: CRITICAL,
			Reason:    "Tecnologia core do projeto",
			Scope:     "epic",
			EpicID:    epic.ID,
		}
	}

	// 3. Usado em quantas histórias?
	storiesUsingTech := countStoriesUsingTech(epic, tech)

	// 3a. Usado em apenas 1 história? → SPECIFIC
	if storiesUsingTech == 1 {
		storyID := findStoryUsingTech(epic, tech)
		return TechClassification{
			Term:      tech,
			Relevance: SPECIFIC,
			Reason:    "Usado em apenas 1 história do épico",
			Scope:     "story",
			EpicID:    epic.ID,
			StoryID:   storyID,
		}
	}

	// 4. Verificar se contexto indica uso específico
	if config.EnableContextAwareness {
		for _, story := range epic.Stories {
			if storyMentionsTech(story, tech) {
				if hasSpecificUsage(story, tech, config.SpecificKeywords) {
					return TechClassification{
						Term:      tech,
						Relevance: SPECIFIC,
						Reason:    "Contexto indica uso específico (keywords encontradas)",
						Scope:     "story",
						EpicID:    epic.ID,
						StoryID:   story.ID,
					}
				}
			}
		}
	}

	// 5. Default: STANDARD (usado em múltiplas histórias, não é core)
	return TechClassification{
		Term:      tech,
		Relevance: STANDARD,
		Reason:    "Tecnologia comum usada em múltiplas histórias",
		Scope:     "epic",
		EpicID:    epic.ID,
	}
}

// ClassifyAllTechsInEpic classifica todas as tecnologias de um épico
func ClassifyAllTechsInEpic(epic types.Epic, config ClassifierConfig) []TechClassification {
	return ClassifyAllTechsInEpicWithRegistry(DefaultRegistry(), epic, config)
}

// ClassifyAllTechsInEpicWithRegistry classifica usando registry
func ClassifyAllTechsInEpicWithRegistry(reg *TechRegistry, epic types.Epic, config ClassifierConfig) []TechClassification {
	if reg == nil {
		reg = DefaultRegistry()
	}

	allTechs := extractAllTechsFromEpicWithRegistry(reg, epic)

	classifications := make([]TechClassification, 0, len(allTechs))
	for _, tech := range allTechs {
		classification := ClassifyTechWithRegistry(reg, tech, epic, config)
		classifications = append(classifications, classification)
	}

	return classifications
}

// FilterByRelevance filtra classificações por nível de relevância
func FilterByRelevance(classifications []TechClassification, relevance TechRelevance) []TechClassification {
	filtered := []TechClassification{}

	for _, c := range classifications {
		if c.Relevance == relevance {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

// groupByScope agrupa classificações por escopo (epic/story)
func groupByScope(classifications []TechClassification) map[string][]TechClassification {
	grouped := map[string][]TechClassification{
		"epic":  {},
		"story": {},
		"none":  {},
	}

	for _, c := range classifications {
		grouped[c.Scope] = append(grouped[c.Scope], c)
	}

	return grouped
}

// GetStatistics retorna estatísticas sobre classificações
func GetStatistics(classifications []TechClassification) map[string]int {
	stats := map[string]int{
		"total":    len(classifications),
		"trivial":  0,
		"standard": 0,
		"specific": 0,
		"critical": 0,
	}

	for _, c := range classifications {
		switch c.Relevance {
		case TRIVIAL:
			stats["trivial"]++
		case STANDARD:
			stats["standard"]++
		case SPECIFIC:
			stats["specific"]++
		case CRITICAL:
			stats["critical"]++
		}
	}

	return stats
}

// ─────────────────────────────────────────────────────────────────
// Helper functions
// ─────────────────────────────────────────────────────────────────

func isCoreTech(tech string, coreTechs []string) bool {
	techLower := strings.ToLower(strings.TrimSpace(tech))

	for _, core := range coreTechs {
		if strings.ToLower(strings.TrimSpace(core)) == techLower {
			return true
		}
	}

	return false
}

func countStoriesUsingTech(epic types.Epic, tech string) int {
	count := 0

	for _, story := range epic.Stories {
		if storyMentionsTech(story, tech) {
			count++
		}
	}

	return count
}

func findStoryUsingTech(epic types.Epic, tech string) string {
	for _, story := range epic.Stories {
		if storyMentionsTech(story, tech) {
			return story.ID
		}
	}
	return ""
}

func storyMentionsTech(story types.Story, tech string) bool {
	techLower := strings.ToLower(tech)

	searchIn := strings.ToLower(
		story.Title + " " +
			story.What + " " +
			story.Why,
	)

	for _, criterion := range story.AcceptanceCriteria {
		searchIn += " " + strings.ToLower(criterion)
	}

	return strings.Contains(searchIn, techLower)
}

func hasSpecificUsage(story types.Story, tech string, keywords []string) bool {
	searchText := strings.ToLower(
		story.Title + " " +
			story.What + " " +
			story.Why,
	)

	for _, criterion := range story.AcceptanceCriteria {
		searchText += " " + strings.ToLower(criterion)
	}

	for _, keyword := range keywords {
		if strings.Contains(searchText, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

// extractAllTechsFromEpic extracts all unique techs from an epic (backward compat).
// FIX: Previously this was a stub that returned empty results.
func extractAllTechsFromEpic(epic types.Epic) []string {
	return extractAllTechsFromEpicWithRegistry(DefaultRegistry(), epic)
}

// extractAllTechsFromEpicWithRegistry extracts techs using registry.
func extractAllTechsFromEpicWithRegistry(reg *TechRegistry, epic types.Epic) []string {
	techsMap := make(map[string]bool)

	for _, story := range epic.Stories {
		techs := ExtractTechsFromStoryWithRegistry(reg, story)
		for _, tech := range techs {
			canonical := reg.NormalizeToCanonical(tech)
			techsMap[canonical] = true
		}
	}

	techs := make([]string, 0, len(techsMap))
	for tech := range techsMap {
		techs = append(techs, tech)
	}

	return techs
}

// shouldGenerateDeepDive decide se deve gerar deep dive baseado na classificação
func shouldGenerateDeepDive(classification TechClassification) bool {
	if classification.Relevance == TRIVIAL {
		return false
	}
	return true
}

// getDeepDiveScope retorna o escopo recomendado para deep dive
func getDeepDiveScope(classification TechClassification) string {
	switch classification.Relevance {
	case TRIVIAL:
		return "none"
	case STANDARD:
		return "epic"
	case SPECIFIC:
		return "story"
	case CRITICAL:
		return "epic"
	default:
		return "epic"
	}
}

// ClassifyTechWithInfra enhances a base classification with real infrastructure data.
// - Tech running + unhealthy/degraded → boost to CRITICAL + "[INFRA: health-critical]"
// - Tech running + healthy → enrich reason with "[INFRA: running on N nodes]"
// - infraCtx nil → returns base classification unchanged
func ClassifyTechWithInfra(reg *TechRegistry, tech string, epic types.Epic, config ClassifierConfig, infraCtx *infracontext.InfraContext) TechClassification {
	base := ClassifyTechWithRegistry(reg, tech, epic, config)

	if infraCtx == nil {
		return base
	}

	// Find this tech in the infrastructure context
	healthByComp := infraCtx.HealthByComponent()
	techLower := strings.ToLower(tech)

	// Check if any topology node or health check matches this tech
	var matchedHealth *infracontext.HealthCheck
	runningCount := 0

	for _, node := range infraCtx.Topology {
		nodeName := strings.ToLower(node.Name)
		if strings.Contains(nodeName, techLower) {
			runningCount++
			if h, ok := healthByComp[node.Name]; ok {
				matchedHealth = &h
			}
		}
		// Also check container images
		for _, c := range node.Containers {
			imgLower := strings.ToLower(c.Image)
			if strings.Contains(imgLower, techLower) {
				runningCount++
			}
		}
	}

	// Also check health checks directly
	for _, h := range infraCtx.Health {
		compLower := strings.ToLower(h.Component)
		if strings.Contains(compLower, techLower) {
			matchedHealth = &h
		}
	}

	if matchedHealth != nil {
		switch matchedHealth.Status {
		case infracontext.HealthStatusUnhealthy, infracontext.HealthStatusDegraded:
			base.Relevance = CRITICAL
			base.Reason = fmt.Sprintf("%s [INFRA: health-%s]", base.Reason, matchedHealth.Status)
		case infracontext.HealthStatusHealthy:
			if runningCount > 0 {
				base.Reason = fmt.Sprintf("%s [INFRA: running on %d nodes]", base.Reason, runningCount)
			}
		}
	} else if runningCount > 0 {
		base.Reason = fmt.Sprintf("%s [INFRA: detected in %d topology nodes]", base.Reason, runningCount)
	}

	return base
}
