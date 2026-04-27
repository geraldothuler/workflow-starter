package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// --- Test helpers ---

func testBacklog() *types.Backlog {
	return &types.Backlog{
		Epics: []types.Epic{
			{
				ID:          "E-001",
				Code:        "AUTH",
				Title:       "Authentication System",
				Description: "Implement user authentication with OAuth2 and JWT.",
				Tags:        []string{"security", "backend"},
				Stories: []types.Story{
					{
						ID:                 "S-001",
						EpicID:             "E-001",
						Title:              "Login with email/password",
						What:               "Users can log in using email and password",
						Why:                "Basic authentication is essential for user access",
						AcceptanceCriteria: []string{"Valid credentials grant access", "Invalid credentials show error"},
						Tags:               []string{"auth"},
						Effort:             5,
					},
					{
						ID:     "S-002",
						EpicID: "E-001",
						Title:  "OAuth2 Google login",
						What:   "Users can log in via Google OAuth2",
						Why:    "Simplifies sign-up for users with Google accounts",
						Tags:   []string{"auth", "oauth"},
						Effort: 8,
					},
				},
			},
			{
				ID:          "E-002",
				Code:        "API",
				Title:       "REST API",
				Description: "Build RESTful API endpoints.",
				Tags:        []string{"backend"},
				Stories: []types.Story{
					{
						ID:     "S-003",
						EpicID: "E-002",
						Title:  "CRUD endpoints for users",
						What:   "Create, read, update, delete user endpoints",
						Why:    "Core API functionality",
						Tags:   []string{"api"},
						Effort: 3,
					},
				},
			},
		},
	}
}

// mockCredentialProvider returns a simple env-like provider with preset values.
type mockCredentialProvider struct {
	values map[string]string
}

func (m *mockCredentialProvider) Name() string { return "mock" }
func (m *mockCredentialProvider) Resolve(_ context.Context, name string) (*credentials.Credential, error) {
	if v, ok := m.values[name]; ok {
		return &credentials.Credential{Name: name, Value: v, Source: "mock"}, nil
	}
	return nil, credentials.ErrNotFound
}
func (m *mockCredentialProvider) Store(_ context.Context, _, _ string) error { return nil }
func (m *mockCredentialProvider) Available() bool                            { return true }

func newMockResolver(values map[string]string) *credentials.Resolver {
	return credentials.NewResolver(nil, &mockCredentialProvider{values: values})
}

// --- ConfigExporter Tests ---

