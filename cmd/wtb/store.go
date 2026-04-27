package main

import (
	"fmt"
	"os"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/store"
	"github.com/spf13/cobra"
)

func newStoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store <subcommand>",
		Short: "Query and analyze the ops result log (~/.workflow/ops-log.db)",
		Long: `Queries the SQLite OpsResult log written by 'wtb ops <probe> --store'.

Discovery 011 — Camada 1: second-order heuristics over accumulated history.
Discovery 011 — Camada 2: LLM analysis of history → suggested heuristic rules.

Examples:
  wtb store trend db-health
  wtb store trend db-health --last 14
  wtb store tail
  wtb store tail --last 20
  wtb store rules
  wtb store analyze --provider claude
  WTB_MOCK_LLM=1 wtb store analyze`,
	}
	cmd.AddCommand(newStoreTrendCmd(), newStoreTailCmd(), newStoreRulesCmd(), newStoreAnalyzeCmd())
	return cmd
}

// ── trend ──────────────────────────────────────────────────────────────────

func newStoreTrendCmd() *cobra.Command {
	var last int
	var storePath string
	cmd := &cobra.Command{
		Use:   "trend <probe>",
		Short: "Show recent executions for a probe + fire trend heuristics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			probe := args[0]
			if storePath == "" {
				storePath = store.DefaultPath()
			}

			records := store.QueryTrend(storePath, probe, last)
			if len(records) == 0 {
				fmt.Printf("No records found for probe %q in %s\n", probe, storePath)
				fmt.Println("Run 'wtb ops <probe> --store' to start logging.")
				return nil
			}

			fmt.Printf("=== trend: %s (last %d) ===\n", probe, len(records))
			for _, r := range records {
				icon := statusIcon(r.Status)
				fmt.Printf("  %s  %-8s  %s  %s\n", r.Ts[:16], icon+r.Status, r.Repo, r.Signal)
			}

			// Evaluate second-order heuristics
			signals, err := store.TrendSignal(storePath)
			if err == nil && len(signals) > 0 {
				fmt.Println()
				fmt.Println("── heurísticas de segunda ordem ──")
				for _, s := range signals {
					fmt.Printf("  ⚠  %s\n", s)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&last, "last", 10, "Number of recent executions to show")
	cmd.Flags().StringVar(&storePath, "path", "", "Path to ops-log.db (default: ~/.workflow/ops-log.db)")
	return cmd
}

// ── tail ───────────────────────────────────────────────────────────────────

func newStoreTailCmd() *cobra.Command {
	var last int
	var storePath string
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Show the last N entries across all probes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if storePath == "" {
				storePath = store.DefaultPath()
			}
			records := store.QueryTrend(storePath, "", last)
			if len(records) == 0 {
				fmt.Printf("No records in %s\n", storePath)
				fmt.Println("Run 'wtb ops <probe> --store' to start logging.")
				return nil
			}
			fmt.Printf("=== ops log tail (last %d) ===\n", len(records))
			for _, r := range records {
				icon := statusIcon(r.Status)
				fmt.Printf("  %s  %-14s  %s  %s\n", r.Ts[:16], icon+r.Status, r.Probe, r.Signal)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&last, "last", 20, "Number of recent entries to show")
	cmd.Flags().StringVar(&storePath, "path", "", "Path to ops-log.db (default: ~/.workflow/ops-log.db)")
	return cmd
}

// ── analyze ────────────────────────────────────────────────────────────────

func newStoreAnalyzeCmd() *cobra.Command {
	var storePath, providerName string
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "LLM analysis of ops history → suggested heuristic rules",
		Long: `Sends the last 200 ops log entries to the LLM and returns:
  - Observed patterns (probe + observation + confidence)
  - Suggested trend_rules in store_rules.yml format + rationale

Use WTB_MOCK_LLM=1 for zero-cost dev feedback loop.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if storePath == "" {
				storePath = store.DefaultPath()
			}
			provider, err := llm.NewProvider(llm.ProviderConfig{Provider: providerName})
			if err != nil {
				return fmt.Errorf("provider %q: %w", providerName, err)
			}

			fmt.Printf("🔍 Store Analysis — %s\n", storePath)
			fmt.Printf("   provider: %s\n\n", provider.ProviderName())

			result, err := store.AnalyzeHistory(storePath, provider)
			if err != nil {
				return fmt.Errorf("analysis failed: %w", err)
			}

			if result.DataPoints == 0 && len(result.Patterns) == 0 {
				fmt.Println("No records found. Run 'wtb ops <probe> --store' to start logging.")
				return nil
			}

			if len(result.Patterns) > 0 {
				fmt.Printf("── patterns detected (%d) ──\n", len(result.Patterns))
				for _, p := range result.Patterns {
					fmt.Printf("  %-20s  %.0f%%  %s\n", p.Probe, p.Confidence*100, p.Observation)
				}
				fmt.Println()
			}

			if len(result.SuggestedRules) > 0 {
				fmt.Printf("── suggested rules (%d) ──\n", len(result.SuggestedRules))
				for _, r := range result.SuggestedRules {
					fmt.Printf("  probe:  %s\n", r.Probe)
					fmt.Printf("  window: %d consecutive %s → %s\n", r.Window, r.ConsecutiveStatus, r.EscalateTo)
					fmt.Printf("  signal: %s\n", r.Signal)
					if r.Rationale != "" {
						fmt.Printf("  why:    %s\n", r.Rationale)
					}
					fmt.Println()
				}
			}

			fmt.Printf("confidence: %.0f%%  |  data points: %d  |  time range: %s\n",
				result.Confidence*100, result.DataPoints, result.TimeRange)
			return nil
		},
	}
	cmd.Flags().StringVar(&storePath, "path", "", "Path to ops-log.db (default: ~/.workflow/ops-log.db)")
	cmd.Flags().StringVar(&providerName, "provider", "claude", "LLM provider (claude, gemini, mock)")
	return cmd
}

// ── rules ──────────────────────────────────────────────────────────────────

func newStoreRulesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rules",
		Short: "List active second-order heuristic rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := store.LoadStoreConfig()
			if err != nil {
				return fmt.Errorf("failed to load store rules: %w", err)
			}
			fmt.Printf("=== store rules (%d) ===\n\n", len(cfg.TrendRules))
			for _, r := range cfg.TrendRules {
				fmt.Printf("  probe:  %s\n", r.Probe)
				fmt.Printf("  window: %d consecutive %s → escalate to %s\n", r.Window, r.ConsecutiveStatus, r.EscalateTo)
				fmt.Printf("  signal: %s\n\n", r.Signal)
			}
			return nil
		},
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func statusIcon(status string) string {
	switch status {
	case "ok":
		return "✓ "
	case "warn":
		return "⚠ "
	case "critical", "error":
		return "✗ "
	default:
		return "• "
	}
}

// storeAppend writes to the JSONL log if storePath is non-empty.
// Called by opsPrintStore in ops.go.
func storeAppend(storePath, probe, status, signal, repo string) {
	if storePath == "" {
		return
	}
	if err := store.Append(storePath, probe, status, signal, repo); err != nil {
		fmt.Fprintf(os.Stderr, "  [store] %v\n", err)
	}
}
