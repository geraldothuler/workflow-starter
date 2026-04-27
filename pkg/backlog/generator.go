package backlog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/critical_path"
	"github.com/Cobliteam/workflow-toolkit/pkg/techref"
	"github.com/Cobliteam/workflow-toolkit/pkg/feasibility"
	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/parser"
	"github.com/Cobliteam/workflow-toolkit/pkg/patterns"
	"github.com/Cobliteam/workflow-toolkit/pkg/patterns_catalog"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ProgressFunc callback para reportar progresso durante geração.
// phase: nome da fase (epics, stories, criteria, deep_dives)
// current/total: progresso parcial dentro da fase
// message: descrição do passo atual
type ProgressFunc func(phase string, current, total int, message string)

// Generator gera backlog a partir do input
type Generator struct {
	provider     llm.Provider
	llmClient    llm.Completer
	specification *types.Specification
	projectInput *types.ProjectInput
	goldenPath   *types.GoldenPath
	teamPatterns *types.TeamPatterns
	patternLayer *patterns.PatternLayer
	verbose      bool
	sessionMgr   *SessionManager
	progressFunc ProgressFunc
	// Cost tracking
	totalInputTokens  int
	totalOutputTokens int
	totalCost         float64
	// Deep dive metrics (populated during FASE 4)
	deepDiveMetrics *techref.GenerationMetrics
	// InfraContext (populated during FASE 3.5 if --infra is enabled)
	infraContext *infracontext.InfraContext
	infraRegistry *infracontext.Registry
}

// SessionManager gerencia sessões de geração
type SessionManager struct {
	currentSession *types.Session
}

// trackCost registra custo de uma chamada LLM
func (g *Generator) trackCost(usage *llm.Usage) {
	if usage != nil {
		g.totalInputTokens += usage.InputTokens
		g.totalOutputTokens += usage.OutputTokens
		g.totalCost += usage.Cost
	}
}

// completeLLM wrapper que rastreia custo
func (g *Generator) completeLLM(prompt string, maxTokens int) (string, error) {
	response, usage, err := g.llmClient.CompleteWithUsage(prompt, maxTokens)
	g.trackCost(usage)
	return response, err
}

// NewGenerator cria novo gerador (assinatura compatível com commands)
func NewGenerator(provider llm.Provider, spec *types.Specification, pi *types.ProjectInput) *Generator {
	client, _ := llm.NewClient(provider)

	// Carregar Golden Paths e Team Patterns (para detecção de referências em deep dives)
	gp, tp := loadPatternStructs(pi)

	return &Generator{
		provider:      provider,
		llmClient:     client,
		specification: spec,
		projectInput:  pi,
		goldenPath:    gp,
		teamPatterns:  tp,
		verbose:       false,
		sessionMgr:    &SessionManager{},
	}
}

// loadPatternStructs carrega patterns estruturados (filesystem > embedded fallback)
func loadPatternStructs(pi *types.ProjectInput) (*types.GoldenPath, *types.TeamPatterns) {
	var gp *types.GoldenPath
	var tp *types.TeamPatterns

	// 1. Tentar filesystem (projeto pode ter patterns próprios)
	if baseDir := pi.Metadata["base_dir"]; baseDir != "" {
		gpPath := baseDir + "/golden-paths.md"
		tpPath := baseDir + "/team-patterns.md"

		gp, _ = parser.ParseGoldenPaths(gpPath)
		tp, _ = parser.ParseTeamPatterns(tpPath)
	}

	// 2. Fallback: usar patterns embarcados se filesystem retornou vazio
	layer := &patterns.PatternLayer{}

	if gp == nil || len(gp.Patterns) == 0 {
		if content := layer.GetGoldenPathsFull(); content != "" {
			gp, _ = parser.ParseGoldenPathsFromContent(content)
		}
	}
	if tp == nil || len(tp.Patterns) == 0 {
		if content := layer.GetTeamPatternsFull(); content != "" {
			tp, _ = parser.ParseTeamPatternsFromContent(content)
		}
	}

	if gp != nil && len(gp.Patterns) > 0 {
		fmt.Printf("✓ Carregados %d Golden Paths\n", len(gp.Patterns))
	}
	if tp != nil && len(tp.Patterns) > 0 {
		fmt.Printf("✓ Carregados %d Team Patterns\n", len(tp.Patterns))
	}

	return gp, tp
}

// NewGeneratorWithProvider creates a generator using a fully-decorated LLMProvider.
// This ensures the SecurityCheckpoint, Retry, and Cache decorators are in the call path.
// Preferred over NewGenerator() which uses the deprecated NewClient() with bare os.Getenv().
func NewGeneratorWithProvider(provider llm.LLMProvider, spec *types.Specification, pi *types.ProjectInput) *Generator {
	gp, tp := loadPatternStructs(pi)

	return &Generator{
		llmClient:     provider,
		specification: spec,
		projectInput:  pi,
		goldenPath:    gp,
		teamPatterns:  tp,
		verbose:       false,
		sessionMgr:    &SessionManager{},
	}
}

// NewGeneratorWithClient cria gerador com client injetado (para testes)
func NewGeneratorWithClient(client llm.Completer, spec *types.Specification, pi *types.ProjectInput) *Generator {
	return &Generator{
		llmClient:     client,
		specification: spec,
		projectInput:  pi,
		verbose:       false,
		sessionMgr:    &SessionManager{},
	}
}

// SetVerbose ativa modo verbose
func (g *Generator) SetVerbose(v bool) {
	g.verbose = v
}

