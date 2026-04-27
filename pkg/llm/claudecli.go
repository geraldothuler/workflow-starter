package llm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeCLIProvider uses the local `claude -p` CLI (Claude Code) as the LLM backend.
// No API key needed — uses the active Claude Code OAuth session.
type ClaudeCLIProvider struct {
	systemPrompt string
	model        string
}

// NewClaudeCLIProvider creates a provider backed by the `claude` CLI.
// model is optional — if empty, uses the CLI default.
// claudePath resolves the `claude` binary, checking PATH and the well-known
// ~/.local/bin fallback used by the Claude Code installer.
func claudePath() (string, error) {
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}
	// Claude Code installs to ~/.local/bin which may not be in PATH when
	// running under launchd or other restricted environments.
	if home, err := os.UserHomeDir(); err == nil {
		fallback := filepath.Join(home, ".local", "bin", "claude")
		if _, err := os.Stat(fallback); err == nil {
			return fallback, nil
		}
	}
	return "", fmt.Errorf("claude CLI not found in PATH — install Claude Code first")
}

func NewClaudeCLIProvider(model string) (*ClaudeCLIProvider, error) {
	if _, err := claudePath(); err != nil {
		return nil, err
	}
	return &ClaudeCLIProvider{model: model}, nil
}

func (p *ClaudeCLIProvider) ProviderName() string { return "claudecli" }
func (p *ClaudeCLIProvider) ModelID() string {
	if p.model != "" {
		return p.model
	}
	return "claude-code-session"
}
func (p *ClaudeCLIProvider) SetSystemPrompt(s string) { p.systemPrompt = s }

// Complete sends prompt via `claude -p` and returns the response.
func (p *ClaudeCLIProvider) Complete(prompt string, maxTokens int) (string, error) {
	resp, _, err := p.CompleteWithUsage(prompt, maxTokens)
	return resp, err
}

// CompleteWithUsage runs `claude -p`, feeding the prompt via stdin.
// Usage is not available from the CLI; returns nil Usage.
func (p *ClaudeCLIProvider) CompleteWithUsage(prompt string, _ int) (string, *Usage, error) {
	// Build: claude -p --output-format text [--model MODEL] [--append-system-prompt SP]
	args := []string{"-p", "--output-format", "text"}

	if p.model != "" {
		args = append(args, "--model", p.model)
	}
	if p.systemPrompt != "" {
		args = append(args, "--append-system-prompt", p.systemPrompt)
	}

	bin, _ := claudePath()
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewBufferString(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Large bundles (proto files, multi-module configs) can take 3-5 minutes.
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			return "", nil, fmt.Errorf("claude CLI error: %w\nstderr: %s", err, stderr.String())
		}
	case <-time.After(5 * 60 * time.Second):
		cmd.Process.Kill()
		return "", nil, fmt.Errorf("claude CLI timeout after %v", time.Since(start))
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", nil, fmt.Errorf("claude CLI returned empty response")
	}
	return result, nil, nil
}
