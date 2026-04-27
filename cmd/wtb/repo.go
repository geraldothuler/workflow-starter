// repo — wtb repo chain
// Indexes codebase entities into repos.db and exposes SQL-based querying.
// Primary consumer: Claude itself, loading context before refactor planning.
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/memory"
	"github.com/Cobliteam/workflow-toolkit/pkg/repoindex"
	"github.com/Cobliteam/workflow-toolkit/pkg/wtbserver"
	"github.com/spf13/cobra"
)

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Index and query codebase entities for refactor planning",
	}
	cmd.AddCommand(
		newRepoStatusCmd(),
		newRepoIndexCmd(),
		newRepoEmbedCmd(),
		newRepoSimilarCmd(),
		newRepoShowCmd(),
		newRepoQueryCmd(),
		newRepoNoteCmd(),
		newRepoListCmd(),
		newRepoTopologyCmd(),
		newRepoImpactCmd(),
		newRepoGrepCmd(),
		newRepoCanvasCmd(),
		newRepoSchemaDepCmd(),
		newRepoExportSummaryCmd(),
		newRepoSetOwnerCmd(),
		newRepoImportArchCmd(),
		newRepoImportChartCmd(),
		newRepoSetDDMonitorCmd(),
		newRepoEnrichDDCmd(),
		newRepoMetricsDDCmd(),
		newRepoImportOpenAPICmd(),
	)
	return cmd
}

// --- status ---

func newRepoStatusCmd() *cobra.Command {
	var staleDays int

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show health of all indexed repos — flags stale entries and suggests re-index",
		Long: `Lists every repo in repos.duckdb with days since last index.
Repos older than --stale days (default: repoindex_stale_days from docs.db, fallback 30)
are marked STALE with the exact command to refresh them.

Examples:
  wtb repo status
  wtb repo status --stale 14`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := workflowDirPath()

			// Load threshold from docs.db memory if not overridden by flag.
			if staleDays == 0 {
				if s, err := loadStaleThreshold(root); err == nil {
					staleDays = s
				}
			}

			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()

			report, err := repoindex.CheckStatus(db, staleDays)
			if err != nil {
				return err
			}
			fmt.Print(repoindex.RenderStatus(report))
			return nil
		},
	}

	cmd.Flags().IntVar(&staleDays, "stale", 0, "Days threshold for stale (0 = read from docs.db, fallback 30)")
	return cmd
}

// loadStaleThreshold reads repoindex_stale_days from docs.db.
func loadStaleThreshold(repoRoot string) (int, error) {
	s, err := memory.LoadStore(repoRoot)
	if err != nil {
		return 0, err
	}
	e, ok := s.Get("repoindex_stale_days")
	if !ok {
		return 0, fmt.Errorf("not set")
	}
	var n int
	_, err = fmt.Sscanf(e.Value, "%d", &n)
	return n, err
}

// --- index ---

