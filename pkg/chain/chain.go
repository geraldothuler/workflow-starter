// Package chain implements recursive chain traversal between use-cases.
// After a use-case completes, FollowChain reads chain.to from the definition,
// presents the options via ChainIO, and executes the chosen next use-case
// with all inputs inherited from the parent run.
package chain

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/runner"
)

// ChainIO abstracts how chain options are presented and a choice is received.
// CLI uses terminal prompts; headless uses auto-follow logic.
type ChainIO interface {
	ProposeNext(completedID string, options []string) (chosen string, ok bool)
}

// FollowChain is called after a use-case completes successfully.
// It presents chain.to options and recursively follows the chosen path
// until the user declines or chain.to is empty.
// All inputs from the completed run carry over to every chained use-case.
//
// term overrides the Runner's TerminalIO for chained runs (pass nil for default).
func FollowChain(completed *runner.UseCaseDefinition, inputs runner.RunInputs, opts runner.RunOptions, term runner.TerminalIO, cio ChainIO) error {
	def := completed

	for {
		if len(def.Chain.To) == 0 {
			return nil
		}

		chosen, ok := cio.ProposeNext(def.ID, def.Chain.To)
		if !ok {
			return nil
		}

		nextDef, err := runner.LoadDefinition(opts.WorkflowHome, chosen)
		if err != nil {
			return fmt.Errorf("chain: cannot load %q: %w", chosen, err)
		}

		if nextDef.IsPipeline() {
			if term == nil {
				fmt.Printf("\n=== chain → wtb run %s ===\n", nextDef.ID)
			}
			r := runner.New(nextDef, runner.DefaultRegistry(), opts)
			if term != nil {
				r.WithTerminal(term)
			} else {
				if cfg, err := runner.LoadResolveConfig(opts.RepoPath); err == nil {
					r.WithResolvers(runner.DefaultResolverRegistry(cfg))
				}
			}
			results, err := r.Run(inputs)
			if err != nil {
				return fmt.Errorf("chain %q: %w", nextDef.ID, err)
			}
			executed, skipped := 0, 0
			for _, res := range results {
				if res.Skipped {
					skipped++
				} else {
					executed++
				}
			}
			if term == nil {
				fmt.Printf("\n✓ %s complete — %d steps executed, %d skipped\n", nextDef.ID, executed, skipped)
			}
		} else {
			// Documentary use-case: no steps to run, but acknowledge the chain link.
			if term == nil {
				fmt.Printf("\n→ chain: %s é um use-case documental (type: %s). Crie o artefato com: wtb new %s <context>\n",
					nextDef.ID, nextDef.Type, nextDef.ID)
			}
		}

		def = nextDef
	}
}

// ── Interactive (CLI) ─────────────────────────────────────────────────────────

// TerminalChainIO prompts the user interactively via stdin/stdout.
type TerminalChainIO struct {
	out io.Writer
}

// NewTerminalChainIO creates a TerminalChainIO that writes prompts to w.
func NewTerminalChainIO(w io.Writer) *TerminalChainIO {
	return &TerminalChainIO{out: w}
}

func (t *TerminalChainIO) ProposeNext(completedID string, options []string) (string, bool) {
	fmt.Fprintf(t.out, "\n→ Próximo passo na chain de %q:\n", completedID)
	for i, opt := range options {
		fmt.Fprintf(t.out, "  [%d] %s\n", i+1, opt)
	}
	fmt.Fprintf(t.out, "  [0] Encerrar aqui\n")
	fmt.Fprintf(t.out, "  Escolha: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", false
	}
	line := strings.TrimSpace(scanner.Text())

	n, err := strconv.Atoi(line)
	if err != nil || n <= 0 || n > len(options) {
		return "", false
	}
	return options[n-1], true
}

// ── Headless (webhook) ────────────────────────────────────────────────────────

// HeadlessChainIO auto-follows if there is exactly one option; stops if multiple.
type HeadlessChainIO struct {
	logger *log.Logger
}

// NewHeadlessChainIO creates a HeadlessChainIO that logs decisions to logger.
func NewHeadlessChainIO(logger *log.Logger) *HeadlessChainIO {
	return &HeadlessChainIO{logger: logger}
}

func (h *HeadlessChainIO) ProposeNext(completedID string, options []string) (string, bool) {
	if len(options) == 1 {
		h.logger.Printf("[chain] %q → auto-following %q (single option)", completedID, options[0])
		return options[0], true
	}
	h.logger.Printf("[chain] %q → stopping (multiple options, headless cannot choose): %v", completedID, options)
	return "", false
}

// ── MCP ───────────────────────────────────────────────────────────────────────

// FormatOptions returns a human-readable string listing chain options,
// suitable for embedding in MCP tool responses.
func FormatOptions(def *runner.UseCaseDefinition) string {
	if len(def.Chain.To) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\nchain_options:")
	for _, opt := range def.Chain.To {
		sb.WriteString("\n  - ")
		sb.WriteString(opt)
	}
	return sb.String()
}
