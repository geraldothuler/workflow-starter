// wtb — workflow toolbox CLI
// Single entry point for the workflow platform.
// Personal layer: export, import, status, security-accept, delete-personal-data, update
// Scaffolding: new, index, list
// Pipeline runner: run
package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/chain"
	"github.com/Cobliteam/workflow-toolkit/pkg/mcp"
	"github.com/Cobliteam/workflow-toolkit/pkg/runner"
	"github.com/Cobliteam/workflow-toolkit/pkg/scaffold"
	"github.com/spf13/cobra"
)

const (
	version         = "1.0.0"
	securityVersion = "1.0"
	workflowDir     = "workflow"
	personalDir     = ".workflow"
)

// credentialPatterns are used in security guardrail tests.
// Patterns match literal values but NOT variable references (starting with $).
var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)password\s*=\s*[^$\s]\S+`),
	regexp.MustCompile(`(?i)secret\s*=\s*[^$\s]\S+`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`), // AWS access key (literal)
	regexp.MustCompile(`(?i)token\s*=\s*[^$\s][a-zA-Z0-9+/]{20,}`),
}

func personalDirPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot determine home directory")
		os.Exit(1)
	}
	return filepath.Join(home, personalDir)
}

func workflowDirPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot determine home directory")
		os.Exit(1)
	}
	return filepath.Join(home, workflowDir)
}

func securityContractPath() string {
	return filepath.Join(personalDirPath(), "security-contract.md")
}

