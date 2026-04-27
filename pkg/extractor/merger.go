package extractor

import (
	"fmt"
	"strings"
)

// MeetingMerger combina múltiplas extrações de reuniões
type MeetingMerger struct {
	extractions []*ExtractionResult
	transcripts []string
}

// NewMeetingMerger cria novo merger
func NewMeetingMerger() *MeetingMerger {
	return &MeetingMerger{
		extractions: []*ExtractionResult{},
		transcripts: []string{},
	}
}

// AddExtraction adiciona uma extração
func (mm *MeetingMerger) AddExtraction(result *ExtractionResult, transcript string) {
	mm.extractions = append(mm.extractions, result)
	mm.transcripts = append(mm.transcripts, transcript)
}

// Merge combina todas as extrações
func (mm *MeetingMerger) Merge() (*MergedResult, error) {
	if len(mm.extractions) == 0 {
		return nil, fmt.Errorf("nenhuma extração para combinar")
	}

	if len(mm.extractions) == 1 {
		// Só uma reunião, retornar como está
		return mm.singleMeetingResult(), nil
	}

	merged := &MergedResult{
		MeetingCount: len(mm.extractions),
		Sources:      []string{},
		Conflicts:    []Conflict{},
	}

	// Coletar sources
	for _, ext := range mm.extractions {
		merged.Sources = append(merged.Sources, ext.Metadata.Source)
	}

	// Merge context (cronológico - última reunião tem prioridade)
	context := mm.mergeContext()

	// Merge problem
	problem := mm.mergeProblem()

	// Merge objectives (união)
	objectives := mm.mergeObjectives()

	// Merge volumetry (última menção prevalece, detecta conflitos)
	volumetry, conflicts := mm.mergeVolumetry()
	merged.Conflicts = conflicts

	// Merge stack (união, marca duplicatas)
	stack := mm.mergeStack()

	// Merge NFRs (união)
	nfrs := mm.mergeNFRs()

	// Speakers (união)
	speakers := mm.mergeSpeakers()

	// Warnings (combinar)
	warnings := mm.mergeWarnings()

	// Calcular confiança geral (média ponderada)
	confidence := mm.calculateMergedConfidence()

	// Criar project definition consolidado
	merged.ProjectDefinition = mm.generateMergedMarkdown(
		context, problem, objectives, volumetry, stack, nfrs, confidence, speakers, warnings,
	)

	return merged, nil
}

// MergedResult resultado de merge
type MergedResult struct {
	MeetingCount       int        `json:"meeting_count"`
	Sources            []string   `json:"sources"`
	Conflicts          []Conflict `json:"conflicts"`
	ProjectDefinition  string     `json:"project_definition"`
}

// Conflict conflito detectado
type Conflict struct {
	Type        string   `json:"type"`
	Field       string   `json:"field"`
	Values      []string `json:"values"`
	Sources     []string `json:"sources"`
	Resolution  string   `json:"resolution"`
	NeedsReview bool     `json:"needs_review"`
}

// mergeContext combina contextos
func (mm *MeetingMerger) mergeContext() string {
	if len(mm.extractions) == 1 {
		return mm.getContext(mm.extractions[0])
	}

	// Usar o mais recente (mais completo)
	return mm.getContext(mm.extractions[len(mm.extractions)-1])
}

func (mm *MeetingMerger) getContext(ext *ExtractionResult) string {
	if ctx, ok := ext.Extractions["context"].(string); ok {
		return ctx
	}
	return ""
}

// mergeProblem combina problemas
func (mm *MeetingMerger) mergeProblem() string {
	longest := ""
	for _, ext := range mm.extractions {
		if problem, ok := ext.Extractions["problem"].(string); ok {
			if len(problem) > len(longest) {
				longest = problem
			}
		}
	}
	return longest
}

