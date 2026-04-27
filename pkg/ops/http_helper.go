package ops

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// httpDo is a package-level variable for HTTP requests, following the
// shellExec/shellOutput pattern from shell.go. Allows test injection.
var httpDo = func(req *http.Request) (*http.Response, error) {
	return (&http.Client{Timeout: 30 * time.Second}).Do(req)
}

// httpGet performs an HTTP GET with custom headers.
func httpGet(url string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpDo(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// httpPost performs an HTTP POST with custom headers and body.
func httpPost(url string, headers map[string]string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpDo(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, err
}
