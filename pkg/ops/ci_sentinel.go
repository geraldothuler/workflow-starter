package ops

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CISentinelConfig holds parameters for the CI sentinel probe.
type CISentinelConfig struct {
	Repo string // owner/repo (e.g. "Cobliteam/fusca")
	PR   int    // PR number to watch
}

// CheckCISentinel fetches the current CI check state for a PR.
// Returns ok (all passing), critical (any failed), warn (still running), error.
// Intended to be called in a polling loop by the CLI command.
func CheckCISentinel(cfg CISentinelConfig) OpsResult {
	if cfg.Repo == "" || cfg.PR <= 0 {
		return OpsResult{
			Status:  "error",
			Signal:  "ci-sentinel: missing --repo or --pr",
			Actions: []string{"wtb ops ci-sentinel --repo owner/name --pr N"},
			Cost:    "zero-llm",
		}
	}

	args := []string{
		"pr", "view", fmt.Sprintf("%d", cfg.PR),
		"--repo", cfg.Repo,
		"--json", "number,title,headRefOid,statusCheckRollup",
	}

	out, err := shellOutput("gh", args...)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("ci-sentinel: gh pr view failed: %v", err),
			Cost:   "zero-llm",
		}
	}

	return evaluateCISentinel(out)
}

func evaluateCISentinel(raw []byte) OpsResult {
	var pr struct {
		Number       int    `json:"number"`
		Title        string `json:"title"`
		SHA          string `json:"headRefOid"`
		StatusChecks struct {
			Contexts []ghCheckContext `json:"contexts"`
		} `json:"statusCheckRollup"`
	}

	if err := json.Unmarshal(raw, &pr); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("ci-sentinel: failed to parse PR JSON: %v", err),
			Cost:   "zero-llm",
		}
	}

	passed, failed, running, skipped := 0, 0, 0, 0
	for _, ctx := range pr.StatusChecks.Contexts {
		conclusion := strings.ToLower(ctx.Conclusion)
		status := strings.ToLower(ctx.Status)
		switch {
		case conclusion == "success":
			passed++
		case conclusion == "skipped" || conclusion == "neutral":
			skipped++
		case conclusion == "failure" || conclusion == "error" ||
			conclusion == "timed_out" || conclusion == "cancelled":
			failed++
		case status == "in_progress" || status == "queued" ||
			status == "requested" || status == "waiting":
			running++
		}
	}

	total := len(pr.StatusChecks.Contexts)
	sha7 := pr.SHA
	if len(sha7) > 7 {
		sha7 = sha7[:7]
	}

	data := map[string]any{
		"pr":      pr.Number,
		"sha":     sha7,
		"total":   total,
		"passed":  passed,
		"failed":  failed,
		"running": running,
		"skipped": skipped,
	}

	base := fmt.Sprintf("CI PR #%d [%s]: %d/%d passed", pr.Number, sha7, passed+skipped, total)

	hStatus, hSignal, hActions := EvalHeuristics(data, loadHeuristics("ci-sentinel"))
	var signal string
	if hStatus == "ok" {
		signal = base + " — all checks passed ✓"
	} else {
		signal = base + " | " + hSignal
	}

	return OpsResult{
		Status:  hStatus,
		Signal:  signal,
		Data:    data,
		Actions: hActions,
		Cost:    "zero-llm",
	}
}
