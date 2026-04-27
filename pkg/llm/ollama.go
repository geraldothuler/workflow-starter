package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultOllamaEndpoint = "http://localhost:11434"
	defaultOllamaModel    = "llama3"
	ollamaTimeout         = 300 * time.Second // Modelos locais podem ser lentos
)

// OllamaProvider implementa LLMProvider para Ollama (modelos locais)
type OllamaProvider struct {
	endpoint     string
	model        string
	client       *http.Client
	systemPrompt string
}

// ollamaRequest formato de requisição da API Ollama
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
}

// ollamaResponse formato de resposta da API Ollama
type ollamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
	// Métricas (quando done=true)
	TotalDuration      int64 `json:"total_duration"`
	LoadDuration       int64 `json:"load_duration"`
	PromptEvalCount    int   `json:"prompt_eval_count"`
	PromptEvalDuration int64 `json:"prompt_eval_duration"`
	EvalCount          int   `json:"eval_count"`
	EvalDuration       int64 `json:"eval_duration"`
}

// NewOllamaProvider cria provider Ollama.
// endpoint: URL do Ollama (default: http://localhost:11434, ou OLLAMA_ENDPOINT env var)
// model: modelo a usar (default: llama3, ou OLLAMA_MODEL env var)
func NewOllamaProvider(endpoint, model string) (*OllamaProvider, error) {
	if endpoint == "" {
		endpoint = os.Getenv("OLLAMA_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = defaultOllamaEndpoint
	}

	if model == "" {
		model = os.Getenv("OLLAMA_MODEL")
	}
	if model == "" {
		model = defaultOllamaModel
	}

	return &OllamaProvider{
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{Timeout: ollamaTimeout},
	}, nil
}

// ProviderName retorna "ollama" (implementa LLMProvider)
func (o *OllamaProvider) ProviderName() string {
	return "ollama"
}

// ModelID retorna o modelo configurado (implementa LLMProvider)
func (o *OllamaProvider) ModelID() string {
	return o.model
}

// SetSystemPrompt define system prompt (implementa LLMProvider)
func (o *OllamaProvider) SetSystemPrompt(system string) {
	o.systemPrompt = system
}

// Complete envia prompt e retorna resposta (implementa Completer)
func (o *OllamaProvider) Complete(prompt string, maxTokens int) (string, error) {
	response, _, err := o.CompleteWithUsage(prompt, maxTokens)
	return response, err
}

// CompleteWithUsage envia prompt e retorna resposta + usage (implementa Completer)
func (o *OllamaProvider) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	url := o.endpoint + "/api/generate"

	reqBody := ollamaRequest{
		Model:  o.model,
		Prompt: prompt,
		System: o.systemPrompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("ollama: erro ao serializar request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, fmt.Errorf("ollama: erro ao criar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("ollama: erro de conexão (servidor rodando em %s?): %w", o.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("ollama: API error %d: %s", resp.StatusCode, string(body))
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, fmt.Errorf("ollama: erro ao decodificar resposta: %w", err)
	}

	if result.Response == "" {
		return "", nil, fmt.Errorf("ollama: resposta vazia")
	}

	// Ollama retorna contagens de tokens nas métricas
	usage := &Usage{
		InputTokens:  result.PromptEvalCount,
		OutputTokens: result.EvalCount,
		TotalTokens:  result.PromptEvalCount + result.EvalCount,
		Cost:         0, // Ollama é local, custo zero
	}

	return result.Response, usage, nil
}