// SetProgressFunc define callback de progresso para acompanhamento em tempo real.
// Usado pelo MCP Server para emitir progresso via stderr.
func (g *Generator) SetProgressFunc(fn ProgressFunc) {
	g.progressFunc = fn
}

// emitProgress envia progresso se callback está configurado
func (g *Generator) emitProgress(phase string, current, total int, message string) {
	if g.progressFunc != nil {
		g.progressFunc(phase, current, total, message)
	}
}

// SetPatternLayer injeta PatternLayer externo (do context)
func (g *Generator) SetPatternLayer(pl *patterns.PatternLayer) {
	g.patternLayer = pl
}

// SetSystemPrompt define system prompt no client LLM subjacente.
// Aceita qualquer provider que implemente LLMProvider (Client, Ollama, Azure, Mock).
func (g *Generator) SetSystemPrompt(prompt string) {
	if p, ok := g.llmClient.(llm.LLMProvider); ok {
		p.SetSystemPrompt(prompt)
	}
}

// getPatternLayer retorna PatternLayer (lazy init se não injetado)
func (g *Generator) getPatternLayer() *patterns.PatternLayer {
	if g.patternLayer == nil {
		g.patternLayer = &patterns.PatternLayer{}
	}
	return g.patternLayer
}

// GetSessionManager retorna session manager
func (g *Generator) GetSessionManager() *SessionManager {
	return g.sessionMgr
}

// GenerateOptions opções de geração
type GenerateOptions struct {
	SkipDeepDive        bool
	GenerateTasks       bool
	ComplexityThreshold int
	SuggestPatterns     bool   // Enable AI pattern suggestion (Phase 5)
	AnalyzeFeasibility  bool   // Enable technical feasibility analysis (Phase 6)
	AnalyzeCriticalPath bool   // Enable critical path analysis (Phase 7)
	ProjectDir          string // Project dir for pattern catalog overrides
	FetchInfra          bool   // Enable infrastructure context fetch
	InfraProvider       string // Provider ID (default: "kubectl")
	InfraNamespace      string // Kubernetes namespace (default: from provider config)
	KubeContext         string // kubectl context name
}

// SetInfraRegistry injects the infrastructure provider registry.
func (g *Generator) SetInfraRegistry(registry *infracontext.Registry) {
	g.infraRegistry = registry
}