func newRepoIndexCmd() *cobra.Command {
	var repoPath string
	var owner string
	var force bool
	var verbose bool
	var providerName string

	cmd := &cobra.Command{
		Use:   "index <name>",
		Short: "Index a repository into repos.db via LLM extraction",
		Long: `Indexes handlers, models, events, APIs, and config from a repo.
Uses file hashes to skip unchanged layers (incremental by default).

Examples:
  wtb repo index hermes
  wtb repo index hermes --path ~/Cobliteam/hermes
  wtb repo index hermes --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName := args[0]
			root := workflowDirPath()

			if repoPath == "" {
				repoPath = filepath.Join(os.Getenv("HOME"), "Cobliteam", repoName)
			}
			if _, err := os.Stat(repoPath); err != nil {
				return fmt.Errorf("repo path not found: %s", repoPath)
			}

			// Try daemon first — avoids DuckDB lock contention.
			params := url.Values{
				"name":     []string{repoName},
				"path":     []string{repoPath},
				"owner":    []string{owner},
				"provider": []string{providerName},
			}
			if force {
				params.Set("force", "true")
			}
			c := wtbserver.DefaultClient()
			ok, err := c.IndexAsync(params)
			if ok {
				return err
			}

			// Daemon not running — fall back to direct execution.
			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()

			resolver, _ := credentials.NewFullResolver(root, os.Getenv("WTB_MASTER_KEY"))
			provider, err := llm.NewProvider(llm.ProviderConfig{
				Provider:     providerName,
				CredResolver: resolver,
			})
			if err != nil {
				return fmt.Errorf("llm provider %q: %w\nhint: store key via: security add-generic-password -s workflow-anthropic-api-key -a geraldothuler -w <key>", providerName, err)
			}

			result := repoindex.IndexRepo(db, repoindex.IndexOptions{
				RepoName: repoName,
				RepoPath: repoPath,
				Owner:    owner,
				LLM:      provider,
				Force:    force,
				Verbose:  verbose || true, // always show progress
			})

			if result.Error != nil {
				return result.Error
			}

			if len(result.LayersIndexed) == 0 {
				fmt.Printf("repo %q is up-to-date (skipped: %s)\n", repoName, strings.Join(result.LayersSkipped, ", "))
			} else {
				fmt.Printf("indexed: %s\n", strings.Join(result.LayersIndexed, ", "))
				if len(result.LayersSkipped) > 0 {
					fmt.Printf("skipped (unchanged): %s\n", strings.Join(result.LayersSkipped, ", "))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repoPath, "path", "", "Absolute path to repo (default: ~/Cobliteam/<name>)")
	cmd.Flags().StringVar(&owner, "owner", "", "Squad/team owner (auto-detected from architecture/summary.md if omitted)")
	cmd.Flags().BoolVar(&force, "force", false, "Re-index all layers regardless of file hashes")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Print layer-level progress")
	cmd.Flags().StringVar(&providerName, "provider", "claudecli", "LLM provider: claudecli (default, uses Claude Code session), claude, gemini, chatgpt")
	return cmd
}

// --- export-summary ---

func newRepoExportSummaryCmd() *cobra.Command {
	var format string
	var outFile string

	cmd := &cobra.Command{
		Use:   "export-summary <name>",
		Short: "Export repo summary as markdown (architecture-compatible) or Datadog Service Catalog YAML",
		Long: `Renders indexed repo data as a structured summary.

Formats:
  markdown  — <reads_from>/<writes_to> compatible with Cobliteam/architecture (default)
  datadog   — Datadog Service Catalog YAML (schema-version: v2.2)

Examples:
  wtb repo export-summary fusca
  wtb repo export-summary fusca --format datadog
  wtb repo export-summary fusca --format markdown --out /tmp/fusca-summary.md`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName := args[0]

			// When writing to a file, skip daemon (need to capture output).
			if outFile == "" {
				params := url.Values{"name": []string{repoName}, "format": []string{format}}
				c := wtbserver.DefaultClient()
				if ok, err := c.Get("/repo/export-summary", params); ok {
					return err
				}
			}

			// Daemon not running or writing to file — direct DB access.
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()

			output, err := repoindex.ExportSummary(db, repoName, format)
			if err != nil {
				return err
			}

			if outFile != "" {
				if err := os.WriteFile(outFile, []byte(output), 0644); err != nil {
					return fmt.Errorf("write %s: %w", outFile, err)
				}
				fmt.Printf("wrote %s\n", outFile)
				return nil
			}

			fmt.Print(output)
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "markdown", "Output format: markdown | datadog")
	cmd.Flags().StringVar(&outFile, "out", "", "Write output to file instead of stdout")
	return cmd
}

// --- show ---

func newRepoShowCmd() *cobra.Command {
	var table bool
	var section string

	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show indexed entities for a repo (JSON by default)",
		Long: `Returns the full indexed snapshot of a repo as JSON (optimized for Claude).
Use --table for human-readable output, --section to filter.

Examples:
  wtb repo show hermes
  wtb repo show hermes --section handlers
  wtb repo show hermes --section models --table
  wtb repo show hermes --section apis --table

Sections: handlers, models, apis, events, config`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{"name": []string{args[0]}}
			if section != "" {
				params.Set("section", section)
			}
			if table {
				params.Set("table", "true")
			}
			c := wtbserver.DefaultClient()
			if ok, err := c.Get("/repo/show", params); ok {
				return err
			}

			// Daemon not running — direct DB access.
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			snap, err := repoindex.GetSnapshot(db, args[0])
			if err != nil {
				return err
			}

			if section != "" {
				cols, rows := repoindex.SnapshotSection(snap, section)
				if cols == nil {
					return fmt.Errorf("unknown section %q — use: handlers, models, apis, events, config", section)
				}
				if table {
					fmt.Print(repoindex.RenderTable(cols, rows))
					return nil
				}
				type row = map[string]string
				var out []row
				for _, r := range rows {
					m := make(row)
					for i, c := range cols {
						m[c] = r[i]
					}
					out = append(out, m)
				}
				return printJSON(out)
			}

			if table {
				printSnapTable(snap)
				return nil
			}
			return printJSON(snap)
		},
	}

	cmd.Flags().BoolVar(&table, "table", false, "Human-readable table output")
	cmd.Flags().StringVar(&section, "section", "", "Filter to one section: handlers, models, apis, events, config")
	return cmd
}

func printSnapTable(snap *repoindex.RepoSnapshot) {
	fmt.Printf("REPO: %s (%s + %s) — %s\n\n", snap.Repo.Name, snap.Repo.Lang, snap.Repo.Framework, snap.Repo.Path)

	for _, sec := range []string{"handlers", "models", "apis", "events", "config"} {
		cols, rows := repoindex.SnapshotSection(snap, sec)
		if len(rows) == 0 {
			continue
		}
		fmt.Printf("── %s ──\n", strings.ToUpper(sec))
		fmt.Print(repoindex.RenderTable(cols, rows))
		fmt.Println()
	}
}

// --- query ---

func newRepoQueryCmd() *cobra.Command {
	var table bool

	cmd := &cobra.Command{
		Use:   "query <sql>",
		Short: "Run a raw SQL query against repos.db",
		Long: `Executes any SQL against repos.db and returns JSON (default) or a table.

Tables: repos, handlers, events, models, model_fields, model_associations,
        external_apis, db_connections, config_vars, notes

Examples:
  wtb repo query "SELECT name,trigger_type,concurrency FROM handlers WHERE repo_id=(SELECT id FROM repos WHERE name='hermes')"
  wtb repo query "SELECT m.name,f.name,f.type FROM models m JOIN model_fields f ON f.model_id=m.id WHERE m.repo_id=(SELECT id FROM repos WHERE name='hermes') AND f.primary_key=1" --table
  wtb repo query "SELECT name,url,auth_type FROM external_apis ORDER BY auth_type" --table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{"sql": []string{args[0]}}
			if table {
				params.Set("table", "true")
			}
			c := wtbserver.DefaultClient()
			if ok, err := c.Get("/repo/query", params); ok {
				return err
			}

			// Daemon not running — direct DB access.
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			cols, rows, err := repoindex.QueryRows(db, args[0])
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}

			if table {
				fmt.Print(repoindex.RenderTable(cols, rows))
				return nil
			}

			type row = map[string]string
			var out []row
			for _, r := range rows {
				m := make(row)
				for i, c := range cols {
					m[c] = r[i]
				}
				out = append(out, m)
			}
			return printJSON(out)
		},
	}

	cmd.Flags().BoolVar(&table, "table", false, "Human-readable table output")
	return cmd
}

