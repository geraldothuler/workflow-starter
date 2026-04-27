package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PatternSuggestion represents a suggested architecture pattern for the project.
type PatternSuggestion struct {
	PatternID     string   `json:"pattern_id"`                // e.g. "cqrs", "distributed-monolith"
	PatternName   string   `json:"pattern_name"`              // Full name (English)
	Type          string   `json:"type"`                      // "pattern" or "anti-pattern"
	Confidence    float64  `json:"confidence"`                // 0.0-1.0
	Reasoning     string   `json:"reasoning"`                 // Why it fits / why it's detected
	AffectedEpics []string `json:"affected_epics,omitempty"`  // Epic IDs/codes
	Category      string   `json:"category"`                  // "data", "resilience", etc.
	Source        string   `json:"source,omitempty"`          // "GoF", "PoEAA", "Microservices", etc.
	Level         string   `json:"level,omitempty"`           // "universal", "company", "team", "project"
	Remediation   []string `json:"remediation,omitempty"`     // For anti-patterns: patterns that fix it
	BlockedBy     string   `json:"blocked_by,omitempty"`      // If guardrail blocked this suggestion
}

// CoherenceIssue represents an ecosystem coherence issue found by heuristic analysis.
// Issues are detected by comparing pattern suggestions against the catalog's relationship data.
type CoherenceIssue struct {
	ID          string   `json:"id"`          // "COH-001", sequential
	Type        string   `json:"type"`        // "missing-companion", "anti-pattern-contradiction", "when-to-use-mismatch"
	Severity    string   `json:"severity"`    // "high", "medium", "low"
	Title       string   `json:"title"`
	Description string   `json:"description"`
	PatternIDs  []string `json:"pattern_ids"` // IDs of patterns involved
	Suggestion  string   `json:"suggestion"`  // Suggested action
}

// FeasibilityItem represents a single risk or concern found during feasibility analysis.
type FeasibilityItem struct {
	ID          string `json:"id"`          // "FSB-001", sequential
	Category    string `json:"category"`    // "complexity", "technology", "requirements", "architecture", "schedule"
	Severity    string `json:"severity"`    // "critical", "high", "medium", "low"
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"`      // What happens if this risk is ignored
	Suggestion  string `json:"suggestion"`  // Actionable recommendation
}

// FeasibilityReport is the aggregated technical feasibility analysis result.
type FeasibilityReport struct {
	Score   int               `json:"score"`   // 0-100 (100 = fully feasible, no risks)
	Items   []FeasibilityItem `json:"items"`
	Summary string            `json:"summary"` // e.g. "2 critical, 1 high risks found"
}

// CriticalPathItem represents a single epic in the ordered execution plan.
type CriticalPathItem struct {
	ID           string   `json:"id"`            // "CPT-001", sequential
	EpicCode     string   `json:"epic_code"`     // Epic code (e.g., "AUTH")
	EpicTitle    string   `json:"epic_title"`
	Phase        int      `json:"phase"`         // Execution phase (1, 2, 3...)
	Priority     int      `json:"priority"`      // 1-100 priority score
	Reasoning    string   `json:"reasoning"`     // Why this ordering
	DependsOn    []string `json:"depends_on"`    // Epic codes this depends on
	IsFoundation bool     `json:"is_foundation"` // Foundation epic (infra, auth, etc.)
	Tags         []string `json:"tags"`          // "foundation", "critical-tech", "high-complexity"
}

// ExecutionPhase groups epics that can run in parallel within one phase.
type ExecutionPhase struct {
	Phase       int      `json:"phase"`        // Phase number (1, 2, 3...)
	EpicCodes   []string `json:"epic_codes"`   // Epics in this phase
	Parallel    bool     `json:"parallel"`     // >1 epic = can run in parallel
	TotalEffort int      `json:"total_effort"` // Total SPs in this phase
	Reasoning   string   `json:"reasoning"`    // Why this grouping
}

// DependencyEdge represents an inferred dependency between two epics.
type DependencyEdge struct {
	From       string  `json:"from"`       // Epic code (source)
	To         string  `json:"to"`         // Epic code (target)
	Type       string  `json:"type"`       // "foundation", "technology", "pattern"
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Reasoning  string  `json:"reasoning"`  // Why this dependency was inferred
}

// CriticalPathReport is the complete critical path analysis result.
type CriticalPathReport struct {
	Items        []CriticalPathItem `json:"items"`        // Ordered by phase then priority
	Phases       []ExecutionPhase   `json:"phases"`       // Grouped execution phases
	Dependencies []DependencyEdge   `json:"dependencies"` // Inferred dependency graph
	Summary      string             `json:"summary"`      // e.g. "3 phases, 2 foundation epics, 5 dependencies"
}

