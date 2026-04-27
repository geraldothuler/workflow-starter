package patterns_catalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ECOSYSTEM COHERENCE ANALYSIS (Heuristic, Zero LLM)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// severityOrder maps severity strings to numeric values for sorting.
// Lower number = higher severity (sorted first).
var severityOrder = map[string]int{
	"high":   0,
	"medium": 1,
	"low":    2,
}

// AnalyzeCoherence runs heuristic checks on pattern suggestions to detect
// ecosystem-level coherence issues. Zero LLM calls — uses catalog metadata
// (related_patterns, remediation, when_to_use) to find gaps.
//
// Checks performed:
//  1. Missing companions: pattern suggested but its related_patterns are absent
//  2. Anti-pattern contradictions: anti-pattern detected but no remediation pattern suggested
//  3. When-to-use mismatch: pattern's when_to_use keywords don't match the suggestion context
func AnalyzeCoherence(suggestions []types.PatternSuggestion, catalog *PatternCatalog) []types.CoherenceIssue {
	if len(suggestions) == 0 || catalog == nil {
		return nil
	}

	// Build set of active (non-blocked) suggestion IDs for fast lookup
	activeIDs := make(map[string]bool)
	var active []types.PatternSuggestion
	for _, s := range suggestions {
		if s.BlockedBy == "" {
			activeIDs[s.PatternID] = true
			active = append(active, s)
		}
	}

	if len(active) == 0 {
		return nil
	}

	var issues []types.CoherenceIssue
	issues = append(issues, checkMissingCompanions(active, activeIDs, catalog)...)
	issues = append(issues, checkAntiPatternContradictions(active, activeIDs, catalog)...)
	issues = append(issues, checkWhenToUseMismatch(active, catalog)...)

	// Sort by severity (high first), then by ID for stability
	sort.Slice(issues, func(i, j int) bool {
		si := severityOrder[issues[i].Severity]
		sj := severityOrder[issues[j].Severity]
		if si != sj {
			return si < sj
		}
		return issues[i].ID < issues[j].ID
	})

	// Assign sequential IDs after sorting
	for i := range issues {
		issues[i].ID = fmt.Sprintf("COH-%03d", i+1)
	}

	return issues
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 1: Missing Companion Patterns
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// checkMissingCompanions detects when a pattern is suggested but its
// related_patterns (companions) from the catalog are absent.
//
// Example: CQRS suggested but event-sourcing is not → missing companion.
// Severity: "medium" (1 missing), "high" (>2 missing).
func checkMissingCompanions(active []types.PatternSuggestion, activeIDs map[string]bool, catalog *PatternCatalog) []types.CoherenceIssue {
	var issues []types.CoherenceIssue

	for _, s := range active {
		if s.Type == "anti-pattern" {
			continue // Only check patterns, not anti-patterns
		}

		entry := catalog.GetByID(s.PatternID)
		if entry == nil || len(entry.RelatedPatterns) == 0 {
			continue
		}

		// Find missing companions
		var missing []string
		for _, relatedID := range entry.RelatedPatterns {
			if !activeIDs[relatedID] {
				missing = append(missing, relatedID)
			}
		}

		if len(missing) == 0 {
			continue
		}

		severity := "medium"
		if len(missing) > 2 {
			severity = "high"
		}

		issues = append(issues, types.CoherenceIssue{
			Type:     "missing-companion",
			Severity: severity,
			Title:    fmt.Sprintf("Missing companion patterns for %s", entry.Name),
			Description: fmt.Sprintf(
				"%s is suggested but %d related pattern(s) are absent: %s. "+
					"These patterns are commonly used together.",
				entry.Name, len(missing), strings.Join(missing, ", ")),
			PatternIDs: append([]string{s.PatternID}, missing...),
			Suggestion: fmt.Sprintf("Consider adding %s to ensure ecosystem coherence.", strings.Join(missing, ", ")),
		})
	}

	return issues
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 2: Anti-Pattern Contradictions
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// checkAntiPatternContradictions detects when an anti-pattern is suggested
// but none of its remediation patterns are present in the suggestions.
//
// Example: "distributed-monolith" detected but none of ["database-per-service",
// "event-driven", "bounded-context"] are suggested → contradiction.
// Severity: always "high".
func checkAntiPatternContradictions(active []types.PatternSuggestion, activeIDs map[string]bool, catalog *PatternCatalog) []types.CoherenceIssue {
	var issues []types.CoherenceIssue

	for _, s := range active {
		if s.Type != "anti-pattern" {
			continue
		}

		entry := catalog.GetByID(s.PatternID)
		if entry == nil || len(entry.Remediation) == 0 {
			continue
		}

		// Check if any remediation pattern is in suggestions
		hasRemediation := false
		for _, remID := range entry.Remediation {
			if activeIDs[remID] {
				hasRemediation = true
				break
			}
		}

		if hasRemediation {
			continue
		}

		issues = append(issues, types.CoherenceIssue{
			Type:     "anti-pattern-contradiction",
			Severity: "high",
			Title:    fmt.Sprintf("Anti-pattern %q detected without remediation", entry.Name),
			Description: fmt.Sprintf(
				"Anti-pattern %s was detected but none of its remediation patterns are suggested: %s. "+
					"Without remediation, the anti-pattern remains unresolved.",
				entry.Name, strings.Join(entry.Remediation, ", ")),
			PatternIDs: append([]string{s.PatternID}, entry.Remediation...),
			Suggestion: fmt.Sprintf("Consider adopting one of: %s.", strings.Join(entry.Remediation, ", ")),
		})
	}

	return issues
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 3: When-to-Use Mismatch
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// minWhenToUseKeywords is the minimum number of when_to_use entries
// a pattern must have for the mismatch check to apply.
const minWhenToUseKeywords = 2

// whenToUseMatchThreshold is the fraction of when_to_use keywords
// that must appear in the suggestion context to avoid a mismatch.
// 0.0 = no keywords need to match (never triggers).
// 1.0 = all keywords must match.
const whenToUseMatchThreshold = 0.0 // Effectively: at least 1 keyword must match

// checkWhenToUseMismatch detects when a pattern's when_to_use keywords
// don't match the suggestion context (reasoning + affected epics).
//
// This is a soft check: severity is always "low" because LLM reasoning
// may use different vocabulary than the catalog's when_to_use entries.
func checkWhenToUseMismatch(active []types.PatternSuggestion, catalog *PatternCatalog) []types.CoherenceIssue {
	var issues []types.CoherenceIssue

	for _, s := range active {
		if s.Type == "anti-pattern" {
			continue // Anti-patterns don't have when_to_use
		}

		entry := catalog.GetByID(s.PatternID)
		if entry == nil || len(entry.WhenToUse) < minWhenToUseKeywords {
			continue
		}

		// Build context from suggestion reasoning + affected epics
		context := strings.ToLower(s.Reasoning)
		for _, epic := range s.AffectedEpics {
			context += " " + strings.ToLower(epic)
		}

		// Count how many when_to_use keywords appear in context
		matchCount := 0
		var unmatched []string
		for _, wtu := range entry.WhenToUse {
			// Check if any significant word from when_to_use appears in context
			wtuLower := strings.ToLower(wtu)
			if containsAnyWord(context, extractSignificantWords(wtuLower)) {
				matchCount++
			} else {
				unmatched = append(unmatched, wtu)
			}
		}

		// If no keywords matched at all, flag it
		if matchCount == 0 {
			issues = append(issues, types.CoherenceIssue{
				Type:     "when-to-use-mismatch",
				Severity: "low",
				Title:    fmt.Sprintf("Context mismatch for %s", entry.Name),
				Description: fmt.Sprintf(
					"%s is suggested but its when-to-use criteria don't align with the context: %s. "+
						"The LLM may have identified a valid use case not captured by the catalog keywords.",
					entry.Name, strings.Join(unmatched, "; ")),
				PatternIDs: []string{s.PatternID},
				Suggestion: "Review the pattern's applicability to confirm it fits the project context.",
			})
		}
	}

	return issues
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TEXT MATCHING HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// extractSignificantWords splits a when_to_use phrase into significant words
// (3+ characters, excluding common stop words).
func extractSignificantWords(phrase string) []string {
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "that": true,
		"are": true, "you": true, "need": true, "have": true, "from": true,
		"when": true, "your": true, "this": true, "than": true, "more": true,
		"use": true, "can": true, "not": true, "all": true, "but": true,
		"com": true, "que": true, "para": true, "por": true, "uma": true,
		"ser": true, "ter": true, "como": true, "mais": true, "dos": true,
	}

	words := strings.Fields(phrase)
	var significant []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:()\"'")
		if len(w) >= 3 && !stopWords[w] {
			significant = append(significant, w)
		}
	}
	return significant
}

// containsAnyWord checks if the text contains any of the given words.
func containsAnyWord(text string, words []string) bool {
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}
