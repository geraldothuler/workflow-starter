package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/taskstore"
	"github.com/spf13/cobra"
)

func newBacklogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backlog",
		Short: "Operational task backlog (SQLite + rendered markdown)",
		Long: `Manage the operational task backlog stored in backlog.db.
Every write auto-regenerates backlog.md for LLM and git history.

  list     [--status pending|in-progress|blocked|done|all] [--repo R] [--tag T] [--jira J] [--since YYYY-MM-DD]
  search   <keyword>
  add      --title "..." [--repo r1,r2] [--tag t1,t2] [--jira J1,J2] [--desc "..."] [--date YYYY-MM-DD]
  update   <id> [--title T] [--desc D] [--date YYYY-MM-DD] [--repo r1,r2] [--tag t1,t2] [--jira J1,J2]
  done     <id>
  block    <id>
  start    <id>`,
	}

	cmd.AddCommand(
		newBacklogListCmd(),
		newBacklogSearchCmd(),
		newBacklogAddCmd(),
		newBacklogUpdateCmd(),
		newBacklogDoneCmd(),
		newBacklogBlockCmd(),
		newBacklogStartCmd(),
	)
	return cmd
}

func openTaskDB() (*taskstore.DB, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	return taskstore.Open(root)
}

// ── list ─────────────────────────────────────────────────────────────────────

func newBacklogListCmd() *cobra.Command {
	var status, repo, tag, jira, since string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entries with optional filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTaskDB()
			if err != nil {
				return err
			}
			defer db.Close()

			entries, err := db.List(taskstore.Filter{
				Status: status,
				Repo:   repo,
				Tag:    tag,
				Jira:   jira,
				Since:  since,
			})
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("— sem entradas para os filtros fornecidos.")
				return nil
			}
			printEntries(entries)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending|in-progress|blocked|done|all)")
	cmd.Flags().StringVar(&repo, "repo", "", "Filter by repo (substring)")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag (exact)")
	cmd.Flags().StringVar(&jira, "jira", "", "Filter by Jira ticket (exact)")
	cmd.Flags().StringVar(&since, "since", "", "Filter by date created >= YYYY-MM-DD")
	return cmd
}

// ── search ────────────────────────────────────────────────────────────────────

func newBacklogSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <keyword>",
		Short: "Full-text search on title and description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTaskDB()
			if err != nil {
				return err
			}
			defer db.Close()

			entries, err := db.Search(args[0])
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Printf("— sem resultados para %q.\n", args[0])
				return nil
			}
			printEntries(entries)
			return nil
		},
	}
}

// ── add ───────────────────────────────────────────────────────────────────────

func newBacklogAddCmd() *cobra.Command {
	var title, desc, date, doneDate, reposFlag, tagsFlag, jiraFlag string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTaskDB()
			if err != nil {
				return err
			}
			defer db.Close()

			e, err := db.Add(taskstore.EntryInput{
				Title:       title,
				Description: desc,
				DateTarget:  date,
				DateDone:    doneDate,
				Repos:       splitCSV(reposFlag),
				Tags:        splitCSV(tagsFlag),
				Jira:        splitCSV(jiraFlag),
			})
			if err != nil {
				return err
			}
			fmt.Printf("✓ adicionado: %s — %s\n", e.ID, e.Title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Entry title (required)")
	cmd.Flags().StringVar(&desc, "desc", "", "Description")
	cmd.Flags().StringVar(&date, "date", "", "Target date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&reposFlag, "repo", "", "Repos (comma-separated)")
	cmd.Flags().StringVar(&tagsFlag, "tag", "", "Tags (comma-separated)")
	cmd.Flags().StringVar(&jiraFlag, "jira", "", "Jira tickets (comma-separated)")
	cmd.Flags().StringVar(&doneDate, "done-date", "", "Mark as done on this date (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

// ── update ────────────────────────────────────────────────────────────────────

func newBacklogUpdateCmd() *cobra.Command {
	var title, desc, date, reposFlag, tagsFlag, jiraFlag string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update title, description, date or relations of an entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTaskDB()
			if err != nil {
				return err
			}
			defer db.Close()

			in := taskstore.EntryUpdateInput{
				Title:       title,
				Description: desc,
				DateTarget:  date,
			}
			if cmd.Flags().Changed("repo") {
				in.Repos = splitCSV(reposFlag)
			}
			if cmd.Flags().Changed("tag") {
				in.Tags = splitCSV(tagsFlag)
			}
			if cmd.Flags().Changed("jira") {
				in.Jira = splitCSV(jiraFlag)
			}

			e, err := db.Update(args[0], in)
			if err != nil {
				return err
			}
			fmt.Printf("✓ atualizado: %s — %s\n", e.ID, e.Title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "New title")
	cmd.Flags().StringVar(&desc, "desc", "", "New description")
	cmd.Flags().StringVar(&date, "date", "", "New target date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&reposFlag, "repo", "", "Replace repos (comma-separated)")
	cmd.Flags().StringVar(&tagsFlag, "tag", "", "Replace tags (comma-separated)")
	cmd.Flags().StringVar(&jiraFlag, "jira", "", "Replace Jira tickets (comma-separated)")
	return cmd
}

// ── done / block / start ─────────────────────────────────────────────────────

func newBacklogDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark entry as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTaskDB()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.Done(args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ done: %s\n", args[0])
			return nil
		},
	}
}

func newBacklogBlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "block <id>",
		Short: "Mark entry as blocked",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTaskDB()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.Block(args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ blocked: %s\n", args[0])
			return nil
		},
	}
}

func newBacklogStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id>",
		Short: "Mark entry as in-progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTaskDB()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.Start(args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ in-progress: %s\n", args[0])
			return nil
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printEntries(entries []taskstore.Entry) {
	statusLabel := map[string]string{
		taskstore.StatusPending:    "pending",
		taskstore.StatusInProgress: "in-progress",
		taskstore.StatusBlocked:    "blocked ⚠",
		taskstore.StatusDone:       "done ✓",
	}
	for _, e := range entries {
		label := statusLabel[e.Status]
		if label == "" {
			label = e.Status
		}
		fmt.Fprintf(os.Stdout, "[%s] %s — %s\n", label, e.ID, e.Title)
		if len(e.Repos) > 0 {
			fmt.Fprintf(os.Stdout, "  repos: %s\n", strings.Join(e.Repos, ", "))
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(os.Stdout, "  tags:  %s\n", strings.Join(e.Tags, ", "))
		}
		if len(e.Jira) > 0 {
			fmt.Fprintf(os.Stdout, "  jira:  %s\n", strings.Join(e.Jira, ", "))
		}
		if e.DateTarget != "" {
			fmt.Fprintf(os.Stdout, "  alvo:  %s\n", e.DateTarget)
		}
		if e.Description != "" {
			lines := strings.SplitN(strings.TrimSpace(e.Description), "\n", 3)
			fmt.Fprintf(os.Stdout, "  desc:  %s\n", lines[0])
		}
		fmt.Fprintln(os.Stdout)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p := strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
