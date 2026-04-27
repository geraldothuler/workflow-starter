package extractor

import (
	"fmt"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// InferenceEngine usa Golden Paths para inferir informações
type InferenceEngine struct {
	goldenPath   *types.GoldenPath
	teamPatterns *types.TeamPatterns
	transcript   string
	extracted    *ExtractedData
}

// NewInferenceEngine cria novo engine
func NewInferenceEngine(gp *types.GoldenPath, tp *types.TeamPatterns, transcript string, extracted *ExtractedData) *InferenceEngine {
	return &InferenceEngine{
		goldenPath:   gp,
		teamPatterns: tp,
		transcript:   transcript,
		extracted:    extracted,
	}
}

// InferenceResult resultado de inferências
type InferenceResult struct {
	InferredTechnologies []InferredTech     `json:"inferred_technologies"`
	InferredNFRs         []InferredNFR      `json:"inferred_nfrs"`
	InferredPatterns     []InferredPattern  `json:"inferred_patterns"`
	Suggestions          []Suggestion       `json:"suggestions"`
	GapAnalysis          GapAnalysis        `json:"gap_analysis"`
}

// InferredTech tecnologia inferida
type InferredTech struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
	Pattern    string  `json:"pattern"`     // GP-001, etc.
	Category   string  `json:"category"`    // backend, database, etc.
	Required   bool    `json:"required"`    // Necessária ou opcional
}

// InferredNFR NFR inferido
type InferredNFR struct {
	Requirement string  `json:"requirement"`
	Confidence  float64 `json:"confidence"`
	Rationale   string  `json:"rationale"`
	Pattern     string  `json:"pattern,omitempty"`
}

// InferredPattern pattern aplicável
type InferredPattern struct {
	PatternID   string   `json:"pattern_id"`
	PatternName string   `json:"pattern_name"`
	Relevance   string   `json:"relevance"`   // Por que é relevante
	Confidence  float64  `json:"confidence"`
	Implications []string `json:"implications"` // O que implica aplicar este pattern
}

// Suggestion sugestão baseada em patterns
type Suggestion struct {
	Type        string  `json:"type"`        // "tech", "nfr", "pattern"
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
	Source      string  `json:"source"`      // Qual GP/TP originou
}

// GapAnalysis análise de gaps
type GapAnalysis struct {
	MissingCategories []string `json:"missing_categories"`
	Completeness      float64  `json:"completeness"` // 0.0-1.0
	CriticalGaps      []string `json:"critical_gaps"`
	NiceToHave        []string `json:"nice_to_have"`
}

// Infer executa todas as inferências
func (ie *InferenceEngine) Infer() *InferenceResult {
	result := &InferenceResult{
		InferredTechnologies: []InferredTech{},
		InferredNFRs:         []InferredNFR{},
		InferredPatterns:     []InferredPattern{},
		Suggestions:          []Suggestion{},
	}

	// 1. Inferir tecnologias baseado em patterns
	result.InferredTechnologies = ie.inferTechnologies()

	// 2. Inferir NFRs baseado em patterns
	result.InferredNFRs = ie.inferNFRs()

	// 3. Detectar patterns aplicáveis
	result.InferredPatterns = ie.detectApplicablePatterns()

	// 4. Gerar sugestões
	result.Suggestions = ie.generateSuggestions()

	// 5. Analisar gaps
	result.GapAnalysis = ie.analyzeGaps()

	return result
}

// inferTechnologies infere tecnologias de patterns
func (ie *InferenceEngine) inferTechnologies() []InferredTech {
	inferred := []InferredTech{}

	if ie.goldenPath == nil || len(ie.goldenPath.Patterns) == 0 {
		return inferred
	}

	// Tecnologias já mencionadas
	mentionedTechs := make(map[string]bool)
	for _, tech := range ie.extracted.Stack {
		mentionedTechs[strings.ToLower(tech.Name)] = true
	}

	// Analisar cada pattern
	for patternID, pattern := range ie.goldenPath.Patterns {
		// Detectar se pattern é relevante para o projeto
		relevance := ie.calculatePatternRelevance(pattern)
		
		if relevance < 0.5 {
			continue // Pattern não relevante
		}

		// Extrair tecnologias do pattern
		techs := ie.extractTechsFromPattern(pattern)
		
		for _, tech := range techs {
			techLower := strings.ToLower(tech.Name)
			
			// Se já foi mencionada explicitamente, pular
			if mentionedTechs[techLower] {
				continue
			}

			// Inferir se necessária
			inferred = append(inferred, InferredTech{
				Name:       tech.Name,
				Confidence: relevance * tech.Confidence,
				Rationale:  fmt.Sprintf("Pattern %s (%s) sugere uso de %s", patternID, pattern.Name, tech.Name),
				Pattern:    patternID,
				Category:   tech.Category,
				Required:   tech.Required,
			})
		}
	}

	return inferred
}

