package sources

// notionPage represents a Notion page metadata response
type notionPage struct {
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties"`
	URL        string                 `json:"url"`
	CreatedBy  struct {
		ID string `json:"id"`
	} `json:"created_by"`
	LastEditedTime string `json:"last_edited_time"`
}

// notionBlock represents a single Notion block
type notionBlock struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	HasChildren bool   `json:"has_children"`

	Paragraph    *paragraphBlock `json:"paragraph,omitempty"`
	Heading1     *headingBlock   `json:"heading_1,omitempty"`
	Heading2     *headingBlock   `json:"heading_2,omitempty"`
	Heading3     *headingBlock   `json:"heading_3,omitempty"`
	BulletedList *listItemBlock  `json:"bulleted_list_item,omitempty"`
	NumberedList *listItemBlock  `json:"numbered_list_item,omitempty"`
	Code         *codeBlock      `json:"code,omitempty"`
	Quote        *quoteBlock     `json:"quote,omitempty"`
	Callout      *calloutBlock   `json:"callout,omitempty"`
	Toggle       *toggleBlock    `json:"toggle,omitempty"`
	ToDo         *toDoBlock      `json:"to_do,omitempty"`
	Divider      *struct{}       `json:"divider,omitempty"`
	Table        *tableBlock     `json:"table,omitempty"`
	TableRow     *tableRowBlock  `json:"table_row,omitempty"`
	ChildPage    *childPageBlock `json:"child_page,omitempty"`
	Image        *imageBlock     `json:"image,omitempty"`
	Bookmark     *bookmarkBlock  `json:"bookmark,omitempty"`
}

// paragraphBlock is a paragraph
type paragraphBlock struct {
	RichText []richText `json:"rich_text"`
}

// headingBlock is a heading (1, 2, or 3)
type headingBlock struct {
	RichText []richText `json:"rich_text"`
}

// listItemBlock is a bulleted or numbered list item
type listItemBlock struct {
	RichText []richText `json:"rich_text"`
}

// codeBlock is a code block with optional language
type codeBlock struct {
	RichText []richText `json:"rich_text"`
	Language string     `json:"language"`
}

// quoteBlock is a quote
type quoteBlock struct {
	RichText []richText `json:"rich_text"`
}

// calloutBlock is a callout with emoji
type calloutBlock struct {
	RichText []richText `json:"rich_text"`
	Icon     *icon      `json:"icon,omitempty"`
}

// toggleBlock is a toggle (collapsible)
type toggleBlock struct {
	RichText []richText `json:"rich_text"`
}

// toDoBlock is a to-do item
type toDoBlock struct {
	RichText []richText `json:"rich_text"`
	Checked  bool       `json:"checked"`
}

// tableBlock is a table container
type tableBlock struct {
	TableWidth      int  `json:"table_width"`
	HasColumnHeader bool `json:"has_column_header"`
	HasRowHeader    bool `json:"has_row_header"`
}

// tableRowBlock is a single row in a table
type tableRowBlock struct {
	Cells [][]richText `json:"cells"`
}

// childPageBlock is a reference to a child page
type childPageBlock struct {
	Title string `json:"title"`
}

// imageBlock is an image
type imageBlock struct {
	Type     string    `json:"type"` // "file" or "external"
	File     *fileRef  `json:"file,omitempty"`
	External *fileRef  `json:"external,omitempty"`
	Caption  []richText `json:"caption"`
}

// bookmarkBlock is a bookmark (URL)
type bookmarkBlock struct {
	URL     string     `json:"url"`
	Caption []richText `json:"caption"`
}

// icon represents an emoji or external icon
type icon struct {
	Type  string `json:"type"` // "emoji" or "external"
	Emoji string `json:"emoji,omitempty"`
}

// fileRef is a reference to a file (internal or external URL)
type fileRef struct {
	URL string `json:"url"`
}

// richText represents Notion's rich text object
type richText struct {
	Type        string      `json:"type"`
	PlainText   string      `json:"plain_text"`
	Annotations annotations `json:"annotations"`
	Href        string      `json:"href,omitempty"`
}

// annotations are formatting applied to rich text
type annotations struct {
	Bold          bool `json:"bold"`
	Italic        bool `json:"italic"`
	Strikethrough bool `json:"strikethrough"`
	Underline     bool `json:"underline"`
	Code          bool `json:"code"`
}

// blockChildrenResponse is the paginated response from blocks/{id}/children
type blockChildrenResponse struct {
	Results    []notionBlock `json:"results"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor"`
}
