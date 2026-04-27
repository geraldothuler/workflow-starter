package compliance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/security"
)

// GDPRExporter exporta dados para GDPR compliance
type GDPRExporter struct {
	configDir string
}

// NewGDPRExporter cria novo exporter
func NewGDPRExporter(configDir string) *GDPRExporter {
	return &GDPRExporter{
		configDir: configDir,
	}
}

// ExportMyData exporta todos os dados do usuário (GDPR Art. 15)
func (ge *GDPRExporter) ExportMyData(outputFile string) error {
	export := &GDPRExport{
		ExportDate:    time.Now(),
		DataController: "Workflow Platform User",
		DataCategories: []string{},
		Files:         []ExportedFile{},
	}

	// Coletar consent (v1 + v2)
	for _, consentFilename := range []string{"consent.json", "consent_v2.json"} {
		consentFile := filepath.Join(ge.configDir, consentFilename)
		if _, err := os.Stat(consentFile); err == nil {
			export.DataCategories = append(export.DataCategories, "consent_records")
			export.Files = append(export.Files, ExportedFile{
				Category:    "consent_records",
				Path:        consentFile,
				Description: "Registro de consentimento LGPD/GDPR",
			})
		}
	}

	// Coletar cached LLM responses (encrypted)
	cachePattern := filepath.Join(ge.configDir, "cache", "llm", "**", "*.enc")
	cacheFiles, _ := filepath.Glob(cachePattern)
	if len(cacheFiles) > 0 {
		export.DataCategories = append(export.DataCategories, "encrypted_cache")
		for _, file := range cacheFiles {
			export.Files = append(export.Files, ExportedFile{
				Category:    "encrypted_cache",
				Path:        file,
				Description: "Cache criptografado de respostas LLM",
			})
		}
	}

	// Coletar audit logs
	auditPattern := filepath.Join(ge.configDir, "audit", "audit-*.jsonl")
	auditFiles, _ := filepath.Glob(auditPattern)
	if len(auditFiles) > 0 {
		export.DataCategories = append(export.DataCategories, "audit_trail")
		for _, file := range auditFiles {
			export.Files = append(export.Files, ExportedFile{
				Category:    "audit_trail",
				Path:        file,
				Description: "Histórico de operações auditadas",
			})
		}
	}

	// Coletar backlogs
	backlogPattern := filepath.Join(ge.configDir, "backlog*.json")
	backlogFiles, _ := filepath.Glob(backlogPattern)
	if len(backlogFiles) > 0 {
		export.DataCategories = append(export.DataCategories, "generated_backlogs")
		for _, file := range backlogFiles {
			export.Files = append(export.Files, ExportedFile{
				Category:    "generated_backlogs",
				Path:        file,
				Description: "Backlogs técnicos gerados",
			})
		}
	}

	// Coletar deep dives
	ddPattern := filepath.Join(ge.configDir, "deep-dives*.json")
	ddFiles, _ := filepath.Glob(ddPattern)
	if len(ddFiles) > 0 {
		export.DataCategories = append(export.DataCategories, "deep_dives")
		for _, file := range ddFiles {
			export.Files = append(export.Files, ExportedFile{
				Category:    "deep_dives",
				Path:        file,
				Description: "Deep dives contextualizados",
			})
		}
	}

	// Adicionar metadados
	export.Summary = fmt.Sprintf("Export contains %d file(s) across %d data categories",
		len(export.Files), len(export.DataCategories))

	// Serializar
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar export: %w", err)
	}

	// Salvar
	if err := os.WriteFile(outputFile, data, 0600); err != nil {
		return fmt.Errorf("erro ao salvar export: %w", err)
	}

	return nil
}

