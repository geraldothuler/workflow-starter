package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Cobliteam/workflow-toolkit/pkg/ops"
)

func registerOpsTools(s *server.MCPServer) {
	s.AddTool(toolOpsProbe(), handleOpsProbe())
	s.AddTool(toolOpsDbHealth(), handleOpsDbHealth())
	s.AddTool(toolOpsK8sStatus(), handleOpsK8sStatus())
	s.AddTool(toolOpsKafkaStatus(), handleOpsKafkaStatus())
	s.AddTool(toolOpsLogsAnalyze(), handleOpsLogsAnalyze())
	s.AddTool(toolOpsPlanNew(), handleOpsPlanNew())
	s.AddTool(toolOpsPlanShow(), handleOpsPlanShow())
	s.AddTool(toolOpsJira(), handleOpsJira())
	s.AddTool(toolOpsSlack(), handleOpsSlack())
	s.AddTool(toolOpsWebSearch(), handleOpsWebSearch())
	s.AddTool(toolOpsSnowflake(), handleOpsSnowflake())
	s.AddTool(toolOpsSnowflakeWarehouseCost(), handleOpsSnowflakeWarehouseCost())
	s.AddTool(toolOpsMonteCarlo(), handleOpsMonteCarlo())
	s.AddTool(toolOpsAirbyte(), handleOpsAirbyte())
	s.AddTool(toolOpsAirbyteScheduleMap(), handleOpsAirbyteScheduleMap())
	s.AddTool(toolOpsAirbyteJobProfile(), handleOpsAirbyteJobProfile())
	s.AddTool(toolOpsGitHub(), handleOpsGitHub())
}

// --- ops_probe ---

func toolOpsProbe() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_probe",
		Description: "Run a full environment probe: AWS auth + DB health + K8s status + Kafka status. Zero-LLM heuristics.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"namespace":       prop("string", "Kubernetes namespace"),
				"profile":         prop("string", "AWS SSO profile name"),
				"deployment":      prop("string", "Kubernetes deployment name"),
				"kubectl_context": prop("string", "kubectl context name"),
			},
		},
	}
}

func handleOpsProbe() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		namespace := req.GetString("namespace", "")
		profile := req.GetString("profile", "")
		deployment := req.GetString("deployment", "")
		kubectlCtx := req.GetString("kubectl_context", "")

		results := map[string]ops.OpsResult{}

		results["auth"] = ops.CheckAWSAuth(profile)
		results["k8s"] = ops.CheckK8sStatus(ops.K8sConfig{
			KubectlContext: kubectlCtx,
			Namespace:      namespace,
			Deployment:     deployment,
		})
		results["db"] = ops.CheckDBHealth(ops.DBHealthConfig{
			KubectlContext: kubectlCtx,
			Namespace:      namespace,
		})
		results["kafka"] = ops.CheckKafkaStatus(ops.KafkaConfig{
			KubectlContext: kubectlCtx,
			Namespace:      namespace,
			Deployment:     deployment,
		})

		data, _ := json.Marshal(results)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_db_health ---

func toolOpsDbHealth() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_db_health",
		Description: "Check PostgreSQL database health via kubectl (locks, WAL lag, long transactions, table bloat). Zero-LLM.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"db_host":        prop("string", "Database hostname"),
				"db_user":        prop("string", "Database user"),
				"db_name":        prop("string", "Database name"),
				"namespace":      prop("string", "Kubernetes namespace"),
				"kubectl_context": prop("string", "kubectl context name"),
			},
		},
	}
}

func handleOpsDbHealth() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckDBHealth(ops.DBHealthConfig{
			KubectlContext: req.GetString("kubectl_context", ""),
			Namespace:      req.GetString("namespace", ""),
			DBHost:         req.GetString("db_host", ""),
			DBUser:         req.GetString("db_user", ""),
			DBName:         req.GetString("db_name", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_k8s_status ---

func toolOpsK8sStatus() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_k8s_status",
		Description: "Check Kubernetes deployment health (pod readiness, restarts, image hash). Zero-LLM.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"namespace":       prop("string", "Kubernetes namespace"),
				"deployment":      prop("string", "Deployment name"),
				"kubectl_context": prop("string", "kubectl context name"),
			},
		},
	}
}

func handleOpsK8sStatus() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckK8sStatus(ops.K8sConfig{
			KubectlContext: req.GetString("kubectl_context", ""),
			Namespace:      req.GetString("namespace", ""),
			Deployment:     req.GetString("deployment", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_kafka_status ---

func toolOpsKafkaStatus() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_kafka_status",
		Description: "Check Kafka consumer lag health via log analysis or Datadog. Zero-LLM heuristics.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"namespace":       prop("string", "Kubernetes namespace"),
				"deployment":      prop("string", "Kafka consumer deployment name"),
				"topic":           prop("string", "Kafka topic"),
				"consumer_group":  prop("string", "Kafka consumer group"),
				"kubectl_context": prop("string", "kubectl context name"),
			},
		},
	}
}

