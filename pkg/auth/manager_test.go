package auth

import (
	"os"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

func TestNewKeyManager(t *testing.T) {
	km := NewKeyManager("/tmp/wtb-test")
	if km == nil {
		t.Fatal("expected non-nil key manager")
	}
}

func TestGetAPIKey_FromEnv(t *testing.T) {
	km := NewKeyManager("/tmp/wtb-test")

	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	key, err := km.GetAPIKey(llm.ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-ant-test-key-12345" {
		t.Errorf("expected key from env, got %q", key)
	}
}

func TestGetAPIKey_Missing(t *testing.T) {
	km := NewKeyManager("/tmp/wtb-test")

	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		}
	}()

	_, err := km.GetAPIKey(llm.ProviderClaude)
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestGetEnvVarName(t *testing.T) {
	km := NewKeyManager("")

	tests := []struct {
		provider llm.Provider
		expected string
	}{
		{llm.ProviderClaude, "ANTHROPIC_API_KEY"},
		{llm.ProviderChatGPT, "OPENAI_API_KEY"},
		{llm.ProviderGemini, "GEMINI_API_KEY"},
		{"unknown", "API_KEY"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := km.getEnvVarName(tt.provider)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetProviderName(t *testing.T) {
	km := NewKeyManager("")

	tests := []struct {
		provider llm.Provider
		expected string
	}{
		{llm.ProviderClaude, "Claude (Anthropic)"},
		{llm.ProviderChatGPT, "ChatGPT (OpenAI)"},
		{llm.ProviderGemini, "Gemini (Google)"},
		{"other", "other"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := km.getProviderName(tt.provider)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestValidateKeyFormat(t *testing.T) {
	km := NewKeyManager("")

	tests := []struct {
		name     string
		provider llm.Provider
		key      string
		valid    bool
	}{
		{"claude valid", llm.ProviderClaude, "sk-ant-1234567890", true},
		{"claude invalid", llm.ProviderClaude, "invalid-key", false},
		{"openai valid", llm.ProviderChatGPT, "sk-1234567890", true},
		{"openai invalid", llm.ProviderChatGPT, "invalid", false},
		{"gemini valid", llm.ProviderGemini, "abcdefghijklmnopqrstuvwxyz12345", true},
		{"gemini too short", llm.ProviderGemini, "short", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := km.validateKeyFormat(tt.provider, tt.key)
			if result != tt.valid {
				t.Errorf("expected %v, got %v", tt.valid, result)
			}
		})
	}
}

func TestCheckKeyFormat(t *testing.T) {
	km := NewKeyManager("")

	err := km.CheckKeyFormat(llm.ProviderClaude, "sk-ant-valid-key-here")
	if err != nil {
		t.Errorf("expected valid key format: %v", err)
	}

	err = km.CheckKeyFormat(llm.ProviderClaude, "invalid")
	if err == nil {
		t.Error("expected error for invalid key format")
	}
}