// Generate gera backlog completo
func (g *Generator) Generate(input *types.ProjectInput, opts GenerateOptions) (*types.Backlog, error) {
	startTotal := time.Now()
	if g.verbose {
		fmt.Printf("\n⏱️  [%s] Iniciando geração de backlog\n", startTotal.Format("15:04:05"))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	backlog := &types.Backlog{
		Meta: types.Metadata{
			GeneratedAt: time.Now().Format(time.RFC3339),
			Provider:    string(g.provider),
			InputFile:   input.Metadata["file"],
		},
	}

	// Fase 1: Gerar épicos
	if g.verbose {
		fmt.Printf("\n📚 FASE 1: Gerando épicos\n")
		fmt.Printf("⏱️  Início: %s\n", time.Now().Format("15:04:05"))
	}
	g.emitProgress("epics", 0, 4, "Generating epics from specification")
	startEpics := time.Now()

	epics, err := g.generateEpics(input)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar épicos: %w", err)
	}
	backlog.Epics = epics
	g.emitProgress("epics", 1, 4, fmt.Sprintf("%d epics generated", len(epics)))

	if g.verbose {
		elapsed := time.Since(startEpics)
		fmt.Printf("✅ %d épicos gerados em %s\n", len(epics), formatDuration(elapsed))
	}

	// Fase 2: Gerar histórias para cada épico
	if g.verbose {
		fmt.Printf("\n📝 FASE 2: Gerando histórias\n")
		fmt.Printf("⏱️  Início: %s\n", time.Now().Format("15:04:05"))
	}
	g.emitProgress("stories", 0, len(backlog.Epics), "Generating stories for each epic")
	startStories := time.Now()
	totalStories := 0

	for i := range backlog.Epics {
		storyStart := time.Now()
		g.emitProgress("stories", i, len(backlog.Epics),
			fmt.Sprintf("Generating stories for %s", backlog.Epics[i].ID))
		stories, err := g.generateStories(input, &backlog.Epics[i])
		if err != nil {
			return nil, fmt.Errorf("erro ao gerar histórias para épico %s: %w", backlog.Epics[i].ID, err)
		}
		backlog.Epics[i].Stories = stories
		totalStories += len(stories)

		if g.verbose {
			fmt.Printf("  ✅ %s: %d histórias em %s\n",
				backlog.Epics[i].ID, len(stories), formatDuration(time.Since(storyStart)))
		}
	}
	g.emitProgress("stories", 2, 4, fmt.Sprintf("%d stories generated", totalStories))

	if g.verbose {
		elapsed := time.Since(startStories)
		fmt.Printf("✅ %d histórias geradas em %s\n", totalStories, formatDuration(elapsed))
	}

	// Fase 3: Gerar critérios de aceite
	if g.verbose {
		fmt.Printf("\n✓ FASE 3: Gerando critérios de aceite\n")
		fmt.Printf("⏱️  Início: %s\n", time.Now().Format("15:04:05"))
	}
	g.emitProgress("criteria", 0, totalStories, "Generating acceptance criteria")
	startCriteria := time.Now()
	
	for i := range backlog.Epics {
		for j := range backlog.Epics[i].Stories {
			criteria, err := g.generateAcceptanceCriteria(input, &backlog.Epics[i].Stories[j])
			if err != nil {
				return nil, fmt.Errorf("erro ao gerar critérios: %w", err)
			}
			backlog.Epics[i].Stories[j].AcceptanceCriteria = criteria
		}
	}
	
	g.emitProgress("criteria", 3, 4, "Acceptance criteria generated")

	if g.verbose {
		elapsed := time.Since(startCriteria)
		fmt.Printf("✅ Critérios gerados em %s\n", formatDuration(elapsed))
	}

	// Fase 3.5: Fetch infrastructure context (optional)
	if opts.FetchInfra && g.infraRegistry != nil {
		if g.verbose {
			fmt.Printf("\n🏗️  FASE 3.5: Fetching infrastructure context\n")
			fmt.Printf("⏱️  Início: %s\n", time.Now().Format("15:04:05"))
		}
		g.emitProgress("infra", 0, 1, "Fetching infrastructure context")

		providerID := opts.InfraProvider
		if providerID == "" {
			providerID = "kubectl"
		}

		infraProvider, err := g.infraRegistry.Get(providerID)
		if err != nil {
			if g.verbose {
				fmt.Printf("⚠️  Infra provider %q not found: %v (skipping)\n", providerID, err)
			}
		} else if !infraProvider.Available() {
			if g.verbose {
				fmt.Printf("⚠️  Infra provider %q not available (skipping)\n", providerID)
			}
		} else {
			ic, err := infraProvider.Fetch(context.Background(), infracontext.FetchOptions{
				Namespace:   opts.InfraNamespace,
				KubeContext: opts.KubeContext,
				UseCache:    true,
				Verbose:     g.verbose,
			})
			if err != nil {
				if g.verbose {
					fmt.Printf("⚠️  Infra fetch error: %v (continuing without infra)\n", err)
				}
			} else {
				g.infraContext = ic
				if g.verbose {
					fmt.Printf("✅ Infrastructure context: %s\n", ic.Summary())
				}
				g.emitProgress("infra", 1, 1, ic.Summary())
			}
		}
	}

	// Fase 4: Gerar deep dives contextualizados (opcional)
	if !opts.SkipDeepDive {
		if g.verbose {
			fmt.Printf("\n📚 FASE 4: Gerando deep dives contextualizados\n")
			fmt.Printf("⏱️  Início: %s\n", time.Now().Format("15:04:05"))
		}
		g.emitProgress("deep_dives", 0, 1, "Extracting technologies and generating deep dives")
		startDeepDives := time.Now()

		deepDives, err := g.generateContextualizedDeepDives(backlog)
		if err != nil {
			return nil, fmt.Errorf("erro ao gerar deep dives: %w", err)
		}
		backlog.DeepDives = deepDives
		g.emitProgress("deep_dives", 4, 4, fmt.Sprintf("%d deep dives generated", len(deepDives)))

		if g.verbose {
			elapsed := time.Since(startDeepDives)
			fmt.Printf("✅ %d deep dives gerados em %s\n", len(deepDives), formatDuration(elapsed))
		}
	}

	// Fase 5: Sugerir padrões de arquitetura (opcional)
	if opts.SuggestPatterns {
		if g.verbose {
			fmt.Printf("\n🏗️  FASE 5: Sugerindo padrões de arquitetura\n")
			fmt.Printf("⏱️  Início: %s\n", time.Now().Format("15:04:05"))
		}
		g.emitProgress("patterns", 0, 1, "Suggesting architecture patterns")
		startPatterns := time.Now()

		var catalogOpts []patterns_catalog.CatalogOption
		if opts.ProjectDir != "" {
			catalogOpts = append(catalogOpts, patterns_catalog.WithProjectOverrides(opts.ProjectDir))
		}

		catalog, err := patterns_catalog.NewPatternCatalog(catalogOpts...)
		if err != nil {
			// Non-fatal: log warning but continue
			if g.verbose {
				fmt.Printf("⚠️  Pattern catalog error: %v (skipping)\n", err)
			}
		} else {
			config := patterns_catalog.DefaultSuggestionConfig()
			suggestions, err := patterns_catalog.SuggestPatterns(catalog, *backlog, g.llmClient, config)
			if err != nil {
				if g.verbose {
					fmt.Printf("⚠️  Pattern suggestion error: %v (skipping)\n", err)
				}
			} else {
				backlog.PatternSuggestions = suggestions
				g.emitProgress("patterns", 1, 1,
					fmt.Sprintf("%d pattern suggestions generated", len(suggestions)))

				if g.verbose {
					elapsed := time.Since(startPatterns)
					fmt.Printf("✅ %d sugestões de padrões em %s\n", len(suggestions), formatDuration(elapsed))
					report := patterns_catalog.GenerateConflictReport(suggestions)
					if len(report.Blocked) > 0 {
						fmt.Printf("⚠️  %d sugestões bloqueadas por hierarquia\n", len(report.Blocked))
					}
				}

				// Phase 5.1: Ecosystem coherence analysis (heuristic, zero LLM)
				coherenceIssues := patterns_catalog.AnalyzeCoherence(suggestions, catalog)
				if len(coherenceIssues) > 0 {
					backlog.CoherenceIssues = coherenceIssues
					if g.verbose {
						fmt.Printf("🔍 %d coherence issues found\n", len(coherenceIssues))
					}
				}
			}
		}
	}

	// Fase 6: Análise de viabilidade técnica (heurística, zero LLM)
	if opts.AnalyzeFeasibility {
		if g.verbose {
			fmt.Printf("\n📊 FASE 6: Analisando viabilidade técnica\n")
		}
		report := feasibility.AnalyzeFeasibility(backlog, nil)
		backlog.FeasibilityReport = report
		if g.verbose {
			fmt.Printf("📊 Feasibility score: %d/100 (%d risks found)\n", report.Score, len(report.Items))
		}
	}

	// Fase 7: Critical path analysis (heuristic, zero LLM)
	if opts.AnalyzeCriticalPath {
		if g.verbose {
			fmt.Printf("\n🔀 FASE 7: Analisando caminho crítico\n")
		}
		cpReport := critical_path.AnalyzeCriticalPath(backlog)
		backlog.CriticalPathReport = cpReport
		if g.verbose {
			fmt.Printf("🔀 Critical path: %d phases, %d dependencies\n",
				len(cpReport.Phases), len(cpReport.Dependencies))
		}
	}

	// Attach InfraContext summary to backlog
	if g.infraContext != nil {
		backlog.InfraContext = g.buildInfraContextData()
	}

	// Calcular estatísticas
	backlog.Meta.Stats = g.calculateStats(backlog)

	// Popolar título do projeto (inferido do input)
	backlog.Meta.ProjectTitle = extractProjectTitle(input)

	// Popolar métricas de geração
	backlog.Meta.Metrics = &types.GenerationMetrics{
		TotalInputTokens:  g.totalInputTokens,
		TotalOutputTokens: g.totalOutputTokens,
		TotalCost:         g.totalCost,
	}
	if g.deepDiveMetrics != nil {
		backlog.Meta.Metrics.TotalTechsExtracted = g.deepDiveMetrics.TotalTechsExtracted
		backlog.Meta.Metrics.TrivialFiltered = g.deepDiveMetrics.TrivialFiltered
		backlog.Meta.Metrics.CrossEpicGlobalDives = g.deepDiveMetrics.CrossEpicGlobalDives
		backlog.Meta.Metrics.CrossEpicDeduplicated = g.deepDiveMetrics.CrossEpicDeduplicated
		backlog.Meta.Metrics.LLMCallsMade = g.deepDiveMetrics.LLMCallsMade
		backlog.Meta.Metrics.LLMCallsSaved = g.deepDiveMetrics.LLMCallsSaved
		backlog.Meta.Metrics.ReductionPercent = g.deepDiveMetrics.ReductionPercent
		backlog.Meta.Metrics.ClassificationStats = map[string]int{
			"trivial":  g.deepDiveMetrics.TrivialCount,
			"standard": g.deepDiveMetrics.StandardCount,
			"specific": g.deepDiveMetrics.SpecificCount,
			"critical": g.deepDiveMetrics.CriticalCount,
		}
	}

	// Resumo final
	if g.verbose {
		totalElapsed := time.Since(startTotal)
		fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("⏱️  RESUMO DE TEMPO\n")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("⏱️  Início: %s\n", startTotal.Format("15:04:05"))
		fmt.Printf("⏱️  Fim:    %s\n", time.Now().Format("15:04:05"))
		fmt.Printf("⏱️  TOTAL:  %s\n", formatDuration(totalElapsed))
		
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("💰 CUSTO REAL\n")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("📊 Tokens Input:   %s\n", formatNumber(g.totalInputTokens))
		fmt.Printf("📊 Tokens Output:  %s\n", formatNumber(g.totalOutputTokens))
		fmt.Printf("📊 Tokens Total:   %s\n", formatNumber(g.totalInputTokens + g.totalOutputTokens))
		fmt.Printf("💵 CUSTO TOTAL:    US$ %s\n", formatCurrency(g.totalCost))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	return backlog, nil
}

// buildInfraContextData creates a serializable summary of the infrastructure context.
func (g *Generator) buildInfraContextData() *types.InfraContextData {
	ic := g.infraContext
	if ic == nil {
		return nil
	}

	data := &types.InfraContextData{
		Provider:  ic.Provider,
		Cluster:   ic.Cluster,
		Namespace: ic.Namespace,
		FetchedAt: ic.FetchedAt.Format(time.RFC3339),
	}

	for _, node := range ic.Topology {
		switch node.Kind {
		case "Node":
			data.NodeCount++
		case "Pod":
			data.PodCount++
		case "Service":
			data.ServiceCount++
		}
	}
	data.AlertCount = len(ic.Alerts)

	// Health summary by component
	data.HealthSummary = make(map[string]string)
	for _, h := range ic.Health {
		data.HealthSummary[h.Component] = h.Status
	}

	return data
}

// extractProjectTitle extrai título do projeto do input com fallback em cadeia:
// 1. input.Metadata["project_name"] (explícito)
// 2. Primeiro heading # do contexto
// 3. Basename do arquivo de input sem extensão
// 4. Fallback: "Backlog Técnico"
func extractProjectTitle(input *types.ProjectInput) string {
	// Prioridade 1: metadata explícita
	if name, ok := input.Metadata["project_name"]; ok && name != "" {
		return name
	}

	// Prioridade 2: primeiro heading do contexto
	if input.Context != "" {
		for _, line := range strings.Split(input.Context, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# ") {
				title := strings.TrimPrefix(trimmed, "# ")
				title = strings.TrimSpace(title)
				if title != "" {
					return title
				}
			}
		}
	}

	// Prioridade 3: basename do arquivo de input
	if file, ok := input.Metadata["file"]; ok && file != "" {
		base := file
		// Extrair basename
		if idx := strings.LastIndex(base, "/"); idx >= 0 {
			base = base[idx+1:]
		}
		// Remover extensão
		if idx := strings.LastIndex(base, "."); idx >= 0 {
			base = base[:idx]
		}
		// Limpar hifens e underscores
		base = strings.ReplaceAll(base, "-", " ")
		base = strings.ReplaceAll(base, "_", " ")
		base = strings.TrimSpace(base)
		if base != "" {
			return base
		}
	}

	// Fallback
	return "Backlog Técnico"
}

