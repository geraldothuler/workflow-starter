package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

func TestNewLogger(t *testing.T) {
	l := NewLogger("/tmp/test-audit", true)
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	if !l.enabled {
		t.Error("expected enabled=true")
	}
}

func TestLog_Disabled(t *testing.T) {
	l := NewLogger("/tmp/test-audit", false)
	err := l.Log(Event{EventType: EventExtract})
	if err != nil {
		t.Errorf("disabled logger should return nil, got: %v", err)
	}
}

func TestLog_BasicEvent(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	event := Event{
		EventType: EventExtract,
		InputFile: "input.md",
		Success:   true,
	}

	err := l.Log(event)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Verify file created
	files, _ := filepath.Glob(filepath.Join(tmpDir, "audit-*.jsonl"))
	if len(files) == 0 {
		t.Fatal("expected audit file created")
	}

	// Read and verify content
	data, _ := os.ReadFile(files[0])
	var logged Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &logged); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if logged.EventType != EventExtract {
		t.Errorf("expected event type 'extract', got %q", logged.EventType)
	}
	if logged.ID == "" {
		t.Error("expected auto-generated ID")
	}
}

func TestLog_MultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	l.Log(Event{EventType: EventExtract, Success: true})
	l.Log(Event{EventType: EventBacklogGenerate, Success: true})
	l.Log(Event{EventType: EventFileExport, Success: false})

	files, _ := filepath.Glob(filepath.Join(tmpDir, "audit-*.jsonl"))
	if len(files) == 0 {
		t.Fatal("expected audit file")
	}

	data, _ := os.ReadFile(files[0])
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 events, got %d lines", len(lines))
	}
}

func TestLogExtract(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	err := l.LogExtract("input.md", "output.md", true, "")
	if err != nil {
		t.Fatalf("LogExtract failed: %v", err)
	}
}

func TestLogBacklogGenerate(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	err := l.LogBacklogGenerate(llm.ProviderClaude, "spec.md", 1500, 0.05, true, "")
	if err != nil {
		t.Fatalf("LogBacklogGenerate failed: %v", err)
	}
}

func TestLogConsent(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	err := l.LogConsent(llm.ProviderClaude, true, true, true)
	if err != nil {
		t.Fatalf("LogConsent failed: %v", err)
	}
}

func TestLogPIIDetection(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	err := l.LogPIIDetection("input.md", 3, []string{"CPF", "Email"})
	if err != nil {
		t.Fatalf("LogPIIDetection failed: %v", err)
	}
}

func TestLogAPIKeySetup(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)
	if err := l.LogAPIKeySetup(llm.ProviderClaude, true); err != nil {
		t.Fatalf("LogAPIKeySetup failed: %v", err)
	}
}

func TestLogExport(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)
	if err := l.LogExport("out.json", "json", true); err != nil {
		t.Fatalf("LogExport failed: %v", err)
	}
}