// --- note ---

func newRepoNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note <entity-type> <entity-id> <content>",
		Short: "Attach a free-text note to a repo entity",
		Long: `Adds a note to any indexed entity for planning annotations.

Entity types: handler, model, api, event, repo
Entity ID: the entity name (e.g., "processTransaction") or repo name.

Examples:
  wtb repo note handler processTransaction "candidate for Flink KafkaConsumer — processes RAW_TRANSACTION_ADDED"
  wtb repo note model RawTransaction "maps to raw_transactions PG table — migrate to JPA Entity"
  wtb repo note repo hermes "refactor target: split into 3 services — scheduler, parser, processor"`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityType, entityID, content := args[0], args[1], args[2]

			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			// Infer repo from entity if possible, else ask.
			// For simplicity, use entityID as repoName when entityType=="repo".
			repoName := entityID
			if entityType != "repo" {
				// Look up which repo owns this entity.
				repoName, err = inferRepo(db, entityType, entityID)
				if err != nil {
					return fmt.Errorf("could not infer repo for %s %q — pass wtb repo note %s %s %q after wtb repo index <name>", entityType, entityID, entityType, entityID, content)
				}
			}

			if err := repoindex.AddNote(db, entityType, entityID, repoName, content); err != nil {
				return err
			}
			fmt.Printf("note added to %s/%s\n", entityType, entityID)
			return nil
		},
	}
	return cmd
}

func inferRepo(db *repoindex.DB, entityType, entityID string) (string, error) {
	tableMap := map[string]string{
		"handler": "handlers",
		"model":   "models",
		"api":     "external_apis",
		"event":   "events",
	}
	table, ok := tableMap[entityType]
	if !ok {
		return "", fmt.Errorf("unknown entity type %q", entityType)
	}
	var repoID string
	err := db.Raw().QueryRow(fmt.Sprintf(`SELECT repo_id FROM %s WHERE name=? LIMIT 1`, table), entityID).Scan(&repoID)
	if err != nil {
		return "", err
	}
	var repoName string
	err = db.Raw().QueryRow(`SELECT name FROM repos WHERE id=?`, repoID).Scan(&repoName)
	return repoName, err
}

// --- list ---

func newRepoListCmd() *cobra.Command {
	var table bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all indexed repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if table {
				params.Set("table", "true")
			}
			c := wtbserver.DefaultClient()
			if ok, err := c.Get("/repo/list", params); ok {
				return err
			}

			// Daemon not running — direct DB access.
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			repos, err := repoindex.ListRepos(db)
			if err != nil {
				return err
			}

			if table {
				cols := []string{"name", "lang", "framework", "last_indexed_at", "path"}
				var rows [][]string
				for _, r := range repos {
					rows = append(rows, []string{r.Name, r.Lang, r.Framework, r.LastIndexedAt, r.Path})
				}
				fmt.Print(repoindex.RenderTable(cols, rows))
				return nil
			}
			return printJSON(repos)
		},
	}

	cmd.Flags().BoolVar(&table, "table", false, "Human-readable table output")
	return cmd
}

// --- embed ---

