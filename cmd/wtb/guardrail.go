// guardrail — wtb guardrail [setup-hooks]
// Drift containment for platform premises. Not an evolution blocker.
//
// Four structural checks:
//   chain-diagram-sync      README Mermaid diverges from use-cases/*/definition.yml
//   zero-llm-ops            LLM import in pkg/ops/ (zero-LLM probe contract)
//   usecase-definition      use-cases/ directory missing definition.yml
//   anonymization-in-docs   PII/infra identifiers exposed in docs/ (Heuristic-First)
//
// Exit code 0 = all passed. Exit code 1 = at least one failed (CI-compatible).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/doccheck"
	"github.com/spf13/cobra"
)

func newGuardrailCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:          "guardrail",
		Short:        "Detect framework drift — drift containment, not evolution blocker",
		SilenceUsage: true,
		Long: `Runs four structural checks to detect unintentional drift from platform premises.

  chain-diagram-sync      README Mermaid diverges from use-cases/*/definition.yml
  zero-llm-ops            LLM import in pkg/ops/ (zero-LLM probe contract)
  usecase-definition      use-cases/ directory missing definition.yml
  anonymization-in-docs   PII/infra identifiers exposed in docs/ (Heuristic-First, zero-LLM)

Intentional evolution passes through with an explicit override comment in the file:
  // wtb-noguard: <check> — <justificativa>

Exit code 0 = all passed. Exit code 1 = at least one check failed (CI-compatible).

Examples:
  wtb guardrail                          # check current directory
  wtb guardrail --repo ~/workflow        # check specific repo
  wtb guardrail setup-hooks              # install as git pre-commit hook`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := expandHome(repo)
			if repoPath == "" {
				var err error
				repoPath, err = repoRoot()
				if err != nil {
					return err
				}
			}

			results := doccheck.RunAll(repoPath)

			fmt.Printf("\n🔍 wtb guardrail — %s\n", repoPath)
			fmt.Println(strings.Repeat("─", 52))

			failed := 0
			for _, r := range results {
				if r.Passed {
					fmt.Printf("  ✓ %s\n", r.Check)
				} else {
					fmt.Print(r.Detail)
					if r.Fix != "" {
						fmt.Printf("  fix: %s\n", r.Fix)
					}
					failed++
				}
			}

			fmt.Println()
			if failed == 0 {
				fmt.Printf("✓ %d/%d checks passed — nenhum drift detectado.\n\n", len(results), len(results))
				return nil
			}
			fmt.Printf("✗ %d/%d checks falharam.\n\n", failed, len(results))
			os.Exit(1)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", ".", "repo root to check (default: current directory)")
	cmd.AddCommand(newGuardrailSetupHooksCmd())
	return cmd
}

func newGuardrailSetupHooksCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:          "setup-hooks",
		Short:        "Install wtb guardrail as a git pre-commit hook",
		SilenceUsage: true,
		Long: `Installs .git/hooks/pre-commit that runs wtb guardrail before each commit.

The hook explains why it exists and how to use overrides for intentional evolution.
To uninstall: rm .git/hooks/pre-commit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := expandHome(repo)
			if repoPath == "" {
				var err error
				repoPath, err = repoRoot()
				if err != nil {
					return err
				}
			}

			hookDir := filepath.Join(repoPath, ".git", "hooks")
			if err := os.MkdirAll(hookDir, 0755); err != nil {
				return fmt.Errorf("creating hooks dir: %w", err)
			}

			hookPath := filepath.Join(hookDir, "pre-commit")
			if err := os.WriteFile(hookPath, []byte(preCommitHookContent()), 0755); err != nil {
				return fmt.Errorf("writing hook: %w", err)
			}

			fmt.Printf("✓ Hook instalado em %s\n", hookPath)
			fmt.Println()
			fmt.Println("  Roda wtb guardrail antes de cada commit.")
			fmt.Println("  Detecta drift — não bloqueia evolução intencional.")
			fmt.Println("  Para desinstalar: rm " + hookPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", ".", "repo root (default: current directory)")
	return cmd
}

func preCommitHookContent() string {
	return `#!/bin/sh
# wtb guardrail — pre-commit hook
#
# Detecta drift das premissas do framework antes do commit.
# Não bloqueia evolução intencional — contém drift não intencional.
#
# Checks:
#   chain-diagram-sync      diagrama Mermaid diverge dos use-cases
#   zero-llm-ops            import LLM em pkg/ops/ (viola zero-LLM)
#   usecase-definition      use-case sem definition.yml
#   anonymization-in-docs   PII/identificadores de infra expostos em docs/
#
# Para override (evolução intencional), adicione no arquivo:
#   // wtb-noguard: <check> — <justificativa da decisão>
#
# Para desinstalar:
#   rm .git/hooks/pre-commit

REPO_ROOT="$(git rev-parse --show-toplevel)"

WTB_BIN="$(command -v wtb 2>/dev/null)"
if [ -z "$WTB_BIN" ]; then
  WTB_BIN="$HOME/workflow/bin/wtb"
fi
if [ ! -x "$WTB_BIN" ]; then
  echo "⚠  wtb não encontrado — hook ignorado."
  echo "   Instale em ~/workflow/bin ou adicione ao PATH."
  exit 0
fi

exec "$WTB_BIN" guardrail --repo "$REPO_ROOT"
`
}
