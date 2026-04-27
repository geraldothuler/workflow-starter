package render

import (
	"fmt"
	"strings"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ConvertToLensData converte types.Backlog para lens.LensData
func ConvertToLensData(backlog *types.Backlog, deepDives []types.DeepDive) *LensData {
	// 1. Calcular esforço (sempre)
	effort := calculateEffort(backlog)
	
	// 2. Tentar carregar team config
	teamConfig := loadTeamConfig()
	
	// 3. Se tem limites de paralelismo, analisar caminho crítico
	if teamConfig != nil && teamConfig.ParallelismLimits != nil {
		effort.CriticalPath = analyzeCriticalPath(
			backlog,
			&effort,
			teamConfig.ParallelismLimits,
		)
	}
	
	// 4. Inferir milestones
	milestones := inferMilestones(backlog, &effort)
	
	// 5. Carregar documentos
	documents := loadDocuments()
	
	lensData := &LensData{
		Meta:       convertMeta(backlog),
		KPIs:       generateKPIs(backlog),
		Epics:      convertEpics(backlog),
		Stories:    convertStories(backlog),
		DeepDives:  convertDeepDives(deepDives),
		Search:     nil, // Não implementado ainda
		Milestones: milestones,  // v3.2: Milestones inferidos
		Timeline:   Timeline{Milestones: milestones}, // v3.2: Timeline com milestones
		Graph:      DependencyGraph{Nodes: []interface{}{}, Edges: []interface{}{}},
		Summary:    generateSummary(backlog),
		Squad:      generateSquadAnalysis(backlog),
		Effort:     effort,     // v3.2: Esforço + caminho crítico
		Documents:  documents,  // v3.2: Documentos de origem
		Metrics:    convertMetrics(backlog.Meta.Metrics), // v3.3: Métricas de geração
		PatternSuggestions: convertPatternSuggestions(backlog.PatternSuggestions), // v3.4: Patterns
		CoherenceIssues:    convertCoherenceIssues(backlog.CoherenceIssues),     // v3.5: Coherence
		FeasibilityReport:  convertFeasibilityReport(backlog.FeasibilityReport), // v3.6: Feasibility
		CriticalPathReport: convertCriticalPathReport(backlog.CriticalPathReport), // v3.7: Critical path
	}

	return lensData
}

// convertMeta converte metadata
func convertMeta(backlog *types.Backlog) MetaData {
	// Usar ProjectTitle inferido, com fallback
	title := backlog.Meta.ProjectTitle
	if title == "" {
		title = "Backlog Técnico"
	}

	// Usar lang do backlog, com fallback para pt-BR
	lang := backlog.Meta.Lang
	if lang == "" {
		lang = "pt-BR"
	}

	return MetaData{
		Title:          title,
		TitleHighlight: fmt.Sprintf("%d Épicos", len(backlog.Epics)),
		Subtitle:       backlog.Meta.InputFile,
		Eyebrow:        "Workflow",
		Lang:           lang,
		Description:    fmt.Sprintf("Backlog gerado com %d épicos e %d histórias", backlog.Meta.TotalEpics, backlog.Meta.TotalStories),
		TotalEpics:     backlog.Meta.TotalEpics,
		TotalStories:   backlog.Meta.TotalStories,
		KPIs:           generateKPIs(backlog),
	}
}

// generateKPIs gera KPIs do backlog
func generateKPIs(backlog *types.Backlog) []KPI {
	return []KPI{
		{
			Label:  "Épicos",
			Value:  len(backlog.Epics),
			Sub:    "features principais",
			Source: "backlog",
		},
		{
			Label:  "Histórias",
			Value:  backlog.Meta.TotalStories,
			Sub:    "histórias técnicas",
			Source: "backlog",
		},
		{
			Label:  "Story Points",
			Value:  backlog.Meta.Stats.TotalStoryPoints,
			Sub:    "esforço total",
			Source: "backlog",
		},
		{
			Label:  "Critérios",
			Value:  backlog.Meta.Stats.TotalCriteria,
			Sub:    "critérios de aceite",
			Source: "backlog",
		},
	}
}

// convertEpics converte épicos para map
func convertEpics(backlog *types.Backlog) map[string]EpicLens {
	epicsMap := make(map[string]EpicLens)
	
	for _, epic := range backlog.Epics {
		// Converter histórias do épico
		stories := []StoryLens{}
		for _, story := range epic.Stories {
			stories = append(stories, convertStory(&story))
		}
		
		// Converter tags
		tags := []interface{}{}
		for _, tag := range epic.Tags {
			tags = append(tags, tag)
		}
		
		epicLens := EpicLens{
			ID:          epic.ID,
			Code:        epic.Code,
			Title:       epic.Title,
			Summary:     truncateAtWord(epic.Description, 120),
			Description: epic.Description,
			Context:     "", // Pode ser enriquecido depois
			Stories:     stories,
			Tags:        tags,
			Priority:    epic.Priority,
		}
		
		epicsMap[epic.ID] = epicLens
	}
	
	return epicsMap
}

// convertStories converte todas as histórias para map
func convertStories(backlog *types.Backlog) map[string]StoryLens {
	storiesMap := make(map[string]StoryLens)
	
	for _, epic := range backlog.Epics {
		for _, story := range epic.Stories {
			storyLens := convertStory(&story)
			storiesMap[story.ID] = storyLens
		}
	}
	
	return storiesMap
}

// convertStory converte uma história individual
func convertStory(story *types.Story) StoryLens {
	// Converter tags
	tags := []interface{}{}
	for _, tag := range story.Tags {
		tags = append(tags, tag)
	}
	
	// Converter tasks se existirem
	tasks := []TaskLens{}
	for _, task := range story.Tasks {
		tasks = append(tasks, TaskLens{
			ID:             task.ID,
			Title:          task.Description,
			Description:    task.Description,
			Category:       "implementation",
			EstimatedHours: 4, // Default 4h
			Status:         "todo",
			Effort:         task.Effort,
		})
	}
	
	return StoryLens{
		ID:                 story.ID,
		Code:               story.ID,
		Title:              story.Title,
		Description:        story.What,
		What:               story.What,
		Why:                story.Why,
		AcceptanceCriteria: story.AcceptanceCriteria,
		Tags:               tags,
		Effort:             story.Effort,
		EstimatedPoints:    story.Effort,
		Source:             "generated",
		Tasks:              tasks,
		RiskLabel:          calculateRisk(story),
		Subtasks:           []string{},
	}
}

// convertDeepDives converte deep dives para map
func convertDeepDives(deepDives []types.DeepDive) map[string]DeepDiveLens {
	ddMap := make(map[string]DeepDiveLens)
	
	for i, dd := range deepDives {
		id := fmt.Sprintf("dd-%d", i+1)

		terms := []interface{}{}
		for _, term := range dd.Terms {
			terms = append(terms, term)
		}

		ddLens := DeepDiveLens{
			ID:               id,
			StoryID:          dd.StoryID,
			Term:             dd.Term,
			Terms:            terms,
			WhatIs:           dd.WhatIs,
			WhatInThisStory:  dd.WhatInThisStory,
			WhyHere:          dd.WhyHere,
			WhyInThisStory:   dd.WhyInThisStory,
			Configuration:    dd.Configuration,
			Patterns:         dd.Patterns,
			SourcePatterns:   dd.SourcePatterns,
			RelatedCriteria:  dd.RelatedCriteria,
			AntiPatterns:     []string{},
			Decisions:        dd.Decisions,
			RelatedTerms:     []string{},
			Classification:   dd.Classification,
			Scope:            dd.Scope,
			Source: SourceTrace{
				Type:       "generated",
				File:       "deep-dives.json",
				Line:       i + 1,
				Confidence: "high",
			},
		}

		// Key por term (ou storyID:term se contextualizado)
		key := dd.Term
		if dd.StoryID != "" {
			key = dd.StoryID + ":" + dd.Term
		}
		ddMap[key] = ddLens
	}
	
	return ddMap
}

// generateSummary gera sumário do backlog
func generateSummary(backlog *types.Backlog) Summary {
	return Summary{
		Overview: fmt.Sprintf("Backlog técnico com %d épicos e %d histórias totalizando %d story points",
			len(backlog.Epics), backlog.Meta.TotalStories, backlog.Meta.Stats.TotalStoryPoints),
		KeyPoints: []string{
			fmt.Sprintf("%d épicos de alta prioridade", countHighPriority(backlog)),
			fmt.Sprintf("%d histórias técnicas detalhadas", backlog.Meta.TotalStories),
			fmt.Sprintf("%d critérios de aceite mensuráveis", backlog.Meta.Stats.TotalCriteria),
		},
	}
}

// generateSquadAnalysis gera análise de squad
func generateSquadAnalysis(backlog *types.Backlog) SquadAnalysis {
	return SquadAnalysis{
		SquadName: "Engineering Team",
		Stories:   backlog.Meta.TotalStories,
		Points:    backlog.Meta.Stats.TotalStoryPoints,
	}
}

// convertMetrics converte métricas de geração para formato Lens
func convertMetrics(m *types.GenerationMetrics) *GenerationMetricsLens {
	if m == nil {
		return nil
	}
	return &GenerationMetricsLens{
		TotalTechsExtracted:   m.TotalTechsExtracted,
		TrivialFiltered:       m.TrivialFiltered,
		ClassificationStats:   m.ClassificationStats,
		CrossEpicGlobalDives:  m.CrossEpicGlobalDives,
		CrossEpicDeduplicated: m.CrossEpicDeduplicated,
		LLMCallsMade:          m.LLMCallsMade,
		LLMCallsSaved:         m.LLMCallsSaved,
		ReductionPercent:      m.ReductionPercent,
		TotalInputTokens:      m.TotalInputTokens,
		TotalOutputTokens:     m.TotalOutputTokens,
		TotalCost:             m.TotalCost,
	}
}

// truncateAtWord trunca string no limite de palavras, sem cortar no meio
func truncateAtWord(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Encontrar último espaço antes do limite
	truncated := s[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 {
		truncated = truncated[:lastSpace]
	}
	return truncated + "..."
}

// Funções auxiliares
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func calculateRisk(story *types.Story) string {
	if story.Effort >= 8 {
		return "high"
	} else if story.Effort >= 5 {
		return "medium"
	}
	return "low"
}

func countHighPriority(backlog *types.Backlog) int {
	count := 0
	for _, epic := range backlog.Epics {
		if epic.Priority == "high" {
			count++
		}
	}
	return count
}

// convertPatternSuggestions converts types.PatternSuggestion to lens format.
func convertPatternSuggestions(suggestions []types.PatternSuggestion) []PatternSuggestionLens {
	if len(suggestions) == 0 {
		return nil
	}
	result := make([]PatternSuggestionLens, len(suggestions))
	for i, s := range suggestions {
		result[i] = PatternSuggestionLens{
			PatternID:     s.PatternID,
			PatternName:   s.PatternName,
			Type:          s.Type,
			Confidence:    s.Confidence,
			Reasoning:     s.Reasoning,
			AffectedEpics: s.AffectedEpics,
			Category:      s.Category,
			Source:        s.Source,
			Level:         s.Level,
			Remediation:   s.Remediation,
			BlockedBy:     s.BlockedBy,
		}
	}
	return result
}

// convertFeasibilityReport converts types.FeasibilityReport to lens format.
func convertFeasibilityReport(report *types.FeasibilityReport) *FeasibilityReportLens {
	if report == nil {
		return nil
	}
	items := make([]FeasibilityItemLens, len(report.Items))
	for i, item := range report.Items {
		items[i] = FeasibilityItemLens{
			ID:          item.ID,
			Category:    item.Category,
			Severity:    item.Severity,
			Title:       item.Title,
			Description: item.Description,
			Impact:      item.Impact,
			Suggestion:  item.Suggestion,
		}
	}
	return &FeasibilityReportLens{
		Score:   report.Score,
		Items:   items,
		Summary: report.Summary,
	}
}

// convertCriticalPathReport converts types.CriticalPathReport to lens format.
func convertCriticalPathReport(report *types.CriticalPathReport) *CriticalPathReportLens {
	if report == nil {
		return nil
	}
	items := make([]CriticalPathItemLens, len(report.Items))
	for i, item := range report.Items {
		items[i] = CriticalPathItemLens{
			ID:           item.ID,
			EpicCode:     item.EpicCode,
			EpicTitle:    item.EpicTitle,
			Phase:        item.Phase,
			Priority:     item.Priority,
			Reasoning:    item.Reasoning,
			DependsOn:    item.DependsOn,
			IsFoundation: item.IsFoundation,
			Tags:         item.Tags,
		}
	}
	phases := make([]ExecutionPhaseLens, len(report.Phases))
	for i, phase := range report.Phases {
		phases[i] = ExecutionPhaseLens{
			Phase:       phase.Phase,
			EpicCodes:   phase.EpicCodes,
			Parallel:    phase.Parallel,
			TotalEffort: phase.TotalEffort,
			Reasoning:   phase.Reasoning,
		}
	}
	deps := make([]DependencyEdgeLens, len(report.Dependencies))
	for i, dep := range report.Dependencies {
		deps[i] = DependencyEdgeLens{
			From:       dep.From,
			To:         dep.To,
			Type:       dep.Type,
			Confidence: dep.Confidence,
			Reasoning:  dep.Reasoning,
		}
	}
	return &CriticalPathReportLens{
		Items:        items,
		Phases:       phases,
		Dependencies: deps,
		Summary:      report.Summary,
	}
}

// convertCoherenceIssues converts types.CoherenceIssue to lens format.
func convertCoherenceIssues(issues []types.CoherenceIssue) []CoherenceIssueLens {
	if len(issues) == 0 {
		return nil
	}
	result := make([]CoherenceIssueLens, len(issues))
	for i, issue := range issues {
		result[i] = CoherenceIssueLens{
			ID:          issue.ID,
			Type:        issue.Type,
			Severity:    issue.Severity,
			Title:       issue.Title,
			Description: issue.Description,
			PatternIDs:  issue.PatternIDs,
			Suggestion:  issue.Suggestion,
		}
	}
	return result
}