func handleOpsKafkaStatus() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckKafkaStatus(ops.KafkaConfig{
			KubectlContext: req.GetString("kubectl_context", ""),
			Namespace:      req.GetString("namespace", ""),
			Deployment:     req.GetString("deployment", ""),
			Topic:          req.GetString("topic", ""),
			ConsumerGroup:  req.GetString("consumer_group", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_logs_analyze ---

func toolOpsLogsAnalyze() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_logs_analyze",
		Description: "Analyze a log file for anomalies using heuristic patterns (kafka, lock, oom, slow-query, connection, panic). Zero-LLM.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"logs_file": prop("string", "Path to log file (or \"-\" for stdin)"),
				"patterns":  prop("array", "Pattern IDs to check: kafka, lock, oom, slow-query, connection, panic, all"),
			},
			Required: []string{"logs_file"},
		},
	}
}

func handleOpsLogsAnalyze() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		logsFile := req.GetString("logs_file", "")
		if logsFile == "" {
			return mcplib.NewToolResultError("logs_file is required"), nil
		}

		patterns := req.GetStringSlice("patterns", []string{"all"})

		result := ops.CheckLogsAnalyze(ops.LogsConfig{
			FilePath: logsFile,
			Patterns: patterns,
		})

		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_plan_new ---

func toolOpsPlanNew() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_plan_new",
		Description: "Create an ops response plan from a built-in template (health-check, lock-contention, kafka-recovery, deploy-rollback) or blank.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"scenario":    prop("string", "Scenario description (required for blank plan)"),
				"template_id": prop("string", "Template ID: health-check | lock-contention | kafka-recovery | deploy-rollback"),
				"namespace":   prop("string", "Kubernetes namespace (injected into template vars)"),
				"deployment":  prop("string", "Deployment name (injected into template vars)"),
			},
			Required: []string{"scenario"},
		},
	}
}

func handleOpsPlanNew() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		scenario := req.GetString("scenario", "")
		if scenario == "" {
			return mcplib.NewToolResultError("scenario is required"), nil
		}

		templateID := req.GetString("template_id", "")
		namespace := req.GetString("namespace", "")
		deployment := req.GetString("deployment", "")

		vars := map[string]string{
			"namespace":  namespace,
			"deployment": deployment,
			"scenario":   scenario,
		}

		var plan ops.Plan
		if templateID != "" {
			p, ok := ops.NewPlanFromTemplate(templateID, scenario, vars)
			if !ok {
				templates := ops.ListTemplates()
				ids := make([]string, len(templates))
				for i, t := range templates {
					ids[i] = t.ID
				}
				return mcplib.NewToolResultError(fmt.Sprintf("template %q not found. Available: %s", templateID, strings.Join(ids, ", "))), nil
			}
			plan = p
		} else {
			plan = ops.NewBlankPlan(scenario, "", "medium")
		}

		data, _ := json.Marshal(plan)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_plan_show ---

func toolOpsPlanShow() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_plan_show",
		Description: "Display a summary of an ops plan. Pass plan JSON from ops_plan_new or a file path.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"plan_json": prop("string", "Plan JSON (output of ops_plan_new)"),
			},
			Required: []string{"plan_json"},
		},
	}
}

func handleOpsPlanShow() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		planJSON := req.GetString("plan_json", "")
		if planJSON == "" {
			return mcplib.NewToolResultError("plan_json is required"), nil
		}

		plan, err := ops.UnmarshalPlan([]byte(planJSON))
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("invalid plan JSON: %v", err)), nil
		}

		return mcplib.NewToolResultText(ops.PlanSummary(plan)), nil
	}
}

// --- ops_jira ---

func toolOpsJira() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_jira",
		Description: "Check Jira for open P0/P1 issues in a project. Zero-LLM heuristic.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url":       prop("string", "Jira URL (e.g. https://company.atlassian.net)"),
				"email":     prop("string", "Jira user email"),
				"api_token": prop("string", "Jira API token"),
				"project":   prop("string", "Jira project key"),
				"jql":       prop("string", "Custom JQL query (optional, overrides default)"),
			},
			Required: []string{"url", "email", "api_token"},
		},
	}
}

