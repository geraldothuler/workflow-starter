package extractor

import (
	"fmt"
	"math"
	"strings"
)

// ConfidenceScorer calcula scores de confiança para extrações
type ConfidenceScorer struct {
	transcript   string
	extracted    *ExtractedData
	goldenPath   *GoldenPath
	teamPatterns *TeamPatterns
}

// GoldenPath placeholder (usar types.GoldenPath real)
type GoldenPath struct {
	Patterns map[string]Pattern
}

// TeamPatterns placeholder (usar types.TeamPatterns real)
type TeamPatterns struct {
	Patterns map[string]Pattern
}

// Pattern placeholder
type Pattern struct {
	Name string
}

// NewConfidenceScorer cria novo scorer
func NewConfidenceScorer(transcript string, extracted *ExtractedData) *ConfidenceScorer {
	return &ConfidenceScorer{
		transcript: transcript,
		extracted:  extracted,
	}
}

// Score calcula scores refinados
func (cs *ConfidenceScorer) Score() *ScoringResult {
	result := &ScoringResult{
		OverallScore:    0.0,
		SectionScores:   make(map[string]SectionScore),
		FactChecks:      []FactCheck{},
		Recommendations: []string{},
	}

	// Calcular score por seção
	result.SectionScores["context"] = cs.scoreContext()
	result.SectionScores["problem"] = cs.scoreProblem()
	result.SectionScores["objectives"] = cs.scoreObjectives()
	result.SectionScores["volumetry"] = cs.scoreVolumetry()
	result.SectionScores["stack"] = cs.scoreStack()
	result.SectionScores["nfrs"] = cs.scoreNFRs()

	// Calcular score geral (média ponderada)
	weights := map[string]float64{
		"context":    0.15,
		"problem":    0.15,
		"objectives": 0.10,
		"volumetry":  0.20, // Mais crítico
		"stack":      0.25, // Mais crítico
		"nfrs":       0.15,
	}

	totalScore := 0.0
	for section, score := range result.SectionScores {
		weight := weights[section]
		totalScore += score.Score * weight
	}
	result.OverallScore = totalScore

	// Fact checks
	result.FactChecks = cs.performFactChecks()

	// Recomendações
	result.Recommendations = cs.generateRecommendations(result)

	return result
}

// ScoringResult resultado do scoring
type ScoringResult struct {
	OverallScore    float64                  `json:"overall_score"`
	SectionScores   map[string]SectionScore  `json:"section_scores"`
	FactChecks      []FactCheck              `json:"fact_checks"`
	Recommendations []string                 `json:"recommendations"`
}

// SectionScore score de uma seção
type SectionScore struct {
	Score      float64  `json:"score"`       // 0.0-1.0
	Level      string   `json:"level"`       // "high", "medium", "low"
	Reasoning  string   `json:"reasoning"`   // Por que este score
	Issues     []string `json:"issues"`      // Problemas detectados
	Strengths  []string `json:"strengths"`   // Pontos fortes
}

// FactCheck verificação de fato
type FactCheck struct {
	Type       string  `json:"type"`        // "consistency", "completeness", "realism"
	Passed     bool    `json:"passed"`
	Issue      string  `json:"issue,omitempty"`
	Suggestion string  `json:"suggestion,omitempty"`
}

// scoreContext avalia contexto
func (cs *ConfidenceScorer) scoreContext() SectionScore {
	context := cs.extracted.Context
	score := SectionScore{
		Issues:    []string{},
		Strengths: []string{},
	}

	baseScore := 0.5

	// Checks positivos
	if len(context) > 200 {
		baseScore += 0.15
		score.Strengths = append(score.Strengths, "Contexto detalhado (>200 chars)")
	}

	if strings.Contains(strings.ToLower(context), "objetivo") || 
	   strings.Contains(strings.ToLower(context), "meta") {
		baseScore += 0.10
		score.Strengths = append(score.Strengths, "Menciona objetivos")
	}

	if containsAny(context, []string{"usuário", "cliente", "negócio"}) {
		baseScore += 0.10
		score.Strengths = append(score.Strengths, "Menciona stakeholders")
	}

	// Checks negativos
	if len(context) < 100 {
		baseScore -= 0.20
		score.Issues = append(score.Issues, "Contexto muito curto (<100 chars)")
	}

	if !containsAny(context, []string{"sistema", "plataforma", "aplicação", "projeto"}) {
		baseScore -= 0.10
		score.Issues = append(score.Issues, "Não menciona tipo de sistema")
	}

	score.Score = clamp(baseScore, 0.0, 1.0)
	score.Level = scoreLevel(score.Score)
	score.Reasoning = fmt.Sprintf("Contexto com %d caracteres, %d pontos fortes, %d issues", 
		len(context), len(score.Strengths), len(score.Issues))

	return score
}

