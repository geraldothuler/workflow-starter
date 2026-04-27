// memory — wtb memory <subcommand>
//
// Single entry point for all LLM memory operations.
// Eliminates hallucination on recovery and rediscovery on structured data.
//
// Subcommands:
//
//	get <topic>                  retrieve topic file content
//	where "<description>"        routing heuristic — where to store a piece of knowledge
//	set <key> <value>            store a structured fact in docs.db
//	append --topic <t> <text>    append narrative content to a topic file
//	add-rule <one-liner>         append one-liner to MEMORY.md § Regras de processo
//	list [--topic <t>]           list structured facts
//	validate                     run memory guardrails (bloat + content-leak)
//	migrate                      import context.json entries into docs.db
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/doccheck"
	"github.com/Cobliteam/workflow-toolkit/pkg/memory"
	"github.com/spf13/cobra"
)

func newMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "memory",
		Short:        "Context management — get, store, route and validate LLM memory",
		SilenceUsage: true,
		Long: `Single entry point for all LLM memory operations.

Three storage layers:
  Keychain       — credentials and operational IDs (security find-generic-password)
  docs.db        — structured facts: thresholds, limits, config values (type=config)
  topic files    — narrative: runbooks, heuristics, architecture notes

Subcommands:
  get <topic>              print topic file content
  where "<description>"    routing heuristic: where should this knowledge live?
  set <key> <value>        store a structured fact in docs.db
  list [--topic <t>]       list structured facts
  migrate                  import context.json entries into docs.db
  validate                 run memory guardrails`,
	}

	cmd.AddCommand(
		newMemoryGetCmd(),
		newMemoryWhereCmd(),
		newMemorySetCmd(),
		newMemoryListCmd(),
		newMemoryValidateCmd(),
		newMemoryAppendCmd(),
		newMemoryAddRuleCmd(),
		newMemoryMigrateCmd(),
	)
	return cmd
}

// ── get ──────────────────────────────────────────────────────────────────

func newMemoryGetCmd() *cobra.Command {
	var (
		repo    string
		section string
		grep    string
		limit   int
	)

	cmd := &cobra.Command{
		Use:          "get <topic>",
		Short:        "Print the content of a topic file",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Example: `  wtb memory get webhook                        # full file
  wtb memory get heuristics --section "## Flink" # only Flink section
  wtb memory get k8s --grep "cerberus"           # lines matching keyword
  wtb memory get feedback_code_review --limit 40 # first 40 lines`,
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := args[0]

			if repo == "" {
				var err error
				repo, err = repoRoot()
				if err != nil {
					return err
				}
			}

			tm, err := memory.LoadTopicMap(repo)
			if err != nil {
				return fmt.Errorf("topic map: %w", err)
			}

			path, err := tm.Resolve(repo, topic)
			if err != nil {
				topics := tm.List()
				sort.Strings(topics)
				return fmt.Errorf("%w\n\nKnown topics:\n  %s", err, strings.Join(topics, "\n  "))
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}

			content := string(data)

			// --section: extract only the matching section
			if section != "" {
				content = memory.ExtractSection(content, section)
			}

			// --grep: filter to matching lines (with 1 line of context)
			if grep != "" {
				content = memory.GrepLines(content, grep)
			}

			// --limit: cap output lines
			if limit > 0 {
				lines := strings.Split(content, "\n")
				if len(lines) > limit {
					lines = lines[:limit]
					lines = append(lines, fmt.Sprintf("… (%d lines truncated, use --limit N to see more)", len(strings.Split(content, "\n"))-limit))
				}
				content = strings.Join(lines, "\n")
			}

			fmt.Printf("# %s — %s\n\n", topic, path)
			fmt.Println(content)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	cmd.Flags().StringVar(&section, "section", "", "extract only the named section (e.g. '## Flink')")
	cmd.Flags().StringVar(&grep, "grep", "", "filter lines matching keyword (case-insensitive, ±1 line context)")
	cmd.Flags().IntVar(&limit, "limit", 0, "max lines to output (0 = unlimited)")
	return cmd
}

// ── where ─────────────────────────────────────────────────────────────────

func newMemoryWhereCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "where <description>",
		Short:        "Routing heuristic: where should this knowledge live?",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		Example: `  wtb memory where "Datadog API key para producao"
  wtb memory where "webhook-builder TM memory limit e 3200Mi"
  wtb memory where "Flink checkpoint interval agressivo causa cascata"`,
		Run: func(cmd *cobra.Command, args []string) {
			description := strings.Join(args, " ")
			result := memory.Route(description)
			fmt.Println(result.String())
		},
	}
}

