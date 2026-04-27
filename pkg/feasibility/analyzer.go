// Package feasibility provides heuristic technical feasibility analysis
// for generated backlogs. Zero LLM calls — aggregates existing pipeline data
// (complexity, effort, deep dives, patterns, coherence) into a structured
// risk report with a composite score.
package feasibility

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SEVERITY WEIGHTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// Score deductions per severity level (from 100 baseline).
var severityDeduction = map[string]int{
	"critical": 25,
	"high":     15,
	"medium":   8,
	"low":      3,
}

// severityOrder for sorting (lower = higher priority).
var severityOrder = map[string]int{
	"critical": 0,
	"high":     1,
	"medium":   2,
	"low":      3,
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// MAIN ENTRY POINT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// AnalyzeFeasibility runs 5 heuristic checks on a backlog and returns
// a feasibility report with a composite score (0-100) and risk items.
//
// Zero LLM calls — uses only data already present in the Backlog struct.
// Gaps are optional: pass nil to skip requirement completeness check.
func AnalyzeFeasibility(backlog *types.Backlog, gaps []types.Gap) *types.FeasibilityReport {
	if backlog == nil {
		return &types.FeasibilityReport{Score: 100}
	}

	var items []types.FeasibilityItem

	items = append(items, checkComplexityEffortMismatch(backlog)...)
	items = append(items, checkCriticalTechConcentration(backlog)...)
	items = append(items, checkRequirementCompleteness(gaps)...)
	items = append(items, checkArchitectureRisk(backlog)...)
	items = append(items, checkScheduleRisk(backlog)...)

	// Sort by severity (critical first), then by ID for stability
	sort.Slice(items, func(i, j int) bool {
		si := severityOrder[items[i].Severity]
		sj := severityOrder[items[j].Severity]
		if si != sj {
			return si < sj
		}
		return items[i].ID < items[j].ID
	})

	// Assign sequential IDs after sorting
	for i := range items {
		items[i].ID = fmt.Sprintf("FSB-%03d", i+1)
	}

	// Calculate composite score
	score := 100
	for _, item := range items {
		score -= severityDeduction[item.Severity]
	}
	if score < 0 {
		score = 0
	}

	// Generate summary
	summary := buildSummary(items)

	return &types.FeasibilityReport{
		Score:   score,
		Items:   items,
		Summary: summary,
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 1: Complexity-Effort Mismatch
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// checkComplexityEffortMismatch detects when declared epic complexity
// doesn't align with the aggregate story effort.
//
// High complexity (≥7) with low effort → underestimated
// Low complexity (≤3) with high effort → overscoped or misclassified
func checkComplexityEffortMismatch(backlog *types.Backlog) []types.FeasibilityItem {
	var items []types.FeasibilityItem

	for _, epic := range backlog.Epics {
		if epic.Complexity == 0 {
			continue // No declared complexity
		}

		totalEffort := 0
		for _, s := range epic.Stories {
			totalEffort += s.Effort
		}

		storyCount := len(epic.Stories)
		if storyCount == 0 {
			continue
		}

		avgEffort := float64(totalEffort) / float64(storyCount)

		// High complexity but low effort → underestimated
		if epic.Complexity >= 7 && avgEffort < 2 {
			items = append(items, types.FeasibilityItem{
				Category: "complexity",
				Severity: "high",
				Title:    fmt.Sprintf("Underestimated effort for epic %q", epic.Code),
				Description: fmt.Sprintf(
					"Epic %s has complexity %d/10 but average story effort is only %.1f SP. "+
						"High-complexity epics typically need higher effort stories.",
					epic.Code, epic.Complexity, avgEffort),
				Impact:     "Teams may run out of time, leading to scope cuts or quality compromises.",
				Suggestion: "Review story breakdowns — consider splitting into more granular stories or increasing effort estimates.",
			})
		}

		// Low complexity but high effort → overscoped
		if epic.Complexity <= 3 && avgEffort >= 5 {
			items = append(items, types.FeasibilityItem{
				Category: "complexity",
				Severity: "medium",
				Title:    fmt.Sprintf("Overscoped stories for simple epic %q", epic.Code),
				Description: fmt.Sprintf(
					"Epic %s has complexity %d/10 but average story effort is %.1f SP. "+
						"Low-complexity epics with high-effort stories may be poorly decomposed.",
					epic.Code, epic.Complexity, avgEffort),
				Impact:     "Work may be harder to parallelize and estimate accurately.",
				Suggestion: "Break large stories into smaller, independently deliverable pieces.",
			})
		}
	}

	return items
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 2: Critical Technology Concentration
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// checkCriticalTechConcentration flags when too many critical/specific
// technologies are present, indicating high integration complexity.
func checkCriticalTechConcentration(backlog *types.Backlog) []types.FeasibilityItem {
	var items []types.FeasibilityItem

	criticalCount := 0
	specificCount := 0
	var criticalTechs []string

	for _, dd := range backlog.DeepDives {
		switch dd.Classification {
		case "critical":
			criticalCount++
			criticalTechs = append(criticalTechs, dd.Term)
		case "specific":
			specificCount++
		}
	}

	// >3 critical technologies → significant integration risk
	if criticalCount > 3 {
		items = append(items, types.FeasibilityItem{
			Category: "technology",
			Severity: "critical",
			Title:    "High critical technology concentration",
			Description: fmt.Sprintf(
				"%d critical technologies detected: %s. "+
					"Each requires specialized knowledge and careful integration.",
				criticalCount, strings.Join(criticalTechs, ", ")),
			Impact:     "Team needs broad expertise across multiple critical systems. Integration testing will be complex.",
			Suggestion: "Consider phased adoption — introduce critical technologies incrementally rather than all at once.",
		})
	} else if criticalCount > 1 {
		// 2-3 critical → moderate risk
		items = append(items, types.FeasibilityItem{
			Category: "technology",
			Severity: "medium",
			Title:    "Multiple critical technologies",
			Description: fmt.Sprintf(
				"%d critical technologies: %s. Each needs dedicated expertise.",
				criticalCount, strings.Join(criticalTechs, ", ")),
			Impact:     "Requires specialized skills and careful integration planning.",
			Suggestion: "Ensure team has expertise in all critical technologies, or plan knowledge ramp-up time.",
		})
	}

	// High specific tech count → broad surface area
	if specificCount > 5 {
		items = append(items, types.FeasibilityItem{
			Category: "technology",
			Severity: "medium",
			Title:    "Wide technology surface area",
			Description: fmt.Sprintf(
				"%d specific technologies detected. A broad technology surface "+
					"increases learning curve and maintenance overhead.", specificCount),
			Impact:     "Higher onboarding cost and potential for integration issues.",
			Suggestion: "Evaluate if all technologies are truly necessary. Consider consolidating overlapping tools.",
		})
	}

	return items
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 3: Requirement Completeness
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// checkRequirementCompleteness converts detected gaps into feasibility items.
// High-severity gaps become critical feasibility risks.
func checkRequirementCompleteness(gaps []types.Gap) []types.FeasibilityItem {
	if len(gaps) == 0 {
		return nil
	}

	var items []types.FeasibilityItem

	highCount := 0
	mediumCount := 0
	for _, g := range gaps {
		switch g.Severity {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		}
	}

	if highCount > 0 {
		severity := "high"
		if highCount >= 3 {
			severity = "critical"
		}
		items = append(items, types.FeasibilityItem{
			Category: "requirements",
			Severity: severity,
			Title:    fmt.Sprintf("%d high-severity requirement gaps", highCount),
			Description: fmt.Sprintf(
				"%d high-severity gaps detected in the specification. "+
					"These represent undefined requirements that could derail development.", highCount),
			Impact:     "Development may proceed on wrong assumptions, requiring costly rework.",
			Suggestion: "Address high-severity gaps before starting implementation. Run 'wtb spec gaps' for details.",
		})
	}

	if mediumCount > 2 {
		items = append(items, types.FeasibilityItem{
			Category: "requirements",
			Severity: "medium",
			Title:    fmt.Sprintf("%d medium-severity requirement gaps", mediumCount),
			Description: fmt.Sprintf(
				"%d medium-severity gaps found. While not blocking, "+
					"they represent areas of ambiguity.", mediumCount),
			Impact:     "Teams may make inconsistent assumptions in ambiguous areas.",
			Suggestion: "Clarify these gaps during sprint planning or early development.",
		})
	}

	return items
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 4: Architecture Risk
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// checkArchitectureRisk aggregates anti-patterns, coherence issues,
// and blocked patterns into architecture risk items.
func checkArchitectureRisk(backlog *types.Backlog) []types.FeasibilityItem {
	var items []types.FeasibilityItem

	// Count anti-patterns without remediation
	antiPatternCount := 0
	for _, s := range backlog.PatternSuggestions {
		if s.Type == "anti-pattern" && s.BlockedBy == "" {
			antiPatternCount++
		}
	}

	// Count high-severity coherence issues
	highCoherenceCount := 0
	for _, c := range backlog.CoherenceIssues {
		if c.Severity == "high" {
			highCoherenceCount++
		}
	}

	// Count blocked patterns (guardrail conflicts)
	blockedCount := 0
	for _, s := range backlog.PatternSuggestions {
		if s.BlockedBy != "" {
			blockedCount++
		}
	}

	if antiPatternCount > 0 {
		severity := "medium"
		if antiPatternCount >= 2 {
			severity = "high"
		}
		items = append(items, types.FeasibilityItem{
			Category: "architecture",
			Severity: severity,
			Title:    fmt.Sprintf("%d anti-pattern(s) detected", antiPatternCount),
			Description: fmt.Sprintf(
				"%d architectural anti-pattern(s) were detected in the backlog. "+
					"These represent known problematic approaches.", antiPatternCount),
			Impact:     "Technical debt accumulation, reduced maintainability, and potential scalability issues.",
			Suggestion: "Review anti-patterns and adopt recommended remediation patterns before building.",
		})
	}

	if highCoherenceCount > 0 {
		items = append(items, types.FeasibilityItem{
			Category: "architecture",
			Severity: "high",
			Title:    fmt.Sprintf("%d high-severity coherence issue(s)", highCoherenceCount),
			Description: fmt.Sprintf(
				"%d high-severity ecosystem coherence issues found. "+
					"The architecture has significant gaps in pattern coverage.", highCoherenceCount),
			Impact:     "Incomplete architecture may lead to integration failures or missing capabilities.",
			Suggestion: "Address coherence issues — add missing companion patterns or resolve anti-pattern contradictions.",
		})
	}

	return items
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 5: Schedule Risk
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// checkScheduleRisk identifies scheduling concerns based on
// story size distribution and backlog scale.
func checkScheduleRisk(backlog *types.Backlog) []types.FeasibilityItem {
	var items []types.FeasibilityItem

	totalStories := 0
	largeStories := 0 // effort ≥ 5
	totalEffort := 0

	for _, epic := range backlog.Epics {
		totalStories += len(epic.Stories)
		for _, s := range epic.Stories {
			totalEffort += s.Effort
			if s.Effort >= 5 {
				largeStories++
			}
		}
	}

	if totalStories == 0 {
		return nil
	}

	// >40% large stories → scheduling risk
	largePercent := float64(largeStories) / float64(totalStories) * 100
	if largePercent > 40 {
		items = append(items, types.FeasibilityItem{
			Category: "schedule",
			Severity: "high",
			Title:    "High concentration of large stories",
			Description: fmt.Sprintf(
				"%.0f%% of stories (%d/%d) have effort ≥5 SP. "+
					"Large stories are harder to estimate, parallelize, and review.",
				largePercent, largeStories, totalStories),
			Impact:     "Sprint velocity becomes unpredictable. Risk of stories carrying over between sprints.",
			Suggestion: "Break stories ≥5 SP into 2-3 smaller, independently deliverable stories.",
		})
	}

	// Very large backlog → coordination risk
	if totalStories > 30 {
		items = append(items, types.FeasibilityItem{
			Category: "schedule",
			Severity: "medium",
			Title:    "Large backlog size",
			Description: fmt.Sprintf(
				"Backlog contains %d stories with total effort %d SP. "+
					"Large backlogs increase coordination overhead and estimation uncertainty.",
				totalStories, totalEffort),
			Impact:     "Harder to maintain visibility, track dependencies, and forecast delivery dates.",
			Suggestion: "Consider breaking into milestones or MVPs. Prioritize a first deliverable slice.",
		})
	}

	// Too few stories per epic → too coarse
	for _, epic := range backlog.Epics {
		if len(epic.Stories) > 0 && len(epic.Stories) < 2 {
			items = append(items, types.FeasibilityItem{
				Category: "schedule",
				Severity: "low",
				Title:    fmt.Sprintf("Epic %q has only %d story", epic.Code, len(epic.Stories)),
				Description: fmt.Sprintf(
					"Epic %s (%s) has only %d story. Single-story epics may indicate "+
						"insufficient decomposition.",
					epic.Code, epic.Title, len(epic.Stories)),
				Impact:     "Reduced ability to track incremental progress within the epic.",
				Suggestion: "Review if this epic can be decomposed into smaller, independent stories.",
			})
		}
	}

	return items
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// buildSummary generates a human-readable summary of the risk distribution.
func buildSummary(items []types.FeasibilityItem) string {
	if len(items) == 0 {
		return "No risks found"
	}

	counts := map[string]int{}
	for _, item := range items {
		counts[item.Severity]++
	}

	var parts []string
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if c, ok := counts[sev]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, sev))
		}
	}

	return strings.Join(parts, ", ") + " risk(s) found"
}