// scoreProblem avalia problema
func (cs *ConfidenceScorer) scoreProblem() SectionScore {
	problem := cs.extracted.Problem
	score := SectionScore{
		Issues:    []string{},
		Strengths: []string{},
	}

	baseScore := 0.5

	// Checks positivos
	if len(problem) > 100 {
		baseScore += 0.20
		score.Strengths = append(score.Strengths, "Problema bem descrito")
	}

	if containsAny(problem, []string{"atual", "hoje", "atualmente"}) {
		baseScore += 0.10
		score.Strengths = append(score.Strengths, "Descreve situação atual")
	}

	if containsAny(problem, []string{"dor", "problema", "dificuldade", "desafio"}) {
		baseScore += 0.10
		score.Strengths = append(score.Strengths, "Identifica dor específica")
	}

	// Checks negativos
	if len(problem) < 50 {
		baseScore -= 0.20
		score.Issues = append(score.Issues, "Problema muito vago (<50 chars)")
	}

	score.Score = clamp(baseScore, 0.0, 1.0)
	score.Level = scoreLevel(score.Score)
	score.Reasoning = fmt.Sprintf("Problema com %d caracteres", len(problem))

	return score
}

// scoreObjectives avalia objetivos
func (cs *ConfidenceScorer) scoreObjectives() SectionScore {
	objectives := cs.extracted.Objectives
	score := SectionScore{
		Issues:    []string{},
		Strengths: []string{},
	}

	baseScore := 0.5

	// Checks positivos
	if len(objectives) >= 3 {
		baseScore += 0.20
		score.Strengths = append(score.Strengths, fmt.Sprintf("%d objetivos definidos", len(objectives)))
	}

	measurableCount := 0
	for _, obj := range objectives {
		if containsNumbers(obj) {
			measurableCount++
		}
	}

	if measurableCount > 0 {
		baseScore += 0.20
		score.Strengths = append(score.Strengths, fmt.Sprintf("%d objetivos mensuráveis", measurableCount))
	}

	// Checks negativos
	if len(objectives) == 0 {
		baseScore -= 0.30
		score.Issues = append(score.Issues, "Nenhum objetivo definido")
	}

	score.Score = clamp(baseScore, 0.0, 1.0)
	score.Level = scoreLevel(score.Score)
	score.Reasoning = fmt.Sprintf("%d objetivos, %d mensuráveis", len(objectives), measurableCount)

	return score
}

// scoreVolumetry avalia volumetria
func (cs *ConfidenceScorer) scoreVolumetry() SectionScore {
	volumetry := cs.extracted.Volumetry
	score := SectionScore{
		Issues:    []string{},
		Strengths: []string{},
	}

	baseScore := 0.3 // Começa baixo (volumetria é crítica)

	explicitCount := 0
	inferredCount := 0

	for _, value := range volumetry {
		if strings.Contains(value, "inferido") || strings.Contains(value, "estimado") {
			inferredCount++
		} else {
			explicitCount++
		}
	}

	// Checks positivos
	if explicitCount >= 2 {
		baseScore += 0.30
		score.Strengths = append(score.Strengths, fmt.Sprintf("%d métricas explícitas", explicitCount))
	}

	if hasKey(volumetry, []string{"users", "devices", "transactions"}) {
		baseScore += 0.20
		score.Strengths = append(score.Strengths, "Métricas chave presentes")
	}

	// Checks negativos
	if len(volumetry) == 0 {
		baseScore -= 0.20
		score.Issues = append(score.Issues, "Nenhuma métrica de volumetria")
	}

	if inferredCount > explicitCount {
		baseScore -= 0.15
		score.Issues = append(score.Issues, fmt.Sprintf("Mais métricas inferidas (%d) que explícitas (%d)", 
			inferredCount, explicitCount))
	}

	score.Score = clamp(baseScore, 0.0, 1.0)
	score.Level = scoreLevel(score.Score)
	score.Reasoning = fmt.Sprintf("%d explícitas, %d inferidas", explicitCount, inferredCount)

	return score
}