// formatDuration formata duração de forma legível
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// formatNumber formata número com separador de milhares
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1000000, (n/1000)%1000, n%1000)
}

// formatCurrency formata valor monetário no padrão brasileiro
// Exemplo: 1.1116 -> "1,11"
func formatCurrency(value float64) string {
	// Arredondar para 2 casas decimais
	rounded := fmt.Sprintf("%.2f", value)
	// Substituir ponto por vírgula
	return strings.Replace(rounded, ".", ",", 1)
}

// deduplicatePatterns remove duplicatas mantendo ordem
func deduplicatePatterns(patterns []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	
	for _, p := range patterns {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	
	return result
}

// getPatternDescription busca descrição do pattern
func getPatternDescription(ref string, gp *types.GoldenPath, tp *types.TeamPatterns) string {
	if gp != nil {
		if p, ok := gp.Patterns[ref]; ok {
			return p.Name
		}
	}
	if tp != nil {
		if p, ok := tp.Patterns[ref]; ok {
			return p.Name
		}
	}
	return ""
}

// GenerateDeepDivesForBacklogWithSession gera deep dives contextualizados por história
func (g *Generator) GenerateDeepDivesForBacklogWithSession(backlog *types.Backlog, session *types.Session) ([]types.DeepDive, error) {
	fmt.Println("📚 Gerando deep dives contextualizados por história...")
	return g.generateContextualizedDeepDives(backlog)
}

// generateContextualizedDeepDives gera deep dives usando abordagem híbrida:
// FASE 1: Classificação heurística via TechRegistry (0 LLM calls)
// FASE 2: Geração de DDs com LLM baseado na classificação (otimizado)
//
// Economia: ~67-71% de redução de chamadas LLM.
// - TRIVIAL: skip
// - STANDARD (mesmo epic): batch em 1 prompt por epic
// - SPECIFIC: 1 DD contextualizado por story
// - CRITICAL: 1 DD detalhado por epic
// - Cross-epic global: 1 DD cross-context
func (g *Generator) generateContextualizedDeepDives(backlog *types.Backlog) ([]types.DeepDive, error) {
	fmt.Println("📚 Iniciando geração híbrida (heurística + LLM)...")

	// Load TechRegistry (embedded defaults + project overrides)
	regOpts := []techref.RegistryOption{}
	if baseDir := g.projectInput.Metadata["base_dir"]; baseDir != "" {
		regOpts = append(regOpts, techref.WithProjectDir(baseDir))
	}
	reg, err := techref.NewTechRegistry(regOpts...)
	if err != nil {
		// Fallback to default registry
		reg = techref.DefaultRegistry()
		fmt.Printf("  ⚠️  Usando registry padrão: %v\n", err)
	}

	// FASE 1: Classificação heurística (0 LLM calls)
	fmt.Println("🔍 FASE 1: Classificação heurística...")
	genConfig := techref.GetDefaultGenerationConfig()
	genConfig.DryRun = true
	genConfig.Verbose = g.verbose

	heuristicResult := techref.GenerateDeepDivesOptimizedWithRegistry(reg, *backlog, genConfig)

	// Salvar métricas para uso posterior em Generate()
	g.deepDiveMetrics = &heuristicResult.Metrics

	if g.verbose {
		fmt.Printf("   Techs extraídas: %d\n", heuristicResult.Metrics.TotalTechsExtracted)
		fmt.Printf("   Triviais filtradas: %d\n", heuristicResult.Metrics.TrivialFiltered)
		fmt.Printf("   Cross-epic global: %d\n", heuristicResult.Metrics.CrossEpicGlobalDives)
		fmt.Printf("   Cross-epic deduped: %d\n", heuristicResult.Metrics.CrossEpicDeduplicated)
		fmt.Printf("   DDs a gerar: %d\n", len(heuristicResult.DeepDives))
	}

	// FASE 2: Gerar DDs com LLM baseado na classificação
	fmt.Println("🤖 FASE 2: Gerando deep dives com LLM...")
	allDeepDives := []types.DeepDive{}

	for _, dryDD := range heuristicResult.DeepDives {
		fmt.Printf("  📖 Gerando deep dive: %s\n", dryDD.Term)

		// Detect patterns for context enrichment
		patternRefs := g.detectPatterns(dryDD.Term, backlog)

		// Generate the DD with LLM
		dd, err := g.generateSingleDeepDive(dryDD, backlog, patternRefs)
		if err != nil {
			fmt.Printf("  ⚠️  Erro ao gerar deep dive para %s: %v\n", dryDD.Term, err)
			continue
		}

		allDeepDives = append(allDeepDives, dd)
		fmt.Printf("  ✅ Deep dive gerado: %s\n", dryDD.Term)

		// Rate limiting
		time.Sleep(300 * time.Millisecond)
	}

	fmt.Printf("✅ Total de %d deep dives gerados (heurística filtrou %d triviais, deduplicou %d cross-epic)\n",
		len(allDeepDives),
		heuristicResult.Metrics.TrivialFiltered,
		heuristicResult.Metrics.CrossEpicDeduplicated)

	return allDeepDives, nil
}

// detectPatterns finds pattern references related to a tech term in the backlog
func (g *Generator) detectPatterns(term string, backlog *types.Backlog) []string {
	var patternRefs []string

	for _, epic := range backlog.Epics {
		for _, story := range epic.Stories {
			fullText := story.What + " " + story.Why + " " + strings.Join(story.AcceptanceCriteria, " ")
			if !strings.Contains(strings.ToLower(fullText), strings.ToLower(term)) {
				continue
			}

			if g.goldenPath != nil {
				refs := parser.DetectPatternReferences(fullText, g.goldenPath, nil)
				patternRefs = append(patternRefs, refs...)
			}
			if g.teamPatterns != nil {
				refs := parser.DetectPatternReferences(fullText, nil, g.teamPatterns)
				patternRefs = append(patternRefs, refs...)
			}
		}
	}

	return deduplicatePatterns(patternRefs)
}

// generateSingleDeepDive generates one DD using LLM with context from heuristic classification
func (g *Generator) generateSingleDeepDive(dryDD types.DeepDive, backlog *types.Backlog, patternRefs []string) (types.DeepDive, error) {
	// Build context from the backlog
	context := g.buildDDContext(dryDD, backlog)

	patternContext := g.buildPatternContext(patternRefs)

	prompt := fmt.Sprintf(`Você é um arquiteto de software. Explique como a tecnologia %s é usada no projeto.

%s

CONTEXTO DO PROJETO:
%s%s

INSTRUÇÕES:
- Foque em COMO esta tecnologia é usada neste contexto específico
- Não explique a tecnologia de forma genérica
- Conecte com os patterns relacionados se houver
- Seja específico sobre configuração e decisões

FORMATO DE SAÍDA (JSON válido):
{
  "term": "%s",
  "what_is": "O que é (1 frase curta)",
  "what_in_this_story": "O que esta tecnologia FAZ neste contexto (2-3 frases específicas)",
  "why_here": "Por que foi escolhida para o projeto",
  "why_in_this_story": "Por que é necessária neste contexto específico (1-2 frases)",
  "configuration": "Configuração específica",
  "patterns": ["Padrão específico 1", "Padrão específico 2"],
  "source_patterns": [%s],
  "decisions": ["Decisão técnica específica"]
}

Retorne APENAS o JSON.`,
		dryDD.Term,
		context,
		g.projectInput.Context,
		patternContext,
		dryDD.Term,
		formatPatternRefs(patternRefs))

	response, err := g.completeLLM(prompt, 2000)
	if err != nil {
		return types.DeepDive{}, err
	}

	jsonStr := extractJSON(response)

	var dd types.DeepDive
	if err := json.Unmarshal([]byte(jsonStr), &dd); err != nil {
		// Fallback: create minimal DD from response
		dd = types.DeepDive{
			Term:   dryDD.Term,
			WhatIs: response,
		}
	}

	// Carry over metadata from heuristic classification
	dd.StoryID = dryDD.StoryID
	dd.Term = dryDD.Term

	// Enrich pattern refs
	enrichedRefs := g.enrichPatternRefs(patternRefs)
	if len(enrichedRefs) > 0 {
		dd.SourcePatterns = enrichedRefs
	}

	return dd, nil
}

// buildDDContext builds context string for a specific DD based on where the tech appears
func (g *Generator) buildDDContext(dryDD types.DeepDive, backlog *types.Backlog) string {
	termLower := strings.ToLower(dryDD.Term)

	// If story-level DD, show specific story context
	if dryDD.StoryID != "" {
		for _, epic := range backlog.Epics {
			for _, story := range epic.Stories {
				if story.ID == dryDD.StoryID {
					return fmt.Sprintf("HISTÓRIA: %s - %s\n%s\nPor que: %s",
						story.ID, story.Title, story.What, story.Why)
				}
			}
		}
	}

	// Otherwise (epic-level or global), show all stories mentioning this tech
	var context strings.Builder
	context.WriteString(fmt.Sprintf("Tecnologia %s encontrada nos seguintes contextos:\n", dryDD.Term))

	for _, epic := range backlog.Epics {
		for _, story := range epic.Stories {
			fullText := strings.ToLower(story.Title + " " + story.What + " " + story.Why)
			if strings.Contains(fullText, termLower) {
				context.WriteString(fmt.Sprintf("\n- %s (%s): %s", story.ID, epic.Title, story.What))
			}
		}
	}

	return context.String()
}

// buildPatternContext builds pattern context string for the prompt
func (g *Generator) buildPatternContext(patternRefs []string) string {
	if len(patternRefs) == 0 {
		return ""
	}

	patternContext := "\n\nPATTERNS RELACIONADOS:\n"
	for _, ref := range patternRefs {
		if g.goldenPath != nil {
			if p, ok := g.goldenPath.Patterns[ref]; ok {
				patternContext += fmt.Sprintf("- %s: %s\n", ref, p.Name)
			}
		}
		if g.teamPatterns != nil {
			if p, ok := g.teamPatterns.Patterns[ref]; ok {
				patternContext += fmt.Sprintf("- %s: %s\n", ref, p.Name)
			}
		}
	}
	return patternContext
}

// enrichPatternRefs adds pattern names to their IDs
func (g *Generator) enrichPatternRefs(patternRefs []string) []string {
	enriched := []string{}
	for _, ref := range patternRefs {
		entry := ref
		if g.goldenPath != nil {
			if p, ok := g.goldenPath.Patterns[ref]; ok {
				entry = fmt.Sprintf("%s: %s", ref, p.Name)
			}
		}
		if g.teamPatterns != nil {
			if p, ok := g.teamPatterns.Patterns[ref]; ok {
				entry = fmt.Sprintf("%s: %s", ref, p.Name)
			}
		}
		enriched = append(enriched, entry)
	}
	return enriched
}

// Deprecated: extractTechnologiesWithLLM is replaced by the hybrid pipeline
// (generateContextualizedDeepDives) which uses TechRegistry heuristics first.
// Kept for backward compatibility tests only.
func (g *Generator) extractTechnologiesWithLLM(text string, storyID string) ([]string, error) {
	prompt := fmt.Sprintf(`Você é um especialista em arquitetura de software. Analise o texto abaixo e extraia TODAS as tecnologias mencionadas.

TEXTO DA HISTÓRIA:
%s

INSTRUÇÕES:
- Extraia TODAS as tecnologias, frameworks, databases, ferramentas mencionadas
- Inclua: databases (PostgreSQL, RocksDB, ScyllaDB), frameworks (Spring Boot, React), 
  ferramentas (Docker, Kubernetes), messaging (Kafka, RabbitMQ), 
  state stores (RocksDB, LevelDB), monitoring (Prometheus, Grafana), etc.
- Use nomes canônicos (ex: "Kafka" não "Apache Kafka")
- Não inclua conceitos genéricos como "API", "REST", "HTTP"
- Se não houver tecnologias específicas, retorne lista vazia

FORMATO DE SAÍDA (JSON válido):
{
  "technologies": ["Tech1", "Tech2", "Tech3"]
}

Retorne APENAS o JSON.`, text)

	response, err := g.completeLLM(prompt, 500)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSON(response)
	
	var result struct {
		Technologies []string `json:"technologies"`
	}
	
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		fmt.Printf("  ⚠️  [%s] Erro ao parsear tecnologias JSON: %v\n", storyID, err)
		return []string{}, nil
	}

	return result.Technologies, nil
}

