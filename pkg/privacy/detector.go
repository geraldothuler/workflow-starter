package privacy

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// PIIType tipo de dado pessoal
type PIIType string

const (
	PIITypeCPF       PIIType = "CPF"
	PIITypeCNPJ      PIIType = "CNPJ"
	PIITypeEmail     PIIType = "Email"
	PIITypePhone     PIIType = "Telefone"
	PIITypeIP        PIIType = "IP"
	PIITypeCreditCard PIIType = "Cartão de Crédito"
)

// PIIDetection representa uma detecção de dado pessoal
type PIIDetection struct {
	Type     PIIType
	Value    string // Valor parcialmente mascarado
	Position int    // Posição no texto
	Line     int    // Linha no arquivo
}

// Detector detecta dados pessoais
type Detector struct {
	patterns map[PIIType]*regexp.Regexp
}

// NewDetector cria novo detector
func NewDetector() *Detector {
	return &Detector{
		patterns: map[PIIType]*regexp.Regexp{
			// CPF: 123.456.789-00 ou 12345678900
			PIITypeCPF: regexp.MustCompile(`\d{3}\.?\d{3}\.?\d{3}-?\d{2}`),
			
			// CNPJ: 12.345.678/0001-00 ou 12345678000100
			PIITypeCNPJ: regexp.MustCompile(`\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}`),
			
			// Email
			PIITypeEmail: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			
			// Telefone brasileiro: (11) 98765-4321, 11987654321, +5511987654321
			PIITypePhone: regexp.MustCompile(`(\+55\s?)?(\(?\d{2}\)?[\s-]?)?\d{4,5}[\s-]?\d{4}`),
			
			// IP Address
			PIITypeIP: regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
			
			// Cartão de crédito (formato simplificado)
			PIITypeCreditCard: regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
		},
	}
}

// Scan escaneia texto em busca de dados pessoais
func (d *Detector) Scan(text string) []PIIDetection {
	detections := []PIIDetection{}
	lines := strings.Split(text, "\n")

	for lineNum, line := range lines {
		for piiType, pattern := range d.patterns {
			matches := pattern.FindAllStringIndex(line, -1)
			
			for _, match := range matches {
				value := line[match[0]:match[1]]
				
				// Validar se é realmente PII (evitar falsos positivos)
				if d.validate(piiType, value) {
					detections = append(detections, PIIDetection{
						Type:     piiType,
						Value:    d.mask(value),
						Position: match[0],
						Line:     lineNum + 1,
					})
				}
			}
		}
	}

	return detections
}

// validate valida se match é realmente PII (reduz falsos positivos)
func (d *Detector) validate(piiType PIIType, value string) bool {
	switch piiType {
	case PIITypeCPF:
		return d.validateCPF(value)
	case PIITypeCNPJ:
		return d.validateCNPJ(value)
	case PIITypeIP:
		// Ignorar IPs locais comuns
		return !strings.HasPrefix(value, "127.") && 
		       !strings.HasPrefix(value, "192.168.") &&
		       !strings.HasPrefix(value, "10.")
	case PIITypeCreditCard:
		// Validar Luhn algorithm (básico)
		return d.validateLuhn(value)
	default:
		return true // Outros tipos sempre validam
	}
}

// validateCPF validação básica de CPF
func (d *Detector) validateCPF(cpf string) bool {
	// Remover pontuação
	cpf = regexp.MustCompile(`[^\d]`).ReplaceAllString(cpf, "")
	
	// CPF deve ter 11 dígitos
	if len(cpf) != 11 {
		return false
	}
	
	// CPFs inválidos comuns (todos os dígitos iguais)
	invalid := []string{
		"00000000000", "11111111111", "22222222222",
		"33333333333", "44444444444", "55555555555",
		"66666666666", "77777777777", "88888888888",
		"99999999999",
	}
	
	for _, inv := range invalid {
		if cpf == inv {
			return false
		}
	}
	
	return true
}

// validateCNPJ validação básica de CNPJ
func (d *Detector) validateCNPJ(cnpj string) bool {
	// Remover pontuação
	cnpj = regexp.MustCompile(`[^\d]`).ReplaceAllString(cnpj, "")
	
	// CNPJ deve ter 14 dígitos
	return len(cnpj) == 14
}

// validateLuhn algoritmo de Luhn para validar cartões
func (d *Detector) validateLuhn(number string) bool {
	// Remover espaços e hífens
	number = regexp.MustCompile(`[\s-]`).ReplaceAllString(number, "")
	
	if len(number) < 13 || len(number) > 19 {
		return false
	}
	
	sum := 0
	double := false
	
	// Processar de trás para frente
	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')
		
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		
		sum += digit
		double = !double
	}
	
	return sum%10 == 0
}

// mask mascara valor para exibição
func (d *Detector) mask(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	// Mostrar apenas últimos 4 caracteres
	return "***" + value[len(value)-4:]
}

// Report gera relatório de detecções
func (d *Detector) Report(detections []PIIDetection) string {
	if len(detections) == 0 {
		return "✅ Nenhum dado sensível detectado"
	}

	report := fmt.Sprintf("⚠️  %d dado(s) sensível(is) detectado(s):\n\n", len(detections))
	
	// Agrupar por tipo
	byType := make(map[PIIType][]PIIDetection)
	for _, detection := range detections {
		byType[detection.Type] = append(byType[detection.Type], detection)
	}
	
	for piiType, items := range byType {
		report += fmt.Sprintf("❌ %s (%d ocorrência(s)):\n", piiType, len(items))
		for i, item := range items {
			if i < 3 { // Mostrar apenas 3 exemplos
				report += fmt.Sprintf("   Linha %d: %s\n", item.Line, item.Value)
			}
		}
		if len(items) > 3 {
			report += fmt.Sprintf("   ... e mais %d ocorrência(s)\n", len(items)-3)
		}
		report += "\n"
	}
	
	return report
}

// ScanFile escaneia arquivo
func (d *Detector) ScanFile(filepath string) ([]PIIDetection, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	
	return d.Scan(string(content)), nil
}

// Anonymize anonimiza dados pessoais (substitui por placeholders)
func (d *Detector) Anonymize(text string) string {
	result := text
	
	// CPF
	result = d.patterns[PIITypeCPF].ReplaceAllString(result, "[CPF-REMOVIDO]")
	
	// CNPJ
	result = d.patterns[PIITypeCNPJ].ReplaceAllString(result, "[CNPJ-REMOVIDO]")
	
	// Email
	result = d.patterns[PIITypeEmail].ReplaceAllString(result, "[EMAIL-REMOVIDO]")
	
	// Telefone
	result = d.patterns[PIITypePhone].ReplaceAllString(result, "[TELEFONE-REMOVIDO]")
	
	// IP
	result = d.patterns[PIITypeIP].ReplaceAllStringFunc(result, func(ip string) string {
		if strings.HasPrefix(ip, "127.") || 
		   strings.HasPrefix(ip, "192.168.") ||
		   strings.HasPrefix(ip, "10.") {
			return ip // Manter IPs locais
		}
		return "[IP-REMOVIDO]"
	})
	
	// Cartão de crédito
	result = d.patterns[PIITypeCreditCard].ReplaceAllString(result, "[CARTAO-REMOVIDO]")
	
	return result
}
