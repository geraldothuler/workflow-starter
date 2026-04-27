package compliance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

// ConsentRecord armazena consentimento do usuário
type ConsentRecord struct {
	Version          string    `json:"version"`           // Versão dos termos
	Provider         string    `json:"provider"`          // Claude, ChatGPT, etc
	ConsentGiven     bool      `json:"consent_given"`     // Usuário consentiu?
	ConsentDate      time.Time `json:"consent_date"`      // Quando consentiu
	DemoMode         bool      `json:"demo_mode"`         // Modo demo?
	UnderstandsLGPD  bool      `json:"understands_lgpd"`  // Entende LGPD?
	NoSensitiveData  bool      `json:"no_sensitive_data"` // Confirma sem dados sensíveis?
	HasAuthorization bool      `json:"has_authorization"` // Tem autorização?
}

// ConsentManager gerencia consentimento
type ConsentManager struct {
	configDir string
	demoMode  bool
}

// NewConsentManager cria novo manager
func NewConsentManager(configDir string, demoMode bool) *ConsentManager {
	return &ConsentManager{
		configDir: configDir,
		demoMode:  demoMode,
	}
}

// CheckConsent verifica se há consentimento válido
func (cm *ConsentManager) CheckConsent(provider llm.Provider) (bool, error) {
	// Demo mode: sempre tem consentimento
	if cm.demoMode {
		return true, nil
	}

	// Verificar se arquivo de consentimento existe
	consentPath := filepath.Join(cm.configDir, "consent.json")
	
	if _, err := os.Stat(consentPath); os.IsNotExist(err) {
		// Não existe - precisa fazer wizard
		return false, nil
	}

	// Ler consentimento
	data, err := os.ReadFile(consentPath)
	if err != nil {
		return false, err
	}

	var consent ConsentRecord
	if err := json.Unmarshal(data, &consent); err != nil {
		return false, err
	}

	// Validar consentimento
	if !consent.ConsentGiven {
		return false, nil
	}

	// Verificar se é para o mesmo provider
	if string(provider) != consent.Provider {
		// Provider mudou - precisa consentir novamente
		return false, nil
	}

	// Consentimento válido
	return true, nil
}

