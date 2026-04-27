package techref

import (
	"regexp"
	"strings"
)

// Pre-compiled regex patterns (compiled once at package init, not per-call)
var (
	acronymPattern  = regexp.MustCompile(`\b[A-Z]{2,5}\b`)
	compoundPattern = regexp.MustCompile(`\b[A-Z][a-z]{2,}\s+[A-Z][a-z]{2,}\b`)
	isolatedPattern = regexp.MustCompile(`\b[A-Z][a-z]{3,}\b`)
)

// ExtractionLayer representa o tipo de camada de extração
type ExtractionLayer string

const (
	LayerKnown    ExtractionLayer = "known"    // Lista conhecida
	LayerAcronym  ExtractionLayer = "acronym"  // Siglas
	LayerCompound ExtractionLayer = "compound" // Palavras compostas
	LayerIsolated ExtractionLayer = "isolated" // Palavras isoladas
)

// TechMatch representa uma tecnologia extraída com metadados
type TechMatch struct {
	Term       string
	Layer      ExtractionLayer
	Confidence float64
	Context    string
	Position   int // Posição no texto original
}

// charRange represents a character range in text for overlap tracking
type charRange struct{ start, end int }

// ─────────────────────────────────────────────────────────────────
// CAMADA 1: Lista Conhecida (Alta Confiança: 0.95-1.0)
// ─────────────────────────────────────────────────────────────────

// extractKnownTechnologies backward compat wrapper
func extractKnownTechnologies(text string) []TechMatch {
	return extractKnownTechnologiesWithRegistry(DefaultRegistry(), text)
}

// extractKnownTechnologiesWithRegistry uses registry + word-boundary matching.
// Techs are pre-sorted by name length DESC so "Spring Boot" is matched
// before "Spring", preventing substring overlap.
func extractKnownTechnologiesWithRegistry(reg *TechRegistry, text string) []TechMatch {
	matches := []TechMatch{}
	textLower := strings.ToLower(text)

	// Track covered character ranges to avoid substring overlaps
	covered := make([]charRange, 0)

	// Registry returns techs sorted by name length DESC
	for _, tech := range reg.KnownTechsSorted() {
		// Search for tech name AND its aliases — first match wins
		searchTerms := append([]string{tech.Name}, tech.Aliases...)

		matched := false
		for _, term := range searchTerms {
			if matched {
				break
			}
			termLower := strings.ToLower(term)

			searchFrom := 0
			for searchFrom < len(textLower) {
				pos := strings.Index(textLower[searchFrom:], termLower)
				if pos == -1 {
					break
				}
				absolutePos := searchFrom + pos
				endPos := absolutePos + len(termLower)

				// Short names (≤3 chars) require word boundary check
				// to avoid matching "Go" inside "going", "cargo", "google", etc.
				if len(termLower) <= 3 {
					if absolutePos > 0 && isWordChar(textLower[absolutePos-1]) {
						searchFrom = absolutePos + 1
						continue
					}
					if endPos < len(textLower) && isWordChar(textLower[endPos]) {
						searchFrom = absolutePos + 1
						continue
					}
					// Context check: if followed by a common verb, it's likely
					// imperative usage ("Go implement..."), not a tech reference
					if isFollowedByVerb(textLower, endPos) {
						searchFrom = absolutePos + 1
						continue
					}
				}

				// Check if this position is already covered by a longer match
				if !isPositionCovered(absolutePos, endPos, covered) {
					covered = append(covered, charRange{absolutePos, endPos})
					matches = append(matches, TechMatch{
						Term:       tech.Name, // Always return canonical name
						Layer:      LayerKnown,
						Confidence: reg.LayerScore("known"),
						Position:   absolutePos,
					})
					matched = true
					break // Only match first occurrence per tech
				}

				searchFrom = absolutePos + 1
			}
		}
	}

	return matches
}

// isPositionCovered checks if a range overlaps with any covered range
func isPositionCovered(start, end int, covered []charRange) bool {
	for _, r := range covered {
		// Check overlap: ranges overlap if one starts before the other ends
		if start < r.end && end > r.start {
			return true
		}
	}
	return false
}

// isWordChar returns true if the byte is a letter or digit (word constituent).
// Used for word boundary checks on short tech names.
func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// isFollowedByVerb checks if the position in text is followed by a common English verb.
// Used to distinguish "Go implement..." (verb) from "com Go e PostgreSQL" (tech).
func isFollowedByVerb(text string, pos int) bool {
	// Skip whitespace
	i := pos
	for i < len(text) && text[i] == ' ' {
		i++
	}
	if i >= len(text) {
		return false
	}

	// Extract next word
	end := i
	for end < len(text) && end-i < 20 && isWordChar(text[end]) {
		end++
	}
	nextWord := text[i:end]

	// Common English verbs that indicate imperative usage
	imperativeVerbs := map[string]bool{
		"implement": true, "create": true, "build": true, "make": true,
		"deploy": true, "install": true, "configure": true, "setup": true,
		"run": true, "start": true, "stop": true, "check": true,
		"update": true, "fix": true, "add": true, "remove": true,
		"delete": true, "get": true, "set": true, "put": true,
		"fetch": true, "push": true, "pull": true, "test": true,
		"write": true, "read": true, "send": true, "receive": true,
		"ahead": true, "back": true, "through": true, "over": true,
		"to": true, "and": true, "for": true,
	}

	return imperativeVerbs[nextWord]
}