// ForgetMe deleta todos os dados do usuário (GDPR Art. 17 - Right to be Forgotten).
// Uses SecureWipe to overwrite files with random data before deletion,
// preventing recovery from filesystem journals or undelete tools.
func (ge *GDPRExporter) ForgetMe(confirm bool) (*ForgetMeResult, error) {
	if !confirm {
		return nil, fmt.Errorf("operação requer confirmação explícita")
	}

	result := &ForgetMeResult{
		FilesDeleted: 0,
		Errors:       []string{},
	}

	// Patterns de arquivos a deletar (secure wipe)
	patterns := []string{
		filepath.Join(ge.configDir, "consent.json"),
		filepath.Join(ge.configDir, "consent_v2.json"),
		filepath.Join(ge.configDir, "audit", "audit-*.jsonl"),
		filepath.Join(ge.configDir, "backlog*.json"),
		filepath.Join(ge.configDir, "deep-dives*.json"),
		filepath.Join(ge.configDir, "extraction-report*.json"),
	}

	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("erro ao listar %s: %v", pattern, err))
			continue
		}

		for _, file := range files {
			// SECURITY: SecureWipe overwrites with random data before deleting
			if err := security.SecureWipe(file); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("erro ao deletar %s: %v", file, err))
				continue
			}
			result.FilesDeleted++
		}
	}

	// Secure wipe directories with sensitive data (cache, secrets)
	secureDirs := []string{
		filepath.Join(ge.configDir, "cache"),
		filepath.Join(ge.configDir, "secrets"),
	}
	for _, dir := range secureDirs {
		if _, err := os.Stat(dir); err == nil {
			if err := security.SecureWipeDir(dir); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("erro ao deletar %s: %v", dir, err))
			}
		}
	}

	// Deletar diretórios vazios
	os.Remove(filepath.Join(ge.configDir, "audit"))
	os.Remove(ge.configDir)

	return result, nil
}

// GDPRExport estrutura de export GDPR
type GDPRExport struct {
	ExportDate     time.Time      `json:"export_date"`
	DataController string         `json:"data_controller"`
	DataCategories []string       `json:"data_categories"`
	Files          []ExportedFile `json:"files"`
	Summary        string         `json:"summary"`
}

// ExportedFile arquivo exportado
type ExportedFile struct {
	Category    string `json:"category"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// ForgetMeResult resultado de forget me
type ForgetMeResult struct {
	FilesDeleted int
	Errors       []string
}

// Report gera relatório
func (fmr *ForgetMeResult) Report() string {
	report := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	report += "🗑️  RIGHT TO BE FORGOTTEN (GDPR Art. 17)\n"
	report += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"
	report += fmt.Sprintf("🗑️  Arquivos deletados: %d\n", fmr.FilesDeleted)

	if len(fmr.Errors) > 0 {
		report += fmt.Sprintf("\n⚠️  Erros (%d):\n", len(fmr.Errors))
		for _, err := range fmr.Errors {
			report += fmt.Sprintf("   • %s\n", err)
		}
	} else {
		report += "\n✅ Todos os dados foram removidos com sucesso\n"
	}

	report += "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"

	return report
}

// PortabilityReport gera relatório de portabilidade (GDPR Art. 20)
func (ge *GDPRExporter) PortabilityReport(outputFile string) error {
	report := &PortabilityReport{
		GeneratedAt: time.Now(),
		Format:      "JSON",
		StandardCompliance: []string{
			"GDPR Art. 20 (Right to Data Portability)",
			"LGPD Art. 18, VI (Portabilidade dos dados)",
		},
	}

	// Adicionar categorias de dados
	report.DataCategories = []DataCategory{
		{
			Name:        "Consent Records",
			Description: "Registros de consentimento LGPD/GDPR",
			Location:    ".workflow/consent.json",
		},
		{
			Name:        "Audit Trail",
			Description: "Histórico de operações auditadas",
			Location:    ".workflow/audit/audit-*.jsonl",
		},
		{
			Name:        "Generated Backlogs",
			Description: "Backlogs técnicos gerados",
			Location:    ".workflow/backlog*.json",
		},
	}

	// Serializar
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputFile, data, 0600)
}

// PortabilityReport relatório de portabilidade
type PortabilityReport struct {
	GeneratedAt        time.Time      `json:"generated_at"`
	Format             string         `json:"format"`
	StandardCompliance []string       `json:"standard_compliance"`
	DataCategories     []DataCategory `json:"data_categories"`
}

// DataCategory categoria de dados
type DataCategory struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Location    string `json:"location"`
}