// formatPatternRefs formata lista de patterns para JSON
func formatPatternRefs(refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	quoted := []string{}
	for _, ref := range refs {
		quoted = append(quoted, fmt.Sprintf(`"%s"`, ref))
	}
	return strings.Join(quoted, ", ")
}

func (g *Generator) generateEpics(input *types.ProjectInput) ([]types.Epic, error) {
	if g.verbose {
		fmt.Println("📚 Gerando épicos...")
	}

	// Construir contexto rico para o prompt
	context := fmt.Sprintf(`# Contexto do Projeto
%s

# Volumetria
%s

# Stack Técnico
%s

# Requisitos Não Funcionais
%s`, input.Context, input.Volumetry, input.Stack, input.NFRs)

	// Usar patterns layer (Essentials = 2KB vs 23KB)
	layer := g.getPatternLayer()
	patternsText := layer.GetCombined(patterns.Essentials)
	patternsFormatted := layer.FormatWithHeader(patternsText, patterns.Essentials)

	prompt := fmt.Sprintf(`Você é um arquiteto de software sênior. Analise o projeto abaixo e identifique os épicos técnicos principais.

%s

%s

INSTRUÇÕES CRÍTICAS:
1. Identifique 3-5 épicos técnicos de alto nível
2. Cada épico deve representar uma área funcional ou técnica significativa
3. Priorize por impacto no negócio e dependências técnicas
4. Use nomenclatura clara e técnica em PT-BR (exceto termos técnicos em inglês)
5. ⚠️  SIGA RIGOROSAMENTE os padrões acima - violar = código rejeitado
6. ❌ NÃO sugira tecnologias que violem os anti-patterns listados

FORMATO DE SAÍDA (JSON válido):
[
  {
    "id": "E1",
    "title": "Nome do Épico em PT-BR",
    "description": "Descrição detalhada (2-3 frases)",
    "tags": ["tag1", "tag2"],
    "priority": "high|medium|low",
    "complexity": 8
  }
]

Retorne APENAS o JSON, sem explicações adicionais.`, context, patternsFormatted)

	response, err := g.completeLLM(prompt, 3000)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar épicos: %w", err)
	}

	// Extrair JSON da resposta (pode vir com markdown backticks)
	jsonStr := extractJSON(response)
	
	var epics []types.Epic
	if err := json.Unmarshal([]byte(jsonStr), &epics); err != nil {
		return nil, fmt.Errorf("erro ao parsear épicos JSON: %w\nResposta: %s", err, jsonStr)
	}

	// Enriquecer épicos com metadados
	for i := range epics {
		if epics[i].ID == "" {
			epics[i].ID = fmt.Sprintf("E%d", i+1)
		}
		if epics[i].Priority == "" {
			epics[i].Priority = "medium"
		}
		if epics[i].Complexity == 0 {
			epics[i].Complexity = 5
		}
		// Adicionar código único
		epics[i].Code = fmt.Sprintf("EPIC-%s", epics[i].ID)
	}

	return epics, nil
}