func main() {
	root := &cobra.Command{
		Use:   "wtb",
		Short: "workflow toolbox — single entry point da plataforma de workflow",
		Long: `wtb e o CLI unificado da plataforma de workflow.

Camada pessoal (~/.workflow/):
  export, import, status, security-accept, delete-personal-data, update

Scaffolding de artefatos (repos):
  new    — cria novo artefato a partir de template
  index  — reconstroi INDEX.md de um repo
  list   — lista artefatos de um repo

Ops Toolbox (acesso direto, zero-LLM):
  ops probe / db-health / k8s-status / kafka-status / logs-analyze
  ops plan new / show / execute`,
	}

	root.AddCommand(
		// Pipeline runner
		newRunCmd(),
		// Chain traversal for documentary use-cases
		newChainCmd(),
		// Webhook management
		newWebhookCmd(),
		// Ops Toolbox (direct access)
		newOpsCmd(),
		// Camada pessoal
		newExportCmd(),
		newImportCmd(),
		newStatusCmd(),
		newSecurityAcceptCmd(),
		newDeletePersonalDataCmd(),
		newUpdateCmd(),
		// Scaffolding
		newNewCmd(),
		newIndexCmd(),
		newListCmd(),
		// MCP server
		newMCPServeCmd(),
		// Cycle detection
		newCycleCheckCmd(),
		// Monitor
		newMonitorCmd(),
		// Docs generation
		newDocsCmd(),
		// Drift containment
		newGuardrailCmd(),
		// Ops result log (Discovery 011 Camada 1)
		newStoreCmd(),
		// Integration test environment orchestration
		newTestEnvCmd(),
		// Memory management (get, where, set, list, validate)
		newMemoryCmd(),
		// Operational task backlog (SQLite)
		newBacklogCmd(),
		// Workflow artefact store (discovery, savepoint, runbook, 1on1, ...)
		newDocCmd(),
		// Repo code intelligence (index, show, query, note)
		newRepoCmd(),
		// Database queries (PostgreSQL, Scylla, Snowflake) via VPN-first
		newDbCmd(),
		// wtb daemon (HTTP over Unix socket)
		newServeCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// export — zip ~/.workflow/ → ~/wtb-export-YYYY-MM-DD.zip
func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export ~/.workflow/ to a zip archive",
		Long:  `Packages ~/.workflow/ into ~/wtb-export-YYYY-MM-DD.zip for portability.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// LGPD guardrail: warn before creating zip with personal data
			fmt.Println("⚠️  AVISO DE DADOS SENSIVEIS")
			fmt.Println("   O arquivo zip gerado contém dados pessoais de ~/.workflow/")
			fmt.Println("   (premises, sessions 1:1, security-contract).")
			fmt.Println("   Recomendacao: criptografar antes de transferir entre maquinas.")
			fmt.Println("   Referencia: ~/workflow/SECURITY.md — Regra 4")
			fmt.Println()

			src := personalDirPath()
			if _, err := os.Stat(src); os.IsNotExist(err) {
				return fmt.Errorf("~/.workflow/ nao encontrado — execute 'wtb security-accept' primeiro")
			}

			home, _ := os.UserHomeDir()
			date := time.Now().Format("2006-01-02")
			dest := filepath.Join(home, fmt.Sprintf("wtb-export-%s.zip", date))

			if err := zipDir(src, dest); err != nil {
				return fmt.Errorf("falha ao criar zip: %w", err)
			}

			fmt.Printf("✓ Exportado: %s\n", dest)
			return nil
		},
	}
}

// import — unzip and merge into ~/.workflow/
func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <arquivo.zip>",
		Short: "Import and merge a wtb export zip into ~/.workflow/",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			if _, err := os.Stat(src); os.IsNotExist(err) {
				return fmt.Errorf("arquivo nao encontrado: %s", src)
			}

			dest := personalDirPath()
			if err := os.MkdirAll(dest, 0700); err != nil {
				return fmt.Errorf("falha ao criar ~/.workflow/: %w", err)
			}

			fmt.Printf("Importando %s → %s\n", src, dest)
			if err := unzipMerge(src, dest); err != nil {
				return fmt.Errorf("falha ao importar: %w", err)
			}

			fmt.Println("✓ Import concluido")
			fmt.Println("  Premises: sobrescrito (versao importada e mais recente)")
			fmt.Println("  Sessions: adicionadas as que nao existiam localmente")
			return nil
		},
	}
}

// status — show summary of ~/.workflow/
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show summary of ~/.workflow/ state",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := personalDirPath()

			fmt.Println("=== wtb status ===")
			fmt.Println()

			// Security contract
			contract := securityContractPath()
			if info, err := os.Stat(contract); err == nil {
				fmt.Printf("Security contract: ✓ aceito em %s\n", info.ModTime().Format("2006-01-02 15:04"))
			} else {
				fmt.Println("Security contract: ✗ nao encontrado — execute 'wtb security-accept'")
			}

			// Premises
			premisesPath := filepath.Join(dir, "1on1", "premises.md")
			if _, err := os.Stat(premisesPath); err == nil {
				count := countActiveLines(premisesPath, "### P")
				fmt.Printf("Premises ativas: %d\n", count)
			} else {
				fmt.Println("Premises: nenhuma encontrada")
			}

			// Sessions
			sessionsDir := filepath.Join(dir, "1on1", "sessions")
			sessions, err := filepath.Glob(filepath.Join(sessionsDir, "*.md"))
			if err == nil && len(sessions) > 0 {
				fmt.Printf("Sessions 1:1: %d total\n", len(sessions))
				last := sessions[len(sessions)-1]
				fmt.Printf("Ultima sessao: %s\n", filepath.Base(last))
			} else {
				fmt.Println("Sessions 1:1: nenhuma encontrada")
			}

			// Workflows used (count dirs in target repos — best-effort)
			fmt.Println()
			fmt.Printf("Workflow dir: %s\n", dir)
			fmt.Printf("Platform dir: %s\n", workflowDirPath())

			return nil
		},
	}
}

// security-accept — create/update ~/.workflow/security-contract.md
func newSecurityAcceptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "security-accept",
		Short: "Accept security rules and create ~/.workflow/security-contract.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := personalDirPath()
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("falha ao criar ~/.workflow/: %w", err)
			}

			now := time.Now().Format("2006-01-02 15:04:05 -0700")
			content := fmt.Sprintf(`# Security Contract — ~/.workflow/

**Regras aceitas de:** ~/workflow/SECURITY.md
**Versao das regras:** %s
**Data de aceite:** %s
**Aceito por:** usuario

---

## Criterios aceitos

Ao executar este comando, o usuario confirma que:

1. Nao armazenara credenciais em artefatos de workflow
2. Protegera PII de clientes e colegas nos documentos
3. Sessions 1:1 sao privadas — nao transmitidas sem consentimento
4. Zips de export sao tratados como dados sensiveis
5. wtb delete-personal-data e o mecanismo de exclusao (LGPD)

---

## Adicoes pessoais

<!-- Adicionar restricoes pessoais adicionais aqui -->
<!-- Exemplo: "nunca mencionar nome do cliente X em artefatos" -->

---

*Atualizado via 'wtb security-accept' em %s*
*Versao wtb: %s*
`, securityVersion, now, now, version)

			contract := securityContractPath()
			if err := os.WriteFile(contract, []byte(content), 0600); err != nil {
				return fmt.Errorf("falha ao criar security-contract.md: %w", err)
			}

			fmt.Printf("✓ Security contract criado/atualizado: %s\n", contract)
			fmt.Printf("  Versao das regras: %s\n", securityVersion)
			fmt.Printf("  Data: %s\n", now)
			return nil
		},
	}
}

// delete-personal-data — remove all of ~/.workflow/ (LGPD right to erasure)
func newDeletePersonalDataCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-personal-data",
		Short: "Remove all personal data from ~/.workflow/ (LGPD right to erasure)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := personalDirPath()

			fmt.Println("⚠️  ATENCAO: Esta operacao e IRREVERSIVEL.")
			fmt.Printf("   Todo o conteudo de %s sera removido.\n", dir)
			fmt.Println("   Isso inclui: premises, sessions 1:1, security-contract.")
			fmt.Println()
			fmt.Print("Digite 'CONFIRMAR' para prosseguir: ")

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("falha ao ler confirmacao: %w", err)
			}

			if strings.TrimSpace(input) != "CONFIRMAR" {
				fmt.Println("Operacao cancelada.")
				return nil
			}

			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("falha ao remover dados: %w", err)
			}

			fmt.Println("✓ Dados pessoais removidos com sucesso.")
			fmt.Println("  Direito de exclusao LGPD exercido.")
			return nil
		},
	}
}

// update — check for new version and offer to update
func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for wtb updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			wfDir := workflowDirPath()
			latestFile := filepath.Join(wfDir, "releases", "LATEST")

			current := version

			latest, err := os.ReadFile(latestFile)
			if err != nil {
				fmt.Printf("Versao atual: %s\n", current)
				fmt.Printf("Nao foi possivel verificar atualizacoes (releases/LATEST nao encontrado: %v)\n", err)
				fmt.Println("Execute 'make build' para compilar a versao mais recente.")
				return nil
			}

			latestStr := strings.TrimSpace(string(latest))
			fmt.Printf("Versao atual: %s\n", current)
			fmt.Printf("Versao disponivel: %s\n", latestStr)

			if current == latestStr {
				fmt.Println("✓ Ja esta na versao mais recente.")
				return nil
			}

			fmt.Println()
			fmt.Printf("Nova versao disponivel: %s → %s\n", current, latestStr)
			fmt.Println("Para atualizar: cd ~/workflow && make build")

			return nil
		},
	}
}

// --- scaffolding commands ---

// new — scaffold a new workflow artefact from template
func newNewCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "new <type> <context>",
		Short: "Scaffold a new workflow artefact from template",
		Long: `Creates a new workflow artefact under <repo>/docs/workflow/<type>/ using
the canonical template from ~/workflow/use-cases/<type>/template.md.

Examples:
  wtb new incident kafka-lag --repo ~/Cobliteam/fusca
  wtb new postmortem db-deadlock
  wtb new 1on1 post-sprint`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			wfType := args[0]
			context := args[1]

			if !scaffold.IsValidType(wfType) {
				return fmt.Errorf("tipo invalido %q — validos: %s", wfType, strings.Join(scaffold.WorkflowTypes, ", "))
			}

			// Resolve repo
			targetRepo := repo
			if targetRepo == "" {
				var err error
				targetRepo, err = repoRoot()
				if err != nil {
					return fmt.Errorf("falha ao obter diretorio atual: %w", err)
				}
			}
			targetRepo = expandHome(targetRepo)

			// Determine next NNN by counting existing artefacts
			typeDir := filepath.Join(targetRepo, "docs", "workflow", wfType)
			nnn, err := scaffold.NextNNN(typeDir)
			if err != nil {
				return fmt.Errorf("falha ao determinar proximo NNN: %w", err)
			}

			date := time.Now().Format("2006-01-02")
			// 1on1 artefacts go to ~/.workflow/1on1/sessions/, not the repo
			var artefactPath string
			if wfType == "1on1" {
				sessionsDir := filepath.Join(personalDirPath(), "1on1", "sessions")
				if err := os.MkdirAll(sessionsDir, 0700); err != nil {
					return fmt.Errorf("falha ao criar sessions dir: %w", err)
				}
				artefactPath = filepath.Join(sessionsDir, fmt.Sprintf("%s-%s.md", nnn, date))
			} else {
				if err := os.MkdirAll(typeDir, 0755); err != nil {
					return fmt.Errorf("falha ao criar dir %s: %w", typeDir, err)
				}
				name := fmt.Sprintf("%s-%s-%s.md", nnn, context, date)
				artefactPath = filepath.Join(typeDir, name)
			}

			// Load template
			templatePath := filepath.Join(workflowDirPath(), "use-cases", wfType, "template.md")
			tmplContent, err := os.ReadFile(templatePath)
			if err != nil {
				return fmt.Errorf("template nao encontrado em %s: %w", templatePath, err)
			}

			// Render template with basic substitutions
			rendered, err := scaffold.RenderTemplate(string(tmplContent), map[string]string{
				"NNN":     nnn,
				"Date":    date,
				"Context": context,
				"Type":    wfType,
			})
			if err != nil {
				return fmt.Errorf("falha ao renderizar template: %w", err)
			}

			if _, err := os.Stat(artefactPath); err == nil {
				return fmt.Errorf("artefato ja existe: %s", artefactPath)
			}

			if err := os.WriteFile(artefactPath, []byte(rendered), 0644); err != nil {
				return fmt.Errorf("falha ao criar artefato: %w", err)
			}

			fmt.Printf("✓ Artefato criado: %s\n", artefactPath)
			fmt.Printf("  Tipo: %s | Context: %s | ID: %s\n", wfType, context, nnn)
			if wfType != "1on1" {
				fmt.Printf("  Proximo passo: wtb index --repo %s\n", targetRepo)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "path do repositorio alvo (default: diretorio atual)")
	return cmd
}

// index — rebuild INDEX.md files in a repo's docs/workflow/
func newIndexCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Rebuild INDEX.md files in <repo>/docs/workflow/",
		Long: `Scans all artefacts under <repo>/docs/workflow/ and rebuilds INDEX.md
for each type and the master INDEX.md.

Example:
  wtb index --repo ~/Cobliteam/fusca`,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetRepo := repo
			if targetRepo == "" {
				var err error
				targetRepo, err = repoRoot()
				if err != nil {
					return fmt.Errorf("falha ao obter diretorio atual: %w", err)
				}
			}
			targetRepo = expandHome(targetRepo)

			workflowRoot := filepath.Join(targetRepo, "docs", "workflow")
			if _, err := os.Stat(workflowRoot); os.IsNotExist(err) {
				return fmt.Errorf("docs/workflow/ nao encontrado em %s", targetRepo)
			}

			presentTypes := []string{}
			for _, wfType := range scaffold.WorkflowTypes {
				typeDir := filepath.Join(workflowRoot, wfType)
				if _, err := os.Stat(typeDir); os.IsNotExist(err) {
					continue
				}
				if err := scaffold.RebuildTypeIndex(typeDir, wfType); err != nil {
					fmt.Printf("  ✗ %s: %v\n", wfType, err)
					continue
				}
				presentTypes = append(presentTypes, wfType)
				fmt.Printf("  ✓ %s/INDEX.md atualizado\n", wfType)
			}

			if err := scaffold.RebuildMasterIndex(workflowRoot, presentTypes); err != nil {
				return fmt.Errorf("falha ao atualizar INDEX.md mestre: %w", err)
			}
			fmt.Println("  ✓ INDEX.md mestre atualizado")
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "path do repositorio alvo (default: diretorio atual)")
	return cmd
}

// list — list workflow artefacts in a repo
func newListCmd() *cobra.Command {
	var repo string
	var wfType string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflow artefacts in <repo>/docs/workflow/",
		Long: `Lists all workflow artefacts found in <repo>/docs/workflow/.

Examples:
  wtb list --repo ~/Cobliteam/fusca
  wtb list --repo ~/Cobliteam/fusca --type postmortem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetRepo := repo
			if targetRepo == "" {
				var err error
				targetRepo, err = repoRoot()
				if err != nil {
					return fmt.Errorf("falha ao obter diretorio atual: %w", err)
				}
			}
			targetRepo = expandHome(targetRepo)

			workflowRoot := filepath.Join(targetRepo, "docs", "workflow")
			if _, err := os.Stat(workflowRoot); os.IsNotExist(err) {
				return fmt.Errorf("docs/workflow/ nao encontrado em %s", targetRepo)
			}

			typesToList := scaffold.WorkflowTypes
			if wfType != "" {
				if !scaffold.IsValidType(wfType) {
					return fmt.Errorf("tipo invalido %q", wfType)
				}
				typesToList = []string{wfType}
			}

			fmt.Printf("Artefatos em %s/docs/workflow/\n\n", filepath.Base(targetRepo))
			total := 0
			for _, t := range typesToList {
				typeDir := filepath.Join(workflowRoot, t)
				entries, err := scaffold.ListArtefacts(typeDir)
				if err != nil || len(entries) == 0 {
					continue
				}
				fmt.Printf("## %s (%d)\n", t, len(entries))
				for _, e := range entries {
					fmt.Printf("  %s\n", e)
				}
				fmt.Println()
				total += len(entries)
			}
			fmt.Printf("Total: %d artefato(s)\n", total)
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "path do repositorio alvo (default: diretorio atual)")
	cmd.Flags().StringVar(&wfType, "type", "", "filtrar por tipo (incident, postmortem, review, 1on1)")
	return cmd
}

