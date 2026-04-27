package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/docstore"
	"github.com/Cobliteam/workflow-toolkit/pkg/wtbserver"
	"github.com/spf13/cobra"
)

func newDocCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Workflow artefact store (discovery, savepoint, runbook, 1on1, ...)",
		Long: `Store and query workflow artefacts in docs.db.

  list    [--type T] [--since YYYY-MM-DD] [--repo R] [--tag T]
  search  <keyword>
  get     <id>
  add     --type T --title "..." [--date D] [--repo R] [--tag t1,t2] [--file path | --content "..."]
  append  <id> <content> [--section "## Heading"]
  update  <id> [--title T] [--tag t1,t2] [--repo R] [--date D] [--content-file path | --content "..."]
  import  <dir> --type T
  delete  <id>`,
	}

	cmd.AddCommand(
		newDocListCmd(),
		newDocSearchCmd(),
		newDocGetCmd(),
		newDocAddCmd(),
		newDocAppendCmd(),
		newDocUpdateCmd(),
		newDocImportCmd(),
		newDocDeleteCmd(),
		newDocTemplateCmd(),
	)
	return cmd
}

func openDocDB() (*docstore.DB, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	return docstore.Open(root)
}

// ── list ──────────────────────────────────────────────────────────────────────

func newDocListCmd() *cobra.Command {
	var docType, since, repo, tag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List artefacts with optional filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if docType != "" {
				params.Set("type", docType)
			}
			if since != "" {
				params.Set("since", since)
			}
			if repo != "" {
				params.Set("repo", repo)
			}
			if tag != "" {
				params.Set("tag", tag)
			}
			c := wtbserver.DefaultClient()
			if ok, err := c.Get("/doc/list", params); ok {
				return err
			}

			// Daemon not running — direct DB access.
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()

			docs, err := db.List(docstore.DocFilter{
				Type:  docType,
				Since: since,
				Repo:  repo,
				Tag:   tag,
			})
			if err != nil {
				return err
			}
			if len(docs) == 0 {
				fmt.Println("— sem artefatos para os filtros fornecidos.")
				return nil
			}
			printDocs(docs, false)
			return nil
		},
	}
	cmd.Flags().StringVar(&docType, "type", "", "Filter by type (discovery|savepoint|runbook|1on1|postmortem|incident|review)")
	cmd.Flags().StringVar(&since, "since", "", "Filter by doc_date >= YYYY-MM-DD")
	cmd.Flags().StringVar(&repo, "repo", "", "Filter by repo (substring)")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag (substring)")
	return cmd
}

// ── search ────────────────────────────────────────────────────────────────────

func newDocSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <keyword>",
		Short: "Full-text search on title, content, and tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := wtbserver.DefaultClient()
			if ok, err := c.Get("/doc/search", url.Values{"q": []string{args[0]}}); ok {
				return err
			}

			// Daemon not running — direct DB access.
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()

			docs, err := db.SearchAll(args[0])
			if err != nil {
				return err
			}
			if len(docs) == 0 {
				fmt.Printf("— sem resultados para %q.\n", args[0])
				return nil
			}
			printDocs(docs, false)
			return nil
		},
	}
}

// ── get ───────────────────────────────────────────────────────────────────────

func newDocGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Print full content of an artefact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := wtbserver.DefaultClient()
			if ok, err := c.Get("/doc/get", url.Values{"id": []string{args[0]}}); ok {
				return err
			}

			// Daemon not running — direct DB access.
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()

			d, err := db.Get(args[0])
			if err != nil {
				return err
			}
			printDocFull(d)
			return nil
		},
	}
}

// ── add ───────────────────────────────────────────────────────────────────────

