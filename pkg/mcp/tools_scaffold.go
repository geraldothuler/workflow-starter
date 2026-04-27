package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Cobliteam/workflow-toolkit/pkg/scaffold"
)

func registerScaffoldTools(s *server.MCPServer, workflowHome, defaultRepo string) {
	s.AddTool(toolWorkflowNew(), handleWorkflowNew(workflowHome, defaultRepo))
	s.AddTool(toolWorkflowIndex(), handleWorkflowIndex())
	s.AddTool(toolWorkflowList(), handleWorkflowList())
}

// --- workflow_new ---

func toolWorkflowNew() mcplib.Tool {
	return mcplib.Tool{
		Name:        "workflow_new",
		Description: "Create a new workflow artefact from the canonical template (incident, postmortem, review, 1on1).",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"type":      prop("string", "Workflow type: incident | postmortem | review | 1on1"),
				"context":   prop("string", "Short context label used in the artefact filename (e.g. kafka-lag, db-deadlock)"),
				"repo_path": prop("string", "Target repo path. 1on1 artefacts go to ~/.workflow/1on1/sessions/ regardless."),
			},
			Required: []string{"type", "context"},
		},
	}
}

func handleWorkflowNew(workflowHome, defaultRepo string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		wfType := req.GetString("type", "")
		wfContext := req.GetString("context", "")
		repoPath := req.GetString("repo_path", defaultRepo)

		if wfType == "" {
			return mcplib.NewToolResultError("type is required"), nil
		}
		if wfContext == "" {
			return mcplib.NewToolResultError("context is required"), nil
		}
		if !scaffold.IsValidType(wfType) {
			return mcplib.NewToolResultError(fmt.Sprintf("invalid type %q — valid: %s", wfType, strings.Join(scaffold.WorkflowTypes, ", "))), nil
		}

		date := time.Now().Format("2006-01-02")

		// Determine artefact destination
		var typeDir, artefactPath string
		if wfType == "1on1" {
			home, _ := os.UserHomeDir()
			typeDir = filepath.Join(home, ".workflow", "1on1", "sessions")
		} else {
			typeDir = filepath.Join(repoPath, "docs", "workflow", wfType)
		}

		nnn, err := scaffold.NextNNN(typeDir)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to determine next NNN: %v", err)), nil
		}

		if err := os.MkdirAll(typeDir, 0755); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to create directory: %v", err)), nil
		}

		if wfType == "1on1" {
			artefactPath = filepath.Join(typeDir, fmt.Sprintf("%s-%s.md", nnn, date))
		} else {
			artefactPath = filepath.Join(typeDir, fmt.Sprintf("%s-%s-%s.md", nnn, wfContext, date))
		}

		if _, err := os.Stat(artefactPath); err == nil {
			return mcplib.NewToolResultError(fmt.Sprintf("artefact already exists: %s", artefactPath)), nil
		}

		// Load and render template
		templatePath := filepath.Join(workflowHome, "use-cases", wfType, "template.md")
		tmplContent, err := os.ReadFile(templatePath)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("template not found at %s: %v", templatePath, err)), nil
		}

		rendered, err := scaffold.RenderTemplate(string(tmplContent), map[string]string{
			"NNN":     nnn,
			"Date":    date,
			"Context": wfContext,
			"Type":    wfType,
		})
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("template render error: %v", err)), nil
		}

		if err := os.WriteFile(artefactPath, []byte(rendered), 0644); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to write artefact: %v", err)), nil
		}

		return mcplib.NewToolResultText(fmt.Sprintf("created: %s\ntype: %s | context: %s | id: %s", artefactPath, wfType, wfContext, nnn)), nil
	}
}

// --- workflow_index ---

func toolWorkflowIndex() mcplib.Tool {
	return mcplib.Tool{
		Name:        "workflow_index",
		Description: "Rebuild INDEX.md files under <repo>/docs/workflow/ (per-type and master index).",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo_path": prop("string", "Target repo path (required)"),
			},
			Required: []string{"repo_path"},
		},
	}
}

func handleWorkflowIndex() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		repoPath, err := req.RequireString("repo_path")
		if err != nil {
			return mcplib.NewToolResultError("repo_path is required"), nil
		}

		workflowRoot := filepath.Join(repoPath, "docs", "workflow")
		if _, err := os.Stat(workflowRoot); os.IsNotExist(err) {
			return mcplib.NewToolResultError(fmt.Sprintf("docs/workflow/ not found in %s", repoPath)), nil
		}

		var updated []string
		var failed []string
		var presentTypes []string

		for _, wfType := range scaffold.WorkflowTypes {
			typeDir := filepath.Join(workflowRoot, wfType)
			if _, err := os.Stat(typeDir); os.IsNotExist(err) {
				continue
			}
			if err := scaffold.RebuildTypeIndex(typeDir, wfType); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", wfType, err))
				continue
			}
			presentTypes = append(presentTypes, wfType)
			updated = append(updated, wfType+"/INDEX.md")
		}

		if err := scaffold.RebuildMasterIndex(workflowRoot, presentTypes); err != nil {
			failed = append(failed, fmt.Sprintf("master: %v", err))
		} else {
			updated = append(updated, "INDEX.md")
		}

		result := fmt.Sprintf("updated: %s", strings.Join(updated, ", "))
		if len(failed) > 0 {
			result += fmt.Sprintf("\nerrors: %s", strings.Join(failed, "; "))
		}
		return mcplib.NewToolResultText(result), nil
	}
}

// --- workflow_list ---

func toolWorkflowList() mcplib.Tool {
	return mcplib.Tool{
		Name:        "workflow_list",
		Description: "List workflow artefacts found in <repo>/docs/workflow/.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo_path": prop("string", "Target repo path (required)"),
				"type":      prop("string", "Filter by type: incident | postmortem | review | 1on1 (optional)"),
			},
			Required: []string{"repo_path"},
		},
	}
}

func handleWorkflowList() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		repoPath, err := req.RequireString("repo_path")
		if err != nil {
			return mcplib.NewToolResultError("repo_path is required"), nil
		}

		wfType := req.GetString("type", "")
		workflowRoot := filepath.Join(repoPath, "docs", "workflow")

		typesToList := scaffold.WorkflowTypes
		if wfType != "" {
			if !scaffold.IsValidType(wfType) {
				return mcplib.NewToolResultError(fmt.Sprintf("invalid type %q", wfType)), nil
			}
			typesToList = []string{wfType}
		}

		type artefactEntry struct {
			Type      string   `json:"type"`
			Artefacts []string `json:"artefacts"`
		}

		var results []artefactEntry
		for _, t := range typesToList {
			typeDir := filepath.Join(workflowRoot, t)
			entries, err := scaffold.ListArtefacts(typeDir)
			if err != nil || len(entries) == 0 {
				continue
			}
			results = append(results, artefactEntry{Type: t, Artefacts: entries})
		}

		data, _ := json.Marshal(results)
		return mcplib.NewToolResultText(string(data)), nil
	}
}
