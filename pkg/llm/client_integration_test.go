//go:build integration

package llm

import (
	"os"
	"testing"
)

// skipIfNoKey skips the test if the API key for the given provider is not set.
func skipIfNoKey(t *testing.T, provider Provider) {
	t.Helper()
	envVars := map[Provider]string{
		ProviderClaude:  "ANTHROPIC_API_KEY",
		ProviderChatGPT: "OPENAI_API_KEY",
		ProviderGemini:  "GEMINI_API_KEY",
	}
	key := envVars[provider]
	if os.Getenv(key) == "" {
		t.Skipf("skipping: %s not set", key)
	}
}

// getTestProvider returns the provider to use based on WTB_TEST_PROVIDER env var
// or auto-selects the cheapest available.
func getTestProvider(t *testing.T) Provider {
	t.Helper()
	if p := os.Getenv("WTB_TEST_PROVIDER"); p != "" {
		provider := Provider(p)
		skipIfNoKey(t, provider)
		return provider
	}
	// Prefer Gemini (cheapest), then Claude, then ChatGPT
	for _, p := range []Provider{ProviderGemini, ProviderClaude, ProviderChatGPT} {
		envVars := map[Provider]string{
			ProviderClaude:  "ANTHROPIC_API_KEY",
			ProviderChatGPT: "OPENAI_API_KEY",
			ProviderGemini:  "GEMINI_API_KEY",
		}
		if os.Getenv(envVars[p]) != "" {
			return p
		}
	}
	t.Skip("no LLM provider configured")
	return ""
}

func TestIntegration_Claude_Smoke(t *testing.T) {
	skipIfNoKey(t, ProviderClaude)

	client, err := NewClient(ProviderClaude)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	response, err := client.Complete("Respond with exactly one word: hello", 50)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if len(response) == 0 {
		t.Error("expected non-empty response")
	}
	t.Logf("Claude response: %s", response)
}

func TestIntegration_ChatGPT_Smoke(t *testing.T) {
	skipIfNoKey(t, ProviderChatGPT)

	client, err := NewClient(ProviderChatGPT)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	response, err := client.Complete("Respond with exactly one word: hello", 50)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if len(response) == 0 {
		t.Error("expected non-empty response")
	}
	t.Logf("ChatGPT response: %s", response)
}

func TestIntegration_Gemini_Smoke(t *testing.T) {
	skipIfNoKey(t, ProviderGemini)

	client, err := NewClient(ProviderGemini)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	response, err := client.Complete("Respond with exactly one word: hello", 50)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if len(response) == 0 {
		t.Error("expected non-empty response")
	}
	t.Logf("Gemini response: %s", response)
}

func TestIntegration_SystemPrompt(t *testing.T) {
	provider := getTestProvider(t)

	client, err := NewClient(provider)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	client.SetSystemPrompt("You are a pirate. Always respond in pirate speak.")
	response, err := client.Complete("Say hello", 100)
	if err != nil {
		t.Fatalf("Complete with system prompt failed: %v", err)
	}
	if len(response) == 0 {
		t.Error("expected non-empty response")
	}
	t.Logf("[%s] System prompt response: %s", provider, response)
}

func TestIntegration_UsageTracking(t *testing.T) {
	provider := getTestProvider(t)

	client, err := NewClient(provider)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	response, usage, err := client.CompleteWithUsage("Respond with exactly: OK", 50)
	if err != nil {
		t.Fatalf("CompleteWithUsage failed: %v", err)
	}
	if len(response) == 0 {
		t.Error("expected non-empty response")
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	if usage.InputTokens <= 0 {
		t.Errorf("expected InputTokens > 0, got %d", usage.InputTokens)
	}
	if usage.OutputTokens <= 0 {
		t.Errorf("expected OutputTokens > 0, got %d", usage.OutputTokens)
	}
	if usage.Cost <= 0 {
		t.Errorf("expected Cost > 0, got %f", usage.Cost)
	}
	t.Logf("[%s] Usage: input=%d, output=%d, cost=$%.6f",
		provider, usage.InputTokens, usage.OutputTokens, usage.Cost)
}

func TestIntegration_JSONResponse(t *testing.T) {
	provider := getTestProvider(t)

	client, err := NewClient(provider)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	prompt := `Return a valid JSON object with exactly these fields:
{"name": "test", "count": 1}
Return ONLY the JSON, no other text.`

	response, err := client.Complete(prompt, 100)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if len(response) == 0 {
		t.Error("expected non-empty response")
	}
	t.Logf("[%s] JSON response: %s", provider, response)
}
