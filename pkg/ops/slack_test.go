package ops

import (
	"testing"
)

func TestCheckSlack_MissingCredentials(t *testing.T) {
	r := CheckSlack(SlackConfig{})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckSlack_CriticalKeywords(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{"ok": true, "messages": [
		{"text": "CRITICAL: service is down", "ts": "1"},
		{"text": "normal message", "ts": "2"}
	]}`)

	r := CheckSlack(SlackConfig{Token: "tok", Channel: "C123"})
	if r.Status != "critical" {
		t.Errorf("expected critical, got %q", r.Status)
	}
}

func TestCheckSlack_NoAnomalies(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{"ok": true, "messages": [
		{"text": "all good", "ts": "1"},
		{"text": "deployment successful", "ts": "2"}
	]}`)

	r := CheckSlack(SlackConfig{Token: "tok", Channel: "C123"})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

func TestCheckSlack_API500(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(500, `error`)
	r := CheckSlack(SlackConfig{Token: "tok", Channel: "C123"})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}
