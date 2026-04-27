package exporter

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
	// Default timeout is tested in pkg/transport. Here we just verify the constructor works.
	tr := NewHTTPTransport("http://example.com", "bearer", "tok", nil, 0)
	if tr == nil {
		t.Error("expected non-nil transport")
	}
}
