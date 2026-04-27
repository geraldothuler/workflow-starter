// Package mcp exposes the workflow platform as an MCP (Model Context Protocol)
// server. Run via `wtb mcp-serve` — compatible with Claude Desktop and Claude Code.
package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates and configures the workflow MCP server with all 40 tools.
//
// workflowHome is the platform directory (~/workflow).
// repoPath is the default target repo for scaffold/index operations (may be
// overridden per-call via the repo_path argument).
func NewServer(workflowHome, repoPath string) *server.MCPServer {
	s := server.NewMCPServer(
		"workflow",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	registerRunTools(s, workflowHome, repoPath)
	registerScaffoldTools(s, workflowHome, repoPath)
	registerOpsTools(s)
	registerPlaybookTools(s, workflowHome)
	registerStatusTools(s, workflowHome)
	registerDocMemoryTools(s, workflowHome)
	registerRepoIndexTools(s, workflowHome)

	return s
}

// Start runs the MCP server in stdio mode (standard MCP transport).
func Start(s *server.MCPServer) error {
	return server.ServeStdio(s)
}

// prop builds a JSON-schema property map for ToolInputSchema.Properties.
func prop(typ, description string) map[string]any {
	return map[string]any{"type": typ, "description": description}
}
