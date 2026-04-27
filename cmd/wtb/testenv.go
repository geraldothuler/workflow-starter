package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/Cobliteam/workflow-toolkit/pkg/testenv"
)

func newTestEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "testenv",
		Short: "Integration test environment orchestration",
	}
	cmd.AddCommand(newTestEnvRunCmd())
	cmd.AddCommand(newTestEnvDownCmd())
	cmd.AddCommand(newTestEnvStatusCmd())
	return cmd
}

func newTestEnvRunCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start services and run integration tests",
		Long: `Detects Docker runtime, resolves docker-compose, stops conflicting containers,
brings services up, waits for health, runs tests, and reports results.

Config: .workflow/testenv.yml in the repo root.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				repo, _ = repoRoot()
			}
			result, err := testenv.Run(repo)
			if err != nil {
				return err
			}
			printTestEnvResult(result)
			if result.ExitCode != 0 || result.Status == "error" {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	return cmd
}

func newTestEnvDownCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop docker-compose services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				repo, _ = repoRoot()
			}
			globalCfg, err := testenv.LoadTestEnvConfig()
			if err != nil {
				return err
			}
			repoCfg, err := testenv.LoadRepoTestEnvConfig(repo)
			if err != nil {
				return err
			}
			composeFile := testenv.DiscoverCompose(repo, globalCfg, repoCfg.Compose.File)
			if composeFile == "" {
				return fmt.Errorf("docker-compose file not found in %s", repo)
			}
			fmt.Printf("Stopping services: %s\n", composeFile)
			return testenv.ComposeDown(composeFile)
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	return cmd
}

func newTestEnvStatusCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Docker runtime and service health",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				repo, _ = repoRoot()
			}
			globalCfg, err := testenv.LoadTestEnvConfig()
			if err != nil {
				return err
			}
			runtime := testenv.DetectRuntime(globalCfg)
			if runtime == "" {
				for _, l := range testenv.RuntimeInstallInstructions(globalCfg) {
					fmt.Println(l)
				}
				return nil
			}
			fmt.Printf("Runtime: %s\n", runtime)

			repoCfg, err := testenv.LoadRepoTestEnvConfig(repo)
			if err != nil {
				return err
			}
			if len(repoCfg.Services) == 0 {
				fmt.Println("No services configured in .workflow/testenv.yml")
				return nil
			}
			fmt.Printf("Checking %d service(s)...\n", len(repoCfg.Services))
			results := testenv.WaitForServices(repoCfg.Services, &globalCfg.Health)
			for _, s := range results {
				fmt.Printf("  %-20s :%d  %s\n", s.Name, s.Port, s.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "repo root (default: current directory)")
	return cmd
}

func printTestEnvResult(r *testenv.TestEnvResult) {
	icon := map[string]string{"ok": "✓", "error": "✗", "timeout": "⏱", "skipped": "–"}[r.Status]
	if icon == "" {
		icon = "?"
	}
	fmt.Printf("\n%s testenv %s", icon, r.Signal)
	if r.Runtime != "" {
		fmt.Printf(" [%s]", r.Runtime)
	}
	if r.Duration != "" {
		fmt.Printf(" (%s)", r.Duration)
	}
	fmt.Println()

	if len(r.Services) > 0 {
		fmt.Println("\nServices:")
		for _, s := range r.Services {
			statusIcon := map[string]string{"healthy": "✓", "timeout": "✗", "unhealthy": "✗"}[s.Status]
			fmt.Printf("  %s %-20s :%d [%s]\n", statusIcon, s.Name, s.Port, s.Elapsed)
		}
	}

	if r.TestOutput != "" {
		fmt.Println("\nTest output:")
		fmt.Println(strings.Repeat("─", 60))
		fmt.Println(r.TestOutput)
		fmt.Println(strings.Repeat("─", 60))
	}

	if len(r.Actions) > 0 {
		fmt.Println("\nNext steps:")
		for _, a := range r.Actions {
			fmt.Println(" ", a)
		}
	}
}