func newRepoEmbedCmd() *cobra.Command {
	var force bool
	var apiKey string

	cmd := &cobra.Command{
		Use:   "embed <name>",
		Short: "Generate vector embeddings for all indexed entities in a repo",
		Long: `Calls OpenAI text-embedding-3-small for each entity (handler, model, api, event).
Embeddings are stored as BLOB in repos.db and used by wtb repo similar.
Requires OPENAI_API_KEY env var (or --api-key).

Run after: wtb repo index <name>

Examples:
  wtb repo embed hermes
  wtb repo embed hermes --force   # re-embed everything`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			return repoindex.EmbedRepo(db, repoindex.EmbedOptions{
				RepoName: args[0],
				APIKey:   apiKey,
				Force:    force,
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Re-embed all entities even if already embedded")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "OpenAI API key (default: $OPENAI_API_KEY)")
	return cmd
}

// --- similar ---

func newRepoSimilarCmd() *cobra.Command {
	var topN int
	var table bool

	cmd := &cobra.Command{
		Use:   "similar <repo> <entity-type> <entity-name>",
		Short: "Find semantically similar entities across all indexed repos",
		Long: `Uses cosine similarity on stored embeddings to find analogous entities.
Useful when mapping hermes handlers to their Scala/Kotlin equivalents for refactor planning.

Requires embeddings: run wtb repo embed <name> first.

Entity types: handler, model, api, event

Examples:
  wtb repo similar hermes handler processTransaction
  wtb repo similar hermes model RawTransaction --top 5 --table
  wtb repo similar hermes api getWizeoTransactions`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName, entityType, entityID := args[0], args[1], args[2]

			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			results, err := repoindex.SimilarEntities(db, repoName, entityType, entityID, topN)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Println("no similar entities found")
				return nil
			}

			if table {
				cols := []string{"score", "entity_type", "entity_id", "repo"}
				var rows [][]string
				for _, r := range results {
					rows = append(rows, []string{fmt.Sprintf("%.4f", r.Score), r.EntityType, r.EntityID, r.RepoName})
				}
				fmt.Print(repoindex.RenderTable(cols, rows))
				return nil
			}
			return printJSON(results)
		},
	}

	cmd.Flags().IntVar(&topN, "top", 10, "Number of results to return")
	cmd.Flags().BoolVar(&table, "table", false, "Human-readable table output")
	return cmd
}

// --- topology ---

func newRepoTopologyCmd() *cobra.Command {
	var repoFilter string
	var topicFilter string
	var dot bool

	cmd := &cobra.Command{
		Use:   "topology",
		Short: "Show Kafka topic dependency graph across all indexed repos",
		Long: `Builds the full producer→topic→consumer map from indexed events.
Config-key placeholders (app.*, ${...}) are excluded automatically.

Examples:
  wtb repo topology
  wtb repo topology --repo alexstrasza-status-event
  wtb repo topology --topic device-path
  wtb repo topology --dot > /tmp/kafka.dot && dot -Tpng /tmp/kafka.dot -o /tmp/kafka.png`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			nodes, err := repoindex.BuildTopology(db)
			if err != nil {
				return err
			}

			filtered := repoindex.FilterTopology(nodes, repoFilter, topicFilter)

			if dot {
				fmt.Print(repoindex.RenderTopologyDOT(filtered))
				return nil
			}
			fmt.Print(repoindex.RenderTopologyTable(filtered))
			return nil
		},
	}

	cmd.Flags().StringVar(&repoFilter, "repo", "", "Filter to topics involving this repo")
	cmd.Flags().StringVar(&topicFilter, "topic", "", "Filter by topic name substring")
	cmd.Flags().BoolVar(&dot, "dot", false, "Output Graphviz DOT format (pipe to dot -Tpng)")
	return cmd
}

// --- impact ---

func newRepoImpactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "impact <repo1> <repo2> [repo3...]",
		Short: "Show what changes if these repos were merged into one",
		Long: `Computes which Kafka topics would become internal (eliminatable),
which must remain as external inputs/outputs, and how many jobs are saved.

Examples:
  wtb repo impact alexstrasza-status-event sherlock-driver
  wtb repo impact alexstrasza-device-path device-stops stop-fuel-cost-job trip-fuel-cost-job`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			result, err := repoindex.ImpactAnalysis(db, args)
			if err != nil {
				return err
			}
			fmt.Print(repoindex.RenderImpact(result))
			return nil
		},
	}
	return cmd
}

// --- grep ---

func newRepoGrepCmd() *cobra.Command {
	var repoFilter string
	var extFilter string
	var maxMatches int
	var table bool

	cmd := &cobra.Command{
		Use:   "grep <pattern>",
		Short: "Search source files across all indexed repos",
		Long: `Searches source files using a Go regexp pattern.
Skips .git, target, build, node_modules. Useful to verify what LLM extraction may have missed.

Examples:
  wtb repo grep "device-driver-identification-events"
  wtb repo grep "KafkaListener" --repo fusca --ext .kt
  wtb repo grep "@KafkaListener|@KafkaHandler" --table
  wtb repo grep "identification-token|device-driver" --repo fusca`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			var exts []string
			if extFilter != "" {
				for _, e := range strings.Split(extFilter, ",") {
					e = strings.TrimSpace(e)
					if !strings.HasPrefix(e, ".") {
						e = "." + e
					}
					exts = append(exts, e)
				}
			}

			matches, err := repoindex.GrepRepos(db, repoindex.GrepOptions{
				Pattern:    args[0],
				RepoFilter: repoFilter,
				ExtFilter:  exts,
				MaxMatches: maxMatches,
			})
			if err != nil {
				return err
			}

			if table {
				fmt.Print(repoindex.RenderGrepTable(matches))
				return nil
			}
			fmt.Print(repoindex.RenderGrepGrouped(matches))
			fmt.Printf("\n%d match(es)\n", len(matches))
			return nil
		},
	}

	cmd.Flags().StringVar(&repoFilter, "repo", "", "Limit search to one repo")
	cmd.Flags().StringVar(&extFilter, "ext", "", "Comma-separated extensions to search (e.g. .kt,.scala)")
	cmd.Flags().IntVar(&maxMatches, "max", 200, "Max matches to return (0 = unlimited)")
	cmd.Flags().BoolVar(&table, "table", false, "Tabular output instead of grouped grep-style")
	return cmd
}

