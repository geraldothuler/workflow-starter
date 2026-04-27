package llm

import (
	"fmt"
	"strings"
	"testing"
)

// mockLLMProvider is a test double that records calls and returns canned responses
type mockLLMProviderForSecurity struct {
	lastPrompt      string
	lastSystemPrompt string
	lastMaxTokens   int
	callCount       int
	response        string
	usage           *Usage
	err             error
}

func (m *mockLLMProviderForSecurity) Complete(prompt string, maxTokens int) (string, error) {
	resp, _, err := m.CompleteWithUsage(prompt, maxTokens)
	return resp, err
}

func (m *mockLLMProviderForSecurity) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	m.lastPrompt = prompt
	m.lastMaxTokens = maxTokens
	m.callCount++
	return m.response, m.usage, m.err
}

func (m *mockLLMProviderForSecurity) ProviderName() string { return "mock" }
func (m *mockLLMProviderForSecurity) ModelID() string      { return "mock-v1" }
func (m *mockLLMProviderForSecurity) SetSystemPrompt(system string) {
	m.lastSystemPrompt = system
}

func TestSecurityCheckpoint_BlockMode_BlocksCredentials(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeBlock,
		ScanCredentials: true,
		ScanPII:         false,
	})
	sc.logFunc = nil // suppress log output in tests

	prompt := "Please use this API key: sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234"
	_, _, err := sc.CompleteWithUsage(prompt, 1000)

	if err == nil {
		t.Fatal("expected error in block mode but got nil")
	}
	if !strings.Contains(err.Error(), "security checkpoint") {
		t.Errorf("error should mention security checkpoint: %v", err)
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error should mention blocked: %v", err)
	}
	if mock.callCount != 0 {
		t.Errorf("inner provider should NOT be called in block mode, got %d calls", mock.callCount)
	}
}

func TestSecurityCheckpoint_BlockMode_BlocksPII(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeBlock,
		ScanCredentials: false,
		ScanPII:         true,
	})
	sc.logFunc = nil

	// CPF is validated by the PII detector
	prompt := "O CPF do cliente é 529.982.247-25" // valid CPF
	_, _, err := sc.CompleteWithUsage(prompt, 1000)

	if err == nil {
		t.Fatal("expected error in block mode with PII but got nil")
	}
	if mock.callCount != 0 {
		t.Errorf("inner provider should NOT be called when PII is blocked, got %d calls", mock.callCount)
	}
}

func TestSecurityCheckpoint_RedactMode_RedactsAndContinues(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "Generated backlog", usage: &Usage{InputTokens: 100}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeRedact,
		ScanCredentials: true,
		ScanPII:         false,
	})
	sc.logFunc = nil

	prompt := "Use ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij to access the repo"
	resp, usage, err := sc.CompleteWithUsage(prompt, 1000)

	if err != nil {
		t.Fatalf("unexpected error in redact mode: %v", err)
	}
	if resp != "Generated backlog" {
		t.Errorf("unexpected response: %s", resp)
	}
	if usage.InputTokens != 100 {
		t.Errorf("unexpected usage: %+v", usage)
	}
	if mock.callCount != 1 {
		t.Errorf("inner provider should be called once, got %d", mock.callCount)
	}
	// The prompt sent to inner provider should be redacted
	if strings.Contains(mock.lastPrompt, "ghp_ABCDEF") {
		t.Errorf("inner provider received unredacted prompt: %s", mock.lastPrompt)
	}
	if !strings.Contains(mock.lastPrompt, "[GITHUB-TOKEN-REDACTED]") {
		t.Errorf("inner provider should receive redacted prompt with tag, got: %s", mock.lastPrompt)
	}
}

func TestSecurityCheckpoint_WarnMode_PassesThrough(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	var logMessages []string
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeWarn,
		ScanCredentials: true,
		ScanPII:         false,
		AuditEvents:     true,
	})
	sc.logFunc = func(format string, args ...interface{}) {
		logMessages = append(logMessages, fmt.Sprintf(format, args...))
	}

	originalKey := "sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234"
	prompt := "Use this key: " + originalKey
	resp, _, err := sc.CompleteWithUsage(prompt, 1000)

	if err != nil {
		t.Fatalf("unexpected error in warn mode: %v", err)
	}
	if resp != "OK" {
		t.Errorf("unexpected response: %s", resp)
	}
	if mock.callCount != 1 {
		t.Errorf("inner provider should be called once, got %d", mock.callCount)
	}
	// In warn mode, the original prompt should be passed through
	if !strings.Contains(mock.lastPrompt, "sk-ant-api03") {
		t.Errorf("warn mode should pass original prompt, got: %s", mock.lastPrompt)
	}
	// But a warning should have been logged
	if len(logMessages) == 0 {
		t.Error("expected log messages in warn mode with audit enabled")
	}
}

func TestSecurityCheckpoint_CleanPrompt_PassesThrough(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "Backlog generated", usage: &Usage{InputTokens: 50}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeBlock,
		ScanCredentials: true,
		ScanPII:         true,
		AuditEvents:     true,
	})
	sc.logFunc = nil

	prompt := "Generate a backlog for a Spring Boot microservice with PostgreSQL and Redis caching"
	resp, _, err := sc.CompleteWithUsage(prompt, 2000)

	if err != nil {
		t.Fatalf("clean prompt should not be blocked: %v", err)
	}
	if resp != "Backlog generated" {
		t.Errorf("unexpected response: %s", resp)
	}
	if mock.callCount != 1 {
		t.Errorf("inner provider should be called once, got %d", mock.callCount)
	}
	// Prompt should be passed through unchanged
	if mock.lastPrompt != prompt {
		t.Errorf("clean prompt should be passed through unchanged")
	}
}