// --- helpers ---

func zipDir(src, dest string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(filepath.Dir(src), path)
		if err != nil {
			return err
		}

		fw, err := w.Create(rel)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(fw, file)
		return err
	})
}

func unzipMerge(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(dest, filepath.Base(filepath.Dir(f.Name)), filepath.Base(f.Name))

		// sessions: only add new (never overwrite)
		if strings.Contains(f.Name, "sessions/") {
			if _, err := os.Stat(target); err == nil {
				continue // already exists — skip
			}
		}

		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func countActiveLines(path, prefix string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

// ContainsCredential checks if text contains credential patterns (used by security guardrail tests)
func ContainsCredential(text string) bool {
	for _, re := range credentialPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// expandHome expands ~ prefix in path
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// run — execute a pipeline use-case via YAML runner engine
func newRunCmd() *cobra.Command {
	var (
		flagDryRun       bool
		flagSkipOptional bool
		flagRepo         string
		flagInputs       []string // repeatable: --input key=value --input key2=value2
		flagProvider     string
		flagEnv          string
	)

	cmd := &cobra.Command{
		Use:   "run <use-case>",
		Short: "Execute a pipeline use-case",
		Long: `Execute a pipeline use-case defined in use-cases/<type>/definition.yml.

--input accepts key=value pairs and can be repeated for multiple inputs.

Examples:
  wtb run backlog --input narrative=meeting-notes.md --dry-run
  wtb run ops-response --env production --input symptom="CDC lag"
  wtb run investigation --input playbook=fusca-cdc-audit --env production
  wtb run ops-response --input kubectl-context=prod --input namespace=fusca --input deployment=fusca-api
  wtb run backlog --skip-optional --provider gemini`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			useCaseID := args[0]
			home := workflowDirPath()

			// Load definition
			def, err := runner.LoadDefinition(home, useCaseID)
			if err != nil {
				// Show available use-cases to help the user
				ids, listErr := runner.ListUseCases(home)
				if listErr == nil && len(ids) > 0 {
					fmt.Fprintf(os.Stderr, "Available use-cases: %s\n", strings.Join(ids, ", "))
				}
				return err
			}

			// Build inputs: parse all --input key=value pairs.
			inputs := runner.RunInputs{}
			for _, inp := range flagInputs {
				if strings.Contains(inp, "=") {
					parts := strings.SplitN(inp, "=", 2)
					inputs[parts[0]] = parts[1]
				} else {
					// Bare value without key → treat as narrative file path.
					inputs["narrative"] = inp
				}
			}
			// Convenience flags override matching inputs.
			if flagProvider != "" {
				inputs["provider"] = flagProvider
			}
			if flagEnv != "" {
				inputs["environment"] = flagEnv
			}

			// Resolve repo path
			repoPath := flagRepo
			if repoPath == "" {
				repoPath, _ = repoRoot()
			}

			opts := runner.RunOptions{
				DryRun:       flagDryRun,
				AutoSkip:     flagSkipOptional,
				WorkflowHome: home,
				RepoPath:     repoPath,
			}

			r := runner.New(def, runner.DefaultRegistry(), opts)

			// Wire YAML-driven Socratic input resolution
			if resolveCfg, err := runner.LoadResolveConfig(repoPath); err == nil {
				r.WithResolvers(runner.DefaultResolverRegistry(resolveCfg))
			}

			if flagDryRun {
				fmt.Printf("=== dry-run: %s (%s) ===\n", def.Name, def.ID)
				fmt.Printf("steps: %d | type: %s\n\n", len(def.Steps), def.Type)
			} else {
				fmt.Printf("=== wtb run %s ===\n", def.ID)
			}

			results, err := r.Run(inputs)
			if err != nil {
				return err
			}

			// Summary
			executed, skipped := 0, 0
			for _, res := range results {
				if res.Skipped {
					skipped++
				} else {
					executed++
				}
			}
			fmt.Printf("\n✓ %s complete — %d steps executed, %d skipped\n", def.ID, executed, skipped)

			// Chain traversal: offer next steps declared in chain.to.
			return chain.FollowChain(def, inputs, opts, nil, chain.NewTerminalChainIO(os.Stdout))
		},
	}

	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what would execute without running")
	cmd.Flags().BoolVar(&flagSkipOptional, "skip-optional", false, "Automatically skip optional steps")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "Target repo path (default: current directory)")
	cmd.Flags().StringArrayVar(&flagInputs, "input", nil, "Input key=value pair (repeatable: --input a=x --input b=y)")
	cmd.Flags().StringVar(&flagProvider, "provider", "", "LLM provider (claude|chatgpt|gemini|ollama|azure)")
	cmd.Flags().StringVar(&flagEnv, "env", "", "Environment (production|staging|namespace)")

	return cmd
}

// chain next — follow chain.to from a documentary use-case
func newChainCmd() *cobra.Command {
	var (
		flagInputs []string
		flagRepo   string
	)
	cmd := &cobra.Command{
		Use:   "chain next <use-case-id>",
		Short: "Follow chain.to from a completed documentary use-case",
		Long: `Presents the chain.to options declared in the use-case definition and
executes the chosen next use-case with the provided inputs.

Use this for documentary use-cases (incident, postmortem, discovery) that
have no executable steps — the chain is triggered explicitly when you're
ready to hand off to the next workflow.

Examples:
  wtb chain next incident --input symptom="CDC lag" --input environment=production
  wtb chain next postmortem
  wtb chain next discovery --input topic=auth-refactor`,
		Args: cobra.ExactArgs(2), // "next" + use-case-id
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "next" {
				return fmt.Errorf("unknown subcommand %q — use: wtb chain next <use-case-id>", args[0])
			}
			useCaseID := args[1]
			home := workflowDirPath()

			def, err := runner.LoadDefinition(home, useCaseID)
			if err != nil {
				return err
			}

			inputs := runner.RunInputs{}
			for _, inp := range flagInputs {
				if strings.Contains(inp, "=") {
					parts := strings.SplitN(inp, "=", 2)
					inputs[parts[0]] = parts[1]
				}
			}

			repoPath := flagRepo
			if repoPath == "" {
				repoPath, _ = repoRoot()
			}

			opts := runner.RunOptions{
				WorkflowHome: home,
				RepoPath:     repoPath,
			}

			if len(def.Chain.To) == 0 {
				fmt.Printf("use-case %q has no chain.to entries — nothing to follow.\n", useCaseID)
				return nil
			}

			return chain.FollowChain(def, inputs, opts, nil, chain.NewTerminalChainIO(os.Stdout))
		},
	}
	cmd.Flags().StringArrayVar(&flagInputs, "input", nil, "Input key=value pair (repeatable)")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "Target repo path (default: current directory)")
	return cmd
}

// mcp-serve — run the MCP server in stdio mode
func newMCPServeCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "mcp-serve",
		Short: "Start the workflow MCP server (stdio transport)",
		Long: `Starts the workflow MCP server using stdio transport (standard MCP protocol).

Exposes 15 tools covering the full workflow platform:
  - workflow_run, workflow_list_use_cases
  - workflow_new, workflow_index, workflow_list
  - ops_probe, ops_db_health, ops_k8s_status, ops_kafka_status, ops_logs_analyze
  - ops_plan_new, ops_plan_show
  - playbook_run, playbook_list
  - workflow_status

Configure in Claude Desktop / Claude Code:
  {
    "mcpServers": {
      "workflow": {
        "command": "~/workflow/bin/wtb",
        "args": ["mcp-serve"]
      }
    }
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home := workflowDirPath()
			repoPath := repo
			if repoPath == "" {
				var err error
				repoPath, err = repoRoot()
				if err != nil {
					repoPath = home
				}
			}
			repoPath = expandHome(repoPath)

			s := mcp.NewServer(home, repoPath)
			return mcp.Start(s)
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "Default target repo path (default: current directory)")
	return cmd
}
