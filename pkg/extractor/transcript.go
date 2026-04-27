package extractor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/parser"
	"github.com/Cobliteam/workflow-toolkit/pkg/patterns"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// TranscriptExtractor extrai project definition de transcrições brutas
type TranscriptExtractor struct {
	llmClient    llm.Completer
	goldenPath   *types.GoldenPath
	teamPatterns *types.TeamPatterns
	systemPrompt string
}

// SetSystemPrompt define o system prompt do Context Loader para enriquecer chamadas LLM.
func (e *TranscriptExtractor) SetSystemPrompt(prompt string) {
	e.systemPrompt = prompt
	// Propagar para o LLM client se suportar
	if sp, ok := e.llmClient.(interface{ SetSystemPrompt(string) }); ok {
		sp.SetSystemPrompt(prompt)
	}
}

// NewTranscriptExtractor cria novo extrator
func NewTranscriptExtractor(provider llm.Provider, gpPath, tpPath string) (*TranscriptExtractor, error) {
	client, err := llm.NewClient(provider)
	if err != nil {
		return nil, err
	}

	// Carregar Golden Paths e Team Patterns (opcional)
	gp, _ := parser.ParseGoldenPaths(gpPath)
	tp, _ := parser.ParseTeamPatterns(tpPath)

	return &TranscriptExtractor{
		llmClient:    client,
		goldenPath:   gp,
		teamPatterns: tp,
	}, nil
}

// NewTranscriptExtractorWithProvider creates an extractor using a fully-decorated LLMProvider.
// This ensures the SecurityCheckpoint, Retry, and Cache decorators are in the call path.
// Preferred over NewTranscriptExtractor() which uses the deprecated NewClient() with bare os.Getenv().
func NewTranscriptExtractorWithProvider(provider llm.LLMProvider, gpPath, tpPath string) (*TranscriptExtractor, error) {
	gp, _ := parser.ParseGoldenPaths(gpPath)
	tp, _ := parser.ParseTeamPatterns(tpPath)

	return &TranscriptExtractor{
		llmClient:    provider,
		goldenPath:   gp,
		teamPatterns: tp,
	}, nil
}

// NewTranscriptExtractorWithClient cria extrator com client injetado (para testes)
func NewTranscriptExtractorWithClient(client llm.Completer, gp *types.GoldenPath, tp *types.TeamPatterns) *TranscriptExtractor {
	return &TranscriptExtractor{
		llmClient:    client,
		goldenPath:   gp,
		teamPatterns: tp,
	}
}

// ExtractionResult resultado da extração
type ExtractionResult struct {
	ProjectDefinition string                 `json:"project_definition"` // Markdown gerado
	Metadata          ExtractionMetadata     `json:"metadata"`
	Extractions       map[string]interface{} `json:"extractions"`
	Scoring           *ScoringResult         `json:"scoring,omitempty"`   // Detalhes de scoring
	Inference         *InferenceResult       `json:"inference,omitempty"` // NOVO: Inferências de GP/TP
}

// ExtractionMetadata metadados da extração
type ExtractionMetadata struct {
	Source             string             `json:"source"`
	OverallConfidence  float64            `json:"overall_confidence"`
	SectionConfidence  map[string]float64 `json:"section_confidence"`
	SpeakersDetected   []string           `json:"speakers_detected"`
	Warnings           []string           `json:"warnings"`
	ExplicitMentions   []Mention          `json:"explicit_mentions"`
	InferredItems      []InferredItem     `json:"inferred_items"`
}

// Mention menção explícita
type Mention struct {
	Type      string  `json:"type"`       // tech, nfr, volumetry
	Value     string  `json:"value"`
	Speaker   string  `json:"speaker,omitempty"`
	Timestamp string  `json:"timestamp,omitempty"`
}

