//go:build integration

package llm

import (
	"net/http"
	"testing"
	"time"
)

func TestIntegration_Ollama_SmokeTest(t *testing.T) {
	// Skip se Ollama não estiver rodando
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		t.Skip("Ollama não está rodando em localhost:11434 — skipping integration test")
	}
	resp.Body.Close()

	provider, err := NewOllamaProvider("", "")
	if err != nil {
		t.Fatalf("erro ao criar Ollama provider: %v", err)
	}

	// Verificar identidade
	if provider.ProviderName() != "ollama" {
		t.Errorf("expected 'ollama', got %q", provider.ProviderName())
	}

	// Smoke test: prompt simples
	response, usage, err := provider.CompleteWithUsage("Say 'hello' in one word.", 50)
	if err != nil {
		t.Fatalf("Ollama completion failed: %v", err)
	}

	if response == "" {
		t.Error("expected non-empty response from Ollama")
	}
	if usage == nil {
		t.Error("expected non-nil usage")
	}
	if usage.Cost != 0 {
		t.Errorf("Ollama should have zero cost, got %f", usage.Cost)
	}

	t.Logf("Ollama response: %q", response)
	t.Logf("Ollama usage: input=%d output=%d", usage.InputTokens, usage.OutputTokens)
}

func TestIntegration_Ollama_SystemPrompt(t *testing.T) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		t.Skip("Ollama não está rodando — skipping")
	}
	resp.Body.Close()

	provider, err := NewOllamaProvider("", "")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	provider.SetSystemPrompt("You always respond with exactly one word.")
	response, _, err := provider.CompleteWithUsage("What color is the sky?", 50)
	if err != nil {
		t.Fatalf("completion failed: %v", err)
	}

	if response == "" {
		t.Error("expected non-empty response")
	}

	t.Logf("Ollama response with system prompt: %q", response)
}
