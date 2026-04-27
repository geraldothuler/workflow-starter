package render


// LensData representa dados processados para o Lens
type LensData struct {
	Meta       MetaData              `json:"meta"`
	KPIs       []KPI                 `json:"kpis"`
	Epics      map[string]EpicLens   `json:"epics"`
	Stories    map[string]StoryLens  `json:"stories"`    // É MAP, não array
	DeepDives  map[string]DeepDiveLens `json:"deep_dives"`
	Search     interface{}           `json:"search"`
	Milestones []Milestone           `json:"milestones"` // v3.2: Milestones com roadmap
	Timeline   Timeline              `json:"timeline"`
	Graph      DependencyGraph       `json:"graph"`
	Summary    Summary               `json:"summary"`
	Squad      SquadAnalysis         `json:"squad"`      // É objeto único, não array
	Effort     EffortSummary         `json:"effort"`     // v3.2: Análise de esforço
	Documents  map[string]DocumentInfo `json:"documents"` // v3.2: Documentos de origem
	Metrics    *GenerationMetricsLens `json:"metrics,omitempty"` // v3.3: Métricas de geração
	PatternSuggestions []PatternSuggestionLens `json:"pattern_suggestions,omitempty"` // v3.4: Pattern suggestions
	CoherenceIssues    []CoherenceIssueLens    `json:"coherence_issues,omitempty"`    // v3.5: Coherence analysis
	FeasibilityReport  *FeasibilityReportLens  `json:"feasibility_report,omitempty"`  // v3.6: Feasibility analysis
	CriticalPathReport *CriticalPathReportLens `json:"critical_path_report,omitempty"` // v3.7: Critical path analysis
}

// PatternSuggestionLens represents a pattern suggestion for Lens visualization.
type PatternSuggestionLens struct {
	PatternID     string   `json:"pattern_id"`
	PatternName   string   `json:"pattern_name"`
	Type          string   `json:"type"`
	Confidence    float64  `json:"confidence"`
	Reasoning     string   `json:"reasoning"`
	AffectedEpics []string `json:"affected_epics,omitempty"`
	Category      string   `json:"category"`
	Source        string   `json:"source,omitempty"`
	Level         string   `json:"level,omitempty"`
	Remediation   []string `json:"remediation,omitempty"`
	BlockedBy     string   `json:"blocked_by,omitempty"`
}

// CoherenceIssueLens represents a coherence issue for Lens visualization.
type CoherenceIssueLens struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	PatternIDs  []string `json:"pattern_ids"`
	Suggestion  string   `json:"suggestion"`
}

// FeasibilityReportLens represents feasibility analysis for Lens visualization.
type FeasibilityReportLens struct {
	Score   int                    `json:"score"`
	Items   []FeasibilityItemLens  `json:"items"`
	Summary string                 `json:"summary"`
}

// FeasibilityItemLens represents a single feasibility risk item for Lens.
type FeasibilityItemLens struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Suggestion  string `json:"suggestion"`
}

// CriticalPathReportLens represents critical path analysis for Lens visualization.
type CriticalPathReportLens struct {
	Items        []CriticalPathItemLens `json:"items"`
	Phases       []ExecutionPhaseLens   `json:"phases"`
	Dependencies []DependencyEdgeLens   `json:"dependencies"`
	Summary      string                 `json:"summary"`
}

// CriticalPathItemLens represents a single epic in the execution plan for Lens.
type CriticalPathItemLens struct {
	ID           string   `json:"id"`
	EpicCode     string   `json:"epic_code"`
	EpicTitle    string   `json:"epic_title"`
	Phase        int      `json:"phase"`
	Priority     int      `json:"priority"`
	Reasoning    string   `json:"reasoning"`
	DependsOn    []string `json:"depends_on"`
	IsFoundation bool     `json:"is_foundation"`
	Tags         []string `json:"tags"`
}

// ExecutionPhaseLens represents a grouped execution phase for Lens.
type ExecutionPhaseLens struct {
	Phase       int      `json:"phase"`
	EpicCodes   []string `json:"epic_codes"`
	Parallel    bool     `json:"parallel"`
	TotalEffort int      `json:"total_effort"`
	Reasoning   string   `json:"reasoning"`
}