// ── set ───────────────────────────────────────────────────────────────────

func newMemorySetCmd() *cobra.Command {
	var (
		entryType   string
		topic       string
		description string
		repo        string
	)

	cmd := &cobra.Command{
		Use:          "set <key> <value>",
		Short:        "Store a structured fact in docs.db",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		Example: `  wtb memory set webhook_builder_tm_memory_limit_mi 3200 --type threshold --topic webhook --desc "TM memory limit (PR #111)"
  wtb memory set flink_checkpoint_interval_ms 5000 --type config --topic webhook --desc "Safe default"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			if repo == "" {
				var err error
				repo, err = repoRoot()
				if err != nil {
					return err
				}
			}

			s, err := memory.LoadStore(repo)
			if err != nil {
				return fmt.Errorf("load store: %w", err)
			}

			s.Set(key, value, entryType, topic, description)
			_ = s.Save(repo) // no-op; docs.db writes synchronously

			fmt.Printf("✓ set %s = %s", key, value)
			if entryType != "" {
				fmt.Printf("  [%s]", entryType)
			}
			if topic != "" {
				fmt.Printf("  topic:%s", topic)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&entryType, "type", "", "entry type: threshold, limit, timeout, endpoint, config, fact")
	cmd.Flags().StringVar(&topic, "topic", "", "topic tag: webhook, airbyte, datadog, fusca, severino, k8s, ...")
	cmd.Flags().StringVar(&description, "desc", "", "human-readable description")
	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	return cmd
}

// ── list ──────────────────────────────────────────────────────────────────

func newMemoryListCmd() *cobra.Command {
	var (
		topic string
		repo  string
		stale int
	)

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List structured facts in docs.db",
		SilenceUsage: true,
		Example: `  wtb memory list                  # all entries
  wtb memory list --topic webhook  # entries for webhook topic
  wtb memory list --stale 60       # entries not verified in 60+ days`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				var err error
				repo, err = repoRoot()
				if err != nil {
					return err
				}
			}

			s, err := memory.LoadStore(repo)
			if err != nil {
				return fmt.Errorf("load store: %w", err)
			}

			var entries []memory.Entry
			if stale > 0 {
				entries = s.Stale(stale)
			} else {
				entries = s.FilterByTopic(topic)
			}

			if len(entries) == 0 {
				if stale > 0 {
					fmt.Printf("No entries older than %d days. ✅\n", stale)
				} else {
					fmt.Println("No entries found.")
				}
				return nil
			}

			// Group by topic for readability
			byTopic := make(map[string][]memory.Entry)
			for _, e := range entries {
				t := e.Topic
				if t == "" {
					t = "(no topic)"
				}
				byTopic[t] = append(byTopic[t], e)
			}

			topics := make([]string, 0, len(byTopic))
			for t := range byTopic {
				topics = append(topics, t)
			}
			sort.Strings(topics)

			for _, t := range topics {
				fmt.Printf("\n[%s]\n", t)
				for _, e := range byTopic[t] {
					verified := e.LastVerified
					if verified == "" {
						verified = "never"
					}
					desc := ""
					if e.Description != "" {
						desc = "  — " + e.Description
					}
					fmt.Printf("  %-45s = %-20s  [%s] verified:%s%s\n",
						e.Key, e.Value, e.Type, verified, desc)
				}
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "filter by topic")
	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	cmd.Flags().IntVar(&stale, "stale", 0, "show entries not verified in N+ days")
	return cmd
}

// ── validate ──────────────────────────────────────────────────────────────

func newMemoryValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "validate",
		Short:        "Run memory guardrails (bloat + content-leak)",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			results := []doccheck.GuardrailResult{
				doccheck.CheckMemoryIndexBloat(),
				doccheck.CheckMemoryContentLeak(),
			}

			allPassed := true
			for _, r := range results {
				fmt.Println(r.String())
				if !r.Passed {
					allPassed = false
				}
			}

			if allPassed {
				fmt.Println("\n✅ Memory guardrails passed.")
			} else {
				fmt.Fprintln(os.Stderr, "\n✗ Memory guardrails failed.")
				os.Exit(1)
			}
		},
	}
}

