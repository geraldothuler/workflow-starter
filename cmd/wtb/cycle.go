package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/cycles"
	"github.com/Cobliteam/workflow-toolkit/pkg/docstore"
	"github.com/spf13/cobra"
)

func newCycleCheckCmd() *cobra.Command {
	var save bool
	var repo string
	var jiraTicket string

	cmd := &cobra.Command{
		Use:   "cycle-check",
		Short: "Evaluate cycle-end signals and optionally generate a savepoint (zero-LLM)",
		Long: `Evaluates heuristic signals (git changes, tests, build, elapsed time, packages)
to determine if a development cycle has reached a stable point worth saving.

All decisions are deterministic — zero LLM calls.

Examples:
  wtb cycle-check
  wtb cycle-check --repo ~/Cobliteam/fusca
  wtb cycle-check --save
  wtb cycle-check --save --repo ~/workflow
  wtb cycle-check --save --jira-ticket SS-1234
  wtb cycle-check --save --jira-ticket auto`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := repo
			if repoPath == "" {
				var err error
				repoPath, err = repoRoot()
				if err != nil {
					return fmt.Errorf("failed to get current directory: %w", err)
				}
			}
			repoPath = expandHome(repoPath)

			result := cycles.CheckCycle(repoPath)

			// Print header
			fmt.Printf("\U0001F50D Cycle check — workflow platform\n")
			fmt.Println(strings.Repeat("─", 35))

			// Print each signal
			for _, sig := range result.Signals {
				icon := "✗"
				scoreStr := "+0"
				if sig.Passed {
					icon = "✔"
					scoreStr = fmt.Sprintf("+%d", sig.Weight)
				}
				fmt.Printf("  %s %-18s (%s)  %s\n", icon, sig.Name, scoreStr, sig.Detail)
			}

			fmt.Println()
			fmt.Printf("Score: %d/%d (threshold: %d)\n", result.Score, result.MaxScore, result.Threshold)

			if save {
				now := time.Now()
				content, err := cycles.RenderSavepoint(result, now)
				if err != nil {
					return fmt.Errorf("failed to render savepoint: %w", err)
				}

				db, err := docstore.Open(repoPath)
				if err != nil {
					return fmt.Errorf("failed to open docstore: %w", err)
				}
				defer db.Close()

				dateStr := now.Format("2006-01-02")
				timeStr := now.Format("15:04:05")
				title := fmt.Sprintf("Savepoint — %s %s", dateStr, timeStr)

				doc, err := db.Add(docstore.DocInput{
					Type:    "savepoint",
					Title:   title,
					DocDate: dateStr,
					Content: content,
				})
				if err != nil {
					return fmt.Errorf("failed to save to docstore: %w", err)
				}
				fmt.Printf("✓ Savepoint: %s\n", doc.ID)

				// Update last-savepoint marker for time_elapsed signal
				markerPath := repoPath + "/" + cycles.LastSavepointMarker
				_ = os.MkdirAll(repoPath+"/.workflow", 0755)
				_ = os.WriteFile(markerPath, []byte(doc.ID+"\n"), 0644)

				// Jira remote link — resolve ticket
				ticket := jiraTicket
				if ticket == "auto" || ticket == "" {
					ticket = detectJiraTicket(repoPath)
					if ticket != "" {
						fmt.Printf("  ticket detectado: %s\n", ticket)
					}
				}
				if ticket != "" && ticket != "auto" {
					if err := addJiraRemoteLink(ticket, doc.ID, repoPath); err != nil {
						fmt.Printf("⚠ Jira remote link: %v\n", err)
					} else {
						fmt.Printf("✓ Jira remote link: %s ← savepoint\n", ticket)
					}
				}
			} else if result.ShouldSavepoint {
				fmt.Println("→ Savepoint recommended. Run with --save to generate.")
			} else {
				name := friendlyName()
				if name != "" {
					fmt.Printf("👋 Ainda não, %s — continue iterando. Bom progresso!\n", name)
				} else {
					fmt.Println("👋 Ainda não — continue iterando. Bom progresso!")
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&save, "save", false, "Generate savepoint (bypasses threshold)")
	cmd.Flags().StringVar(&repo, "repo", "", "Path to the repository (default: current directory)")
	cmd.Flags().StringVar(&jiraTicket, "jira-ticket", "", "Jira ticket to link (e.g. SS-1234; 'auto' to detect from branch)")
	return cmd
}

// detectJiraTicket reads the current git branch and extracts a Jira-style
// ticket ID (e.g. feat/SS-2273-foo → SS-2273).
func detectJiraTicket(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	for _, segment := range strings.Split(branch, "/") {
		parts := strings.SplitN(segment, "-", 3)
		if len(parts) < 2 {
			continue
		}
		project, num := parts[0], parts[1]
		if isAlpha(project) && isDigits(num) && len(project) >= 2 {
			return strings.ToUpper(project) + "-" + num
		}
	}
	return ""
}

// addJiraRemoteLink POSTs a remote link to the Jira ticket via curl.
// Credentials are read from JIRA_URL, JIRA_EMAIL, JIRA_TOKEN env vars.
func addJiraRemoteLink(ticket, savepointPath, repoPath string) error {
	jiraURL := os.Getenv("JIRA_URL")
	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_TOKEN")
	if jiraURL == "" || email == "" || token == "" {
		return fmt.Errorf("set JIRA_URL, JIRA_EMAIL, JIRA_TOKEN env vars")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
	linkURL := savepointGitHubURL(repoPath, savepointPath)
	title := "Savepoint: " + savepointPath[max(0, len(savepointPath)-40):]

	body := fmt.Sprintf(
		`{"globalId":"workflow-savepoint","object":{"url":%q,"title":%q}}`,
		linkURL, title,
	)
	endpoint := jiraURL + "/rest/api/3/issue/" + ticket + "/remotelink"

	out, err := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
		"-X", "POST",
		"-H", "Authorization: Basic "+auth,
		"-H", "Content-Type: application/json",
		"-d", body,
		endpoint,
	).Output()
	if err != nil {
		return fmt.Errorf("curl: %w", err)
	}
	code := strings.TrimSpace(string(out))
	if code != "200" && code != "201" {
		return fmt.Errorf("Jira returned HTTP %s", code)
	}
	return nil
}

// savepointGitHubURL builds a GitHub blob URL for the savepoint file,
// derived from the git remote origin. Falls back to file:// if unavailable.
func savepointGitHubURL(repoPath, savepointPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
	if err != nil {
		return "file://" + savepointPath
	}
	remote := strings.TrimSpace(string(out))
	// git@github.com:Org/Repo.git → https://github.com/Org/Repo
	if strings.HasPrefix(remote, "git@github.com:") {
		remote = "https://github.com/" + strings.TrimPrefix(remote, "git@github.com:")
	}
	remote = strings.TrimSuffix(remote, ".git")
	rel := strings.TrimPrefix(savepointPath, repoPath+"/")
	return remote + "/blob/main/" + rel
}

// friendlyName extracts a first name from $USER (e.g. "username" → "Geraldo").
// Returns empty string if unavailable.
func friendlyName() string {
	user := os.Getenv("USER")
	if user == "" {
		return ""
	}
	// Take first segment before . or - or _
	for _, sep := range []string{".", "-", "_"} {
		if i := strings.Index(user, sep); i > 0 {
			user = user[:i]
		}
	}
	if len(user) == 0 {
		return ""
	}
	// Capitalize first letter
	return strings.ToUpper(user[:1]) + strings.ToLower(user[1:])
}

func isAlpha(s string) bool {
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return len(s) > 0
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
