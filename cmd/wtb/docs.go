// docs — wtb docs chain
// Generates a Mermaid flowchart from use-cases/*/definition.yml.
// Source of truth: the YAML files. No manual maintenance needed.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cobliteam/workflow-toolkit/pkg/docs"
	"github.com/spf13/cobra"
)

func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate documentation artifacts from use-case definitions",
	}
	cmd.AddCommand(newDocsChainCmd())
	return cmd
}

func newDocsChainCmd() *cobra.Command {
	var output string
	var repo string

	cmd := &cobra.Command{
		Use:   "chain",
		Short: "Generate Mermaid chain diagram from use-cases/*/definition.yml",
		Long: `Reads all use-cases/*/definition.yml and produces a Mermaid flowchart
showing the full incident → ops-response → investigation → postmortem → review → 1:1 chain.

Source of truth: the definition.yml files. Run after adding or changing a use-case.

Examples:
  wtb docs chain                          # print to stdout
  wtb docs chain --output docs/chain.md  # write to file
  wtb docs chain --repo ~/Cobliteam/fusca`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := expandHome(repo)
			if repoPath == "" {
				var err error
				repoPath, err = repoRoot()
				if err != nil {
					return err
				}
			}

			defs, err := docs.LoadUseCases(repoPath)
			if err != nil {
				return fmt.Errorf("loading use-cases: %w", err)
			}
			if len(defs) == 0 {
				return fmt.Errorf("no use-cases found in %s/use-cases/", repoPath)
			}

			mermaid := docs.GenerateMermaid(defs)
			block := "```mermaid\n" + mermaid + "```\n"

			if output == "" {
				fmt.Print(block)
				return nil
			}

			outPath := expandHome(output)
			if !filepath.IsAbs(outPath) {
				outPath = filepath.Join(repoPath, outPath)
			}
			if err := os.WriteFile(outPath, []byte(block), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}
			fmt.Printf("✓ wrote %s (%d use-cases)\n", outPath, len(defs))
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "", "output file path (default: stdout)")
	cmd.Flags().StringVar(&repo, "repo", workflowDir, "repo root containing use-cases/")
	return cmd
}
