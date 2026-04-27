package ops

import (
	"testing"
)

func TestCheckWebSearch_MissingCredentials(t *testing.T) {
	r := CheckWebSearch(WebSearchConfig{})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckWebSearch_HappyPath(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{
		"searchInformation": {"totalResults": "100"},
		"items": [
			{"title": "Result 1", "link": "https://example.com/1", "snippet": "First result"},
			{"title": "Result 2", "link": "https://example.com/2", "snippet": "Second result"}
		]
	}`)

	r := CheckWebSearch(WebSearchConfig{APIKey: "key", CSEID: "cse", Query: "test"})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

func TestCheckWebSearch_API500(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(500, `error`)
	r := CheckWebSearch(WebSearchConfig{APIKey: "key", CSEID: "cse", Query: "test"})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}
