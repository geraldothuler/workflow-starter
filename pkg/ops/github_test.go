package ops

import (
	"testing"
)

func TestCheckGitHub_MissingRepo(t *testing.T) {
	r := CheckGitHub(GitHubConfig{})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckGitHub_InvalidScope(t *testing.T) {
	r := CheckGitHub(GitHubConfig{Repo: "owner/repo", Scope: "invalid"})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

// ── PR single ─────────────────────────────────────────────────────────────────

func TestEvaluateGitHubPRSingle_Approved_AllGreen(t *testing.T) {
	data := []byte(`{
		"number": 1078,
		"title": "fix: REQUIRES_NEW + DLQ",
		"state": "OPEN",
		"url": "https://github.com/Cobliteam/fusca/pull/1078",
		"author": {"login": "dev"},
		"createdAt": "2026-02-20T10:00:00Z",
		"isDraft": false,
		"reviewDecision": "APPROVED",
		"statusCheckRollup": {
			"contexts": [
				{"name": "build", "state": "SUCCESS", "conclusion": "SUCCESS"},
				{"name": "test", "state": "SUCCESS", "conclusion": "SUCCESS"}
			]
		},
		"comments": {"totalCount": 5},
		"reviews": {"totalCount": 2},
		"labels": [{"name": "hotfix"}]
	}`)

	r := evaluateGitHubPRSingle(data)
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Signal)
	}
	if r.Data["checks_passed"] != 2 {
		t.Errorf("expected 2 checks passed, got %v", r.Data["checks_passed"])
	}
}

