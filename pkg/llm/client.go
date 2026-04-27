package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/logging"
)

// Provider tipos de LLM
type Provider string

const (
	ProviderClaude  Provider = "claude"
	ProviderChatGPT Provider = "chatgpt"
	ProviderGemini  Provider = "gemini"
	ProviderOllama  Provider = "ollama"
	ProviderAzure   Provider = "azure"
)

// Completer interface para chamadas LLM (permite mocking em testes)
type Completer interface {
	Complete(prompt string, maxTokens int) (string, error)
	CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error)
}

// Client cliente LLM
type Client struct {
	Provider     Provider
	APIKey       string
	Model        string
	client       *http.Client
	sanitizer    *logging.Sanitizer
	systemPrompt string
}

// ProviderName retorna o nome do provider (implementa LLMProvider)
func (c *Client) ProviderName() string {
	return string(c.Provider)
}

// ModelID retorna o identificador do modelo (implementa LLMProvider)
func (c *Client) ModelID() string {
	return c.Model
}

// SetSystemPrompt define o system prompt para todas as chamadas subsequentes
func (c *Client) SetSystemPrompt(system string) {
	c.systemPrompt = system
}

// NewClient cria novo cliente LLM usando os.Getenv() para API keys.
// Deprecated: Use NewProvider() with ProviderConfig.CredResolver for secure credential resolution.
func NewClient(provider Provider) (*Client, error) {
	var apiKey, model string

	switch provider {
	case ProviderClaude:
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		model = "claude-sonnet-4-20250514"
	case ProviderChatGPT:
		apiKey = os.Getenv("OPENAI_API_KEY")
		model = "gpt-4"
	case ProviderGemini:
		apiKey = os.Getenv("GEMINI_API_KEY")
		model = "gemini-1.5-flash"
	default:
		return nil, fmt.Errorf("provider não suportado: %s", provider)
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key não configurada para %s", provider)
	}

	return newClientInternal(provider, apiKey, model)
}

// NewClientWithKey creates a client with an explicitly provided API key.
// This is used by NewProvider when credentials are resolved via the credential chain.
func NewClientWithKey(provider Provider, apiKey string) (*Client, error) {
	var model string

	switch provider {
	case ProviderClaude:
		model = "claude-sonnet-4-20250514"
	case ProviderChatGPT:
		model = "gpt-4"
	case ProviderGemini:
		model = "gemini-1.5-flash"
	default:
		return nil, fmt.Errorf("provider não suportado: %s", provider)
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key não configurada para %s", provider)
	}

	return newClientInternal(provider, apiKey, model)
}

func newClientInternal(provider Provider, apiKey, model string) (*Client, error) {
	return &Client{
		Provider:  provider,
		APIKey:    apiKey,
		Model:     model,
		client:    &http.Client{Timeout: 120 * time.Second},
		sanitizer: logging.NewSanitizer(true),
	}, nil
}

// Message representa uma mensagem
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest request para API (Claude/Anthropic)
type CompletionRequest struct {
	Model       string    `json:"model"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
}

// CompletionResponse resposta da API
type CompletionResponse struct {
	ID      string `json:"id"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Usage tracking de tokens e custo
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	Cost         float64 // Em USD
}

// Complete envia prompt e retorna resposta + usage
func (c *Client) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	switch c.Provider {
	case ProviderClaude:
		return c.completeClaudeWithUsage(prompt, maxTokens)
	case ProviderChatGPT:
		return c.completeChatGPTWithUsage(prompt, maxTokens)
	case ProviderGemini:
		return c.completeGeminiWithUsage(prompt, maxTokens)
	default:
		return "", nil, fmt.Errorf("provider não implementado: %s", c.Provider)
	}
}

// Complete mantém compatibilidade (sem usage)
func (c *Client) Complete(prompt string, maxTokens int) (string, error) {
	response, _, err := c.CompleteWithUsage(prompt, maxTokens)
	return response, err
}

