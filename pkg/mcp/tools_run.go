package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Cobliteam/workflow-toolkit/pkg/chain"
	"github.com/Cobliteam/workflow-toolkit/pkg/runner"
)

func registerRunTools(s *server.MCPServer, workflowHome, repoPath string) {
	s.AddTool(toolWorkflowRun(), handleWorkflowRun(workflowHome, repoPath))
	s.AddTool(toolWorkflowListUseCases(), handleWorkflowListUseCases(workflowHome))
}

// --- workflow_run ---

func toolWorkflowRun() mcplib.Tool {
	return mcplib.Tool{
		Name:        "workflow_run",
		Description: "Execute a workflow use-case pipeline (backlog, ops-response, investigation). Runs all steps defined in use-cases/<id>/definition.yml.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"use_case":  prop("string", "Use-case ID (e.g. backlog, ops-response, investigation)"),
				"inputs":    prop("object", "Key-value pairs passed as --input flags (e.g. {\"namespace\":\"prod\"})"),
				"dry_run":   prop("boolean", "Print steps without executing (default: false)"),
				"repo_path": prop("string", "Target repo path (default: current directory)"),
			},
			Required: []string{"use_case"},
		},
	}
}

func handleWorkflowRun(workflowHome, defaultRepo string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		useCaseID := req.GetString("use_case", "")
		if useCaseID == "" {
			return mcplib.NewToolResultError("use_case is required"), nil
		}

		dryRun := req.GetBool("dry_run", false)
		repoPath := req.GetString("repo_path", defaultRepo)

		// Convert inputs object to RunInputs
		inputs := runner.RunInputs{}
		if rawInputs := req.GetArguments()["inputs"]; rawInputs != nil {
			if m, ok := rawInputs.(map[string]any); ok {
				for k, v := range m {
					inputs[k] = fmt.Sprintf("%v", v)
				}
			}
		}

		def, err := runner.LoadDefinition(workflowHome, useCaseID)
		if err != nil {
			ids, _ := runner.ListUseCases(workflowHome)
			return mcplib.NewToolResultError(fmt.Sprintf("use-case %q not found. Available: %s", useCaseID, strings.Join(ids, ", "))), nil
		}

		opts := runner.RunOptions{
			DryRun:       dryRun,
			WorkflowHome: workflowHome,
			RepoPath:     repoPath,
		}

		r := runner.New(def, runner.DefaultRegistry(), opts)
		results, err := r.Run(inputs)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("pipeline error: %v", err)), nil
		}

		executed, skipped := 0, 0
		for _, res := range results {
			if res.Skipped {
				skipped++
			} else {
				executed++
			}
		}

		summary := fmt.Sprintf("use_case: %s\nexecuted: %d steps\nskipped: %d steps\nstatus: ok", useCaseID, executed, skipped)
		if dryRun {
			summary = fmt.Sprintf("[dry-run] %s (%s)\nsteps: %d\n", def.Name, def.ID, len(def.Steps))
			for _, s := range def.Steps {
				cmds := s.AllCommands()
				if len(cmds) == 0 {
					cmds = []string{"(engine: " + s.Engine + ")"}
				}
				summary += fmt.Sprintf("  [%s] %s — %s\n", s.Engine, s.Name, strings.Join(cmds, "; "))
			}
		}

		// Append chain options so the caller (Claude) can decide to follow the chain
		// by calling workflow_run again with the chosen use_case and the same inputs.
		if opts := chain.FormatOptions(def); opts != "" {
			summary += opts
		}

		return mcplib.NewToolResultText(summary), nil
	}
}

// --- workflow_list_use_cases ---

func toolWorkflowListUseCases() mcplib.Tool {
	return mcplib.Tool{
		Name:        "workflow_list_use_cases",
		Description: "List all available workflow use-cases with their ID, type, and description.",
		InputSchema: mcplib.ToolInputSchema{Type: "object", Properties: map[string]any{}},
	}
}

func handleWorkflowListUseCases(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		ids, err := runner.ListUseCases(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to list use-cases: %v", err)), nil
		}

		type ucSummary struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description"`
		}

		var summaries []ucSummary
		for _, id := range ids {
			def, err := runner.LoadDefinition(workflowHome, id)
			if err != nil {
				continue
			}
			summaries = append(summaries, ucSummary{
				ID:          def.ID,
				Name:        def.Name,
				Type:        def.Type,
				Description: def.Description,
			})
		}

		data, _ := json.Marshal(summaries)
		return mcplib.NewToolResultText(string(data)), nil
	}
}