// --- canvas ---

func newRepoCanvasCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "canvas [repo]",
		Short: "Migration canvas: pipeline position, deps, state, blast radius, merge candidates",
		Long: `Renders a consolidated migration planning view for a repo (or all repos).

Shows 5 axes used to evaluate refactor and merge hypotheses:
  • Pipeline position  — what it consumes and produces, with peer repos
  • Dependency weight  — external providers, DBs, APIs, blocking I/O
  • State complexity   — Flink FSM state classes and serializers
  • Blast radius       — downstream impact if this job stops
  • Merge candidates   — adjacent repos with coupling score

Examples:
  wtb repo canvas device-stops
  wtb repo canvas alexstrasza-device-path
  wtb repo canvas --all`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			if all {
				repos, err := repoindex.ListRepos(db)
				if err != nil {
					return err
				}
				for _, r := range repos {
					canvas, err := repoindex.BuildCanvas(db, r.Name)
					if err != nil {
						fmt.Fprintf(os.Stderr, "canvas %s: %v\n", r.Name, err)
						continue
					}
					fmt.Print(repoindex.RenderCanvas(canvas))
				}
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("provide a repo name or use --all")
			}

			canvas, err := repoindex.BuildCanvas(db, args[0])
			if err != nil {
				return err
			}
			fmt.Print(repoindex.RenderCanvas(canvas))
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Render canvas for all indexed repos")
	return cmd
}

func newRepoSchemaDepCmd() *cobra.Command {
	var repoFilter, modelFilter string
	var byModel, index bool

	cmd := &cobra.Command{
		Use:   "schema-deps",
		Short: "Map schema-registry proto contracts to repos that use them",
		Long: `Scans source files of all indexed repos for proto model name occurrences
and records the linkage in the database.

After indexing, query the results:

  wtb repo schema-deps --index             # scan all repos (run once / after re-index)
  wtb repo schema-deps                     # all deps, grouped by repo
  wtb repo schema-deps --repo fusca        # contracts used by fusca
  wtb repo schema-deps --model StatusEvent # repos that reference StatusEvent
  wtb repo schema-deps --by-model          # invert: group by contract, show consumer repos`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return err
			}
			defer db.Close()

			if index {
				fmt.Fprintln(os.Stderr, "Scanning repos for proto model references…")
				n, err := repoindex.IndexSchemaDeps(db)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Indexed %d (repo, model) pairs.\n", n)
				return nil
			}

			deps, err := repoindex.QuerySchemaDeps(db, repoFilter, modelFilter)
			if err != nil {
				return err
			}
			if len(deps) == 0 {
				fmt.Println("(no results — run: wtb repo schema-deps --index first)")
				return nil
			}

			if byModel {
				fmt.Print(repoindex.RenderSchemaDepsByModel(deps))
			} else {
				fmt.Print(repoindex.RenderSchemaDepsByRepo(deps))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repoFilter, "repo", "", "Filter by repo name substring")
	cmd.Flags().StringVar(&modelFilter, "model", "", "Filter by model name substring")
	cmd.Flags().BoolVar(&byModel, "by-model", false, "Group output by proto model (invert view)")
	cmd.Flags().BoolVar(&index, "index", false, "Scan all repos and populate schema_deps table")
	return cmd
}

// --- set-owner ---

func newRepoSetOwnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-owner <name> <owner>",
		Short: "Update the owner/squad of an indexed repo without re-indexing",
		Example: `  wtb repo set-owner webhook kali
  wtb repo set-owner fusca account-management`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName, owner := args[0], args[1]
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()
			if err := repoindex.SetOwner(db, repoName, owner); err != nil {
				return err
			}
			fmt.Printf("owner of %q set to %q\n", repoName, owner)
			return nil
		},
	}
	return cmd
}

// --- import-arch ---

