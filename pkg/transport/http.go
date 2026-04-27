package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPTransport handles HTTP communication with external APIs.
type HTTPTransport struct {
	client  *http.Client
	baseURL string
	auth    string            // pre-computed Authorization header value
	headers map[string]string // default headers for all requests
}

// NewHTTPTransport creates an HTTP transport from an expanded transport spec.
// The spec's BaseURL and AuthValue should already be template-expanded with credentials.
func NewHTTPTransport(baseURL, authType, authValue string, headers map[string]string, timeout time.Duration) *HTTPTransport {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Build auth header value
	var auth string
	switch strings.ToLower(authType) {
	case "basic":
		auth = "Basic " + authValue
	case "bearer":
		auth = "Bearer " + authValue
	default:
		auth = authValue
	}

	return &HTTPTransport{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL: strings.TrimRight(baseURL, "/"),
		auth:    auth,
		headers: headers,
	}
}

// Execute sends an HTTP request and returns the parsed JSON response.
func (t *HTTPTransport) Execute(ctx context.Context, method, path, body string, extraHeaders map[string]string) (json.RawMessage, int, error) {
	url := t.baseURL + path

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	// Set auth
	if t.auth != "" {
		req.Header.Set("Authorization", t.auth)
	}

	// Set per-call headers (override defaults)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return json.RawMessage(respBody), resp.StatusCode, fmt.Errorf(
			"HTTP %d %s: %s",
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
			TruncateBody(respBody, 500),
		)
	}

	return json.RawMessage(respBody), resp.StatusCode, nil
}

// BaseURL returns the configured base URL.
func (t *HTTPTransport) BaseURL() string {
	return t.baseURL
}

// TruncateBody truncates a response body for error messages.
func TruncateBody(body []byte, maxLen int) string {
	s := string(body)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
