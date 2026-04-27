package ops

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GitHubConfig holds configuration for GitHub probe queries.
type GitHubConfig struct {
	Repo  string // owner/repo (e.g. "Cobliteam/fusca")
	PR    int    // PR number (for scope=pr)
	Query string // search query (for scope=issues)
	Scope string // pr | issues | ci | releases | deployments
	Limit int    // max items to fetch (default 10)
}

// CheckGitHub queries GitHub via the `gh` CLI (already authenticated) and
// evaluates results with zero-LLM heuristics.
func CheckGitHub(cfg GitHubConfig) OpsResult {
	if cfg.Repo == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "GitHub: missing repo (--repo owner/name)",
			Actions: []string{"set --input github-repo=owner/name"},
			Cost:    "zero-llm",
		}
	}

	if cfg.Limit <= 0 {
		cfg.Limit = 10
	}

	scope := cfg.Scope
	if scope == "" {
		scope = "pr"
	}

	switch scope {
	case "pr":
		return checkGitHubPR(cfg)
	case "issues":
		return checkGitHubIssues(cfg)
	case "ci":
		return checkGitHubCI(cfg)
	case "releases":
		return checkGitHubReleases(cfg)
	case "deployments":
		return checkGitHubDeployments(cfg)
	default:
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: unknown scope %q (valid: pr, issues, ci, releases, deployments)", scope),
			Cost:   "zero-llm",
		}
	}
}

// ── scope: pr ─────────────────────────────────────────────────────────────────

type ghPR struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	State          string    `json:"state"`
	URL            string    `json:"url"`
	Author         ghUser    `json:"author"`
	CreatedAt      time.Time `json:"createdAt"`
	MergedAt       *string   `json:"mergedAt"`
	IsDraft        bool      `json:"isDraft"`
	ReviewDecision string    `json:"reviewDecision"`
	StatusChecks   ghChecks  `json:"statusCheckRollup"`
	Comments       ghCount   `json:"comments"`
	Reviews        ghCount   `json:"reviews"`
	Labels         []ghLabel `json:"labels"`
}

type ghUser struct {
	Login string `json:"login"`
}

type ghChecks struct {
	Contexts []ghCheckContext `json:"contexts"`
}

type ghCheckContext struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

type ghCount struct {
	TotalCount int `json:"totalCount"`
}

type ghLabel struct {
	Name string `json:"name"`
}

func checkGitHubPR(cfg GitHubConfig) OpsResult {
	if cfg.PR <= 0 {
		return checkGitHubPRList(cfg)
	}
	return checkGitHubPRSingle(cfg)
}

func checkGitHubPRSingle(cfg GitHubConfig) OpsResult {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", cfg.PR),
		"--repo", cfg.Repo,
		"--json", "number,title,state,url,author,createdAt,mergedAt,isDraft,reviewDecision,statusCheckRollup,comments,reviews,labels",
	}

	out, err := shellOutput("gh", args...)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: gh pr view failed: %v", err),
			Cost:   "zero-llm",
		}
	}

	return evaluateGitHubPRSingle(out)
}