func handleOpsJira() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckJira(ops.JiraConfig{
			URL:      req.GetString("url", ""),
			Email:    req.GetString("email", ""),
			APIToken: req.GetString("api_token", ""),
			Project:  req.GetString("project", ""),
			JQL:      req.GetString("jql", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_slack ---

func toolOpsSlack() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_slack",
		Description: "Scan a Slack channel for incident keywords (critical, outage, down, error, alert). Zero-LLM heuristic.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"token":   prop("string", "Slack Bot token"),
				"channel": prop("string", "Slack channel ID"),
				"query":   prop("string", "Keyword filter (optional)"),
				"window":  prop("string", "Time window e.g. 1h (optional)"),
			},
			Required: []string{"token", "channel"},
		},
	}
}

func handleOpsSlack() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckSlack(ops.SlackConfig{
			Token:   req.GetString("token", ""),
			Channel: req.GetString("channel", ""),
			Query:   req.GetString("query", ""),
			Window:  req.GetString("window", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_websearch ---

func toolOpsWebSearch() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_websearch",
		Description: "Search the web for relevant context via Google Custom Search API. Zero-LLM.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"api_key":     prop("string", "Google API key"),
				"cse_id":      prop("string", "Custom Search Engine ID"),
				"query":       prop("string", "Search query"),
				"num_results": prop("number", "Number of results (default 5)"),
			},
			Required: []string{"api_key", "cse_id", "query"},
		},
	}
}

func handleOpsWebSearch() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckWebSearch(ops.WebSearchConfig{
			APIKey: req.GetString("api_key", ""),
			CSEID:  req.GetString("cse_id", ""),
			Query:  req.GetString("query", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_snowflake ---

func toolOpsSnowflake() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_snowflake",
		Description: "Run a Snowflake query via snowsql CLI and evaluate results. Zero-LLM.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"account":   prop("string", "Snowflake account identifier"),
				"user":      prop("string", "Snowflake user"),
				"password":  prop("string", "Snowflake password"),
				"warehouse": prop("string", "Snowflake warehouse (optional)"),
				"database":  prop("string", "Snowflake database (optional)"),
				"schema":    prop("string", "Snowflake schema (optional)"),
				"query":     prop("string", "SQL query to execute"),
			},
			Required: []string{"account", "user", "query"},
		},
	}
}

func handleOpsSnowflake() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckSnowflake(ops.SnowflakeConfig{
			Account:   req.GetString("account", ""),
			User:      req.GetString("user", ""),
			Password:  req.GetString("password", ""),
			Warehouse: req.GetString("warehouse", ""),
			Database:  req.GetString("database", ""),
			Schema:    req.GetString("schema", ""),
			Query:     req.GetString("query", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_snowflake_warehouse_cost ---

func toolOpsSnowflakeWarehouseCost() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_snowflake_warehouse_cost",
		Description: "Query Snowflake warehouse AUTO_SUSPEND config and 24h metering history. Applies gap heuristic: gap_real_min vs auto_suspend_min → warehouse always-on or suspends between syncs. Zero-LLM.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"account":      prop("string", "Snowflake account identifier"),
				"user":         prop("string", "Snowflake user"),
				"warehouse":    prop("string", "Warehouse name to analyse"),
				"gap_real_min": prop("number", "Gap between syncs in minutes (schedule_min - avg_duration_min). Used for heuristic signal."),
			},
			Required: []string{"account", "user", "warehouse"},
		},
	}
}

func handleOpsSnowflakeWarehouseCost() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckSnowflakeWarehouseCost(ops.WarehouseCostConfig{
			SnowflakeConfig: ops.SnowflakeConfig{
				Account:   req.GetString("account", ""),
				User:      req.GetString("user", ""),
				Warehouse: req.GetString("warehouse", ""),
			},
			GapRealMin: req.GetFloat("gap_real_min", 0),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_montecarlo ---

func toolOpsMonteCarlo() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_montecarlo",
		Description: "Check Monte Carlo data observability for active data quality alerts. Zero-LLM heuristic.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"api_key":  prop("string", "Monte Carlo API key"),
				"table_id": prop("string", "Filter alerts by table ID (optional)"),
			},
			Required: []string{"api_key"},
		},
	}
}

