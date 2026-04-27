package playbook

import (
	"fmt"
	"strings"
)

// RenderMarkdown generates a markdown report from an investigation report.
func RenderMarkdown(report *InvestigationReport, spec *PlaybookSpec) string {
	var sb strings.Builder

	// Title
	title := report.PlaybookTitle
	if spec != nil && spec.Report.TitleTemplate != "" {
		title = spec.Report.TitleTemplate
	}
	sb.WriteString("# " + title + "\n\n")

	// Metadata
	sb.WriteString(fmt.Sprintf("**Date:** %s  \n", report.StartedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Duration:** %s  \n", report.Duration.Round(1000000).String()))
	sb.WriteString(fmt.Sprintf("**Steps:** %d executed, %d skipped  \n", report.StepsExecuted, report.StepsSkipped))
	sb.WriteString("\n")

	// Executive Summary
	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString(report.Summary + "\n\n")

	critical, warning, info := countBySeverity(report.Findings)
	if len(report.Findings) > 0 {
		sb.WriteString(fmt.Sprintf("| Severity | Count |\n"))
		sb.WriteString(fmt.Sprintf("|----------|-------|\n"))
		if critical > 0 {
			sb.WriteString(fmt.Sprintf("| Critical | %d |\n", critical))
		}
		if warning > 0 {
			sb.WriteString(fmt.Sprintf("| Warning  | %d |\n", warning))
		}
		if info > 0 {
			sb.WriteString(fmt.Sprintf("| Info     | %d |\n", info))
		}
		sb.WriteString("\n")
	}

	// Findings
	if len(report.Findings) > 0 {
		sb.WriteString("## Findings\n\n")

		// Group by severity: critical first
		for _, severity := range []string{SeverityCritical, SeverityWarning, SeverityInfo} {
			findings := filterBySeverity(report.Findings, severity)
			if len(findings) == 0 {
				continue
			}

			sb.WriteString(fmt.Sprintf("### %s\n\n", capitalize(severity)))
			for _, f := range findings {
				sb.WriteString(fmt.Sprintf("#### %s\n\n", f.Title))
				sb.WriteString(f.Detail + "\n\n")
				if f.Evidence != "" {
					sb.WriteString(fmt.Sprintf("**Evidence:** `%s`\n\n", f.Evidence))
				}
				if f.Recommendation != "" {
					sb.WriteString(fmt.Sprintf("**Recommendation:** %s\n\n", f.Recommendation))
				}
			}
		}
	}

	// Causal Chain
	if len(report.CausalChain) > 0 {
		sb.WriteString("## Causal Chain\n\n")

		findingByID := make(map[string]Finding)
		for _, f := range report.Findings {
			findingByID[f.ID] = f
		}

		for _, link := range report.CausalChain {
			cause := findingByID[link.From]
			effect := findingByID[link.To]
			sb.WriteString(fmt.Sprintf("- **%s** → **%s**\n", cause.Title, effect.Title))
			sb.WriteString(fmt.Sprintf("  - %s\n", link.Reasoning))
		}
		sb.WriteString("\n")
	}

	// Recommendations (aggregated)
	recommendations := collectRecommendations(report.Findings)
	if len(recommendations) > 0 {
		sb.WriteString("## Recommendations\n\n")
		for i, rec := range recommendations {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
		}
		sb.WriteString("\n")
	}

	// Investigation Steps table
	sb.WriteString("## Investigation Steps\n\n")
	sb.WriteString("| Step | Provider | Status | Findings | Duration |\n")
	sb.WriteString("|------|----------|--------|----------|----------|\n")
	for _, sr := range report.StepResults {
		findingCount := len(sr.Findings)
		duration := sr.Duration.Round(1000000).String()
		errInfo := ""
		if sr.Error != "" {
			errInfo = " (" + sr.Error + ")"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s%s | %d | %s |\n",
			sr.Title, sr.Provider, sr.Status, errInfo, findingCount, duration))
	}

	return sb.String()
}

func countBySeverity(findings []Finding) (critical, warning, info int) {
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warning++
		case SeverityInfo:
			info++
		}
	}
	return
}

func filterBySeverity(findings []Finding, severity string) []Finding {
	var result []Finding
	for _, f := range findings {
		if f.Severity == severity {
			result = append(result, f)
		}
	}
	return result
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func collectRecommendations(findings []Finding) []string {
	seen := make(map[string]bool)
	var result []string
	for _, f := range findings {
		if f.Recommendation != "" && !seen[f.Recommendation] {
			seen[f.Recommendation] = true
			result = append(result, f.Recommendation)
		}
	}
	return result
}
