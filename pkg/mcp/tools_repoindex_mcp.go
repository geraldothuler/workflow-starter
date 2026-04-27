package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Cobliteam/workflow-toolkit/pkg/repoindex"
)

func registerRepoIndexTools(s *server.MCPServer, workflowHome string) {
	s.AddTool(toolRepoShow(), handleRepoShow(workflowHome))
	s.AddTool(toolRepoList(), handleRepoList(workflowHome))
	s.AddTool(toolRepoStatus(), handleRepoStatusMCP(workflowHome))
	s.AddTool(toolRepoQuery(), handleRepoQuery(workflowHome))
	s.AddTool(toolRepoTopologyMCP(), handleRepoTopologyMCP(workflowHome))
	s.AddTool(toolRepoImpact(), handleRepoImpact(workflowHome))
	s.AddTool(toolRepoGrep(), handleRepoGrep(workflowHome))
	s.AddTool(toolRepoSimilar(), handleRepoSimilar(workflowHome))
}

func openRepoIndexRO(workflowHome string) (*repoindex.DB, error) {
	db, err := repoindex.OpenReadOnly(workflowHome)
	if err != nil {
		return nil, fmt.Errorf("failed to open repos.duckdb: %w", err)
	}
	return db, nil
}

// --- repo_show ---

func toolRepoShow() mcplib.Tool {
	return mcplib.Tool{
		Name:        "repo_show",
		Description: "Show the full indexed snapshot of a repo: handlers, models, events, APIs, config vars, Kafka topics.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo":    prop("string", "Repo name (e.g. fusca, iris, webhook)"),
				"section": prop("string", "Section to show: handlers | models | events | apis | config | kafka | all (default: all)"),
			},
			Required: []string{"repo"},
		},
	}
}

func handleRepoShow(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		repoName := req.GetString("repo", "")
		section := req.GetString("section", "all")
		if repoName == "" {
			return mcplib.NewToolResultError("repo is required"), nil
		}

		db, err := openRepoIndexRO(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer db.Close()

		snap, err := repoindex.GetSnapshot(db, repoName)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		if section == "all" {
			data, _ := json.Marshal(snap)
			return mcplib.NewToolResultText(string(data)), nil
		}

		cols, rows := repoindex.SnapshotSection(snap, section)
		if cols == nil {
			return mcplib.NewToolResultError(fmt.Sprintf("unknown section %q — valid: handlers, models, events, apis, config, kafka", section)), nil
		}
		return mcplib.NewToolResultText(repoindex.RenderTable(cols, rows)), nil
	}
}

// --- repo_list ---

func toolRepoList() mcplib.Tool {
	return mcplib.Tool{
		Name:        "repo_list",
		Description: "List all repos in the repoindex with their language, framework, and last indexed date.",
		InputSchema: mcplib.ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
		},
	}
}

func handleRepoList(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		db, err := openRepoIndexRO(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer db.Close()

		repos, err := repoindex.ListRepos(db)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
		}

		type repoSummary struct {
			Name          string `json:"name"`
			Lang          string `json:"lang"`
			Framework     string `json:"framework,omitempty"`
			Owner         string `json:"owner,omitempty"`
			LastIndexedAt string `json:"last_indexed_at,omitempty"`
		}

		var summaries []repoSummary
		for _, r := range repos {
			summaries = append(summaries, repoSummary{
				Name:          r.Name,
				Lang:          r.Lang,
				Framework:     r.Framework,
				Owner:         r.Owner,
				LastIndexedAt: r.LastIndexedAt,
			})
		}

		data, _ := json.Marshal(summaries)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- repo_status (MCP variant) ---

func toolRepoStatus() mcplib.Tool {
	return mcplib.Tool{
		Name:        "repo_status",
		Description: "Check staleness of the repoindex. Returns OK/STALE classification for each repo with suggestions to re-index.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"stale_days": prop("number", "Days threshold to consider a repo stale (default: 30)"),
			},
		},
	}
}

func handleRepoStatusMCP(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		staleDays := int(req.GetFloat("stale_days", 30))

		db, err := openRepoIndexRO(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer db.Close()

		report, err := repoindex.CheckStatus(db, staleDays)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("status check failed: %v", err)), nil
		}

		return mcplib.NewToolResultText(repoindex.RenderStatus(report)), nil
	}
}

// --- repo_query ---

func toolRepoQuery() mcplib.Tool {
	return mcplib.Tool{
		Name:        "repo_query",
		Description: "Execute a raw SQL query against the repoindex DuckDB. Tables: repos, handlers, models, events, external_apis, config_vars, kafka_topics.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"sql": prop("string", "SQL SELECT query to run against repos.duckdb"),
			},
			Required: []string{"sql"},
		},
	}
}

func handleRepoQuery(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		query := req.GetString("sql", "")
		if query == "" {
			return mcplib.NewToolResultError("sql is required"), nil
		}

		db, err := openRepoIndexRO(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer db.Close()

		cols, rows, err := repoindex.QueryRows(db, query)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
		}

		return mcplib.NewToolResultText(repoindex.RenderTable(cols, rows)), nil
	}
}

// --- repo_topology ---

func toolRepoTopologyMCP() mcplib.Tool {
	return mcplib.Tool{
		Name:        "repo_topology",
		Description: "Show the Kafka topic graph: which repos produce to which topics and which consume from them.",
		InputSchema: mcplib.ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
		},
	}
}

func handleRepoTopologyMCP(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		db, err := openRepoIndexRO(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer db.Close()

		cols, rows, err := repoindex.QueryRows(db, `
			SELECT kt.topic_name, kt.role, r.name AS repo
			FROM kafka_topics kt
			JOIN repos r ON kt.repo_id = r.id
			ORDER BY kt.topic_name, kt.role, r.name
		`)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("topology query failed: %v", err)), nil
		}

		return mcplib.NewToolResultText(repoindex.RenderTable(cols, rows)), nil
	}
}

