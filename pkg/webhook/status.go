package webhook

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Status holds the result of a webhook readiness check.
type Status struct {
	SecretGitHub  bool // Keychain workflow-webhook-secret-github exists
	SecretDatadog bool // Keychain workflow-webhook-secret-datadog exists
	DaemonRunning bool // wtb serve responding on Unix socket or :7654
	NgrokInstalled bool // ngrok binary found in PATH
	NgrokRunning  bool // ngrok API responding on localhost:4040
	NgrokURL      string // public URL from ngrok API (empty if not running)
}

// Check probes all webhook prerequisites and returns the current status.
func Check() Status {
	s := Status{}

	_, err := keychainSecret("workflow-webhook-secret-github")
	s.SecretGitHub = err == nil

	_, err = keychainSecret("workflow-webhook-secret-datadog")
	s.SecretDatadog = err == nil

	_, err = exec.LookPath("ngrok")
	s.NgrokInstalled = err == nil

	s.NgrokURL, s.NgrokRunning = ngrokPublicURL()

	s.DaemonRunning = isDaemonRunning()

	return s
}

// ngrokPublicURL queries the ngrok local API for the active public tunnel URL.
func ngrokPublicURL() (string, bool) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:4040/api/tunnels")
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	// Quick parse: look for "public_url":"https://..." without full JSON unmarshal
	raw, _ := io.ReadAll(resp.Body)
	body := string(raw)

	const marker = `"public_url":"`
	idx := strings.Index(body, marker)
	if idx < 0 {
		return "", false
	}
	start := idx + len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		return "", false
	}
	url := body[start : start+end]
	return url, url != ""
}

// isDaemonRunning checks if the wtb daemon is responding on its Unix socket.
func isDaemonRunning() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	sockPath := filepath.Join(home, ".wtb", "wtb.sock")
	if _, err := os.Stat(sockPath); err != nil {
		return false
	}
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sockPath)
			},
		},
	}
	resp, err := client.Get("http://unix/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Report returns a human-readable status report with actionable gaps.
func (s Status) Report() string {
	var b strings.Builder

	b.WriteString("=== wtb webhook status ===\n\n")

	check := func(ok bool, label string) {
		if ok {
			fmt.Fprintf(&b, "  ✓ %s\n", label)
		} else {
			fmt.Fprintf(&b, "  ✗ %s\n", label)
		}
	}

	check(s.SecretGitHub, "Keychain: workflow-webhook-secret-github")
	check(s.SecretDatadog, "Keychain: workflow-webhook-secret-datadog")
	check(s.DaemonRunning, "wtb daemon rodando (:7654)")
	check(s.NgrokInstalled, "ngrok instalado")
	if s.NgrokRunning {
		fmt.Fprintf(&b, "  ✓ ngrok ativo: %s\n", s.NgrokURL)
	} else {
		fmt.Fprintf(&b, "  ✗ ngrok não está rodando\n")
	}

	b.WriteString("\n")

	if s.IsReady() {
		b.WriteString("Status: pronto — webhooks ativos.\n")
		if s.NgrokURL != "" {
			fmt.Fprintf(&b, "\nURLs de webhook:\n")
			fmt.Fprintf(&b, "  GitHub  : %s/webhooks/github\n", s.NgrokURL)
			fmt.Fprintf(&b, "  Datadog : %s/webhooks/datadog\n", s.NgrokURL)
		}
		return b.String()
	}

	b.WriteString("Status: não pronto. Execute: wtb webhook setup\n")
	return b.String()
}

// IsReady returns true when all prerequisites are satisfied.
func (s Status) IsReady() bool {
	return s.SecretGitHub && s.SecretDatadog && s.DaemonRunning && s.NgrokInstalled && s.NgrokRunning
}
