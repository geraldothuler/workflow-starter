package sources

import (
	"fmt"
	"strings"
)

const maxRecursionDepth = 10

// BlockConverter converts Notion blocks to markdown.
// It handles nested blocks via a fetcher callback for recursive content.
type BlockConverter struct {
	depth   int // Current nesting depth (for indentation)
	fetcher BlockFetcher
}

// BlockFetcher is a callback to fetch child blocks by block ID.
// Used for recursive block fetching (toggles, nested lists, etc.).
type BlockFetcher func(blockID string) ([]notionBlock, error)

// NewBlockConverter creates a new converter with the given fetcher for recursive blocks.
func NewBlockConverter(fetcher BlockFetcher) *BlockConverter {
	return &BlockConverter{
		depth:   0,
		fetcher: fetcher,
	}
}

// Convert converts a slice of Notion blocks to a markdown string.
func (bc *BlockConverter) Convert(blocks []notionBlock) string {
	var builder strings.Builder
	numberedCounter := 0

	for _, block := range blocks {
		if block.Type != "numbered_list_item" {
			// Reset numbered counter when leaving a numbered list
			if numberedCounter > 0 {
				numberedCounter = 0
			}
		}

		md := bc.convertBlock(block, &numberedCounter)
		if md != "" {
			builder.WriteString(md)
		}

		// Recursive: fetch children if needed
		if block.HasChildren && bc.depth < maxRecursionDepth && bc.fetcher != nil {
			children, err := bc.fetcher(block.ID)
			if err == nil && len(children) > 0 {
				childConverter := &BlockConverter{
					depth:   bc.depth + 1,
					fetcher: bc.fetcher,
				}
				childMd := childConverter.Convert(children)
				if childMd != "" {
					// For toggles, wrap children in details block
					if block.Type == "toggle" {
						builder.WriteString(childMd)
						builder.WriteString("</details>\n\n")
					} else {
						builder.WriteString(childMd)
					}
				}
			}
		}
	}

	return builder.String()
}

