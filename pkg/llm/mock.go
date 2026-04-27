package llm

import "sync"

// MockClient implementa LLMProvider para testes (superset de Completer)
type MockClient struct {
	mu sync.Mutex

	// Respostas configuráveis
	Responses []string // Respostas em sequência (FIFO)
	Error     error    // Erro fixo para todos os calls

	// Tracking
	Calls     []MockCall
	CallCount int

	// Callback customizado (tem prioridade sobre Responses/Error)
	CompleteFunc func(prompt string, maxTokens int) (string, *Usage, error)

	// LLMProvider fields
	provider     string
	model        string
	systemPrompt string
}

// MockCall registra uma chamada ao mock
type MockCall struct {
	Prompt    string
	MaxTokens int
}

// NewMockClient cria mock com respostas pré-definidas
func NewMockClient(responses ...string) *MockClient {
	return &MockClient{
		Responses: responses,
	}
}

// NewMockClientWithError cria mock que sempre retorna erro
func NewMockClientWithError(err error) *MockClient {
	return &MockClient{
		Error: err,
	}
}

// Complete implementa Completer
func (m *MockClient) Complete(prompt string, maxTokens int) (string, error) {
	response, _, err := m.CompleteWithUsage(prompt, maxTokens)
	return response, err
}

// CompleteWithUsage implementa Completer
func (m *MockClient) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, MockCall{Prompt: prompt, MaxTokens: maxTokens})
	m.CallCount++

	// Callback customizado tem prioridade
	if m.CompleteFunc != nil {
		return m.CompleteFunc(prompt, maxTokens)
	}

	// Erro fixo
	if m.Error != nil {
		return "", nil, m.Error
	}

	// Retornar próxima resposta da fila
	if len(m.Responses) == 0 {
		return "", &Usage{}, nil
	}

	idx := m.CallCount - 1
	if idx >= len(m.Responses) {
		idx = len(m.Responses) - 1 // Repete última resposta se exceder
	}

	usage := &Usage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		Cost:         0.001,
	}

	return m.Responses[idx], usage, nil
}

// ProviderName retorna nome do provider mock (implementa LLMProvider)
func (m *MockClient) ProviderName() string {
	if m.provider != "" {
		return m.provider
	}
	return "mock"
}

// ModelID retorna modelo do mock (implementa LLMProvider)
func (m *MockClient) ModelID() string {
	if m.model != "" {
		return m.model
	}
	return "mock-model"
}

// SetSystemPrompt armazena system prompt no mock (implementa LLMProvider)
func (m *MockClient) SetSystemPrompt(system string) {
	m.systemPrompt = system
}

// GetSystemPrompt retorna system prompt configurado (útil em testes)
func (m *MockClient) GetSystemPrompt() string {
	return m.systemPrompt
}

// NewMockProvider cria mock configurado como LLMProvider com nome/modelo
func NewMockProvider(provider, model string, responses ...string) *MockClient {
	return &MockClient{
		Responses: responses,
		provider:  provider,
		model:     model,
	}
}
