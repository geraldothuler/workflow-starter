package ops

import (
	"testing"
)

func TestCheckJira_MissingCredentials(t *testing.T) {
	r := CheckJira(JiraConfig{})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckJira_HappyPath_Critical(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{
		"total": 3,
		"issues": [
			{"key": "PROJ-1", "fields": {"summary": "Critical bug", "priority": {"name": "Highest"}, "status": {"name": "Open"}}},
			{"key": "PROJ-2", "fields": {"summary": "High bug", "priority": {"name": "High"}, "status": {"name": "In Progress"}}},
			{"key": "PROJ-3", "fields": {"summary": "Normal", "priority": {"name": "Medium"}, "status": {"name": "Open"}}}
		]
	}`)

	r := CheckJira(JiraConfig{URL: "https://test.atlassian.net", Email: "a@b.com", APIToken: "tok", Project: "PROJ"})
	if r.Status != "critical" {
		t.Errorf("expected critical, got %q", r.Status)
	}
}

func TestCheckJira_API500(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(500, `error`)
	r := CheckJira(JiraConfig{URL: "https://test.atlassian.net", Email: "a@b.com", APIToken: "tok", Project: "PROJ"})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestEvaluateJira_NoP0P1(t *testing.T) {
	r := evaluateJira([]byte(`{"total": 2, "issues": [
		{"key": "X-1", "fields": {"summary": "Normal", "priority": {"name": "Medium"}, "status": {"name": "Open"}}}
	]}`))
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}