// extractJSON extrai JSON de resposta que pode conter markdown
func extractJSON(response string) string {
	// Remover markdown code blocks
	response = strings.TrimSpace(response)
	
	// Se tem ```json, extrair conteúdo entre os backticks
	if strings.Contains(response, "```json") {
		start := strings.Index(response, "```json")
		if start != -1 {
			// Pular "```json" e qualquer whitespace/newline
			start += 7
			// Encontrar o closing ```
			end := strings.Index(response[start:], "```")
			if end != -1 {
				response = response[start : start+end]
			}
		}
	} else if strings.Contains(response, "```") {
		// Se tem apenas ```, extrair também
		start := strings.Index(response, "```")
		if start != -1 {
			start += 3
			end := strings.Index(response[start:], "```")
			if end != -1 {
				response = response[start : start+end]
			}
		}
	}
	
	return strings.TrimSpace(response)
}

func (g *Generator) generateStories(input *types.ProjectInput, epic *types.Epic) ([]types.Story, error) {
	if g.verbose {
		fmt.Printf("  📝 Gerando histórias para %s - %s...\n", epic.ID, epic.Title)
	}

	// Usar Essentials layer - histórias PRECISAM de contexto técnico
	layer := g.getPatternLayer()
	patternsText := layer.GetCombined(patterns.Essentials)

	prompt := fmt.Sprintf(`Você é um Product Owner técnico. Para o épico abaixo, gere histórias de usuário técnicas.

ÉPICO: %s - %s
%s

CONTEXTO DO PROJETO:
%s

VOLUMETRIA:
%s

%s

INSTRUÇÕES:
1. Gere 2-4 histórias de usuário técnicas
2. Use formato: "Como [papel], quero [ação] para [benefício]"
3. Cada história deve ser independente e entregável
4. Estime story points (1, 2, 3, 5, 8)
5. Inclua critérios de aceite técnicos
6. ⚠️  Respeite os padrões obrigatórios acima

FORMATO DE SAÍDA (JSON válido):
[
  {
    "id": "%s.1",
    "title": "Título da história",
    "what": "Como engenheiro, quero X para Y",
    "why": "Razão de negócio",
    "effort": 5,
    "acceptance_criteria": []
  }
]

Retorne APENAS o JSON.`, epic.ID, epic.Title, epic.Description, input.Context, input.Volumetry, patternsText, epic.ID)

	response, err := g.completeLLM(prompt, 4000)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar histórias: %w", err)
	}

	jsonStr := extractJSON(response)
	
	var stories []types.Story
	if err := json.Unmarshal([]byte(jsonStr), &stories); err != nil {
		return nil, fmt.Errorf("erro ao parsear histórias: %w\nResposta: %s", err, jsonStr)
	}

	// Enriquecer histórias
	for i := range stories {
		if stories[i].ID == "" {
			stories[i].ID = fmt.Sprintf("%s.%d", epic.ID, i+1)
		}
		if stories[i].Effort == 0 {
			stories[i].Effort = 3 // Default
		}
		stories[i].EpicID = epic.ID
		stories[i].Status = "todo"
	}

	// Rate limiting: delay entre chamadas
	time.Sleep(500 * time.Millisecond)

	return stories, nil
}

