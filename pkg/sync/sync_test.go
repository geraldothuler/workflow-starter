package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestSaveSession(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session.json")

	session := &types.Session{
		ID:        "test-123",
		InputFile: "input.md",
		Phase:     "extraction",
		Progress:  50,
	}

	err := SaveSession(session, path)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("session file not created")
	}
}

func TestLoadSession(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session.json")

	original := &types.Session{
		ID:        "test-456",
		InputFile: "project.md",
		Phase:     "backlog",
		Progress:  75,
	}

	if err := SaveSession(original, path); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded.ID != "test-456" {
		t.Errorf("expected ID 'test-456', got %q", loaded.ID)
	}
	if loaded.Phase != "backlog" {
		t.Errorf("expected phase 'backlog', got %q", loaded.Phase)
	}
	if loaded.Progress != 75 {
		t.Errorf("expected progress 75, got %d", loaded.Progress)
	}
}

func TestLoadSession_NonExistent(t *testing.T) {
	_, err := LoadSession("/nonexistent/session.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestNewSynchronizer(t *testing.T) {
	s, err := NewSynchronizer(&SyncConfig{Enabled: true, Path: "/tmp"}, "/tmp/out")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil synchronizer")
	}
}

func TestSyncStatus(t *testing.T) {
	s := &Synchronizer{}
	if err := s.SyncStatus(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSyncBacklog(t *testing.T) {
	s := &Synchronizer{}
	result, err := s.SyncBacklog(&types.Backlog{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestGetSyncState(t *testing.T) {
	s := &Synchronizer{}
	state, err := s.GetSyncState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
}