// Backlog representa o backlog gerado
type Backlog struct {
	Epics              []Epic              `json:"epics"`
	DeepDives          []DeepDive          `json:"deep_dives,omitempty"`
	PatternSuggestions []PatternSuggestion `json:"pattern_suggestions,omitempty"`
	CoherenceIssues    []CoherenceIssue    `json:"coherence_issues,omitempty"`
	FeasibilityReport  *FeasibilityReport  `json:"feasibility_report,omitempty"`
	CriticalPathReport *CriticalPathReport `json:"critical_path_report,omitempty"`
	InfraContext       *InfraContextData   `json:"infra_context,omitempty"`
	Meta               Metadata            `json:"meta"`
}

// InfraContextData is a serializable summary of infrastructure context.
type InfraContextData struct {
	Provider      string            `json:"provider"`
	Cluster       string            `json:"cluster"`
	Namespace     string            `json:"namespace"`
	FetchedAt     string            `json:"fetched_at"`
	NodeCount     int               `json:"node_count"`
	PodCount      int               `json:"pod_count"`
	ServiceCount  int               `json:"service_count"`
	AlertCount    int               `json:"alert_count"`
	TechsDetected map[string]string `json:"techs_detected,omitempty"`
	HealthSummary map[string]string `json:"health_summary,omitempty"`
}

// Epic representa um épico
type Epic struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`        // Código único do épico
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Stories     []Story  `json:"stories"`
	Tags        []string `json:"tags"`        // Tags simples como strings
	Priority    string   `json:"priority,omitempty"`
	Complexity  int      `json:"complexity,omitempty"` // 1-10
}

// Story representa uma história de usuário
type Story struct {
	ID                 string   `json:"id"`
	EpicID             string   `json:"epic_id"`
	Title              string   `json:"title"`
	What               string   `json:"what"`
	Why                string   `json:"why"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Tags               []string `json:"tags"`        // Tags simples como strings
	Effort             int      `json:"effort"`
	Status             string   `json:"status"` // todo, in_progress, done
	Tasks              []Task   `json:"tasks,omitempty"`
}

// Task representa uma task (2-4h)
type Task struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Effort      string `json:"effort"`
}

// Tag representa uma tag com rastreabilidade
type Tag struct {
	Name       string `json:"name"`
	Source     string `json:"source"`      // "golden_path", "input", "inferred"
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Confidence string `json:"confidence"`  // "explicit", "implicit", "inferred"
}

// DeepDive representa um deep dive técnico
type DeepDive struct {
	StoryID          string          `json:"story_id,omitempty"`          // História relacionada (NOVO)
	Term             string          `json:"term"`
	Terms            []DeepDiveTerm  `json:"terms,omitempty"`             // Lista de termos relacionados
	WhatIs           string          `json:"what_is"`
	WhatInThisStory  string          `json:"what_in_this_story,omitempty"` // O que faz NESTA história (NOVO)
	WhyHere          string          `json:"why_here"`
	WhyInThisStory   string          `json:"why_in_this_story,omitempty"`  // Por que NESTA história (NOVO)
	Configuration    string          `json:"configuration,omitempty"`
	Patterns         []string        `json:"patterns,omitempty"`           // Padrões recomendados
	SourcePatterns   []string        `json:"source_patterns,omitempty"`    // IDs dos patterns (GP-001, etc) (NOVO)
	RelatedCriteria  []string        `json:"related_criteria,omitempty"`   // Critérios relacionados (NOVO)
	Decisions        []string        `json:"decisions,omitempty"`          // Decisões técnicas
	RelatedTerms     []string        `json:"related_terms,omitempty"`
	Classification   string          `json:"classification,omitempty"`     // trivial/standard/specific/critical
	Scope            string          `json:"scope,omitempty"`              // epic/story/global
	InfraConfig      string          `json:"infra_config,omitempty"`       // Infrastructure configuration from real data
	InfraAlerts      []string        `json:"infra_alerts,omitempty"`       // Infrastructure alerts for this tech
	Source           Tag             `json:"source"`
}

// DeepDiveTerm termo dentro de deep dive
type DeepDiveTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
	WhyChosen  string `json:"why_chosen"`
}

// Metadata representa metadados da geração
type Metadata struct {
	GeneratedAt  string              `json:"generated_at"`
	Provider     string              `json:"provider"`
	InputFile    string              `json:"input_file"`
	ProjectTitle string              `json:"project_title,omitempty"` // Título inferido do projeto
	Lang         string              `json:"lang,omitempty"`          // Idioma da UI (pt-BR, en). Default: pt-BR
	TotalEpics   int                 `json:"total_epics"`            // Acesso direto
	TotalStories int                 `json:"total_stories"`          // Acesso direto
	Stats        GenerationStats     `json:"stats"`
	Metrics      *GenerationMetrics  `json:"metrics,omitempty"`      // Métricas de geração (deep dives, custos)
}