func (c *Client) completeClaude(prompt string, maxTokens int) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	reqBody := CompletionRequest{
		Model:     c.Model,
		System:    c.systemPrompt,
		MaxTokens: maxTokens,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Retry logic: 3 tentativas com backoff exponencial
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Backoff: 2^attempt segundos
			waitTime := time.Duration(1<<uint(attempt)) * time.Second
			fmt.Printf("Tentativa %d/%d após %v...\n", attempt+1, maxRetries, waitTime)
			time.Sleep(waitTime)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.client.Do(req)
		if err != nil {
			if attempt == maxRetries-1 {
				return "", err
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 429 {
			// Rate limit - retry
			if attempt == maxRetries-1 {
				retryAfter := resp.Header.Get("Retry-After")
				return "", fmt.Errorf("rate limit: retry after %s", retryAfter)
			}
			continue
		}

		if resp.StatusCode == 500 || resp.StatusCode == 502 || resp.StatusCode == 503 {
			// Server errors - retry
			if attempt == maxRetries-1 {
				body, _ := io.ReadAll(resp.Body)
				return "", c.sanitizeError("API error %d: %s", resp.StatusCode, string(body))
			}
			continue
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return "", c.sanitizeError("API error %d: %s", resp.StatusCode, string(body))
		}

		var result CompletionResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}

		if len(result.Content) == 0 {
			return "", fmt.Errorf("resposta vazia da API")
		}

		return result.Content[0].Text, nil
	}

	return "", fmt.Errorf("falha após %d tentativas", maxRetries)
}

func (c *Client) completeClaudeWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	url := "https://api.anthropic.com/v1/messages"

	reqBody := CompletionRequest{
		Model:     c.Model,
		System:    c.systemPrompt,
		MaxTokens: maxTokens,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	// Retry logic
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			waitTime := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(waitTime)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.client.Do(req)
		if err != nil {
			if attempt == maxRetries-1 {
				return "", nil, err
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if attempt == maxRetries-1 {
				body, _ := io.ReadAll(resp.Body)
				return "", nil, c.sanitizeError("API error %d: %s", resp.StatusCode, string(body))
			}
			continue
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return "", nil, c.sanitizeError("API error %d: %s", resp.StatusCode, string(body))
		}

		var result CompletionResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", nil, err
		}

		if len(result.Content) == 0 {
			return "", nil, fmt.Errorf("resposta vazia da API")
		}

		// Calcular custo (Claude Sonnet 4: $3/MTok input, $15/MTok output)
		inputCost := float64(result.Usage.InputTokens) * 3.0 / 1_000_000
		outputCost := float64(result.Usage.OutputTokens) * 15.0 / 1_000_000

		usage := &Usage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
			TotalTokens:  result.Usage.InputTokens + result.Usage.OutputTokens,
			Cost:         inputCost + outputCost,
		}

		return result.Content[0].Text, usage, nil
	}

	return "", nil, fmt.Errorf("falha após %d tentativas", maxRetries)
}

func (c *Client) completeChatGPTWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	url := "https://api.openai.com/v1/chat/completions"

	type ChatGPTMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type ChatGPTRequest struct {
		Model       string           `json:"model"`
		Messages    []ChatGPTMessage `json:"messages"`
		MaxTokens   int              `json:"max_tokens"`
		Temperature float64          `json:"temperature,omitempty"`
	}

	type ChatGPTResponse struct {
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

	messages := []ChatGPTMessage{}
	if c.systemPrompt != "" {
		messages = append(messages, ChatGPTMessage{Role: "system", Content: c.systemPrompt})
	}
	messages = append(messages, ChatGPTMessage{Role: "user", Content: prompt})

	reqBody := ChatGPTRequest{
		Model:     c.Model,
		MaxTokens: maxTokens,
		Messages:  messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, c.sanitizeError("API error %d: %s", resp.StatusCode, string(body))
	}

	var result ChatGPTResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, err
	}

	if len(result.Choices) == 0 {
		return "", nil, fmt.Errorf("resposta vazia da API")
	}

	// Calcular custo (GPT-4: $30/MTok input, $60/MTok output)
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

