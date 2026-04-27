package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/ops"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// opsStore holds persistent --store flags set on the ops root command.
// Package-level so child command RunE closures can read them directly.
var (
	opsStorePath string // "" = disabled; "default" or a path = enabled
	opsStoreRepo string // repo context written to each record
)

// opsPrintStore prints an OpsResult and optionally appends to the JSONL store.
// probe is the canonical probe name (e.g. "db-health").
func opsPrintStore(probe string, r ops.OpsResult) {
	opsPrint(r)
	path := opsStorePath
	if path == "default" {
		path = os.Getenv("WTB_STORE_PATH")
		if path == "" {
			// inline import avoided — resolve via store package at link time
			home, _ := os.UserHomeDir()
			path = home + "/.workflow/ops-log.jsonl"
		}
	}
	repo := opsStoreRepo
	if repo == "" {
		repo, _ = repoRoot()
	}
	storeAppend(path, probe, r.Status, r.Signal, repo)
}

// newOpsCmd builds the `wtb ops` subcommand tree.
func newOpsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ops <subcommand>",
		Short: "Direct access to Ops Toolbox commands (zero-LLM heuristics)",
		Long: `Direct CLI access to ops commands without running the full pipeline.

Examples:
  wtb ops probe --namespace fusca --profile cobli-tech
  wtb ops db-health --namespace fusca --db-host pg.internal
  wtb ops k8s-status --namespace fusca --deployment fusca-api
  wtb ops kafka-status --namespace fusca --deployment fusca-cdc
  wtb ops logs-analyze --file /var/log/app.log --patterns kafka,oom
  wtb ops jira --url https://company.atlassian.net --email a@b.com --token XXX --project PROJ
  wtb ops slack --token xoxb-XXX --channel C0123456
  wtb ops websearch --api-key XXX --cse-id XXX --query "service outage"
  wtb ops snowflake --account acct --user usr --query "SELECT 1"
  wtb ops montecarlo --api-key XXX
  wtb ops airbyte --url https://airbyte.internal --workspace-id ws1
  wtb ops airbyte --url https://airbyte.internal --workspace-id ws1 --mode schedule-map
  wtb ops airbyte --url https://airbyte.internal --mode job-profile --connection-id conn-abc --schedule-interval-min 15
  wtb ops github --repo Cobliteam/fusca --scope pr --pr 1078
  wtb ops github --repo Cobliteam/fusca --scope ci
  wtb ops github --repo Cobliteam/fusca --scope issues --query "label:bug"
  wtb ops github --repo Cobliteam/fusca --scope releases
  wtb ops github --repo Cobliteam/fusca --scope deployments
  wtb ops plan new --template health-check --scenario "CDC lag increasing"
  wtb ops plan show
  wtb ops plan execute --dry-run`,
	}

	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Ops plan management (new / show / execute)",
	}
	planCmd.AddCommand(newOpsPlanNewCmd(), newOpsPlanShowCmd(), newOpsPlanExecuteCmd())

	// Persistent store flags — available to all ops subcommands.
	cmd.PersistentFlags().StringVar(&opsStorePath, "store", "", `append result to ops log ("default" = ~/.workflow/ops-log.jsonl, or explicit path)`)
	cmd.PersistentFlags().StringVar(&opsStoreRepo, "store-repo", "", "repo context tag written to each store record (default: current dir)")

	cmd.AddCommand(
		newOpsProbeCmd(),
		newOpsDBHealthCmd(),
		newOpsK8sStatusCmd(),
		newOpsKafkaStatusCmd(),
		newOpsLogsAnalyzeCmd(),
		newOpsJiraCmd(),
		newOpsSlackCmd(),
		newOpsWebSearchCmd(),
		newOpsSnowflakeCmd(),
		newOpsMonteCarloCmd(),
		newOpsAirbyteCmd(),
		newOpsGitHubCmd(),
		newOpsCISentinelCmd(),
		newOpsTestDiagnoseCmd(),
		planCmd,
	)
	return cmd
}

// opsLine prints a labelled OpsResult on one line.
func opsLine(label string, r ops.OpsResult) {
	icon := map[string]string{"ok": "✓", "warn": "⚠", "critical": "✗", "error": "✗"}[r.Status]
	if icon == "" {
		icon = "•"
	}
	fmt.Printf("%-12s %s %s\n", "["+label+"]", icon, r.Signal)
	for _, a := range r.Actions {
		fmt.Printf("             → %s\n", a)
	}
}