func evaluateGitHubPRSingle(data []byte) OpsResult {
	var pr ghPR
	if err := json.Unmarshal(data, &pr); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: failed to parse PR JSON: %v", err),
			Cost:   "zero-llm",
		}
	}

	// Count check statuses
	checksPassed, checksFailed, checksRunning := 0, 0, 0
	for _, ctx := range pr.StatusChecks.Contexts {
		switch {
		case ctx.Conclusion == "SUCCESS" || ctx.Conclusion == "success":
			checksPassed++
		case ctx.Conclusion == "FAILURE" || ctx.Conclusion == "failure" ||
			ctx.Conclusion == "ERROR" || ctx.Conclusion == "error":
			checksFailed++
		case ctx.Status == "IN_PROGRESS" || ctx.Status == "in_progress" ||
			ctx.Status == "QUEUED" || ctx.Status == "queued":
			checksRunning++
		default:
			if ctx.State == "FAILURE" || ctx.State == "ERROR" {
				checksFailed++
			} else if ctx.State == "SUCCESS" {
				checksPassed++
			} else if ctx.State == "PENDING" {
				checksRunning++
			}
		}
	}

	resultData := map[string]any{
		"number":          pr.Number,
		"title":           pr.Title,
		"state":           pr.State,
		"url":             pr.URL,
		"author":          pr.Author.Login,
		"is_draft":        pr.IsDraft,
		"review_decision": pr.ReviewDecision,
		"checks_passed":   checksPassed,
		"checks_failed":   checksFailed,
		"checks_running":  checksRunning,
		"comments_count":  pr.Comments.TotalCount,
		"reviews_count":   pr.Reviews.TotalCount,
	}

	labels := make([]string, len(pr.Labels))
	for i, l := range pr.Labels {
		labels[i] = l.Name
	}
	resultData["labels"] = labels

	var actions []string

	// Heuristics
	status := "ok"
	var signals []string

	if pr.State == "MERGED" {
		signals = append(signals, fmt.Sprintf("PR #%d merged", pr.Number))
	} else if pr.State == "CLOSED" {
		signals = append(signals, fmt.Sprintf("PR #%d closed (not merged)", pr.Number))
	} else {
		// OPEN
		if checksFailed > 0 {
			status = "critical"
			signals = append(signals, fmt.Sprintf("CI failing: %d/%d checks failed", checksFailed, checksPassed+checksFailed+checksRunning))
			actions = append(actions, "fix CI failures before merging")
		}

		if pr.ReviewDecision == "CHANGES_REQUESTED" {
			if status != "critical" {
				status = "warn"
			}
			signals = append(signals, "changes requested by reviewer")
			actions = append(actions, "address review feedback")
		} else if pr.ReviewDecision == "" || pr.ReviewDecision == "REVIEW_REQUIRED" {
			if status == "ok" {
				status = "warn"
			}
			signals = append(signals, "review pending")
			actions = append(actions, "request review from team")
		} else if pr.ReviewDecision == "APPROVED" {
			signals = append(signals, "approved")
			if checksFailed == 0 && checksRunning == 0 {
				actions = append(actions, "ready to merge")
			}
		}

		if checksRunning > 0 {
			signals = append(signals, fmt.Sprintf("%d checks still running", checksRunning))
		}

		// Age check — PRs open >7 days without review
		age := time.Since(pr.CreatedAt)
		if age > 7*24*time.Hour && pr.Reviews.TotalCount == 0 {
			if status == "ok" {
				status = "warn"
			}
			signals = append(signals, fmt.Sprintf("open %d days without review", int(age.Hours()/24)))
			actions = append(actions, "escalate — PR aging without review")
		}

		signals = append([]string{fmt.Sprintf("PR #%d open", pr.Number)}, signals...)
	}

	return OpsResult{
		Status:  status,
		Signal:  fmt.Sprintf("GitHub: %s — %s", pr.Title, strings.Join(signals, "; ")),
		Data:    resultData,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

func checkGitHubPRList(cfg GitHubConfig) OpsResult {
	args := []string{
		"pr", "list",
		"--repo", cfg.Repo,
		"--state", "open",
		"--limit", fmt.Sprintf("%d", cfg.Limit),
		"--json", "number,title,state,url,author,createdAt,isDraft,reviewDecision,statusCheckRollup,comments,reviews,labels",
	}

	out, err := shellOutput("gh", args...)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: gh pr list failed: %v", err),
			Cost:   "zero-llm",
		}
	}

	return evaluateGitHubPRList(out)
}