// GenerationMetrics métricas serializáveis do pipeline de geração
type GenerationMetrics struct {
	// Deep Dive extraction & classification
	TotalTechsExtracted   int            `json:"total_techs_extracted"`
	TrivialFiltered       int            `json:"trivial_filtered"`
	ClassificationStats   map[string]int `json:"classification_stats,omitempty"`   // trivial/standard/specific/critical → count
	CrossEpicGlobalDives  int            `json:"cross_epic_global_dives"`
	CrossEpicDeduplicated int            `json:"cross_epic_deduplicated"`

	// LLM performance
	LLMCallsMade    int     `json:"llm_calls_made"`
	LLMCallsSaved   int     `json:"llm_calls_saved"`
	ReductionPercent float64 `json:"reduction_percent"`

	// Cost tracking
	TotalInputTokens  int     `json:"total_input_tokens"`
	TotalOutputTokens int     `json:"total_output_tokens"`
	TotalCost         float64 `json:"total_cost"`
}

// GenerationStats estatísticas da geração
type GenerationStats struct {
	TotalEpics      int `json:"total_epics"`
	TotalStories    int `json:"total_stories"`
	TotalCriteria   int `json:"total_criteria"`
	TotalDeepDives  int `json:"total_deep_dives"`
	TotalStoryPoints int `json:"total_story_points"`
}

// Session representa uma sessão de geração
type Session struct {
	ID          string `json:"id"`
	InputFile   string `json:"input_file"`
	InputHash   string `json:"input_hash"`
	Phase       string `json:"phase"`
	Progress    int    `json:"progress"`
	Backlog     Backlog `json:"backlog"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ProjectInput representa o input parseado
type ProjectInput struct {
	Context      string            `json:"context"`
	Volumetry    string            `json:"volumetry"`
	NFRs         string            `json:"nfrs"`
	Stack        string            `json:"stack"`
	DataFlow     string            `json:"data_flow"`
	BusinessRules string           `json:"business_rules"`
	EdgeCases    string            `json:"edge_cases,omitempty"`
	Integrations string            `json:"integrations,omitempty"`
	RawContent   string            `json:"raw_content"`
	Metadata     map[string]string `json:"metadata"`
}

// Specification representa especificação completa (usado em commands)
type Specification struct {
	Gaps           []Gap                  `json:"gaps,omitempty"`
	Epics          []Epic                 `json:"epics"`
	StackDecisions map[string]interface{} `json:"stack_decisions"` // Decisões de stack técnico
}

// ProjectConfig configuração de projeto (usado em commands)
type ProjectConfig struct {
	ProjectName      string
	GoldenPathFile   string
	TeamConfigFile   string
	ProjectInputFile string
}

// SaveToFile salva backlog em arquivo JSON
func (b *Backlog) SaveToFile(path string) error {
	// Criar diretório se não existe
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %w", err)
	}

	// Marshal com indentação
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar backlog: %w", err)
	}

	// Escrever arquivo
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("erro ao escrever arquivo: %w", err)
	}

	return nil
}

// Gap representa um gap identificado
type Gap struct {
	Type     string `json:"type"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// Resolution representa resolução de gap
type Resolution struct {
	GapType string `json:"gap_type"`
	Value   string `json:"value"`
}

// SaveToFile salva specification em arquivo
func (s *Specification) SaveToFile(path string) error {
	return nil
}

// TaskBreakdown representa breakdown de tasks
type TaskBreakdown struct {
	StoryID string `json:"story_id"`
	Tasks   []Task `json:"tasks"`
}

// TeamConfig alias para TeamPatterns
type TeamConfig = TeamPatterns

// Pattern representa um pattern documentado (Golden Path ou Team Pattern)
type Pattern struct {
	ID          string   `json:"id"`          // GP-001, TP-003, etc
	Name        string   `json:"name"`        // Nome do pattern
	Description string   `json:"description"` // Descrição
	When        string   `json:"when"`        // Quando usar
	How         string   `json:"how"`         // Como implementar
	Decisions   []string `json:"decisions"`   // Decisões tomadas
	Validated   string   `json:"validated"`   // Onde foi validado
}

// GoldenPath representa patterns validados
type GoldenPath struct {
	Patterns map[string]Pattern `json:"patterns"` // Indexado por ID (GP-001, etc)
}

// TeamPatterns representa decisões da equipe
type TeamPatterns struct {
	Patterns map[string]Pattern `json:"patterns"` // Indexado por ID (TP-001, etc)
}
