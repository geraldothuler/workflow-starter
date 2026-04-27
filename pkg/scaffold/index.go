package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RebuildTypeIndex rebuilds <typeDir>/INDEX.md from artefacts present.
func RebuildTypeIndex(typeDir, wfType string) error {
	entries, err := ListArtefacts(typeDir)
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s Index\n\n", strings.Title(wfType))) //nolint:staticcheck

	switch wfType {
	case "review":
		sb.WriteString("| # | Data | Titulo | Status | Score | Link |\n")
		sb.WriteString("|---|------|--------|--------|-------|------|\n")
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("| %s | — | %s | — | — | [%s](./%s) |\n", ExtractNNN(e), e, e, e))
		}
	default:
		sb.WriteString("| # | Data | Titulo | Status | Link |\n")
		sb.WriteString("|---|------|--------|--------|------|\n")
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("| %s | — | %s | — | [%s](./%s) |\n", ExtractNNN(e), e, e, e))
		}
	}

	sb.WriteString("\n---\n\n## Chain\n\n")
	sb.WriteString("<!-- Adicionar links de chain manualmente ou via wtb -->\n\n")
	sb.WriteString(fmt.Sprintf("---\n\n*Template: `~/workflow/use-cases/%s/template.md` | Gerado por `wtb index`*\n", wfType))

	return os.WriteFile(filepath.Join(typeDir, "INDEX.md"), []byte(sb.String()), 0644)
}

// RebuildMasterIndex rebuilds the top-level docs/workflow/INDEX.md.
func RebuildMasterIndex(workflowRoot string, presentTypes []string) error {
	var sb strings.Builder
	sb.WriteString("# Workflow Index\n\n")
	sb.WriteString("Indice mestre dos workflows documentados neste repositorio.\n\n")
	sb.WriteString("| Tipo | Link |\n")
	sb.WriteString("|------|------|\n")
	for _, t := range presentTypes {
		sb.WriteString(fmt.Sprintf("| **%s** | [%s/INDEX.md](%s/INDEX.md) |\n", t, t, t))
	}
	sb.WriteString("\n---\n\n")
	sb.WriteString("**Templates:** `~/workflow/use-cases/<tipo>/template.md` | `wtb new <tipo> <context>` para scaffolding.\n\n")
	sb.WriteString(fmt.Sprintf("*Gerado por `wtb index` em %s*\n", time.Now().Format("2006-01-02")))

	return os.WriteFile(filepath.Join(workflowRoot, "INDEX.md"), []byte(sb.String()), 0644)
}