func TestConfigExporter_Push_Jira(t *testing.T) {
	requestCount := 0
	var epicBody, storyBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Verify auth header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("request %d: expected Basic auth, got %q", requestCount, auth)
		}

		// Read body
		buf := make([]byte, 65536)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])

		w.Header().Set("Content-Type", "application/json")

		// Check if this is an epic or story creation
		if strings.Contains(body, `"name": "Epic"`) || strings.Contains(body, `"name":"Epic"`) {
			epicBody = body
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": "AUTH-1", "id": "10001"})
		} else {
			storyBody = body
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": fmt.Sprintf("AUTH-%d", requestCount), "id": fmt.Sprintf("1000%d", requestCount)})
		}
	}))
	defer server.Close()

	resolver := newMockResolver(map[string]string{
		"JIRA_BASE_URL":   server.URL,
		"JIRA_USER_EMAIL": "test@example.com",
		"JIRA_API_TOKEN":  "api-token-123",
	})

	spec := ExporterSpec{
		ID:   "jira",
		Name: "Jira",
		Auth: credentials.AuthContract{
			Credentials: []credentials.CredentialSpec{
				{Name: "JIRA_BASE_URL", Required: true},
				{Name: "JIRA_USER_EMAIL", Required: true},
				{Name: "JIRA_API_TOKEN", Required: true},
			},
		},
		Transport: ExportTransportSpec{
			Type:      "http",
			BaseURL:   "{{.JIRA_BASE_URL}}/rest/api/3",
			AuthType:  "basic",
			AuthValue: "{{base64 .JIRA_USER_EMAIL .JIRA_API_TOKEN}}",
			Headers:   map[string]string{"Content-Type": "application/json"},
		},
		Push: PushSpec{
			Epic: APICallSpec{
				Method: "POST",
				Path:   "/issue",
				Body: `{
  "fields": {
    "project": {"key": "{{.project_key}}"},
    "issuetype": {"name": "Epic"},
    "summary": "{{jsonEscape .epic.Title}}"
  }
}`,
				Extract: map[string]string{"epic_key": "key", "epic_id": "id"},
			},
			Story: APICallSpec{
				Method: "POST",
				Path:   "/issue",
				Body: `{
  "fields": {
    "project": {"key": "{{.project_key}}"},
    "issuetype": {"name": "Story"},
    "summary": "{{jsonEscape .story.Title}}",
    "parent": {"key": "{{.epic_key}}"}
  }
}`,
				Extract: map[string]string{"story_key": "key"},
			},
		},
	}

	ce, err := NewConfigExporter(spec, resolver)
	if err != nil {
		t.Fatal(err)
	}

	backlog := testBacklog()
	result, err := ce.Push(context.Background(), backlog, PushOptions{
		ProjectKey: "MYPROJ",
	})
	if err != nil {
		t.Fatalf("Push error: %v", err)
	}

	// Verify result
	if result.Target != "jira" {
		t.Errorf("target = %q, want jira", result.Target)
	}
	if result.EpicsPushed != 2 {
		t.Errorf("epics pushed = %d, want 2", result.EpicsPushed)
	}
	if result.StoriesPushed != 3 {
		t.Errorf("stories pushed = %d, want 3", result.StoriesPushed)
	}
	if result.DryRun {
		t.Error("should not be dry-run")
	}

	// Verify epic body contains correct fields
	if !strings.Contains(epicBody, `"key": "MYPROJ"`) && !strings.Contains(epicBody, `"key":"MYPROJ"`) {
		t.Error("epic body missing project key")
	}

	// Verify story body contains parent key
	if !strings.Contains(storyBody, `"key": "AUTH-1"`) && !strings.Contains(storyBody, `"key":"AUTH-1"`) {
		t.Error("story body missing parent epic key")
	}
}