// convertBlock converts a single Notion block to markdown.
func (bc *BlockConverter) convertBlock(block notionBlock, numberedCounter *int) string {
	indent := strings.Repeat("  ", bc.depth)

	switch block.Type {
	case "paragraph":
		if block.Paragraph == nil {
			return "\n"
		}
		text := renderRichText(block.Paragraph.RichText)
		if text == "" {
			return "\n"
		}
		return fmt.Sprintf("%s%s\n\n", indent, text)

	case "heading_1":
		if block.Heading1 == nil {
			return ""
		}
		text := renderRichText(block.Heading1.RichText)
		return fmt.Sprintf("# %s\n\n", text)

	case "heading_2":
		if block.Heading2 == nil {
			return ""
		}
		text := renderRichText(block.Heading2.RichText)
		return fmt.Sprintf("## %s\n\n", text)

	case "heading_3":
		if block.Heading3 == nil {
			return ""
		}
		text := renderRichText(block.Heading3.RichText)
		return fmt.Sprintf("### %s\n\n", text)

	case "bulleted_list_item":
		if block.BulletedList == nil {
			return ""
		}
		text := renderRichText(block.BulletedList.RichText)
		return fmt.Sprintf("%s- %s\n", indent, text)

	case "numbered_list_item":
		if block.NumberedList == nil {
			return ""
		}
		*numberedCounter++
		text := renderRichText(block.NumberedList.RichText)
		return fmt.Sprintf("%s%d. %s\n", indent, *numberedCounter, text)

	case "code":
		if block.Code == nil {
			return ""
		}
		text := renderRichText(block.Code.RichText)
		lang := block.Code.Language
		if lang == "plain text" {
			lang = ""
		}
		return fmt.Sprintf("%s```%s\n%s%s\n%s```\n\n", indent, lang, indent, text, indent)

	case "quote":
		if block.Quote == nil {
			return ""
		}
		text := renderRichText(block.Quote.RichText)
		lines := strings.Split(text, "\n")
		var result strings.Builder
		for _, line := range lines {
			result.WriteString(fmt.Sprintf("%s> %s\n", indent, line))
		}
		result.WriteString("\n")
		return result.String()

	case "to_do":
		if block.ToDo == nil {
			return ""
		}
		text := renderRichText(block.ToDo.RichText)
		check := " "
		if block.ToDo.Checked {
			check = "x"
		}
		return fmt.Sprintf("%s- [%s] %s\n", indent, check, text)

	case "divider":
		return fmt.Sprintf("%s---\n\n", indent)

	case "callout":
		if block.Callout == nil {
			return ""
		}
		text := renderRichText(block.Callout.RichText)
		emoji := ""
		if block.Callout.Icon != nil && block.Callout.Icon.Emoji != "" {
			emoji = block.Callout.Icon.Emoji + " "
		}
		return fmt.Sprintf("%s> %s%s\n\n", indent, emoji, text)

	case "toggle":
		if block.Toggle == nil {
			return ""
		}
		text := renderRichText(block.Toggle.RichText)
		return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n", text)

	case "table":
		// Table blocks are containers — actual content comes from child table_row blocks
		return ""

	case "table_row":
		if block.TableRow == nil {
			return ""
		}
		var cells []string
		for _, cell := range block.TableRow.Cells {
			cells = append(cells, renderRichText(cell))
		}
		return fmt.Sprintf("| %s |\n", strings.Join(cells, " | "))

	case "child_page":
		if block.ChildPage == nil {
			return ""
		}
		return fmt.Sprintf("%s📄 **%s** (child page)\n\n", indent, block.ChildPage.Title)

	case "image":
		if block.Image == nil {
			return ""
		}
		url := ""
		if block.Image.External != nil {
			url = block.Image.External.URL
		} else if block.Image.File != nil {
			url = block.Image.File.URL
		}
		caption := renderRichText(block.Image.Caption)
		if caption == "" {
			caption = "image"
		}
		return fmt.Sprintf("%s![%s](%s)\n\n", indent, caption, url)

	case "bookmark":
		if block.Bookmark == nil {
			return ""
		}
		caption := renderRichText(block.Bookmark.Caption)
		if caption == "" {
			caption = block.Bookmark.URL
		}
		return fmt.Sprintf("%s[%s](%s)\n\n", indent, caption, block.Bookmark.URL)

	default:
		// Unsupported block type — skip gracefully
		return fmt.Sprintf("%s<!-- unsupported block: %s -->\n", indent, block.Type)
	}
}

// renderRichText converts a Notion rich text array to markdown inline formatting.
// Annotations are applied from inside out: code > link > bold/italic/strikethrough
func renderRichText(texts []richText) string {
	var builder strings.Builder

	for _, t := range texts {
		text := t.PlainText

		// Apply annotations from inside out
		if t.Annotations.Code {
			text = "`" + text + "`"
		}

		if t.Href != "" {
			text = fmt.Sprintf("[%s](%s)", text, t.Href)
		}

		if t.Annotations.Bold {
			text = "**" + text + "**"
		}

		if t.Annotations.Italic {
			text = "*" + text + "*"
		}

		if t.Annotations.Strikethrough {
			text = "~~" + text + "~~"
		}

		builder.WriteString(text)
	}

	return builder.String()
}

// AddTableSeparator injects a markdown table separator after the first table_row
// if it looks like a header row. Called after converting all blocks.
func AddTableSeparator(markdown string) string {
	// Find first table row and add separator after it
	lines := strings.Split(markdown, "\n")
	var result []string
	headerSeen := false

	for _, line := range lines {
		result = append(result, line)
		if !headerSeen && strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			// Count cells for separator
			cells := strings.Count(line, "|") - 1
			sep := "|"
			for i := 0; i < cells; i++ {
				sep += " --- |"
			}
			result = append(result, sep)
			headerSeen = true
		}
	}

	return strings.Join(result, "\n")
}