// scoreStack avalia stack técnico
func (cs *ConfidenceScorer) scoreStack() SectionScore {
	stack := cs.extracted.Stack
	score := SectionScore{
		Issues:    []string{},
		Strengths: []string{},
	}

	baseScore := 0.4

	explicitCount := 0
	inferredCount := 0
	highConfCount := 0

	for _, tech := range stack {
		if tech.Source == "explicit" {
			explicitCount++
		} else {
			inferredCount++
		}

		if tech.Confidence >= 0.8 {
			highConfCount++
		}
	}

	// Checks positivos
	if explicitCount >= 3 {
		baseScore += 0.25
		score.Strengths = append(score.Strengths, fmt.Sprintf("%d tecnologias mencionadas", explicitCount))
	}

	if highConfCount >= 5 {
		baseScore += 0.20
		score.Strengths = append(score.Strengths, fmt.Sprintf("%d tecnologias alta confiança", highConfCount))
	}

	// Check de stack completo (backend, database, observability)
	categories := cs.categorizeStack(stack)
	if len(categories) >= 3 {
		baseScore += 0.15
		score.Strengths = append(score.Strengths, "Stack completo (múltiplas categorias)")
	}

	// Checks negativos
	if len(stack) == 0 {
		baseScore -= 0.30
		score.Issues = append(score.Issues, "Nenhuma tecnologia definida")
	}

	if inferredCount > explicitCount*2 {
		baseScore -= 0.10
		score.Issues = append(score.Issues, "Muitas tecnologias inferidas")
	}

	score.Score = clamp(baseScore, 0.0, 1.0)
	score.Level = scoreLevel(score.Score)
	score.Reasoning = fmt.Sprintf("%d tecnologias (%d explícitas, %d inferidas)", 
		len(stack), explicitCount, inferredCount)

	return score
}

// scoreNFRs avalia NFRs
func (cs *ConfidenceScorer) scoreNFRs() SectionScore {
	nfrs := cs.extracted.NFRs
	score := SectionScore{
		Issues:    []string{},
		Strengths: []string{},
	}

	baseScore := 0.4

	// Checks positivos
	if len(nfrs) >= 3 {
		baseScore += 0.25
		score.Strengths = append(score.Strengths, fmt.Sprintf("%d NFRs definidos", len(nfrs)))
	}

	// Categorias importantes
	hasPerformance := false
	hasAvailability := false
	hasSecurity := false

	for _, nfr := range nfrs {
		nfrLower := strings.ToLower(nfr)
		if containsAny(nfrLower, []string{"latência", "latency", "throughput", "performance"}) {
			hasPerformance = true
		}
		if containsAny(nfrLower, []string{"uptime", "disponibilidade", "sla"}) {
			hasAvailability = true
		}
		if containsAny(nfrLower, []string{"segurança", "security", "compliance"}) {
			hasSecurity = true
		}
	}

	categoryCount := 0
	if hasPerformance {
		categoryCount++
		score.Strengths = append(score.Strengths, "Requisitos de performance")
	}
	if hasAvailability {
		categoryCount++
		score.Strengths = append(score.Strengths, "Requisitos de disponibilidade")
	}
	if hasSecurity {
		categoryCount++
		score.Strengths = append(score.Strengths, "Requisitos de segurança")
	}

	if categoryCount >= 2 {
		baseScore += 0.20
	}

	// Checks negativos
	if len(nfrs) == 0 {
		baseScore -= 0.25
		score.Issues = append(score.Issues, "Nenhum NFR definido")
	}

	score.Score = clamp(baseScore, 0.0, 1.0)
	score.Level = scoreLevel(score.Score)
	score.Reasoning = fmt.Sprintf("%d NFRs, %d categorias", len(nfrs), categoryCount)

	return score
}