func TestConfigExporter_Push_DryRun(t *testing.T) {
	resolver := newMockResolver(map[string]string{
		"JIRA_BASE_URL":   "https://example.atlassian.net",
		"JIRA_USER_EMAIL": "test@example.com",
		"JIRA_API_TOKEN":  "token",
	})

	spec := ExporterSpec{
		ID:   "jira",
		Name: "Jira",
		Auth: credentials.AuthContract{
			Credentials: []credentials.CredentialSpec{
				{Name: "JIRA_BASE_URL", Required: true},
				{Name: "JIRA_USER_EMAIL", Required: true},
				{Name: "JIRA_API_TOKEN", Required: true},
			},
		},
		Transport: ExportTransportSpec{
			Type:      "http",
			BaseURL:   "{{.JIRA_BASE_URL}}/rest/api/3",
			AuthType:  "basic",
			AuthValue: "{{base64 .JIRA_USER_EMAIL .JIRA_API_TOKEN}}",
		},
		Push: PushSpec{
			Epic:  APICallSpec{Method: "POST", Path: "/issue", Body: "{}"},
			Story: APICallSpec{Method: "POST", Path: "/issue", Body: "{}"},
		},
	}

	ce, _ := NewConfigExporter(spec, resolver)

	result, err := ce.Push(context.Background(), testBacklog(), PushOptions{
		ProjectKey: "TEST",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("DryRun error: %v", err)
	}

	if !result.DryRun {
		t.Error("expected dry-run result")
	}
	if result.EpicsPushed != 2 {
		t.Errorf("dry-run epics = %d, want 2", result.EpicsPushed)
	}
	if result.StoriesPushed != 3 {
		t.Errorf("dry-run stories = %d, want 3", result.StoriesPushed)
	}
	// Total items: 2 epics + 3 stories = 5
	if len(result.Items) != 5 {
		t.Errorf("dry-run items = %d, want 5", len(result.Items))
	}
}

func TestConfigExporter_Push_AuthMissing(t *testing.T) {
	// Resolver with missing credentials
	resolver := newMockResolver(map[string]string{
		"JIRA_BASE_URL": "https://example.atlassian.net",
		// Missing JIRA_USER_EMAIL and JIRA_API_TOKEN
	})

	spec := ExporterSpec{
		ID:   "jira",
		Name: "Jira",
		Auth: credentials.AuthContract{
			Credentials: []credentials.CredentialSpec{
				{Name: "JIRA_BASE_URL", Required: true},
				{Name: "JIRA_USER_EMAIL", Required: true},
				{Name: "JIRA_API_TOKEN", Required: true},
			},
		},
		Transport: ExportTransportSpec{
			Type:      "http",
			BaseURL:   "{{.JIRA_BASE_URL}}",
			AuthType:  "basic",
			AuthValue: "{{base64 .JIRA_USER_EMAIL .JIRA_API_TOKEN}}",
		},
		Push: PushSpec{
			Epic:  APICallSpec{Method: "POST", Path: "/", Body: "{}"},
			Story: APICallSpec{Method: "POST", Path: "/", Body: "{}"},
		},
	}

	ce, _ := NewConfigExporter(spec, resolver)
	_, err := ce.Push(context.Background(), testBacklog(), PushOptions{ProjectKey: "TEST"})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), "auth failed") {
		t.Errorf("error should mention auth failure, got: %v", err)
	}
}

func TestConfigExporter_Push_NoResolver(t *testing.T) {
	spec := ExporterSpec{
		ID:   "test",
		Name: "Test",
		Auth: credentials.AuthContract{
			Credentials: []credentials.CredentialSpec{{Name: "TOKEN", Required: true}},
		},
		Transport: ExportTransportSpec{
			Type:      "http",
			BaseURL:   "http://example.com",
			AuthType:  "bearer",
			AuthValue: "{{.TOKEN}}",
		},
		Push: PushSpec{
			Epic:  APICallSpec{Method: "POST", Path: "/", Body: "{}"},
			Story: APICallSpec{Method: "POST", Path: "/", Body: "{}"},
		},
	}

	ce, _ := NewConfigExporter(spec, nil) // nil resolver
	_, err := ce.Push(context.Background(), testBacklog(), PushOptions{ProjectKey: "TEST"})
	if err == nil {
		t.Fatal("expected error with nil resolver")
	}
}