// inferNFRs infere NFRs de patterns
func (ie *InferenceEngine) inferNFRs() []InferredNFR {
	inferred := []InferredNFR{}

	if ie.goldenPath == nil {
		return inferred
	}

	// NFRs comuns de Golden Paths
	commonNFRs := map[string]struct {
		requirement string
		keywords    []string
		confidence  float64
	}{
		"high-throughput": {
			requirement: "Throughput: >= 100K eventos/segundo",
			keywords:    []string{"streaming", "kafka", "flink", "eventos", "tempo real"},
			confidence:  0.8,
		},
		"low-latency": {
			requirement: "Latência P95 < 100ms",
			keywords:    []string{"tempo real", "rápido", "baixa latência", "real-time"},
			confidence:  0.7,
		},
		"high-availability": {
			requirement: "Uptime 99.9%",
			keywords:    []string{"crítico", "disponibilidade", "não pode cair", "sla"},
			confidence:  0.8,
		},
		"scalability": {
			requirement: "Escalabilidade horizontal (10x crescimento)",
			keywords:    []string{"escalar", "crescimento", "muitos usuários", "milhões"},
			confidence:  0.6,
		},
		"data-retention": {
			requirement: "Retenção de dados: 90 dias",
			keywords:    []string{"compliance", "logs", "auditoria", "regulatório"},
			confidence:  0.7,
		},
	}

	transcriptLower := strings.ToLower(ie.transcript)
	
	// Verificar quais NFRs são aplicáveis
	for nfrType, nfrData := range commonNFRs {
		matches := 0
		for _, keyword := range nfrData.keywords {
			if strings.Contains(transcriptLower, keyword) {
				matches++
			}
		}

		// Se encontrou pelo menos 2 keywords, inferir
		if matches >= 2 {
			confidence := nfrData.confidence * (float64(matches) / float64(len(nfrData.keywords)))
			
			inferred = append(inferred, InferredNFR{
				Requirement: nfrData.requirement,
				Confidence:  confidence,
				Rationale:   fmt.Sprintf("Detectadas %d keywords relacionadas a %s", matches, nfrType),
			})
		}
	}

	return inferred
}

// detectApplicablePatterns detecta quais patterns são aplicáveis
func (ie *InferenceEngine) detectApplicablePatterns() []InferredPattern {
	patterns := []InferredPattern{}

	if ie.goldenPath == nil {
		return patterns
	}

	for patternID, pattern := range ie.goldenPath.Patterns {
		relevance := ie.calculatePatternRelevance(pattern)
		
		if relevance < 0.5 {
			continue
		}

		implications := ie.extractPatternImplications(pattern)
		
		patterns = append(patterns, InferredPattern{
			PatternID:    patternID,
			PatternName:  pattern.Name,
			Relevance:    ie.explainRelevance(pattern),
			Confidence:   relevance,
			Implications: implications,
		})
	}

	return patterns
}

// generateSuggestions gera sugestões baseadas em análise
func (ie *InferenceEngine) generateSuggestions() []Suggestion {
	suggestions := []Suggestion{}

	// Sugestão 1: Se menciona streaming mas não tem observability
	hasStreaming := false
	hasObservability := false

	for _, tech := range ie.extracted.Stack {
		techLower := strings.ToLower(tech.Name)
		if strings.Contains(techLower, "kafka") || strings.Contains(techLower, "flink") {
			hasStreaming = true
		}
		if strings.Contains(techLower, "prometheus") || strings.Contains(techLower, "grafana") {
			hasObservability = true
		}
	}

	if hasStreaming && !hasObservability {
		suggestions = append(suggestions, Suggestion{
			Type:        "tech",
			Description: "Sistema de streaming detectado - considere adicionar Prometheus + Grafana para observabilidade",
			Confidence:  0.8,
			Source:      "Best practice: Streaming systems need monitoring",
		})
	}

	// Sugestão 2: Se tem database write-heavy mas não menciona replicação
	hasDatabase := false
	mentionsReplication := strings.Contains(strings.ToLower(ie.transcript), "replicação") ||
		strings.Contains(strings.ToLower(ie.transcript), "replication")

	for _, tech := range ie.extracted.Stack {
		techLower := strings.ToLower(tech.Name)
		if strings.Contains(techLower, "scylla") || strings.Contains(techLower, "cassandra") {
			hasDatabase = true
		}
	}

	if hasDatabase && !mentionsReplication {
		suggestions = append(suggestions, Suggestion{
			Type:        "nfr",
			Description: "Database distribuída detectada - definir fator de replicação (sugestão: RF=3 para HA)",
			Confidence:  0.7,
			Source:      "GP: Distributed databases require replication strategy",
		})
	}

	// Sugestão 3: Se tem volumetria alta mas não menciona auto-scaling
	hasHighVolume := false
	for key, value := range ie.extracted.Volumetry {
		if strings.Contains(key, "events") || strings.Contains(key, "transactions") {
			// Tentar extrair número
			if strings.Contains(value, "100") || strings.Contains(value, "milhão") {
				hasHighVolume = true
			}
		}
	}

	mentionsScaling := strings.Contains(strings.ToLower(ie.transcript), "auto-scaling") ||
		strings.Contains(strings.ToLower(ie.transcript), "escala automática")

	if hasHighVolume && !mentionsScaling {
		suggestions = append(suggestions, Suggestion{
			Type:        "nfr",
			Description: "Alta volumetria detectada - considere auto-scaling para lidar com picos",
			Confidence:  0.75,
			Source:      "Best practice: High volume systems need elasticity",
		})
	}

	return suggestions
}

