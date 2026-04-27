package techref

// ConfidenceScore representa score de confiança com razão
type ConfidenceScore struct {
	Score  float64
	Reason string
}

// Deprecated: Use reg.MinConfidence(), reg.Confidence.Thresholds.High, etc.
// These constants may diverge from YAML config values.
const (
	MinConfidenceThreshold      = 0.60 // Deprecated: use reg.MinConfidence()
	HighConfidenceThreshold     = 0.85 // Deprecated: use reg.Confidence.Thresholds.High
	VeryHighConfidenceThreshold = 0.95 // Deprecated: use reg.Confidence.Thresholds.VeryHigh
)

// calculateConfidence calcula score de confiança para match (registry-based)
func calculateConfidence(match TechMatch, context ContextInfo) ConfidenceScore {
	return calculateConfidenceWithRegistry(DefaultRegistry(), match, context)
}

// calculateConfidenceWithRegistry calcula score usando registry configurável
func calculateConfidenceWithRegistry(reg *TechRegistry, match TechMatch, context ContextInfo) ConfidenceScore {
	if reg == nil {
		reg = DefaultRegistry()
	}

	baseScore := reg.LayerScore(string(match.Layer))

	// Ajustar baseado em contexto
	adjustedScore := baseScore
	reasons := []string{}

	// Penalidades
	if context.HasVerbBefore {
		adjustedScore += reg.Penalty("verb_before") // negative value
		reasons = append(reasons, "verbo antes")
	}

	if context.HasVerbAfter {
		adjustedScore += reg.Penalty("verb_after") // negative value
		reasons = append(reasons, "verbo depois")
	}

	// Bônus
	if context.IsStartOfSentence {
		adjustedScore += reg.Bonus("start_of_sentence")
		reasons = append(reasons, "início de sentença")
	}

	if context.HasTechBefore || context.HasTechAfter {
		adjustedScore += reg.Bonus("tech_nearby")
		reasons = append(reasons, "próximo a tech conhecida")
	}

	// Garantir limites [0.0, 1.0]
	if adjustedScore > 1.0 {
		adjustedScore = 1.0
	}
	if adjustedScore < 0.0 {
		adjustedScore = 0.0
	}

	// Montar razão
	reason := getLayerReason(match.Layer)
	if len(reasons) > 0 {
		reason += " (ajustado: "
		for i, r := range reasons {
			if i > 0 {
				reason += ", "
			}
			reason += r
		}
		reason += ")"
	}

	return ConfidenceScore{
		Score:  adjustedScore,
		Reason: reason,
	}
}

// getBaseScore retorna score base por camada (backward compat)
func getBaseScore(layer ExtractionLayer) float64 {
	return DefaultRegistry().LayerScore(string(layer))
}

// getLayerReason retorna razão textual por camada
func getLayerReason(layer ExtractionLayer) string {
	switch layer {
	case LayerKnown:
		return "tecnologia conhecida"
	case LayerAcronym:
		return "sigla identificada"
	case LayerCompound:
		return "composto validado"
	case LayerIsolated:
		return "palavra isolada"
	default:
		return "desconhecido"
	}
}

// filterByConfidenceWithScores filtra e retorna com scores
func filterByConfidenceWithScores(matches []TechMatch, threshold float64) []TechMatch {
	filtered := []TechMatch{}

	for _, match := range matches {
		context := ContextInfo{}
		score := calculateConfidence(match, context)

		if score.Score >= threshold {
			match.Confidence = score.Score
			match.Context = score.Reason
			filtered = append(filtered, match)
		}
	}

	return filtered
}
