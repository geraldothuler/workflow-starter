package patterns_catalog

import (
	"fmt"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HIERARCHY GUARDRAIL
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ApplyHierarchy filters suggestions against higher-priority catalog entries.
//
// Rules:
//  1. If a suggestion references a pattern that has been reclassified as
//     anti-pattern at a higher level → BLOCK
//  2. If a suggestion references an anti-pattern that has been reclassified
//     as pattern at a higher level → BLOCK
//  3. Anti-pattern suggestions are enriched with remediation from catalog
//  4. Blocked suggestions get BlockedBy explaining why
func ApplyHierarchy(suggestions []types.PatternSuggestion, catalog *PatternCatalog) []types.PatternSuggestion {
	var result []types.PatternSuggestion

	for _, s := range suggestions {
		conflict := detectConflict(s, catalog)
		if conflict != "" {
			// Block the suggestion
			s.BlockedBy = conflict
		}

		// Enrich anti-pattern remediation
		if s.Type == "anti-pattern" && len(s.Remediation) == 0 {
			entry := catalog.GetByID(s.PatternID)
			if entry != nil && len(entry.Remediation) > 0 {
				s.Remediation = entry.Remediation
			}
		}

		result = append(result, s)
	}

	return result
}

// detectConflict checks if a suggestion conflicts with a higher-priority catalog entry.
// Returns empty string if no conflict, otherwise returns explanation.
func detectConflict(suggestion types.PatternSuggestion, catalog *PatternCatalog) string {
	entry := catalog.GetByID(suggestion.PatternID)
	if entry == nil {
		return "" // Not in catalog — no conflict possible
	}

	suggestionLevel := levelPriority[suggestion.Level]
	catalogLevel := levelPriority[entry.Level]

	// Conflict: LLM suggested based on one type, but catalog has different type at higher level
	if catalogLevel > suggestionLevel && entry.Type != suggestion.Type {
		return fmt.Sprintf(
			"%s reclassified as %q at %s level (was %q at %s level)",
			entry.Name, entry.Type, entry.Level, suggestion.Type, suggestion.Level,
		)
	}

	// Conflict: LLM suggested a pattern, but catalog says it's an anti-pattern (same level, catalog wins)
	if entry.Type != suggestion.Type && catalogLevel >= suggestionLevel {
		return fmt.Sprintf(
			"%s is classified as %q in catalog (%s level)",
			entry.Name, entry.Type, entry.Level,
		)
	}

	return ""
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CONFLICT REPORT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ConflictReport summarizes hierarchy conflicts found in suggestions.
type ConflictReport struct {
	Blocked    []types.PatternSuggestion // Suggestions that were blocked
	Clean      []types.PatternSuggestion // Suggestions with no conflicts
	TotalInput int                       // Total suggestions before filtering
}

// GenerateConflictReport creates a report from the full suggestion list.
func GenerateConflictReport(suggestions []types.PatternSuggestion) ConflictReport {
	report := ConflictReport{
		TotalInput: len(suggestions),
	}

	for _, s := range suggestions {
		if s.BlockedBy != "" {
			report.Blocked = append(report.Blocked, s)
		} else {
			report.Clean = append(report.Clean, s)
		}
	}

	return report
}

// FormatReport returns a human-readable summary of the conflict report.
func (r ConflictReport) FormatReport() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Pattern Suggestions: %d total, %d clean, %d blocked\n",
		r.TotalInput, len(r.Clean), len(r.Blocked)))

	if len(r.Blocked) > 0 {
		sb.WriteString("\nBlocked suggestions:\n")
		for _, s := range r.Blocked {
			sb.WriteString(fmt.Sprintf("  - %s (%s): %s\n", s.PatternName, s.PatternID, s.BlockedBy))
			if len(s.Remediation) > 0 {
				sb.WriteString(fmt.Sprintf("    Remediation: %s\n", strings.Join(s.Remediation, ", ")))
			}
		}
	}

	return sb.String()
}
