package render

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EFFORT CALCULATION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func calculateEffort(backlog *types.Backlog) EffortSummary {
	summary := EffortSummary{
		Velocity: 20, // Default: 20 SPs/sprint
		ByEpic:   make(map[string]EpicEffort),
	}
	
	// DEBUG: Ver quantos épicos e stories temos
	fmt.Printf("[EFFORT DEBUG] Épicos no backlog: %d\n", len(backlog.Epics))
	
	// Somar por épico
	for _, epic := range backlog.Epics {
		epicEffort := EpicEffort{
			EpicID: epic.ID,
		}
		
		fmt.Printf("[EFFORT DEBUG] Épico %s tem %d stories\n", epic.ID, len(epic.Stories))
		
		for _, story := range epic.Stories {
			epicEffort.Stories++
			epicEffort.SPs += story.Effort
		}
		
		// 1 SP ≈ 0.5 dia
		epicEffort.Days = (epicEffort.SPs + 1) / 2
		
		summary.ByEpic[epic.ID] = epicEffort
		summary.TotalStories += epicEffort.Stories
		summary.TotalSPs += epicEffort.SPs
	}
	
	// DEBUG: Ver totais
	fmt.Printf("[EFFORT DEBUG] Total SPs: %d, Total Stories: %d\n", summary.TotalSPs, summary.TotalStories)
	
	// Total de dias (otimista - tudo paralelo)
	summary.OptimisticDays = (summary.TotalSPs + 1) / 2
	summary.TotalDays = summary.OptimisticDays // Default
	summary.RealisticDays = summary.OptimisticDays // Default
	
	// Calcular percentagens
	if summary.TotalSPs > 0 {
		for epicID, effort := range summary.ByEpic {
			e := effort
			e.Percentage = float64(effort.SPs) / float64(summary.TotalSPs) * 100
			summary.ByEpic[epicID] = e
		}
	}
	
	return summary
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TEAM CONFIG LOADING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func loadTeamConfig() *TeamConfig {
	// Tentar ler team-patterns.md
	content, err := os.ReadFile("team-patterns.md")
	if err != nil {
		return nil
	}
	
	config := parseParallelismConfig(string(content))
	return config
}

func parseParallelismConfig(content string) *TeamConfig {
	// Procurar por seção "Limites de Paralelismo"
	if !strings.Contains(content, "Limites de Paralelismo") {
		return nil
	}
	
	config := &TeamConfig{
		Velocity: 20, // Default
		ParallelismLimits: &ParallelismLimits{},
	}
	
	// Extrair limites
	config.ParallelismLimits.LargeStories = extractLimit(content, "Grandes/Complexas", "5\\+ SPs")
	config.ParallelismLimits.MediumStories = extractLimit(content, "Médias", "3-4 SPs")
	config.ParallelismLimits.SmallStories = extractLimit(content, "Pequenas", "1-2 SPs")
	
	// Se não encontrou nenhum limite, retornar nil
	if config.ParallelismLimits.LargeStories == 0 &&
	   config.ParallelismLimits.MediumStories == 0 &&
	   config.ParallelismLimits.SmallStories == 0 {
		return nil
	}
	
	return config
}

func extractLimit(content, category, sizePattern string) int {
	// Padrão: "**Histórias {category}** ({size}):.*Limite: {N}"
	pattern := fmt.Sprintf(
		`\*\*Histórias %s\*\*.*%s.*Limite:\s*(\d+)`,
		category, sizePattern,
	)
	
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)
	
	if len(matches) > 1 {
		limit, _ := strconv.Atoi(matches[1])
		return limit
	}
	
	return 0 // Sem limite = ilimitado
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CRITICAL PATH ANALYSIS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func analyzeCriticalPath(
	backlog *types.Backlog,
	effort *EffortSummary,
	limits *ParallelismLimits,
) *CriticalPathAnalysis {
	// Só analisar se limites existem
	if limits == nil {
		return nil
	}
	
	analysis := &CriticalPathAnalysis{
		Enabled:      true,
		LargeStories: []CriticalStory{},
		Bottlenecks:  []Bottleneck{},
	}
	
	// Encontrar histórias afetadas pelos limites
	largeStories := findLargeStories(backlog, limits)
	
	// Calcular dias bloqueados
	for _, story := range largeStories {
		days := float64(story.Effort) * 0.5 // 1 SP = 0.5 dia
		
		critStory := CriticalStory{
			Code:         story.ID,
			Title:        story.Title,
			EpicID:       story.EpicID,
			Effort:       story.Effort,
			Days:         days,
			IsBottleneck: true,
			Reason:       generateStoryReason(story, limits),
		}
		
		analysis.LargeStories = append(analysis.LargeStories, critStory)
		analysis.TotalBlockedDays += int(days)
	}
	
	// Agrupar por épico
	bottlenecks := groupBottlenecksByEpic(largeStories, backlog)
	for _, bottleneck := range bottlenecks {
		analysis.Bottlenecks = append(analysis.Bottlenecks, bottleneck)
	}
	
	// Gerar recomendações
	if len(analysis.LargeStories) > 0 {
		analysis.Recommendations = generateRecommendations(analysis)
	}
	
	// Calcular dias realistas (com gargalos)
	effort.RealisticDays = effort.OptimisticDays + analysis.TotalBlockedDays
	
	return analysis
}

func findLargeStories(backlog *types.Backlog, limits *ParallelismLimits) []types.Story {
	large := []types.Story{}
	
	for _, epic := range backlog.Epics {
		for _, story := range epic.Stories {
			// História grande com limite?
			if story.Effort >= 5 && limits.LargeStories > 0 {
				large = append(large, story)
			}
		}
	}
	
	return large
}

func generateStoryReason(story types.Story, limits *ParallelismLimits) string {
	reasons := []string{}
	
	// Razão por tamanho
	if story.Effort >= 8 {
		reasons = append(reasons, fmt.Sprintf(
			"História muito grande (%d SPs = %.1f dias)",
			story.Effort, float64(story.Effort)*0.5,
		))
	} else if story.Effort >= 5 {
		reasons = append(reasons, fmt.Sprintf(
			"História grande (%d SPs = %.1f dias)",
			story.Effort, float64(story.Effort)*0.5,
		))
	}
	
	// Razão por limite
	reasons = append(reasons, fmt.Sprintf(
		"Limite de paralelismo: %d história(s) por vez",
		limits.LargeStories,
	))
	
	// Razão por tags (tecnologias complexas)
	complexTechs := []string{"Flink", "Kafka", "Migration", "Pipeline", "Schema"}
	for _, tag := range story.Tags {
		tagStr := fmt.Sprint(tag)
		for _, tech := range complexTechs {
			if strings.Contains(tagStr, tech) {
				reasons = append(reasons, fmt.Sprintf(
					"Requer expertise em %s (recurso limitado)",
					tech,
				))
				break
			}
		}
	}
	
	return strings.Join(reasons, ". ") + "."
}

func groupBottlenecksByEpic(stories []types.Story, backlog *types.Backlog) []Bottleneck {
	// Agrupar histórias por épico
	byEpic := make(map[string][]types.Story)
	for _, story := range stories {
		byEpic[story.EpicID] = append(byEpic[story.EpicID], story)
	}
	
	// Criar bottlenecks
	bottlenecks := []Bottleneck{}
	for epicID, epicStories := range byEpic {
		epic := findEpicByID(backlog, epicID)
		if epic == nil {
			continue
		}
		
		totalSPs := 0
		for _, s := range epicStories {
			totalSPs += s.Effort
		}
		totalDays := (totalSPs + 1) / 2
		
		bottleneck := Bottleneck{
			EpicID:      epicID,
			EpicTitle:   epic.Title,
			BlockedDays: totalDays,
			Reason: fmt.Sprintf(
				"Épico '%s' tem %d história(s) grande(s) totalizando %d SPs. "+
				"Como apenas 1 pode ser trabalhada por vez, isso bloqueia o progresso "+
				"por ~%d dias, enquanto o resto da squad fica parcialmente ocioso.",
				epic.Title, len(epicStories), totalSPs, totalDays,
			),
			Suggestion: generateBottleneckSuggestion(epicStories, epic),
		}
		
		bottlenecks = append(bottlenecks, bottleneck)
	}
	
	return bottlenecks
}

func findEpicByID(backlog *types.Backlog, epicID string) *types.Epic {
	for i := range backlog.Epics {
		if backlog.Epics[i].ID == epicID {
			return &backlog.Epics[i]
		}
	}
	return nil
}

func generateBottleneckSuggestion(stories []types.Story, epic *types.Epic) string {
	suggestions := []string{}
	
	// Sugestão: quebrar histórias
	for _, story := range stories {
		if story.Effort >= 8 {
			suggestions = append(suggestions, fmt.Sprintf(
				"Considere quebrar '%s' (%d SPs) em histórias menores de 3-5 SPs cada",
				story.Title, story.Effort,
			))
		}
	}
	
	// Sugestão: parear
	if len(stories) > 1 {
		suggestions = append(suggestions,
			"Parear desenvolvedores nas histórias críticas para acelerar execução",
		)
	}
	
	// Sugestão: priorizar
	suggestions = append(suggestions,
		"Priorizar histórias grandes no início do sprint para desbloquear o restante",
	)
	
	return strings.Join(suggestions, ". ") + "."
}

func generateRecommendations(analysis *CriticalPathAnalysis) []string {
	recs := []string{}
	
	// Recomendação geral
	if len(analysis.LargeStories) > 3 {
		recs = append(recs, fmt.Sprintf(
			"⚠️ Projeto tem %d histórias grandes que criam gargalos. "+
			"Priorize quebrar essas histórias para reduzir tempo total de %d dias.",
			len(analysis.LargeStories),
			analysis.TotalBlockedDays,
		))
	}
	
	// Recomendação por bottleneck
	for _, b := range analysis.Bottlenecks {
		if b.BlockedDays > 5 {
			recs = append(recs, fmt.Sprintf(
				"🔴 '%s': Gargalo de %d dias. %s",
				b.EpicTitle, b.BlockedDays, b.Suggestion,
			))
		}
	}
	
	// Recomendação sobre parear
	if len(analysis.LargeStories) > 0 {
		recs = append(recs,
			"💡 Considere alocar 2 desenvolvedores em histórias grandes para "+
			"reduzir o tempo pela metade (ex: 8 SPs = 2 dias ao invés de 4).",
		)
	}
	
	return recs
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// DOCUMENTS LOADING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func loadDocuments() map[string]DocumentInfo {
	docs := make(map[string]DocumentInfo)
	
	// Tentar carregar cada documento
	docPaths := map[string]string{
		"transcript":    "meeting-transcript.txt",
		"specification": "project-definition.md",
		"golden_paths":  "golden-paths.md",
		"team_patterns": "team-patterns.md",
	}
	
	for key, path := range docPaths {
		if content, err := os.ReadFile(path); err == nil {
			stat, _ := os.Stat(path)
			docs[key] = DocumentInfo{
				Name:      filepath.Base(path),
				Path:      path,
				SizeKB:    int(stat.Size() / 1024),
				Available: true,
				Content:   string(content),
			}
		} else {
			docs[key] = DocumentInfo{
				Name:      filepath.Base(path),
				Path:      path,
				Available: false,
			}
		}
	}
	
	return docs
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// MILESTONES INFERENCE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func inferMilestones(backlog *types.Backlog, effort *EffortSummary) []Milestone {
	milestones := []Milestone{}
	
	// Se backlog muito pequeno (1 épico), não criar milestones
	if len(backlog.Epics) < 2 {
		return milestones
	}
	
	// Para 2 épicos: criar 2 milestones simples (MVP e V1.0)
	if len(backlog.Epics) == 2 {
		// M1: Primeiro épico
		epic1 := backlog.Epics[0]
		epic1Effort := effort.ByEpic[epic1.ID]
		
		milestones = append(milestones, Milestone{
			ID:           "M1",
			Title:        "MVP - " + epic1.Title,
			Description:  "Primeira entrega funcional",
			EpicIDs:      []string{epic1.ID},
			TotalSPs:     epic1Effort.SPs,
			DaysEstimate: epic1Effort.Days,
			ValueProp:    "Validar viabilidade técnica do sistema",
		})
		
		// M2: Ambos épicos (versão completa)
		milestones = append(milestones, Milestone{
			ID:           "M2",
			Title:        "V1.0 - Produto Completo",
			Description:  "Sistema completo com todas funcionalidades",
			EpicIDs:      []string{backlog.Epics[0].ID, backlog.Epics[1].ID},
			TotalSPs:     effort.TotalSPs,
			DaysEstimate: effort.TotalDays,
			ValueProp:    "Produto pronto para uso em produção",
		})
		
		return milestones
	}
	
	// Estratégia para 3+ épicos: agrupar épicos em fases
	// MVP = primeiros épicos até ~40% dos SPs
	// Beta = até ~70% dos SPs
	// V1.0 = 100%
	
	totalSPs := effort.TotalSPs
	
	// Ordenar épicos por ordem (assumindo que IDs são sequenciais E1, E2, E3...)
	epicsSorted := make([]types.Epic, len(backlog.Epics))
	copy(epicsSorted, backlog.Epics)
	
	// MVP: primeiros épicos até 40%
	mvpSPs := 0
	mvpEpics := []string{}
	mvpThreshold := int(float64(totalSPs) * 0.4)
	
	for _, epic := range epicsSorted {
		if mvpSPs >= mvpThreshold {
			break
		}
		epicEffort := effort.ByEpic[epic.ID]
		mvpSPs += epicEffort.SPs
		mvpEpics = append(mvpEpics, epic.ID)
	}
	
	if len(mvpEpics) > 0 {
		mvpDays := (mvpSPs + 1) / 2
		milestones = append(milestones, Milestone{
			ID:           "M1",
			Title:        "MVP",
			Description:  "Versão mínima viável do sistema",
			EpicIDs:      mvpEpics,
			TotalSPs:     mvpSPs,
			DaysEstimate: mvpDays,
			ValueProp:    "Sistema funcional básico com features essenciais",
		})
	}
	
	// Beta: até 70%
	betaSPs := 0
	betaEpics := []string{}
	betaThreshold := int(float64(totalSPs) * 0.7)
	
	for _, epic := range epicsSorted {
		if betaSPs >= betaThreshold {
			break
		}
		epicEffort := effort.ByEpic[epic.ID]
		betaSPs += epicEffort.SPs
		betaEpics = append(betaEpics, epic.ID)
	}
	
	if len(betaEpics) > len(mvpEpics) {
		betaDays := (betaSPs + 1) / 2
		milestones = append(milestones, Milestone{
			ID:           "M2",
			Title:        "Beta",
			Description:  "Sistema completo pronto para validação",
			EpicIDs:      betaEpics,
			TotalSPs:     betaSPs,
			DaysEstimate: betaDays,
			ValueProp:    "MVP + features complementares e integrações",
		})
	}
	
	// V1.0: todos os épicos
	allEpics := []string{}
	for _, epic := range backlog.Epics {
		allEpics = append(allEpics, epic.ID)
	}
	
	if len(allEpics) > len(betaEpics) {
		milestones = append(milestones, Milestone{
			ID:           "M3",
			Title:        "V1.0",
			Description:  "Sistema production-ready completo",
			EpicIDs:      allEpics,
			TotalSPs:     totalSPs,
			DaysEstimate: effort.TotalDays,
			ValueProp:    "Sistema completo com todas as features planejadas",
		})
	}
	
	return milestones
}