// ─────────────────────────────────────────────────────────────────
// CAMADA 2: Siglas (Média-Alta Confiança: 0.80-0.90)
// ─────────────────────────────────────────────────────────────────

// extractAcronyms backward compat wrapper
func extractAcronyms(text string) []TechMatch {
	return extractAcronymsWithRegistry(DefaultRegistry(), text)
}

// extractAcronymsWithRegistry uses registry for blacklist and scoring
func extractAcronymsWithRegistry(reg *TechRegistry, text string) []TechMatch {
	matches := []TechMatch{}

	acronyms := acronymPattern.FindAllString(text, -1)

	for _, acronym := range acronyms {
		if reg.IsBlacklistedAcronym(acronym) {
			continue
		}

		pos := strings.Index(text, acronym)

		matches = append(matches, TechMatch{
			Term:       acronym,
			Layer:      LayerAcronym,
			Confidence: reg.LayerScore("acronym"),
			Position:   pos,
		})
	}

	return matches
}

// ─────────────────────────────────────────────────────────────────
// CAMADA 3: Compostos Validados (Média Confiança: 0.60-0.80)
// ─────────────────────────────────────────────────────────────────

func extractValidCompounds(text string) []TechMatch {
	return extractValidCompoundsWithRegistry(DefaultRegistry(), text)
}

func extractValidCompoundsWithRegistry(reg *TechRegistry, text string) []TechMatch {
	matches := []TechMatch{}

	compounds := compoundPattern.FindAllString(text, -1)

	for _, compound := range compounds {
		pos := strings.Index(text, compound)

		matches = append(matches, TechMatch{
			Term:       compound,
			Layer:      LayerCompound,
			Confidence: reg.LayerScore("compound"),
			Position:   pos,
		})
	}

	return matches
}

// ─────────────────────────────────────────────────────────────────
// CAMADA 4: Isoladas Contextuais (Baixa Confiança: 0.40-0.70)
// ─────────────────────────────────────────────────────────────────

func extractIsolatedWithContext(text string) []TechMatch {
	return extractIsolatedWithContextRegistry(DefaultRegistry(), text)
}

func extractIsolatedWithContextRegistry(reg *TechRegistry, text string) []TechMatch {
	matches := []TechMatch{}

	words := isolatedPattern.FindAllString(text, -1)

	for _, word := range words {
		if reg.IsCommonWord(word) {
			continue
		}

		pos := strings.Index(text, word)

		matches = append(matches, TechMatch{
			Term:       word,
			Layer:      LayerIsolated,
			Confidence: reg.LayerScore("isolated"),
			Position:   pos,
		})
	}

	return matches
}

// isCommonWordHelper kept for backward compat
func isCommonWordHelper(word string) bool {
	return DefaultRegistry().IsCommonWord(word)
}

// ─────────────────────────────────────────────────────────────────
// MERGE E DEDUPLICAÇÃO
// ─────────────────────────────────────────────────────────────────

// mergeLayers combina resultados de todas as camadas
func mergeLayers(layers ...[]TechMatch) []TechMatch {
	all := []TechMatch{}

	for _, layer := range layers {
		all = append(all, layer...)
	}

	seen := make(map[string]TechMatch)

	for _, match := range all {
		if existing, exists := seen[match.Term]; exists {
			if match.Confidence > existing.Confidence {
				seen[match.Term] = match
			}
		} else {
			seen[match.Term] = match
		}
	}

	result := []TechMatch{}
	for _, match := range seen {
		result = append(result, match)
	}

	return result
}

// ─────────────────────────────────────────────────────────────────
// FILTROS
// ─────────────────────────────────────────────────────────────────

// filterByConfidence filtra por threshold de confiança
func filterByConfidence(matches []TechMatch, threshold float64) []TechMatch {
	filtered := []TechMatch{}

	for _, match := range matches {
		if match.Confidence >= threshold {
			filtered = append(filtered, match)
		}
	}

	return filtered
}

// convertToStrings converte TechMatch para lista de strings
func convertToStrings(matches []TechMatch) []string {
	result := []string{}

	for _, match := range matches {
		result = append(result, match.Term)
	}

	return result
}
