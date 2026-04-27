// Package webhook implements HTTP handlers for incoming GitHub and Datadog webhooks.
// It verifies signatures, routes events to use-cases via webhook-routing.yml,
// extracts inputs from payloads, and dispatches async runs with chain traversal.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"

	"github.com/Cobliteam/workflow-toolkit/pkg/chain"
	"github.com/Cobliteam/workflow-toolkit/pkg/runner"
)

// ── Routing ───────────────────────────────────────────────────────────────────

// Rule maps a source+event pair to a use-case with input extraction paths.
type Rule struct {
	Source   string            `yaml:"source"`
	Event    string            `yaml:"event"`
	UseCase  string            `yaml:"use_case"`
	Inputs   map[string]string `yaml:"inputs"`
}

// WebhookRouting holds the loaded routing rules.
type WebhookRouting struct {
	Rules []Rule `yaml:"rules"`
}

// LoadRouting reads use-cases/webhook-routing.yml from workflowHome.
func LoadRouting(workflowHome string) (*WebhookRouting, error) {
	path := filepath.Join(workflowHome, "use-cases", "webhook-routing.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("webhook-routing.yml not found at %s: %w", path, err)
	}
	var wr WebhookRouting
	if err := yaml.Unmarshal(data, &wr); err != nil {
		return nil, fmt.Errorf("invalid webhook-routing.yml: %w", err)
	}
	return &wr, nil
}

// Match returns the first Rule matching source+eventType.
func (wr *WebhookRouting) Match(source, eventType string) (Rule, bool) {
	for _, r := range wr.Rules {
		if r.Source == source && r.Event == eventType {
			return r, true
		}
	}
	return Rule{}, false
}

// extractPath walks a nested map using dot-notation (e.g. "check_run.pull_requests.0.number").
// Returns "" if any segment is missing. "literal:<v>" returns <v> directly.
func extractPath(data map[string]any, path string) string {
	if strings.HasPrefix(path, "literal:") {
		return strings.TrimPrefix(path, "literal:")
	}
	parts := strings.Split(path, ".")
	var cur any = data
	for _, p := range parts {
		switch v := cur.(type) {
		case map[string]any:
			cur = v[p]
		case []any:
			i, err := strconv.Atoi(p)
			if err != nil || i < 0 || i >= len(v) {
				return ""
			}
			cur = v[i]
		default:
			return ""
		}
		if cur == nil {
			return ""
		}
	}
	return fmt.Sprintf("%v", cur)
}

// ── Dedup ─────────────────────────────────────────────────────────────────────

type dedupCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
}

func newDedupCache(ttl time.Duration) *dedupCache {
	return &dedupCache{entries: make(map[string]time.Time), ttl: ttl}
}

