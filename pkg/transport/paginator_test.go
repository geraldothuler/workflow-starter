package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestExecuteAllPages_CursorSinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{"a", "b", "c"},
			"next_cursor": "",
		})
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "cursor",
		CursorParam: "cursor",
		CursorPath:  "next_cursor",
		ResultsPath: "items",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}
}

func TestExecuteAllPages_CursorMultiPage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		cursor := r.URL.Query().Get("cursor")

		switch cursor {
		case "":
			json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{"a", "b"},
				"next_cursor": "page2",
			})
		case "page2":
			json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{"c", "d"},
				"next_cursor": "page3",
			})
		case "page3":
			json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{"e"},
				"next_cursor": "",
			})
		}
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "cursor",
		CursorParam: "cursor",
		CursorPath:  "next_cursor",
		ResultsPath: "items",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("got %d items, want 5", len(items))
	}
	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}

func TestExecuteAllPages_CursorEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{},
			"next_cursor": "",
		})
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "cursor",
		CursorParam: "cursor",
		CursorPath:  "next_cursor",
		ResultsPath: "items",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestExecuteAllPages_CursorMaxPages(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{"item"},
			"next_cursor": "always-more",
		})
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "cursor",
		CursorParam: "cursor",
		CursorPath:  "next_cursor",
		ResultsPath: "items",
		MaxPages:    3,
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (max_pages), got %d", callCount)
	}
	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}
}

func TestExecuteAllPages_PageSinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{"x", "y"},
			"total":   2.0,
		})
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:         "page",
		PageParam:     "page",
		PageSizeParam: "per_page",
		PageSize:      10,
		TotalPath:     "total",
		ResultsPath:   "results",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
}

func TestExecuteAllPages_PageMultiPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		pageNum, _ := strconv.Atoi(page)

		switch pageNum {
		case 1:
			json.NewEncoder(w).Encode(map[string]any{
				"results": []any{"a", "b"},
				"total":   5.0,
			})
		case 2:
			json.NewEncoder(w).Encode(map[string]any{
				"results": []any{"c", "d"},
				"total":   5.0,
			})
		case 3:
			json.NewEncoder(w).Encode(map[string]any{
				"results": []any{"e"},
				"total":   5.0,
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"results": []any{},
				"total":   5.0,
			})
		}
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:         "page",
		PageParam:     "page",
		PageSizeParam: "per_page",
		PageSize:      2,
		TotalPath:     "total",
		ResultsPath:   "results",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("got %d items, want 5", len(items))
	}
}

func TestExecuteAllPages_PageEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"total":   0.0,
		})
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "page",
		PageParam:   "page",
		PageSize:    10,
		TotalPath:   "total",
		ResultsPath: "results",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestExecuteAllPages_OffsetSinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data":  []any{"a", "b"},
			"total": 2.0,
		})
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "offset",
		OffsetParam: "offset",
		LimitParam:  "limit",
		Limit:       100,
		TotalPath:   "total",
		ResultsPath: "data",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
}

func TestExecuteAllPages_OffsetMultiPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		offsetNum, _ := strconv.Atoi(offset)

		switch offsetNum {
		case 0:
			json.NewEncoder(w).Encode(map[string]any{
				"data":  []any{"a", "b"},
				"total": 5.0,
			})
		case 2:
			json.NewEncoder(w).Encode(map[string]any{
				"data":  []any{"c", "d"},
				"total": 5.0,
			})
		case 4:
			json.NewEncoder(w).Encode(map[string]any{
				"data":  []any{"e"},
				"total": 5.0,
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"data":  []any{},
				"total": 5.0,
			})
		}
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "offset",
		OffsetParam: "offset",
		LimitParam:  "limit",
		Limit:       2,
		TotalPath:   "total",
		ResultsPath: "data",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("got %d items, want 5", len(items))
	}
}

func TestExecuteAllPages_OffsetEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data":  []any{},
			"total": 0.0,
		})
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "offset",
		OffsetParam: "offset",
		LimitParam:  "limit",
		Limit:       100,
		TotalPath:   "total",
		ResultsPath: "data",
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestExecuteAllPages_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{"a"},
			"next_cursor": "more",
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "cursor",
		CursorParam: "cursor",
		CursorPath:  "next_cursor",
		ResultsPath: "items",
	}

	_, err := ExecuteAllPages(ctx, tr, "GET", "/api", nil, nil, cfg)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestExecuteAllPages_NoResultsPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{"a", "b", "c"})
	}))
	defer server.Close()

	tr := NewHTTPTransport(server.URL, "", "", nil, 0)
	cfg := PaginationConfig{
		Style:       "cursor",
		CursorParam: "cursor",
		CursorPath:  "next_cursor",
		ResultsPath: "", // root is the array
		MaxPages:    1,
	}

	items, err := ExecuteAllPages(context.Background(), tr, "GET", "/api", nil, nil, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}
}