func evaluateGitHubPRList(data []byte) OpsResult {
	var prs []ghPR
	if err := json.Unmarshal(data, &prs); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: failed to parse PR list JSON: %v", err),
			Cost:   "zero-llm",
		}
	}

	totalOpen := len(prs)
	ciFailing, needsReview, stale := 0, 0, 0
	var actions []string

	for _, pr := range prs {
		for _, ctx := range pr.StatusChecks.Contexts {
			if ctx.Conclusion == "FAILURE" || ctx.Conclusion == "failure" ||
				ctx.Conclusion == "ERROR" || ctx.Conclusion == "error" ||
				ctx.State == "FAILURE" || ctx.State == "ERROR" {
				ciFailing++
				break
			}
		}

		if pr.ReviewDecision == "" || pr.ReviewDecision == "REVIEW_REQUIRED" || pr.ReviewDecision == "CHANGES_REQUESTED" {
			needsReview++
		}

		if time.Since(pr.CreatedAt) > 7*24*time.Hour {
			stale++
		}
	}

	resultData := map[string]any{
		"open_prs":     totalOpen,
		"ci_failing":   ciFailing,
		"needs_review": needsReview,
		"stale_7d":     stale,
	}

	status := "ok"
	var signals []string
	signals = append(signals, fmt.Sprintf("%d open PRs", totalOpen))

	if ciFailing > 0 {
		status = "critical"
		signals = append(signals, fmt.Sprintf("%d with CI failures", ciFailing))
		actions = append(actions, "fix CI failures")
	}

	if needsReview > 0 {
		if status == "ok" {
			status = "warn"
		}
		signals = append(signals, fmt.Sprintf("%d need review", needsReview))
	}

	if stale > 0 {
		signals = append(signals, fmt.Sprintf("%d stale (>7d)", stale))
		actions = append(actions, "review stale PRs for merge or close")
	}

	return OpsResult{
		Status:  status,
		Signal:  fmt.Sprintf("GitHub: %s", strings.Join(signals, ", ")),
		Data:    resultData,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// ── scope: issues ─────────────────────────────────────────────────────────────

type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	URL       string    `json:"url"`
	Author    ghUser    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	Labels    []ghLabel `json:"labels"`
}

func checkGitHubIssues(cfg GitHubConfig) OpsResult {
	args := []string{
		"issue", "list",
		"--repo", cfg.Repo,
		"--state", "open",
		"--limit", fmt.Sprintf("%d", cfg.Limit),
		"--json", "number,title,state,url,author,createdAt,labels",
	}

	if cfg.Query != "" {
		args = append(args, "--search", cfg.Query)
	}

	out, err := shellOutput("gh", args...)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: gh issue list failed: %v", err),
			Cost:   "zero-llm",
		}
	}

	return evaluateGitHubIssues(out)
}

