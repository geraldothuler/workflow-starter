package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/monitor"
	"github.com/spf13/cobra"
)

func newMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor Slack channels for incident signals (zero-LLM)",
		Long: `Polls configured Slack channels for messages matching P0/P1 signal keywords.
All scoring is heuristic — zero LLM calls.

Subcommands:
  slack            Start polling loop
  slack keywords   Manage active keywords`,
	}

	cmd.AddCommand(
		newMonitorSlackCmd(),
	)
	return cmd
}

func newMonitorSlackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slack",
		Short: "Monitor Slack channels for incident signals",
	}

	cmd.AddCommand(
		newMonitorSlackPollCmd(),
		newMonitorSlackKeywordsCmd(),
	)
	return cmd
}

func newMonitorSlackPollCmd() *cobra.Command {
	var (
		channels []string
		interval int
		token    string
		repo     string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start Slack polling loop",
		Long: `Polls the configured Slack channels at a fixed interval, scoring each new message
against P0/P1 keyword rules from slack_monitor.yml.

On signal match:
  - Prints to terminal: HH:MM #channel [P0 92%] "text..." → wtb investigate --slack-thread <url>
  - Sends macOS notification (osascript) with the signal summary

Examples:
  wtb monitor slack start
  wtb monitor slack start --channels suporte,alarms --interval 60
  wtb monitor slack start --token xoxb-... --repo ~/Cobliteam/fusca`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				token = os.Getenv("SLACK_TOKEN")
			}
			if token == "" {
				return fmt.Errorf("Slack token required: use --token or set SLACK_TOKEN env var")
			}

			cfg, err := monitor.LoadSlackMonitorConfig(expandHome(repo))
			if err != nil {
				return fmt.Errorf("failed to load slack_monitor.yml: %w", err)
			}

			// CLI flags override config
			if len(channels) > 0 {
				cfg.Channels = channels
			}
			if interval > 0 {
				cfg.PollIntervalSeconds = interval
			}

			fmt.Printf("wtb monitor slack — polling %d channels every %ds\n",
				len(cfg.Channels), cfg.PollIntervalSeconds)
			fmt.Printf("Channels: %s\n", strings.Join(cfg.Channels, ", "))
			fmt.Println("Press Ctrl+C to stop.")
			fmt.Println()

			lastTS := map[string]string{}
			ticker := time.NewTicker(time.Duration(cfg.PollIntervalSeconds) * time.Second)
			defer ticker.Stop()

			// First poll immediately
			pollAndReport(cfg, token, lastTS)

			for range ticker.C {
				pollAndReport(cfg, token, lastTS)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&channels, "channels", nil, "Channels to monitor (overrides config)")
	cmd.Flags().IntVar(&interval, "interval", 0, "Poll interval in seconds (overrides config)")
	cmd.Flags().StringVar(&token, "token", "", "Slack Bot token (default: $SLACK_TOKEN)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repo path for local override (default: current dir)")
	return cmd
}

// pollAndReport runs one polling cycle and prints/notifies on signal matches.
func pollAndReport(cfg *monitor.SlackMonitorConfig, token string, lastTS map[string]string) {
	result := monitor.PollChannels(cfg, token, lastTS)
	if len(result.Signals) == 0 {
		return
	}

	for _, sig := range result.Signals {
		ts := time.Now().Format("15:04")
		line := fmt.Sprintf("%s #%s [%s %d%%] %q → wtb investigate --slack-thread %s",
			ts, sig.Channel,
			strings.ToUpper(sig.Level), sig.Score,
			sig.Text,
			sig.ThreadURL,
		)
		fmt.Println(line)

		// Update lastTS for this channel so we don't re-process
		lastTS[sig.Channel] = sig.Timestamp

		// macOS notification
		notify(sig)
	}
}