func TestSecurityCheckpoint_SystemPrompt_ScansAndRedacts(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:             SecurityModeRedact,
		ScanCredentials:  true,
		ScanSystemPrompt: true,
	})
	sc.logFunc = nil

	systemPrompt := "You are an assistant. API key for testing: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
	sc.SetSystemPrompt(systemPrompt)

	// The system prompt sent to inner provider should be redacted
	if strings.Contains(mock.lastSystemPrompt, "ghp_ABCDEF") {
		t.Errorf("inner provider received unredacted system prompt: %s", mock.lastSystemPrompt)
	}
}

func TestSecurityCheckpoint_SystemPrompt_SkipsScanWhenDisabled(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:             SecurityModeBlock,
		ScanCredentials:  true,
		ScanSystemPrompt: false, // Disabled
	})
	sc.logFunc = nil

	key := "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
	systemPrompt := "System with key: " + key
	sc.SetSystemPrompt(systemPrompt)

	// System prompt should pass through unchanged when scan is disabled
	if !strings.Contains(mock.lastSystemPrompt, key) {
		t.Errorf("system prompt should pass through when scanning disabled")
	}
}

func TestSecurityCheckpoint_InnerProviderError_Propagates(t *testing.T) {
	expectedErr := fmt.Errorf("API rate limit exceeded")
	mock := &mockLLMProviderForSecurity{err: expectedErr}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeRedact,
		ScanCredentials: true,
	})
	sc.logFunc = nil

	_, _, err := sc.CompleteWithUsage("clean prompt", 1000)

	if err == nil {
		t.Fatal("expected inner provider error to propagate")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestSecurityCheckpoint_Complete_DelegatesToCompleteWithUsage(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeBlock,
		ScanCredentials: true,
	})
	sc.logFunc = nil

	resp, err := sc.Complete("clean prompt", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "OK" {
		t.Errorf("unexpected response: %s", resp)
	}
}

func TestSecurityCheckpoint_ProviderName_Delegates(t *testing.T) {
	mock := &mockLLMProviderForSecurity{}
	sc := WithSecurityCheckpoint(mock, DefaultSecurityConfig())

	if sc.ProviderName() != "mock" {
		t.Errorf("ProviderName() = %s, want mock", sc.ProviderName())
	}
	if sc.ModelID() != "mock-v1" {
		t.Errorf("ModelID() = %s, want mock-v1", sc.ModelID())
	}
}

func TestSecurityCheckpoint_MultipleCredentials_AllDetected(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeBlock,
		ScanCredentials: true,
	})
	sc.logFunc = nil

	prompt := `
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij
`
	_, _, err := sc.CompleteWithUsage(prompt, 1000)

	if err == nil {
		t.Fatal("expected error with multiple credentials")
	}

	errMsg := err.Error()
	// Should mention credential count
	if !strings.Contains(errMsg, "credential") {
		t.Errorf("error should mention credentials: %v", err)
	}
}

func TestSecurityCheckpoint_RedactMode_CredentialsAndPII(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeRedact,
		ScanCredentials: true,
		ScanPII:         true,
	})
	sc.logFunc = nil

	prompt := "API key: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij, email: user@example.com"
	_, _, err := sc.CompleteWithUsage(prompt, 1000)

	if err != nil {
		t.Fatalf("redact mode should not return error: %v", err)
	}

	// Both should be redacted
	if strings.Contains(mock.lastPrompt, "ghp_ABCDEF") {
		t.Errorf("GitHub token should be redacted in prompt sent to provider")
	}
	if strings.Contains(mock.lastPrompt, "user@example.com") {
		t.Errorf("email should be redacted in prompt sent to provider")
	}
}

func TestSecurityCheckpoint_DisabledScanning_PassesThrough(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeBlock,
		ScanCredentials: false,
		ScanPII:         false,
	})
	sc.logFunc = nil

	// Even with credentials, should pass through since scanning is disabled
	prompt := "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
	_, _, err := sc.CompleteWithUsage(prompt, 1000)

	if err != nil {
		t.Fatalf("should pass through with scanning disabled: %v", err)
	}
	if mock.callCount != 1 {
		t.Errorf("should call inner provider, got %d calls", mock.callCount)
	}
}

func TestSecurityCheckpoint_BlockError_ContainsCategories(t *testing.T) {
	mock := &mockLLMProviderForSecurity{response: "OK", usage: &Usage{}}
	sc := WithSecurityCheckpoint(mock, SecurityConfig{
		Mode:            SecurityModeBlock,
		ScanCredentials: true,
	})
	sc.logFunc = nil

	prompt := "AKIAIOSFODNN7EXAMPLE"
	_, _, err := sc.CompleteWithUsage(prompt, 1000)

	if err == nil {
		t.Fatal("expected block error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "aws_access_key") {
		t.Errorf("block error should contain category name: %s", errMsg)
	}
}

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	if config.Mode != SecurityModeRedact {
		t.Errorf("default mode should be Redact, got %d", config.Mode)
	}
	if !config.ScanCredentials {
		t.Error("ScanCredentials should be true by default")
	}
	if !config.ScanPII {
		t.Error("ScanPII should be true by default")
	}
	if !config.ScanSystemPrompt {
		t.Error("ScanSystemPrompt should be true by default")
	}
	if !config.AuditEvents {
		t.Error("AuditEvents should be true by default")
	}
}