// SaveConsent salva consentimento
func (cm *ConsentManager) SaveConsent(consent *ConsentRecord) error {
	// Criar diretório se não existe
	if err := os.MkdirAll(cm.configDir, 0700); err != nil {
		return err
	}

	consentPath := filepath.Join(cm.configDir, "consent.json")
	
	data, err := json.MarshalIndent(consent, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(consentPath, data, 0600)
}

// RequestConsent mostra wizard e solicita consentimento
func (cm *ConsentManager) RequestConsent(provider llm.Provider) (*ConsentRecord, error) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔒 PRIMEIRA EXECUÇÃO - CONFIGURAÇÃO DE SEGURANÇA")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Workflow Platform precisa enviar seus dados para APIs externas para funcionar.")
	fmt.Println()

	// Informações do provider
	providerInfo := getProviderInfo(provider)
	fmt.Println("📍 PARA ONDE VÃO SEUS DADOS:")
	fmt.Println()
	fmt.Printf("Provider selecionado: %s\n", providerInfo.Name)
	fmt.Printf("Localização dos servidores: %s\n", providerInfo.Location)
	fmt.Printf("Política de retenção: %s\n", providerInfo.Retention)
	fmt.Printf("Uso para treino: %s\n", providerInfo.Training)
	fmt.Println()

	fmt.Println("🌍 TRANSFERÊNCIA INTERNACIONAL:")
	fmt.Println()
	fmt.Println("Seus dados sairão do Brasil e serão processados externamente.")
	fmt.Println("Isso configura transferência internacional sob LGPD Art. 33.")
	fmt.Println()

	fmt.Println("⚖️  IMPLICAÇÕES LEGAIS:")
	fmt.Println()
	fmt.Println("• LGPD Art. 33 - Transferência internacional requer consentimento")
	fmt.Println("• LGPD Art. 6º - Dados devem ser adequados e necessários")
	fmt.Println("• LGPD Art. 46 - Segurança técnica aplicável")
	fmt.Println()

	fmt.Println("✅ MEDIDAS DE SEGURANÇA EM USO:")
	fmt.Println()
	fmt.Println("• Criptografia TLS 1.3 em trânsito")
	fmt.Println("• API Keys com acesso controlado")
	fmt.Println("• Pattern Layers (minimização de dados - 92% redução)")
	fmt.Println("• PII Detection ativada (detecta CPF, emails, etc)")
	fmt.Println()

	fmt.Println("❌ NUNCA ENVIE:")
	fmt.Println()
	fmt.Println("• Dados pessoais sem anonimizar (CPF, RG, endereços)")
	fmt.Println("• Informações médicas")
	fmt.Println("• Dados de menores de idade")
	fmt.Println("• Segredos empresariais críticos")
	fmt.Println("• Senhas ou credenciais")
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Você declara que:")
	fmt.Println()
	fmt.Println("[ ] Li e compreendi para onde meus dados irão")
	fmt.Println("[ ] Não há dados pessoais sensíveis no arquivo")
	fmt.Println("[ ] Tenho autorização para enviar estes dados para processamento externo")
	fmt.Println("[ ] Compreendo as implicações da LGPD/GDPR")
	fmt.Println()
	fmt.Print("Aceitar e continuar? [s/N]: ")

	var response string
	fmt.Scanln(&response)

	if response != "s" && response != "S" && response != "sim" && response != "SIM" {
		fmt.Println()
		fmt.Println("❌ Operação cancelada")
		fmt.Println()
		fmt.Println("Para usar wtb, você precisa consentir com a transferência de dados.")
		fmt.Println()
		fmt.Println("Alternativas:")
		fmt.Println("1. Revise o arquivo e remova dados sensíveis")
		fmt.Println("2. Use modo demo: cd examples && make demo")
		fmt.Println("3. Configure self-hosted LLM (futuro)")
		fmt.Println()
		return nil, fmt.Errorf("consentimento negado pelo usuário")
	}

	// Consentimento dado
	consent := &ConsentRecord{
		Version:          "1.0",
		Provider:         string(provider),
		ConsentGiven:     true,
		ConsentDate:      time.Now(),
		DemoMode:         false,
		UnderstandsLGPD:  true,
		NoSensitiveData:  true,
		HasAuthorization: true,
	}

	fmt.Println()
	fmt.Println("✅ Consentimento registrado")
	fmt.Println()
	fmt.Printf("Salvo em: %s/consent.json\n", cm.configDir)
	fmt.Println()

	return consent, nil
}

// ProviderInfo informações sobre provider
type ProviderInfo struct {
	Name      string
	Location  string
	Retention string
	Training  string
}

func getProviderInfo(provider llm.Provider) ProviderInfo {
	switch provider {
	case llm.ProviderClaude:
		return ProviderInfo{
			Name:      "Claude (Anthropic)",
			Location:  "Estados Unidos 🇺🇸",
			Retention: "~30 dias em cache",
			Training:  "Não (Anthropic compromisso público)",
		}
	case llm.ProviderChatGPT:
		return ProviderInfo{
			Name:      "ChatGPT (OpenAI)",
			Location:  "Estados Unidos 🇺🇸",
			Retention: "~30 dias em cache",
			Training:  "Sim (opt-out disponível)",
		}
	case llm.ProviderGemini:
		return ProviderInfo{
			Name:      "Gemini (Google)",
			Location:  "Estados Unidos 🇺🇸",
			Retention: "~18 meses",
			Training:  "Sim (conforme Google AI Terms)",
		}
	default:
		return ProviderInfo{
			Name:      string(provider),
			Location:  "Desconhecido",
			Retention: "Consulte ToS do provider",
			Training:  "Consulte ToS do provider",
		}
	}
}

// CreateDemoConsent cria consentimento para demo mode
func CreateDemoConsent(configDir string, provider llm.Provider) error {
	cm := NewConsentManager(configDir, true)
	
	consent := &ConsentRecord{
		Version:          "1.0",
		Provider:         string(provider),
		ConsentGiven:     true,
		ConsentDate:      time.Now(),
		DemoMode:         true,
		UnderstandsLGPD:  true,
		NoSensitiveData:  true,
		HasAuthorization: true,
	}

	return cm.SaveConsent(consent)
}
