package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
	"github.com/Cobliteam/workflow-toolkit/pkg/playbook"
)

func registerPlaybookTools(s *server.MCPServer, workflowHome string) {
	s.AddTool(toolPlaybookRun(), handlePlaybookRun(workflowHome))
	s.AddTool(toolPlaybookList(), handlePlaybookList(workflowHome))
}

// --- playbook_run ---

func toolPlaybookRun() mcplib.Tool {
	return mcplib.Tool{
		Name:        "playbook_run",
		Description: "Execute a YAML-driven investigation playbook (e.g. fusca-cdc-audit). Returns findings, causal chain, and markdown report.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"playbook_id":   prop("string", "Playbook ID (e.g. fusca-cdc-audit)"),
				"namespace":     prop("string", "Kubernetes namespace"),
				"kube_context":  prop("string", "kubectl context name"),
			},
			Required: []string{"playbook_id"},
		},
	}
}

func handlePlaybookRun(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		playbookID := req.GetString("playbook_id", "")
		if playbookID == "" {
			return mcplib.NewToolResultError("playbook_id is required"), nil
		}

		spec, err := playbook.LoadPlaybook(playbookID, workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("playbook %q not found: %v", playbookID, err)), nil
		}

		reg := infracontext.NewRegistry()
		exec := playbook.NewExecutor(reg)

		report, err := exec.Execute(ctx, spec, playbook.ExecuteOptions{
			Namespace:   req.GetString("namespace", ""),
			KubeContext: req.GetString("kube_context", ""),
		})
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("playbook execution error: %v", err)), nil
		}

		// Return markdown if available, otherwise JSON summary
		if report.Markdown != "" {
			return mcplib.NewToolResultText(report.Markdown), nil
		}

		data, _ := json.Marshal(report)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- playbook_list ---

func toolPlaybookList() mcplib.Tool {
	return mcplib.Tool{
		Name:        "playbook_list",
		Description: "List all available investigation playbooks with their ID, title, and description.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo_path": prop("string", "Project directory for custom playbooks (optional)"),
			},
		},
	}
}

func handlePlaybookList(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		repoPath := req.GetString("repo_path", workflowHome)

		configs, err := playbook.LoadPlaybookConfigs(repoPath)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to list playbooks: %v", err)), nil
		}

		type pbSummary struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Tags        []string `json:"tags,omitempty"`
		}

		var summaries []pbSummary
		for id, cfg := range configs {
			summaries = append(summaries, pbSummary{
				ID:          id,
				Title:       cfg.Title,
				Description: cfg.Description,
				Tags:        cfg.Tags,
			})
		}

		data, _ := json.Marshal(summaries)
		return mcplib.NewToolResultText(string(data)), nil
	}
}