// mergeObjectives combina objetivos
func (mm *MeetingMerger) mergeObjectives() []string {
	objMap := make(map[string]bool)
	objectives := []string{}

	for _, ext := range mm.extractions {
		if objs, ok := ext.Extractions["objectives"].([]string); ok {
			for _, obj := range objs {
				normalized := strings.ToLower(strings.TrimSpace(obj))
				if !objMap[normalized] {
					objMap[normalized] = true
					objectives = append(objectives, obj)
				}
			}
		}
	}

	return objectives
}

// mergeVolumetry combina volumetria
func (mm *MeetingMerger) mergeVolumetry() (map[string]string, []Conflict) {
	volumetry := make(map[string]string)
	conflicts := []Conflict{}
	metricValues := make(map[string]map[string][]int)

	for i, ext := range mm.extractions {
		if vol, ok := ext.Extractions["volumetry"].(map[string]string); ok {
			for metric, value := range vol {
				if metricValues[metric] == nil {
					metricValues[metric] = make(map[string][]int)
				}
				metricValues[metric][value] = append(metricValues[metric][value], i)
			}
		}
	}

	// Resolver conflitos
	for metric, values := range metricValues {
		if len(values) == 1 {
			for value := range values {
				volumetry[metric] = value
			}
		} else {
			// Conflito: usar última reunião
			lastIdx := len(mm.extractions) - 1
			for value, meetingIndices := range values {
				for _, idx := range meetingIndices {
					if idx == lastIdx {
						volumetry[metric] = value
						
						var conflictValues []string
						for v := range values {
							conflictValues = append(conflictValues, v)
						}
						
						conflicts = append(conflicts, Conflict{
							Type:        "volumetry",
							Field:       metric,
							Values:      conflictValues,
							Resolution:  fmt.Sprintf("Usando valor da última reunião: %s", value),
							NeedsReview: len(values) > 2,
						})
						break
					}
				}
			}
		}
	}

	return volumetry, conflicts
}

// mergeStack combina stack
func (mm *MeetingMerger) mergeStack() []TechMention {
	techMap := make(map[string]*TechMention)

	for _, ext := range mm.extractions {
		if stack, ok := ext.Extractions["stack"].([]TechMention); ok {
			for _, tech := range stack {
				nameLower := strings.ToLower(tech.Name)
				if existing, exists := techMap[nameLower]; exists {
					// Média de confiança
					existing.Confidence = (existing.Confidence + tech.Confidence) / 2.0
				} else {
					techCopy := tech
					techMap[nameLower] = &techCopy
				}
			}
		}
	}

	stack := []TechMention{}
	for _, tech := range techMap {
		stack = append(stack, *tech)
	}
	return stack
}

// mergeNFRs combina NFRs
func (mm *MeetingMerger) mergeNFRs() []string {
	nfrMap := make(map[string]bool)
	nfrs := []string{}

	for _, ext := range mm.extractions {
		if nfrList, ok := ext.Extractions["nfrs"].([]string); ok {
			for _, nfr := range nfrList {
				normalized := strings.ToLower(strings.TrimSpace(nfr))
				if !nfrMap[normalized] {
					nfrMap[normalized] = true
					nfrs = append(nfrs, nfr)
				}
			}
		}
	}
	return nfrs
}

// mergeSpeakers combina speakers
func (mm *MeetingMerger) mergeSpeakers() []string {
	speakerMap := make(map[string]bool)
	speakers := []string{}

	for _, ext := range mm.extractions {
		for _, speaker := range ext.Metadata.SpeakersDetected {
			normalized := strings.TrimSpace(speaker)
			if !speakerMap[normalized] {
				speakerMap[normalized] = true
				speakers = append(speakers, speaker)
			}
		}
	}
	return speakers
}

// mergeWarnings combina warnings
func (mm *MeetingMerger) mergeWarnings() []string {
	warnings := []string{
		fmt.Sprintf("⚠️  Combinadas %d reuniões", len(mm.extractions)),
	}

	warningMap := make(map[string]bool)
	for i := len(mm.extractions) - 1; i >= 0; i-- {
		for _, warning := range mm.extractions[i].Metadata.Warnings {
			normalized := strings.ToLower(strings.TrimSpace(warning))
			if !warningMap[normalized] {
				warningMap[normalized] = true
				warnings = append(warnings, warning)
			}
		}
	}
	return warnings
}

