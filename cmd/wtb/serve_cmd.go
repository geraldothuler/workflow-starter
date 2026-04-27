package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Cobliteam/workflow-toolkit/pkg/wtbserver"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var daemon bool
	var mcpPort string
	var webhookPort string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the wtb daemon (CLI Unix socket + MCP streamable HTTP)",
		Long: `Starts the wtb daemon with two transports:

  1. Unix socket (~/.wtb/wtb.sock) — used by wtb CLI commands
  2. MCP streamable HTTP (localhost:7654/mcp) — used by Claude Code

Configure Claude Code in .claude/settings.json:
  {
    "mcpServers": {
      "workflow": { "url": "http://localhost:7654/mcp" }
    }
  }

The daemon is auto-started by any wtb command that needs DB access.
Use this command only for explicit lifecycle management.

Examples:
  wtb serve                             # foreground, Ctrl+C to stop
  wtb serve --mcp-port 7655             # custom MCP port
  wtb serve --webhook-port 7655         # enable webhook ingestion on :7655
  wtb serve --daemon                    # background (forked by auto-start)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := repoRoot()
			if err != nil {
				return fmt.Errorf("repoRoot: %w", err)
			}

			srv, err := wtbserver.New(root)
			if err != nil {
				return fmt.Errorf("init server: %w", err)
			}

			sockPath := wtbserver.SockPath()
			if err := srv.Start(sockPath, mcpPort); err != nil {
				return fmt.Errorf("start server: %w", err)
			}
			if webhookPort != "" {
				if err := srv.StartWebhook(webhookPort); err != nil {
					return fmt.Errorf("start webhook server: %w", err)
				}
			}

			if daemon {
				// Running as daemon forked by autoStart — keep alive silently.
				os.Stdout.Close()
				os.Stderr.Close()
				null, _ := os.Open(os.DevNull)
				os.Stdout = null
				os.Stderr = null
			} else {
				port := mcpPort
				if port == "" {
					port = wtbserver.DefaultMCPPort
				}
				fmt.Fprintf(os.Stderr, "wtb daemon started\n")
				fmt.Fprintf(os.Stderr, "  CLI socket : %s\n", sockPath)
				fmt.Fprintf(os.Stderr, "  MCP HTTP   : http://localhost:%s/mcp\n", port)
				if webhookPort != "" {
					fmt.Fprintf(os.Stderr, "  Webhooks   : http://localhost:%s/webhooks/{github,datadog}\n", webhookPort)
				}
				fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop.\n")
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit

			if !daemon {
				fmt.Fprintf(os.Stderr, "\nstopping wtb daemon...\n")
			}
			srv.Shutdown()
			return nil
		},
	}

	cmd.Flags().BoolVar(&daemon, "daemon", false, "Run as background daemon (suppress output)")
	cmd.Flags().StringVar(&mcpPort, "mcp-port", "", "TCP port for MCP streamable HTTP (default: 7654)")
	cmd.Flags().StringVar(&webhookPort, "webhook-port", "", "TCP port for webhook ingestion (disabled by default; expose via ngrok)")
	return cmd
}
