package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	notionAPIBase    = "https://api.notion.com/v1"
	notionAPIVersion = "2022-06-28"
	notionPageSize   = 100
	maxRetries       = 3
)

// NotionSource fetches and converts Notion pages to markdown.
type NotionSource struct {
	apiToken   string
	httpClient *http.Client
}

// NewNotionSource creates a Notion source with the given API token.
// If apiToken is empty, reads from NOTION_API_TOKEN env var.
func NewNotionSource(apiToken string) (*NotionSource, error) {
	if apiToken == "" {
		apiToken = os.Getenv("NOTION_API_TOKEN")
	}
	if apiToken == "" {
		ns := &NotionSource{}
		return nil, fmt.Errorf("Notion API token não configurado.\n\n%s", ns.SetupGuide())
	}
	return &NotionSource{
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Name returns "notion".
func (n *NotionSource) Name() string { return "notion" }

// CanHandle checks if the URL is a Notion page URL.
func (n *NotionSource) CanHandle(url string) bool {
	return isNotionURL(url)
}

// SetupGuide returns a tutorial for configuring the Notion API token.
func (n *NotionSource) SetupGuide() string {
	return `📋 Como obter seu token Notion:
  1. Acesse https://www.notion.so/my-integrations
  2. Clique "New integration"
  3. Dê um nome (ex: "Workflow Platform Import")
  4. Copie o "Internal Integration Secret"
  5. Configure: export NOTION_API_TOKEN='secret_xxx...'

🔗 Depois, compartilhe a página com sua integração:
  1. Abra a página no Notion
  2. Clique "..." → "Connections" → "Add connections"
  3. Selecione sua integração "Workflow Platform Import"

Mais info: https://developers.notion.com/docs/getting-started`
}

// ensureAuth lazily initializes the API token from env if not set.
// Returns error with setup guide if token is missing.
func (n *NotionSource) ensureAuth() error {
	if n.apiToken != "" {
		return nil
	}
	n.apiToken = os.Getenv("NOTION_API_TOKEN")
	if n.apiToken == "" {
		return fmt.Errorf("Notion API token não configurado.\n\n%s", n.SetupGuide())
	}
	if n.httpClient == nil {
		n.httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return nil
}

// Fetch retrieves a Notion page and converts it to markdown.
func (n *NotionSource) Fetch(url string) (*FetchResult, error) {
	// Lazy auth — check token before making API calls
	if err := n.ensureAuth(); err != nil {
		return nil, err
	}

	pageID, err := parsePageID(url)
	if err != nil {
		return nil, fmt.Errorf("URL Notion inválida: %w", err)
	}

	// 1. Fetch page metadata (title)
	page, err := n.fetchPage(pageID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar página: %w", err)
	}

	title := extractPageTitle(page)

	// 2. Fetch all blocks recursively
	blocks, err := n.fetchAllBlocks(pageID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar blocos: %w", err)
	}

	// 3. Convert blocks to markdown
	converter := NewBlockConverter(func(blockID string) ([]notionBlock, error) {
		return n.fetchAllBlocks(blockID)
	})

	markdown := converter.Convert(blocks)

	// Add title as H1
	if title != "" {
		markdown = fmt.Sprintf("# %s\n\n%s", title, markdown)
	}

	return &FetchResult{
		Content:    markdown,
		Title:      title,
		URL:        url,
		Source:     "notion",
		BlockCount: countBlocks(blocks),
		Metadata: map[string]string{
			"page_id":     pageID,
			"last_edited": page.LastEditedTime,
		},
	}, nil
}

// --- URL Parsing ---

// notionURLPattern matches Notion page URLs
var notionURLPattern = regexp.MustCompile(`^https?://(www\.)?notion\.so/`)

// hexIDPattern matches a 32-char hex string (Notion page ID without dashes)
var hexIDPattern = regexp.MustCompile(`[0-9a-f]{32}`)

// uuidPattern matches a UUID with dashes
var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// isNotionURL checks if a string looks like a Notion URL.
func isNotionURL(url string) bool {
	return notionURLPattern.MatchString(strings.ToLower(url))
}

// parsePageID extracts and formats the page ID from a Notion URL.
// Notion URLs can have several formats:
//   - https://www.notion.so/workspace/Page-Title-{32hex}
//   - https://www.notion.so/Page-Title-{32hex}
//   - https://www.notion.so/{32hex}
//   - https://notion.so/{uuid-with-dashes}
//   - With query params ?v=...
//
// Returns the page ID formatted as UUID (with dashes) for the API.
func parsePageID(url string) (string, error) {
	url = strings.TrimSpace(url)

	if !isNotionURL(url) {
		return "", fmt.Errorf("não é uma URL Notion: %s", url)
	}

	// Strip query params
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// Strip fragment
	if idx := strings.Index(url, "#"); idx != -1 {
		url = url[:idx]
	}

	lowered := strings.ToLower(url)

	// Try UUID format first (with dashes)
	if match := uuidPattern.FindString(lowered); match != "" {
		return match, nil
	}

	// Try 32-char hex (without dashes) — take the LAST match (page ID is at the end)
	matches := hexIDPattern.FindAllString(lowered, -1)
	if len(matches) > 0 {
		hex := matches[len(matches)-1]
		// Convert to UUID format
		return fmt.Sprintf("%s-%s-%s-%s-%s",
			hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32]), nil
	}

	return "", fmt.Errorf("não foi possível extrair page ID de: %s", url)
}

// --- HTTP Methods ---

// fetchPage retrieves page metadata from the Notion API.
func (n *NotionSource) fetchPage(pageID string) (*notionPage, error) {
	url := fmt.Sprintf("%s/pages/%s", notionAPIBase, pageID)

	body, err := n.doRequest("GET", url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var page notionPage
	if err := json.NewDecoder(body).Decode(&page); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta de página: %w", err)
	}

	return &page, nil
}

// fetchAllBlocks fetches all blocks for a given block ID, handling pagination.
func (n *NotionSource) fetchAllBlocks(blockID string) ([]notionBlock, error) {
	var allBlocks []notionBlock
	cursor := ""

	for {
		resp, err := n.fetchBlockChildren(blockID, cursor)
		if err != nil {
			return nil, err
		}

		allBlocks = append(allBlocks, resp.Results...)

		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}

	return allBlocks, nil
}

// fetchBlockChildren fetches one page of block children.
func (n *NotionSource) fetchBlockChildren(blockID, startCursor string) (*blockChildrenResponse, error) {
	url := fmt.Sprintf("%s/blocks/%s/children?page_size=%d", notionAPIBase, blockID, notionPageSize)
	if startCursor != "" {
		url += "&start_cursor=" + startCursor
	}

	body, err := n.doRequest("GET", url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var resp blockChildrenResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("erro ao decodificar blocos: %w", err)
	}

	return &resp, nil
}

// doRequest performs an HTTP request to the Notion API with auth headers and retry.
func (n *NotionSource) doRequest(method, url string) (io.ReadCloser, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("erro ao criar request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+n.apiToken)
		req.Header.Set("Notion-Version", notionAPIVersion)
		req.Header.Set("Content-Type", "application/json")

		resp, err := n.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("erro na requisição HTTP: %w", err)
			if attempt < maxRetries {
				time.Sleep(backoffDuration(attempt))
				continue
			}
			return nil, lastErr
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return resp.Body, nil

		case http.StatusUnauthorized:
			resp.Body.Close()
			return nil, fmt.Errorf("token Notion inválido ou expirado (401).\n\n%s", n.SetupGuide())

		case http.StatusForbidden:
			resp.Body.Close()
			return nil, fmt.Errorf("sem acesso a esta página (403).\n\nCompartilhe a página com sua integração:\n" +
				"  1. Abra a página no Notion\n" +
				"  2. Clique \"...\" → \"Connections\" → \"Add connections\"\n" +
				"  3. Selecione sua integração")

		case http.StatusNotFound:
			resp.Body.Close()
			return nil, fmt.Errorf("página não encontrada (404). Verifique a URL e se a página existe")

		case http.StatusTooManyRequests:
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limit Notion (429)")
			if attempt < maxRetries {
				time.Sleep(backoffDuration(attempt))
				continue
			}
			return nil, lastErr

		default:
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("Notion API erro %d: %s", resp.StatusCode, string(bodyBytes))
			if resp.StatusCode >= 500 && attempt < maxRetries {
				time.Sleep(backoffDuration(attempt))
				continue
			}
			return nil, lastErr
		}
	}

	return nil, lastErr
}

// backoffDuration returns exponential backoff duration for retry attempts.
func backoffDuration(attempt int) time.Duration {
	base := time.Second
	for i := 0; i < attempt; i++ {
		base *= 2
	}
	return base
}

// --- Helpers ---

// extractPageTitle extracts the title from a Notion page's properties.
func extractPageTitle(page *notionPage) string {
	if page == nil || page.Properties == nil {
		return ""
	}

	// Notion stores title in a property named "title" or "Name"
	for _, key := range []string{"title", "Title", "Name", "name"} {
		prop, ok := page.Properties[key]
		if !ok {
			continue
		}

		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}

		titleArr, ok := propMap["title"]
		if !ok {
			continue
		}

		arr, ok := titleArr.([]interface{})
		if !ok {
			continue
		}

		var parts []string
		for _, item := range arr {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if plainText, ok := itemMap["plain_text"].(string); ok {
				parts = append(parts, plainText)
			}
		}

		if len(parts) > 0 {
			return strings.Join(parts, "")
		}
	}

	return ""
}

// countBlocks counts total blocks including nested
func countBlocks(blocks []notionBlock) int {
	return len(blocks)
}