// DependencyEdgeLens represents an inferred dependency edge for Lens.
type DependencyEdgeLens struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// MetaData metadata enriquecido
type MetaData struct {
	Title          string `json:"title"`
	TitleHighlight string `json:"title_highlight"`
	Subtitle       string `json:"subtitle"`
	Eyebrow        string `json:"eyebrow"`
	Lang           string `json:"lang"`
	Description    string `json:"description"`
	TotalEpics     int    `json:"total_epics"`
	TotalStories   int    `json:"total_stories"`
	KPIs           []KPI  `json:"kpis"`
}

// KPI representa um indicador
type KPI struct {
	Label  string      `json:"label"`
	Value  interface{} `json:"value"`
	Sub    string      `json:"sub"`
	Source string      `json:"source"`
}

// EpicLens épico enriquecido para Lens
type EpicLens struct {
	ID          string       `json:"id"`
	Code        string       `json:"code"`
	Title       string       `json:"title"`
	Summary     string       `json:"summary"`
	Description string       `json:"description"`
	Context     string       `json:"context"`
	Stories     []StoryLens  `json:"stories"`
	Tags        []interface{} `json:"tags"`
	Priority    string       `json:"priority"`
}

// StoryLens história enriquecida para Lens
type StoryLens struct {
	ID                 string        `json:"id"`
	Code               string        `json:"code"`
	Title              string        `json:"title"`
	Description        string        `json:"description"`
	What               string        `json:"what"`
	Why                string        `json:"why"`
	AcceptanceCriteria []string      `json:"acceptance_criteria"`
	Tags               []interface{} `json:"tags"`
	Effort             int           `json:"effort"`
	EstimatedPoints    int           `json:"estimated_points"`
	Source             string        `json:"source"`
	Tasks              []TaskLens    `json:"tasks"`
	RiskLabel          string        `json:"risk_label"`
	Subtasks           []string      `json:"subtasks"`
}

// TaskLens task enriquecida para Lens
type TaskLens struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	EstimatedHours int    `json:"estimated_hours"`
	Status         string `json:"status"`
	Effort         string `json:"effort"`
}

// DeepDiveLens deep dive enriquecido para Lens
type DeepDiveLens struct {
	ID                string        `json:"id"`
	StoryID           string        `json:"story_id,omitempty"`           // NOVO
	Term              string        `json:"term"`
	Terms             []interface{} `json:"terms"`
	WhatIs            string        `json:"what_is"`
	WhatInThisStory   string        `json:"what_in_this_story,omitempty"` // NOVO
	WhyHere           string        `json:"why_here"`
	WhyInThisStory    string        `json:"why_in_this_story,omitempty"`  // NOVO
	Configuration     string        `json:"configuration"`
	Patterns          []string      `json:"patterns"`
	SourcePatterns    []string      `json:"source_patterns,omitempty"`    // NOVO
	RelatedCriteria   []string      `json:"related_criteria,omitempty"`   // NOVO
	AntiPatterns      []string      `json:"anti_patterns"`
	Decisions         []string      `json:"decisions"`
	RelatedTerms      []string      `json:"related_terms"`
	Classification    string        `json:"classification,omitempty"`     // trivial/standard/specific/critical
	Scope             string        `json:"scope,omitempty"`              // epic/story/global
	Source            SourceTrace   `json:"source"`
}

// Milestone representa um milestone com informações completas
type Milestone struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	EpicIDs      []string `json:"epic_ids"`
	TotalSPs     int      `json:"total_sps"`
	DaysEstimate int      `json:"days_estimate"`
	ValueProp    string   `json:"value_prop"`
}

// Timeline representa timeline do projeto
type Timeline struct {
	Milestones []Milestone `json:"milestones"`
}

// DependencyGraph representa grafo de dependências
type DependencyGraph struct {
	Nodes []interface{} `json:"nodes"`
	Edges []interface{} `json:"edges"`
}

// Summary representa sumário executivo
type Summary struct {
	Overview  string   `json:"overview"`
	KeyPoints []string `json:"key_points"`
}

// SquadAnalysis representa análise por squad
type SquadAnalysis struct {
	SquadName string `json:"squad_name"`
	Stories   int    `json:"stories"`
	Points    int    `json:"points"`
}

// RiskBar representa barra de risco
type RiskBar struct {
	Level       string `json:"level"`
	Description string `json:"description"`
	Percentage  int    `json:"percentage"`
}

// Stat representa uma estatística
type Stat struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
	Unit  string      `json:"unit"`
}