func TestEvaluateGitHubPRSingle_CIFailing(t *testing.T) {
	data := []byte(`{
		"number": 42,
		"title": "feat: new feature",
		"state": "OPEN",
		"url": "https://github.com/org/repo/pull/42",
		"author": {"login": "dev"},
		"createdAt": "2026-02-20T10:00:00Z",
		"isDraft": false,
		"reviewDecision": "APPROVED",
		"statusCheckRollup": {
			"contexts": [
				{"name": "build", "state": "FAILURE", "conclusion": "FAILURE"},
				{"name": "test", "state": "SUCCESS", "conclusion": "SUCCESS"}
			]
		},
		"comments": {"totalCount": 0},
		"reviews": {"totalCount": 1},
		"labels": []
	}`)

	r := evaluateGitHubPRSingle(data)
	if r.Status != "critical" {
		t.Errorf("expected critical (CI failing), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["checks_failed"] != 1 {
		t.Errorf("expected 1 check failed, got %v", r.Data["checks_failed"])
	}
}

func TestEvaluateGitHubPRSingle_ChangesRequested(t *testing.T) {
	data := []byte(`{
		"number": 10,
		"title": "feat: something",
		"state": "OPEN",
		"url": "https://github.com/org/repo/pull/10",
		"author": {"login": "dev"},
		"createdAt": "2026-02-20T10:00:00Z",
		"isDraft": false,
		"reviewDecision": "CHANGES_REQUESTED",
		"statusCheckRollup": {"contexts": []},
		"comments": {"totalCount": 3},
		"reviews": {"totalCount": 1},
		"labels": []
	}`)

	r := evaluateGitHubPRSingle(data)
	if r.Status != "warn" {
		t.Errorf("expected warn (changes requested), got %q: %s", r.Status, r.Signal)
	}
}

func TestEvaluateGitHubPRSingle_Merged(t *testing.T) {
	data := []byte(`{
		"number": 99,
		"title": "chore: cleanup",
		"state": "MERGED",
		"url": "https://github.com/org/repo/pull/99",
		"author": {"login": "dev"},
		"createdAt": "2026-02-10T10:00:00Z",
		"mergedAt": "2026-02-15T10:00:00Z",
		"isDraft": false,
		"reviewDecision": "APPROVED",
		"statusCheckRollup": {"contexts": []},
		"comments": {"totalCount": 0},
		"reviews": {"totalCount": 1},
		"labels": []
	}`)

	r := evaluateGitHubPRSingle(data)
	if r.Status != "ok" {
		t.Errorf("expected ok (merged), got %q: %s", r.Status, r.Signal)
	}
}

// ── PR list ───────────────────────────────────────────────────────────────────

func TestEvaluateGitHubPRList_MixedState(t *testing.T) {
	data := []byte(`[
		{
			"number": 1, "title": "feat A", "state": "OPEN",
			"url": "u", "author": {"login": "a"}, "createdAt": "2026-02-20T10:00:00Z",
			"isDraft": false, "reviewDecision": "REVIEW_REQUIRED",
			"statusCheckRollup": {
				"contexts": [{"name": "ci", "state": "FAILURE", "conclusion": "FAILURE"}]
			},
			"comments": {"totalCount": 0}, "reviews": {"totalCount": 0}, "labels": []
		},
		{
			"number": 2, "title": "feat B", "state": "OPEN",
			"url": "u", "author": {"login": "b"}, "createdAt": "2026-02-01T10:00:00Z",
			"isDraft": false, "reviewDecision": "APPROVED",
			"statusCheckRollup": {
				"contexts": [{"name": "ci", "state": "SUCCESS", "conclusion": "SUCCESS"}]
			},
			"comments": {"totalCount": 0}, "reviews": {"totalCount": 1}, "labels": []
		}
	]`)

	r := evaluateGitHubPRList(data)
	if r.Status != "critical" {
		t.Errorf("expected critical (CI failing), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["open_prs"] != 2 {
		t.Errorf("expected 2 open PRs, got %v", r.Data["open_prs"])
	}
	if r.Data["ci_failing"] != 1 {
		t.Errorf("expected 1 CI failing, got %v", r.Data["ci_failing"])
	}
}

func TestEvaluateGitHubPRList_Empty(t *testing.T) {
	r := evaluateGitHubPRList([]byte(`[]`))
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

// ── issues ────────────────────────────────────────────────────────────────────

func TestEvaluateGitHubIssues_Critical(t *testing.T) {
	data := []byte(`[
		{"number": 1, "title": "Prod down", "state": "OPEN", "url": "u",
		 "author": {"login": "a"}, "createdAt": "2026-02-20T10:00:00Z",
		 "labels": [{"name": "bug"}, {"name": "critical"}]},
		{"number": 2, "title": "Minor fix", "state": "OPEN", "url": "u",
		 "author": {"login": "b"}, "createdAt": "2026-02-20T10:00:00Z",
		 "labels": [{"name": "enhancement"}]}
	]`)

	r := evaluateGitHubIssues(data)
	if r.Status != "critical" {
		t.Errorf("expected critical, got %q: %s", r.Status, r.Signal)
	}
	if r.Data["critical_count"] != 1 {
		t.Errorf("expected 1 critical, got %v", r.Data["critical_count"])
	}
	if r.Data["bug_count"] != 1 {
		t.Errorf("expected 1 bug, got %v", r.Data["bug_count"])
	}
}

func TestEvaluateGitHubIssues_NoCritical(t *testing.T) {
	data := []byte(`[
		{"number": 1, "title": "Docs update", "state": "OPEN", "url": "u",
		 "author": {"login": "a"}, "createdAt": "2026-02-20T10:00:00Z",
		 "labels": [{"name": "docs"}]}
	]`)

	r := evaluateGitHubIssues(data)
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Signal)
	}
}

func TestEvaluateGitHubIssues_Empty(t *testing.T) {
	r := evaluateGitHubIssues([]byte(`[]`))
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

// ── ci ────────────────────────────────────────────────────────────────────────

func TestEvaluateGitHubCI_DefaultBranchFailing(t *testing.T) {
	data := []byte(`[
		{"databaseId": 1, "name": "CI", "status": "completed", "conclusion": "failure",
		 "url": "u", "createdAt": "2026-02-20T10:00:00Z", "headBranch": "main", "event": "push"},
		{"databaseId": 2, "name": "CI", "status": "completed", "conclusion": "success",
		 "url": "u", "createdAt": "2026-02-19T10:00:00Z", "headBranch": "main", "event": "push"}
	]`)

	r := evaluateGitHubCI(data)
	if r.Status != "critical" {
		t.Errorf("expected critical (default branch failing), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["default_branch_failing"] != true {
		t.Errorf("expected default_branch_failing=true")
	}
}

func TestEvaluateGitHubCI_AllGreen(t *testing.T) {
	data := []byte(`[
		{"databaseId": 1, "name": "CI", "status": "completed", "conclusion": "success",
		 "url": "u", "createdAt": "2026-02-20T10:00:00Z", "headBranch": "main", "event": "push"},
		{"databaseId": 2, "name": "CI", "status": "completed", "conclusion": "success",
		 "url": "u", "createdAt": "2026-02-19T10:00:00Z", "headBranch": "feat-x", "event": "pull_request"}
	]`)

	r := evaluateGitHubCI(data)
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Signal)
	}
}

func TestEvaluateGitHubCI_Empty(t *testing.T) {
	r := evaluateGitHubCI([]byte(`[]`))
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

func TestEvaluateGitHubCI_Running(t *testing.T) {
	data := []byte(`[
		{"databaseId": 1, "name": "CI", "status": "in_progress", "conclusion": "",
		 "url": "u", "createdAt": "2026-02-20T10:00:00Z", "headBranch": "feat-y", "event": "push"}
	]`)

	r := evaluateGitHubCI(data)
	if r.Status != "ok" {
		t.Errorf("expected ok (just running, no failures), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["running"] != 1 {
		t.Errorf("expected 1 running, got %v", r.Data["running"])
	}
}

// ── releases ──────────────────────────────────────────────────────────────────

func TestEvaluateGitHubReleases_WithDrafts(t *testing.T) {
	data := []byte("v2.1.0\tLatest\tv2.1.0\t2026-02-20T10:00:00Z\nv2.0.0\tDraft\tv2.0.0\t2026-02-15T10:00:00Z")

	r := evaluateGitHubReleases(data)
	if r.Status != "warn" {
		t.Errorf("expected warn (draft), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["draft_count"] != 1 {
		t.Errorf("expected 1 draft, got %v", r.Data["draft_count"])
	}
	if r.Data["latest_tag"] != "v2.1.0" {
		t.Errorf("expected latest_tag v2.1.0, got %v", r.Data["latest_tag"])
	}
}

func TestEvaluateGitHubReleases_NoDrafts(t *testing.T) {
	data := []byte("v1.0.0\tLatest\tv1.0.0\t2026-02-20T10:00:00Z")

	r := evaluateGitHubReleases(data)
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Signal)
	}
}

func TestEvaluateGitHubReleases_Empty(t *testing.T) {
	r := evaluateGitHubReleases([]byte(""))
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

// ── deployments ───────────────────────────────────────────────────────────────

func TestEvaluateGitHubDeployments_Multiple(t *testing.T) {
	data := []byte(`[
		{"id": 1, "environment": "production", "ref": "main", "sha": "abc1234def", "task": "deploy",
		 "description": "", "created_at": "2026-02-20T10:00:00Z", "updated_at": "2026-02-20T10:05:00Z",
		 "creator": {"login": "deployer"}},
		{"id": 2, "environment": "staging", "ref": "feat-x", "sha": "def5678abc", "task": "deploy",
		 "description": "", "created_at": "2026-02-19T10:00:00Z", "updated_at": "2026-02-19T10:05:00Z",
		 "creator": {"login": "dev"}}
	]`)

	r := evaluateGitHubDeployments(data)
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Signal)
	}
	if r.Data["total_deployments"] != 2 {
		t.Errorf("expected 2 deployments, got %v", r.Data["total_deployments"])
	}
	if r.Data["latest_env"] != "production" {
		t.Errorf("expected latest_env=production, got %v", r.Data["latest_env"])
	}
	if r.Data["latest_sha"] != "abc1234" {
		t.Errorf("expected latest_sha=abc1234, got %v", r.Data["latest_sha"])
	}
}

func TestEvaluateGitHubDeployments_Empty(t *testing.T) {
	r := evaluateGitHubDeployments([]byte(`[]`))
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

// ── integration with CheckGitHub ──────────────────────────────────────────────

func TestCheckGitHub_PRScope_UsesGhCLI(t *testing.T) {
	origShell := shellOutput
	defer func() { shellOutput = origShell }()

	shellOutput = func(name string, args ...string) ([]byte, error) {
		if name != "gh" {
			t.Errorf("expected gh command, got %q", name)
		}
		return []byte(`[
			{"number": 1, "title": "test", "state": "OPEN",
			 "url": "u", "author": {"login": "a"}, "createdAt": "2026-02-20T10:00:00Z",
			 "isDraft": false, "reviewDecision": "APPROVED",
			 "statusCheckRollup": {"contexts": []},
			 "comments": {"totalCount": 0}, "reviews": {"totalCount": 1}, "labels": []}
		]`), nil
	}

	r := CheckGitHub(GitHubConfig{Repo: "owner/repo", Scope: "pr"})
	if r.Status == "error" {
		t.Errorf("unexpected error: %s", r.Signal)
	}
}

func TestCheckGitHub_CIScope_UsesGhCLI(t *testing.T) {
	origShell := shellOutput
	defer func() { shellOutput = origShell }()

	shellOutput = func(name string, args ...string) ([]byte, error) {
		return []byte(`[
			{"databaseId": 1, "name": "CI", "status": "completed", "conclusion": "success",
			 "url": "u", "createdAt": "2026-02-20T10:00:00Z", "headBranch": "main", "event": "push"}
		]`), nil
	}

	r := CheckGitHub(GitHubConfig{Repo: "owner/repo", Scope: "ci"})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Signal)
	}
}

func TestCheckGitHub_DefaultScope_IsPR(t *testing.T) {
	origShell := shellOutput
	defer func() { shellOutput = origShell }()

	shellOutput = func(name string, args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}

	r := CheckGitHub(GitHubConfig{Repo: "owner/repo"})
	if r.Status == "error" {
		t.Errorf("unexpected error (default scope should be pr): %s", r.Signal)
	}
}
