package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/Cobliteam/workflow-toolkit/pkg/dbops"
	"github.com/spf13/cobra"
)

func newDbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Consultas a banco de dados (PostgreSQL, Scylla, Snowflake) via VPN-first",
	}
	cmd.AddCommand(
		newDbQueryCmd(),
		newDbLsCmd(),
		newDbProbeCmd(),
	)
	return cmd
}

// wtb db query --repo <repo> --named <query> [--param k=v ...] [--sql <sql>] [--driver <d>] [--json]
func newDbQueryCmd() *cobra.Command {
	var (
		repo      string
		namedQ    string
		rawSQL    string
		driver    string
		params    []string
		jsonOut   bool
		prettyOut bool
	)

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Executa query nomeada ou SQL ad-hoc contra o banco de um repo",
		Example: `  wtb db query --repo maintenance --named reminders-overdue
  wtb db query --repo maintenance --named reminders-overdue --param days=14
  wtb db query --repo frentista --sql "SELECT count(*) FROM fuel_transactions" --driver postgres
  wtb db query --repo maintenance --named reminders-overdue --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				return fmt.Errorf("--repo é obrigatório")
			}
			if namedQ == "" && rawSQL == "" {
				return fmt.Errorf("use --named <query> ou --sql <sql>")
			}
			if rawSQL != "" && driver == "" {
				return fmt.Errorf("--driver é obrigatório com --sql (postgres | cassandra | snowflake)")
			}

			queriesDir := queriesDirPath()

			// Parse --param k=v pairs
			paramMap := map[string]string{}
			for _, p := range params {
				parts := strings.SplitN(p, "=", 2)
				if len(parts) == 2 {
					paramMap[parts[0]] = parts[1]
				}
			}

			var result *dbops.QueryResult
			var err error

			if namedQ != "" {
				result, err = dbops.Run(queriesDir, repo, namedQ, paramMap)
			} else {
				result, err = dbops.RunSQL(queriesDir, repo, driver, rawSQL, paramMap)
			}
			if err != nil {
				return err
			}

			if jsonOut || !prettyOut {
				// Envelope JSON — default output (LLM-friendly)
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Human-readable table
			printTable(result)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Nome do repo (ex: maintenance)")
	cmd.Flags().StringVar(&namedQ, "named", "", "Nome da query definida no YAML")
	cmd.Flags().StringVar(&rawSQL, "sql", "", "SQL ad-hoc")
	cmd.Flags().StringVar(&driver, "driver", "", "Driver para --sql: postgres | cassandra | snowflake")
	cmd.Flags().StringArrayVar(&params, "param", nil, "Parâmetro (--param key=value, repetível)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Força saída JSON")
	cmd.Flags().BoolVar(&prettyOut, "pretty", false, "Tabela legível no terminal")

	return cmd
}

// wtb db ls [--repo <repo>]
func newDbLsCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lista repos e queries disponíveis",
		RunE: func(cmd *cobra.Command, args []string) error {
			queriesDir := queriesDirPath()

			if repo != "" {
				queries, err := dbops.ListQueryNames(queriesDir, repo)
				if err != nil {
					return err
				}
				fmt.Printf("Queries disponíveis para %s:\n\n", repo)
				w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
				for _, q := range queries {
					fmt.Fprintf(w, "  %-30s\t%s\t[%s]\n", q.Name, q.Desc, q.Driver)
				}
				w.Flush()
				return nil
			}

			// List all repos
			entries, err := os.ReadDir(queriesDir)
			if err != nil {
				return fmt.Errorf("db-queries dir not found at %s", queriesDir)
			}
			fmt.Println("Repos com queries configuradas:")
			w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
					continue
				}
				repoName := strings.TrimSuffix(e.Name(), ".yml")
				queries, err := dbops.ListQueryNames(queriesDir, repoName)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "  %-35s\t%d queries\n", repoName, len(queries))
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Filtra por repo específico")
	return cmd
}

// wtb db probe [--repo <repo>]
func newDbProbeCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "probe",
		Short: "Testa conectividade VPN com os endpoints de banco dos repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			queriesDir := queriesDirPath()

			var repos []string
			if repo != "" {
				repos = []string{repo}
			} else {
				entries, _ := os.ReadDir(queriesDir)
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".yml") {
						repos = append(repos, strings.TrimSuffix(e.Name(), ".yml"))
					}
				}
			}

			w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
			fmt.Fprintln(w, "  REPO\tHOST\tPORTA\tSTATUS")
			fmt.Fprintln(w, "  ----\t----\t-----\t------")

			for _, r := range repos {
				rq, err := dbops.LoadQueries(queriesDir, r)
				if err != nil {
					continue
				}
				checked := map[string]bool{}
				for _, q := range rq.Queries {
					switch q.Driver {
					case "postgres":
						creds := dbops.LoadCached(r, "postgres")
						if creds == nil || checked[creds.Host] {
							continue
						}
						checked[creds.Host] = true
						status := "BLOCKED"
						if dbops.ProbeVPN(creds.Host, creds.Port) {
							status = "OK"
						}
						fmt.Fprintf(w, "  %-30s\t%-45s\t%s\t%s\n", r, creds.Host, creds.Port, status)
					case "cassandra", "scylla":
						host := "herbie-database.prod.aws.cobli.co"
						if checked[host] {
							continue
						}
						checked[host] = true
						status := "BLOCKED"
						if dbops.ProbeVPN(host, "9042") {
							status = "OK"
						}
						fmt.Fprintf(w, "  %-30s\t%-45s\t%s\t%s\n", r, host, "9042", status)
					}
				}
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Filtra por repo específico")
	return cmd
}

// ── helpers ───────────────────────────────────────────────────────────────────

func queriesDirPath() string {
	root := os.Getenv("WTB_REPO_ROOT")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, "workflow")
	}
	return filepath.Join(root, "db-queries")
}

func printTable(result *dbops.QueryResult) {
	if len(result.Rows) == 0 {
		fmt.Printf("(%s/%s) 0 rows — %dms\n", result.Repo, result.Query, result.ElapsedMs)
		return
	}

	fmt.Printf("(%s/%s) %d rows — %dms\n\n", result.Repo, result.Query, result.Count, result.ElapsedMs)

	w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
	// Header
	if len(result.Columns) > 0 {
		cols := make([]string, len(result.Columns))
		for i, c := range result.Columns {
			cols[i] = strings.ToUpper(c.Name)
		}
		fmt.Fprintln(w, "  "+strings.Join(cols, "\t"))
	}
	// Rows
	for _, row := range result.Rows {
		vals := make([]string, len(result.Columns))
		for i, c := range result.Columns {
			v := row[c.Name]
			if v == nil {
				vals[i] = "NULL"
			} else {
				vals[i] = fmt.Sprintf("%v", v)
			}
		}
		fmt.Fprintln(w, "  "+strings.Join(vals, "\t"))
	}
	w.Flush()
}
