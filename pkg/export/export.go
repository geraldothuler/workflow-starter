package export

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ExportBacklogJSON exporta backlog para JSON
func ExportBacklogJSON(backlog *types.Backlog, outputPath string) error {
	data, err := json.MarshalIndent(backlog, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

// ExportBacklogMarkdown exporta backlog para Markdown
func ExportBacklogMarkdown(backlog *types.Backlog, outputPath string) error {
	md := FormatBacklogMarkdown(backlog)
	return os.WriteFile(outputPath, []byte(md), 0644)
}

// FormatBacklogMarkdown formata backlog como Markdown string
func FormatBacklogMarkdown(backlog *types.Backlog) string {
	var b strings.Builder

	// Header
	b.WriteString("# Backlog\n\n")

	// Metadata
	if backlog.Meta.Provider != "" || backlog.Meta.InputFile != "" {
		b.WriteString("## Metadata\n\n")
		if backlog.Meta.Provider != "" {
			fmt.Fprintf(&b, "- **Provider:** %s\n", backlog.Meta.Provider)
		}
		if backlog.Meta.InputFile != "" {
			fmt.Fprintf(&b, "- **Input:** %s\n", backlog.Meta.InputFile)
		}
		if backlog.Meta.GeneratedAt != "" {
			fmt.Fprintf(&b, "- **Generated:** %s\n", backlog.Meta.GeneratedAt)
		}
		fmt.Fprintf(&b, "- **Epics:** %d\n", backlog.Meta.TotalEpics)
		fmt.Fprintf(&b, "- **Stories:** %d\n", backlog.Meta.TotalStories)
		b.WriteString("\n")
	}

	// Epics
	for _, epic := range backlog.Epics {
		fmt.Fprintf(&b, "## %s", epic.Title)
		if epic.Code != "" {
			fmt.Fprintf(&b, " [%s]", epic.Code)
		}
		b.WriteString("\n\n")

		if epic.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", epic.Description)
		}

		if epic.Priority != "" {
			fmt.Fprintf(&b, "**Priority:** %s\n\n", epic.Priority)
		}

		// Stories
		for _, story := range epic.Stories {
			fmt.Fprintf(&b, "### %s\n\n", story.Title)

			if story.What != "" {
				fmt.Fprintf(&b, "**What:** %s\n\n", story.What)
			}
			if story.Why != "" {
				fmt.Fprintf(&b, "**Why:** %s\n\n", story.Why)
			}

			if len(story.AcceptanceCriteria) > 0 {
				b.WriteString("**Acceptance Criteria:**\n\n")
				for _, ac := range story.AcceptanceCriteria {
					fmt.Fprintf(&b, "- %s\n", ac)
				}
				b.WriteString("\n")
			}

			if story.Effort > 0 {
				fmt.Fprintf(&b, "**Effort:** %d points\n\n", story.Effort)
			}

			if len(story.Tasks) > 0 {
				b.WriteString("**Tasks:**\n\n")
				for _, task := range story.Tasks {
					fmt.Fprintf(&b, "- [ ] %s", task.Description)
					if task.Effort != "" {
						fmt.Fprintf(&b, " (%s)", task.Effort)
					}
					b.WriteString("\n")
				}
				b.WriteString("\n")
			}
		}
	}

	// Pattern Suggestions
	if len(backlog.PatternSuggestions) > 0 {
		b.WriteString("---\n\n")
		b.WriteString("## Architecture Pattern Suggestions\n\n")

		for _, s := range backlog.PatternSuggestions {
			marker := "📐"
			if s.Type == "anti-pattern" {
				marker = "⚠️"
			}
			if s.BlockedBy != "" {
				marker = "🚫"
			}
			fmt.Fprintf(&b, "### %s %s (%.0f%%)\n\n", marker, s.PatternName, s.Confidence*100)
			fmt.Fprintf(&b, "- **Category:** %s\n", s.Category)
			fmt.Fprintf(&b, "- **Source:** %s\n", s.Source)
			fmt.Fprintf(&b, "- **Level:** %s\n", s.Level)
			fmt.Fprintf(&b, "- **Reasoning:** %s\n", s.Reasoning)

			if len(s.AffectedEpics) > 0 {
				fmt.Fprintf(&b, "- **Affected Epics:** %s\n", strings.Join(s.AffectedEpics, ", "))
			}
			if len(s.Remediation) > 0 {
				fmt.Fprintf(&b, "- **Remediation:** %s\n", strings.Join(s.Remediation, ", "))
			}
			if s.BlockedBy != "" {
				fmt.Fprintf(&b, "- **Blocked:** %s\n", s.BlockedBy)
			}
			b.WriteString("\n")
		}
	}

	// Coherence Issues
	if len(backlog.CoherenceIssues) > 0 {
		b.WriteString("### Ecosystem Coherence\n\n")
		for _, issue := range backlog.CoherenceIssues {
			severityIcon := "🟢"
			switch issue.Severity {
			case "high":
				severityIcon = "🔴"
			case "medium":
				severityIcon = "🟡"
			}
			fmt.Fprintf(&b, "%s **%s** — %s\n", severityIcon, issue.Title, issue.Description)
			if issue.Suggestion != "" {
				fmt.Fprintf(&b, "  - 💡 %s\n", issue.Suggestion)
			}
			b.WriteString("\n")
		}
	}

	// Feasibility Report
	if backlog.FeasibilityReport != nil && len(backlog.FeasibilityReport.Items) > 0 {
		b.WriteString("---\n\n")
		b.WriteString("## Technical Feasibility Analysis\n\n")
		fmt.Fprintf(&b, "**Score: %d/100** — %s\n\n", backlog.FeasibilityReport.Score, backlog.FeasibilityReport.Summary)

		for _, item := range backlog.FeasibilityReport.Items {
			severityIcon := "🟢"
			switch item.Severity {
			case "critical":
				severityIcon = "🔴"
			case "high":
				severityIcon = "🟠"
			case "medium":
				severityIcon = "🟡"
			}
			fmt.Fprintf(&b, "%s **%s** [%s] — %s\n", severityIcon, item.Title, item.Category, item.Description)
			fmt.Fprintf(&b, "  - ⚡ %s\n", item.Impact)
			fmt.Fprintf(&b, "  - 💡 %s\n", item.Suggestion)
			b.WriteString("\n")
		}
	}

	// Critical Path Report
	if backlog.CriticalPathReport != nil && len(backlog.CriticalPathReport.Phases) > 0 {
		b.WriteString("---\n\n")
		b.WriteString("## Critical Path Analysis\n\n")
		fmt.Fprintf(&b, "**%s**\n\n", backlog.CriticalPathReport.Summary)

		for _, phase := range backlog.CriticalPathReport.Phases {
			parallelLabel := ""
			if phase.Parallel {
				parallelLabel = " (parallel)"
			}
			fmt.Fprintf(&b, "### Phase %d%s\n\n", phase.Phase, parallelLabel)
			if phase.Reasoning != "" {
				fmt.Fprintf(&b, "%s\n\n", phase.Reasoning)
			}

			for _, item := range backlog.CriticalPathReport.Items {
				if item.Phase != phase.Phase {
					continue
				}
				foundationIcon := ""
				if item.IsFoundation {
					foundationIcon = "🏗️ "
				}
				fmt.Fprintf(&b, "- %s**%s** — %s (priority %d)\n", foundationIcon, item.EpicCode, item.EpicTitle, item.Priority)
				if len(item.DependsOn) > 0 {
					fmt.Fprintf(&b, "  - Depends on: %s\n", strings.Join(item.DependsOn, ", "))
				}
			}
			b.WriteString("\n")
		}

		if len(backlog.CriticalPathReport.Dependencies) > 0 {
			b.WriteString("### Inferred Dependencies\n\n")
			for _, dep := range backlog.CriticalPathReport.Dependencies {
				fmt.Fprintf(&b, "- %s → %s (%s, %.0f%%)\n", dep.From, dep.To, dep.Type, dep.Confidence*100)
			}
			b.WriteString("\n")
		}
	}

	// Deep Dives
	if len(backlog.DeepDives) > 0 {
		b.WriteString("---\n\n")
		b.WriteString("## Deep Dives\n\n")

		for _, dd := range backlog.DeepDives {
			fmt.Fprintf(&b, "### Deep Dive: %s\n\n", dd.Term)

			if dd.WhatIs != "" {
				b.WriteString("**O que é?**\n\n")
				fmt.Fprintf(&b, "%s\n\n", dd.WhatIs)
			}

			if dd.WhyHere != "" {
				b.WriteString("**Por que usar?**\n\n")
				fmt.Fprintf(&b, "%s\n\n", dd.WhyHere)
			}

			if dd.Configuration != "" {
				b.WriteString("**Configuração:**\n\n")
				fmt.Fprintf(&b, "%s\n\n", dd.Configuration)
			}

			if len(dd.Patterns) > 0 {
				b.WriteString("**Padrões:**\n\n")
				for _, p := range dd.Patterns {
					fmt.Fprintf(&b, "- %s\n", p)
				}
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

// ExportStatic exporta arquivos estáticos
func ExportStatic(backlogPath, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(backlogPath)
	if err != nil {
		return err
	}

	return os.WriteFile(outputDir+"/backlog.json", data, 0644)
}