// add returns true if the ID is new (not a duplicate). Cleans up expired entries.
func (d *dedupCache) add(id string) bool {
	if id == "" {
		return true // no ID → always process
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if t, seen := d.entries[id]; seen && now.Sub(t) < d.ttl {
		return false
	}

	// Lazy cleanup when cache exceeds 1000 entries
	if len(d.entries) >= 1000 {
		for k, t := range d.entries {
			if now.Sub(t) >= d.ttl {
				delete(d.entries, k)
			}
		}
	}

	d.entries[id] = now
	return true
}

// ── Headless terminal ─────────────────────────────────────────────────────────

// headlessTerminal implements runner.TerminalIO for async webhook-triggered runs.
// It never blocks on stdin and auto-confirms human checkpoints with a log warning.
type headlessTerminal struct {
	logger *log.Logger
}

func (t *headlessTerminal) Confirm(prompt string) bool {
	t.logger.Printf("[webhook] auto-confirm: %s", strings.TrimSpace(prompt))
	return true
}

func (t *headlessTerminal) Printf(format string, args ...any) {
	t.logger.Printf("[webhook] "+format, args...)
}

func (t *headlessTerminal) Ask(prompt string, _ string) string {
	t.logger.Printf("[webhook] ask skipped (headless): %s", strings.TrimSpace(prompt))
	return ""
}

func (t *headlessTerminal) CanAsk() bool { return false }

// ── Keychain ──────────────────────────────────────────────────────────────────

func keychainSecret(service string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", "geraldothuler", "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain %q not found: %w", service, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ── Handler ───────────────────────────────────────────────────────────────────

// Handler dispatches webhook events to workflow use-cases.
type Handler struct {
	routing      *WebhookRouting
	workflowHome string
	repoRoot     string
	dedup        *dedupCache
	logger       *log.Logger
}

// New creates a Handler. Secrets are looked up from Keychain at dispatch time.
func New(routing *WebhookRouting, workflowHome, repoRoot string) *Handler {
	return &Handler{
		routing:      routing,
		workflowHome: workflowHome,
		repoRoot:     repoRoot,
		dedup:        newDedupCache(5 * time.Minute),
		logger:       log.New(os.Stderr, "[webhook] ", log.LstdFlags),
	}
}

// RegisterRoutes wires /webhooks/github and /webhooks/datadog onto mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/webhooks/github", h.handleGitHub)
	mux.HandleFunc("/webhooks/datadog", h.handleDatadog)
}

// ── GitHub ────────────────────────────────────────────────────────────────────

func (h *Handler) handleGitHub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	// Verify HMAC-SHA256 signature
	secret, err := keychainSecret("workflow-webhook-secret-github")
	if err != nil {
		h.logger.Printf("github: keychain lookup failed: %v", err)
		http.Error(w, "webhook secret not configured", http.StatusInternalServerError)
		return
	}
	if !verifyGitHubSignature(secret, body, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	eventType := githubEventType(r.Header.Get("X-GitHub-Event"), payload)
	eventID := r.Header.Get("X-GitHub-Delivery")

	h.dispatch("github", eventType, eventID, payload, w)
}

func verifyGitHubSignature(secret string, body []byte, sigHeader string) bool {
	if !strings.HasPrefix(sigHeader, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

// githubEventType derives a dot-notation event key from the GitHub event header
// and relevant payload fields: "<event>.<action>.<conclusion>".
func githubEventType(event string, payload map[string]any) string {
	parts := []string{event}
	if action, ok := payload["action"].(string); ok && action != "" {
		parts = append(parts, action)
	}
	// For check_run: append conclusion when action=completed
	if event == "check_run" {
		if cr, ok := payload["check_run"].(map[string]any); ok {
			if conclusion, ok := cr["conclusion"].(string); ok && conclusion != "" {
				parts = append(parts, conclusion)
			}
		}
	}
	return strings.Join(parts, ".")
}

// ── Datadog ───────────────────────────────────────────────────────────────────

func (h *Handler) handleDatadog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	// Verify shared secret token
	secret, err := keychainSecret("workflow-webhook-secret-datadog")
	if err != nil {
		h.logger.Printf("datadog: keychain lookup failed: %v", err)
		http.Error(w, "webhook secret not configured", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("X-Webhook-Secret") != secret {
		http.Error(w, "invalid secret", http.StatusUnauthorized)
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	eventType := datadogEventType(payload)
	eventID := extractPath(payload, "id")

	h.dispatch("datadog", eventType, eventID, payload, w)
}

// datadogEventType derives event key from alert_status field.
func datadogEventType(payload map[string]any) string {
	status, _ := payload["alert_status"].(string)
	switch strings.ToLower(status) {
	case "triggered":
		return "alert.triggered"
	case "recovered":
		return "alert.recovered"
	default:
		return "alert." + strings.ToLower(status)
	}
}

// ── Dispatch ──────────────────────────────────────────────────────────────────

func (h *Handler) dispatch(source, eventType, eventID string, payload map[string]any, w http.ResponseWriter) {
	// Dedup
	if !h.dedup.add(eventID) {
		h.logger.Printf("%s: duplicate event %q — skipping", source, eventID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "duplicate")
		return
	}

	// Route
	rule, ok := h.routing.Match(source, eventType)
	if !ok {
		h.logger.Printf("%s: no rule for event %q", source, eventType)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "no rule matched for %s/%s\n", source, eventType)
		return
	}

	// Extract inputs from payload
	inputs := make(runner.RunInputs, len(rule.Inputs))
	for key, path := range rule.Inputs {
		inputs[key] = extractPath(payload, path)
	}

	h.logger.Printf("%s: event %q → use-case %q inputs=%v", source, eventType, rule.UseCase, inputs)

	// Acknowledge immediately — run is async
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "accepted: %s → %s\n", eventType, rule.UseCase)

	go h.run(rule.UseCase, inputs)
}

func (h *Handler) run(useCaseID string, inputs runner.RunInputs) {
	def, err := runner.LoadDefinition(h.workflowHome, useCaseID)
	if err != nil {
		h.logger.Printf("run %q: load definition: %v", useCaseID, err)
		return
	}

	term := &headlessTerminal{logger: h.logger}
	opts := runner.RunOptions{
		AutoSkip:     true,
		WorkflowHome: h.workflowHome,
		RepoPath:     h.repoRoot,
	}

	if def.IsPipeline() {
		r := runner.New(def, runner.DefaultRegistry(), opts).WithTerminal(term)
		results, err := r.Run(inputs)
		if err != nil {
			h.logger.Printf("run %q: %v", useCaseID, err)
			return
		}
		executed, skipped := 0, 0
		for _, res := range results {
			if res.Skipped {
				skipped++
			} else {
				executed++
			}
		}
		h.logger.Printf("run %q: complete — %d executed, %d skipped", useCaseID, executed, skipped)
	}

	// Follow chain headlessly: auto-follow if single option, stop if multiple.
	if err := chain.FollowChain(def, inputs, opts, term, chain.NewHeadlessChainIO(h.logger)); err != nil {
		h.logger.Printf("chain %q: %v", useCaseID, err)
	}
}
