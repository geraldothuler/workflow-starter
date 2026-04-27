package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerStatusTools(s *server.MCPServer, workflowHome string) {
	s.AddTool(toolWorkflowStatus(), handleWorkflowStatus(workflowHome))
}

// --- workflow_status ---

func toolWorkflowStatus() mcplib.Tool {
	return mcplib.Tool{
		Name:        "workflow_status",
		Description: "Show the status of ~/.workflow/: security contract, active premises, 1:1 sessions.",
		InputSchema: mcplib.ToolInputSchema{Type: "object", Properties: map[string]any{}},
	}
}

func handleWorkflowStatus(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return mcplib.NewToolResultError("cannot determine home directory"), nil
		}

		personalDir := filepath.Join(home, ".workflow")
		var lines []string

		// Security contract
		contract := filepath.Join(personalDir, "security-contract.md")
		if info, err := os.Stat(contract); err == nil {
			lines = append(lines, fmt.Sprintf("security_contract: accepted (%s)", info.ModTime().Format(time.RFC3339)))
		} else {
			lines = append(lines, "security_contract: not found (run 'wtb security-accept')")
		}

		// Premises
		premisesPath := filepath.Join(personalDir, "1on1", "premises.md")
		if data, err := os.ReadFile(premisesPath); err == nil {
			count := 0
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "### P") {
					count++
				}
			}
			lines = append(lines, fmt.Sprintf("active_premises: %d", count))
		} else {
			lines = append(lines, "active_premises: 0 (premises.md not found)")
		}

		// Sessions
		sessionsDir := filepath.Join(personalDir, "1on1", "sessions")
		sessions, _ := filepath.Glob(filepath.Join(sessionsDir, "*.md"))
		lines = append(lines, fmt.Sprintf("sessions_1on1: %d", len(sessions)))
		if len(sessions) > 0 {
			lines = append(lines, fmt.Sprintf("last_session: %s", filepath.Base(sessions[len(sessions)-1])))
		}

		lines = append(lines, fmt.Sprintf("personal_dir: %s", personalDir))
		lines = append(lines, fmt.Sprintf("platform_dir: %s", workflowHome))

		return mcplib.NewToolResultText(strings.Join(lines, "\n")), nil
	}
}
