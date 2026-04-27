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
	azureAPIVersion = "2024-06-01"
	azureTimeout    = 120 * time.Second
)

// AzureProvider implementa LLMProvider para Azure OpenAI
type AzureProvider struct {
	endpoint     string // https://<resource>.openai.azure.com
	apiKey       string
	deployment   string // Nome do deployment no Azure
	model        string // Para referência/display (ex: "gpt-4")
	client       *http.Client
	systemPrompt string
}

// azureChatRequest formato compatível com OpenAI
type azureChatRequest struct {
	Messages    []azureChatMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
}

type azureChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type azureChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewAzureProvider cria provider Azure OpenAI.
// Parâmetros prioritários, com fallback para env vars:
//   - endpoint: AZURE_OPENAI_ENDPOINT
//   - apiKey: AZURE_OPENAI_API_KEY
//   - deployment: AZURE_OPENAI_DEPLOYMENT
//   - model: para display (default: "gpt-4")
func NewAzureProvider(endpoint, apiKey, deployment, model string) (*AzureProvider, error) {
	if endpoint == "" {
		endpoint = os.Getenv("AZURE_OPENAI_ENDPOINT")
	}
	if apiKey == "" {
		apiKey = os.Getenv("AZURE_OPENAI_API_KEY")
	}
	if deployment == "" {
		deployment = os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	}
	if model == "" {
		model = "gpt-4"
	}

	if endpoint == "" {
		return nil, fmt.Errorf("azure: endpoint não configurado (defina AZURE_OPENAI_ENDPOINT)")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("azure: API key não configurada (defina AZURE_OPENAI_API_KEY)")
	}
	if deployment == "" {
		return nil, fmt.Errorf("azure: deployment não configurado (defina AZURE_OPENAI_DEPLOYMENT)")
	}

	return &AzureProvider{
		endpoint:   endpoint,
		apiKey:     apiKey,
		deployment: deployment,
		model:      model,
		client:     &http.Client{Timeout: azureTimeout},
	}, nil
}

// ProviderName retorna "azure" (implementa LLMProvider)
func (a *AzureProvider) ProviderName() string {
	return "azure"
}

// ModelID retorna o modelo/deployment (implementa LLMProvider)
func (a *AzureProvider) ModelID() string {
	return a.model
}

// SetSystemPrompt define system prompt (implementa LLMProvider)
func (a *AzureProvider) SetSystemPrompt(system string) {
	a.systemPrompt = system
}

// Complete envia prompt e retorna resposta (implementa Completer)
func (a *AzureProvider) Complete(prompt string, maxTokens int) (string, error) {
	response, _, err := a.CompleteWithUsage(prompt, maxTokens)
	return response, err
}

// CompleteWithUsage envia prompt e retorna resposta + usage (implementa Completer)
func (a *AzureProvider) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		a.endpoint, a.deployment, azureAPIVersion)

	messages := []azureChatMessage{}
	if a.systemPrompt != "" {
		messages = append(messages, azureChatMessage{Role: "system", Content: a.systemPrompt})
	}
	messages = append(messages, azureChatMessage{Role: "user", Content: prompt})

	reqBody := azureChatRequest{
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("azure: erro ao serializar request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, fmt.Errorf("azure: erro ao criar request: %w", err)
	}

	// Azure usa api-key header (não Authorization: Bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("azure: erro de conexão: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("azure: API error %d: %s", resp.StatusCode, string(body))
	}

	var result azureChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, fmt.Errorf("azure: erro ao decodificar resposta: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", nil, fmt.Errorf("azure: resposta vazia")
	}

	// Custo similar ao OpenAI (GPT-4: $30/MTok input, $60/MTok output)
	inputCost := float64(result.Usage.PromptTokens) * 30.0 / 1_000_000
	outputCost := float64(result.Usage.CompletionTokens) * 60.0 / 1_000_000

	usage := &Usage{
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
		TotalTokens:  result.Usage.TotalTokens,
		Cost:         inputCost + outputCost,
	}

	return result.Choices[0].Message.Content, usage, nil
}
