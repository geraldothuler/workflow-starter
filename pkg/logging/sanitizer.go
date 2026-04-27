package logging

import (
	"fmt"
	"regexp"
	"strings"
)

// Sanitizer sanitiza logs removendo dados sensíveis
type Sanitizer struct {
	patterns map[string]*regexp.Regexp
	enabled  bool
}

// NewSanitizer cria novo sanitizer
func NewSanitizer(enabled bool) *Sanitizer {
	return &Sanitizer{
		enabled: enabled,
		patterns: map[string]*regexp.Regexp{
			// API Keys
			"anthropic_key": regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]{95,}`),
			"openai_key":    regexp.MustCompile(`sk-[a-zA-Z0-9]{48,}`),
			"generic_key":   regexp.MustCompile(`[a-zA-Z0-9_-]{32,}`),
			
			// PII
			"cpf":           regexp.MustCompile(`\d{3}\.?\d{3}\.?\d{3}-?\d{2}`),
			"cnpj":          regexp.MustCompile(`\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}`),
			"email":         regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			"phone":         regexp.MustCompile(`(\+55\s?)?(\(?\d{2}\)?[\s-]?)?\d{4,5}[\s-]?\d{4}`),
			"ip":            regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
			"credit_card":   regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
			
			// Senhas e tokens
			"password":      regexp.MustCompile(`(?i)(password|senha|pwd|pass)\s*[=:]\s*['"]?([^'"\s]+)['"]?`),
			"bearer_token":  regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9._-]+`),
			"jwt":           regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`),
		},
	}
}

// Sanitize sanitiza texto
func (s *Sanitizer) Sanitize(text string) string {
	if !s.enabled {
		return text
	}

	result := text

	// API Keys
	result = s.patterns["anthropic_key"].ReplaceAllString(result, "[ANTHROPIC-KEY-REDACTED]")
	result = s.patterns["openai_key"].ReplaceAllString(result, "[OPENAI-KEY-REDACTED]")
	
	// PII
	result = s.patterns["cpf"].ReplaceAllString(result, "[CPF-REDACTED]")
	result = s.patterns["cnpj"].ReplaceAllString(result, "[CNPJ-REDACTED]")
	result = s.patterns["email"].ReplaceAllString(result, "[EMAIL-REDACTED]")
	result = s.patterns["phone"].ReplaceAllString(result, "[PHONE-REDACTED]")
	result = s.patterns["credit_card"].ReplaceAllString(result, "[CARD-REDACTED]")
	
	// IPs (exceto locais)
	result = s.patterns["ip"].ReplaceAllStringFunc(result, func(ip string) string {
		if strings.HasPrefix(ip, "127.") || 
		   strings.HasPrefix(ip, "192.168.") ||
		   strings.HasPrefix(ip, "10.") {
			return ip // Manter IPs locais
		}
		return "[IP-REDACTED]"
	})
	
	// Senhas
	result = s.patterns["password"].ReplaceAllString(result, "$1=[PASSWORD-REDACTED]")
	result = s.patterns["bearer_token"].ReplaceAllString(result, "Bearer [TOKEN-REDACTED]")
	result = s.patterns["jwt"].ReplaceAllString(result, "[JWT-REDACTED]")

	return result
}

// SafeLog retorna versão segura para logging
func (s *Sanitizer) SafeLog(format string, args ...interface{}) string {
	// Formatar string
	msg := fmt.Sprintf(format, args...)
	
	// Sanitizar
	return s.Sanitize(msg)
}

// SafePrintf imprime de forma segura
func (s *Sanitizer) SafePrintf(format string, args ...interface{}) {
	safe := s.SafeLog(format, args...)
	fmt.Print(safe)
}

// SafePrintln imprime de forma segura com nova linha
func (s *Sanitizer) SafePrintln(args ...interface{}) {
	msg := fmt.Sprintln(args...)
	safe := s.Sanitize(msg)
	fmt.Print(safe)
}

// Level representa nível de verbosidade
type Level int

const (
	LevelQuiet   Level = 0 // Apenas erros críticos
	LevelNormal  Level = 1 // Progresso normal
	LevelVerbose Level = 2 // Detalhes adicionais
	LevelDebug   Level = 3 // Tudo, inclusive dados sensíveis (CUIDADO!)
)

// Logger logger seguro
type Logger struct {
	sanitizer *Sanitizer
	level     Level
}

// NewLogger cria novo logger
func NewLogger(level Level, sanitizeEnabled bool) *Logger {
	return &Logger{
		sanitizer: NewSanitizer(sanitizeEnabled),
		level:     level,
	}
}

// Quiet log apenas se level >= Quiet (sempre)
func (l *Logger) Quiet(format string, args ...interface{}) {
	if l.level >= LevelQuiet {
		l.sanitizer.SafePrintf(format, args...)
	}
}

// Info log se level >= Normal
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LevelNormal {
		l.sanitizer.SafePrintf(format, args...)
	}
}

// Verbose log se level >= Verbose
func (l *Logger) Verbose(format string, args ...interface{}) {
	if l.level >= LevelVerbose {
		l.sanitizer.SafePrintf(format, args...)
	}
}

// Debug log se level >= Debug (sanitizado — credenciais nunca aparecem em output)
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LevelDebug {
		msg := fmt.Sprintf(format, args...)
		fmt.Printf("[DEBUG] %s", l.sanitizer.Sanitize(msg))
	}
}

// Error sempre loga erros (sanitizado)
func (l *Logger) Error(format string, args ...interface{}) {
	l.sanitizer.SafePrintf("❌ ERROR: "+format, args...)
}

// Warning sempre loga warnings (sanitizado)
func (l *Logger) Warning(format string, args ...interface{}) {
	l.sanitizer.SafePrintf("⚠️  WARNING: "+format, args...)
}

// Success sempre loga sucessos
func (l *Logger) Success(format string, args ...interface{}) {
	l.sanitizer.SafePrintf("✅ "+format, args...)
}

// SanitizeForExport sanitiza dados antes de export
func (s *Sanitizer) SanitizeForExport(data map[string]interface{}) map[string]interface{} {
	if !s.enabled {
		return data
	}

	result := make(map[string]interface{})
	
	for key, value := range data {
		switch v := value.(type) {
		case string:
			result[key] = s.Sanitize(v)
		case map[string]interface{}:
			result[key] = s.SanitizeForExport(v)
		case []interface{}:
			sanitized := make([]interface{}, len(v))
			for i, item := range v {
				if str, ok := item.(string); ok {
					sanitized[i] = s.Sanitize(str)
				} else if m, ok := item.(map[string]interface{}); ok {
					sanitized[i] = s.SanitizeForExport(m)
				} else {
					sanitized[i] = item
				}
			}
			result[key] = sanitized
		default:
			result[key] = value
		}
	}
	
	return result
}

// TruncateForLog trunca strings longas para logs
func TruncateForLog(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "... [truncated]"
}

// MaskSensitive mascara parte sensível mantendo contexto
func MaskSensitive(value string, showFirst, showLast int) string {
	if len(value) <= showFirst+showLast {
		return strings.Repeat("*", len(value))
	}
	
	return value[:showFirst] + 
	       strings.Repeat("*", len(value)-showFirst-showLast) + 
	       value[len(value)-showLast:]
}