// notify sends a macOS notification using osascript.
// AppleScript does not support \" inside string literals — a bare " closes the
// string and the following \ becomes an unexpected token (syntax error -2741).
// Fix: split on " and rejoin with the AppleScript `quote` constant so the
// result is valid AppleScript regardless of the input text.
func notify(sig monitor.MonitorSignal) {
	title := fmt.Sprintf("[%s] #%s", strings.ToUpper(sig.Level), sig.Channel)
	script := fmt.Sprintf(
		"display notification %s with title %s",
		asString(sig.Text), asString(title),
	)
	_ = exec.Command("osascript", "-e", script).Run()
}

// asString wraps s in an AppleScript string literal, handling embedded double-
// quotes via concatenation with the built-in `quote` constant.
// Example: hello "world" → "hello " & quote & "world" & quote & ""
func asString(s string) string {
	parts := strings.Split(s, `"`)
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = `"` + p + `"`
	}
	return strings.Join(quoted, " & quote & ")
}

// newMonitorSlackKeywordsCmd manages keywords in the active config.
func newMonitorSlackKeywordsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keywords",
		Short: "Manage active signal keywords",
	}

	cmd.AddCommand(
		newKeywordsListCmd(),
		newKeywordsAddCmd(),
		newKeywordsRemoveCmd(),
	)
	return cmd
}

func newKeywordsListCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active keywords from slack_monitor.yml",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := monitor.LoadSlackMonitorConfig(expandHome(repo))
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Println("Active keywords (from merged config):")
			fmt.Println()
			for level, sig := range cfg.Signals {
				fmt.Printf("[%s] score=%d\n", strings.ToUpper(level), sig.Score)
				for _, kw := range sig.Keywords {
					fmt.Printf("  - %s\n", kw)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "Repo path for local override")
	return cmd
}

func newKeywordsAddCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "add <level> <keyword>",
		Short: "Add a keyword to a signal level in the personal override",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			level := strings.ToLower(args[0])
			keyword := args[1]

			cfg, err := monitor.LoadSlackMonitorConfig(expandHome(repo))
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			sig, ok := cfg.Signals[level]
			if !ok {
				return fmt.Errorf("unknown level %q — valid levels: %s",
					level, knownLevels(cfg))
			}

			for _, kw := range sig.Keywords {
				if strings.EqualFold(kw, keyword) {
					fmt.Printf("Keyword %q already exists in [%s]\n", keyword, level)
					return nil
				}
			}

			sig.Keywords = append(sig.Keywords, keyword)
			cfg.Signals[level] = sig

			if err := monitor.WritePersonalOverride(cfg); err != nil {
				return fmt.Errorf("failed to save override: %w", err)
			}

			fmt.Printf("✓ Added %q to [%s]\n", keyword, level)
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "Repo path for local override")
	return cmd
}

func newKeywordsRemoveCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "remove <level> <keyword>",
		Short: "Remove a keyword from a signal level in the personal override",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			level := strings.ToLower(args[0])
			keyword := args[1]

			cfg, err := monitor.LoadSlackMonitorConfig(expandHome(repo))
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			sig, ok := cfg.Signals[level]
			if !ok {
				return fmt.Errorf("unknown level %q", level)
			}

			var filtered []string
			found := false
			for _, kw := range sig.Keywords {
				if strings.EqualFold(kw, keyword) {
					found = true
					continue
				}
				filtered = append(filtered, kw)
			}

			if !found {
				return fmt.Errorf("keyword %q not found in [%s]", keyword, level)
			}

			sig.Keywords = filtered
			cfg.Signals[level] = sig

			if err := monitor.WritePersonalOverride(cfg); err != nil {
				return fmt.Errorf("failed to save override: %w", err)
			}

			fmt.Printf("✓ Removed %q from [%s]\n", keyword, level)
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "Repo path for local override")
	return cmd
}

func knownLevels(cfg *monitor.SlackMonitorConfig) string {
	levels := make([]string, 0, len(cfg.Signals))
	for l := range cfg.Signals {
		levels = append(levels, l)
	}
	return strings.Join(levels, ", ")
}
