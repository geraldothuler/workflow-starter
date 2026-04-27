package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestNewServer(t *testing.T) {
	s := NewServer(8080, &types.Backlog{}, nil, nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.port != 8080 {
		t.Errorf("expected port 8080, got %d", s.port)
	}
}

func TestServeIndex(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	serveIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Workflow Platform Server") {
		t.Error("expected body to contain 'Workflow Platform Server'")
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/html" {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}
}