func handleOpsMonteCarlo() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckMonteCarlo(ops.MonteCarloConfig{
			APIKey:  req.GetString("api_key", ""),
			TableID: req.GetString("table_id", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_airbyte ---

func toolOpsAirbyte() mcplib.Tool {
	return mcplib.Tool{
		Name:        "ops_airbyte",
		Description: "Check Airbyte connection sync health (failed vs healthy syncs). Zero-LLM heuristic.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url":          prop("string", "Airbyte API URL"),
				"api_key":      prop("string", "Airbyte API key (optional)"),
				"workspace_id": prop("string", "Airbyte workspace ID"),
			},
			Required: []string{"url", "workspace_id"},
		},
	}
}

func handleOpsAirbyte() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckAirbyte(ops.AirbyteConfig{
			URL:         req.GetString("url", ""),
			APIKey:      req.GetString("api_key", ""),
			WorkspaceID: req.GetString("workspace_id", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_airbyte_schedule_map ---

func toolOpsAirbyteScheduleMap() mcplib.Tool {
	return mcplib.Tool{
		Name: "ops_airbyte_schedule_map",
		Description: `List all Airbyte connections in a workspace with their schedule type (cron/basic/manual),
cron expression or interval_min, destination name, and active/inactive status.
Heuristic: active manual connections (no automation) → warn; inactive connections → warn. Zero-LLM.`,
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url":          prop("string", "Airbyte API URL"),
				"api_key":      prop("string", "Airbyte API key (optional)"),
				"workspace_id": prop("string", "Airbyte workspace ID"),
			},
			Required: []string{"url", "workspace_id"},
		},
	}
}

func handleOpsAirbyteScheduleMap() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckAirbyteScheduleMap(ops.AirbyteConfig{
			URL:         req.GetString("url", ""),
			APIKey:      req.GetString("api_key", ""),
			WorkspaceID: req.GetString("workspace_id", ""),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_airbyte_job_profile ---

func toolOpsAirbyteJobProfile() mcplib.Tool {
	return mcplib.Tool{
		Name: "ops_airbyte_job_profile",
		Description: `Analyse Airbyte job history for a specific connection: avg_duration_min, avg_records, success_rate, gap_real_min.
If auto_suspend_min is provided, applies gap heuristic: gap_real > auto_suspend → warehouse suspends between syncs (ok);
gap_real ≤ auto_suspend → warehouse always-on (warn + ALTER WAREHOUSE suggestion). Zero-LLM heuristic.`,
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url":                   prop("string", "Airbyte API URL"),
				"api_key":               prop("string", "Airbyte API key (optional)"),
				"connection_id":         prop("string", "Airbyte connection ID to analyse"),
				"schedule_interval_min": prop("number", "Schedule cadence in minutes (used to compute gap_real_min = schedule - avg_duration)"),
				"auto_suspend_min":      prop("number", "Snowflake warehouse AUTO_SUSPEND in minutes (optional, triggers gap heuristic)"),
				"page_size":             prop("number", "Number of jobs to fetch (default 50)"),
			},
			Required: []string{"url", "connection_id"},
		},
	}
}

func handleOpsAirbyteJobProfile() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result := ops.CheckAirbyteJobProfile(ops.AirbyteJobProfileConfig{
			AirbyteConfig:       ops.AirbyteConfig{URL: req.GetString("url", ""), APIKey: req.GetString("api_key", "")},
			ConnectionID:        req.GetString("connection_id", ""),
			ScheduleIntervalMin: req.GetFloat("schedule_interval_min", 0),
			AutoSuspendMin:      req.GetFloat("auto_suspend_min", 0),
			PageSize:            int(req.GetFloat("page_size", 50)),
		})
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- ops_github ---

func toolOpsGitHub() mcplib.Tool {
	return mcplib.Tool{
		Name: "ops_github",
		Description: `Check GitHub repository health via gh CLI (PRs, issues, CI, releases, deployments). Zero-LLM heuristic.

Scopes:
- pr: List open PRs or view specific PR (set pr_number). Checks CI status, review state, staleness.
- issues: List open issues. Detects critical/bug labels.
- ci: List recent workflow runs. Detects default branch failures.
- releases: List recent releases. Detects pending drafts.
- deployments: List recent deployments across environments.`,
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo":      prop("string", "GitHub repository (owner/name, e.g. Cobliteam/fusca)"),
				"scope":     prop("string", "Scope: pr | issues | ci | releases | deployments (default: pr)"),
				"pr_number": prop("number", "PR number for scope=pr single view (optional, omit for list)"),
				"query":     prop("string", "Search query for scope=issues (optional)"),
				"limit":     prop("number", "Max items to fetch (default 10)"),
			},
			Required: []string{"repo"},
		},
	}
}

func handleOpsGitHub() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		repo := req.GetString("repo", "")
		if repo == "" {
			return mcplib.NewToolResultError("repo is required (owner/name)"), nil
		}

		pr := 0
		if v := req.GetString("pr_number", ""); v != "" {
			fmt.Sscanf(v, "%d", &pr)
		}
		limit := 10
		if v := req.GetString("limit", ""); v != "" {
			fmt.Sscanf(v, "%d", &limit)
		}

		result := ops.CheckGitHub(ops.GitHubConfig{
			Repo:  repo,
			Scope: req.GetString("scope", "pr"),
			PR:    pr,
			Query: req.GetString("query", ""),
			Limit: limit,
		})

		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	}
}
