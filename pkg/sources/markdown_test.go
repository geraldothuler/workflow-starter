package sources

import (
	"strings"
	"testing"
)

func TestRenderRichText(t *testing.T) {
	tests := []struct {
		name     string
		input    []richText
		expected string
	}{
		{
			name:     "plain text",
			input:    []richText{{PlainText: "hello world"}},
			expected: "hello world",
		},
		{
			name: "bold",
			input: []richText{{
				PlainText:   "bold text",
				Annotations: annotations{Bold: true},
			}},
			expected: "**bold text**",
		},
		{
			name: "italic",
			input: []richText{{
				PlainText:   "italic text",
				Annotations: annotations{Italic: true},
			}},
			expected: "*italic text*",
		},
		{
			name: "code",
			input: []richText{{
				PlainText:   "code",
				Annotations: annotations{Code: true},
			}},
			expected: "`code`",
		},
		{
			name: "strikethrough",
			input: []richText{{
				PlainText:   "deleted",
				Annotations: annotations{Strikethrough: true},
			}},
			expected: "~~deleted~~",
		},
		{
			name: "bold and italic",
			input: []richText{{
				PlainText:   "emphasis",
				Annotations: annotations{Bold: true, Italic: true},
			}},
			expected: "***emphasis***",
		},
		{
			name: "link",
			input: []richText{{
				PlainText: "click here",
				Href:      "https://example.com",
			}},
			expected: "[click here](https://example.com)",
		},
		{
			name: "bold link",
			input: []richText{{
				PlainText:   "bold link",
				Href:        "https://example.com",
				Annotations: annotations{Bold: true},
			}},
			expected: "**[bold link](https://example.com)**",
		},
		{
			name: "mixed segments",
			input: []richText{
				{PlainText: "normal "},
				{PlainText: "bold", Annotations: annotations{Bold: true}},
				{PlainText: " end"},
			},
			expected: "normal **bold** end",
		},
		{
			name:     "empty list",
			input:    []richText{},
			expected: "",
		},
		{
			name:     "nil list",
			input:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderRichText(tt.input)
			if got != tt.expected {
				t.Errorf("renderRichText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBlockConverter_Paragraph(t *testing.T) {
	blocks := []notionBlock{
		{
			Type: "paragraph",
			Paragraph: &paragraphBlock{
				RichText: []richText{{PlainText: "Hello world"}},
			},
		},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)
	expected := "Hello world\n\n"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestBlockConverter_EmptyParagraph(t *testing.T) {
	blocks := []notionBlock{
		{Type: "paragraph", Paragraph: &paragraphBlock{}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)
	expected := "\n"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestBlockConverter_Headings(t *testing.T) {
	blocks := []notionBlock{
		{Type: "heading_1", Heading1: &headingBlock{RichText: []richText{{PlainText: "H1"}}}},
		{Type: "heading_2", Heading2: &headingBlock{RichText: []richText{{PlainText: "H2"}}}},
		{Type: "heading_3", Heading3: &headingBlock{RichText: []richText{{PlainText: "H3"}}}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "# H1") {
		t.Error("missing H1")
	}
	if !strings.Contains(got, "## H2") {
		t.Error("missing H2")
	}
	if !strings.Contains(got, "### H3") {
		t.Error("missing H3")
	}
}

func TestBlockConverter_BulletedList(t *testing.T) {
	blocks := []notionBlock{
		{Type: "bulleted_list_item", BulletedList: &listItemBlock{RichText: []richText{{PlainText: "Item 1"}}}},
		{Type: "bulleted_list_item", BulletedList: &listItemBlock{RichText: []richText{{PlainText: "Item 2"}}}},
		{Type: "bulleted_list_item", BulletedList: &listItemBlock{RichText: []richText{{PlainText: "Item 3"}}}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "- Item 1\n") {
		t.Error("missing bullet 1")
	}
	if !strings.Contains(got, "- Item 2\n") {
		t.Error("missing bullet 2")
	}
}

func TestBlockConverter_NumberedList(t *testing.T) {
	blocks := []notionBlock{
		{Type: "numbered_list_item", NumberedList: &listItemBlock{RichText: []richText{{PlainText: "First"}}}},
		{Type: "numbered_list_item", NumberedList: &listItemBlock{RichText: []richText{{PlainText: "Second"}}}},
		{Type: "numbered_list_item", NumberedList: &listItemBlock{RichText: []richText{{PlainText: "Third"}}}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "1. First\n") {
		t.Error("missing numbered 1")
	}
	if !strings.Contains(got, "2. Second\n") {
		t.Error("missing numbered 2")
	}
	if !strings.Contains(got, "3. Third\n") {
		t.Error("missing numbered 3")
	}
}

func TestBlockConverter_NumberedListResets(t *testing.T) {
	blocks := []notionBlock{
		{Type: "numbered_list_item", NumberedList: &listItemBlock{RichText: []richText{{PlainText: "First"}}}},
		{Type: "paragraph", Paragraph: &paragraphBlock{RichText: []richText{{PlainText: "Break"}}}},
		{Type: "numbered_list_item", NumberedList: &listItemBlock{RichText: []richText{{PlainText: "New first"}}}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "1. First\n") {
		t.Error("missing first list item")
	}
	if !strings.Contains(got, "1. New first\n") {
		t.Error("numbered counter should reset after paragraph")
	}
}

func TestBlockConverter_CodeBlock(t *testing.T) {
	blocks := []notionBlock{
		{
			Type: "code",
			Code: &codeBlock{
				RichText: []richText{{PlainText: "fmt.Println(\"hello\")"}},
				Language: "go",
			},
		},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "```go\n") {
		t.Error("missing language tag")
	}
	if !strings.Contains(got, "fmt.Println") {
		t.Error("missing code content")
	}
	if !strings.Contains(got, "```\n") {
		t.Error("missing closing fence")
	}
}

func TestBlockConverter_CodeBlockPlainText(t *testing.T) {
	blocks := []notionBlock{
		{
			Type: "code",
			Code: &codeBlock{
				RichText: []richText{{PlainText: "some text"}},
				Language: "plain text",
			},
		},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	// "plain text" should become empty lang
	if strings.Contains(got, "```plain text") {
		t.Error("should not include 'plain text' as language")
	}
	if !strings.Contains(got, "```\n") {
		t.Error("missing code fence")
	}
}

func TestBlockConverter_Quote(t *testing.T) {
	blocks := []notionBlock{
		{Type: "quote", Quote: &quoteBlock{RichText: []richText{{PlainText: "Quote text"}}}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "> Quote text") {
		t.Error("missing quote prefix")
	}
}

func TestBlockConverter_ToDo(t *testing.T) {
	blocks := []notionBlock{
		{Type: "to_do", ToDo: &toDoBlock{RichText: []richText{{PlainText: "Done task"}}, Checked: true}},
		{Type: "to_do", ToDo: &toDoBlock{RichText: []richText{{PlainText: "Pending task"}}, Checked: false}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "- [x] Done task") {
		t.Error("missing checked todo")
	}
	if !strings.Contains(got, "- [ ] Pending task") {
		t.Error("missing unchecked todo")
	}
}

func TestBlockConverter_Divider(t *testing.T) {
	blocks := []notionBlock{
		{Type: "divider", Divider: &struct{}{}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "---") {
		t.Error("missing divider")
	}
}

func TestBlockConverter_Callout(t *testing.T) {
	blocks := []notionBlock{
		{
			Type: "callout",
			Callout: &calloutBlock{
				RichText: []richText{{PlainText: "Important note"}},
				Icon:     &icon{Type: "emoji", Emoji: "⚠️"},
			},
		},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "> ⚠️ Important note") {
		t.Error("missing callout with emoji")
	}
}

func TestBlockConverter_Toggle(t *testing.T) {
	blocks := []notionBlock{
		{
			Type:        "toggle",
			Toggle:      &toggleBlock{RichText: []richText{{PlainText: "Click to expand"}}},
			HasChildren: true,
		},
	}

	// Mock fetcher returns child blocks
	fetcher := func(blockID string) ([]notionBlock, error) {
		return []notionBlock{
			{Type: "paragraph", Paragraph: &paragraphBlock{RichText: []richText{{PlainText: "Hidden content"}}}},
		}, nil
	}

	converter := NewBlockConverter(fetcher)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "<details>") {
		t.Error("missing details open tag")
	}
	if !strings.Contains(got, "<summary>Click to expand</summary>") {
		t.Error("missing summary")
	}
	if !strings.Contains(got, "Hidden content") {
		t.Error("missing child content")
	}
	if !strings.Contains(got, "</details>") {
		t.Error("missing details close tag")
	}
}

func TestBlockConverter_Image(t *testing.T) {
	blocks := []notionBlock{
		{
			Type: "image",
			Image: &imageBlock{
				Type:     "external",
				External: &fileRef{URL: "https://example.com/img.png"},
				Caption:  []richText{{PlainText: "My image"}},
			},
		},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "![My image](https://example.com/img.png)") {
		t.Error("missing image markdown")
	}
}

func TestBlockConverter_Bookmark(t *testing.T) {
	blocks := []notionBlock{
		{
			Type: "bookmark",
			Bookmark: &bookmarkBlock{
				URL:     "https://example.com",
				Caption: []richText{{PlainText: "Example Site"}},
			},
		},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "[Example Site](https://example.com)") {
		t.Error("missing bookmark link")
	}
}

func TestBlockConverter_ChildPage(t *testing.T) {
	blocks := []notionBlock{
		{Type: "child_page", ChildPage: &childPageBlock{Title: "Sub Page"}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "Sub Page") {
		t.Error("missing child page title")
	}
}

func TestBlockConverter_UnsupportedBlock(t *testing.T) {
	blocks := []notionBlock{
		{Type: "column_list"},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "<!-- unsupported block: column_list -->") {
		t.Error("missing unsupported block comment")
	}
}

func TestBlockConverter_MixedContent(t *testing.T) {
	blocks := []notionBlock{
		{Type: "heading_1", Heading1: &headingBlock{RichText: []richText{{PlainText: "Title"}}}},
		{Type: "paragraph", Paragraph: &paragraphBlock{RichText: []richText{{PlainText: "Some text."}}}},
		{Type: "bulleted_list_item", BulletedList: &listItemBlock{RichText: []richText{{PlainText: "Bullet"}}}},
		{Type: "divider", Divider: &struct{}{}},
		{Type: "code", Code: &codeBlock{RichText: []richText{{PlainText: "x := 1"}}, Language: "go"}},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "# Title") {
		t.Error("missing title")
	}
	if !strings.Contains(got, "Some text.") {
		t.Error("missing paragraph")
	}
	if !strings.Contains(got, "- Bullet") {
		t.Error("missing bullet")
	}
	if !strings.Contains(got, "---") {
		t.Error("missing divider")
	}
	if !strings.Contains(got, "```go") {
		t.Error("missing code block")
	}
}

func TestBlockConverter_NestedList(t *testing.T) {
	blocks := []notionBlock{
		{
			Type:        "bulleted_list_item",
			BulletedList: &listItemBlock{RichText: []richText{{PlainText: "Parent"}}},
			HasChildren: true,
			ID:          "parent-1",
		},
	}

	fetcher := func(blockID string) ([]notionBlock, error) {
		return []notionBlock{
			{Type: "bulleted_list_item", BulletedList: &listItemBlock{RichText: []richText{{PlainText: "Child"}}}},
		}, nil
	}

	converter := NewBlockConverter(fetcher)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "- Parent\n") {
		t.Error("missing parent bullet")
	}
	if !strings.Contains(got, "  - Child\n") {
		t.Error("missing indented child bullet")
	}
}

func TestBlockConverter_TableRow(t *testing.T) {
	blocks := []notionBlock{
		{
			Type: "table_row",
			TableRow: &tableRowBlock{
				Cells: [][]richText{
					{{PlainText: "Name"}},
					{{PlainText: "Value"}},
				},
			},
		},
	}

	converter := NewBlockConverter(nil)
	got := converter.Convert(blocks)

	if !strings.Contains(got, "| Name | Value |") {
		t.Error("missing table row")
	}
}

func TestAddTableSeparator(t *testing.T) {
	input := "| Name | Value |\n| Alice | 100 |\n"
	got := AddTableSeparator(input)

	if !strings.Contains(got, "| --- | --- |") {
		t.Error("missing table separator")
	}
}
