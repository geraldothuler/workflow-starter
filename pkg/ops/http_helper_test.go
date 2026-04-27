package ops

import (
	"io"
	"net/http"
	"strings"
)

// mockHTTPResponse creates a mock httpDo function that returns a fixed response.
func mockHTTPResponse(statusCode int, body string) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}, nil
	}
}
