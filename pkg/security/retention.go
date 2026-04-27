package security

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RetentionPolicy política de retenção de dados
type RetentionPolicy struct {
	RetentionDays    int      `json:"retention_days"`
	AutoCleanup      bool     `json:"auto_cleanup"`
	Exceptions       []string `json:"exceptions"`
	NotifyBeforeDays int      `json:"notify_before_days"`
}

// DefaultRetentionPolicy política padrão
func DefaultRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		RetentionDays:    30,
		AutoCleanup:      false, // Manual por padrão
		Exceptions:       []string{},
		NotifyBeforeDays: 7,
	}
}

// RetentionManager gerencia retenção de dados
type RetentionManager struct {
	policy    *RetentionPolicy
	configDir string
}

// NewRetentionManager cria novo manager
func NewRetentionManager(configDir string, policy *RetentionPolicy) *RetentionManager {
	if policy == nil {
		policy = DefaultRetentionPolicy()
	}
	return &RetentionManager{
		policy:    policy,
		configDir: configDir,
	}
}

// FindExpiredFiles encontra arquivos expirados
func (rm *RetentionManager) FindExpiredFiles() ([]FileInfo, error) {
	var expired []FileInfo

	// Data de corte
	cutoff := time.Now().AddDate(0, 0, -rm.policy.RetentionDays)

	// Padrões de arquivos a verificar
	patterns := []string{
		filepath.Join(rm.configDir, "backlog-*.json"),
		filepath.Join(rm.configDir, "deep-dives-*.json"),
		filepath.Join(rm.configDir, "extraction-report-*.json"),
	}

	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, file := range files {
			// Verificar se é exceção
			if rm.isException(file) {
				continue
			}

			// Verificar idade
			info, err := os.Stat(file)
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				expired = append(expired, FileInfo{
					Path:    file,
					Size:    info.Size(),
					ModTime: info.ModTime(),
					Age:     time.Since(info.ModTime()),
				})
			}
		}
	}

	return expired, nil
}

// FindExpiringFiles encontra arquivos que vão expirar em breve
func (rm *RetentionManager) FindExpiringFiles() ([]FileInfo, error) {
	var expiring []FileInfo

	// Janela de notificação
	cutoffExpired := time.Now().AddDate(0, 0, -rm.policy.RetentionDays)
	cutoffExpiring := time.Now().AddDate(0, 0, -(rm.policy.RetentionDays - rm.policy.NotifyBeforeDays))

	patterns := []string{
		filepath.Join(rm.configDir, "backlog-*.json"),
		filepath.Join(rm.configDir, "deep-dives-*.json"),
	}

	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, file := range files {
			if rm.isException(file) {
				continue
			}

			info, err := os.Stat(file)
			if err != nil {
				continue
			}

			// Entre cutoffExpiring e cutoffExpired
			if info.ModTime().After(cutoffExpired) && info.ModTime().Before(cutoffExpiring) {
				expiring = append(expiring, FileInfo{
					Path:    file,
					Size:    info.Size(),
					ModTime: info.ModTime(),
					Age:     time.Since(info.ModTime()),
				})
			}
		}
	}

	return expiring, nil
}

// Cleanup limpa arquivos expirados
func (rm *RetentionManager) Cleanup(dryRun bool) (*CleanupResult, error) {
	expired, err := rm.FindExpiredFiles()
	if err != nil {
		return nil, err
	}

	result := &CleanupResult{
		DryRun:        dryRun,
		FilesFound:    len(expired),
		FilesDeleted:  0,
		BytesFreed:    0,
		Errors:        []string{},
	}

	for _, file := range expired {
		if dryRun {
			result.BytesFreed += file.Size
			continue
		}

		// SECURITY: SecureWipe overwrites with random data before deleting
		if err := SecureWipe(file.Path); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file.Path, err))
			continue
		}

		result.FilesDeleted++
		result.BytesFreed += file.Size
	}

	return result, nil
}

// AddException adiciona exceção
func (rm *RetentionManager) AddException(pattern string) {
	rm.policy.Exceptions = append(rm.policy.Exceptions, pattern)
}

// isException verifica se arquivo é exceção
func (rm *RetentionManager) isException(filePath string) bool {
	for _, pattern := range rm.policy.Exceptions {
		matched, err := filepath.Match(pattern, filePath)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// GetPolicy retorna política atual
func (rm *RetentionManager) GetPolicy() *RetentionPolicy {
	return rm.policy
}

// FileInfo informações de arquivo
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	Age     time.Duration
}

// CleanupResult resultado de cleanup
type CleanupResult struct {
	DryRun       bool
	FilesFound   int
	FilesDeleted int
	BytesFreed   int64
	Errors       []string
}

// Report gera relatório
func (cr *CleanupResult) Report() string {
	report := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	
	if cr.DryRun {
		report += "🔍 SIMULAÇÃO DE CLEANUP (Dry Run)\n"
	} else {
		report += "🗑️  CLEANUP EXECUTADO\n"
	}
	
	report += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"
	report += fmt.Sprintf("📁 Arquivos encontrados: %d\n", cr.FilesFound)
	
	if !cr.DryRun {
		report += fmt.Sprintf("🗑️  Arquivos deletados: %d\n", cr.FilesDeleted)
		report += fmt.Sprintf("💾 Espaço liberado: %s\n", formatBytes(cr.BytesFreed))
		
		if len(cr.Errors) > 0 {
			report += fmt.Sprintf("\n⚠️  Erros (%d):\n", len(cr.Errors))
			for _, err := range cr.Errors {
				report += fmt.Sprintf("   • %s\n", err)
			}
		}
	} else {
		report += fmt.Sprintf("💾 Espaço a liberar: %s\n", formatBytes(cr.BytesFreed))
		report += "\n⚠️  Para executar de verdade, remova --dry-run\n"
	}
	
	report += "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	
	return report
}

// formatBytes formata bytes de forma legível
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ScheduleCleanup runs cleanup if auto-cleanup is enabled in the policy.
// In a CLI context, this executes cleanup immediately (no cron/scheduler needed).
// Returns nil if auto-cleanup is disabled (no-op), or the cleanup result.
func (rm *RetentionManager) ScheduleCleanup() error {
	if !rm.policy.AutoCleanup {
		return nil // Auto-cleanup disabled — manual only
	}

	result, err := rm.Cleanup(false)
	if err != nil {
		return fmt.Errorf("auto-cleanup failed: %w", err)
	}

	if result.FilesDeleted > 0 && len(result.Errors) > 0 {
		return fmt.Errorf("auto-cleanup: deleted %d files, but %d errors occurred",
			result.FilesDeleted, len(result.Errors))
	}

	return nil
}