func (g *Generator) generateAcceptanceCriteria(input *types.ProjectInput, story *types.Story) ([]string, error) {
	// Se história já tem critérios, usar eles
	if len(story.AcceptanceCriteria) > 0 {
		return story.AcceptanceCriteria, nil
	}

	if g.verbose {
		fmt.Printf("    ✓ Gerando critérios para %s...\n", story.ID)
	}

	prompt := fmt.Sprintf(`Você é um QA técnico sênior. Para a história abaixo, gere critérios de aceite mensuráveis.

HISTÓRIA: %s
%s

RNFs DO PROJETO:
%s

INSTRUÇÕES:
1. Gere 2-3 critérios de aceite técnicos e mensuráveis
2. Use formato Given-When-Then quando apropriado
3. Inclua métricas específicas (latência, throughput, etc)
4. Seja específico e testável

FORMATO DE SAÍDA (JSON válido):
[
  "Critério 1 com métrica específica",
  "Critério 2 testável",
  "Critério 3 mensurável"
]

Retorne APENAS o JSON array.`, story.ID, story.What, input.NFRs)

	response, err := g.completeLLM(prompt, 1500)
	if err != nil {
		// Se falhar, retornar critérios default
		return []string{
			fmt.Sprintf("Implementação de %s completa", story.Title),
			"Testes unitários com cobertura > 80%",
			"Code review aprovado",
		}, nil
	}

	jsonStr := extractJSON(response)
	
	var criteria []string
	if err := json.Unmarshal([]byte(jsonStr), &criteria); err != nil {
		// Fallback para critérios default se parsing falhar
		return []string{
			fmt.Sprintf("Implementação de %s completa", story.Title),
			"Testes unitários com cobertura > 80%",
			"Code review aprovado",
		}, nil
	}

	// Rate limiting
	time.Sleep(300 * time.Millisecond)

	return criteria, nil
}

func (g *Generator) calculateStats(backlog *types.Backlog) types.GenerationStats {
	stats := types.GenerationStats{
		TotalEpics: len(backlog.Epics),
	}

	for _, epic := range backlog.Epics {
		stats.TotalStories += len(epic.Stories)
		for _, story := range epic.Stories {
			stats.TotalCriteria += len(story.AcceptanceCriteria)
			stats.TotalStoryPoints += story.Effort
		}
	}

	stats.TotalDeepDives = len(backlog.DeepDives)

	// Também atualizar Meta diretamente
	backlog.Meta.TotalEpics = stats.TotalEpics
	backlog.Meta.TotalStories = stats.TotalStories

	return stats
}
