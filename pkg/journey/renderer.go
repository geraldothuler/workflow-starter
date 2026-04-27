package journey

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OutputMode define o formato de saída
type OutputMode string

const (
	ModeCLI OutputMode = "cli" // Terminal interativo (texto formatado)
	ModeMCP OutputMode = "mcp" // MCP Server (JSON estruturado)
	ModeAPI OutputMode = "api" // HTTP API (JSON)
)

// Renderer converte NextStepResult para diferentes formatos de saída
type Renderer struct {
	Mode OutputMode
}

// NewRenderer cria um renderer para o modo especificado
func NewRenderer(mode OutputMode) *Renderer {
	return &Renderer{Mode: mode}
}

// RenderStep renderiza um passo do journey no formato apropriado
func (r *Renderer) RenderStep(def *JourneyDefinition, state *JourneyState, next *NextStepResult) string {
	switch r.Mode {
	case ModeMCP, ModeAPI:
		return r.renderJSON(def, state, next)
	default:
		return r.renderCLI(def, state, next)
	}
}

// RenderList renderiza a lista de journeys disponíveis
func (r *Renderer) RenderList(journeys []*JourneyDefinition) string {
	switch r.Mode {
	case ModeMCP, ModeAPI:
		return r.renderListJSON(journeys)
	default:
		return r.renderListCLI(journeys)
	}
}

// --- CLI Rendering ---

func (r *Renderer) renderCLI(def *JourneyDefinition, state *JourneyState, next *NextStepResult) string {
	var b strings.Builder

	if next.Done {
		r.renderCLIComplete(&b, def, state, next)
		return b.String()
	}

	r.renderCLIStep(&b, def, state, next)
	return b.String()
}

func (r *Renderer) renderCLIStep(b *strings.Builder, def *JourneyDefinition, state *JourneyState, next *NextStepResult) {
	// Header
	fmt.Fprintf(b, "━━━ %s ━━━\n", def.Title)
	fmt.Fprintf(b, "Phase %d/%d: %s\n", next.PhaseNum, next.TotalPhases, next.PhaseTitle)
	b.WriteString(r.progressBar(next.PhaseNum, next.TotalPhases))
	b.WriteString("\n\n")

	q := next.Question
	if q == nil {
		return
	}

	// Question
	fmt.Fprintf(b, "❓ %s\n", q.Prompt)

	// Why we ask (Socratic context)
	if q.WhyAsk != "" {
		fmt.Fprintf(b, "   💡 %s\n", q.WhyAsk)
	}

	b.WriteString("\n")

	// Type-specific hints
	switch q.Type {
	case "select":
		for i, opt := range q.Options {
			fmt.Fprintf(b, "   [%d] %s\n", i+1, opt)
		}
		b.WriteString("\n")
	case "multiline":
		if q.Placeholder != "" {
			fmt.Fprintf(b, "   📝 %s\n", q.Placeholder)
		}
	case "text":
		if q.Placeholder != "" {
			fmt.Fprintf(b, "   📝 %s\n", q.Placeholder)
		}
	}

	// Default value hint
	if q.Default != "" {
		fmt.Fprintf(b, "   Default: %s\n", q.Default)
	}

	// Required indicator
	if q.Required {
		b.WriteString("   * Required\n")
	}
}

