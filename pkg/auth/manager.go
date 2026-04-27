package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

// KeyManager gerencia API keys de forma segura.
// Supports pluggable credential resolution via credentials.Resolver.
type KeyManager struct {
	configDir    string
	credResolver *credentials.Resolver // optional; nil = legacy os.Getenv fallback
}

// NewKeyManager cria novo manager
func NewKeyManager(configDir string) *KeyManager {
	return &KeyManager{
		configDir: configDir,
	}
}

// NewKeyManagerWithResolver creates a KeyManager that delegates to the credential resolver.
// This is the preferred constructor — credentials are resolved through the pluggable chain
// (env → keyring → encrypted-file → pass → 1password → aws-ssm).
func NewKeyManagerWithResolver(configDir string, resolver *credentials.Resolver) *KeyManager {
	return &KeyManager{
		configDir:    configDir,
		credResolver: resolver,
	}
}

// GetAPIKey obtém API key para provider.
// When a credential resolver is configured, uses the pluggable chain.
// Falls back to os.Getenv() for backward compatibility.
func (km *KeyManager) GetAPIKey(provider llm.Provider) (string, error) {
	envVar := km.getEnvVarName(provider)

	// Prefer resolver if available
	if km.credResolver != nil {
		cred, err := km.credResolver.Resolve(context.Background(), envVar, nil)
		if err == nil {
			return cred.Value, nil
		}
		// Fall through to helpful error message
	} else {
		// Legacy: direct env var lookup
		if key := os.Getenv(envVar); key != "" {
			return key, nil
		}
	}

	// Se não encontrou, sugerir configuração
	return "", fmt.Errorf("API key não configurada para %s\n\n"+
		"Configure de forma segura:\n"+
		"  wtb credentials store %s --provider encrypted-file\n\n"+
		"Ou use variável de ambiente (menos seguro):\n"+
		"  export %s='sua-key-aqui'",
		provider, envVar, envVar)
}

// SetupInteractive setup interativo de API key
func (km *KeyManager) SetupInteractive(provider llm.Provider) error {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔐 CONFIGURAÇÃO SEGURA DE API KEY")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	
	providerName := km.getProviderName(provider)
	fmt.Printf("Provider: %s\n", providerName)
	fmt.Println()
	
	fmt.Println("⚠️  IMPORTANTE - SEGURANÇA DA API KEY:")
	fmt.Println()
	fmt.Println("❌ NUNCA:")
	fmt.Println("  • Commite a key no Git")
	fmt.Println("  • Compartilhe em chat/email")
	fmt.Println("  • Exponha em logs públicos")
	fmt.Println()
	fmt.Println("✅ Esta configuração:")
	fmt.Println("  • Não salva em histórico do shell")
	fmt.Println("  • Armazena em variável de ambiente")
	fmt.Println("  • Não aparece na tela ao digitar")
	fmt.Println()
	
	// Obter key de forma segura (sem exibir na tela)
	fmt.Printf("Cole sua API key para %s (não será exibida): ", providerName)
	
	// Desabilitar echo no terminal
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("erro ao ler key: %w", err)
	}
	fmt.Println() // Nova linha após input
	
	apiKey := strings.TrimSpace(string(bytePassword))
	
	if apiKey == "" {
		return fmt.Errorf("API key não pode ser vazia")
	}
	
	// Validar formato básico da key
	if !km.validateKeyFormat(provider, apiKey) {
		return fmt.Errorf("formato de API key inválido para %s", provider)
	}
	
	// Salvar como variável de ambiente para sessão atual
	envVar := km.getEnvVarName(provider)
	os.Setenv(envVar, apiKey)
	
	fmt.Println()
	fmt.Println("✅ API key configurada com sucesso!")
	fmt.Println()
	fmt.Printf("Para tornar permanente, adicione ao seu shell profile:\n")
	fmt.Printf("  echo 'export %s=\"sua-key\"' >> ~/.bashrc\n", envVar)
	fmt.Printf("  source ~/.bashrc\n")
	fmt.Println()
	fmt.Println("⚠️  IMPORTANTE: Use .env com .gitignore em projetos!")
	fmt.Println()
	
	return nil
}