func newRepoImportArchCmd() *cobra.Command {
	var all bool
	var cobliteamDir string

	cmd := &cobra.Command{
		Use:   "import-arch [name]",
		Short: "Import architecture enrichment from Cobliteam/architecture summary.md files",
		Long: `Reads ~/Cobliteam/architecture/docs/*/repoName/summary.md and populates:
  - deployment_units  (names, namespaces, replicas, consumer groups, deprecated flag)
  - topic_enrichments (Kafka topics with serialization + consumer groups)
  - repos.dd_service_name / repos.primary_hostname

Examples:
  wtb repo import-arch fusca
  wtb repo import-arch --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return fmt.Errorf("provide a repo name or --all")
			}
			root := workflowDirPath()
			if cobliteamDir == "" {
				cobliteamDir = filepath.Join(os.Getenv("HOME"), "Cobliteam")
			}

			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()

			var names []string
			if all {
				repos, err := repoindex.ListRepos(db)
				if err != nil {
					return err
				}
				for _, r := range repos {
					names = append(names, r.Name)
				}
			} else {
				names = args
			}

			ok, skipped, errs := 0, 0, 0
			for _, name := range names {
				summary, err := repoindex.ParseArchSummary(name, cobliteamDir)
				if err != nil {
					if all {
						skipped++
						continue
					}
					return err
				}
				if err := repoindex.ImportArchSummary(db, summary); err != nil {
					fmt.Fprintf(os.Stderr, "  error importing %s: %v\n", name, err)
					errs++
					continue
				}
				units := len(summary.Units)
				topics := len(summary.Topics)
				dd := summary.DDServiceName
				if dd == "" {
					dd = "—"
				}
				fmt.Printf("  %-35s  units=%-2d  topics=%-3d  dd=%s\n", name, units, topics, dd)
				ok++
			}

			if all {
				fmt.Printf("\nimported=%d  skipped=%d  errors=%d\n", ok, skipped, errs)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Import architecture for all indexed repos")
	cmd.Flags().StringVar(&cobliteamDir, "cobliteam", "", "Path to Cobliteam dir (default: ~/Cobliteam)")
	return cmd
}

// --- set-dd-monitor ---

// newRepoSetDDMonitorCmd stores a single Datadog monitor reference for a repo.
// Used by Claude/scripts after querying the DD MCP or REST API.
// --- import-chart ---

func newRepoImportChartCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "import-chart [name]",
		Short: "Import Helm chart snapshots (image, resources, env vars, sidecars) from deploy/helm/",
		Long: `Reads deploy/helm/**/values.yaml files and stores:
  - chart_snapshots  (app version, image tag, env)
  - chart_resources  (CPU/memory limits, heap size, replicas)
  - chart_env_vars   (non-secret environment variables)
  - chart_sidecars   (sidecar containers)

Examples:
  wtb repo import-chart fusca
  wtb repo import-chart --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return fmt.Errorf("provide a repo name or --all")
			}
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()

			var names []string
			if all {
				repos, err := repoindex.ListRepos(db)
				if err != nil {
					return err
				}
				for _, r := range repos {
					names = append(names, r.Name)
				}
			} else {
				names = args
			}

			ok, skipped := 0, 0
			for _, name := range names {
				snap, err := repoindex.GetSnapshot(db, name)
				if err != nil {
					skipped++
					continue
				}
				repoPath := snap.Repo.Path
				snaps, err := repoindex.ParseChartSnapshots(name, repoPath)
				if err != nil {
					if all {
						skipped++
						continue
					}
					return err
				}
				if err := repoindex.ImportChartSnapshots(db, name, snaps); err != nil {
					fmt.Fprintf(os.Stderr, "  error importing %s: %v\n", name, err)
					skipped++
					continue
				}

				// Also import service routes from the same values.yaml files
				routes := repoindex.ParseServiceRoutes(repoPath)
				if len(routes) > 0 {
					if err := repoindex.ImportServiceRoutes(db, name, routes); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: routes import %s: %v\n", name, err)
					}
				}

				for _, s := range snaps {
					nenv := len(s.EnvVars)
					nres := len(s.Resources)
					tag := s.ImageTag
					if tag == "" || tag == "change-me" {
						tag = "—"
					}
					fmt.Printf("  %-35s  deployment=%-35s  resources=%d  env=%d  routes=%d  tag=%s\n",
						name, s.AppVersion, nres, nenv, len(routes), tag)
				}
				ok++
			}
			if all {
				fmt.Printf("\nimported=%d  skipped=%d\n", ok, skipped)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Import charts for all indexed repos")
	return cmd
}

// --- import-openapi ---

func newRepoImportOpenAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-openapi <path>",
		Short: "Import public endpoints from an OpenAPI spec repo (api-docs)",
		Long: `Parses spec/spec.yaml (and referenced path files) from a local api-docs clone.
Populates public_endpoints with path, method, operationId, summary, and auth_type.

After running, wtb repo canvas <name> will show a PUBLIC API SURFACE section
with documented endpoints and handler gaps for each service.

Examples:
  wtb repo import-openapi ~/Cobliteam/api-docs`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := args[0]
			if _, err := os.Stat(repoPath); err != nil {
				return fmt.Errorf("path not found: %s", repoPath)
			}

			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()

			fmt.Printf("parsing %s/spec/spec.yaml ...\n", repoPath)
			n, err := repoindex.ImportPublicEndpoints(db, repoPath)
			if err != nil {
				return fmt.Errorf("import: %w", err)
			}
			fmt.Printf("imported %d public endpoints\n", n)
			return nil
		},
	}
	return cmd
}

func newRepoSetDDMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-dd-monitor <repo> <monitor-id> <name> <type> <status>",
		Short: "Cache a Datadog monitor reference for a repo",
		Example: `  wtb repo set-dd-monitor fusca 12345678 "fusca error rate" metric alert`,
		Args: cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName, monitorID, name, mtype, status := args[0], args[1], args[2], args[3], args[4]
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()
			if err := repoindex.SetDDMonitor(db, repoName, monitorID, name, mtype, status); err != nil {
				return err
			}
			fmt.Printf("monitor %s (%s) linked to %q\n", monitorID, name, repoName)
			return nil
		},
	}
	return cmd
}

// --- dd-enrich ---

// newRepoEnrichDDCmd fetches Datadog monitors for repos via the DD REST API and caches them.
func newRepoEnrichDDCmd() *cobra.Command {
	var all bool
	var ddServiceOverride string
	var staleHours int

	cmd := &cobra.Command{
		Use:   "dd-enrich [name]",
		Short: "Fetch and cache Datadog monitors for a repo (uses DD API key from Keychain)",
		Long: `Queries the Datadog API for monitors tagged service:<dd_service_name> and stores
results in dd_monitors. Uses DD API/App keys from macOS Keychain.

Use --stale N to skip repos whose monitors were fetched within the last N hours.

Examples:
  wtb repo dd-enrich fusca
  wtb repo dd-enrich fusca --dd-service fusca-api
  wtb repo dd-enrich --all
  wtb repo dd-enrich --all --stale 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return fmt.Errorf("provide a repo name or --all")
			}
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()

			// Try resolver first (wtb keychain); fall back to geraldothuler account (manual Keychain setup)
			resolver, _ := credentials.NewFullResolver(root, os.Getenv("WTB_MASTER_KEY"))
			ddAPIKey, ddAppKey := "", ""
			if cred, err := resolver.Resolve(cmd.Context(), "workflow-dd-api-key", nil); err == nil {
				ddAPIKey = cred.Value
			}
			if ddAPIKey == "" {
				out, err2 := exec.Command("security", "find-generic-password", "-s", "workflow-dd-api-key", "-a", "geraldothuler", "-w").Output()
				if err2 != nil {
					return fmt.Errorf("DD API key not found: %w\nhint: security add-generic-password -s workflow-dd-api-key -a geraldothuler -w <key>", err2)
				}
				ddAPIKey = strings.TrimSpace(string(out))
			}
			if cred, err := resolver.Resolve(cmd.Context(), "workflow-dd-app-key", nil); err == nil {
				ddAppKey = cred.Value
			}
			if ddAppKey == "" {
				out, err2 := exec.Command("security", "find-generic-password", "-s", "workflow-dd-app-key", "-a", "geraldothuler", "-w").Output()
				if err2 != nil {
					return fmt.Errorf("DD App key not found: %w", err2)
				}
				ddAppKey = strings.TrimSpace(string(out))
			}

			var names []string
			if all {
				repos, err := repoindex.ListRepos(db)
				if err != nil {
					return err
				}
				for _, r := range repos {
					names = append(names, r.Name)
				}
			} else {
				names = args
			}

			ok, skipped, total := 0, 0, 0
			for _, name := range names {
				snap, err := repoindex.GetSnapshot(db, name)
				if err != nil {
					skipped++
					continue
				}

				// Staleness gate: skip if monitors were fetched within --stale hours
				if staleHours > 0 && len(snap.DDMonitors) > 0 {
					newest := snap.DDMonitors[len(snap.DDMonitors)-1].FetchedAt
					if t, err2 := time.Parse(time.RFC3339, newest); err2 == nil {
						if time.Since(t) < time.Duration(staleHours)*time.Hour {
							skipped++
							continue
						}
					}
				}

				svcName := snap.Repo.DDServiceName
				if ddServiceOverride != "" {
					svcName = ddServiceOverride
				}
				if svcName == "" {
					svcName = name // fallback: use repo name
				}

				monitors, err := repoindex.FetchDDMonitors(ddAPIKey, ddAppKey, svcName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  warn: %s: %v\n", name, err)
					skipped++
					continue
				}
				if len(monitors) == 0 {
					skipped++
					continue
				}
				for _, m := range monitors {
					if err := repoindex.SetDDMonitor(db, name, m.MonitorID, m.Name, m.Type, m.Status); err != nil {
						fmt.Fprintf(os.Stderr, "  error storing monitor %s: %v\n", m.MonitorID, err)
					}
				}
				fmt.Printf("  %-35s  monitors=%d  (svc=%s)\n", name, len(monitors), svcName)
				total += len(monitors)
				ok++
			}
			fmt.Printf("\nrepos=%d  total_monitors=%d  skipped=%d\n", ok, total, skipped)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Enrich all indexed repos")
	cmd.Flags().StringVar(&ddServiceOverride, "dd-service", "", "Override DD service name (default: dd_service_name from arch import)")
	cmd.Flags().IntVar(&staleHours, "stale", 0, "Skip repos whose monitors were fetched within N hours (0 = always refresh)")
	return cmd
}

// --- dd-metrics ---

// newRepoMetricsDDCmd fetches active Datadog metrics for repos and stores categorized results.
func newRepoMetricsDDCmd() *cobra.Command {
	var all bool
	var ddServiceOverride string
	var staleHours int

	cmd := &cobra.Command{
		Use:   "dd-metrics [name]",
		Short: "Fetch and cache categorized Datadog metrics for a repo (business, apm, middleware, kafka, flink, jvm)",
		Long: `Queries the DD v2 metrics API for metrics active in last 24h tagged service:<name>.
Categorizes them (business, apm, middleware, kafka, flink, jvm) and stores in service_metrics.
Skips generic infra noise (container.*, kubernetes.*, etc.).

Use --stale N to skip repos whose metrics were fetched within the last N hours (for scheduled refresh).

Examples:
  wtb repo dd-metrics fusca
  wtb repo dd-metrics webhook --dd-service webhook-builder
  wtb repo dd-metrics --all
  wtb repo dd-metrics --all --stale 24`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return fmt.Errorf("provide a repo name or --all")
			}
			root := workflowDirPath()
			db, err := repoindex.Open(root)
			if err != nil {
				return fmt.Errorf("open repos.db: %w", err)
			}
			defer db.Close()

			// Keychain fallback (same pattern as dd-enrich)
			resolver, _ := credentials.NewFullResolver(root, os.Getenv("WTB_MASTER_KEY"))
			ddAPIKey, ddAppKey := "", ""
			if cred, err2 := resolver.Resolve(cmd.Context(), "workflow-dd-api-key", nil); err2 == nil {
				ddAPIKey = cred.Value
			}
			if ddAPIKey == "" {
				out, err2 := exec.Command("security", "find-generic-password", "-s", "workflow-dd-api-key", "-a", "geraldothuler", "-w").Output()
				if err2 != nil {
					return fmt.Errorf("DD API key not found: %w\nhint: security add-generic-password -s workflow-dd-api-key -a geraldothuler -w <key>", err2)
				}
				ddAPIKey = strings.TrimSpace(string(out))
			}
			if cred, err2 := resolver.Resolve(cmd.Context(), "workflow-dd-app-key", nil); err2 == nil {
				ddAppKey = cred.Value
			}
			if ddAppKey == "" {
				out, err2 := exec.Command("security", "find-generic-password", "-s", "workflow-dd-app-key", "-a", "geraldothuler", "-w").Output()
				if err2 != nil {
					return fmt.Errorf("DD App key not found: %w", err2)
				}
				ddAppKey = strings.TrimSpace(string(out))
			}

			var names []string
			if all {
				repos, err := repoindex.ListRepos(db)
				if err != nil {
					return err
				}
				for _, r := range repos {
					names = append(names, r.Name)
				}
			} else {
				names = args
			}

			ok, skipped, total := 0, 0, 0
			for _, name := range names {
				snap, err := repoindex.GetSnapshot(db, name)
				if err != nil {
					skipped++
					continue
				}

				// Staleness gate: skip if metrics were fetched within --stale hours
				if staleHours > 0 && len(snap.ServiceMetrics) > 0 {
					newest := snap.ServiceMetrics[len(snap.ServiceMetrics)-1].FetchedAt
					if t, err2 := time.Parse(time.RFC3339, newest); err2 == nil {
						if time.Since(t) < time.Duration(staleHours)*time.Hour {
							skipped++
							continue
						}
					}
				}

				svcName := snap.Repo.DDServiceName
				if ddServiceOverride != "" {
					svcName = ddServiceOverride
				}
				if svcName == "" {
					svcName = name
				}

				metrics, err := repoindex.FetchServiceMetrics(ddAPIKey, ddAppKey, svcName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  warn: %s: %v\n", name, err)
					skipped++
					continue
				}
				if len(metrics) == 0 {
					skipped++
					continue
				}
				if err := repoindex.StoreServiceMetrics(db, name, metrics); err != nil {
					fmt.Fprintf(os.Stderr, "  error storing metrics %s: %v\n", name, err)
					skipped++
					continue
				}

				// Count by category for display
				cats := map[string]int{}
				for _, m := range metrics {
					cats[m.Category]++
				}
				fmt.Printf("  %-35s  metrics=%d  business=%d apm=%d middleware=%d kafka=%d flink=%d jvm=%d  (svc=%s)\n",
					name, len(metrics),
					cats["business"], cats["apm"], cats["middleware"], cats["kafka"], cats["flink"], cats["jvm"],
					svcName)
				total += len(metrics)
				ok++
			}
			fmt.Printf("\nrepos=%d  total_metrics=%d  skipped=%d\n", ok, total, skipped)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Enrich all indexed repos")
	cmd.Flags().StringVar(&ddServiceOverride, "dd-service", "", "Override DD service name")
	cmd.Flags().IntVar(&staleHours, "stale", 0, "Skip repos whose metrics were fetched within N hours (0 = always refresh)")
	return cmd
}

// printJSON marshals v to indented JSON on stdout.
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