func newDocAddCmd() *cobra.Command {
	var docType, title, date, repo, tagsFlag, file, contentInline string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new artefact",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file != "" && contentInline != "" {
				return fmt.Errorf("--file and --content are mutually exclusive")
			}

			content := contentInline
			if file != "" {
				b, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				content = string(b)
			}

			// Apply template when no content was supplied — happens before
			// the daemon check so both paths receive the populated content.
			if content == "" {
				if db, err := openDocDB(); err == nil {
					if tmpl, ok := db.GetTemplate(docType); ok {
						content = tmpl
					}
					db.Close()
				}
			}

			inp := docstore.DocInput{
				Type:    docType,
				Title:   title,
				DocDate: date,
				Repo:    repo,
				Tags:    splitCSV(tagsFlag),
				Content: content,
			}

			c := wtbserver.DefaultClient()
			var result struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}
			if ok, err := c.PostJSON("/doc/add", nil, inp, &result); ok {
				if err != nil {
					return err
				}
				fmt.Printf("✓ adicionado: %s — %s\n", result.ID, result.Title)
				return nil
			}

			// Daemon not running — direct DB access.
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()

			d, err := db.Add(inp)
			if err != nil {
				return err
			}
			fmt.Printf("✓ adicionado: %s — %s\n", d.ID, d.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&docType, "type", "", "Artefact type (discovery|savepoint|runbook|1on1|postmortem|incident) [required]")
	cmd.Flags().StringVarP(&title, "title", "t", "", "Title [required]")
	cmd.Flags().StringVar(&date, "date", "", "Document date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&repo, "repo", "", "Associated repo")
	cmd.Flags().StringVar(&tagsFlag, "tag", "", "Tags (comma-separated)")
	cmd.Flags().StringVar(&file, "file", "", "Read content from file (mutually exclusive with --content)")
	cmd.Flags().StringVar(&contentInline, "content", "", "Inline content string (mutually exclusive with --file)")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

// ── append ────────────────────────────────────────────────────────────────────

func newDocAppendCmd() *cobra.Command {
	var section string

	cmd := &cobra.Command{
		Use:   "append <id> <content>",
		Short: "Append content to the end of an artefact",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()

			content := args[1]
			if section != "" {
				// Insert before next heading after the given section
				d, err := db.Get(args[0])
				if err != nil {
					return err
				}
				content = insertAfterSection(d.Content, section, args[1])
				_, err = db.Update(args[0], docstore.DocUpdateInput{Content: content})
				if err != nil {
					return err
				}
				fmt.Printf("✓ atualizado: %s\n", args[0])
				return nil
			}

			d, err := db.Append(args[0], content)
			if err != nil {
				return err
			}
			fmt.Printf("✓ conteúdo adicionado: %s — %s\n", d.ID, d.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&section, "section", "", "Insert content before the next heading after this section")
	return cmd
}

// insertAfterSection inserts text after the line matching section heading,
// before the next heading. Falls back to appending at the end.
func insertAfterSection(content, section, addition string) string {
	lines := strings.Split(content, "\n")
	insertAt := -1
	inSection := false
	for i, line := range lines {
		if strings.TrimSpace(line) == strings.TrimSpace(section) {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "#") {
			insertAt = i
			break
		}
	}
	addition = "\n" + strings.TrimLeft(addition, "\n")
	if insertAt < 0 {
		return content + addition
	}
	before := strings.Join(lines[:insertAt], "\n")
	after := strings.Join(lines[insertAt:], "\n")
	return before + addition + "\n" + after
}

// ── update ────────────────────────────────────────────────────────────────────

func newDocUpdateCmd() *cobra.Command {
	var title, tagsFlag, repo, date, contentFile, contentInline string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update metadata or content of an existing artefact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if contentFile != "" && contentInline != "" {
				return fmt.Errorf("--content-file and --content are mutually exclusive")
			}
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()

			in := docstore.DocUpdateInput{
				Title:   title,
				Repo:    repo,
				DocDate: date,
			}
			if cmd.Flags().Changed("tag") {
				in.Tags = splitCSV(tagsFlag)
			}
			if contentFile != "" {
				b, err := os.ReadFile(contentFile)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				in.Content = string(b)
			} else if contentInline != "" {
				in.Content = contentInline
			}

			d, err := db.Update(args[0], in)
			if err != nil {
				return err
			}
			fmt.Printf("✓ atualizado: %s — %s\n", d.ID, d.Title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "New title")
	cmd.Flags().StringVar(&tagsFlag, "tag", "", "Replace tags (comma-separated)")
	cmd.Flags().StringVar(&repo, "repo", "", "New repo")
	cmd.Flags().StringVar(&date, "date", "", "New document date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&contentFile, "content-file", "", "Replace content with file contents (mutually exclusive with --content)")
	cmd.Flags().StringVar(&contentInline, "content", "", "Inline content string (mutually exclusive with --content-file)")
	return cmd
}

// ── import ────────────────────────────────────────────────────────────────────

func newDocImportCmd() *cobra.Command {
	var docType string
	var recursive bool

	cmd := &cobra.Command{
		Use:   "import <dir>",
		Short: "Batch import .md files from a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()

			dirs := []string{args[0]}
			if recursive {
				dirs = nil
				err := filepath.WalkDir(args[0], func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if d.IsDir() {
						dirs = append(dirs, path)
					}
					return nil
				})
				if err != nil {
					return err
				}
			}

			total := 0
			for _, dir := range dirs {
				n, errs := db.ImportDir(dir, docType)
				total += n
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "⚠ %v\n", e)
				}
			}
			fmt.Printf("✓ importados: %d artefatos\n", total)
			return nil
		},
	}
	cmd.Flags().StringVar(&docType, "type", "", "Artefact type to assign to all imported files [required]")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recurse into subdirectories")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

