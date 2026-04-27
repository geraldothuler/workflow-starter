package techref

import (
	"strings"
)

// canonicalForms kept for backward compat — delegates to registry
var canonicalForms map[string]string

func init() {
	// Load from registry. This is lazy — if default registry fails,
	// canonicalForms will be nil and normalizeToCanonical falls through.
	reg := DefaultRegistry()
	if reg != nil {
		canonicalForms = reg.CanonicalForms
	}
}

// normalizeToCanonical converte termo para forma canônica (backward compat)
func normalizeToCanonical(tech string) string {
	lower := strings.ToLower(strings.TrimSpace(tech))
	if canonical, exists := canonicalForms[lower]; exists {
		return canonical
	}
	return tech
}

// normalizeToCanonicalWithRegistry converte usando registry configurável
func normalizeToCanonicalWithRegistry(reg *TechRegistry, tech string) string {
	return reg.NormalizeToCanonical(tech)
}

// deduplicateByCanonical remove duplicatas usando forma canônica (backward compat)
func deduplicateByCanonical(matches []TechMatch) []TechMatch {
	return deduplicateByCanonicalWithRegistry(DefaultRegistry(), matches)
}

// deduplicateByCanonicalWithRegistry remove duplicatas usando registry
func deduplicateByCanonicalWithRegistry(reg *TechRegistry, matches []TechMatch) []TechMatch {
	seen := make(map[string]TechMatch)

	for _, match := range matches {
		canonical := reg.NormalizeToCanonical(match.Term)
		match.Term = canonical

		if existing, exists := seen[canonical]; exists {
			if match.Confidence > existing.Confidence {
				seen[canonical] = match
			}
			if match.Confidence == existing.Confidence &&
				getLayerPriority(match.Layer) > getLayerPriority(existing.Layer) {
				seen[canonical] = match
			}
		} else {
			seen[canonical] = match
		}
	}

	result := []TechMatch{}
	for _, match := range seen {
		result = append(result, match)
	}
	return result
}

// deduplicateSubstrings remove termos que são substrings de outros termos mais específicos.
// Ex: "Spring" é removido quando "Spring Boot" existe; "React" removido quando "React Native" existe.
// Mantém sempre o termo mais longo (mais específico).
func deduplicateSubstrings(matches []TechMatch) []TechMatch {
	if len(matches) <= 1 {
		return matches
	}

	toRemove := make(map[int]bool)

	for i := 0; i < len(matches); i++ {
		if toRemove[i] {
			continue
		}
		for j := 0; j < len(matches); j++ {
			if i == j || toRemove[j] {
				continue
			}

			termI := strings.ToLower(matches[i].Term)
			termJ := strings.ToLower(matches[j].Term)

			if termI != termJ && strings.Contains(termJ, termI) {
				toRemove[i] = true
				break
			}
		}
	}

	result := make([]TechMatch, 0, len(matches)-len(toRemove))
	for i, match := range matches {
		if !toRemove[i] {
			result = append(result, match)
		}
	}
	return result
}

// getLayerPriority retorna prioridade da camada (maior = melhor)
func getLayerPriority(layer ExtractionLayer) int {
	switch layer {
	case LayerKnown:
		return 4
	case LayerAcronym:
		return 3
	case LayerCompound:
		return 2
	case LayerIsolated:
		return 1
	default:
		return 0
	}
}