func TestQuery_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	events, err := l.Query(QueryFilters{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestQuery_Disabled(t *testing.T) {
	l := NewLogger("/tmp", false)
	_, err := l.Query(QueryFilters{})
	if err == nil {
		t.Error("expected error for disabled logger")
	}
}

func TestQuery_WithEvents(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	l.Log(Event{EventType: EventExtract, Provider: "claude", Success: true})
	l.Log(Event{EventType: EventBacklogGenerate, Provider: "chatgpt", Success: false})

	events, err := l.Query(QueryFilters{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestQuery_WithFilters(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	l.Log(Event{EventType: EventExtract, Provider: "claude", Success: true})
	l.Log(Event{EventType: EventBacklogGenerate, Provider: "claude", Success: false})
	l.Log(Event{EventType: EventExtract, Provider: "chatgpt", Success: true})

	// Filter by event type
	et := EventExtract
	events, _ := l.Query(QueryFilters{EventType: &et})
	if len(events) != 2 {
		t.Errorf("expected 2 extract events, got %d", len(events))
	}

	// Filter by provider
	provider := "claude"
	events, _ = l.Query(QueryFilters{Provider: &provider})
	if len(events) != 2 {
		t.Errorf("expected 2 claude events, got %d", len(events))
	}

	// Filter by success
	success := true
	events, _ = l.Query(QueryFilters{Success: &success})
	if len(events) != 2 {
		t.Errorf("expected 2 successful events, got %d", len(events))
	}
}

func TestQuery_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	for i := 0; i < 5; i++ {
		l.Log(Event{EventType: EventExtract, Success: true})
	}

	events, _ := l.Query(QueryFilters{Limit: 2})
	if len(events) != 2 {
		t.Errorf("expected 2 events with limit, got %d", len(events))
	}
}

func TestExportToCSV(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	l.Log(Event{EventType: EventExtract, Provider: "claude", Success: true, Cost: 0.05})

	csvPath := filepath.Join(tmpDir, "export.csv")
	err := l.ExportToCSV(csvPath, QueryFilters{})
	if err != nil {
		t.Fatalf("ExportToCSV failed: %v", err)
	}

	data, _ := os.ReadFile(csvPath)
	content := string(data)
	if !strings.Contains(content, "timestamp,event_type") {
		t.Error("CSV should have header")
	}
	if !strings.Contains(content, "extract") {
		t.Error("CSV should contain event data")
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	l.Log(Event{EventType: EventExtract, Provider: "claude", Success: true, Cost: 0.01, TokensUsed: 100})
	l.Log(Event{EventType: EventExtract, Provider: "claude", Success: true, Cost: 0.02, TokensUsed: 200})
	l.Log(Event{EventType: EventBacklogGenerate, Provider: "chatgpt", Success: false})

	stats, err := l.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", stats.TotalEvents)
	}
	if stats.SuccessCount != 2 {
		t.Errorf("expected 2 successes, got %d", stats.SuccessCount)
	}
	if stats.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", stats.FailureCount)
	}
	if stats.TotalTokens != 300 {
		t.Errorf("expected 300 total tokens, got %d", stats.TotalTokens)
	}
}

func TestQuery_DateFilter(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewLogger(tmpDir, true)

	now := time.Now()
	l.Log(Event{EventType: EventExtract, Timestamp: now, Success: true})

	future := now.Add(24 * time.Hour)
	events, _ := l.Query(QueryFilters{EndDate: &future})
	if len(events) != 1 {
		t.Errorf("expected 1 event within date range, got %d", len(events))
	}

	past := now.Add(-24 * time.Hour)
	events, _ = l.Query(QueryFilters{StartDate: &future, EndDate: &past})
	if len(events) != 0 {
		t.Errorf("expected 0 events outside date range, got %d", len(events))
	}
}

func TestSplitLines(t *testing.T) {
	lines := splitLines("a\nb\nc")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestSplitLines_NoNewline(t *testing.T) {
	lines := splitLines("single")
	if len(lines) != 1 || lines[0] != "single" {
		t.Errorf("expected ['single'], got %v", lines)
	}
}

func TestEscapeCSV(t *testing.T) {
	if escapeCSV("") != "" {
		t.Error("empty should stay empty")
	}
	if escapeCSV("normal") != "normal" {
		t.Error("normal string should not be escaped")
	}
	result := escapeCSV("has,comma")
	if !strings.Contains(result, `"`) {
		t.Error("should wrap with quotes when comma present")
	}
	result2 := escapeCSV(`has"quote`)
	if !strings.Contains(result2, `""`) {
		t.Error("should double quotes")
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("hello,world", ",") {
		t.Error("should find comma")
	}
	if containsAny("hello", ",") {
		t.Error("should not find comma")
	}
}

func TestReplaceAll(t *testing.T) {
	result := replaceAll(`say "hello"`, `"`, `""`)
	if result != `say ""hello""` {
		t.Errorf("expected doubled quotes, got %q", result)
	}
}

func TestEventTypes(t *testing.T) {
	if EventExtract != "extract" {
		t.Error("wrong event type constant")
	}
	if EventBacklogGenerate != "backlog_generate" {
		t.Error("wrong event type constant")
	}
	if EventConsentGiven != "consent_given" {
		t.Error("wrong event type constant")
	}
}
