package llm

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGetProvider(t *testing.T) {
	tests := []struct {
		name     string
		expected Provider
		wantErr  bool
	}{
		{"claude", ProviderClaude, false},
		{"chatgpt", ProviderChatGPT, false},
		{"gemini", ProviderGemini, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := GetProvider(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if provider != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, provider)
			}
		})
	}
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		provider Provider
		tokens   int
		wantZero bool
	}{
		{ProviderClaude, 1000, false},
		{ProviderChatGPT, 1000, false},
		{ProviderGemini, 1000, false},
		{"unknown", 1000, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			cost := EstimateCost(tt.provider, tt.tokens)
			if tt.wantZero && cost != 0 {
				t.Errorf("expected 0 cost, got %f", cost)
			}
			if !tt.wantZero && cost <= 0 {
				t.Errorf("expected positive cost, got %f", cost)
			}
		})
	}
}

func TestNewClient_MissingAPIKey(t *testing.T) {
	// Ensure env vars are not set
	origClaude := os.Getenv("ANTHROPIC_API_KEY")
	origOpenAI := os.Getenv("OPENAI_API_KEY")
	origGemini := os.Getenv("GEMINI_API_KEY")

	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")

	defer func() {
		if origClaude != "" {
			os.Setenv("ANTHROPIC_API_KEY", origClaude)
		}
		if origOpenAI != "" {
			os.Setenv("OPENAI_API_KEY", origOpenAI)
		}
		if origGemini != "" {
			os.Setenv("GEMINI_API_KEY", origGemini)
		}
	}()

	providers := []Provider{ProviderClaude, ProviderChatGPT, ProviderGemini}
	for _, p := range providers {
		t.Run(string(p), func(t *testing.T) {
			_, err := NewClient(p)
			if err == nil {
				t.Error("expected error for missing API key")
			}
		})
	}
}

func TestNewClient_InvalidProvider(t *testing.T) {
	_, err := NewClient("nonexistent")
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

func TestNewClient_WithAPIKey(t *testing.T) {
	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	client, err := NewClient(ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Provider != ProviderClaude {
		t.Errorf("expected provider %q, got %q", ProviderClaude, client.Provider)
	}
	if client.APIKey != "test-key-12345" {
		t.Error("API key not set correctly")
	}
	if client.Model == "" {
		t.Error("model should be set")
	}
}

func TestNewClient_HasSanitizer(t *testing.T) {
	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	client, err := NewClient(ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.sanitizer == nil {
		t.Error("expected sanitizer to be initialized")
	}
}

func TestSanitizeError_RedactsAPIKey(t *testing.T) {
	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	client, err := NewClient(ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// API key in error body should be redacted
	apiKey := "sk-ant-abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwx"
	err = client.sanitizeError("API error 401: invalid key %s", apiKey)

	errMsg := err.Error()
	if contains(errMsg, "sk-ant-") {
		t.Errorf("expected API key to be redacted, got: %s", errMsg)
	}
	if !contains(errMsg, "ANTHROPIC-KEY-REDACTED") {
		t.Errorf("expected redaction marker, got: %s", errMsg)
	}
}

func TestSanitizeError_RedactsEmail(t *testing.T) {
	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	client, err := NewClient(ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = client.sanitizeError("error for user@example.com: unauthorized")

	errMsg := err.Error()
	if contains(errMsg, "user@example.com") {
		t.Errorf("expected email to be redacted, got: %s", errMsg)
	}
}

func TestSanitizeError_PreservesNonSensitive(t *testing.T) {
	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	client, err := NewClient(ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = client.sanitizeError("API error 429: rate limited")
	if err.Error() != "API error 429: rate limited" {
		t.Errorf("expected message preserved, got: %s", err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSetSystemPrompt(t *testing.T) {
	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	client, err := NewClient(ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.systemPrompt != "" {
		t.Error("system prompt should be empty initially")
	}

	client.SetSystemPrompt("You are a helpful assistant.")
	if client.systemPrompt != "You are a helpful assistant." {
		t.Errorf("expected system prompt to be set, got %q", client.systemPrompt)
	}

	client.SetSystemPrompt("")
	if client.systemPrompt != "" {
		t.Error("system prompt should be empty after reset")
	}
}

func TestCompletionRequest_SystemOmittedWhenEmpty(t *testing.T) {
	req := CompletionRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 100,
		Messages:  []Message{{Role: "user", Content: "Hello"}},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, "system") {
		t.Error("empty system field should be omitted from JSON")
	}
}

func TestCompletionRequest_SystemIncludedWhenSet(t *testing.T) {
	req := CompletionRequest{
		Model:     "claude-sonnet-4-20250514",
		System:    "You are a tech lead.",
		MaxTokens: 100,
		Messages:  []Message{{Role: "user", Content: "Hello"}},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	sys, ok := parsed["system"]
	if !ok {
		t.Fatal("system field should be present in JSON")
	}
	if sys != "You are a tech lead." {
		t.Errorf("expected system prompt in JSON, got %v", sys)
	}
}

func TestProviderConstants(t *testing.T) {
	if ProviderClaude != "claude" {
		t.Errorf("expected 'claude', got %q", ProviderClaude)
	}
	if ProviderChatGPT != "chatgpt" {
		t.Errorf("expected 'chatgpt', got %q", ProviderChatGPT)
	}
	if ProviderGemini != "gemini" {
		t.Errorf("expected 'gemini', got %q", ProviderGemini)
	}
}

func TestUsageStruct(t *testing.T) {
	usage := Usage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		Cost:         0.001,
	}

	if usage.InputTokens != 100 {
		t.Error("InputTokens mismatch")
	}
	if usage.OutputTokens != 50 {
		t.Error("OutputTokens mismatch")
	}
	if usage.TotalTokens != 150 {
		t.Error("TotalTokens mismatch")
	}
	if usage.Cost != 0.001 {
		t.Error("Cost mismatch")
	}
}
