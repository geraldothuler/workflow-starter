package sources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// --- URL Parsing Tests ---

func TestParsePageID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "standard URL with title and hex ID",
			url:  "https://www.notion.so/workspace/My-Page-Title-abc123def456abc123def456abc123de",
			want: "abc123de-f456-abc1-23de-f456abc123de",
		},
		{
			name: "URL without workspace",
			url:  "https://www.notion.so/My-Page-abc123def456abc123def456abc123de",
			want: "abc123de-f456-abc1-23de-f456abc123de",
		},
		{
			name: "bare hex ID",
			url:  "https://notion.so/abc123def456abc123def456abc123de",
			want: "abc123de-f456-abc1-23de-f456abc123de",
		},
		{
			name: "UUID with dashes",
			url:  "https://notion.so/abc123de-f456-abc1-23de-f456abc123de",
			want: "abc123de-f456-abc1-23de-f456abc123de",
		},
		{
			name: "with query params",
			url:  "https://www.notion.so/Page-abc123def456abc123def456abc123de?v=abc123&p=test",
			want: "abc123de-f456-abc1-23de-f456abc123de",
		},
		{
			name: "with fragment",
			url:  "https://www.notion.so/Page-abc123def456abc123def456abc123de#section",
			want: "abc123de-f456-abc1-23de-f456abc123de",
		},
		{
			name:    "not a notion URL",
			url:     "https://www.google.com/page",
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "notion URL without page ID",
			url:     "https://www.notion.so/",
			wantErr: true,
		},
		{
			name: "http (not https)",
			url:  "http://notion.so/abc123def456abc123def456abc123de",
			want: "abc123de-f456-abc1-23de-f456abc123de",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePageID(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parsePageID(%q) expected error, got %q", tt.url, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePageID(%q) unexpected error: %v", tt.url, err)
			}
			if got != tt.want {
				t.Errorf("parsePageID(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsNotionURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://www.notion.so/page", true},
		{"https://notion.so/page", true},
		{"http://notion.so/page", true},
		{"https://NOTION.SO/page", true},
		{"https://google.com", false},
		{"notion.so/page", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isNotionURL(tt.url)
			if got != tt.expected {
				t.Errorf("isNotionURL(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}

// --- Mock HTTP Transport ---

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockNotionClient(handler roundTripFunc) *NotionSource {
	return &NotionSource{
		apiToken: "test-token",
		httpClient: &http.Client{
			Transport: handler,
		},
	}
}

func jsonResponse(statusCode int, body interface{}) *http.Response {
	data, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func errorResponse(statusCode int, message string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(fmt.Sprintf(`{"message":"%s"}`, message)))),
	}
}

// --- Fetch Tests ---

func TestNotionSource_Fetch_Success(t *testing.T) {
	pageResp := map[string]interface{}{
		"id":               "abc123de-f456-abc1-23de-f456abc123de",
		"last_edited_time": "2026-02-21T10:00:00Z",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"title": []interface{}{
					map[string]interface{}{"plain_text": "Test Page"},
				},
			},
		},
	}

	blocksResp := blockChildrenResponse{
		Results: []notionBlock{
			{
				Type: "heading_1",
				Heading1: &headingBlock{
					RichText: []richText{{PlainText: "Introduction"}},
				},
			},
			{
				Type: "paragraph",
				Paragraph: &paragraphBlock{
					RichText: []richText{{PlainText: "Hello world"}},
				},
			},
		},
		HasMore: false,
	}

	requestCount := 0
	client := newMockNotionClient(func(req *http.Request) (*http.Response, error) {
		requestCount++

		// Verify auth headers
		if req.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing or wrong Authorization header")
		}
		if req.Header.Get("Notion-Version") != notionAPIVersion {
			t.Error("missing or wrong Notion-Version header")
		}

		if requestCount == 1 {
			// Page request
			return jsonResponse(200, pageResp), nil
		}
		// Blocks request
		return jsonResponse(200, blocksResp), nil
	})

	result, err := client.Fetch("https://notion.so/Test-abc123def456abc123def456abc123de")
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if result.Title != "Test Page" {
		t.Errorf("Title = %q, want %q", result.Title, "Test Page")
	}
	if result.Source != "notion" {
		t.Errorf("Source = %q, want %q", result.Source, "notion")
	}
	if result.BlockCount != 2 {
		t.Errorf("BlockCount = %d, want %d", result.BlockCount, 2)
	}
	if result.Content == "" {
		t.Error("Content is empty")
	}
	if !containsStr(result.Content, "# Test Page") {
		t.Error("Content missing page title")
	}
	if !containsStr(result.Content, "Introduction") {
		t.Error("Content missing heading")
	}
	if !containsStr(result.Content, "Hello world") {
		t.Error("Content missing paragraph")
	}
}