// ── append ────────────────────────────────────────────────────────────────

func newMemoryAppendCmd() *cobra.Command {
	var (
		topic   string
		section string
		file    string
		repo    string
	)

	cmd := &cobra.Command{
		Use:          "append <content>",
		Short:        "Append narrative content to a topic file",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		Example: `  wtb memory append --topic heuristics "Flink: helm uninstall+install resolve state incompatibility"
  wtb memory append --topic feedback_code_review --section "## CodeRabbit" "Always read all comments before applying any fix"
  wtb memory append --file memory/k8s-prod-access.md "kubectl patch: validate target env var name first"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			content := strings.Join(args, " ")

			if repo == "" {
				var err error
				repo, err = repoRoot()
				if err != nil {
					return err
				}
			}

			var filePath string
			if file != "" {
				filePath = file
				if !strings.HasPrefix(filePath, "/") {
					filePath = repo + "/" + filePath
				}
			} else if topic != "" {
				tm, err := memory.LoadTopicMap(repo)
				if err != nil {
					return fmt.Errorf("topic map: %w", err)
				}
				filePath, err = tm.Resolve(repo, topic)
				if err != nil {
					return fmt.Errorf("topic %q not found: %w", topic, err)
				}
			} else {
				return fmt.Errorf("one of --topic or --file is required")
			}

			if err := memory.AppendToTopic(filePath, section, content); err != nil {
				return err
			}

			fmt.Printf("✓ appended to %s", filePath)
			if section != "" {
				fmt.Printf(" (section: %s)", section)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "topic alias (resolved via topic-map.yml)")
	cmd.Flags().StringVar(&section, "section", "", "insert before next heading after this section (e.g. '## Flink')")
	cmd.Flags().StringVar(&file, "file", "", "direct file path (relative to repo root or absolute)")
	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	return cmd
}

// ── add-rule ──────────────────────────────────────────────────────────────

func newMemoryAddRuleCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:          "add-rule <one-liner>",
		Short:        "Append a one-liner rule to MEMORY.md § Regras de processo",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		Example: `  wtb memory add-rule "Helm --set numeric: always use values file — type coercion silenciosa causa falha"
  wtb memory add-rule "File deletion: grep -r referências antes de deletar — testes dependem"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rule := strings.Join(args, " ")

			if repo == "" {
				var err error
				repo, err = repoRoot()
				if err != nil {
					return err
				}
			}

			memPath := repo + "/.claude/projects/-Users-geraldo-thuler-workflow/memory/MEMORY.md"
			// Fallback: check common locations
			if _, err := os.Stat(memPath); err != nil {
				home, _ := os.UserHomeDir()
				memPath = home + "/.claude/projects/-Users-geraldo-thuler-workflow/memory/MEMORY.md"
			}

			if err := memory.AppendRuleToMemory(memPath, rule); err != nil {
				return err
			}

			fmt.Printf("✓ rule added to MEMORY.md § Regras de processo\n  %s\n", rule)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	return cmd
}

// ── migrate ───────────────────────────────────────────────────────────────

func newMemoryMigrateCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "Import context.json entries into docs.db",
		SilenceUsage: true,
		Example:      `  wtb memory migrate   # imports ~/workflow/context.json → docs.db`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				var err error
				repo, err = repoRoot()
				if err != nil {
					return err
				}
			}

			s, err := memory.LoadStore(repo)
			if err != nil {
				return fmt.Errorf("load store: %w", err)
			}

			n, err := memory.MigrateFromJSON(repo, s)
			if err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			if n == 0 {
				fmt.Println("Nothing to migrate (context.json absent or all entries already present).")
				return nil
			}
			fmt.Printf("✓ migrated %d entries from context.json → docs.db\n", n)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	return cmd
}

// repoRoot returns the workflow repo root.
// Priority: WTB_REPO_ROOT env var → current working directory.
func repoRoot() (string, error) {
	if envRoot := os.Getenv("WTB_REPO_ROOT"); envRoot != "" {
		return expandHome(envRoot), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	return wd, nil
}