// getEnvVarName retorna nome da variável de ambiente
func (km *KeyManager) getEnvVarName(provider llm.Provider) string {
	switch provider {
	case llm.ProviderClaude:
		return "ANTHROPIC_API_KEY"
	case llm.ProviderChatGPT:
		return "OPENAI_API_KEY"
	case llm.ProviderGemini:
		return "GEMINI_API_KEY"
	case llm.ProviderOllama:
		return "OLLAMA_ENDPOINT"
	case llm.ProviderAzure:
		return "AZURE_OPENAI_API_KEY"
	default:
		return "API_KEY"
	}
}

// getProviderName retorna nome amigável do provider
func (km *KeyManager) getProviderName(provider llm.Provider) string {
	switch provider {
	case llm.ProviderClaude:
		return "Claude (Anthropic)"
	case llm.ProviderChatGPT:
		return "ChatGPT (OpenAI)"
	case llm.ProviderGemini:
		return "Gemini (Google)"
	case llm.ProviderOllama:
		return "Ollama (Local)"
	case llm.ProviderAzure:
		return "Azure OpenAI"
	default:
		return string(provider)
	}
}

// validateKeyFormat valida formato básico da key
func (km *KeyManager) validateKeyFormat(provider llm.Provider, key string) bool {
	switch provider {
	case llm.ProviderClaude:
		// Claude keys começam com sk-ant-
		return strings.HasPrefix(key, "sk-ant-")
	case llm.ProviderChatGPT:
		// OpenAI keys começam com sk-
		return strings.HasPrefix(key, "sk-")
	case llm.ProviderGemini:
		// Gemini keys têm formato específico (mais flexível)
		return len(key) > 20
	case llm.ProviderOllama:
		// Ollama não requer API key (endpoint local)
		return true
	case llm.ProviderAzure:
		// Azure keys têm formato variável
		return len(key) > 20
	default:
		return len(key) > 10
	}
}

// PromptForKey solicita key interativamente (versão simples)
func (km *KeyManager) PromptForKey(provider llm.Provider) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	
	providerName := km.getProviderName(provider)
	envVar := km.getEnvVarName(provider)
	
	fmt.Printf("\n⚠️  API key para %s não encontrada\n\n", providerName)
	fmt.Println("Opções:")
	fmt.Printf("1. Cole sua key agora (será armazenada apenas nesta sessão)\n")
	fmt.Printf("2. Configure permanentemente: wtb auth setup --provider %s\n", provider)
	fmt.Printf("3. Use variável de ambiente: export %s='sua-key'\n\n", envVar)
	fmt.Print("Cole sua API key (Enter para cancelar): ")
	
	key, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("operação cancelada pelo usuário")
	}
	
	// Validar formato
	if !km.validateKeyFormat(provider, key) {
		return "", fmt.Errorf("formato de API key inválido")
	}
	
	// Salvar para sessão atual
	os.Setenv(envVar, key)
	
	fmt.Println("\n✅ Key configurada para esta sessão")
	fmt.Println("⚠️  Para configurar permanentemente, use: wtb auth setup")
	
	return key, nil
}

// CheckKeyFormat verifica se key tem formato válido (sem fazer API call)
func (km *KeyManager) CheckKeyFormat(provider llm.Provider, key string) error {
	if !km.validateKeyFormat(provider, key) {
		return fmt.Errorf("formato de API key inválido para %s\n\n"+
			"Formato esperado:\n"+
			"  Claude: sk-ant-...\n"+
			"  OpenAI: sk-...\n"+
			"  Gemini: key com 20+ caracteres",
			provider)
	}
	return nil
}