func (c *Client) completeGeminiWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	// SECURITY: API key moved from URL query parameter to header to prevent
	// exposure in server logs, browser history, proxy logs, and referrer headers.
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", c.Model)

	type GeminiPart struct {
		Text string `json:"text"`
	}
	type GeminiContent struct {
		Parts []GeminiPart `json:"parts"`
	}
	type GeminiRequest struct {
		SystemInstruction *GeminiContent `json:"systemInstruction,omitempty"`
		Contents          []GeminiContent `json:"contents"`
		GenerationConfig  struct {
			MaxOutputTokens int     `json:"maxOutputTokens"`
			Temperature     float64 `json:"temperature,omitempty"`
		} `json:"generationConfig"`
	}

	type GeminiResponse struct {
		Candidates []struct {
			Content GeminiContent `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{Parts: []GeminiPart{{Text: prompt}}},
		},
	}
	if c.systemPrompt != "" {
		reqBody.SystemInstruction = &GeminiContent{
			Parts: []GeminiPart{{Text: c.systemPrompt}},
		}
	}
	reqBody.GenerationConfig.MaxOutputTokens = maxTokens

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.APIKey) // SECURITY: API key in header, not URL

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, c.sanitizeError("API error %d: %s", resp.StatusCode, string(body))
	}

	var result GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", nil, fmt.Errorf("resposta vazia da API")
	}

	// Calcular custo (gemini-1.5-flash: $0.075/MTok input, $0.30/MTok output)
	inputCost := float64(result.UsageMetadata.PromptTokenCount) * 0.075 / 1_000_000
	outputCost := float64(result.UsageMetadata.CandidatesTokenCount) * 0.30 / 1_000_000

	usage := &Usage{
		InputTokens:  result.UsageMetadata.PromptTokenCount,
		OutputTokens: result.UsageMetadata.CandidatesTokenCount,
		TotalTokens:  result.UsageMetadata.TotalTokenCount,
		Cost:         inputCost + outputCost,
	}

	return result.Candidates[0].Content.Parts[0].Text, usage, nil
}

func (c *Client) completeChatGPT(prompt string, maxTokens int) (string, error) {
	// Implementação simplificada do ChatGPT
	return "", fmt.Errorf("ChatGPT não implementado ainda")
}

func (c *Client) completeGemini(prompt string, maxTokens int) (string, error) {
	// Implementação simplificada do Gemini
	return "", fmt.Errorf("Gemini não implementado ainda")
}

// sanitizeError sanitiza mensagens de erro que podem conter dados sensíveis
func (c *Client) sanitizeError(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s", c.sanitizer.Sanitize(msg))
}

// GetProvider retorna provider pelo nome
func GetProvider(name string) (Provider, error) {
	switch name {
	case "claude":
		return ProviderClaude, nil
	case "chatgpt":
		return ProviderChatGPT, nil
	case "gemini":
		return ProviderGemini, nil
	case "ollama":
		return ProviderOllama, nil
	case "azure":
		return ProviderAzure, nil
	default:
		return "", fmt.Errorf("provider desconhecido: %s", name)
	}
}

// EstimateCost estima custo (stub)
func EstimateCost(provider Provider, tokens int) float64 {
	switch provider {
	case ProviderClaude:
		return float64(tokens) * 0.00001
	case ProviderChatGPT:
		return float64(tokens) * 0.000015
	case ProviderGemini:
		return float64(tokens) * 0.000000075 // gemini-1.5-flash: $0.075/MTok input
	case ProviderOllama:
		return 0 // Local, custo zero
	case ProviderAzure:
		return float64(tokens) * 0.000015 // Similar ao OpenAI
	default:
		return 0
	}
}
