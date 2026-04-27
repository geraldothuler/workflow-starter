package llm

import (
	"os"
	"testing"
)

// === Interface Compliance Tests ===

func TestClient_ImplementsLLMProvider(t *testing.T) {
	// Compile-time check: Client implements LLMProvider
	var _ LLMProvider = &Client{}
}

func TestMockClient_ImplementsLLMProvider(t *testing.T) {
	// Compile-time check: MockClient implements LLMProvider
	var _ LLMProvider = &MockClient{}
}

func TestOllamaProvider_ImplementsLLMProvider(t *testing.T) {
	// Compile-time check: OllamaProvider implements LLMProvider
	var _ LLMProvider = &OllamaProvider{}
}

func TestAzureProvider_ImplementsLLMProvider(t *testing.T) {
	// Compile-time check: AzureProvider implements LLMProvider
	var _ AzureProvider = AzureProvider{}
	var _ LLMProvider = &AzureProvider{}
}

// === Provider Constants Tests ===

func TestProviderConstants_OllamaAndAzure(t *testing.T) {
	if ProviderOllama != "ollama" {
		t.Errorf("expected 'ollama', got %q", ProviderOllama)
	}
	if ProviderAzure != "azure" {
		t.Errorf("expected 'azure', got %q", ProviderAzure)
	}
}