func (r *Renderer) renderCLIComplete(b *strings.Builder, def *JourneyDefinition, state *JourneyState, next *NextStepResult) {
	fmt.Fprintf(b, "━━━ %s — Complete ✅ ━━━\n\n", def.Title)

	if next.Summary != "" {
		b.WriteString(next.Summary)
		b.WriteString("\n")
	}

	if len(next.Insights) > 0 {
		b.WriteString("### Insights\n")
		for _, insight := range next.Insights {
			fmt.Fprintf(b, "- 💡 %s\n", insight)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(b, "📊 %d responses across %d phases\n", len(state.Responses), next.TotalPhases)
}

func (r *Renderer) progressBar(current, total int) string {
	if total <= 0 {
		return ""
	}

	width := 20
	filled := (current * width) / total
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %d%%", bar, (current*100)/total)
}

func (r *Renderer) renderListCLI(journeys []*JourneyDefinition) string {
	var b strings.Builder
	b.WriteString("━━━ Available Journeys ━━━\n\n")

	for i, j := range journeys {
		fmt.Fprintf(&b, "%d. %s\n", i+1, j.Title)
		fmt.Fprintf(&b, "   Name: %s\n", j.Name)
		fmt.Fprintf(&b, "   %s\n", j.Description)
		fmt.Fprintf(&b, "   Phases: %d\n", len(j.Phases))

		// Count total questions
		total := 0
		for _, p := range j.Phases {
			total += len(p.Questions)
		}
		fmt.Fprintf(&b, "   Questions: %d\n", total)
		b.WriteString("\n")
	}

	return b.String()
}

// --- JSON Rendering (MCP + API) ---

// MCPStepOutput é o formato JSON para um passo MCP
type MCPStepOutput struct {
	SessionID   string            `json:"session_id"`
	JourneyName string            `json:"journey_name"`
	Status      string            `json:"status"` // "active", "completed"
	Progress    MCPProgress       `json:"progress"`
	Question    *MCPQuestion      `json:"question,omitempty"`
	Summary     string            `json:"summary,omitempty"`
	Insights    []string          `json:"insights,omitempty"`
	Context     map[string]string `json:"context,omitempty"`
	NextAction  string            `json:"next_action"` // "answer", "done"
}

// MCPProgress mostra o progresso no journey
type MCPProgress struct {
	Phase      int    `json:"phase"`
	TotalPhases int   `json:"total_phases"`
	PhaseTitle string `json:"phase_title"`
	Percent    int    `json:"percent"`
}

// MCPQuestion é a representação JSON de uma question
type MCPQuestion struct {
	ID          string   `json:"id"`
	Prompt      string   `json:"prompt"`
	WhyAsk      string   `json:"why_ask,omitempty"`
	Type        string   `json:"type"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
}

// MCPJourneyListOutput é a lista de journeys em JSON
type MCPJourneyListOutput struct {
	Journeys []MCPJourneyInfo `json:"journeys"`
	Count    int              `json:"count"`
}

// MCPJourneyInfo representa um journey na lista
type MCPJourneyInfo struct {
	Name           string   `json:"name"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	PhaseCount     int      `json:"phase_count"`
	QuestionCount  int      `json:"question_count"`
	PhaseNames     []string `json:"phase_names"`
}

func (r *Renderer) renderJSON(def *JourneyDefinition, state *JourneyState, next *NextStepResult) string {
	output := MCPStepOutput{
		SessionID:   state.ID,
		JourneyName: state.JourneyName,
		Status:      state.Status,
		Progress: MCPProgress{
			Phase:       next.PhaseNum,
			TotalPhases: next.TotalPhases,
			PhaseTitle:  next.PhaseTitle,
		},
		Context: state.Context,
	}

	if next.TotalPhases > 0 {
		output.Progress.Percent = (next.PhaseNum * 100) / next.TotalPhases
	}

	if next.Done {
		output.NextAction = "done"
		output.Summary = next.Summary
		output.Insights = next.Insights
		output.Progress.Percent = 100
	} else {
		output.NextAction = "answer"
		if next.Question != nil {
			output.Question = &MCPQuestion{
				ID:          next.Question.ID,
				Prompt:      next.Question.Prompt,
				WhyAsk:      next.Question.WhyAsk,
				Type:        next.Question.Type,
				Options:     next.Question.Options,
				Required:    next.Question.Required,
				Default:     next.Question.Default,
				Placeholder: next.Question.Placeholder,
			}
		}
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	return string(data)
}

func (r *Renderer) renderListJSON(journeys []*JourneyDefinition) string {
	infos := make([]MCPJourneyInfo, 0, len(journeys))

	for _, j := range journeys {
		totalQ := 0
		phaseNames := make([]string, 0, len(j.Phases))
		for _, p := range j.Phases {
			totalQ += len(p.Questions)
			phaseNames = append(phaseNames, p.Title)
		}

		infos = append(infos, MCPJourneyInfo{
			Name:          j.Name,
			Title:         j.Title,
			Description:   j.Description,
			PhaseCount:    len(j.Phases),
			QuestionCount: totalQ,
			PhaseNames:    phaseNames,
		})
	}

	output := MCPJourneyListOutput{
		Journeys: infos,
		Count:    len(infos),
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	return string(data)
}
