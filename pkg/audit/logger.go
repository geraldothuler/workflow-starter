package audit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/logging"
)

// EventType tipo de evento auditável
type EventType string

const (
	EventExtract         EventType = "extract"
	EventBacklogGenerate EventType = "backlog_generate"
	EventConsentGiven    EventType = "consent_given"
	EventPIIDetected     EventType = "pii_detected"
	EventAPIKeySetup     EventType = "api_key_setup"
	EventFileExport      EventType = "file_export"
)

// Event evento de auditoria
type Event struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	EventType    EventType              `json:"event_type"`
	User         string                 `json:"user,omitempty"`
	Provider     string                 `json:"provider,omitempty"`
	InputFile    string                 `json:"input_file,omitempty"`
	OutputFile   string                 `json:"output_file,omitempty"`
	DataHash     string                 `json:"data_hash,omitempty"`      // SHA256 do input (não o conteúdo)
	TokensUsed   int                    `json:"tokens_used,omitempty"`
	Cost         float64                `json:"cost,omitempty"`
	Success      bool                   `json:"success"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Logger audit logger
type Logger struct {
	auditDir  string
	enabled   bool
	sanitizer *logging.Sanitizer
}

// NewLogger cria novo audit logger.
// All audit entries are sanitized before persistence to prevent credential/PII leakage.
func NewLogger(auditDir string, enabled bool) *Logger {
	return &Logger{
		auditDir:  auditDir,
		enabled:   enabled,
		sanitizer: logging.NewSanitizer(true),
	}
}

// Log registra evento
func (l *Logger) Log(event Event) error {
	if !l.enabled {
		return nil
	}

	// Criar diretório se não existe
	if err := os.MkdirAll(l.auditDir, 0700); err != nil {
		return fmt.Errorf("erro ao criar diretório de auditoria: %w", err)
	}

	// Gerar ID único
	if event.ID == "" {
		event.ID = generateEventID()
	}

	// Timestamp
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Nome do arquivo: audit-YYYY-MM-DD.jsonl
	filename := fmt.Sprintf("audit-%s.jsonl", event.Timestamp.Format("2006-01-02"))
	filepath := filepath.Join(l.auditDir, filename)

	// SECURITY: Sanitize sensitive fields before persistence.
	// ErrorMessage and Metadata may contain credentials or PII from error contexts.
	if event.ErrorMessage != "" {
		event.ErrorMessage = l.sanitizer.Sanitize(event.ErrorMessage)
	}
	if event.Metadata != nil {
		event.Metadata = l.sanitizer.SanitizeForExport(event.Metadata)
	}

	// Serializar evento
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("erro ao serializar evento: %w", err)
	}

	// Append ao arquivo (JSONL - JSON Lines)
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("erro ao abrir arquivo de auditoria: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("erro ao escrever evento: %w", err)
	}

	return nil
}

// LogExtract registra extração
func (l *Logger) LogExtract(inputFile, outputFile string, success bool, errorMsg string) error {
	return l.Log(Event{
		EventType:    EventExtract,
		InputFile:    inputFile,
		OutputFile:   outputFile,
		DataHash:     hashFile(inputFile),
		Success:      success,
		ErrorMessage: errorMsg,
	})
}

// LogBacklogGenerate registra geração de backlog
func (l *Logger) LogBacklogGenerate(provider llm.Provider, inputFile string, tokensUsed int, cost float64, success bool, errorMsg string) error {
	return l.Log(Event{
		EventType:    EventBacklogGenerate,
		Provider:     string(provider),
		InputFile:    inputFile,
		DataHash:     hashFile(inputFile),
		TokensUsed:   tokensUsed,
		Cost:         cost,
		Success:      success,
		ErrorMessage: errorMsg,
	})
}

// LogConsent registra consentimento
func (l *Logger) LogConsent(provider llm.Provider, understands, noSensitiveData, hasAuthorization bool) error {
	return l.Log(Event{
		EventType: EventConsentGiven,
		Provider:  string(provider),
		Success:   true,
		Metadata: map[string]interface{}{
			"understands_lgpd":    understands,
			"no_sensitive_data":   noSensitiveData,
			"has_authorization":   hasAuthorization,
		},
	})
}

// LogPIIDetection registra detecção de PII
func (l *Logger) LogPIIDetection(inputFile string, detectionsCount int, types []string) error {
	return l.Log(Event{
		EventType: EventPIIDetected,
		InputFile: inputFile,
		DataHash:  hashFile(inputFile),
		Success:   true,
		Metadata: map[string]interface{}{
			"detections_count": detectionsCount,
			"types_detected":   types,
		},
	})
}

// LogAPIKeySetup registra configuração de API key
func (l *Logger) LogAPIKeySetup(provider llm.Provider, success bool) error {
	return l.Log(Event{
		EventType: EventAPIKeySetup,
		Provider:  string(provider),
		Success:   success,
	})
}

// LogExport registra export de arquivo
func (l *Logger) LogExport(outputFile, format string, success bool) error {
	return l.Log(Event{
		EventType:  EventFileExport,
		OutputFile: outputFile,
		Success:    success,
		Metadata: map[string]interface{}{
			"format": format,
		},
	})
}

// Query busca eventos
func (l *Logger) Query(filters QueryFilters) ([]Event, error) {
	if !l.enabled {
		return nil, fmt.Errorf("audit logging está desabilitado")
	}

	var events []Event

	// Listar arquivos de audit
	files, err := filepath.Glob(filepath.Join(l.auditDir, "audit-*.jsonl"))
	if err != nil {
		return nil, err
	}

	// Ler cada arquivo
	for _, file := range files {
		fileEvents, err := readAuditFile(file, filters)
		if err != nil {
			continue // Skip arquivo com erro
		}
		events = append(events, fileEvents...)
	}

	return events, nil
}

// QueryFilters filtros para query
type QueryFilters struct {
	EventType *EventType
	Provider  *string
	StartDate *time.Time
	EndDate   *time.Time
	Success   *bool
	Limit     int
}

// readAuditFile lê arquivo de audit
func readAuditFile(filepath string, filters QueryFilters) ([]Event, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var events []Event
	lines := splitLines(string(data))

	for _, line := range lines {
		if line == "" {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip linha inválida
		}

		// Aplicar filtros
		if filters.EventType != nil && event.EventType != *filters.EventType {
			continue
		}
		if filters.Provider != nil && event.Provider != *filters.Provider {
			continue
		}
		if filters.StartDate != nil && event.Timestamp.Before(*filters.StartDate) {
			continue
		}
		if filters.EndDate != nil && event.Timestamp.After(*filters.EndDate) {
			continue
		}
		if filters.Success != nil && event.Success != *filters.Success {
			continue
		}

		events = append(events, event)

		// Limite
		if filters.Limit > 0 && len(events) >= filters.Limit {
			break
		}
	}

	return events, nil
}

// ExportToCSV exporta audit trail para CSV
func (l *Logger) ExportToCSV(outputFile string, filters QueryFilters) error {
	events, err := l.Query(filters)
	if err != nil {
		return err
	}

	// Criar arquivo CSV
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	// Header
	f.WriteString("timestamp,event_type,provider,input_file,tokens_used,cost,success,error_message\n")

	// Eventos
	for _, event := range events {
		f.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d,%.4f,%t,%s\n",
			event.Timestamp.Format(time.RFC3339),
			event.EventType,
			event.Provider,
			event.InputFile,
			event.TokensUsed,
			event.Cost,
			event.Success,
			escapeCSV(event.ErrorMessage),
		))
	}

	return nil
}

// GetStats retorna estatísticas
func (l *Logger) GetStats() (Stats, error) {
	events, err := l.Query(QueryFilters{})
	if err != nil {
		return Stats{}, err
	}

	stats := Stats{
		TotalEvents: len(events),
		ByType:      make(map[EventType]int),
		ByProvider:  make(map[string]int),
	}

	for _, event := range events {
		stats.ByType[event.EventType]++
		if event.Provider != "" {
			stats.ByProvider[event.Provider]++
		}
		if event.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}
		stats.TotalCost += event.Cost
		stats.TotalTokens += event.TokensUsed
	}

	return stats, nil
}

// Stats estatísticas de auditoria
type Stats struct {
	TotalEvents  int
	SuccessCount int
	FailureCount int
	TotalCost    float64
	TotalTokens  int
	ByType       map[EventType]int
	ByProvider   map[string]int
}

// Helper functions

func generateEventID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func hashFile(filepath string) string {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func escapeCSV(s string) string {
	if s == "" {
		return ""
	}
	// Se contém vírgula ou aspas, envolver em aspas
	if containsAny(s, ",\"") {
		s = `"` + replaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func containsAny(s, chars string) bool {
	for _, c := range chars {
		for _, sc := range s {
			if c == sc {
				return true
			}
		}
	}
	return false
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i <= len(s)-len(old) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}