func evaluateGitHubIssues(data []byte) OpsResult {
	var issues []ghIssue
	if err := json.Unmarshal(data, &issues); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: failed to parse issues JSON: %v", err),
			Cost:   "zero-llm",
		}
	}

	totalOpen := len(issues)
	bugCount, criticalCount := 0, 0
	var actions []string

	priorityLabels := map[string]bool{
		"critical": true, "p0": true, "urgent": true, "severity/critical": true,
		"priority/critical": true, "priority: critical": true,
	}
	bugLabels := map[string]bool{
		"bug": true, "type/bug": true, "type: bug": true, "defect": true,
	}

	for _, issue := range issues {
		isBug, isCritical := false, false
		for _, label := range issue.Labels {
			name := strings.ToLower(label.Name)
			if priorityLabels[name] {
				isCritical = true
			}
			if bugLabels[name] {
				isBug = true
			}
		}
		if isCritical {
			criticalCount++
		}
		if isBug {
			bugCount++
		}
	}

	resultData := map[string]any{
		"open_issues":    totalOpen,
		"critical_count": criticalCount,
		"bug_count":      bugCount,
	}

	status := "ok"
	var signals []string
	signals = append(signals, fmt.Sprintf("%d open issues", totalOpen))

	if criticalCount > 0 {
		status = "critical"
		signals = append(signals, fmt.Sprintf("%d critical", criticalCount))
		actions = append(actions, "triage critical issues immediately")
	}

	if bugCount > 0 {
		if status == "ok" {
			status = "warn"
		}
		signals = append(signals, fmt.Sprintf("%d bugs", bugCount))
	}

	return OpsResult{
		Status:  status,
		Signal:  fmt.Sprintf("GitHub: %s", strings.Join(signals, ", ")),
		Data:    resultData,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// ── scope: ci ─────────────────────────────────────────────────────────────────

type ghRun struct {
	DatabaseID int       `json:"databaseId"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	URL        string    `json:"url"`
	CreatedAt  time.Time `json:"createdAt"`
	HeadBranch string    `json:"headBranch"`
	Event      string    `json:"event"`
}

func checkGitHubCI(cfg GitHubConfig) OpsResult {
	args := []string{
		"run", "list",
		"--repo", cfg.Repo,
		"--limit", fmt.Sprintf("%d", cfg.Limit),
		"--json", "databaseId,name,status,conclusion,url,createdAt,headBranch,event",
	}

	out, err := shellOutput("gh", args...)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: gh run list failed: %v", err),
			Cost:   "zero-llm",
		}
	}

	return evaluateGitHubCI(out)
}

func evaluateGitHubCI(data []byte) OpsResult {
	var runs []ghRun
	if err := json.Unmarshal(data, &runs); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: failed to parse CI runs JSON: %v", err),
			Cost:   "zero-llm",
		}
	}

	if len(runs) == 0 {
		return OpsResult{
			Status: "ok",
			Signal: "GitHub: no recent CI runs",
			Data:   map[string]any{"total_runs": 0},
			Cost:   "zero-llm",
		}
	}

	totalRuns := len(runs)
	succeeded, failed, running := 0, 0, 0
	defaultBranchFailing := false
	var actions []string

	for _, run := range runs {
		switch run.Conclusion {
		case "success":
			succeeded++
		case "failure", "timed_out", "cancelled":
			failed++
			// Check if default branch (main/master) is failing
			if run.HeadBranch == "main" || run.HeadBranch == "master" {
				defaultBranchFailing = true
			}
		default:
			if run.Status == "in_progress" || run.Status == "queued" {
				running++
			}
		}
	}

	resultData := map[string]any{
		"total_runs":             totalRuns,
		"succeeded":              succeeded,
		"failed":                 failed,
		"running":                running,
		"default_branch_failing": defaultBranchFailing,
	}

	status := "ok"
	var signals []string

	if defaultBranchFailing {
		status = "critical"
		signals = append(signals, "CI failing on default branch")
		actions = append(actions, "fix default branch CI immediately — deploys may be blocked")
	}

	if failed > 0 {
		if status == "ok" {
			status = "warn"
		}
		signals = append(signals, fmt.Sprintf("%d/%d runs failed", failed, totalRuns))
	}

	if running > 0 {
		signals = append(signals, fmt.Sprintf("%d running", running))
	}

	if succeeded > 0 {
		signals = append(signals, fmt.Sprintf("%d succeeded", succeeded))
	}

	if len(signals) == 0 {
		signals = append(signals, fmt.Sprintf("%d recent runs", totalRuns))
	}

	return OpsResult{
		Status:  status,
		Signal:  fmt.Sprintf("GitHub CI: %s", strings.Join(signals, ", ")),
		Data:    resultData,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// ── scope: releases ───────────────────────────────────────────────────────────

type ghRelease struct {
	TagName     string    `json:"tagName"`
	Name        string    `json:"name"`
	IsDraft     bool      `json:"isDraft"`
	IsPrerelease bool     `json:"isPrerelease"`
	CreatedAt   time.Time `json:"createdAt"`
	PublishedAt time.Time `json:"publishedAt"`
	URL         string    `json:"url"`
	Author      ghUser    `json:"author"`
}

func checkGitHubReleases(cfg GitHubConfig) OpsResult {
	args := []string{
		"release", "list",
		"--repo", cfg.Repo,
		"--limit", fmt.Sprintf("%d", cfg.Limit),
	}

	out, err := shellOutput("gh", args...)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: gh release list failed: %v", err),
			Cost:   "zero-llm",
		}
	}

	return evaluateGitHubReleases(out)
}

// evaluateGitHubReleases parses the tab-separated output from `gh release list`.
// Format: title\ttype\ttag\tpublished
func evaluateGitHubReleases(data []byte) OpsResult {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return OpsResult{
			Status: "ok",
			Signal: "GitHub: no releases found",
			Data:   map[string]any{"total_releases": 0},
			Cost:   "zero-llm",
		}
	}

	type release struct {
		Title     string
		Type      string
		Tag       string
		Published string
	}

	var releases []release
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		r := release{}
		if len(parts) >= 1 {
			r.Title = parts[0]
		}
		if len(parts) >= 2 {
			r.Type = parts[1]
		}
		if len(parts) >= 3 {
			r.Tag = parts[2]
		}
		if len(parts) >= 4 {
			r.Published = parts[3]
		}
		releases = append(releases, r)
	}

	resultData := map[string]any{
		"total_releases": len(releases),
	}

	if len(releases) > 0 {
		latest := releases[0]
		resultData["latest_tag"] = latest.Tag
		resultData["latest_title"] = latest.Title
		resultData["latest_published"] = latest.Published
		resultData["latest_type"] = latest.Type
	}

	draftCount := 0
	for _, r := range releases {
		if strings.Contains(strings.ToLower(r.Type), "draft") {
			draftCount++
		}
	}
	resultData["draft_count"] = draftCount

	status := "ok"
	var signals []string
	signals = append(signals, fmt.Sprintf("%d releases", len(releases)))

	if len(releases) > 0 {
		signals = append(signals, fmt.Sprintf("latest: %s", releases[0].Tag))
		if releases[0].Published != "" {
			signals = append(signals, releases[0].Published)
		}
	}

	var actions []string
	if draftCount > 0 {
		status = "warn"
		signals = append(signals, fmt.Sprintf("%d drafts pending", draftCount))
		actions = append(actions, "publish or discard draft releases")
	}

	return OpsResult{
		Status:  status,
		Signal:  fmt.Sprintf("GitHub: %s", strings.Join(signals, ", ")),
		Data:    resultData,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// ── scope: deployments ────────────────────────────────────────────────────────

type ghDeployment struct {
	Environment string     `json:"environment"`
	State       string     `json:"state"`
	Ref         ghRef      `json:"ref"`
	CreatedAt   string     `json:"createdAt"`
	Creator     ghUser     `json:"creator"`
	Statuses    []ghDeplSt `json:"statuses"`
}

type ghRef struct {
	Name string `json:"name"`
}

type ghDeplSt struct {
	State string `json:"state"`
}

func checkGitHubDeployments(cfg GitHubConfig) OpsResult {
	// gh api repos/{owner}/{repo}/deployments?per_page=N
	args := []string{
		"api",
		fmt.Sprintf("repos/%s/deployments?per_page=%d", cfg.Repo, cfg.Limit),
	}

	out, err := shellOutput("gh", args...)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: gh api deployments failed: %v", err),
			Cost:   "zero-llm",
		}
	}

	return evaluateGitHubDeployments(out)
}

type ghAPIDeployment struct {
	ID          int    `json:"id"`
	Environment string `json:"environment"`
	Ref         string `json:"ref"`
	SHA         string `json:"sha"`
	Task        string `json:"task"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	Creator     ghUser `json:"creator"`
}

func evaluateGitHubDeployments(data []byte) OpsResult {
	var deployments []ghAPIDeployment
	if err := json.Unmarshal(data, &deployments); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("GitHub: failed to parse deployments JSON: %v", err),
			Cost:   "zero-llm",
		}
	}

	if len(deployments) == 0 {
		return OpsResult{
			Status: "ok",
			Signal: "GitHub: no deployments found",
			Data:   map[string]any{"total_deployments": 0},
			Cost:   "zero-llm",
		}
	}

	envs := map[string]int{}
	for _, d := range deployments {
		envs[d.Environment]++
	}

	latest := deployments[0]

	resultData := map[string]any{
		"total_deployments": len(deployments),
		"environments":      envs,
		"latest_env":        latest.Environment,
		"latest_ref":        latest.Ref,
		"latest_sha":        latest.SHA[:minInt(7, len(latest.SHA))],
		"latest_created":    latest.CreatedAt,
		"latest_creator":    latest.Creator.Login,
	}

	signals := []string{
		fmt.Sprintf("%d deployments across %d envs", len(deployments), len(envs)),
		fmt.Sprintf("latest: %s@%s", latest.Environment, latest.Ref),
	}

	return OpsResult{
		Status: "ok",
		Signal: fmt.Sprintf("GitHub: %s", strings.Join(signals, ", ")),
		Data:   resultData,
		Cost:   "zero-llm",
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