// analyzeGaps analisa o que está faltando
func (ie *InferenceEngine) analyzeGaps() GapAnalysis {
	gap := GapAnalysis{
		MissingCategories: []string{},
		CriticalGaps:      []string{},
		NiceToHave:        []string{},
	}

	// Categorias esperadas em um sistema completo
	expectedCategories := map[string]bool{
		"backend":        false,
		"database":       false,
		"observability":  false,
		"cloud":          false,
	}

	// Marcar categorias presentes
	for _, tech := range ie.extracted.Stack {
		techLower := strings.ToLower(tech.Name)
		
		if containsAny(techLower, []string{"spring", "node", "django", "kotlin"}) {
			expectedCategories["backend"] = true
		}
		if containsAny(techLower, []string{"postgres", "scylla", "redis", "mongo"}) {
			expectedCategories["database"] = true
		}
		if containsAny(techLower, []string{"prometheus", "grafana", "jaeger", "datadog"}) {
			expectedCategories["observability"] = true
		}
		if containsAny(techLower, []string{"aws", "kubernetes", "docker", "eks"}) {
			expectedCategories["cloud"] = true
		}
	}

	// Identificar gaps
	presentCount := 0
	for category, present := range expectedCategories {
		if present {
			presentCount++
		} else {
			gap.MissingCategories = append(gap.MissingCategories, category)
			
			// Classificar como crítico ou nice-to-have
			if category == "backend" || category == "database" {
				gap.CriticalGaps = append(gap.CriticalGaps, 
					fmt.Sprintf("Categoria %s não definida (crítico)", category))
			} else {
				gap.NiceToHave = append(gap.NiceToHave, 
					fmt.Sprintf("Categoria %s não definida (recomendado)", category))
			}
		}
	}

	gap.Completeness = float64(presentCount) / float64(len(expectedCategories))

	return gap
}

// Helper functions

func (ie *InferenceEngine) calculatePatternRelevance(pattern types.Pattern) float64 {
	score := 0.0
	checks := 0

	transcriptLower := strings.ToLower(ie.transcript)
	patternText := strings.ToLower(pattern.Name + " " + pattern.When + " " + pattern.How)

	// Extrair keywords do pattern
	keywords := extractKeywords(patternText)

	for _, keyword := range keywords {
		checks++
		if strings.Contains(transcriptLower, keyword) {
			score += 1.0
		}
	}

	if checks == 0 {
		return 0.0
	}

	return score / float64(checks)
}

func (ie *InferenceEngine) explainRelevance(pattern types.Pattern) string {
	return fmt.Sprintf("Pattern '%s' é relevante pois o projeto menciona conceitos relacionados", pattern.Name)
}

func (ie *InferenceEngine) extractPatternImplications(pattern types.Pattern) []string {
	implications := []string{}

	// Analisar campo "How" para extrair implicações
	if pattern.How != "" {
		// Simplificado: extrair sentenças que começam com verbos de ação
		sentences := strings.Split(pattern.How, ".")
		for _, sentence := range sentences {
			sentence = strings.TrimSpace(sentence)
			if len(sentence) > 20 {
				implications = append(implications, sentence)
			}
		}
	}

	// Se tiver decisões, adicionar
	if len(pattern.Decisions) > 0 {
		implications = append(implications, pattern.Decisions...)
	}

	// Limitar a 3 implicações mais relevantes
	if len(implications) > 3 {
		implications = implications[:3]
	}

	return implications
}

type TechFromPattern struct {
	Name       string
	Confidence float64
	Category   string
	Required   bool
}

func (ie *InferenceEngine) extractTechsFromPattern(pattern types.Pattern) []TechFromPattern {
	techs := []TechFromPattern{}

	patternText := strings.ToLower(pattern.Name + " " + pattern.How)

	// Detectar tecnologias comuns mencionadas em patterns
	techMap := map[string]struct {
		category string
		required bool
	}{
		"kafka":      {"streaming", true},
		"flink":      {"streaming", true},
		"rocksdb":    {"database", false},
		"scylladb":   {"database", true},
		"postgresql": {"database", false},
		"prometheus": {"observability", false},
		"grafana":    {"observability", false},
	}

	for techName, techInfo := range techMap {
		if strings.Contains(patternText, techName) {
			techs = append(techs, TechFromPattern{
				Name:       strings.Title(techName),
				Confidence: 0.8,
				Category:   techInfo.category,
				Required:   techInfo.required,
			})
		}
	}

	return techs
}

func extractKeywords(text string) []string {
	// Palavras comuns a ignorar
	stopwords := map[string]bool{
		"o": true, "a": true, "de": true, "para": true, "com": true,
		"em": true, "por": true, "do": true, "da": true, "que": true,
		"the": true, "is": true, "in": true, "to": true, "and": true,
	}

	words := strings.Fields(text)
	keywords := []string{}

	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,!?;:"))
		if len(word) > 3 && !stopwords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}
