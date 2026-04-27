package patterns_catalog

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SUGGESTION ENGINE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// rawSuggestion is the JSON structure returned by the LLM.
type rawSuggestion struct {
	PatternID     string   `json:"pattern_id"`
	Confidence    float64  `json:"confidence"`
	Reasoning     string   `json:"reasoning"`
	AffectedEpics []string `json:"affected_epics"`
}

// SuggestPatterns analyzes a backlog and suggests architecture patterns using an LLM.
//
// Flow:
//  1. Build compact backlog summary (~1500 tokens)
//  2. Format catalog for prompt (~2500 tokens)
//  3. Send single LLM call with prompt
//  4. Parse JSON response
//  5. Validate against catalog (drop hallucinated IDs, enrich metadata)
//  6. Filter by confidence threshold
//  7. Apply hierarchy guardrail
func SuggestPatterns(catalog *PatternCatalog, backlog types.Backlog, client llm.Completer, config SuggestionConfig) ([]types.PatternSuggestion, error) {
	// 1. Build backlog summary
	summary := BuildBacklogSummary(backlog)

	// 2. Format catalog for prompt
	catalogPrompt := catalog.FormatForPrompt()

	// 3. Build full prompt
	prompt := BuildSuggestionPrompt(summary, catalogPrompt, config)

	// 4. Call LLM
	response, err := client.Complete(prompt, 4096)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// 5. Parse response
	raw, err := parseSuggestionResponse(response)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM response: %w", err)
	}

	// 6. Validate against catalog + enrich
	suggestions := validateAgainstCatalog(raw, catalog)

	// 7. Filter by confidence and limit
	suggestions = filterSuggestions(suggestions, config)

	// 8. Apply hierarchy guardrail
	suggestions = ApplyHierarchy(suggestions, catalog)

	return suggestions, nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PARSING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// jsonArrayPattern extracts JSON array from LLM response (may have surrounding text).
var jsonArrayPattern = regexp.MustCompile(`\[[\s\S]*\]`)

// parseSuggestionResponse extracts the JSON array from the LLM response.
func parseSuggestionResponse(response string) ([]rawSuggestion, error) {
	response = strings.TrimSpace(response)

	// Try direct parse first
	var suggestions []rawSuggestion
	if err := json.Unmarshal([]byte(response), &suggestions); err == nil {
		return suggestions, nil
	}

	// Try extracting JSON array from surrounding text
	match := jsonArrayPattern.FindString(response)
	if match == "" {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	if err := json.Unmarshal([]byte(match), &suggestions); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	return suggestions, nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// VALIDATION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// validateAgainstCatalog drops hallucinated IDs and enriches with catalog metadata.
func validateAgainstCatalog(raw []rawSuggestion, catalog *PatternCatalog) []types.PatternSuggestion {
	var result []types.PatternSuggestion

	for _, r := range raw {
		entry := catalog.GetByID(r.PatternID)
		if entry == nil {
			// Hallucinated ID — skip
			continue
		}

		suggestion := types.PatternSuggestion{
			PatternID:     entry.ID,
			PatternName:   entry.Name,
			Type:          entry.Type,
			Confidence:    clampConfidence(r.Confidence),
			Reasoning:     r.Reasoning,
			AffectedEpics: r.AffectedEpics,
			Category:      entry.Category,
			Source:         entry.Source.Reference,
			Level:         entry.Level,
		}

		// Enrich anti-patterns with remediation
		if entry.Type == "anti-pattern" {
			suggestion.Remediation = entry.Remediation
		}

		result = append(result, suggestion)
	}

	return result
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FILTERING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// filterSuggestions applies confidence threshold and max limit.
func filterSuggestions(suggestions []types.PatternSuggestion, config SuggestionConfig) []types.PatternSuggestion {
	// Filter by confidence
	var filtered []types.PatternSuggestion
	for _, s := range suggestions {
		if s.Confidence >= config.MinConfidence {
			filtered = append(filtered, s)
		}
	}

	// Sort by confidence descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})

	// Limit
	if config.MaxPatterns > 0 && len(filtered) > config.MaxPatterns {
		filtered = filtered[:config.MaxPatterns]
	}

	return filtered
}

// clampConfidence ensures confidence is within [0.0, 1.0].
func clampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}
