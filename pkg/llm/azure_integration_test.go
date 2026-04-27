//go:build integration

package llm

import (
	"os"
	"testing"
)

func TestIntegration_Azure_SmokeTest(t *testing.T) {
	// Skip se env vars não estiverem configuradas
	if os.Getenv("AZURE_OPENAI_ENDPOINT") == "" {
		t.Skip("AZURE_OPENAI_ENDPOINT não configurado — skipping integration test")
	}
	if os.Getenv("AZURE_OPENAI_API_KEY") == "" {
		t.Skip("AZURE_OPENAI_API_KEY não configurado — skipping")
	}
	if os.Getenv("AZURE_OPENAI_DEPLOYMENT") == "" {
		t.Skip("AZURE_OPENAI_DEPLOYMENT não configurado — skipping")
	}

	provider, err := NewAzureProvider("", "", "", "")
	if err != nil {
		t.Fatalf("erro ao criar Azure provider: %v", err)
	}

	// Verificar identidade
	if provider.ProviderName() != "azure" {
		t.Errorf("expected 'azure', got %q", provider.ProviderName())
	}

	// Smoke test
	response, usage, err := provider.CompleteWithUsage("Say 'hello' in one word.", 50)
	if err != nil {
		t.Fatalf("Azure completion failed: %v", err)
	}

	if response == "" {
		t.Error("expected non-empty response")
	}
	if usage == nil {
		t.Error("expected non-nil usage")
	}
	if usage.TotalTokens == 0 {
		t.Error("expected positive token count")
	}

	t.Logf("Azure response: %q", response)
	t.Logf("Azure usage: input=%d output=%d cost=$%.6f", usage.InputTokens, usage.OutputTokens, usage.Cost)
}

func TestIntegration_Azure_SystemPrompt(t *testing.T) {
	if os.Getenv("AZURE_OPENAI_ENDPOINT") == "" {
		t.Skip("AZURE_OPENAI_ENDPOINT não configurado — skipping")
	}
	if os.Getenv("AZURE_OPENAI_API_KEY") == "" {
		t.Skip("AZURE_OPENAI_API_KEY não configurado — skipping")
	}
	if os.Getenv("AZURE_OPENAI_DEPLOYMENT") == "" {
		t.Skip("AZURE_OPENAI_DEPLOYMENT não configurado — skipping")
	}

	provider, err := NewAzureProvider("", "", "", "")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	provider.SetSystemPrompt("You always respond in Portuguese.")
	response, _, err := provider.CompleteWithUsage("What color is the sky?", 100)
	if err != nil {
		t.Fatalf("completion failed: %v", err)
	}

	if response == "" {
		t.Error("expected non-empty response")
	}

	t.Logf("Azure response with system prompt: %q", response)
}