// --- repo_impact ---

func toolRepoImpact() mcplib.Tool {
	return mcplib.Tool{
		Name:        "repo_impact",
		Description: "Analyze the impact of combining or changing repos: shared Kafka topics, common DB tables, shared HTTP endpoints.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repos": prop("string", "Comma-separated repo names to analyze (e.g. fusca,iris)"),
			},
			Required: []string{"repos"},
		},
	}
}

func handleRepoImpact(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		reposStr := req.GetString("repos", "")
		if reposStr == "" {
			return mcplib.NewToolResultError("repos is required"), nil
		}

		names := strings.Split(reposStr, ",")
		for i, n := range names {
			names[i] = strings.TrimSpace(n)
		}

		db, err := openRepoIndexRO(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer db.Close()

		placeholders := make([]string, len(names))
		args := make([]any, len(names))
		for i, n := range names {
			placeholders[i] = "?"
			args[i] = n
		}
		inClause := strings.Join(placeholders, ",")

		kafkaCols, kafkaRows, err := repoindex.QueryRows(db, fmt.Sprintf(`
			SELECT kt.topic_name, kt.role, r.name
			FROM kafka_topics kt
			JOIN repos r ON kt.repo_id = r.id
			WHERE r.name IN (%s)
			ORDER BY kt.topic_name, kt.role
		`, inClause), args...)
		if err != nil {
			kafkaRows = nil
		}

		var out strings.Builder
		out.WriteString(fmt.Sprintf("Impact analysis: %s\n\n", strings.Join(names, " + ")))
		if len(kafkaRows) > 0 {
			out.WriteString("Kafka topics:\n")
			out.WriteString(repoindex.RenderTable(kafkaCols, kafkaRows))
		} else {
			out.WriteString("No shared Kafka topics found.\n")
		}

		return mcplib.NewToolResultText(out.String()), nil
	}
}

// --- repo_grep ---

func toolRepoGrep() mcplib.Tool {
	return mcplib.Tool{
		Name:        "repo_grep",
		Description: "Search for a keyword across all repos in the index (handler names, models, events, API paths, config keys).",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"keyword": prop("string", "Keyword to search across handlers, models, events, APIs, and config vars"),
			},
			Required: []string{"keyword"},
		},
	}
}

func handleRepoGrep(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		keyword := req.GetString("keyword", "")
		if keyword == "" {
			return mcplib.NewToolResultError("keyword is required"), nil
		}

		db, err := openRepoIndexRO(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer db.Close()

		like := "%" + keyword + "%"
		args := []any{like, like, like, like, like, like}

		cols, rows, err := repoindex.QueryRows(db, `
			SELECT 'handler' AS kind, r.name AS repo, h.name AS match, h.trigger_type AS detail
			FROM handlers h JOIN repos r ON h.repo_id = r.id WHERE h.name ILIKE ?
			UNION ALL
			SELECT 'model', r.name, m.name, m.table_name
			FROM models m JOIN repos r ON m.repo_id = r.id WHERE m.name ILIKE ?
			UNION ALL
			SELECT 'event', r.name, e.name, e.direction
			FROM events e JOIN repos r ON e.repo_id = r.id WHERE e.name ILIKE ?
			UNION ALL
			SELECT 'api', r.name, ea.endpoint, ea.method
			FROM external_apis ea JOIN repos r ON ea.repo_id = r.id WHERE ea.endpoint ILIKE ?
			UNION ALL
			SELECT 'config', r.name, cv.key, cv.default_value
			FROM config_vars cv JOIN repos r ON cv.repo_id = r.id WHERE cv.key ILIKE ?
			UNION ALL
			SELECT 'kafka', r.name, kt.topic_name, kt.role
			FROM kafka_topics kt JOIN repos r ON kt.repo_id = r.id WHERE kt.topic_name ILIKE ?
			ORDER BY kind, repo
		`, args...)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("grep failed: %v", err)), nil
		}

		if len(rows) == 0 {
			return mcplib.NewToolResultText(fmt.Sprintf("no matches for %q", keyword)), nil
		}

		return mcplib.NewToolResultText(repoindex.RenderTable(cols, rows)), nil
	}
}

// --- repo_similar ---

func toolRepoSimilar() mcplib.Tool {
	return mcplib.Tool{
		Name:        "repo_similar",
		Description: "Find repos that share handlers, models, or Kafka topics with a given repo — useful for impact analysis.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo": prop("string", "Repo name to find similar repos for"),
			},
			Required: []string{"repo"},
		},
	}
}

func handleRepoSimilar(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		repoName := req.GetString("repo", "")
		if repoName == "" {
			return mcplib.NewToolResultError("repo is required"), nil
		}

		db, err := openRepoIndexRO(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		defer db.Close()

		cols, rows, err := repoindex.QueryRows(db, `
			SELECT DISTINCT r2.name AS similar_repo, 'kafka' AS reason, kt1.topic_name AS shared
			FROM kafka_topics kt1
			JOIN repos r1 ON kt1.repo_id = r1.id AND r1.name = ?
			JOIN kafka_topics kt2 ON kt1.topic_name = kt2.topic_name AND kt2.repo_id != r1.id
			JOIN repos r2 ON kt2.repo_id = r2.id
			ORDER BY similar_repo
		`, repoName)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("similar query failed: %v", err)), nil
		}

		if len(rows) == 0 {
			return mcplib.NewToolResultText(fmt.Sprintf("no similar repos found for %q in Kafka topics", repoName)), nil
		}

		return mcplib.NewToolResultText(repoindex.RenderTable(cols, rows)), nil
	}
}
