package sources

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMockMCPSession_CallTool(t *testing.T) {
	session := NewMockMCPSession(map[string]json.RawMessage{
		"get_file":     json.RawMessage(`{"name": "Test File", "version": "1.0"}`),
		"get_metadata": json.RawMessage(`{"title": "My Page"}`),
	})

	ctx := context.Background()

	// Call get_file
	result, err := session.CallTool(ctx, "get_file", map[string]any{"file_key": "abc123"})
	if err != nil {
		t.Fatalf("CallTool(get_file) error: %v", err)
	}

	var fileData map[string]any
	if err := json.Unmarshal(result, &fileData); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if fileData["name"] != "Test File" {
		t.Errorf("name = %v, want 'Test File'", fileData["name"])
	}

	// Call get_metadata
	result, err = session.CallTool(ctx, "get_metadata", nil)
	if err != nil {
		t.Fatalf("CallTool(get_metadata) error: %v", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(result, &metadata); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if metadata["title"] != "My Page" {
		t.Errorf("title = %v, want 'My Page'", metadata["title"])
	}

	// Verify calls recorded
	if len(session.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(session.Calls))
	}
	if session.Calls[0].Name != "get_file" {
		t.Errorf("first call = %q, want 'get_file'", session.Calls[0].Name)
	}
	if session.Calls[0].Args["file_key"] != "abc123" {
		t.Errorf("first call arg file_key = %v, want 'abc123'", session.Calls[0].Args["file_key"])
	}
	if session.Calls[1].Name != "get_metadata" {
		t.Errorf("second call = %q, want 'get_metadata'", session.Calls[1].Name)
	}
}

func TestMockMCPSession_UnexpectedTool(t *testing.T) {
	session := NewMockMCPSession(map[string]json.RawMessage{
		"known_tool": json.RawMessage(`{}`),
	})

	ctx := context.Background()
	_, err := session.CallTool(ctx, "unknown_tool", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !containsStr(err.Error(), "unexpected tool call") {
		t.Errorf("error should mention unexpected tool, got: %v", err)
	}
}

func TestMockMCPSession_ToolError(t *testing.T) {
	session := NewMockMCPSession(map[string]json.RawMessage{})
	session.Errors["failing_tool"] = java_error("tool execution failed")

	ctx := context.Background()
	_, err := session.CallTool(ctx, "failing_tool", nil)
	if err == nil {
		t.Fatal("expected error for failing tool")
	}
	if !containsStr(err.Error(), "tool execution failed") {
		t.Errorf("error should mention failure, got: %v", err)
	}
}

func TestMockMCPSession_Close(t *testing.T) {
	session := NewMockMCPSession(map[string]json.RawMessage{})

	if session.IsClosed() {
		t.Error("session should not be closed initially")
	}

	if err := session.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if !session.IsClosed() {
		t.Error("session should be closed after Close()")
	}
}

func TestMockMCPFactory_Connect(t *testing.T) {
	session := NewMockMCPSession(map[string]json.RawMessage{
		"test_tool": json.RawMessage(`{"ok": true}`),
	})

	factory := &MockMCPFactory{Session: session}

	ctx := context.Background()
	got, err := factory.Connect(ctx, TransportSpec{Type: "mcp", Command: "npx"})
	if err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	result, err := got.CallTool(ctx, "test_tool", nil)
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(result, &data); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if data["ok"] != true {
		t.Errorf("ok = %v, want true", data["ok"])
	}
}

func TestMockMCPFactory_ConnectError(t *testing.T) {
	factory := &MockMCPFactory{
		Err: java_error("connection refused"),
	}

	ctx := context.Background()
	_, err := factory.Connect(ctx, TransportSpec{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsStr(err.Error(), "connection refused") {
		t.Errorf("error should mention connection refused, got: %v", err)
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set a test env var
	t.Setenv("TEST_TOKEN", "secret123")

	got := expandEnvVars("${TEST_TOKEN}")
	if got != "secret123" {
		t.Errorf("expandEnvVars(${TEST_TOKEN}) = %q, want 'secret123'", got)
	}

	got = expandEnvVars("Bearer ${TEST_TOKEN}")
	if got != "Bearer secret123" {
		t.Errorf("expandEnvVars with prefix = %q, want 'Bearer secret123'", got)
	}

	got = expandEnvVars("no vars here")
	if got != "no vars here" {
		t.Errorf("expandEnvVars(no vars) = %q, want 'no vars here'", got)
	}
}

func TestExpandTemplateArgs(t *testing.T) {
	vars := map[string]string{
		"file_key": "abc123",
		"node_id":  "node456",
	}

	args := map[string]any{
		"file_key": "{{.file_key}}",
		"node":     "{{.node_id}}",
		"depth":    2,       // non-string, should pass through
		"literal":  "hello", // no template syntax
	}

	got, err := expandTemplateArgs(args, vars)
	if err != nil {
		t.Fatalf("expandTemplateArgs error: %v", err)
	}

	if got["file_key"] != "abc123" {
		t.Errorf("file_key = %v, want 'abc123'", got["file_key"])
	}
	if got["node"] != "node456" {
		t.Errorf("node = %v, want 'node456'", got["node"])
	}
	if got["depth"] != 2 {
		t.Errorf("depth = %v, want 2", got["depth"])
	}
	if got["literal"] != "hello" {
		t.Errorf("literal = %v, want 'hello'", got["literal"])
	}
}

func TestExpandTemplateArgs_Empty(t *testing.T) {
	got, err := expandTemplateArgs(nil, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}

// Helper to create errors (avoids importing errors package)
func java_error(msg string) error {
	return &simpleError{msg}
}

type simpleError struct {
	msg string
}

func (e *simpleError) Error() string { return e.msg }