// performFactChecks verifica consistência
func (cs *ConfidenceScorer) performFactChecks() []FactCheck {
	checks := []FactCheck{}

	// Check 1: Volumetria vs Stack
	if len(cs.extracted.Volumetry) > 0 && len(cs.extracted.Stack) == 0 {
		checks = append(checks, FactCheck{
			Type:       "completeness",
			Passed:     false,
			Issue:      "Volumetria definida mas stack técnico ausente",
			Suggestion: "Definir tecnologias para suportar volumetria mencionada",
		})
	}

	// Check 2: NFRs vs Stack
	hasPerformanceNFR := false
	for _, nfr := range cs.extracted.NFRs {
		if containsAny(strings.ToLower(nfr), []string{"latência", "throughput"}) {
			hasPerformanceNFR = true
			break
		}
	}

	if hasPerformanceNFR && len(cs.extracted.Stack) == 0 {
		checks = append(checks, FactCheck{
			Type:       "consistency",
			Passed:     false,
			Issue:      "NFR de performance sem stack técnico definido",
			Suggestion: "Especificar tecnologias para atender requisitos de performance",
		})
	}

	// Check 3: Números realistas
	for key, value := range cs.extracted.Volumetry {
		if strings.Contains(value, "1000000000") { // 1 bilhão
			checks = append(checks, FactCheck{
				Type:       "realism",
				Passed:     false,
				Issue:      fmt.Sprintf("Volumetria %s parece irrealista: %s", key, value),
				Suggestion: "Verificar se ordem de grandeza está correta",
			})
		}
	}

	// Se nenhum problema, marcar como OK
	if len(checks) == 0 {
		checks = append(checks, FactCheck{
			Type:   "consistency",
			Passed: true,
		})
	}

	return checks
}

// generateRecommendations gera recomendações
func (cs *ConfidenceScorer) generateRecommendations(result *ScoringResult) []string {
	recs := []string{}

	// Baseado em scores baixos
	for section, score := range result.SectionScores {
		if score.Score < 0.6 {
			recs = append(recs, fmt.Sprintf("⚠️  %s: score baixo (%.0f%%) - revisar e expandir", 
				section, score.Score*100))
		}
	}

	// Baseado em fact checks
	for _, check := range result.FactChecks {
		if !check.Passed && check.Suggestion != "" {
			recs = append(recs, fmt.Sprintf("💡 %s", check.Suggestion))
		}
	}

	// Recomendações gerais
	if result.OverallScore < 0.7 {
		recs = append(recs, "📋 Confiança geral < 70% - recomendado validar com stakeholders antes de prosseguir")
	}

	return recs
}

// Helper functions

func containsAny(text string, keywords []string) bool {
	textLower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(textLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func containsNumbers(text string) bool {
	for _, char := range text {
		if char >= '0' && char <= '9' {
			return true
		}
	}
	return false
}

func hasKey(m map[string]string, keys []string) bool {
	for _, key := range keys {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func clamp(value, min, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}

func scoreLevel(score float64) string {
	if score >= 0.8 {
		return "high"
	} else if score >= 0.6 {
		return "medium"
	}
	return "low"
}

func (cs *ConfidenceScorer) categorizeStack(stack []TechMention) map[string]bool {
	categories := make(map[string]bool)

	for _, tech := range stack {
		techLower := strings.ToLower(tech.Name)
		
		// Backend
		if containsAny(techLower, []string{"spring", "node", "django", "flask", "rails"}) {
			categories["backend"] = true
		}
		
		// Database
		if containsAny(techLower, []string{"postgres", "mysql", "mongodb", "scylla", "cassandra", "redis"}) {
			categories["database"] = true
		}
		
		// Streaming
		if containsAny(techLower, []string{"kafka", "flink", "spark"}) {
			categories["streaming"] = true
		}
		
		// Observability
		if containsAny(techLower, []string{"prometheus", "grafana", "jaeger", "datadog"}) {
			categories["observability"] = true
		}
		
		// Cloud
		if containsAny(techLower, []string{"aws", "azure", "gcp", "kubernetes"}) {
			categories["cloud"] = true
		}
	}

	return categories
}
