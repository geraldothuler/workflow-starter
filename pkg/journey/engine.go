package journey

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Phase representa uma fase de um journey
type Phase struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Order       int        `json:"order"`
	Questions   []Question `json:"questions"`
}

// Question representa uma pergunta dentro de uma fase
type Question struct {
	ID          string   `json:"id"`
	Prompt      string   `json:"prompt"`
	WhyAsk      string   `json:"why_ask,omitempty"` // Socratic context: WHY we ask this
	Type        string   `json:"type"`              // "text", "multiline", "select", "confirm"
	Options     []string `json:"options,omitempty"`  // For "select" type
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
}

// StepResponse registra a resposta do usuário a uma pergunta
type StepResponse struct {
	PhaseID    string    `json:"phase_id"`
	QuestionID string   `json:"question_id"`
	Answer     string    `json:"answer"`
	Timestamp  time.Time `json:"timestamp"`
}

// JourneyState armazena o estado persistente de um journey em execução
type JourneyState struct {
	ID          string         `json:"id"`
	JourneyName string         `json:"journey_name"`
	Phase       int            `json:"phase"`       // Índice da fase atual (0-based)
	Step        int            `json:"step"`        // Índice da pergunta atual na fase (0-based)
	Responses   []StepResponse `json:"responses"`
	Context     map[string]string `json:"context"` // Respostas acumuladas key=questionID
	Status      string         `json:"status"`      // "active", "completed", "abandoned"
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// JourneyDefinition define a estrutura de um journey completo
type JourneyDefinition struct {
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Phases      []Phase `json:"phases"`
}

// NextStepResult é o resultado de processar um passo
type NextStepResult struct {
	Done       bool      `json:"done"`                  // Journey completo?
	PhaseTitle string    `json:"phase_title,omitempty"`  // Título da fase atual
	PhaseNum   int       `json:"phase_num"`              // Número da fase (1-based)
	TotalPhases int      `json:"total_phases"`
	Question   *Question `json:"question,omitempty"`     // Próxima pergunta (nil se done)
	Summary    string    `json:"summary,omitempty"`      // Resumo final (se done)
	Insights   []string  `json:"insights,omitempty"`     // Insights acumulados
}

// Engine gerencia a execução de journeys
type Engine struct {
	definitions map[string]*JourneyDefinition
	stateDir    string // Diretório para persistir estados

	// Injetáveis para testes
	nowFunc   func() time.Time
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
}

// NewEngine cria um novo engine de journeys
func NewEngine(stateDir string) *Engine {
	e := &Engine{
		definitions: make(map[string]*JourneyDefinition),
		stateDir:    stateDir,
		nowFunc:     time.Now,
		readFile:    os.ReadFile,
		writeFile:   os.WriteFile,
		mkdirAll:    os.MkdirAll,
	}

	// Registrar journeys built-in
	for _, jd := range BuiltinJourneys() {
		e.Register(jd)
	}

	return e
}

// Register adiciona uma definição de journey ao engine
func (e *Engine) Register(def *JourneyDefinition) {
	e.definitions[def.Name] = def
}

// ListJourneys retorna todas as definições registradas
func (e *Engine) ListJourneys() []*JourneyDefinition {
	result := make([]*JourneyDefinition, 0, len(e.definitions))
	for _, def := range e.definitions {
		result = append(result, def)
	}
	return result
}

// GetDefinition retorna uma definição por nome
func (e *Engine) GetDefinition(name string) (*JourneyDefinition, bool) {
	def, ok := e.definitions[name]
	return def, ok
}

// Start inicia um novo journey e retorna o primeiro passo
func (e *Engine) Start(journeyName string) (*JourneyState, *NextStepResult, error) {
	def, ok := e.definitions[journeyName]
	if !ok {
		return nil, nil, fmt.Errorf("journey not found: %s", journeyName)
	}

	if len(def.Phases) == 0 {
		return nil, nil, fmt.Errorf("journey %s has no phases", journeyName)
	}

	state := &JourneyState{
		ID:          fmt.Sprintf("%s-%d", journeyName, e.nowFunc().UnixMilli()),
		JourneyName: journeyName,
		Phase:       0,
		Step:        0,
		Responses:   []StepResponse{},
		Context:     make(map[string]string),
		Status:      "active",
		CreatedAt:   e.nowFunc(),
		UpdatedAt:   e.nowFunc(),
	}

	// Salvar estado inicial
	if err := e.saveState(state); err != nil {
		return nil, nil, fmt.Errorf("error saving state: %w", err)
	}

	// Retornar primeira pergunta
	next := e.currentStep(def, state)
	return state, next, nil
}

// Next processa uma resposta e retorna o próximo passo
func (e *Engine) Next(sessionID string, answer string) (*JourneyState, *NextStepResult, error) {
	state, err := e.loadState(sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading session: %w", err)
	}

	if state.Status != "active" {
		return state, &NextStepResult{Done: true, Summary: "Journey already completed"}, nil
	}

	def, ok := e.definitions[state.JourneyName]
	if !ok {
		return nil, nil, fmt.Errorf("journey definition not found: %s", state.JourneyName)
	}

	// Registrar resposta
	phase := def.Phases[state.Phase]
	question := phase.Questions[state.Step]

	state.Responses = append(state.Responses, StepResponse{
		PhaseID:    phase.ID,
		QuestionID: question.ID,
		Answer:     answer,
		Timestamp:  e.nowFunc(),
	})
	state.Context[question.ID] = answer
	state.UpdatedAt = e.nowFunc()

	// Avançar para próximo step
	state.Step++

	// Se excedeu perguntas da fase, avançar fase
	if state.Step >= len(phase.Questions) {
		state.Step = 0
		state.Phase++
	}

	// Se excedeu fases, journey completo
	if state.Phase >= len(def.Phases) {
		state.Status = "completed"
		if err := e.saveState(state); err != nil {
			return nil, nil, fmt.Errorf("error saving state: %w", err)
		}

		return state, &NextStepResult{
			Done:        true,
			TotalPhases: len(def.Phases),
			Summary:     e.buildSummary(def, state),
			Insights:    e.buildInsights(def, state),
		}, nil
	}

	// Salvar e retornar próximo step
	if err := e.saveState(state); err != nil {
		return nil, nil, fmt.Errorf("error saving state: %w", err)
	}

	next := e.currentStep(def, state)
	return state, next, nil
}

// Status retorna o estado atual de um journey
func (e *Engine) Status(sessionID string) (*JourneyState, *NextStepResult, error) {
	state, err := e.loadState(sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading session: %w", err)
	}

	def, ok := e.definitions[state.JourneyName]
	if !ok {
		return nil, nil, fmt.Errorf("journey definition not found: %s", state.JourneyName)
	}

	if state.Status != "active" {
		return state, &NextStepResult{
			Done:        true,
			TotalPhases: len(def.Phases),
			Summary:     e.buildSummary(def, state),
		}, nil
	}

	next := e.currentStep(def, state)
	return state, next, nil
}

// currentStep retorna o NextStepResult para o estado atual
func (e *Engine) currentStep(def *JourneyDefinition, state *JourneyState) *NextStepResult {
	phase := def.Phases[state.Phase]
	question := phase.Questions[state.Step]

	return &NextStepResult{
		Done:        false,
		PhaseTitle:  phase.Title,
		PhaseNum:    state.Phase + 1,
		TotalPhases: len(def.Phases),
		Question:    &question,
	}
}

// buildSummary gera resumo do journey completo
func (e *Engine) buildSummary(def *JourneyDefinition, state *JourneyState) string {
	summary := fmt.Sprintf("## %s — Complete\n\n", def.Title)

	for _, phase := range def.Phases {
		summary += fmt.Sprintf("### %s\n", phase.Title)
		for _, q := range phase.Questions {
			if answer, ok := state.Context[q.ID]; ok {
				summary += fmt.Sprintf("- **%s**: %s\n", q.Prompt, answer)
			}
		}
		summary += "\n"
	}

	return summary
}

// buildInsights gera insights baseados nas respostas
func (e *Engine) buildInsights(def *JourneyDefinition, state *JourneyState) []string {
	insights := []string{}

	responseCount := len(state.Responses)
	if responseCount > 0 {
		insights = append(insights, fmt.Sprintf("%d questions answered across %d phases", responseCount, len(def.Phases)))
	}

	// Detectar respostas curtas que podem precisar aprofundamento
	shortAnswers := 0
	for _, r := range state.Responses {
		if len(r.Answer) < 10 {
			shortAnswers++
		}
	}
	if shortAnswers > 0 && shortAnswers > responseCount/2 {
		insights = append(insights, "Consider providing more detail in short answers for better results")
	}

	return insights
}

// saveState persiste o estado do journey em disco
func (e *Engine) saveState(state *JourneyState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := e.mkdirAll(e.stateDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(e.stateDir, state.ID+".json")
	return e.writeFile(path, data, 0644)
}

// loadState carrega o estado de um journey do disco
func (e *Engine) loadState(sessionID string) (*JourneyState, error) {
	path := filepath.Join(e.stateDir, sessionID+".json")
	data, err := e.readFile(path)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var state JourneyState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid session data: %w", err)
	}

	return &state, nil
}