func TestGetProvider_OllamaAndAzure(t *testing.T) {
	tests := []struct {
		name     string
		expected Provider
		wantErr  bool
	}{
		{"ollama", ProviderOllama, false},
		{"azure", ProviderAzure, false},
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

// === Client LLMProvider Methods ===

func TestClient_ProviderName(t *testing.T) {
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

	if client.ProviderName() != "claude" {
		t.Errorf("expected 'claude', got %q", client.ProviderName())
	}
}

func TestClient_ModelID(t *testing.T) {
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

	if client.ModelID() == "" {
		t.Error("expected non-empty ModelID")
	}
	if client.ModelID() != client.Model {
		t.Errorf("ModelID() should return Model field, got %q vs %q", client.ModelID(), client.Model)
	}
}

// === MockClient LLMProvider Methods ===

func TestMockClient_ProviderName_Default(t *testing.T) {
	mock := NewMockClient("resp")
	if mock.ProviderName() != "mock" {
		t.Errorf("expected 'mock', got %q", mock.ProviderName())
	}
}

func TestMockClient_ModelID_Default(t *testing.T) {
	mock := NewMockClient("resp")
	if mock.ModelID() != "mock-model" {
		t.Errorf("expected 'mock-model', got %q", mock.ModelID())
	}
}

func TestMockClient_SetSystemPrompt(t *testing.T) {
	mock := NewMockClient("resp")

	mock.SetSystemPrompt("You are a tech lead.")
	if mock.GetSystemPrompt() != "You are a tech lead." {
		t.Errorf("expected system prompt set, got %q", mock.GetSystemPrompt())
	}

	mock.SetSystemPrompt("")
	if mock.GetSystemPrompt() != "" {
		t.Error("expected empty system prompt after reset")
	}
}

func TestNewMockProvider(t *testing.T) {
	mock := NewMockProvider("ollama", "llama3", "response1")

	if mock.ProviderName() != "ollama" {
		t.Errorf("expected 'ollama', got %q", mock.ProviderName())
	}
	if mock.ModelID() != "llama3" {
		t.Errorf("expected 'llama3', got %q", mock.ModelID())
	}

	resp, err := mock.Complete("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response1" {
		t.Errorf("expected 'response1', got %q", resp)
	}
}

// === OllamaProvider Unit Tests ===

func TestOllamaProvider_Defaults(t *testing.T) {
	// Limpar env vars
	origEndpoint := os.Getenv("OLLAMA_ENDPOINT")
	origModel := os.Getenv("OLLAMA_MODEL")
	os.Unsetenv("OLLAMA_ENDPOINT")
	os.Unsetenv("OLLAMA_MODEL")
	defer func() {
		if origEndpoint != "" {
			os.Setenv("OLLAMA_ENDPOINT", origEndpoint)
		}
		if origModel != "" {
			os.Setenv("OLLAMA_MODEL", origModel)
		}
	}()

	provider, err := NewOllamaProvider("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.ProviderName() != "ollama" {
		t.Errorf("expected 'ollama', got %q", provider.ProviderName())
	}
	if provider.ModelID() != defaultOllamaModel {
		t.Errorf("expected %q, got %q", defaultOllamaModel, provider.ModelID())
	}
	if provider.endpoint != defaultOllamaEndpoint {
		t.Errorf("expected %q, got %q", defaultOllamaEndpoint, provider.endpoint)
	}
}

func TestOllamaProvider_EnvVars(t *testing.T) {
	origEndpoint := os.Getenv("OLLAMA_ENDPOINT")
	origModel := os.Getenv("OLLAMA_MODEL")
	os.Setenv("OLLAMA_ENDPOINT", "http://gpu-server:11434")
	os.Setenv("OLLAMA_MODEL", "codellama")
	defer func() {
		if origEndpoint != "" {
			os.Setenv("OLLAMA_ENDPOINT", origEndpoint)
		} else {
			os.Unsetenv("OLLAMA_ENDPOINT")
		}
		if origModel != "" {
			os.Setenv("OLLAMA_MODEL", origModel)
		} else {
			os.Unsetenv("OLLAMA_MODEL")
		}
	}()

	provider, err := NewOllamaProvider("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.endpoint != "http://gpu-server:11434" {
		t.Errorf("expected env endpoint, got %q", provider.endpoint)
	}
	if provider.ModelID() != "codellama" {
		t.Errorf("expected 'codellama', got %q", provider.ModelID())
	}
}

func TestOllamaProvider_ExplicitParams(t *testing.T) {
	provider, err := NewOllamaProvider("http://custom:8080", "mistral")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.endpoint != "http://custom:8080" {
		t.Errorf("expected explicit endpoint, got %q", provider.endpoint)
	}
	if provider.ModelID() != "mistral" {
		t.Errorf("expected 'mistral', got %q", provider.ModelID())
	}
}

func TestOllamaProvider_SetSystemPrompt(t *testing.T) {
	provider, _ := NewOllamaProvider("", "")
	provider.SetSystemPrompt("You are a Go expert")
	if provider.systemPrompt != "You are a Go expert" {
		t.Errorf("expected system prompt, got %q", provider.systemPrompt)
	}
}

// === AzureProvider Unit Tests ===

func TestAzureProvider_MissingEndpoint(t *testing.T) {
	origEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	origKey := os.Getenv("AZURE_OPENAI_API_KEY")
	origDeploy := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	os.Unsetenv("AZURE_OPENAI_ENDPOINT")
	os.Unsetenv("AZURE_OPENAI_API_KEY")
	os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
	defer func() {
		if origEndpoint != "" {
			os.Setenv("AZURE_OPENAI_ENDPOINT", origEndpoint)
		}
		if origKey != "" {
			os.Setenv("AZURE_OPENAI_API_KEY", origKey)
		}
		if origDeploy != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeploy)
		}
	}()

	_, err := NewAzureProvider("", "", "", "")
	if err == nil {
		t.Error("expected error for missing endpoint")
	}
}

func TestAzureProvider_MissingAPIKey(t *testing.T) {
	origKey := os.Getenv("AZURE_OPENAI_API_KEY")
	origDeploy := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	os.Unsetenv("AZURE_OPENAI_API_KEY")
	os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
	defer func() {
		if origKey != "" {
			os.Setenv("AZURE_OPENAI_API_KEY", origKey)
		}
		if origDeploy != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeploy)
		}
	}()

	_, err := NewAzureProvider("https://my-resource.openai.azure.com", "", "", "")
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestAzureProvider_MissingDeployment(t *testing.T) {
	origDeploy := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
	defer func() {
		if origDeploy != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeploy)
		}
	}()

	_, err := NewAzureProvider("https://my-resource.openai.azure.com", "test-key-123456789012345", "", "")
	if err == nil {
		t.Error("expected error for missing deployment")
	}
}