// SourceTrace representa rastreamento de origem
type SourceTrace struct {
	Type       string `json:"type"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Confidence string `json:"confidence"`
}

// SourceBadge representa badge de origem
type SourceBadge struct {
	Label      string `json:"label"`
	Color      string `json:"color"`
	Icon       string `json:"icon"`
	SourcePath string `json:"source_path"`
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// GENERATION METRICS (v3.3)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// GenerationMetricsLens métricas de geração para visualização no Lens
type GenerationMetricsLens struct {
	TotalTechsExtracted   int            `json:"total_techs_extracted"`
	TrivialFiltered       int            `json:"trivial_filtered"`
	ClassificationStats   map[string]int `json:"classification_stats,omitempty"`
	CrossEpicGlobalDives  int            `json:"cross_epic_global_dives"`
	CrossEpicDeduplicated int            `json:"cross_epic_deduplicated"`
	LLMCallsMade          int            `json:"llm_calls_made"`
	LLMCallsSaved         int            `json:"llm_calls_saved"`
	ReductionPercent      float64        `json:"reduction_percent"`
	TotalInputTokens      int            `json:"total_input_tokens"`
	TotalOutputTokens     int            `json:"total_output_tokens"`
	TotalCost             float64        `json:"total_cost"`
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EFFORT & CRITICAL PATH ANALYSIS (v3.2)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// EffortSummary agregação de esforço por épico e total
type EffortSummary struct {
	TotalStories     int                    `json:"total_stories"`
	TotalSPs         int                    `json:"total_sps"`
	TotalDays        int                    `json:"total_days"`
	OptimisticDays   int                    `json:"optimistic_days"`   // Se tudo paralelo
	RealisticDays    int                    `json:"realistic_days"`    // Com paralelismo
	Velocity         int                    `json:"velocity"`          // SPs/sprint
	ByEpic           map[string]EpicEffort  `json:"by_epic"`
	CriticalPath     *CriticalPathAnalysis  `json:"critical_path,omitempty"` // Opcional
}

// EpicEffort esforço agregado por épico
type EpicEffort struct {
	EpicID       string  `json:"epic_id"`
	Stories      int     `json:"stories"`
	SPs          int     `json:"sps"`
	Days         int     `json:"days"`
	Percentage   float64 `json:"percentage"`  // % do total
}

// CriticalPathAnalysis análise de gargalos e paralelismo
type CriticalPathAnalysis struct {
	Enabled          bool              `json:"enabled"`            // Se análise foi feita
	LargeStories     []CriticalStory   `json:"large_stories"`      // Histórias grandes
	TotalBlockedDays int               `json:"total_blocked_days"` // Dias bloqueados
	Bottlenecks      []Bottleneck      `json:"bottlenecks"`        // Gargalos por épico
	Recommendations  []string          `json:"recommendations"`    // Sugestões
}

// CriticalStory história que cria gargalo
type CriticalStory struct {
	Code         string  `json:"code"`
	Title        string  `json:"title"`
	EpicID       string  `json:"epic_id"`
	Effort       int     `json:"effort"`
	Days         float64 `json:"days"`
	IsBottleneck bool    `json:"is_bottleneck"`
	Reason       string  `json:"reason"` // Explicação em linguagem natural
}

// Bottleneck gargalo identificado em um épico
type Bottleneck struct {
	EpicID      string `json:"epic_id"`
	EpicTitle   string `json:"epic_title"`
	BlockedDays int    `json:"blocked_days"`
	Reason      string `json:"reason"`     // Por quê é gargalo
	Suggestion  string `json:"suggestion"` // Como mitigar
}

// ParallelismLimits limites de paralelismo da squad
type ParallelismLimits struct {
	LargeStories  int `json:"large_stories"`   // Limite para 5+ SPs (0 = ilimitado)
	MediumStories int `json:"medium_stories"`  // Limite para 3-4 SPs
	SmallStories  int `json:"small_stories"`   // Limite para 1-2 SPs
}

// TeamConfig configuração da squad (velocity + paralelismo)
type TeamConfig struct {
	Velocity           int                `json:"velocity"`              // SPs/sprint
	ParallelismLimits  *ParallelismLimits `json:"parallelism_limits"`    // Opcional
}

// DocumentInfo metadados de documento de origem
type DocumentInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeKB    int    `json:"size_kb"`
	Available bool   `json:"available"`
	Content   string `json:"content,omitempty"` // Só se disponível
}

