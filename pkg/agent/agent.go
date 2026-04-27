// Package agent handles agent-type use-cases: renders prompt templates and
// formats spawn_agent responses for MCP consumers (Claude Code).
//
// Agent use-cases have type: agent in their definition.yml and declare an
// agent: block with subagent_type, prompt_template, and description_template.
// When workflow_run is called with an agent use-case, it returns a structured
// spawn_agent block that Claude interprets and acts on via Agent(...).
package agent

import (
	"fmt"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/runner"
)

// Render fills ${key} and ${key:-default} placeholders in description and
// prompt using the resolved inputs, then returns the populated values.
func Render(spec runner.AgentSpec, inputs runner.RunInputs) (description, prompt string) {
	description = runner.ResolveMappingTemplate(spec.DescriptionTemplate, inputs)
	prompt = runner.ResolveMappingTemplate(spec.PromptTemplate, inputs)
	return
}

// FormatRequest formats a spawn_agent block for inclusion in a workflow_run
// MCP response. Claude reads this block and calls Agent(...) with the params.
//
// Format:
//
//	spawn_agent:
//	  subagent_type: <type>
//	  background: true|false
//	  description: "<description>"
//	  prompt: |
//	    <prompt lines>
func FormatRequest(spec runner.AgentSpec, description, prompt string) string {
	var sb strings.Builder
	sb.WriteString("\nspawn_agent:\n")
	fmt.Fprintf(&sb, "  subagent_type: %s\n", spec.SubagentType)
	fmt.Fprintf(&sb, "  background: %v\n", spec.Background)
	fmt.Fprintf(&sb, "  description: %q\n", description)
	sb.WriteString("  prompt: |\n")
	for _, line := range strings.Split(strings.TrimRight(prompt, "\n"), "\n") {
		fmt.Fprintf(&sb, "    %s\n", line)
	}
	return sb.String()
}