func TestAzureProvider_ValidConfig(t *testing.T) {
	provider, err := NewAzureProvider(
		"https://my-resource.openai.azure.com",
		"test-key-123456789012345",
		"gpt4-deployment",
		"gpt-4",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.ProviderName() != "azure" {
		t.Errorf("expected 'azure', got %q", provider.ProviderName())
	}
	if provider.ModelID() != "gpt-4" {
		t.Errorf("expected 'gpt-4', got %q", provider.ModelID())
	}
}

func TestAzureProvider_EnvVars(t *testing.T) {
	origEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	origKey := os.Getenv("AZURE_OPENAI_API_KEY")
	origDeploy := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	os.Setenv("AZURE_OPENAI_ENDPOINT", "https://env-resource.openai.azure.com")
	os.Setenv("AZURE_OPENAI_API_KEY", "env-key-1234567890123456")
	os.Setenv("AZURE_OPENAI_DEPLOYMENT", "env-deploy")
	defer func() {
		if origEndpoint != "" {
			os.Setenv("AZURE_OPENAI_ENDPOINT", origEndpoint)
		} else {
			os.Unsetenv("AZURE_OPENAI_ENDPOINT")
		}
		if origKey != "" {
			os.Setenv("AZURE_OPENAI_API_KEY", origKey)
		} else {
			os.Unsetenv("AZURE_OPENAI_API_KEY")
		}
		if origDeploy != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeploy)
		} else {
			os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
		}
	}()

	provider, err := NewAzureProvider("", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.endpoint != "https://env-resource.openai.azure.com" {
		t.Errorf("expected env endpoint, got %q", provider.endpoint)
	}
}

func TestAzureProvider_SetSystemPrompt(t *testing.T) {
	provider, _ := NewAzureProvider(
		"https://test.openai.azure.com",
		"test-key-123456789012345",
		"deploy",
		"",
	)
	provider.SetSystemPrompt("You are a tech lead")
	if provider.systemPrompt != "You are a tech lead" {
		t.Errorf("expected system prompt, got %q", provider.systemPrompt)
	}
}

// === NewProvider Factory Tests ===

func TestNewProvider_ClassicProviders(t *testing.T) {
	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	provider, err := NewProvider(ProviderConfig{Provider: "claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.ProviderName() != "claude" {
		t.Errorf("expected 'claude', got %q", provider.ProviderName())
	}
}

func TestNewProvider_Ollama(t *testing.T) {
	provider, err := NewProvider(ProviderConfig{
		Provider: "ollama",
		Endpoint: "http://localhost:11434",
		Model:    "llama3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.ProviderName() != "ollama" {
		t.Errorf("expected 'ollama', got %q", provider.ProviderName())
	}
	if provider.ModelID() != "llama3" {
		t.Errorf("expected 'llama3', got %q", provider.ModelID())
	}
}

func TestNewProvider_Azure(t *testing.T) {
	provider, err := NewProvider(ProviderConfig{
		Provider:   "azure",
		Endpoint:   "https://test.openai.azure.com",
		APIKey:     "test-key-123456789012345",
		Deployment: "gpt4-deploy",
		Model:      "gpt-4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.ProviderName() != "azure" {
		t.Errorf("expected 'azure', got %q", provider.ProviderName())
	}
}

func TestNewProvider_InvalidProvider(t *testing.T) {
	_, err := NewProvider(ProviderConfig{Provider: "nonexistent"})
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

func TestNewProvider_ClassicWithModelOverride(t *testing.T) {
	origKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer func() {
		if origKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", origKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	provider, err := NewProvider(ProviderConfig{
		Provider: "claude",
		Model:    "claude-3-haiku-20240307",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.ModelID() != "claude-3-haiku-20240307" {
		t.Errorf("expected model override, got %q", provider.ModelID())
	}
}

// === EstimateCost Tests ===

func TestEstimateCost_NewProviders(t *testing.T) {
	tests := []struct {
		provider Provider
		tokens   int
		wantZero bool
	}{
		{ProviderOllama, 1000, true},
		{ProviderAzure, 1000, false},
		{ProviderGemini, 1000, false},
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
