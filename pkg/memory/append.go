package memory

import (
	"fmt"
	"os"
	"strings"
)

// AppendToTopic appends content to a topic file.
// If section is non-empty, the content is inserted just before the next same-level heading
// after the matching section. If the section is not found, content is appended at EOF.
// If section is empty, content is always appended at EOF.
func AppendToTopic(filePath, section, content string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	body := string(data)

	if section == "" {
		// Simple EOF append with blank-line separator
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		body += "\n" + strings.TrimRight(content, "\n") + "\n"
		return os.WriteFile(filePath, []byte(body), 0644)
	}

	// Find the section heading
	lines := strings.Split(body, "\n")
	sectionLevel := headingLevel(section)

	insertAt := -1
	inSection := false
	for i, line := range lines {
		if inSection {
			// Stop at the next heading of the same or higher level
			lvl := headingLevel(line)
			if lvl > 0 && lvl <= sectionLevel {
				insertAt = i
				break
			}
		}
		if isHeading(line, section) {
			inSection = true
		}
	}

	if !inSection {
		// Section not found — append at EOF
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		body += "\n" + strings.TrimRight(content, "\n") + "\n"
		return os.WriteFile(filePath, []byte(body), 0644)
	}

	if insertAt == -1 {
		// Section found but no following heading — append at EOF
		insertAt = len(lines)
	}

	// Insert before insertAt, with a blank line if needed
	inserted := strings.TrimRight(content, "\n")
	var newLines []string
	newLines = append(newLines, lines[:insertAt]...)
	// Ensure blank line before inserted content
	if insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) != "" {
		newLines = append(newLines, "")
	}
	newLines = append(newLines, inserted)
	newLines = append(newLines, lines[insertAt:]...)

	return os.WriteFile(filePath, []byte(strings.Join(newLines, "\n")), 0644)
}

// AppendRuleToMemory appends a one-liner under "## Regras de processo" in MEMORY.md.
func AppendRuleToMemory(memoryPath, rule string) error {
	return AppendToTopic(memoryPath, "## Regras de processo", "**"+strings.TrimSpace(rule)+"**")
}

// ExtractSection returns the content from the matching section heading to the next same-level heading.
// If section is not found, returns the full content with a header note.
func ExtractSection(content, section string) string {
	lines := strings.Split(content, "\n")
	sectionLevel := headingLevel(section)
	if sectionLevel == 0 {
		// treat as text search in heading
		sectionLevel = 2
	}

	var result []string
	inSection := false
	for _, line := range lines {
		if inSection {
			lvl := headingLevel(line)
			if lvl > 0 && lvl <= sectionLevel {
				break
			}
			result = append(result, line)
		}
		if isHeading(line, section) {
			inSection = true
			result = append(result, line)
		}
	}

	if !inSection {
		return fmt.Sprintf("(section %q not found)\n\n%s", section, content)
	}
	return strings.Join(result, "\n")
}

// GrepLines returns lines matching keyword (case-insensitive) with ±1 line context,
// deduplicating overlapping windows.
func GrepLines(content, keyword string) string {
	lines := strings.Split(content, "\n")
	kw := strings.ToLower(keyword)
	include := make(map[int]bool)
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), kw) {
			if i > 0 {
				include[i-1] = true
			}
			include[i] = true
			if i+1 < len(lines) {
				include[i+1] = true
			}
		}
	}

	if len(include) == 0 {
		return fmt.Sprintf("(no lines matching %q)", keyword)
	}

	var result []string
	prevIdx := -2
	for i := 0; i < len(lines); i++ {
		if include[i] {
			if i > prevIdx+1 && prevIdx >= 0 {
				result = append(result, "---")
			}
			result = append(result, lines[i])
			prevIdx = i
		}
	}
	return strings.Join(result, "\n")
}

// headingLevel returns the Markdown heading level (1-6) for a line like "## Foo", or 0.
func headingLevel(line string) int {
	trimmed := strings.TrimSpace(line)
	for i := 6; i >= 1; i-- {
		prefix := strings.Repeat("#", i) + " "
		if strings.HasPrefix(trimmed, prefix) {
			return i
		}
	}
	return 0
}

// isHeading returns true if line is a Markdown heading whose text starts with or equals title (case-insensitive).
// Prefix match allows passing "## Regras de processo" to match "## Regras de processo (one-liners — ...)".
func isHeading(line, title string) bool {
	trimmed := strings.TrimSpace(line)
	for i := 6; i >= 1; i-- {
		prefix := strings.Repeat("#", i) + " "
		if strings.HasPrefix(trimmed, prefix) {
			headingText := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)))
			titleText := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(title), strings.Repeat("#", i)+" ")))
			return headingText == titleText || strings.HasPrefix(headingText, titleText)
		}
	}
	return false
}