func TestConfigExporter_Push_PartialFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// First epic succeeds, second fails
		if callCount == 1 {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1", "id": "1"})
		} else if callCount == 4 { // Second epic (after 2 stories for first epic)
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"Permission denied"}`))
		} else {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": fmt.Sprintf("PROJ-%d", callCount), "id": fmt.Sprintf("%d", callCount)})
		}
	}))
	defer server.Close()

	resolver := newMockResolver(map[string]string{"TOKEN": "tok"})

	spec := ExporterSpec{
		ID:   "test",
		Name: "Test",
		Auth: credentials.AuthContract{
			Credentials: []credentials.CredentialSpec{{Name: "TOKEN", Required: true}},
		},
		Transport: ExportTransportSpec{
			Type:      "http",
			BaseURL:   server.URL,
			AuthType:  "bearer",
			AuthValue: "{{.TOKEN}}",
		},
		Push: PushSpec{
			Epic: APICallSpec{
				Method:  "POST",
				Path:    "/epic",
				Body:    `{"title": "{{.epic.Title}}"}`,
				Extract: map[string]string{"epic_key": "key"},
			},
			Story: APICallSpec{
				Method: "POST",
				Path:   "/story",
				Body:   `{"title": "{{.story.Title}}", "parent": "{{.epic_key}}"}`,
			},
		},
	}

	ce, _ := NewConfigExporter(spec, resolver)
	result, err := ce.Push(context.Background(), testBacklog(), PushOptions{ProjectKey: "PROJ"})

	// Push should not return fatal error — partial failures are collected
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Error("expected partial errors")
	}

	// First epic + 2 stories should succeed
	if result.EpicsPushed < 1 {
		t.Errorf("expected at least 1 epic pushed, got %d", result.EpicsPushed)
	}
}

// --- Registry Tests ---

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	exporters := registry.List()
	if len(exporters) != 3 {
		t.Errorf("expected 3 exporters, got %d", len(exporters))
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	jira := registry.Get("jira")
	if jira == nil {
		t.Fatal("jira exporter not found")
	}
	if jira.Name() != "jira" {
		t.Errorf("name = %q, want jira", jira.Name())
	}

	linear := registry.Get("linear")
	if linear == nil {
		t.Fatal("linear exporter not found")
	}

	azdo := registry.Get("azure-devops")
	if azdo == nil {
		t.Fatal("azure-devops exporter not found")
	}

	unknown := registry.Get("unknown")
	if unknown != nil {
		t.Error("expected nil for unknown exporter")
	}
}

// --- DryRun Formatter Tests ---

func TestFormatDryRunResult(t *testing.T) {
	result := &PushResult{
		Target:        "jira",
		ProjectKey:    "TEST",
		EpicsPushed:   1,
		StoriesPushed: 2,
		DryRun:        true,
		Items: []PushedItem{
			{Type: "epic", LocalID: "E-001", Title: "Auth Epic"},
			{Type: "story", LocalID: "S-001", Title: "Login Story", ParentKey: "<parent of Auth Epic>"},
			{Type: "story", LocalID: "S-002", Title: "OAuth Story", ParentKey: "<parent of Auth Epic>"},
		},
	}

	output := FormatDryRunResult(result)

	expectations := []string{
		"DRY RUN",
		"Target:  jira",
		"Epics:   1",
		"Stories: 2",
		"EPIC: Auth Epic",
		"STORY: Login Story",
		"STORY: OAuth Story",
		"--dry-run",
	}

	for _, expected := range expectations {
		if !strings.Contains(output, expected) {
			t.Errorf("dry-run output missing %q", expected)
		}
	}
}

func TestFormatPushResult(t *testing.T) {
	result := &PushResult{
		Target:        "jira",
		ProjectKey:    "PROJ",
		EpicsPushed:   1,
		StoriesPushed: 1,
		Items: []PushedItem{
			{Type: "epic", LocalID: "E-001", Title: "Epic", RemoteKey: "PROJ-1"},
			{Type: "story", LocalID: "S-001", Title: "Story", RemoteKey: "PROJ-2"},
		},
	}

	output := FormatPushResult(result)

	expectations := []string{
		"EXPORT COMPLETE",
		"PROJ-1",
		"PROJ-2",
	}

	for _, expected := range expectations {
		if !strings.Contains(output, expected) {
			t.Errorf("push result output missing %q", expected)
		}
	}
}

func TestFormatPushResult_WithErrors(t *testing.T) {
	result := &PushResult{
		Target:        "jira",
		ProjectKey:    "PROJ",
		EpicsPushed:   1,
		StoriesPushed: 0,
		Errors:        []string{"story creation failed"},
		Items: []PushedItem{
			{Type: "epic", LocalID: "E-001", Title: "Epic", RemoteKey: "PROJ-1"},
			{Type: "story", LocalID: "S-001", Title: "Story", Error: "HTTP 403"},
		},
	}

	output := FormatPushResult(result)

	if !strings.Contains(output, "Errors") {
		t.Error("output missing errors section")
	}
	if !strings.Contains(output, "HTTP 403") {
		t.Error("output missing error detail")
	}
}
