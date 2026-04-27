package techref

import (
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// TechExtraction representa uma tecnologia extraída com metadados
type TechExtraction struct {
	Term     string
	StoryIDs []string // IDs das histórias que mencionam essa tech
	Count    int      // Número de histórias que usam
	Contexts []string // Contextos onde aparece (para análise)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FUNÇÕES PÚBLICAS - API Principal
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ExtractTechsByEpic extrai todas as tecnologias de um épico (backward compat)
func ExtractTechsByEpic(epic types.Epic) []TechExtraction {
	return ExtractTechsByEpicWithRegistry(DefaultRegistry(), epic)
}

// ExtractTechsByEpicWithRegistry extrai techs usando registry configurável
func ExtractTechsByEpicWithRegistry(reg *TechRegistry, epic types.Epic) []TechExtraction {
	if reg == nil {
		reg = DefaultRegistry()
	}

	techMap := make(map[string]*TechExtraction)

	for _, story := range epic.Stories {
		techs := ExtractTechsFromStoryWithRegistry(reg, story)

		for _, tech := range techs {
			normalized := reg.NormalizeToCanonical(tech)

			if _, exists := techMap[normalized]; !exists {
				techMap[normalized] = &TechExtraction{
					Term:     normalized,
					StoryIDs: []string{},
					Contexts: []string{},
				}
			}

			if !contains(techMap[normalized].StoryIDs, story.ID) {
				techMap[normalized].StoryIDs = append(techMap[normalized].StoryIDs, story.ID)
				techMap[normalized].Count++
			}

			context := extractContext(story, tech)
			if context != "" {
				techMap[normalized].Contexts = append(techMap[normalized].Contexts, context)
			}
		}
	}

	extractions := make([]TechExtraction, 0, len(techMap))
	for _, extraction := range techMap {
		extractions = append(extractions, *extraction)
	}

	return extractions
}

// ExtractTechsFromStory extrai tecnologias de uma única história (backward compat)
func ExtractTechsFromStory(story types.Story) []string {
	return ExtractTechsFromStoryWithRegistry(DefaultRegistry(), story)
}

// ExtractTechsFromStoryWithRegistry extrai techs usando registry configurável
func ExtractTechsFromStoryWithRegistry(reg *TechRegistry, story types.Story) []string {
	if reg == nil {
		reg = DefaultRegistry()
	}

	fullText := story.Title + " " +
		story.What + " " +
		story.Why

	for _, criterion := range story.AcceptanceCriteria {
		fullText += " " + criterion
	}

	// CAMADA 1: Lista Conhecida (Prioridade Máxima)
	layer1 := extractKnownTechnologiesWithRegistry(reg, fullText)

	// CAMADA 2: Siglas (JWT, AWS, GCP...)
	layer2 := extractAcronymsWithRegistry(reg, fullText)

	// CAMADA 3: Compostos Validados (Spring Boot, React Native...)
	layer3 := extractValidCompoundsWithRegistry(reg, fullText)

	layer3Valid := []TechMatch{}
	for _, match := range layer3 {
		if validateCompoundStrictWithRegistry(reg, match.Term) {
			layer3Valid = append(layer3Valid, match)
		}
	}

	// CAMADA 4: Isoladas Contextuais
	layer4 := extractIsolatedWithContextRegistry(reg, fullText)

	layer4Valid := []TechMatch{}
	for _, match := range layer4 {
		context := analyzeContextWithRegistry(reg, fullText, match.Term, match.Position)
		if isValidIsolatedWithRegistry(reg, match.Term, context) {
			score := calculateConfidenceWithRegistry(reg, match, context)
			match.Confidence = score.Score
			match.Context = score.Reason
			layer4Valid = append(layer4Valid, match)
		}
	}

	// MERGE E NORMALIZAÇÃO
	all := mergeLayers(layer1, layer2, layer3Valid, layer4Valid)

	normalized := deduplicateByCanonicalWithRegistry(reg, all)
	dedupSubstr := deduplicateSubstrings(normalized)
	filtered := filterByConfidence(dedupSubstr, reg.MinConfidence())

	return convertToStrings(filtered)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FUNÇÕES AUXILIARES
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// FilterSingleStoryTechs filtra tecnologias usadas em apenas 1 história
func FilterSingleStoryTechs(extractions []TechExtraction) []TechExtraction {
	filtered := []TechExtraction{}

	for _, extraction := range extractions {
		if extraction.Count == 1 {
			filtered = append(filtered, extraction)
		}
	}

	return filtered
}

// FilterMultiStoryTechs filtra tecnologias usadas em 2+ histórias
func FilterMultiStoryTechs(extractions []TechExtraction) []TechExtraction {
	filtered := []TechExtraction{}

	for _, extraction := range extractions {
		if extraction.Count >= 2 {
			filtered = append(filtered, extraction)
		}
	}

	return filtered
}

// GroupTechsByCount agrupa tecnologias por contagem
func GroupTechsByCount(extractions []TechExtraction) map[int][]TechExtraction {
	groups := make(map[int][]TechExtraction)

	for _, extraction := range extractions {
		count := extraction.Count
		groups[count] = append(groups[count], extraction)
	}

	return groups
}

// GetExtractionStatistics retorna estatísticas de extração
func GetExtractionStatistics(extractions []TechExtraction) map[string]int {
	stats := map[string]int{
		"total":        len(extractions),
		"single_story": 0,
		"multi_story":  0,
	}

	for _, extraction := range extractions {
		if extraction.Count == 1 {
			stats["single_story"]++
		} else {
			stats["multi_story"]++
		}
	}

	return stats
}

// extractContext extrai contexto onde tech aparece
func extractContext(story types.Story, tech string) string {
	techLower := strings.ToLower(tech)

	if strings.Contains(strings.ToLower(story.Title), techLower) {
		return "title: " + story.Title
	}

	if strings.Contains(strings.ToLower(story.What), techLower) {
		return "what: " + extractSentence(story.What, tech)
	}

	if strings.Contains(strings.ToLower(story.Why), techLower) {
		return "why: " + extractSentence(story.Why, tech)
	}

	for _, criterion := range story.AcceptanceCriteria {
		if strings.Contains(strings.ToLower(criterion), techLower) {
			return "criteria: " + criterion
		}
	}

	return ""
}

// extractSentence extrai sentença contendo termo (UTF-8 safe)
func extractSentence(text string, term string) string {
	termLower := strings.ToLower(term)
	textLower := strings.ToLower(text)

	pos := strings.Index(textLower, termLower)
	if pos == -1 {
		return text
	}

	// Convert to runes for character-aware slicing (handles accented PT-BR chars)
	runes := []rune(text)
	// Find rune position from byte position
	runePos := len([]rune(text[:pos]))
	runeTermLen := len([]rune(term))

	start := runePos - 50
	if start < 0 {
		start = 0
	}

	end := runePos + runeTermLen + 50
	if end > len(runes) {
		end = len(runes)
	}

	return "..." + string(runes[start:end]) + "..."
}

// contains verifica se slice contém string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// normalizeTech normaliza nome de tecnologia (DEPRECATED - usar NormalizeToCanonical)
func normalizeTech(tech string) string {
	return normalizeToCanonical(tech)
}
