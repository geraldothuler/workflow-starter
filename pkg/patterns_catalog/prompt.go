package patterns_catalog

import (
	"fmt"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SUGGESTION CONFIG
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// SuggestionConfig controls the pattern suggestion behavior.
type SuggestionConfig struct {
	MinConfidence float64 // Minimum confidence threshold (0.0-1.0, default 0.5)
	MaxPatterns   int     // Maximum suggestions to return (default 10)
}

// DefaultSuggestionConfig returns sensible defaults.
func DefaultSuggestionConfig() SuggestionConfig {
	return SuggestionConfig{
		MinConfidence: 0.5,
		MaxPatterns:   10,
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PROMPT CONSTRUCTION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// BuildSuggestionPrompt constructs the LLM prompt for pattern suggestion.
// Always in English for LLM quality. Includes SOLID principles reference.
func BuildSuggestionPrompt(backlogSummary string, catalogPrompt string, config SuggestionConfig) string {
	var sb strings.Builder

	// Role
	sb.WriteString("You are an expert software architect. ")
	sb.WriteString("Analyze the following backlog and suggest architecture patterns from the catalog.\n\n")

	// SOLID Reference
	sb.WriteString("## Reasoning Framework\n\n")
	sb.WriteString("Apply SOLID principles when reasoning about pattern recommendations:\n")
	sb.WriteString("- **SRP** (Single Responsibility): Each component should have one reason to change\n")
	sb.WriteString("- **OCP** (Open/Closed): Open for extension, closed for modification\n")
	sb.WriteString("- **LSP** (Liskov Substitution): Subtypes must be substitutable for their base types\n")
	sb.WriteString("- **ISP** (Interface Segregation): Many specific interfaces over one general-purpose\n")
	sb.WriteString("- **DIP** (Dependency Inversion): Depend on abstractions, not concretions\n\n")

	// Backlog Summary
	sb.WriteString("## Backlog Summary\n\n")
	sb.WriteString(backlogSummary)
	sb.WriteString("\n\n")

	// Pattern Catalog
	sb.WriteString(catalogPrompt)
	sb.WriteString("\n")

	// Instructions
	sb.WriteString("## Task\n\n")
	sb.WriteString("Analyze the backlog and suggest relevant patterns AND detect anti-patterns.\n\n")
	sb.WriteString("For each suggestion, provide:\n")
	sb.WriteString("1. **pattern_id**: Exact ID from the catalog above\n")
	sb.WriteString("2. **confidence**: 0.0-1.0 (how confident you are this pattern applies)\n")
	sb.WriteString("3. **reasoning**: Why this pattern is relevant (2-3 sentences, reference SOLID if applicable)\n")
	sb.WriteString("4. **affected_epics**: List of epic IDs/names this pattern relates to\n\n")
	sb.WriteString("For anti-pattern detection:\n")
	sb.WriteString("- Look for signs described in the catalog's anti-pattern entries\n")
	sb.WriteString("- If you detect an anti-pattern, suggest it with type \"anti-pattern\" and include remediation\n\n")

	// Output Format
	sb.WriteString("## Output Format\n\n")
	sb.WriteString("Return a JSON array of suggestions. Example:\n")
	sb.WriteString("```json\n")
	sb.WriteString("[\n")
	sb.WriteString("  {\n")
	sb.WriteString("    \"pattern_id\": \"cqrs\",\n")
	sb.WriteString("    \"confidence\": 0.85,\n")
	sb.WriteString("    \"reasoning\": \"The system has separate read-heavy dashboards and write-heavy transactional flows. CQRS would allow independent scaling of these workloads (DIP: query and command models depend on abstractions).\",\n")
	sb.WriteString("    \"affected_epics\": [\"Dashboard Analytics\", \"Order Processing\"]\n")
	sb.WriteString("  },\n")
	sb.WriteString("  {\n")
	sb.WriteString("    \"pattern_id\": \"god-class\",\n")
	sb.WriteString("    \"confidence\": 0.7,\n")
	sb.WriteString("    \"reasoning\": \"Epic 'Core Service' concentrates authentication, billing, and notification logic. This violates SRP — consider splitting into bounded contexts.\",\n")
	sb.WriteString("    \"affected_epics\": [\"Core Service\"]\n")
	sb.WriteString("  }\n")
	sb.WriteString("]\n")
	sb.WriteString("```\n\n")

	// Constraints
	sb.WriteString("## Constraints\n\n")
	sb.WriteString(fmt.Sprintf("- Only suggest patterns from the catalog above (do NOT invent IDs)\n"))
	sb.WriteString(fmt.Sprintf("- Minimum confidence: %.1f\n", config.MinConfidence))
	sb.WriteString(fmt.Sprintf("- Maximum suggestions: %d\n", config.MaxPatterns))
	sb.WriteString("- Focus on patterns with clear applicability to the backlog\n")
	sb.WriteString("- Prefer fewer high-confidence suggestions over many low-confidence ones\n")
	sb.WriteString("- Return ONLY the JSON array, no additional text\n")

	return sb.String()
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// BACKLOG SUMMARY
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// BuildBacklogSummary creates a compact backlog representation for the LLM prompt.
// Targets ~1500 tokens to leave room for catalog + instructions.
func BuildBacklogSummary(backlog types.Backlog) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**Epics:** %d | **Stories:** %d\n\n", len(backlog.Epics), countStories(backlog)))

	for i, epic := range backlog.Epics {
		if i >= 15 {
			sb.WriteString(fmt.Sprintf("... and %d more epics\n", len(backlog.Epics)-15))
			break
		}

		sb.WriteString(fmt.Sprintf("### Epic: %s\n", epic.Title))
		if epic.Description != "" {
			// Truncate long descriptions
			desc := epic.Description
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			sb.WriteString(desc)
			sb.WriteString("\n")
		}

		// List stories (max 8 per epic for compactness)
		for j, story := range epic.Stories {
			if j >= 8 {
				sb.WriteString(fmt.Sprintf("  ... +%d more stories\n", len(epic.Stories)-8))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s", story.Title))
			if story.Effort > 0 {
				sb.WriteString(fmt.Sprintf(" (%dp)", story.Effort))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Tech context from deep dives (if available)
	if len(backlog.DeepDives) > 0 {
		sb.WriteString("### Technologies Mentioned\n")
		techs := make(map[string]bool)
		for _, dd := range backlog.DeepDives {
			techs[dd.Term] = true
		}
		techList := make([]string, 0, len(techs))
		for t := range techs {
			techList = append(techList, t)
		}
		sb.WriteString(strings.Join(techList, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

// countStories returns the total number of stories across all epics.
func countStories(backlog types.Backlog) int {
	count := 0
	for _, epic := range backlog.Epics {
		count += len(epic.Stories)
	}
	return count
}
