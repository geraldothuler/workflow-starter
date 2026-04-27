package ops

import "testing"

func TestCheckCISentinel_MissingConfig(t *testing.T) {
	r := CheckCISentinel(CISentinelConfig{})
	if r.Status != "error" {
		t.Errorf("expected error for missing config, got %q", r.Status)
	}
}

func TestCheckCISentinel_MissingPR(t *testing.T) {
	r := CheckCISentinel(CISentinelConfig{Repo: "Cobliteam/fusca"})
	if r.Status != "error" {
		t.Errorf("expected error when PR=0, got %q", r.Status)
	}
}

func TestEvaluateCISentinel_AllPassed(t *testing.T) {
	raw := []byte(`{
		"number": 123,
		"title": "feat: add feature",
		"headRefOid": "abc1234567890",
		"statusCheckRollup": {
			"contexts": [
				{"name": "test",    "state": "SUCCESS", "conclusion": "success",  "status": "COMPLETED"},
				{"name": "lint",    "state": "SUCCESS", "conclusion": "success",  "status": "COMPLETED"},
				{"name": "build",   "state": "SUCCESS", "conclusion": "skipped",  "status": "COMPLETED"}
			]
		}
	}`)
	r := evaluateCISentinel(raw)
	if r.Status != "ok" {
		t.Errorf("expected ok (all passed), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["failed"] != 0 {
		t.Errorf("expected failed=0, got %v", r.Data["failed"])
	}
}

func TestEvaluateCISentinel_SomeFailed(t *testing.T) {
	raw := []byte(`{
		"number": 124,
		"title": "fix: broken feature",
		"headRefOid": "def9876543210",
		"statusCheckRollup": {
			"contexts": [
				{"name": "test",  "state": "FAILURE", "conclusion": "failure",  "status": "COMPLETED"},
				{"name": "lint",  "state": "SUCCESS", "conclusion": "success",  "status": "COMPLETED"}
			]
		}
	}`)
	r := evaluateCISentinel(raw)
	if r.Status != "critical" {
		t.Errorf("expected critical (check failed), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["failed"] != 1 {
		t.Errorf("expected failed=1, got %v", r.Data["failed"])
	}
}

func TestEvaluateCISentinel_StillRunning(t *testing.T) {
	raw := []byte(`{
		"number": 125,
		"title": "refactor: cleanup",
		"headRefOid": "fff1111222333",
		"statusCheckRollup": {
			"contexts": [
				{"name": "test",  "state": "PENDING", "conclusion": "", "status": "in_progress"},
				{"name": "lint",  "state": "SUCCESS", "conclusion": "success", "status": "COMPLETED"}
			]
		}
	}`)
	r := evaluateCISentinel(raw)
	if r.Status != "warn" {
		t.Errorf("expected warn (still running), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["running"] != 1 {
		t.Errorf("expected running=1, got %v", r.Data["running"])
	}
}