// InferredItem item inferido
type InferredItem struct {
	Type       string  `json:"type"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
}

// Extract extrai project definition de transcrição
func (e *TranscriptExtractor) Extract(transcriptPath string) (*ExtractionResult, error) {
	// 1. Ler transcrição
	transcript, err := readFile(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler transcrição: %w", err)
	}

	// 2. Construir contexto de Golden Paths e Team Patterns
	contextInfo := e.buildContextInfo()

	// 3. Chamar LLM para extrair
	extraction, err := e.extractWithLLM(transcript, contextInfo)
	if err != nil {
		return nil, fmt.Errorf("erro ao extrair: %w", err)
	}

	// 4. NOVO: Calcular confidence scores refinados
	scorer := NewConfidenceScorer(transcript, extraction)
	scoringResult := scorer.Score()

	// Atualizar confiança com scores calculados
	extraction.OverallConfidence = scoringResult.OverallScore
	for section, sectionScore := range scoringResult.SectionScores {
		extraction.SectionConfidence[section] = sectionScore.Score
	}

	// Adicionar recomendações aos warnings
	extraction.Warnings = append(extraction.Warnings, scoringResult.Recommendations...)

	// 5. NOVO: Executar inferências com Golden Paths
	var inferenceResult *InferenceResult
	if e.goldenPath != nil || e.teamPatterns != nil {
		inferenceEngine := NewInferenceEngine(e.goldenPath, e.teamPatterns, transcript, extraction)
		inferenceResult = inferenceEngine.Infer()

		// Adicionar tecnologias inferidas ao extracted
		for _, tech := range inferenceResult.InferredTechnologies {
			if tech.Confidence >= 0.6 { // Só adicionar se confiança >= 60%
				extraction.Stack = append(extraction.Stack, TechMention{
					Name:       tech.Name,
					Confidence: tech.Confidence,
					Source:     "inferred",
					Rationale:  tech.Rationale,
				})
			}
		}

		// Adicionar NFRs inferidos
		for _, nfr := range inferenceResult.InferredNFRs {
			if nfr.Confidence >= 0.7 { // Só adicionar se confiança >= 70%
				extraction.NFRs = append(extraction.NFRs, nfr.Requirement)
				extraction.InferredItems = append(extraction.InferredItems, InferredItem{
					Type:       "nfr",
					Value:      nfr.Requirement,
					Confidence: nfr.Confidence,
					Rationale:  nfr.Rationale,
				})
			}
		}

		// Adicionar sugestões aos warnings
		for _, suggestion := range inferenceResult.Suggestions {
			if suggestion.Confidence >= 0.7 {
				extraction.Warnings = append(extraction.Warnings, 
					fmt.Sprintf("💡 %s", suggestion.Description))
			}
		}
	}

	// 6. Gerar markdown estruturado
	markdown := e.generateMarkdown(extraction)

	// 7. Montar resultado
	result := &ExtractionResult{
		ProjectDefinition: markdown,
		Metadata: ExtractionMetadata{
			Source:            transcriptPath,
			OverallConfidence: extraction.OverallConfidence,
			SectionConfidence: extraction.SectionConfidence,
			SpeakersDetected:  extraction.Speakers,
			Warnings:          extraction.Warnings,
			ExplicitMentions:  extraction.ExplicitMentions,
			InferredItems:     extraction.InferredItems,
		},
		Extractions: extraction.RawData,
		Scoring:     scoringResult,
		Inference:   inferenceResult, // NOVO: Incluir inferências
	}

	return result, nil
}

// ExtractedData dados extraídos pelo LLM
type ExtractedData struct {
	Context           string             `json:"context"`
	Problem           string             `json:"problem"`
	Objectives        []string           `json:"objectives"`
	Volumetry         map[string]string  `json:"volumetry"`
	Stack             []TechMention      `json:"stack"`
	NFRs              []string           `json:"nfrs"`
	OverallConfidence float64            `json:"overall_confidence"`
	SectionConfidence map[string]float64 `json:"section_confidence"`
	Speakers          []string           `json:"speakers"`
	Warnings          []string           `json:"warnings"`
	ExplicitMentions  []Mention          `json:"explicit_mentions"`
	InferredItems     []InferredItem     `json:"inferred_items"`
	RawData           map[string]interface{} `json:"raw_data"`
}

// TechMention menção de tecnologia
type TechMention struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"` // "explicit" ou "inferred"
	Rationale  string  `json:"rationale,omitempty"`
}

// buildContextInfo constrói informações de contexto usando layers
func (e *TranscriptExtractor) buildContextInfo() string {
	// Usar sistema de layers para economizar tokens
	layer := &patterns.PatternLayer{}
	
	// Extração usa Essentials (2KB vs 23KB)
	patternsText := layer.GetCombined(patterns.Essentials)
	return layer.FormatWithHeader(patternsText, patterns.Essentials)
}

// extractWithLLM extrai usando LLM com prompt avançado
func (e *TranscriptExtractor) extractWithLLM(transcript string, contextInfo string) (*ExtractedData, error) {
	// Usar PromptBuilder para criar prompt otimizado
	builder := NewPromptBuilder(transcript, contextInfo)
	prompt := builder.Build()

	response, err := e.llmClient.Complete(prompt, 4000)
	if err != nil {
		return nil, err
	}

	// Parse JSON
	jsonStr := extractJSON(response)
	
	var extracted ExtractedData
	if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
		// Fallback: tentar com prompt alternativo
		fmt.Println("⚠️  Ajustando estratégia de extração...")
		simplifiedPrompt := builder.BuildSimplified()
		response, err = e.llmClient.Complete(simplifiedPrompt, 3000)
		if err != nil {
			return nil, fmt.Errorf("erro na segunda tentativa: %w", err)
		}
		
		jsonStr = extractJSON(response)
		if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
			return nil, fmt.Errorf("erro ao parsear JSON da extração: %w", err)
		}
	}

	// Popular RawData para metadata
	extracted.RawData = map[string]interface{}{
		"context":    extracted.Context,
		"problem":    extracted.Problem,
		"objectives": extracted.Objectives,
		"volumetry":  extracted.Volumetry,
		"stack":      extracted.Stack,
		"nfrs":       extracted.NFRs,
	}

	return &extracted, nil
}

