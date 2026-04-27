package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
)

// LLMProvider interface estendida que todo provider deve implementar.
// Embeds Completer (Complete + CompleteWithUsage) e adiciona
// métodos de identidade e configuração.
type LLMProvider interface {
	Completer                      // Complete + CompleteWithUsage
	ProviderName() string          // "claude", "ollama", etc.
	ModelID() string               // Model identifier (e.g., "claude-sonnet-4-20250514")
	SetSystemPrompt(system string) // Define system prompt para chamadas subsequentes
}

// ProviderConfig configuração genérica para criar um provider
type ProviderConfig struct {
	Provider   string // "claude", "chatgpt", "gemini", "ollama", "azure"
	APIKey     string // API key (pode ser vazio para Ollama)
	Model      string // Model override (opcional)
	Endpoint   string // Endpoint customizado (para Ollama/Azure)
	Deployment string // Azure deployment name

	// CredResolver is the pluggable credential resolver.
	// When set, API keys and endpoints are resolved through the credential chain
	// instead of bare os.Getenv(). This is the preferred approach.
	// If nil, falls back to direct os.Getenv() (backward compatible).
	CredResolver *credentials.Resolver

	// SecurityConfig overrides the default security checkpoint configuration.
	// If nil, DefaultSecurityConfig() is used (Redact mode, scan credentials + PII).
	SecurityConfig *SecurityConfig
}

// NewProvider cria um LLMProvider a partir de configuração genérica.
// Factory function que roteia para o construtor apropriado.
// O provider retornado é automaticamente wrapped com retry (exponential backoff).
func NewProvider(config ProviderConfig) (LLMProvider, error) {
	// Resolve credentials via resolver if available and fields are empty
	if config.CredResolver != nil {
		resolveProviderCredentials(&config)
	}

	// Mock toggle: WTB_MOCK_LLM=1 bypasses toda a cadeia e usa fixture YAML.
	// Útil para dev feedback loop sem custo ou latência de API.
	if os.Getenv("WTB_MOCK_LLM") == "1" {
		return NewMockProviderFromConfig()
	}

	var provider LLMProvider
	var err error

	switch config.Provider {
	case "claudecli":
		// Uses the local `claude -p` CLI — no API key needed, uses Claude Code OAuth session.
		p, pErr := NewClaudeCLIProvider(config.Model)
		if pErr != nil {
			return nil, pErr
		}
		if config.Model != "" {
			// model already set via constructor
		}
		return p, nil // skip decorator chain — CLI handles retries internally

	case "claude", "chatgpt", "gemini":
		// Providers clássicos via Client
		p, pErr := GetProvider(config.Provider)
		if pErr != nil {
			return nil, pErr
		}
		var client *Client
		if config.APIKey != "" {
			// API key explicitly set (from config or resolver)
			client, err = NewClientWithKey(p, config.APIKey)
		} else {
			// Fallback: try env var directly (backward compat)
			client, err = NewClient(p)
		}
		if err != nil {
			return nil, err
		}
		// Override model se especificado
		if config.Model != "" {
			client.Model = config.Model
		}
		provider = client

	case "ollama":
		provider, err = NewOllamaProvider(config.Endpoint, config.Model)

	case "azure":
		provider, err = NewAzureProvider(config.Endpoint, config.APIKey, config.Deployment, config.Model)

	case "mock":
		// Dev feedback loop: zero API calls, YAML-driven fixture responses.
		// Bypasses all decorators (retry, security checkpoint, cache) — not needed for mock.
		return NewMockProviderFromConfig()

	default:
		return nil, fmt.Errorf("provider não suportado: %s", config.Provider)
	}

	if err != nil {
		return nil, err
	}

	// Decorator chain: EncryptedCache → SecurityCheckpoint → Retry → Provider
	// EncryptedCache is the outer layer (cache hit skips all inner layers, entries AES-256-GCM encrypted)
	// SecurityCheckpoint scans prompts for credentials/PII before API call
	// Retry wraps the provider directly (transient errors are retried)

	// 1. Retry wraps raw provider
	retried := WithRetry(provider, DefaultRetryConfig())

	// 2. SecurityCheckpoint wraps retry (scans/redacts before API call)
	securityConfig := DefaultSecurityConfig()
	if config.SecurityConfig != nil {
		securityConfig = *config.SecurityConfig
	}
	secured := WithSecurityCheckpoint(retried, securityConfig)

	// 3. EncryptedCache is outermost (can be disabled via WTB_NO_CACHE=1)
	// Uses AES-256-GCM encryption, 0600 file permissions, system prompt NOT stored in entries.
	// Master key from WTB_MASTER_KEY env var, or auto-generated session key.
	if os.Getenv("WTB_NO_CACHE") == "1" {
		return secured, nil
	}

	masterKey := os.Getenv("WTB_MASTER_KEY")
	return WithEncryptedCache(secured, DefaultCacheConfig(), masterKey), nil
}

// resolveProviderCredentials resolves API keys/endpoints via the credential resolver.
// Only fills in empty fields — explicit values take precedence.
func resolveProviderCredentials(config *ProviderConfig) {
	ctx := context.Background()
	r := config.CredResolver

	switch config.Provider {
	case "claude":
		if config.APIKey == "" {
			if cred, err := r.Resolve(ctx, "ANTHROPIC_API_KEY", nil); err == nil {
				config.APIKey = cred.Value
			}
		}
	case "chatgpt":
		if config.APIKey == "" {
			if cred, err := r.Resolve(ctx, "OPENAI_API_KEY", nil); err == nil {
				config.APIKey = cred.Value
			}
		}
	case "gemini":
		if config.APIKey == "" {
			if cred, err := r.Resolve(ctx, "GEMINI_API_KEY", nil); err == nil {
				config.APIKey = cred.Value
			}
		}
	case "ollama":
		if config.Endpoint == "" {
			if cred, err := r.Resolve(ctx, "OLLAMA_ENDPOINT", nil); err == nil {
				config.Endpoint = cred.Value
			}
		}
		if config.Model == "" {
			if cred, err := r.Resolve(ctx, "OLLAMA_MODEL", nil); err == nil {
				config.Model = cred.Value
			}
		}
	case "azure":
		if config.Endpoint == "" {
			if cred, err := r.Resolve(ctx, "AZURE_OPENAI_ENDPOINT", nil); err == nil {
				config.Endpoint = cred.Value
			}
		}
		if config.APIKey == "" {
			if cred, err := r.Resolve(ctx, "AZURE_OPENAI_API_KEY", nil); err == nil {
				config.APIKey = cred.Value
			}
		}
		if config.Deployment == "" {
			if cred, err := r.Resolve(ctx, "AZURE_OPENAI_DEPLOYMENT", nil); err == nil {
				config.Deployment = cred.Value
			}
		}
	}
}