// opsPrint prints an OpsResult without a label prefix.
func opsPrint(r ops.OpsResult) {
	icon := map[string]string{"ok": "✓", "warn": "⚠", "critical": "✗", "error": "✗"}[r.Status]
	if icon == "" {
		icon = "•"
	}
	fmt.Printf("%s %s\n", icon, r.Signal)
	for _, a := range r.Actions {
		fmt.Printf("  → %s\n", a)
	}
}

// ── probe ─────────────────────────────────────────────────────────────────────

func newOpsProbeCmd() *cobra.Command {
	var namespace, profile, deployment, kubectlCtx string
	cmd := &cobra.Command{
		Use:   "probe",
		Short: "Run all environment probes: auth + DB health + k8s + Kafka",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("=== ops probe ===")
			opsLine("auth", ops.CheckAWSAuth(profile))
			opsLine("db-health", ops.CheckDBHealth(ops.DBHealthConfig{
				KubectlContext: kubectlCtx,
				Namespace:      namespace,
				AWSProfile:     profile,
			}))
			opsLine("k8s-status", ops.CheckK8sStatus(ops.K8sConfig{
				KubectlContext: kubectlCtx,
				Namespace:      namespace,
				Deployment:     deployment,
			}))
			opsLine("kafka-status", ops.CheckKafkaStatus(ops.KafkaConfig{
				KubectlContext: kubectlCtx,
				Namespace:      namespace,
				Deployment:     deployment,
			}))
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS SSO profile")
	cmd.Flags().StringVar(&deployment, "deployment", "", "Kubernetes deployment name")
	cmd.Flags().StringVar(&kubectlCtx, "kubectl-context", "", "kubectl context")
	return cmd
}

// ── db-health ─────────────────────────────────────────────────────────────────

func newOpsDBHealthCmd() *cobra.Command {
	var namespace, profile, kubectlCtx, dbHost, dbUser, dbName, dbPort string
	cmd := &cobra.Command{
		Use:   "db-health",
		Short: "Check PostgreSQL health (locks, WAL lag, long txns, bloat)",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := ops.CheckDBHealth(ops.DBHealthConfig{
				KubectlContext: kubectlCtx,
				Namespace:      namespace,
				AWSProfile:     profile,
				DBHost:         dbHost,
				DBUser:         dbUser,
				DBName:         dbName,
				DBPort:         dbPort,
			})
			opsPrintStore("db-health", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS SSO profile")
	cmd.Flags().StringVar(&kubectlCtx, "kubectl-context", "", "kubectl context")
	cmd.Flags().StringVar(&dbHost, "db-host", "", "Database hostname")
	cmd.Flags().StringVar(&dbUser, "db-user", "", "Database user")
	cmd.Flags().StringVar(&dbName, "db-name", "", "Database name")
	cmd.Flags().StringVar(&dbPort, "db-port", "5432", "Database port")
	return cmd
}

// ── k8s-status ────────────────────────────────────────────────────────────────

func newOpsK8sStatusCmd() *cobra.Command {
	var namespace, deployment, kubectlCtx, labelSelector string
	cmd := &cobra.Command{
		Use:   "k8s-status",
		Short: "Check Kubernetes deployment health (pods, restarts, image)",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := ops.CheckK8sStatus(ops.K8sConfig{
				KubectlContext: kubectlCtx,
				Namespace:      namespace,
				Deployment:     deployment,
				LabelSelector:  labelSelector,
			})
			opsPrintStore("k8s-status", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&deployment, "deployment", "", "Deployment name")
	cmd.Flags().StringVar(&kubectlCtx, "kubectl-context", "", "kubectl context")
	cmd.Flags().StringVar(&labelSelector, "label-selector", "", "Label selector (e.g. app=fusca-api)")
	return cmd
}

// ── kafka-status ──────────────────────────────────────────────────────────────

func newOpsKafkaStatusCmd() *cobra.Command {
	var namespace, deployment, kubectlCtx, topic, consumerGroup, window, source string
	cmd := &cobra.Command{
		Use:   "kafka-status",
		Short: "Check Kafka consumer lag via log analysis or Datadog",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := ops.CheckKafkaStatus(ops.KafkaConfig{
				KubectlContext: kubectlCtx,
				Namespace:      namespace,
				Deployment:     deployment,
				Topic:          topic,
				ConsumerGroup:  consumerGroup,
				Window:         window,
				Source:         source,
			})
			opsPrintStore("kafka-status", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&deployment, "deployment", "", "Kafka consumer deployment name")
	cmd.Flags().StringVar(&kubectlCtx, "kubectl-context", "", "kubectl context")
	cmd.Flags().StringVar(&topic, "topic", "", "Kafka topic")
	cmd.Flags().StringVar(&consumerGroup, "consumer-group", "", "Kafka consumer group")
	cmd.Flags().StringVar(&window, "window", "10m", "Analysis window (e.g. 10m, 1h)")
	cmd.Flags().StringVar(&source, "source", "logs", "Source: logs | datadog")
	return cmd
}

// ── logs-analyze ──────────────────────────────────────────────────────────────

func newOpsLogsAnalyzeCmd() *cobra.Command {
	var filePath, patternsCSV string
	cmd := &cobra.Command{
		Use:   "logs-analyze",
		Short: "Analyze a log file for anomalies using heuristic patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			patterns := []string{"all"}
			if patternsCSV != "" {
				parts := strings.Split(patternsCSV, ",")
				patterns = make([]string, 0, len(parts))
				for _, p := range parts {
					if p = strings.TrimSpace(p); p != "" {
						patterns = append(patterns, p)
					}
				}
			}
			r := ops.CheckLogsAnalyze(ops.LogsConfig{
				FilePath: filePath,
				Patterns: patterns,
			})
			opsPrintStore("logs-analyze", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "-", "Log file path (or \"-\" for stdin)")
	cmd.Flags().StringVar(&patternsCSV, "patterns", "", "Comma-separated pattern IDs: kafka,lock,oom,slow-query,connection,panic,all")
	return cmd
}

// ── plan new ──────────────────────────────────────────────────────────────────

func newOpsPlanNewCmd() *cobra.Command {
	var templateID, scenario, namespace, deployment, profile, output string
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create an ops response plan from template or blank",
		Long: `Create an ops response plan and save it to --output (default: .workflow/ops/plan.md).

Built-in templates: health-check | lock-contention | kafka-recovery | deploy-rollback

Examples:
  wtb ops plan new --template health-check --scenario "CDC lag" --namespace fusca
  wtb ops plan new --scenario "custom recovery" --output /tmp/my-plan.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if scenario == "" && templateID == "" {
				return fmt.Errorf("--scenario is required (or use --template to pick a built-in)")
			}
			if scenario == "" {
				scenario = templateID
			}

			vars := map[string]string{
				"namespace":  namespace,
				"deployment": deployment,
				"aws-profile": profile,
			}

			var plan ops.Plan
			if templateID != "" {
				p, ok := ops.NewPlanFromTemplate(templateID, scenario, vars)
				if !ok {
					tpls := ops.ListTemplates()
					ids := make([]string, len(tpls))
					for i, t := range tpls {
						ids[i] = t.ID
					}
					return fmt.Errorf("template %q not found — available: %s", templateID, strings.Join(ids, ", "))
				}
				plan = p
			} else {
				plan = ops.NewBlankPlan(scenario, scenario, "medium")
			}

			planPath := output
			if planPath == "" {
				planPath = ".workflow/ops/plan.md"
			}
			if err := os.MkdirAll(filepath.Dir(planPath), 0755); err == nil {
				data, err := ops.MarshalPlan(plan)
				if err == nil {
					if err := os.WriteFile(planPath, data, 0644); err != nil {
						fmt.Fprintf(os.Stderr, "⚠ could not save plan: %v\n", err)
					} else {
						fmt.Printf("✓ plan saved: %s\n", planPath)
					}
				}
			}

			fmt.Println(ops.PlanSummary(plan))
			return nil
		},
	}
	cmd.Flags().StringVar(&templateID, "template", "", "Template ID: health-check | lock-contention | kafka-recovery | deploy-rollback")
	cmd.Flags().StringVar(&scenario, "scenario", "", "Scenario description (required for blank plan)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace (injected into template vars)")
	cmd.Flags().StringVar(&deployment, "deployment", "", "Deployment name (injected into template vars)")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS SSO profile")
	cmd.Flags().StringVar(&output, "output", "", "Output path (default: .workflow/ops/plan.md)")
	return cmd
}

// ── plan show ─────────────────────────────────────────────────────────────────

func newOpsPlanShowCmd() *cobra.Command {
	var planPath string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display summary of an ops plan file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if planPath == "" {
				planPath = ".workflow/ops/plan.md"
			}
			data, err := os.ReadFile(planPath)
			if err != nil {
				return fmt.Errorf("plan not found at %s: %w", planPath, err)
			}
			plan, err := ops.UnmarshalPlan(data)
			if err != nil {
				return fmt.Errorf("failed to parse plan: %w", err)
			}
			fmt.Println(ops.PlanSummary(plan))
			return nil
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "Plan file path (default: .workflow/ops/plan.md)")
	return cmd
}

// ── plan execute ──────────────────────────────────────────────────────────────

func newOpsPlanExecuteCmd() *cobra.Command {
	var planPath string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute an ops plan step-by-step with human checkpoints",
		Long: `Execute an ops plan loaded from --plan (default: .workflow/ops/plan.md).

owner=auto steps invoke the wtb binary automatically.
owner=human steps print the tool string and wait for [Enter] before proceeding.

Examples:
  wtb ops plan execute
  wtb ops plan execute --plan /tmp/my-plan.md --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if planPath == "" {
				planPath = ".workflow/ops/plan.md"
			}
			data, err := os.ReadFile(planPath)
			if err != nil {
				return fmt.Errorf("plan not found at %s: %w", planPath, err)
			}
			plan, err := ops.UnmarshalPlan(data)
			if err != nil {
				return fmt.Errorf("failed to parse plan: %w", err)
			}

			result := ops.ExecutePlan(plan, ops.PlanExecuteConfig{
				DryRun: dryRun,
			})
			opsPrint(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "Plan file path (default: .workflow/ops/plan.md)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print steps without executing")
	return cmd
}

// ── jira ──────────────────────────────────────────────────────────────────────

func newOpsJiraCmd() *cobra.Command {
	var jiraURL, email, token, project, jql string
	cmd := &cobra.Command{
		Use:   "jira",
		Short: "Check Jira for open P0/P1 issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := ops.CheckJira(ops.JiraConfig{
				URL: jiraURL, Email: email, APIToken: token, Project: project, JQL: jql,
			})
			opsPrintStore("jira", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&jiraURL, "url", "", "Jira URL (e.g. https://company.atlassian.net)")
	cmd.Flags().StringVar(&email, "email", "", "Jira user email")
	cmd.Flags().StringVar(&token, "token", "", "Jira API token")
	cmd.Flags().StringVar(&project, "project", "", "Jira project key")
	cmd.Flags().StringVar(&jql, "jql", "", "Custom JQL query (optional)")
	return cmd
}

// ── slack ─────────────────────────────────────────────────────────────────────

// resolveSlackToken resolves the Slack token using: flag → SLACK_BOT_TOKEN env → keychain.
func resolveSlackToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("SLACK_BOT_TOKEN"); v != "" {
		return v
	}
	// macOS keychain fallback
	out, err := runSilent("security", "find-generic-password", "-s", "workflow-slack-token", "-w")
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	return ""
}

// resolveSlackChannel resolves the channel using: flag → SLACK_CHANNEL env → session.yml slack-channel.
func resolveSlackChannel(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("SLACK_CHANNEL"); v != "" {
		return v
	}
	// session.yml fallback
	sessionPath := filepath.Join(os.Getenv("HOME"), ".workflow", "session.yml")
	if data, err := os.ReadFile(sessionPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "slack-channel:") {
				val := strings.TrimSpace(strings.TrimPrefix(line, "slack-channel:"))
				if val != "" && !strings.HasPrefix(val, "#") {
					return val
				}
			}
		}
	}
	return ""
}

func newOpsSlackCmd() *cobra.Command {
	var token, channel, query, window string
	cmd := &cobra.Command{
		Use:   "slack",
		Short: "Scan Slack channel for incident keywords",
		Long: `Scan a Slack channel for recent messages matching incident keywords.

Token resolution chain (first non-empty wins):
  1. --token flag
  2. SLACK_BOT_TOKEN env var   (set in ~/.zshrc for persistence)
  3. macOS Keychain            (service: workflow-slack-token)

Channel resolution chain:
  1. --channel flag
  2. SLACK_CHANNEL env var
  3. ~/.workflow/session.yml   (field: slack-channel)

Setup (one-time):
  security add-generic-password -s workflow-slack-token -a workflow_toolbox -w <token>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedToken := resolveSlackToken(token)
			resolvedChannel := resolveSlackChannel(channel)
			r := ops.CheckSlack(ops.SlackConfig{
				Token: resolvedToken, Channel: resolvedChannel, Query: query, Window: window,
			})
			opsPrintStore("slack", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Slack Bot token (or set SLACK_BOT_TOKEN env / keychain workflow-slack-token)")
	cmd.Flags().StringVar(&channel, "channel", "", "Slack channel ID (or set SLACK_CHANNEL env / session.yml slack-channel)")
	cmd.Flags().StringVar(&query, "query", "", "Keyword filter (optional)")
	cmd.Flags().StringVar(&window, "window", "1h", "Time window")
	cmd.AddCommand(newOpsSlackSetupCmd())
	return cmd
}

func newOpsSlackSetupCmd() *cobra.Command {
	var token, channel string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Store Slack token and channel in keychain + session.yml",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token != "" {
				out, err := runSilent("security", "add-generic-password",
					"-U", "-s", "workflow-slack-token", "-a", "workflow_toolbox", "-w", token)
				if err != nil {
					return fmt.Errorf("keychain write failed: %v — %s", err, out)
				}
				fmt.Println("✓ token armazenado no keychain (service: workflow-slack-token)")
			}
			if channel != "" {
				sessionPath := filepath.Join(os.Getenv("HOME"), ".workflow", "session.yml")
				data, err := os.ReadFile(sessionPath)
				if err != nil {
					return fmt.Errorf("session.yml não encontrado: %w", err)
				}
				content := string(data)
				newLine := "slack-channel: " + channel
				if strings.Contains(content, "slack-channel:") {
					lines := strings.Split(content, "\n")
					for i, l := range lines {
						if strings.HasPrefix(l, "slack-channel:") {
							lines[i] = newLine
						}
					}
					content = strings.Join(lines, "\n")
				} else {
					content += "\nslack-channel: " + channel + "\n"
				}
				if err := os.WriteFile(sessionPath, []byte(content), 0600); err != nil {
					return fmt.Errorf("erro ao atualizar session.yml: %w", err)
				}
				fmt.Printf("✓ canal %s salvo em ~/.workflow/session.yml\n", channel)
			}
			if token == "" && channel == "" {
				fmt.Println("Use --token e/ou --channel para configurar.")
				fmt.Println()
				fmt.Println("Exemplo:")
				fmt.Println("  wtb ops slack setup --token xoxb-... --channel C0123456")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Slack Bot token (armazenado no keychain)")
	cmd.Flags().StringVar(&channel, "channel", "", "Slack channel ID (salvo em session.yml)")
	return cmd
}

// credSpec holds the resolution strategy for a named credential (from session.yml credentials section).
type credSpec struct {
	Env      string `yaml:"env,omitempty"`
	Keychain string `yaml:"keychain,omitempty"`
}

// resolveOpsCredential resolves a credential by name using: flagValue → env → keychain.
// The resolution chain is declared in ~/.workflow/session.yml under credentials.<name>.
func resolveOpsCredential(name, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	sessionPath := filepath.Join(os.Getenv("HOME"), ".workflow", "session.yml")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return os.Getenv(name)
	}
	var cfg struct {
		Credentials map[string]credSpec `yaml:"credentials"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil || cfg.Credentials == nil {
		return os.Getenv(name)
	}
	spec, ok := cfg.Credentials[name]
	if !ok {
		return os.Getenv(name)
	}
	if spec.Env != "" {
		if v := os.Getenv(spec.Env); v != "" {
			return v
		}
	} else {
		// fallback: try the credential name itself as an env var
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	if spec.Keychain != "" {
		if out, err := runSilent("security", "find-generic-password", "-s", spec.Keychain, "-w"); err == nil {
			return strings.TrimSpace(out)
		}
	}
	return ""
}

// readSessionFields reads one or more flat key: "value" fields from a session.yml file.
// Returns a map of field → value (stripped of surrounding quotes).
func readSessionFields(sessionPath string, fields ...string) map[string]string {
	result := make(map[string]string, len(fields))
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		for _, field := range fields {
			if strings.HasPrefix(line, field+":") {
				val := strings.TrimSpace(strings.TrimPrefix(line, field+":"))
				result[field] = strings.Trim(val, `"`)
			}
		}
	}
	return result
}

// runSilent runs a command and returns stdout (ignoring stderr).
func runSilent(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return string(out), err
}

// ── websearch ─────────────────────────────────────────────────────────────────

func newOpsWebSearchCmd() *cobra.Command {
	var apiKey, cseID, query string
	var numResults int
	cmd := &cobra.Command{
		Use:   "websearch",
		Short: "Search web for relevant context via Google Custom Search",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := ops.CheckWebSearch(ops.WebSearchConfig{
				APIKey: apiKey, CSEID: cseID, Query: query, NumResults: numResults,
			})
			opsPrintStore("websearch", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Google API key")
	cmd.Flags().StringVar(&cseID, "cse-id", "", "Custom Search Engine ID")
	cmd.Flags().StringVar(&query, "query", "", "Search query")
	cmd.Flags().IntVar(&numResults, "num-results", 5, "Number of results")
	return cmd
}

// ── snowflake ─────────────────────────────────────────────────────────────────

func newOpsSnowflakeCmd() *cobra.Command {
	var account, user, password, warehouse, database, schema, query, mode string
	var gapRealMin float64
	cmd := &cobra.Command{
		Use:   "snowflake",
		Short: "Run a Snowflake query via snowsql and evaluate results",
		Long: `Run a Snowflake query or a built-in analysis mode.

Modes:
  (default)       run --query SQL and evaluate row count / latency
  warehouse-cost  SHOW WAREHOUSES + 24h metering history + gap heuristic

Password: set SNOWSQL_PWD env var (preferred) or --password flag (deprecated — visible in process list).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			base := ops.SnowflakeConfig{
				Account: account, User: user, Password: resolveOpsCredential("SNOWSQL_PWD", password),
				Warehouse: warehouse, Database: database, Schema: schema, Query: query,
			}
			var r ops.OpsResult
			switch mode {
			case "warehouse-cost":
				r = ops.CheckSnowflakeWarehouseCost(ops.WarehouseCostConfig{
					SnowflakeConfig: base,
					GapRealMin:      gapRealMin,
				})
			default:
				r = ops.CheckSnowflake(base)
			}
			opsPrintStore("snowflake", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Snowflake account")
	cmd.Flags().StringVar(&user, "user", "", "Snowflake user")
	cmd.Flags().StringVar(&password, "password", "", "Snowflake password (prefer SNOWSQL_PWD env var)")
	cmd.Flags().StringVar(&warehouse, "warehouse", "", "Snowflake warehouse")
	cmd.Flags().StringVar(&database, "database", "", "Snowflake database")
	cmd.Flags().StringVar(&schema, "schema", "", "Snowflake schema")
	cmd.Flags().StringVar(&query, "query", "", "SQL query to execute (default mode)")
	cmd.Flags().StringVar(&mode, "mode", "", "Analysis mode: warehouse-cost")
	cmd.Flags().Float64Var(&gapRealMin, "gap-real-min", 0, "Gap between syncs in minutes (schedule_min - avg_duration_min), used by warehouse-cost heuristic")
	return cmd
}

// ── montecarlo ────────────────────────────────────────────────────────────────

func newOpsMonteCarloCmd() *cobra.Command {
	var apiKey, apiToken, probeID, ruleUUID string
	cmd := &cobra.Command{
		Use:   "montecarlo",
		Short: "Check Monte Carlo data quality alerts",
		Long: `Check Monte Carlo for active data quality alerts.

Credential resolution (first non-empty wins):
  --api-key flag → session.yml montecarlo-api-key-id
  --api-token flag → credentials.MCD_API_TOKEN in session.yml (keychain: workflow-montecarlo-token)

Rule UUID resolution (first non-empty wins):
  --rule-uuid flag → session.yml montecarlo-rule-uuid

Setup (one-time):
  security add-generic-password -U -s workflow-montecarlo-token -a workflow_toolbox -w <secret>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionPath := filepath.Join(os.Getenv("HOME"), ".workflow", "session.yml")
			sessionFields := readSessionFields(sessionPath, "montecarlo-api-key-id", "montecarlo-rule-uuid")

			resolvedKey := apiKey
			if resolvedKey == "" {
				resolvedKey = sessionFields["montecarlo-api-key-id"]
			}
			resolvedUUID := ruleUUID
			if resolvedUUID == "" {
				resolvedUUID = sessionFields["montecarlo-rule-uuid"]
			}
			if resolvedUUID == "" {
				return fmt.Errorf("Monte Carlo: --rule-uuid is required (or set montecarlo-rule-uuid in session.yml)")
			}

			r := ops.CheckMonteCarlo(ops.MonteCarloConfig{
				APIKey:   resolvedKey,
				APIToken: resolveOpsCredential("MCD_API_TOKEN", apiToken),
				ProbeID:  probeID,
				Vars:     map[string]string{"RuleUUID": resolvedUUID},
			})
			opsPrintStore("montecarlo", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Monte Carlo API key ID (or set in session.yml montecarlo-api-key-id)")
	cmd.Flags().StringVar(&apiToken, "api-token", "", "Monte Carlo API token (or keychain: workflow-montecarlo-token)")
	cmd.Flags().StringVar(&probeID, "probe", "", "Probe to run (default: first in montecarlo.yml)")
	cmd.Flags().StringVar(&ruleUUID, "rule-uuid", "", "Custom rule UUID (or set in session.yml montecarlo-rule-uuid)")
	return cmd
}

// ── airbyte ───────────────────────────────────────────────────────────────────

func newOpsAirbyteCmd() *cobra.Command {
	var airbyteURL, apiKey, workspaceID string
	var mode, connectionID string
	var scheduleIntervalMin, autoSuspendMin float64
	var pageSize int
	cmd := &cobra.Command{
		Use:   "airbyte",
		Short: "Check Airbyte connection sync health",
		Long: `Query Airbyte for connection health, schedule map, or job profile analysis.

Modes:
  (default)      Check all connections sync health (latestSyncJobStatus)
  schedule-map   List all connections: type (cron/basic/manual), schedule, destination
  job-profile    Analyse job history for a specific connection: avg duration,
                 records/sync, success rate, and gap heuristic vs warehouse.

Examples:
  wtb ops airbyte --url https://airbyte.internal --workspace-id ws1
  wtb ops airbyte --url https://airbyte.internal --workspace-id ws1 --mode schedule-map
  wtb ops airbyte --url https://airbyte.internal --mode job-profile \
    --connection-id conn-abc --schedule-interval-min 15 --auto-suspend-min 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			base := ops.AirbyteConfig{URL: airbyteURL, APIKey: apiKey, WorkspaceID: workspaceID}
			var r ops.OpsResult
			switch mode {
			case "schedule-map":
				r = ops.CheckAirbyteScheduleMap(base)
			case "job-profile":
				r = ops.CheckAirbyteJobProfile(ops.AirbyteJobProfileConfig{
					AirbyteConfig:       base,
					ConnectionID:        connectionID,
					ScheduleIntervalMin: scheduleIntervalMin,
					AutoSuspendMin:      autoSuspendMin,
					PageSize:            pageSize,
				})
			default:
				r = ops.CheckAirbyte(base)
			}
			opsPrintStore("airbyte", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&airbyteURL, "url", "", "Airbyte API URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Airbyte API key (optional)")
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Airbyte workspace ID")
	cmd.Flags().StringVar(&mode, "mode", "", "Analysis mode: job-profile")
	cmd.Flags().StringVar(&connectionID, "connection-id", "", "Connection ID for job-profile mode")
	cmd.Flags().Float64Var(&scheduleIntervalMin, "schedule-interval-min", 0, "Schedule cadence in minutes (used to compute gap_real_min)")
	cmd.Flags().Float64Var(&autoSuspendMin, "auto-suspend-min", 0, "Snowflake warehouse AUTO_SUSPEND in minutes (optional, triggers gap heuristic)")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Number of jobs to fetch (default 50)")
	return cmd
}

// ── github ─────────────────────────────────────────────────────────────────────

func newOpsGitHubCmd() *cobra.Command {
	var repo, scope, query string
	var pr, limit int
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Check GitHub PRs, issues, CI, releases, and deployments via gh CLI",
		Long: `Query GitHub repository health using the gh CLI (already authenticated).

Scopes:
  pr           List open PRs or view a specific PR (--pr N)
  issues       List open issues (optionally filtered with --query)
  ci           List recent CI workflow runs
  releases     List recent releases
  deployments  List recent deployments via GitHub API

Examples:
  wtb ops github --repo Cobliteam/fusca --scope pr
  wtb ops github --repo Cobliteam/fusca --scope pr --pr 1078
  wtb ops github --repo Cobliteam/fusca --scope ci
  wtb ops github --repo Cobliteam/fusca --scope issues --query "label:bug"
  wtb ops github --repo Cobliteam/fusca --scope releases
  wtb ops github --repo Cobliteam/fusca --scope deployments`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := ops.CheckGitHub(ops.GitHubConfig{
				Repo:  repo,
				PR:    pr,
				Query: query,
				Scope: scope,
				Limit: limit,
			})
			opsPrintStore("github", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository (owner/name)")
	cmd.Flags().StringVar(&scope, "scope", "pr", "Scope: pr | issues | ci | releases | deployments")
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number (for scope=pr, view single PR)")
	cmd.Flags().StringVar(&query, "query", "", "Search query (for scope=issues)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max items to fetch")
	return cmd
}

// ── ci-sentinel ───────────────────────────────────────────────────────────────

func newOpsCISentinelCmd() *cobra.Command {
	var repo string
	var pr, intervalSec, timeoutMin int
	cmd := &cobra.Command{
		Use:   "ci-sentinel",
		Short: "Poll CI checks for a PR until all complete (pass or fail)",
		Long: `Poll GitHub CI checks for a specific PR until all checks reach a conclusion.
Prints progress every --interval seconds and exits when CI concludes.

Examples:
  wtb ops ci-sentinel --repo Cobliteam/fusca --pr 123
  wtb ops ci-sentinel --repo Cobliteam/fusca --pr 123 --interval 60 --timeout 30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := ops.CISentinelConfig{Repo: repo, PR: pr}
			deadline := time.Now().Add(time.Duration(timeoutMin) * time.Minute)

			for {
				r := ops.CheckCISentinel(cfg)
				running, _ := r.Data["running"].(int)
				// "warn" with running > 0 = still in progress; anything else = conclusive
				if r.Status != "warn" || running == 0 {
					opsPrintStore("ci-sentinel", r)
					return nil
				}
				if time.Now().After(deadline) {
					r.Status = "error"
					r.Signal += fmt.Sprintf(" — timeout após %dmin", timeoutMin)
					opsPrintStore("ci-sentinel", r)
					return nil
				}
				fmt.Printf("[%s] %s\n", time.Now().Format("15:04:05"), r.Signal)
				time.Sleep(time.Duration(intervalSec) * time.Second)
			}
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository (owner/name)")
	cmd.Flags().IntVar(&pr, "pr", 0, "PR number to watch")
	cmd.Flags().IntVar(&intervalSec, "interval", 30, "Poll interval in seconds")
	cmd.Flags().IntVar(&timeoutMin, "timeout", 60, "Timeout in minutes")
	return cmd
}

// ── test-diagnose ─────────────────────────────────────────────────────────────

func newOpsTestDiagnoseCmd() *cobra.Command {
	var path, module string
	cmd := &cobra.Command{
		Use:   "test-diagnose",
		Short: "Classify JUnit XML test failures as infra or code",
		Long: `Parse JUnit XML test reports and classify each failure as:
  infra — environment unavailable (Scylla/Kafka down, testcontainers, connection refused)
  code  — actual test defect (assertion failure, logic error)

Infra failures → warn (tests inconclusive, not a code bug)
Code failures  → critical (must fix before merge)

Examples:
  wtb ops test-diagnose --path webhook-sender/build/test-results/test --module webhook-sender
  wtb ops test-diagnose --path ./build/test-results/test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := ops.CheckTestDiagnose(ops.TestDiagnoseConfig{Path: path, Module: module})
			opsPrintStore("test-diagnose", r)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Directory containing JUnit XML reports (e.g. module/build/test-results/test)")
	cmd.Flags().StringVar(&module, "module", "", "Module name for display (default: dirname of --path)")
	return cmd
}

