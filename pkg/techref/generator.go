package techref

import (
	"fmt"
	"log"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// GenerationConfig configura o processo de geração
type GenerationConfig struct {
	// Configuração do classificador
	ClassifierConfig ClassifierConfig

	// Habilitar logs detalhados
	Verbose bool

	// Dry-run: não chama LLM, apenas simula
	DryRun bool

	// LLMCaller: função para chamar a LLM (injetada)
	LLMCaller func(prompt string) (string, error)

	// EnableBatching: agrupar múltiplas techs num único prompt LLM (default: true)
	EnableBatching bool

	// TemplateOnly: quando true, usa apenas templates (zero-LLM). Se nenhum template
	// existir para a tech, retorna erro ao inves de chamar LLM.
	TemplateOnly bool

	// Registry: referência ao TechRegistry para template-first generation.
	// Quando set, tenta gerar a partir de templates antes de chamar LLM.
	Registry *TechRegistry
}

// GenerationMetrics guarda métricas da geração
type GenerationMetrics struct {
	// Extração
	TotalTechsExtracted int
	TrivialFiltered     int

	// Classificação
	TrivialCount  int
	StandardCount int
	SpecificCount int
	CriticalCount int

	// Cross-epic deduplication
	CrossEpicGlobalDives int // Techs shared across 2+ epics → 1 global DD
	CrossEpicDeduplicated int // Redundant epic-level DDs avoided

	// Deep Dives Gerados
	EpicLevelDives  int
	StoryLevelDives int
	TotalDives      int

	// Performance
	LLMCallsMade     int
	LLMCallsSaved    int
	ReductionPercent float64
}

// GenerationResult resultado completo da geração
type GenerationResult struct {
	DeepDives []types.DeepDive
	Metrics   GenerationMetrics
	Errors    []error
}

// crossEpicTech tracks a technology found in multiple epics
type crossEpicTech struct {
	Term         string
	EpicIDs      []string
	TotalStories int
	BestScope    string // "global" if 2+ epics, "epic" if 1
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// BACKWARD COMPAT WRAPPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// GenerateDeepDivesOptimized gera deep dives usando a estratégia otimizada (backward compat)
func GenerateDeepDivesOptimized(backlog types.Backlog, config GenerationConfig) GenerationResult {
	return GenerateDeepDivesOptimizedWithRegistry(DefaultRegistry(), backlog, config)
}

// calculateOldApproachCalls backward compat wrapper
func calculateOldApproachCalls(backlog types.Backlog) int {
	return calculateOldApproachCallsWithRegistry(DefaultRegistry(), backlog)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// REGISTRY-BASED IMPLEMENTATION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// GenerateDeepDivesOptimizedWithRegistry gera deep dives usando registry configurável
func GenerateDeepDivesOptimizedWithRegistry(reg *TechRegistry, backlog types.Backlog, config GenerationConfig) GenerationResult {
	if reg == nil {
		reg = DefaultRegistry()
	}
	config.Registry = reg

	result := GenerationResult{
		DeepDives: []types.DeepDive{},
		Errors:    []error{},
	}

	if config.Verbose {
		log.Println("🚀 Iniciando geração otimizada de deep dives")
	}

	// FASE 0: Cross-epic scan — identify techs shared across multiple epics
	// Also caches extraction results to avoid re-extracting in FASE 2
	extractionCache := make(map[string][]TechExtraction) // epicID → extractions
	globalMap := buildCrossEpicMapCached(reg, backlog, extractionCache)
	globalGenerated := make(map[string]bool) // track already-generated global DDs

	if config.Verbose && len(globalMap) > 0 {
		log.Printf("🌍 Cross-epic scan: %d techs encontradas em 2+ épicos", len(globalMap))
		for term, info := range globalMap {
			log.Printf("   🔗 %s → %d épicos, %d histórias", term, len(info.EpicIDs), info.TotalStories)
		}
	}

	// FASE 1: Generate global deep dives (techs in 2+ epics → 1 DD)
	if config.EnableBatching && len(globalMap) > 1 {
		// Batch: todas as global techs num único LLM call
		crossTechList := make([]*crossEpicTech, 0, len(globalMap))
		for _, ct := range globalMap {
			crossTechList = append(crossTechList, ct)
		}

		if config.Verbose {
			log.Printf("🌍 Batch global: %d techs cross-epic em 1 LLM call", len(crossTechList))
		}

		dives, err := batchGenerateGlobalDDs(backlog, crossTechList, config)
		if err != nil {
			if config.Verbose {
				log.Printf("   ❌ Erro no batch global: %v — fallback individual", err)
			}
			result.Errors = append(result.Errors, err)
		} else {
			result.DeepDives = append(result.DeepDives, dives...)
			result.Metrics.CrossEpicGlobalDives += len(dives)
			result.Metrics.EpicLevelDives += len(dives)
			result.Metrics.LLMCallsMade++
			for _, d := range dives {
				globalGenerated[d.Term] = true
			}
		}
	} else {
		// Individual: 1 LLM call por global tech
		for term, crossTech := range globalMap {
			if config.Verbose {
				log.Printf("🌍 Gerando deep dive global: %s (%d épicos)", term, len(crossTech.EpicIDs))
			}

			dive, err := generateGlobalDeepDive(backlog, crossTech, config)
			if err != nil {
				if config.Verbose {
					log.Printf("   ❌ Erro ao gerar deep dive global para %s: %v", term, err)
				}
				result.Errors = append(result.Errors, err)
				continue
			}

			result.DeepDives = append(result.DeepDives, dive)
			result.Metrics.CrossEpicGlobalDives++
			result.Metrics.EpicLevelDives++
			result.Metrics.LLMCallsMade++
			globalGenerated[term] = true
		}
	}

	// FASE 2: Process each epic (excluding global techs)
	for _, epic := range backlog.Epics {
		if config.Verbose {
			log.Printf("📦 Processando épico: %s - %s", epic.ID, epic.Title)
		}

		// 1. Extrair tecnologias do épico (use cache from cross-epic scan)
		extractions, cached := extractionCache[epic.ID]
		if !cached {
			extractions = ExtractTechsByEpicWithRegistry(reg, epic)
		}
		result.Metrics.TotalTechsExtracted += len(extractions)

		if config.Verbose {
			log.Printf("   Extraídas %d tecnologias", len(extractions))
		}

		// 2. Filtrar triviais
		nonTrivial := []TechExtraction{}
		for _, ext := range extractions {
			if reg.IsTrivial(ext.Term) {
				result.Metrics.TrivialFiltered++
				if config.Verbose {
					log.Printf("   ⏭️  Filtrado (trivial): %s", ext.Term)
				}
			} else {
				nonTrivial = append(nonTrivial, ext)
			}
		}

		if config.Verbose {
			log.Printf("   %d tecnologias após filtro trivial", len(nonTrivial))
		}

		// 3. Classificar cada tecnologia (skip globals already generated)
		classifications := []TechClassification{}
		for _, ext := range nonTrivial {
			// Skip techs already handled as global cross-epic DDs
			if globalGenerated[ext.Term] {
				result.Metrics.CrossEpicDeduplicated++
				if config.Verbose {
					log.Printf("   ⏭️  Skip (global): %s", ext.Term)
				}
				continue
			}

			classification := ClassifyTechWithRegistry(reg, ext.Term, epic, config.ClassifierConfig)
			classifications = append(classifications, classification)

			// Contar por tipo
			switch classification.Relevance {
			case TRIVIAL:
				result.Metrics.TrivialCount++
			case STANDARD:
				result.Metrics.StandardCount++
			case SPECIFIC:
				result.Metrics.SpecificCount++
			case CRITICAL:
				result.Metrics.CriticalCount++
			}

			if config.Verbose {
				log.Printf("   🏷️  %s → %s (%s)", ext.Term, classification.Relevance, classification.Reason)
			}
		}

		// 3.5. Consolidar por tech group (PostgreSQL + pgx → 1 DD)
		consolidated := consolidateByTechGroup(reg, classifications)
		if config.Verbose && len(consolidated) < len(classifications) {
			log.Printf("   🔗 Tech groups: %d → %d (consolidado %d)",
				len(classifications), len(consolidated), len(classifications)-len(consolidated))
		}

		// 4. Gerar deep dives baseado em classificação
		epicDives, epicErrors := generateDeepDivesForEpic(epic, consolidated, config, &result.Metrics)
		result.DeepDives = append(result.DeepDives, epicDives...)
		result.Errors = append(result.Errors, epicErrors...)
	}

	// FASE 3: Calcular métricas finais
	result.Metrics.TotalDives = len(result.DeepDives)

	// Calcular economia baseada em extração real
	oldApproach := calculateOldApproachCallsWithRegistry(reg, backlog)
	result.Metrics.LLMCallsSaved = oldApproach - result.Metrics.LLMCallsMade
	if oldApproach > 0 {
		result.Metrics.ReductionPercent = float64(result.Metrics.LLMCallsSaved) / float64(oldApproach) * 100
	}

	if config.Verbose {
		printMetrics(result.Metrics, oldApproach)
	}

	return result
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CROSS-EPIC DEDUPLICATION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// buildCrossEpicMapCached scans all epics, identifies techs shared across 2+ epics,
// and caches extraction results per epic to avoid re-extracting later.
func buildCrossEpicMapCached(reg *TechRegistry, backlog types.Backlog, cache map[string][]TechExtraction) map[string]*crossEpicTech {
	// First pass: collect all techs per epic (and cache extractions)
	techEpics := make(map[string]map[string]bool) // tech → set of epicIDs
	techStories := make(map[string]int)            // tech → total story count

	for _, epic := range backlog.Epics {
		extractions := ExtractTechsByEpicWithRegistry(reg, epic)
		cache[epic.ID] = extractions // cache for reuse in FASE 2

		for _, ext := range extractions {
			if reg.IsTrivial(ext.Term) {
				continue
			}
			if _, ok := techEpics[ext.Term]; !ok {
				techEpics[ext.Term] = make(map[string]bool)
			}
			techEpics[ext.Term][epic.ID] = true
			techStories[ext.Term] += ext.Count
		}
	}

	// Second pass: keep only techs in 2+ epics
	globalMap := make(map[string]*crossEpicTech)
	for term, epicSet := range techEpics {
		if len(epicSet) >= 2 {
			epicIDs := make([]string, 0, len(epicSet))
			for id := range epicSet {
				epicIDs = append(epicIDs, id)
			}
			globalMap[term] = &crossEpicTech{
				Term:         term,
				EpicIDs:      epicIDs,
				TotalStories: techStories[term],
				BestScope:    "global",
			}
		}
	}

	return globalMap
}

// generateGlobalDeepDive generates a single DD for a tech spanning multiple epics
func generateGlobalDeepDive(backlog types.Backlog, crossTech *crossEpicTech, config GenerationConfig) (types.DeepDive, error) {
	// Template-first: try pre-defined template before LLM.
	if config.Registry != nil {
		vars := TemplateVars{ProjectContext: backlog.Meta.ProjectTitle, Scope: "global"}
		if dive, ok := GenerateFromTemplate(config.Registry, crossTech.Term, "global", vars); ok {
			dive.Classification = "standard"
			return dive, nil
		}
		if config.TemplateOnly {
			return types.DeepDive{}, fmt.Errorf("no template for %q and TemplateOnly=true", crossTech.Term)
		}
	}

	prompt := buildGlobalPrompt(backlog, crossTech)

	if config.DryRun {
		return types.DeepDive{
			Term:           crossTech.Term,
			WhatIs:         fmt.Sprintf("[DRY-RUN] Global deep dive for %s across %d epics", crossTech.Term, len(crossTech.EpicIDs)),
			Classification: "standard",
			Scope:          "global",
		}, nil
	}

	if config.LLMCaller == nil {
		return types.DeepDive{}, fmt.Errorf("LLMCaller não configurado")
	}

	response, err := config.LLMCaller(prompt)
	if err != nil {
		return types.DeepDive{}, err
	}

	dive := parseDeepDiveResponse(response, crossTech.Term, "", "")
	dive.Classification = "standard"
	dive.Scope = "global"
	return dive, nil
}

// buildGlobalPrompt builds prompt for cross-epic global deep dive
func buildGlobalPrompt(backlog types.Backlog, crossTech *crossEpicTech) string {
	epicContext := ""
	epicSet := make(map[string]bool)
	for _, id := range crossTech.EpicIDs {
		epicSet[id] = true
	}

	for _, epic := range backlog.Epics {
		if !epicSet[epic.ID] {
			continue
		}
		epicContext += fmt.Sprintf("\n\nÉpico %s - %s:", epic.ID, epic.Title)
		for i, story := range epic.Stories {
			epicContext += fmt.Sprintf("\n  %d. %s - %s: %s", i+1, story.ID, story.Title, story.What)
		}
	}

	return fmt.Sprintf(`Explique a tecnologia "%s" no contexto GLOBAL do projeto.

Esta tecnologia aparece em %d épicos (%d histórias total):%s

Forneça:
1. O que é %s
2. Por que esta tecnologia é central para este projeto
3. Como ela conecta os diferentes épicos
4. Configuração e boas práticas para este contexto
5. Decisões arquiteturais relevantes

Considere o uso cross-cutting desta tecnologia.`,
		crossTech.Term,
		len(crossTech.EpicIDs),
		crossTech.TotalStories,
		epicContext,
		crossTech.Term,
	)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EPIC/STORY LEVEL GENERATION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// generateDeepDivesForEpic gera deep dives para um épico
func generateDeepDivesForEpic(epic types.Epic, classifications []TechClassification, config GenerationConfig, metrics *GenerationMetrics) ([]types.DeepDive, []error) {
	dives := []types.DeepDive{}
	var errs []error

	// Agrupar por scope
	grouped := groupByScope(classifications)

	// 1. Gerar deep dives epic-level (STANDARD + CRITICAL)
	epicLevelClassifications := append(grouped["epic"], grouped["none"]...)
	var epicToGenerate []TechClassification
	for _, c := range epicLevelClassifications {
		if shouldGenerateDeepDive(c) {
			epicToGenerate = append(epicToGenerate, c)
		}
	}

	if config.EnableBatching && len(epicToGenerate) > 1 {
		// Batch: todas as techs epic-level num único LLM call
		batchDives, batchErrs := batchGenerateDeepDivesForEpic(epic, epicToGenerate, config, metrics)
		dives = append(dives, batchDives...)
		errs = append(errs, batchErrs...)
	} else {
		// Individual: 1 LLM call por tech
		for _, classification := range epicToGenerate {
			dive, err := generateEpicLevelDeepDive(epic, classification, config)
			if err != nil {
				if config.Verbose {
					log.Printf("   ❌ Erro ao gerar deep dive épico para %s: %v", classification.Term, err)
				}
				errs = append(errs, err)
				continue
			}

			dives = append(dives, dive)
			metrics.EpicLevelDives++
			metrics.LLMCallsMade++

			if config.Verbose {
				log.Printf("   ✅ Deep dive epic-level: %s", classification.Term)
			}
		}
	}

	// 2. Gerar deep dives story-level (SPECIFIC)
	// Story-level: mantém individual (contexto é específico por história)
	for _, classification := range grouped["story"] {
		if !shouldGenerateDeepDive(classification) {
			continue
		}

		// Encontrar a história
		var story types.Story
		for _, s := range epic.Stories {
			if s.ID == classification.StoryID {
				story = s
				break
			}
		}

		dive, err := generateStoryLevelDeepDive(story, classification, config)
		if err != nil {
			if config.Verbose {
				log.Printf("   ❌ Erro ao gerar deep dive story para %s: %v", classification.Term, err)
			}
			errs = append(errs, err)
			continue
		}

		dives = append(dives, dive)
		metrics.StoryLevelDives++
		metrics.LLMCallsMade++

		if config.Verbose {
			log.Printf("   ✅ Deep dive story-level: %s (história %s)", classification.Term, story.ID)
		}
	}

	return dives, errs
}

// generateEpicLevelDeepDive gera deep dive no contexto do épico
func generateEpicLevelDeepDive(epic types.Epic, classification TechClassification, config GenerationConfig) (types.DeepDive, error) {
	// Template-first: try pre-defined template before LLM.
	if config.Registry != nil {
		vars := TemplateVars{EpicTitle: epic.Title, EpicID: epic.ID, Scope: classification.Scope}
		if dive, ok := GenerateFromTemplate(config.Registry, classification.Term, classification.Scope, vars); ok {
			dive.Classification = string(classification.Relevance)
			return dive, nil
		}
		if config.TemplateOnly {
			return types.DeepDive{}, fmt.Errorf("no template for %q and TemplateOnly=true", classification.Term)
		}
	}

	prompt := buildEpicPrompt(epic, classification)

	if config.DryRun {
		return types.DeepDive{
			Term:           classification.Term,
			WhatIs:         fmt.Sprintf("[DRY-RUN] Deep dive for %s in epic %s", classification.Term, epic.ID),
			Classification: string(classification.Relevance),
			Scope:          classification.Scope,
		}, nil
	}

	if config.LLMCaller == nil {
		return types.DeepDive{}, fmt.Errorf("LLMCaller não configurado")
	}

	response, err := config.LLMCaller(prompt)
	if err != nil {
		return types.DeepDive{}, err
	}

	dive := parseDeepDiveResponse(response, classification.Term, classification.EpicID, "")
	dive.Classification = string(classification.Relevance)
	dive.Scope = classification.Scope
	return dive, nil
}

// generateStoryLevelDeepDive gera deep dive no contexto da história
func generateStoryLevelDeepDive(story types.Story, classification TechClassification, config GenerationConfig) (types.DeepDive, error) {
	// Template-first: try pre-defined template before LLM.
	if config.Registry != nil {
		vars := TemplateVars{StoryTitle: story.Title, StoryID: story.ID, EpicID: classification.EpicID, Scope: classification.Scope}
		if dive, ok := GenerateFromTemplate(config.Registry, classification.Term, classification.Scope, vars); ok {
			dive.Classification = string(classification.Relevance)
			dive.StoryID = story.ID
			return dive, nil
		}
		if config.TemplateOnly {
			return types.DeepDive{}, fmt.Errorf("no template for %q and TemplateOnly=true", classification.Term)
		}
	}

	prompt := buildStoryPrompt(story, classification)

	if config.DryRun {
		return types.DeepDive{
			Term:           classification.Term,
			StoryID:        story.ID,
			WhatIs:         fmt.Sprintf("[DRY-RUN] Deep dive for %s in story %s", classification.Term, story.ID),
			Classification: string(classification.Relevance),
			Scope:          classification.Scope,
		}, nil
	}

	if config.LLMCaller == nil {
		return types.DeepDive{}, fmt.Errorf("LLMCaller não configurado")
	}

	response, err := config.LLMCaller(prompt)
	if err != nil {
		return types.DeepDive{}, err
	}

	dive := parseDeepDiveResponse(response, classification.Term, classification.EpicID, story.ID)
	dive.Classification = string(classification.Relevance)
	dive.Scope = classification.Scope
	return dive, nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PROMPTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// buildEpicPrompt constrói prompt para deep dive epic-level
func buildEpicPrompt(epic types.Epic, classification TechClassification) string {
	storiesContext := ""
	for i, story := range epic.Stories {
		storiesContext += fmt.Sprintf("\n%d. %s - %s: %s", i+1, story.ID, story.Title, story.What)
	}

	return fmt.Sprintf(`Explique a tecnologia "%s" no contexto do épico "%s - %s".

Este épico contém as seguintes histórias:%s

Forneça:
1. O que é %s
2. Por que usamos aqui (no contexto deste épico)
3. Como configurar/usar
4. Padrões e boas práticas
5. Decisões técnicas relevantes

Seja específico para este contexto, não genérico.`,
		classification.Term,
		epic.ID,
		epic.Title,
		storiesContext,
		classification.Term,
	)
}

// buildStoryPrompt constrói prompt para deep dive story-level
func buildStoryPrompt(story types.Story, classification TechClassification) string {
	criteria := ""
	for _, c := range story.AcceptanceCriteria {
		criteria += fmt.Sprintf("\n- %s", c)
	}

	return fmt.Sprintf(`Explique a tecnologia "%s" especificamente no contexto da história "%s - %s".

O que fazer: %s
Por que: %s
Critérios de aceite:%s

Explique:
1. O que é %s neste contexto específico
2. Por que é necessário para ESTA história
3. Como usar/configurar especificamente aqui
4. Padrões relevantes para este caso de uso
5. Decisões técnicas desta implementação específica

Foque no uso ESPECÍFICO desta história, não no geral.`,
		classification.Term,
		story.ID,
		story.Title,
		story.What,
		story.Why,
		criteria,
		classification.Term,
	)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// BATCHING — Agrupa múltiplas techs em 1 LLM call
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// buildBatchEpicPrompt constrói prompt para múltiplos deep dives de um épico
func buildBatchEpicPrompt(epic types.Epic, classifications []TechClassification) string {
	storiesContext := ""
	for i, story := range epic.Stories {
		storiesContext += fmt.Sprintf("\n%d. %s - %s: %s", i+1, story.ID, story.Title, story.What)
	}

	techList := ""
	for i, c := range classifications {
		techList += fmt.Sprintf("\n### %d. %s (%s)", i+1, c.Term, string(c.Relevance))
	}

	return fmt.Sprintf(`Para o épico "%s - %s", gere deep dives para CADA tecnologia listada abaixo.

Histórias do épico:%s

Tecnologias (gere 1 deep dive por tech):%s

Para CADA tecnologia, forneça:
1. **O que é** — explicação no contexto deste épico
2. **Por que aqui** — justificativa específica
3. **Como configurar** — setup e boas práticas
4. **Padrões** — patterns recomendados
5. **Decisões** — trade-offs relevantes

Separe cada tecnologia com "---" entre elas.
Use exatamente o formato: ### {Nome da Tecnologia}`,
		epic.ID, epic.Title, storiesContext, techList)
}

// buildBatchGlobalPrompt constrói prompt batched para techs cross-epic
func buildBatchGlobalPrompt(backlog types.Backlog, crossTechs []*crossEpicTech) string {
	techList := ""
	for i, ct := range crossTechs {
		techList += fmt.Sprintf("\n### %d. %s (aparece em %d épicos, %d histórias)",
			i+1, ct.Term, len(ct.EpicIDs), ct.TotalStories)
	}

	epicContext := ""
	for _, epic := range backlog.Epics {
		epicContext += fmt.Sprintf("\n\nÉpico %s - %s:", epic.ID, epic.Title)
		for i, story := range epic.Stories {
			epicContext += fmt.Sprintf("\n  %d. %s - %s: %s", i+1, story.ID, story.Title, story.What)
		}
	}

	return fmt.Sprintf(`Gere deep dives GLOBAIS para cada tecnologia cross-cutting abaixo.
Estas tecnologias aparecem em múltiplos épicos do projeto.

Tecnologias:%s

Contexto do projeto:%s

Para CADA tecnologia, forneça:
1. **O que é** — explicação no contexto GLOBAL do projeto
2. **Por que é central** — como conecta os diferentes épicos
3. **Como configurar** — setup e boas práticas
4. **Padrões** — patterns recomendados
5. **Decisões** — trade-offs arquiteturais

Separe cada tecnologia com "---" entre elas.
Use exatamente o formato: ### {Nome da Tecnologia}`, techList, epicContext)
}

// parseBatchResponse parseia resposta com múltiplos deep dives separados por ---
func parseBatchResponse(response string, classifications []TechClassification, epicID string) []types.DeepDive {
	dives := make([]types.DeepDive, 0, len(classifications))

	// Tentar split por "---" ou "### N." patterns
	sections := splitBatchResponse(response, classifications)

	for i, c := range classifications {
		content := response // fallback: resposta inteira
		if i < len(sections) && sections[i] != "" {
			content = sections[i]
		}

		dive := types.DeepDive{
			Term:           c.Term,
			WhatIs:         strings.TrimSpace(content),
			Classification: string(c.Relevance),
			Scope:          c.Scope,
		}
		if c.StoryID != "" {
			dive.StoryID = c.StoryID
		}

		dives = append(dives, dive)
	}

	return dives
}

// splitBatchResponse divide resposta em seções por tecnologia
func splitBatchResponse(response string, classifications []TechClassification) []string {
	sections := make([]string, len(classifications))

	// Estratégia 1: split por "### {Term}"
	for i, c := range classifications {
		// Procurar pela seção desta tech
		marker := "### " + c.Term
		idx := strings.Index(response, marker)
		if idx == -1 {
			// Tentar case-insensitive
			lowerResp := strings.ToLower(response)
			lowerMarker := strings.ToLower(marker)
			idx = strings.Index(lowerResp, lowerMarker)
		}

		if idx >= 0 {
			// Encontrar o fim da seção (próximo "###" ou "---" ou fim)
			sectionStart := idx + len(marker)
			rest := response[sectionStart:]

			endIdx := len(rest)
			// Procurar próximo delimitador
			for _, delim := range []string{"### ", "---"} {
				delimIdx := strings.Index(rest, delim)
				if delimIdx > 0 && delimIdx < endIdx {
					endIdx = delimIdx
				}
			}

			sections[i] = strings.TrimSpace(rest[:endIdx])
		}
	}

	// Estratégia 2: se nenhuma seção foi encontrada, split por "---"
	allEmpty := true
	for _, s := range sections {
		if s != "" {
			allEmpty = false
			break
		}
	}

	if allEmpty {
		parts := strings.Split(response, "---")
		for i, part := range parts {
			if i < len(sections) {
				sections[i] = strings.TrimSpace(part)
			}
		}
	}

	return sections
}

// consolidateByTechGroup consolida classificações de techs da mesma família.
// Se PostgreSQL e pgx pertencem ao grupo "PostgreSQL Ecosystem",
// mantém apenas o primary (PostgreSQL) com nota sobre members usados.
func consolidateByTechGroup(reg *TechRegistry, classifications []TechClassification) []TechClassification {
	if reg == nil || len(reg.TechGroups) == 0 {
		return classifications
	}

	// Agrupar por grupo
	groupSeen := make(map[string]int) // group name → index na saída
	consolidated := make([]TechClassification, 0, len(classifications))

	for _, c := range classifications {
		group := reg.FindGroup(c.Term)
		if group == nil {
			// Sem grupo — manter como está
			consolidated = append(consolidated, c)
			continue
		}

		if idx, exists := groupSeen[group.Name]; exists {
			// Grupo já tem representante — adicionar info ao reason
			consolidated[idx].Reason += fmt.Sprintf(" (+%s)", c.Term)
		} else {
			// Primeiro membro do grupo — usar o primary como representante
			entry := c
			if !strings.EqualFold(c.Term, group.Primary) {
				entry.Term = group.Primary
				entry.Reason = fmt.Sprintf("Consolidado (grupo %s, inclui %s)", group.Name, c.Term)
			}
			groupSeen[group.Name] = len(consolidated)
			consolidated = append(consolidated, entry)
		}
	}

	return consolidated
}

// batchGenerateDeepDivesForEpic gera múltiplos deep dives num único LLM call
func batchGenerateDeepDivesForEpic(epic types.Epic, classifications []TechClassification, config GenerationConfig, metrics *GenerationMetrics) ([]types.DeepDive, []error) {
	// Template-first: resolve what we can from templates, batch only the rest.
	if config.Registry != nil {
		var templated []types.DeepDive
		var remaining []TechClassification
		vars := TemplateVars{EpicTitle: epic.Title, EpicID: epic.ID}
		for _, c := range classifications {
			if dive, ok := GenerateFromTemplate(config.Registry, c.Term, c.Scope, vars); ok {
				dive.Classification = string(c.Relevance)
				templated = append(templated, dive)
				metrics.EpicLevelDives++
			} else {
				remaining = append(remaining, c)
			}
		}
		if len(remaining) == 0 {
			return templated, nil
		}
		if config.TemplateOnly {
			var errs []error
			for _, c := range remaining {
				errs = append(errs, fmt.Errorf("no template for %q and TemplateOnly=true", c.Term))
			}
			return templated, errs
		}
		// Continue with remaining via LLM batch, prepending templated results.
		batchDives, batchErrs := batchGenerateDeepDivesForEpicLLM(epic, classifications, config, metrics)
		return append(templated, batchDives...), batchErrs
	}

	return batchGenerateDeepDivesForEpicLLM(epic, classifications, config, metrics)
}

// batchGenerateDeepDivesForEpicLLM is the LLM-only batch path.
func batchGenerateDeepDivesForEpicLLM(epic types.Epic, classifications []TechClassification, config GenerationConfig, metrics *GenerationMetrics) ([]types.DeepDive, []error) {
	if config.DryRun {
		dives := make([]types.DeepDive, 0, len(classifications))
		for _, c := range classifications {
			dives = append(dives, types.DeepDive{
				Term:           c.Term,
				WhatIs:         fmt.Sprintf("[DRY-RUN] Batched deep dive for %s in epic %s", c.Term, epic.ID),
				Classification: string(c.Relevance),
				Scope:          c.Scope,
			})
		}
		metrics.LLMCallsMade++
		metrics.EpicLevelDives += len(classifications)
		return dives, nil
	}

	if config.LLMCaller == nil {
		return nil, []error{fmt.Errorf("LLMCaller não configurado")}
	}

	prompt := buildBatchEpicPrompt(epic, classifications)
	response, err := config.LLMCaller(prompt)
	if err != nil {
		// Fallback: tentar gerar individualmente
		if config.Verbose {
			log.Printf("   ⚠️  Batch falhou, tentando individual: %v", err)
		}
		return generateDeepDivesForEpic(epic, classifications, config, metrics)
	}

	dives := parseBatchResponse(response, classifications, epic.ID)
	metrics.LLMCallsMade++
	metrics.EpicLevelDives += len(dives)

	if config.Verbose {
		log.Printf("   ✅ Batch: %d deep dives em 1 LLM call (épico %s)", len(dives), epic.ID)
	}

	return dives, nil
}

// batchGenerateGlobalDDs gera múltiplos deep dives globais num único LLM call
func batchGenerateGlobalDDs(backlog types.Backlog, crossTechs []*crossEpicTech, config GenerationConfig) ([]types.DeepDive, error) {
	// Template-first: resolve what we can from templates, batch only the rest.
	if config.Registry != nil {
		var templated []types.DeepDive
		var remaining []*crossEpicTech
		vars := TemplateVars{ProjectContext: backlog.Meta.ProjectTitle, Scope: "global"}
		for _, ct := range crossTechs {
			if dive, ok := GenerateFromTemplate(config.Registry, ct.Term, "global", vars); ok {
				dive.Classification = "standard"
				templated = append(templated, dive)
			} else {
				remaining = append(remaining, ct)
			}
		}
		if len(remaining) == 0 {
			return templated, nil
		}
		if config.TemplateOnly {
			return templated, fmt.Errorf("no template for %d techs and TemplateOnly=true", len(remaining))
		}
		crossTechs = remaining
		batchDives, err := batchGenerateGlobalDDsLLM(backlog, crossTechs, config)
		return append(templated, batchDives...), err
	}

	return batchGenerateGlobalDDsLLM(backlog, crossTechs, config)
}

// batchGenerateGlobalDDsLLM is the LLM-only batch path.
func batchGenerateGlobalDDsLLM(backlog types.Backlog, crossTechs []*crossEpicTech, config GenerationConfig) ([]types.DeepDive, error) {
	if config.DryRun {
		dives := make([]types.DeepDive, 0, len(crossTechs))
		for _, ct := range crossTechs {
			dives = append(dives, types.DeepDive{
				Term:           ct.Term,
				WhatIs:         fmt.Sprintf("[DRY-RUN] Batched global deep dive for %s across %d epics", ct.Term, len(ct.EpicIDs)),
				Classification: "standard",
				Scope:          "global",
			})
		}
		return dives, nil
	}

	if config.LLMCaller == nil {
		return nil, fmt.Errorf("LLMCaller não configurado")
	}

	prompt := buildBatchGlobalPrompt(backlog, crossTechs)
	response, err := config.LLMCaller(prompt)
	if err != nil {
		return nil, err
	}

	// Criar classifications fake para o parser
	classifications := make([]TechClassification, 0, len(crossTechs))
	for _, ct := range crossTechs {
		classifications = append(classifications, TechClassification{
			Term:      ct.Term,
			Relevance: STANDARD,
			Scope:     "global",
		})
	}

	dives := parseBatchResponse(response, classifications, "")
	// Marcar como global
	for i := range dives {
		dives[i].Scope = "global"
	}
	return dives, nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// parseDeepDiveResponse parseia resposta da LLM em DeepDive
func parseDeepDiveResponse(response, term, epicID, storyID string) types.DeepDive {
	return types.DeepDive{
		Term:    term,
		StoryID: storyID,
		WhatIs:  response,
	}
}

// calculateOldApproachCallsWithRegistry calcula chamadas reais baseado em extração
func calculateOldApproachCallsWithRegistry(reg *TechRegistry, backlog types.Backlog) int {
	totalCalls := 0

	for _, epic := range backlog.Epics {
		for _, story := range epic.Stories {
			// Na abordagem antiga: 1 deep dive por tech por história
			techs := ExtractTechsFromStoryWithRegistry(reg, story)
			nonTrivial := reg.FilterTrivialTerms(techs)
			totalCalls += len(nonTrivial)
		}
	}

	// Fallback: se extração retorna 0, usar estimativa conservadora
	if totalCalls == 0 {
		for _, epic := range backlog.Epics {
			totalCalls += len(epic.Stories) * 6
		}
	}

	return totalCalls
}

// printMetrics imprime métricas de forma legível
func printMetrics(metrics GenerationMetrics, oldApproachCalls int) {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 MÉTRICAS DE GERAÇÃO")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n🔍 Extração:")
	fmt.Printf("  Tecnologias extraídas: %d\n", metrics.TotalTechsExtracted)
	fmt.Printf("  Triviais filtradas: %d\n", metrics.TrivialFiltered)

	fmt.Println("\n🏷️  Classificação:")
	fmt.Printf("  TRIVIAL:  %d (skip)\n", metrics.TrivialCount)
	fmt.Printf("  STANDARD: %d (epic-level)\n", metrics.StandardCount)
	fmt.Printf("  SPECIFIC: %d (story-level)\n", metrics.SpecificCount)
	fmt.Printf("  CRITICAL: %d (epic-level detalhado)\n", metrics.CriticalCount)

	if metrics.CrossEpicGlobalDives > 0 || metrics.CrossEpicDeduplicated > 0 {
		fmt.Println("\n🌍 Cross-Epic:")
		fmt.Printf("  Global DDs:     %d\n", metrics.CrossEpicGlobalDives)
		fmt.Printf("  Deduplicated:   %d\n", metrics.CrossEpicDeduplicated)
	}

	fmt.Println("\n📝 Deep Dives Gerados:")
	fmt.Printf("  Epic-level:  %d\n", metrics.EpicLevelDives)
	fmt.Printf("  Story-level: %d\n", metrics.StoryLevelDives)
	fmt.Printf("  TOTAL:       %d\n", metrics.TotalDives)

	fmt.Println("\n⚡ Performance:")
	fmt.Printf("  Abordagem antiga: ~%d chamadas LLM\n", oldApproachCalls)
	fmt.Printf("  Abordagem nova:   %d chamadas LLM\n", metrics.LLMCallsMade)
	fmt.Printf("  Economia:         %d chamadas (%.1f%%)\n",
		metrics.LLMCallsSaved,
		metrics.ReductionPercent)

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// GetDefaultGenerationConfig retorna configuração padrão
func GetDefaultGenerationConfig() GenerationConfig {
	return GenerationConfig{
		ClassifierConfig: DefaultClassifierConfig(),
		Verbose:          false,
		DryRun:           false,
		LLMCaller:        nil, // Deve ser injetado
		EnableBatching:   true,
	}
}
