package compliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

func TestNewConsentManager(t *testing.T) {
	cm := NewConsentManager("/tmp/wtb-test", false)
	if cm == nil {
		t.Fatal("expected non-nil consent manager")
	}
	if cm.demoMode {
		t.Error("expected demoMode=false")
	}
}

func TestCheckConsent_DemoMode(t *testing.T) {
	cm := NewConsentManager("/tmp/wtb-test", true)

	ok, err := cm.CheckConsent(llm.ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected consent=true in demo mode")
	}
}

func TestCheckConsent_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManager(tmpDir, false)

	ok, err := cm.CheckConsent(llm.ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected consent=false when no file exists")
	}
}

func TestSaveAndCheckConsent(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManager(tmpDir, false)

	consent := &ConsentRecord{
		Version:      "1.0",
		Provider:     "claude",
		ConsentGiven: true,
		ConsentDate:  time.Now(),
	}

	err := cm.SaveConsent(consent)
	if err != nil {
		t.Fatalf("SaveConsent failed: %v", err)
	}

	// Verify file exists
	consentPath := filepath.Join(tmpDir, "consent.json")
	if _, err := os.Stat(consentPath); os.IsNotExist(err) {
		t.Fatal("consent file not created")
	}

	// Check consent
	ok, err := cm.CheckConsent(llm.ProviderClaude)
	if err != nil {
		t.Fatalf("CheckConsent failed: %v", err)
	}
	if !ok {
		t.Error("expected consent=true after saving")
	}
}

func TestCheckConsent_DifferentProvider(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManager(tmpDir, false)

	consent := &ConsentRecord{
		Version:      "1.0",
		Provider:     "claude",
		ConsentGiven: true,
		ConsentDate:  time.Now(),
	}
	cm.SaveConsent(consent)

	// Check with different provider
	ok, err := cm.CheckConsent(llm.ProviderChatGPT)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected consent=false for different provider")
	}
}

func TestCheckConsent_NotGiven(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManager(tmpDir, false)

	consent := &ConsentRecord{
		Version:      "1.0",
		Provider:     "claude",
		ConsentGiven: false, // Not given
		ConsentDate:  time.Now(),
	}
	cm.SaveConsent(consent)

	ok, err := cm.CheckConsent(llm.ProviderClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected consent=false when not given")
	}
}

func TestConsentRecord_JSON(t *testing.T) {
	consent := ConsentRecord{
		Version:      "1.0",
		Provider:     "claude",
		ConsentGiven: true,
		ConsentDate:  time.Now(),
		DemoMode:     false,
	}

	data, err := json.Marshal(consent)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ConsentRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", decoded.Provider)
	}
}

func TestGetProviderInfo(t *testing.T) {
	tests := []struct {
		provider    llm.Provider
		expectedName string
	}{
		{llm.ProviderClaude, "Claude (Anthropic)"},
		{llm.ProviderChatGPT, "ChatGPT (OpenAI)"},
		{llm.ProviderGemini, "Gemini (Google)"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			info := getProviderInfo(tt.provider)
			if info.Name != tt.expectedName {
				t.Errorf("expected %q, got %q", tt.expectedName, info.Name)
			}
		})
	}
}

func TestCreateDemoConsent(t *testing.T) {
	tmpDir := t.TempDir()

	err := CreateDemoConsent(tmpDir, llm.ProviderClaude)
	if err != nil {
		t.Fatalf("CreateDemoConsent failed: %v", err)
	}

	// Verify file exists and is valid
	consentPath := filepath.Join(tmpDir, "consent.json")
	data, err := os.ReadFile(consentPath)
	if err != nil {
		t.Fatalf("failed to read consent: %v", err)
	}

	var consent ConsentRecord
	if err := json.Unmarshal(data, &consent); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !consent.DemoMode {
		t.Error("expected demoMode=true")
	}
	if !consent.ConsentGiven {
		t.Error("expected consentGiven=true")
	}
}
