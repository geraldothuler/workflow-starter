package repoindex

import (
	"fmt"
	"strings"
	"time"
)

// RepoStatusEntry is the health status of one indexed repo.
type RepoStatusEntry struct {
	Repo       Repo
	DaysOld    int
	Stale      bool
	Suggestion string
}

// RepoStatusReport is the full output of CheckStatus.
type RepoStatusReport struct {
	Entries    []RepoStatusEntry
	StaleDays  int
	CountOK    int
	CountStale int
}

// CheckStatus loads all indexed repos and classifies each as OK or STALE.
// staleDays is the threshold; 0 falls back to 30.
func CheckStatus(db *DB, staleDays int) (*RepoStatusReport, error) {
	if staleDays <= 0 {
		staleDays = 30
	}

	repos, err := ListRepos(db)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -staleDays)
	report := &RepoStatusReport{StaleDays: staleDays}

	for _, r := range repos {
		entry := RepoStatusEntry{Repo: r}

		if r.LastIndexedAt == "" {
			entry.Stale = true
			entry.Suggestion = fmt.Sprintf("wtb repo index %s --path %s", r.Name, r.Path)
		} else {
			t, err := time.Parse(time.RFC3339, r.LastIndexedAt)
			if err != nil {
				// Try without timezone
				t, err = time.Parse("2006-01-02T15:04:05", r.LastIndexedAt)
			}
			if err == nil {
				entry.DaysOld = int(time.Since(t).Hours() / 24)
				if t.Before(cutoff) {
					entry.Stale = true
					entry.Suggestion = fmt.Sprintf("wtb repo index %s", r.Name)
				}
			}
		}

		if entry.Stale {
			report.CountStale++
		} else {
			report.CountOK++
		}
		report.Entries = append(report.Entries, entry)
	}

	return report, nil
}

// RenderStatus formats a RepoStatusReport as a human-readable table.
func RenderStatus(report *RepoStatusReport) string {
	if len(report.Entries) == 0 {
		return "No repos indexed. Run: wtb repo index <name> --path <path>\n"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%-35s %-16s %-14s %-8s %s\n",
		"REPO", "OWNER", "LANG/FRAMEWORK", "DAYS", "STATUS")
	sb.WriteString(strings.Repeat("─", 100) + "\n")

	for _, e := range report.Entries {
		lang := e.Repo.Lang
		if e.Repo.Framework != "" {
			lang += "/" + e.Repo.Framework
		}
		owner := e.Repo.Owner
		if owner == "" {
			owner = "—"
		}
		days := fmt.Sprintf("%dd", e.DaysOld)
		if e.Repo.LastIndexedAt == "" {
			days = "never"
		}

		status := "✓ ok"
		if e.Stale {
			status = "⚠ STALE"
		}

		fmt.Fprintf(&sb, "%-35s %-16s %-14s %-8s %s\n",
			e.Repo.Name, owner, lang, days, status)

		if e.Stale {
			fmt.Fprintf(&sb, "  → %s\n", e.Suggestion)
		}
	}

	sb.WriteString(strings.Repeat("─", 100) + "\n")
	fmt.Fprintf(&sb, "total=%d  ok=%d  stale=%d  (threshold: %dd)\n",
		len(report.Entries), report.CountOK, report.CountStale, report.StaleDays)

	if report.CountStale > 0 {
		sb.WriteString("\nTo re-index all stale repos at once:\n")
		for _, e := range report.Entries {
			if e.Stale {
				fmt.Fprintf(&sb, "  %s\n", e.Suggestion)
			}
		}
	}

	return sb.String()
}