// calculateMergedConfidence calcula confiança do merge
func (mm *MeetingMerger) calculateMergedConfidence() float64 {
	if len(mm.extractions) == 0 {
		return 0.0
	}

	total := 0.0
	for _, ext := range mm.extractions {
		total += ext.Metadata.OverallConfidence
	}
	return total / float64(len(mm.extractions))
}

// generateMergedMarkdown gera markdown consolidado
func (mm *MeetingMerger) generateMergedMarkdown(
	context, problem string,
	objectives []string,
	volumetry map[string]string,
	stack []TechMention,
	nfrs []string,
	confidence float64,
	speakers []string,
	warnings []string,
) string {
	var md strings.Builder

	md.WriteString("# Projeto Consolidado (Múltiplas Reuniões)\n\n")

	md.WriteString(fmt.Sprintf("_Combinadas %d reuniões_\n\n", len(mm.extractions)))

	md.WriteString("## Contexto\n\n")
	md.WriteString(context)
	md.WriteString("\n\n")

	md.WriteString("## Problema\n\n")
	md.WriteString(problem)
	md.WriteString("\n\n")

	if len(objectives) > 0 {
		md.WriteString("## Objetivos\n\n")
		for _, obj := range objectives {
			md.WriteString(fmt.Sprintf("- %s\n", obj))
		}
		md.WriteString("\n")
	}

	if len(volumetry) > 0 {
		md.WriteString("## Volumetria\n\n")
		for key, value := range volumetry {
			md.WriteString(fmt.Sprintf("- **%s**: %s\n", key, value))
		}
		md.WriteString("\n")
	}

	if len(stack) > 0 {
		md.WriteString("## Stack Técnico\n\n")
		for _, tech := range stack {
			md.WriteString(fmt.Sprintf("- **%s** (%.0f%% confiança)\n", tech.Name, tech.Confidence*100))
		}
		md.WriteString("\n")
	}

	if len(nfrs) > 0 {
		md.WriteString("## Requisitos Não-Funcionais\n\n")
		for _, nfr := range nfrs {
			md.WriteString(fmt.Sprintf("- %s\n", nfr))
		}
		md.WriteString("\n")
	}

	md.WriteString("---\n\n")
	md.WriteString(fmt.Sprintf("_Confiança: %.0f%%_\n", confidence*100))
	md.WriteString(fmt.Sprintf("_Participantes: %s_\n", strings.Join(speakers, ", ")))

	return md.String()
}

// singleMeetingResult resultado para uma única reunião
func (mm *MeetingMerger) singleMeetingResult() *MergedResult {
	ext := mm.extractions[0]
	
	return &MergedResult{
		MeetingCount:      1,
		Sources:           []string{ext.Metadata.Source},
		Conflicts:         []Conflict{},
		ProjectDefinition: ext.ProjectDefinition,
	}
}

// FormatConflictsReport formata relatório de conflitos
func FormatConflictsReport(conflicts []Conflict) string {
	if len(conflicts) == 0 {
		return "✅ Nenhum conflito detectado"
	}

	var report strings.Builder
	report.WriteString("⚠️  CONFLITOS DETECTADOS\n")
	report.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	for i, conflict := range conflicts {
		report.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, conflict.Type, conflict.Field))
		report.WriteString(fmt.Sprintf("   Valores: %v\n", conflict.Values))
		report.WriteString(fmt.Sprintf("   Resolução: %s\n", conflict.Resolution))
		if conflict.NeedsReview {
			report.WriteString("   ⚠️  REQUER REVISÃO MANUAL\n")
		}
		report.WriteString("\n")
	}

	return report.String()
}
