package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPTransport_BasicAuth(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "basic", "dXNlcjpwYXNz", nil, 10*time.Second)

	_, _, err := transport.Execute(context.Background(), "GET", "/test", "", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	expected := "Basic dXNlcjpwYXNz"
	if receivedAuth != expected {
		t.Errorf("auth header = %q, want %q", receivedAuth, expected)
	}
}

func TestHTTPTransport_BearerAuth(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "bearer", "my-token-123", nil, 10*time.Second)

	_, _, err := transport.Execute(context.Background(), "GET", "/test", "", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	expected := "Bearer my-token-123"
	if receivedAuth != expected {
		t.Errorf("auth header = %q, want %q", receivedAuth, expected)
	}
}

func TestHTTPTransport_PostWithBody(t *testing.T) {
	var receivedBody string
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-123", "id": 12345})
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "basic", "dG9rZW4=", map[string]string{
		"Content-Type": "application/json",
	}, 10*time.Second)

	body := `{"fields":{"summary":"Test Epic"}}`
	resp, statusCode, err := transport.Execute(context.Background(), "POST", "/issue", body, nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if statusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", statusCode, http.StatusCreated)
	}

	if receivedContentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", receivedContentType)
	}

	if receivedBody != body {
		t.Errorf("body = %q, want %q", receivedBody, body)
	}

	// Parse response
	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("response parse error: %v", err)
	}
	if result["key"] != "PROJ-123" {
		t.Errorf("response key = %v, want PROJ-123", result["key"])
	}
}

func TestHTTPTransport_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Invalid token"}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "bearer", "bad-token", nil, 10*time.Second)

	_, statusCode, err := transport.Execute(context.Background(), "GET", "/test", "", nil)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if statusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", statusCode, http.StatusUnauthorized)
	}
}

func TestHTTPTransport_ExtraHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "bearer", "tok", map[string]string{
		"Content-Type": "application/json",
	}, 10*time.Second)

	// Extra headers should override defaults
	_, _, err := transport.Execute(context.Background(), "POST", "/test", "{}", map[string]string{
		"Content-Type": "application/json-patch+json",
		"X-Custom":     "value",
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if receivedHeaders.Get("Content-Type") != "application/json-patch+json" {
		t.Errorf("Content-Type = %q, want application/json-patch+json", receivedHeaders.Get("Content-Type"))
	}
	if receivedHeaders.Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %q, want value", receivedHeaders.Get("X-Custom"))
	}
}

func TestHTTPTransport_DefaultTimeout(t *testing.T) {
	transport := NewHTTPTransport("http://example.com", "bearer", "tok", nil, 0)
	if transport.client.Timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", transport.client.Timeout)
	}
}

func TestHTTPTransport_CustomHeaders_NoAuth(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer server.Close()

	// Datadog-style: auth in headers, no Authorization header
	transport := NewHTTPTransport(server.URL, "", "", map[string]string{
		"DD-API-KEY":         "test-api-key",
		"DD-APPLICATION-KEY": "test-app-key",
		"Content-Type":       "application/json",
	}, 10*time.Second)

	_, _, err := transport.Execute(context.Background(), "GET", "/api/v1/hosts", "", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if receivedHeaders.Get("DD-API-KEY") != "test-api-key" {
		t.Errorf("DD-API-KEY = %q, want test-api-key", receivedHeaders.Get("DD-API-KEY"))
	}
	if receivedHeaders.Get("DD-APPLICATION-KEY") != "test-app-key" {
		t.Errorf("DD-APPLICATION-KEY = %q, want test-app-key", receivedHeaders.Get("DD-APPLICATION-KEY"))
	}
	// No Authorization header should be set
	if receivedHeaders.Get("Authorization") != "" {
		t.Errorf("Authorization should be empty, got %q", receivedHeaders.Get("Authorization"))
	}
}

func TestHTTPTransport_BaseURL(t *testing.T) {
	transport := NewHTTPTransport("https://api.datadoghq.com/", "", "", nil, 0)
	if transport.BaseURL() != "https://api.datadoghq.com" {
		t.Errorf("BaseURL() = %q, want trailing slash trimmed", transport.BaseURL())
	}
}

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		body   string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long body", 10, "this is a ..."},
		{"", 10, ""},
	}
	for _, tt := range tests {
		got := TruncateBody([]byte(tt.body), tt.maxLen)
		if got != tt.want {
			t.Errorf("TruncateBody(%q, %d) = %q, want %q", tt.body, tt.maxLen, got, tt.want)
		}
	}
}