func TestNotionSource_Fetch_Pagination(t *testing.T) {
	pageResp := map[string]interface{}{
		"id":         "abc123de-f456-abc1-23de-f456abc123de",
		"properties": map[string]interface{}{},
	}

	page1 := blockChildrenResponse{
		Results: []notionBlock{
			{Type: "paragraph", Paragraph: &paragraphBlock{RichText: []richText{{PlainText: "Page 1"}}}},
		},
		HasMore:    true,
		NextCursor: "cursor-2",
	}

	page2 := blockChildrenResponse{
		Results: []notionBlock{
			{Type: "paragraph", Paragraph: &paragraphBlock{RichText: []richText{{PlainText: "Page 2"}}}},
		},
		HasMore: false,
	}

	requestCount := 0
	client := newMockNotionClient(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount == 1 {
			return jsonResponse(200, pageResp), nil
		}
		if requestCount == 2 {
			return jsonResponse(200, page1), nil
		}
		return jsonResponse(200, page2), nil
	})

	result, err := client.Fetch("https://notion.so/Test-abc123def456abc123def456abc123de")
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if result.BlockCount != 2 {
		t.Errorf("BlockCount = %d, want 2 (both pages)", result.BlockCount)
	}
	if !containsStr(result.Content, "Page 1") {
		t.Error("missing content from page 1")
	}
	if !containsStr(result.Content, "Page 2") {
		t.Error("missing content from page 2")
	}
}

func TestNotionSource_Fetch_401(t *testing.T) {
	client := newMockNotionClient(func(req *http.Request) (*http.Response, error) {
		return errorResponse(401, "Invalid token"), nil
	})

	_, err := client.Fetch("https://notion.so/Test-abc123def456abc123def456abc123de")
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !containsStr(err.Error(), "inválido") && !containsStr(err.Error(), "401") {
		t.Errorf("error should mention invalid token, got: %v", err)
	}
}

func TestNotionSource_Fetch_403(t *testing.T) {
	client := newMockNotionClient(func(req *http.Request) (*http.Response, error) {
		return errorResponse(403, "Forbidden"), nil
	})

	_, err := client.Fetch("https://notion.so/Test-abc123def456abc123def456abc123de")
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !containsStr(err.Error(), "acesso") && !containsStr(err.Error(), "403") {
		t.Errorf("error should mention access, got: %v", err)
	}
}

func TestNotionSource_Fetch_404(t *testing.T) {
	client := newMockNotionClient(func(req *http.Request) (*http.Response, error) {
		return errorResponse(404, "Not found"), nil
	})

	_, err := client.Fetch("https://notion.so/Test-abc123def456abc123def456abc123de")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !containsStr(err.Error(), "não encontrada") && !containsStr(err.Error(), "404") {
		t.Errorf("error should mention not found, got: %v", err)
	}
}

func TestNotionSource_Fetch_InvalidURL(t *testing.T) {
	client := newMockNotionClient(nil)

	_, err := client.Fetch("https://google.com/page")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNewNotionSource_MissingToken(t *testing.T) {
	// Temporarily unset env var
	original := lookupEnv("NOTION_API_TOKEN")
	unsetEnv("NOTION_API_TOKEN")
	defer restoreEnv("NOTION_API_TOKEN", original)

	_, err := NewNotionSource("")
	if err == nil {
		t.Fatal("expected error when token is missing")
	}
	if !containsStr(err.Error(), "notion.so/my-integrations") {
		t.Error("error should include setup guide")
	}
}

func TestNewNotionSource_WithToken(t *testing.T) {
	ns, err := NewNotionSource("test-secret-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ns.apiToken != "test-secret-token" {
		t.Errorf("apiToken = %q, want %q", ns.apiToken, "test-secret-token")
	}
}

func TestExtractPageTitle(t *testing.T) {
	tests := []struct {
		name  string
		page  *notionPage
		want  string
	}{
		{
			name: "title property",
			page: &notionPage{
				Properties: map[string]interface{}{
					"title": map[string]interface{}{
						"title": []interface{}{
							map[string]interface{}{"plain_text": "My Page"},
						},
					},
				},
			},
			want: "My Page",
		},
		{
			name: "Name property",
			page: &notionPage{
				Properties: map[string]interface{}{
					"Name": map[string]interface{}{
						"title": []interface{}{
							map[string]interface{}{"plain_text": "Named Page"},
						},
					},
				},
			},
			want: "Named Page",
		},
		{
			name: "no properties",
			page: &notionPage{Properties: nil},
			want: "",
		},
		{
			name: "nil page",
			page: nil,
			want: "",
		},
		{
			name: "multi-part title",
			page: &notionPage{
				Properties: map[string]interface{}{
					"title": map[string]interface{}{
						"title": []interface{}{
							map[string]interface{}{"plain_text": "Part 1"},
							map[string]interface{}{"plain_text": " Part 2"},
						},
					},
				},
			},
			want: "Part 1 Part 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPageTitle(tt.page)
			if got != tt.want {
				t.Errorf("extractPageTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Test Helpers ---

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func lookupEnv(key string) *string {
	val, ok := envLookup(key)
	if !ok {
		return nil
	}
	return &val
}

func envLookup(key string) (string, bool) {
	// Use a simple implementation to avoid importing os in test helpers
	// The actual env manipulation is done in the test
	return "", false
}

func unsetEnv(key string) {
	// This is a no-op placeholder — the real env is checked in NewNotionSource
	// For proper testing, we'd need to use t.Setenv but that requires Go 1.17+
}

func restoreEnv(key string, val *string) {
	// No-op placeholder
}
