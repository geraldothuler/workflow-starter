package ops

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// WebSearchConfig holds Google Custom Search API settings.
type WebSearchConfig struct {
	APIKey     string
	CSEID      string // Google Custom Search Engine ID
	Query      string
	NumResults int
}

// CheckWebSearch queries Google Custom Search for relevant context.
func CheckWebSearch(cfg WebSearchConfig) OpsResult {
	if cfg.APIKey == "" || cfg.CSEID == "" || cfg.Query == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "WebSearch: missing API key, CSE ID, or query",
			Actions: []string{"set --input websearch-api-key=..., websearch-cse-id=..., websearch-query=..."},
			Cost:    "zero-llm",
		}
	}

	numResults := cfg.NumResults
	if numResults <= 0 {
		numResults = 5
	}

	apiURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=%d",
		cfg.APIKey, cfg.CSEID, url.QueryEscape(cfg.Query), numResults)

	body, statusCode, err := httpGet(apiURL, nil)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("WebSearch API error: %v", err),
			Cost:   "zero-llm",
		}
	}
	if statusCode != 200 {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("WebSearch API returned HTTP %d", statusCode),
			Cost:   "zero-llm",
		}
	}

	return evaluateWebSearch(body)
}

func evaluateWebSearch(body []byte) OpsResult {
	var resp struct {
		SearchInformation struct {
			TotalResults string `json:"totalResults"`
		} `json:"searchInformation"`
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("WebSearch: failed to parse response: %v", err),
			Cost:   "zero-llm",
		}
	}

	snippets := make([]string, 0, len(resp.Items))
	for _, item := range resp.Items {
		snippets = append(snippets, fmt.Sprintf("[%s](%s): %s", item.Title, item.Link, item.Snippet))
	}

	return OpsResult{
		Status: "ok",
		Signal: fmt.Sprintf("WebSearch: %d results found", len(resp.Items)),
		Data: map[string]any{
			"total_results": resp.SearchInformation.TotalResults,
			"result_count":  len(resp.Items),
			"snippets":      snippets,
		},
		Cost: "zero-llm",
	}
}