// ── delete ────────────────────────────────────────────────────────────────────

func newDocDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Soft-delete an artefact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()

			if err := db.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ deletado (soft): %s\n", args[0])
			return nil
		},
	}
}

// ── template ──────────────────────────────────────────────────────────────────

func newDocTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage document templates stored in docs.db",
		Long: `Read and write templates for doc types.

  get <type>                   print the template for a doc type
  set <type> --file <path>     load template content from file
  set <type> --content "..."   set template content inline`,
	}

	var contentInline, file string

	getCmd := &cobra.Command{
		Use:   "get <type>",
		Short: "Print the template for a doc type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()
			tmpl, ok := db.GetTemplate(args[0])
			if !ok {
				return fmt.Errorf("no template for type %q", args[0])
			}
			fmt.Println(tmpl)
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:   "set <type>",
		Short: "Create or update a template for a doc type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file != "" && contentInline != "" {
				return fmt.Errorf("--file and --content are mutually exclusive")
			}
			content := contentInline
			if file != "" {
				b, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				content = string(b)
			}
			if content == "" {
				return fmt.Errorf("one of --file or --content is required")
			}
			db, err := openDocDB()
			if err != nil {
				return err
			}
			defer db.Close()
			title := fmt.Sprintf("Template: %s", args[0])
			if err := db.SetTemplate(args[0], title, content); err != nil {
				return err
			}
			fmt.Printf("✓ template definido para tipo %q\n", args[0])
			return nil
		},
	}
	setCmd.Flags().StringVar(&file, "file", "", "read template from file")
	setCmd.Flags().StringVar(&contentInline, "content", "", "inline template content")

	cmd.AddCommand(getCmd, setCmd)
	return cmd
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printDocs(docs []docstore.Document, withContent bool) {
	for _, d := range docs {
		fmt.Fprintf(os.Stdout, "[%s] %s — %s", d.Type, d.ID, d.Title)
		if d.DocDate != "" {
			fmt.Fprintf(os.Stdout, " (%s)", d.DocDate)
		}
		fmt.Fprintln(os.Stdout)
		if d.Repo != "" {
			fmt.Fprintf(os.Stdout, "  repo: %s\n", d.Repo)
		}
		if len(d.Tags) > 0 {
			fmt.Fprintf(os.Stdout, "  tags: %s\n", strings.Join(d.Tags, ", "))
		}
		if withContent && d.Content != "" {
			lines := strings.SplitN(strings.TrimSpace(d.Content), "\n", 4)
			preview := strings.Join(lines[:min(3, len(lines))], " ")
			fmt.Fprintf(os.Stdout, "  preview: %s\n", preview)
		}
		fmt.Fprintln(os.Stdout)
	}
}

func printDocFull(d docstore.Document) {
	fmt.Fprintf(os.Stdout, "# %s\n", d.Title)
	fmt.Fprintf(os.Stdout, "id: %s | type: %s | date: %s", d.ID, d.Type, d.DocDate)
	if d.Repo != "" {
		fmt.Fprintf(os.Stdout, " | repo: %s", d.Repo)
	}
	if len(d.Tags) > 0 {
		fmt.Fprintf(os.Stdout, " | tags: %s", strings.Join(d.Tags, ", "))
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "---")
	fmt.Fprintln(os.Stdout, d.Content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
