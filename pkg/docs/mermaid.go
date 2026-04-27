package docs

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateMermaid produces a Mermaid flowchart LR from use-case definitions.
//
// Node styles:
//   - documentary → blue tones (doc class)
//   - pipeline    → green tones (pipe class)
//
// Edge styles:
//   - discovery edges use dashed arrows (-.->): exploratory, optional
//   - all other edges use solid arrows (-->)
func GenerateMermaid(defs []UseCaseDef) string {
	sorted := make([]UseCaseDef, len(defs))
	copy(sorted, defs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	known := make(map[string]bool, len(defs))
	for _, d := range defs {
		known[d.ID] = true
	}

	var sb strings.Builder

	sb.WriteString("flowchart LR\n")
	sb.WriteString("    classDef doc  fill:#0d2137,stroke:#4a9eff,color:#b0ccff\n")
	sb.WriteString("    classDef pipe fill:#0d2b0d,stroke:#3dba4e,color:#a8e6b0\n")
	sb.WriteString("\n")

	// Node declarations
	for _, d := range sorted {
		class := "doc"
		if d.Type == "pipeline" {
			class = "pipe"
		}
		fmt.Fprintf(&sb, "    %s[\"%s\"]:::%s\n", mermaidID(d.ID), d.ID, class)
	}

	sb.WriteString("\n")

	// Edges — derived from each use-case's chain.to
	// Edges are deduplicated: ops-response declares "to incident" for the followup loop,
	// and incident declares "to ops-response" — both are preserved intentionally.
	for _, d := range sorted {
		arrow := "-->"
		if d.ID == "discovery" {
			arrow = "-.->"
		}
		for _, target := range d.Chain.To {
			if !known[target] {
				continue // skip non-existent targets (e.g. "qualquer", "feasibility")
			}
			fmt.Fprintf(&sb, "    %s %s %s\n", mermaidID(d.ID), arrow, mermaidID(target))
		}
	}

	return sb.String()
}

// mermaidID converts a use-case ID to a valid Mermaid node identifier.
// Mermaid IDs must not start with a digit and must not contain hyphens.
func mermaidID(id string) string {
	safe := strings.ReplaceAll(id, "-", "_")
	if len(safe) > 0 && safe[0] >= '0' && safe[0] <= '9' {
		safe = "uc_" + safe
	}
	return safe
}