// generateMarkdown gera markdown estruturado
func (e *TranscriptExtractor) generateMarkdown(data *ExtractedData) string {
	var md strings.Builder

	md.WriteString("# Projeto Extraído de Reunião\n\n")

	// Contexto
	md.WriteString("## Contexto\n\n")
	md.WriteString(data.Context)
	md.WriteString(fmt.Sprintf("\n\n_Confiança: %.0f%%_\n\n", data.SectionConfidence["context"]*100))

	// Problema
	md.WriteString("## Problema\n\n")
	md.WriteString(data.Problem)
	md.WriteString(fmt.Sprintf("\n\n_Confiança: %.0f%%_\n\n", data.SectionConfidence["problem"]*100))

	// Objetivos
	if len(data.Objectives) > 0 {
		md.WriteString("## Objetivos\n\n")
		for _, obj := range data.Objectives {
			md.WriteString(fmt.Sprintf("- %s\n", obj))
		}
		md.WriteString("\n")
	}

	// Volumetria
	if len(data.Volumetry) > 0 {
		md.WriteString("## Volumetria\n\n")
		for key, value := range data.Volumetry {
			md.WriteString(fmt.Sprintf("- **%s**: %s\n", strings.Title(key), value))
		}
		md.WriteString(fmt.Sprintf("\n_Confiança: %.0f%%_\n\n", data.SectionConfidence["volumetry"]*100))
	}

	// Stack Técnico
	if len(data.Stack) > 0 {
		md.WriteString("## Stack Técnico\n\n")
		
		// Separar explícito vs inferido
		var explicit, inferred []TechMention
		for _, tech := range data.Stack {
			if tech.Source == "explicit" {
				explicit = append(explicit, tech)
			} else {
				inferred = append(inferred, tech)
			}
		}

		if len(explicit) > 0 {
			md.WriteString("### Confirmado (mencionado explicitamente)\n\n")
			for _, tech := range explicit {
				md.WriteString(fmt.Sprintf("- **%s**", tech.Name))
				if tech.Rationale != "" {
					md.WriteString(fmt.Sprintf(" _%s_", tech.Rationale))
				}
				md.WriteString("\n")
			}
			md.WriteString("\n")
		}

		if len(inferred) > 0 {
			md.WriteString("### Sugerido (inferido)\n\n")
			for _, tech := range inferred {
				md.WriteString(fmt.Sprintf("- **%s** ⚠️  (confiança: %.0f%%)\n", tech.Name, tech.Confidence*100))
				if tech.Rationale != "" {
					md.WriteString(fmt.Sprintf("  _%s_\n", tech.Rationale))
				}
			}
			md.WriteString("\n")
		}

		md.WriteString(fmt.Sprintf("_Confiança geral: %.0f%%_\n\n", data.SectionConfidence["stack"]*100))
	}

	// NFRs
	if len(data.NFRs) > 0 {
		md.WriteString("## Requisitos Não-Funcionais\n\n")
		for _, nfr := range data.NFRs {
			md.WriteString(fmt.Sprintf("- %s\n", nfr))
		}
		md.WriteString(fmt.Sprintf("\n_Confiança: %.0f%%_\n\n", data.SectionConfidence["nfrs"]*100))
	}

	// Warnings
	if len(data.Warnings) > 0 {
		md.WriteString("## ⚠️  Avisos\n\n")
		for _, warning := range data.Warnings {
			md.WriteString(fmt.Sprintf("- %s\n", warning))
		}
		md.WriteString("\n")
	}

	// Footer
	md.WriteString("---\n\n")
	md.WriteString(fmt.Sprintf("_Gerado automaticamente com confiança geral de %.0f%%_\n", data.OverallConfidence*100))
	md.WriteString("_Revisar seções marcadas com ⚠️  antes de prosseguir_\n")

	return md.String()
}

// Helper functions

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func extractJSON(text string) string {
	// Extrair JSON de resposta que pode ter markdown backticks
	text = strings.TrimSpace(text)
	
	// Remover ```json e ```
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	
	return strings.TrimSpace(text)
}
