package exporter

import (
	"fmt"
	"strings"
)

// FormatDryRunResult formats a PushResult from dry-run mode as a human-readable report.
func FormatDryRunResult(result *PushResult) string {
	var b strings.Builder

	b.WriteString("🔍 DRY RUN — Export Preview\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(&b, "Target:  %s\n", result.Target)
	fmt.Fprintf(&b, "Project: %s\n", result.ProjectKey)
	fmt.Fprintf(&b, "Epics:   %d\n", result.EpicsPushed)
	fmt.Fprintf(&b, "Stories: %d\n", result.StoriesPushed)
	b.WriteString("\n")

	b.WriteString("Items that would be created:\n\n")

	currentEpic := ""
	for _, item := range result.Items {
		switch item.Type {
		case "epic":
			currentEpic = item.Title
			fmt.Fprintf(&b, "📦 EPIC: %s\n", item.Title)
			if item.LocalID != "" {
				fmt.Fprintf(&b, "   ID: %s\n", item.LocalID)
			}
		case "story":
			fmt.Fprintf(&b, "  📝 STORY: %s\n", item.Title)
			if item.LocalID != "" {
				fmt.Fprintf(&b, "     ID: %s | Parent: %s\n", item.LocalID, currentEpic)
			}
		}
	}

	b.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString("⚠️  No changes were made. Remove --dry-run to push.\n")

	return b.String()
}

// FormatPushResult formats a PushResult from an actual push operation.
func FormatPushResult(result *PushResult) string {
	var b strings.Builder

	b.WriteString("✅ EXPORT COMPLETE\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(&b, "Target:  %s\n", result.Target)
	fmt.Fprintf(&b, "Project: %s\n", result.ProjectKey)
	fmt.Fprintf(&b, "Epics:   %d created\n", result.EpicsPushed)
	fmt.Fprintf(&b, "Stories: %d created\n", result.StoriesPushed)
	b.WriteString("\n")

	if len(result.Items) > 0 {
		b.WriteString("Created items:\n\n")
		for _, item := range result.Items {
			status := "✅"
			if item.Error != "" {
				status = "❌"
			}
			switch item.Type {
			case "epic":
				fmt.Fprintf(&b, "%s EPIC: %s", status, item.Title)
				if item.RemoteKey != "" {
					fmt.Fprintf(&b, " → %s", item.RemoteKey)
				}
				b.WriteString("\n")
			case "story":
				fmt.Fprintf(&b, "  %s STORY: %s", status, item.Title)
				if item.RemoteKey != "" {
					fmt.Fprintf(&b, " → %s", item.RemoteKey)
				}
				b.WriteString("\n")
			}
			if item.Error != "" {
				fmt.Fprintf(&b, "     Error: %s\n", item.Error)
			}
		}
	}

	if len(result.Errors) > 0 {
		b.WriteString("\n⚠️  Errors:\n")
		for _, err := range result.Errors {
			fmt.Fprintf(&b, "  - %s\n", err)
		}
	}

	return b.String()
}
