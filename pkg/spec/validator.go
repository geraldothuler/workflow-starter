package spec

import (
	"fmt"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// Validator valida especificação
type Validator struct {
	goldenPath *types.GoldenPath
}

// NewValidator cria validador
func NewValidator(gp *types.GoldenPath) *Validator {
	return &Validator{goldenPath: gp}
}

// Validate valida input
func (v *Validator) Validate(input *types.ProjectInput) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []string{},
		Warnings: []string{},
		Score: 100,
	}

	// Validar seções obrigatórias
	if input.Context == "" {
		result.Errors = append(result.Errors, "Contexto ausente")
		result.Score -= 20
		result.Valid = false
	}

	if input.Volumetry == "" {
		result.Errors = append(result.Errors, "Volumetria ausente")
		result.Score -= 20
		result.Valid = false
	}

	if input.NFRs == "" {
		result.Warnings = append(result.Warnings, "RNFs não especificados")
		result.Score -= 10
	}

	if input.Stack == "" {
		result.Errors = append(result.Errors, "Stack técnico ausente")
		result.Score -= 20
		result.Valid = false
	}

	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

// ValidationResult resultado da validação
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
	Score    int      `json:"score"`
}

// Generator gera specification
type Generator struct {
	merged *types.Specification
	gp     *types.GoldenPath
	tc     *types.TeamPatterns
	pi     *types.ProjectInput
}

// NewGenerator cria generator
func NewGenerator(merged *types.Specification, gp *types.GoldenPath, tc *types.TeamPatterns, pi *types.ProjectInput) *Generator {
	return &Generator{merged: merged, gp: gp, tc: tc, pi: pi}
}

// Generate gera specification enriquecida a partir do merge
func (g *Generator) Generate() (*types.Specification, error) {
	if g.merged == nil {
		return nil, fmt.Errorf("no merged specification provided")
	}

	spec := g.merged

	// Enrich: inferir stack decisions do input se vazio
	if spec.StackDecisions == nil && g.pi != nil && g.pi.Stack != "" {
		spec.StackDecisions = map[string]interface{}{
			"stack": g.pi.Stack,
		}
	}

	return spec, nil
}

// Report gera relatório de qualidade da especificação
func (g *Generator) Report() *SpecReport {
	report := &SpecReport{
		Sections: []SectionScore{},
		Score:    100,
	}

	if g.merged == nil {
		report.Score = 0
		report.Recommendations = append(report.Recommendations, "Nenhuma especificação disponível")
		return report
	}

	// Avaliar épicos
	epicScore := 100
	if len(g.merged.Epics) == 0 {
		epicScore = 0
		report.Recommendations = append(report.Recommendations, "Adicionar épicos à especificação")
	}
	report.Sections = append(report.Sections, SectionScore{
		Name:  "Epics",
		Score: epicScore,
		Items: len(g.merged.Epics),
	})

	// Avaliar gaps
	gapScore := 100
	if len(g.merged.Gaps) > 0 {
		gapScore = max(0, 100-len(g.merged.Gaps)*20)
		report.Recommendations = append(report.Recommendations,
			fmt.Sprintf("Resolver %d gaps pendentes", len(g.merged.Gaps)))
	}
	report.Sections = append(report.Sections, SectionScore{
		Name:  "Gaps",
		Score: gapScore,
		Items: len(g.merged.Gaps),
	})

	// Avaliar stack decisions
	stackScore := 100
	if g.merged.StackDecisions == nil || len(g.merged.StackDecisions) == 0 {
		stackScore = 0
		report.Recommendations = append(report.Recommendations, "Definir decisões de stack")
	}
	report.Sections = append(report.Sections, SectionScore{
		Name:  "Stack Decisions",
		Score: stackScore,
		Items: len(g.merged.StackDecisions),
	})

	// Avaliar input
	if g.pi != nil {
		inputScore := evaluateInput(g.pi)
		report.Sections = append(report.Sections, SectionScore{
			Name:  "Input Quality",
			Score: inputScore,
		})
	}

	// Calcular score geral
	total := 0
	for _, s := range report.Sections {
		total += s.Score
	}
	if len(report.Sections) > 0 {
		report.Score = total / len(report.Sections)
	}

	return report
}

func evaluateInput(pi *types.ProjectInput) int {
	score := 100
	if pi.Context == "" {
		score -= 25
	}
	if pi.Volumetry == "" {
		score -= 25
	}
	if pi.NFRs == "" {
		score -= 15
	}
	if pi.Stack == "" {
		score -= 20
	}
	if pi.BusinessRules == "" {
		score -= 15
	}
	if score < 0 {
		score = 0
	}
	return score
}

// FormatReport formata report como string
func (r *SpecReport) FormatReport() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Relatório de Qualidade da Especificação\n\n")
	fmt.Fprintf(&b, "**Score Geral:** %d/100\n\n", r.Score)

	b.WriteString("## Seções\n\n")
	b.WriteString("| Seção | Score | Items |\n")
	b.WriteString("|-------|-------|-------|\n")
	for _, s := range r.Sections {
		fmt.Fprintf(&b, "| %s | %d/100 | %d |\n", s.Name, s.Score, s.Items)
	}

	if len(r.Recommendations) > 0 {
		b.WriteString("\n## Recomendações\n\n")
		for _, rec := range r.Recommendations {
			fmt.Fprintf(&b, "- %s\n", rec)
		}
	}

	return b.String()
}

// SpecReport relatório de qualidade da especificação
type SpecReport struct {
	Score           int            `json:"score"`
	Sections        []SectionScore `json:"sections"`
	Recommendations []string       `json:"recommendations"`
}

// SectionScore score por seção
type SectionScore struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
	Items int    `json:"items"`
}
