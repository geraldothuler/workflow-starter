package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/Cobliteam/workflow-toolkit/pkg/sources"
)

// PaginationConfig configures how to paginate through API responses.
type PaginationConfig struct {
	Style         string // "cursor" | "page" | "offset"
	CursorParam   string // query param name for cursor
	CursorPath    string // dot-path in response for next cursor
	PageParam     string // query param name for page number
	PageSizeParam string // query param name for page size
	PageSize      int
	OffsetParam   string // query param name for offset
	LimitParam    string // query param name for limit
	Limit         int
	TotalPath     string // dot-path in response for total count
	ResultsPath   string // dot-path in response for items array
	MaxPages      int    // safety limit (default: 50)
}

// ExecuteAllPages fetches all pages of a paginated API endpoint and returns
// the accumulated items from ResultsPath across all pages.
func ExecuteAllPages(ctx context.Context, t *HTTPTransport, method, basePath string, params map[string]string, headers map[string]string, cfg PaginationConfig) ([]any, error) {
	maxPages := cfg.MaxPages
	if maxPages <= 0 {
		maxPages = 50
	}

	var allItems []any

	for page := 0; page < maxPages; page++ {
		select {
		case <-ctx.Done():
			return allItems, ctx.Err()
		default:
		}

		// Build path with query params
		fullPath, err := buildPaginatedPath(basePath, params, cfg, page)
		if err != nil {
			return allItems, fmt.Errorf("building paginated path: %w", err)
		}

		resp, _, err := t.Execute(ctx, method, fullPath, "", headers)
		if err != nil {
			return allItems, fmt.Errorf("page %d: %w", page, err)
		}

		var respData any
		if err := json.Unmarshal(resp, &respData); err != nil {
			return allItems, fmt.Errorf("page %d: parsing JSON: %w", page, err)
		}

		// Extract items from this page
		items := extractItemsFromPath(respData, cfg.ResultsPath)
		allItems = append(allItems, items...)

		// Check if we should continue
		if shouldStop(respData, items, cfg, page) {
			break
		}

		// Update pagination state for next iteration
		if cfg.Style == "cursor" {
			nextCursor := sources.ExtractString(respData, cfg.CursorPath)
			if nextCursor == "" {
				break
			}
			if params == nil {
				params = make(map[string]string)
			}
			params[cfg.CursorParam] = nextCursor
		}
	}

	return allItems, nil
}

func buildPaginatedPath(basePath string, params map[string]string, cfg PaginationConfig, pageNum int) (string, error) {
	u, err := url.Parse(basePath)
	if err != nil {
		return "", err
	}

	q := u.Query()

	// Copy existing params
	for k, v := range params {
		q.Set(k, v)
	}

	// Add pagination-specific params
	switch cfg.Style {
	case "cursor":
		// Cursor is set via params map (updated between pages)
	case "page":
		if cfg.PageParam != "" {
			q.Set(cfg.PageParam, strconv.Itoa(pageNum+1)) // 1-indexed
		}
		if cfg.PageSizeParam != "" && cfg.PageSize > 0 {
			q.Set(cfg.PageSizeParam, strconv.Itoa(cfg.PageSize))
		}
	case "offset":
		limit := cfg.Limit
		if limit <= 0 {
			limit = 100
		}
		if cfg.OffsetParam != "" {
			q.Set(cfg.OffsetParam, strconv.Itoa(pageNum*limit))
		}
		if cfg.LimitParam != "" {
			q.Set(cfg.LimitParam, strconv.Itoa(limit))
		}
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func extractItemsFromPath(data any, resultsPath string) []any {
	if resultsPath == "" {
		if items, ok := data.([]any); ok {
			return items
		}
		return nil
	}
	return sources.ExtractSlice(data, resultsPath)
}

func shouldStop(respData any, pageItems []any, cfg PaginationConfig, pageNum int) bool {
	if len(pageItems) == 0 {
		return true
	}

	switch cfg.Style {
	case "page":
		if cfg.PageSize > 0 && len(pageItems) < cfg.PageSize {
			return true
		}
		if cfg.TotalPath != "" {
			total := extractFloat(respData, cfg.TotalPath)
			fetched := (pageNum + 1) * cfg.PageSize
			if fetched >= int(total) {
				return true
			}
		}
	case "offset":
		limit := cfg.Limit
		if limit <= 0 {
			limit = 100
		}
		if len(pageItems) < limit {
			return true
		}
		if cfg.TotalPath != "" {
			total := extractFloat(respData, cfg.TotalPath)
			fetched := (pageNum+1)*limit
			if fetched >= int(total) {
				return true
			}
		}
	case "cursor":
		// Cursor-based: stop is handled by empty cursor check in ExecuteAllPages
		return false
	}

	return false
}

func extractFloat(data any, path string) float64 {
	v := sources.ExtractPath(data, path)
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		var f float64
		fmt.Sscanf(fmt.Sprintf("%v", v), "%f", &f)
		return f
	}
}
