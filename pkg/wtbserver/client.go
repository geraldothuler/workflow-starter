package wtbserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

// Client connects to the wtb daemon over a Unix socket.
type Client struct {
	http     *http.Client
	sockPath string
}

// DefaultClient returns a client for the default socket path.
// If the daemon is not running, it auto-starts it (Fase 4).
func DefaultClient() *Client {
	sock := SockPath()
	c := newClient(sock)
	if !c.IsRunning() {
		if err := autoStart(sock); err == nil {
			// Wait up to 3s for daemon to become ready
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
				if c.IsRunning() {
					break
				}
			}
		}
	}
	return c
}

func newClient(sockPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 1 * time.Second}).DialContext(ctx, "unix", sockPath)
		},
	}
	return &Client{
		http:     &http.Client{Transport: transport, Timeout: 30 * time.Second},
		sockPath: sockPath,
	}
}

// IsRunning returns true if the daemon is reachable.
func (c *Client) IsRunning() bool {
	resp, err := c.http.Get("http://local/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// Get sends a GET request and streams the response text to stdout.
// Returns (true, nil) on success, (false, nil) if daemon not running.
func (c *Client) Get(path string, params url.Values) (bool, error) {
	u := "http://local" + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	resp, err := c.http.Get(u)
	if err != nil {
		return false, nil // daemon not running → caller falls back to direct
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return true, fmt.Errorf("%s", bytes.TrimSpace(body))
	}
	os.Stdout.Write(body)
	return true, nil
}

// Post sends a POST request with a JSON body and streams the response text to stdout.
// Returns (true, nil) on success, (false, nil) if daemon not running.
func (c *Client) Post(path string, params url.Values, bodyObj interface{}) (bool, error) {
	u := "http://local" + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	payload, err := json.Marshal(bodyObj)
	if err != nil {
		return true, fmt.Errorf("marshal body: %w", err)
	}
	resp, err := c.http.Post(u, "application/json", bytes.NewReader(payload))
	if err != nil {
		return false, nil // daemon not running
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return true, fmt.Errorf("%s", bytes.TrimSpace(body))
	}
	os.Stdout.Write(body)
	return true, nil
}

// PostJSON sends a POST request and decodes the JSON response into dest.
func (c *Client) PostJSON(path string, params url.Values, bodyObj interface{}, dest interface{}) (bool, error) {
	u := "http://local" + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	payload, err := json.Marshal(bodyObj)
	if err != nil {
		return true, fmt.Errorf("marshal body: %w", err)
	}
	resp, err := c.http.Post(u, "application/json", bytes.NewReader(payload))
	if err != nil {
		return false, nil // daemon not running
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return true, fmt.Errorf("%s", bytes.TrimSpace(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return true, fmt.Errorf("decode response: %w", err)
	}
	return true, nil
}

// IndexAsync submits a repo index job and streams its output to stdout.
// Blocks until the job completes.
func (c *Client) IndexAsync(params url.Values) (bool, error) {
	// Submit job
	var jobResp struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	ok, err := c.PostJSON("/repo/index", params, struct{}{}, &jobResp)
	if !ok || err != nil {
		return ok, err
	}

	// Stream output
	streamClient := newClient(c.sockPath)
	streamClient.http.Timeout = 30 * time.Minute // long timeout for indexing

	streamURL := "http://local/jobs/stream?id=" + jobResp.JobID
	resp, err := streamClient.http.Get(streamURL)
	if err != nil {
		// Fall back to polling
		return c.pollJobUntilDone(jobResp.JobID)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	return true, nil
}

// pollJobUntilDone polls job status and prints output incrementally.
func (c *Client) pollJobUntilDone(jobID string) (bool, error) {
	var offset int
	for {
		time.Sleep(500 * time.Millisecond)

		resp, err := c.http.Get("http://local/jobs/status?id=" + jobID)
		if err != nil {
			return true, err
		}
		var st struct {
			Status string `json:"status"`
			Err    string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&st)
		resp.Body.Close()

		// Print any new output
		chunk, _ := c.Get("/jobs/stream", url.Values{"id": []string{jobID}, "offset": []string{fmt.Sprintf("%d", offset)}})
		_ = chunk
		offset += 0 // offset tracking simplified — stream handles it server-side

		if st.Status == "done" {
			return true, nil
		}
		if st.Status == "failed" {
			return true, fmt.Errorf("indexing failed: %s", st.Err)
		}
	}
}

// autoStart forks the daemon in the background.
func autoStart(sockPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "serve", "--daemon")
	cmd.Env = append(os.Environ()) // inherit env (WTB_REPO_ROOT, etc.)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	return cmd.Start()
}
