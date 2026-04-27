package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultRetentionPolicy(t *testing.T) {
	policy := DefaultRetentionPolicy()
	if policy == nil {
		t.Fatal("expected non-nil policy")
	}
	if policy.RetentionDays != 30 {
		t.Errorf("expected 30 days, got %d", policy.RetentionDays)
	}
	if policy.AutoCleanup {
		t.Error("auto cleanup should be false by default")
	}
	if policy.NotifyBeforeDays != 7 {
		t.Errorf("expected 7 notify days, got %d", policy.NotifyBeforeDays)
	}
	if len(policy.Exceptions) != 0 {
		t.Error("expected empty exceptions")
	}
}

func TestNewRetentionManager(t *testing.T) {
	rm := NewRetentionManager("/tmp/test", nil)
	if rm == nil {
		t.Fatal("expected non-nil manager")
	}
	if rm.policy.RetentionDays != 30 {
		t.Error("nil policy should use defaults")
	}
}

func TestNewRetentionManager_CustomPolicy(t *testing.T) {
	policy := &RetentionPolicy{RetentionDays: 7, NotifyBeforeDays: 2}
	rm := NewRetentionManager("/tmp/test", policy)
	if rm.policy.RetentionDays != 7 {
		t.Errorf("expected 7 days, got %d", rm.policy.RetentionDays)
	}
}

func TestFindExpiredFiles(t *testing.T) {
	tmpDir := t.TempDir()
	policy := &RetentionPolicy{RetentionDays: 1, NotifyBeforeDays: 0}
	rm := NewRetentionManager(tmpDir, policy)

	// Create an old file matching the pattern
	oldFile := filepath.Join(tmpDir, "backlog-old.json")
	os.WriteFile(oldFile, []byte("{}"), 0644)

	// Set modification time to 10 days ago
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldFile, oldTime, oldTime)

	// Create a recent file
	newFile := filepath.Join(tmpDir, "backlog-new.json")
	os.WriteFile(newFile, []byte("{}"), 0644)

	expired, err := rm.FindExpiredFiles()
	if err != nil {
		t.Fatalf("FindExpiredFiles failed: %v", err)
	}

	if len(expired) != 1 {
		t.Errorf("expected 1 expired file, got %d", len(expired))
	}
	if len(expired) > 0 && expired[0].Path != oldFile {
		t.Errorf("expected %s, got %s", oldFile, expired[0].Path)
	}
}

func TestFindExpiredFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRetentionManager(tmpDir, nil)

	expired, err := rm.FindExpiredFiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expired) != 0 {
		t.Errorf("expected 0 expired files, got %d", len(expired))
	}
}

func TestFindExpiringFiles(t *testing.T) {
	tmpDir := t.TempDir()
	policy := &RetentionPolicy{RetentionDays: 30, NotifyBeforeDays: 7}
	rm := NewRetentionManager(tmpDir, policy)

	// File in the "expiring soon" window: 25 days old (between 23 and 30 days)
	expiringFile := filepath.Join(tmpDir, "backlog-expiring.json")
	os.WriteFile(expiringFile, []byte("{}"), 0644)
	expiringTime := time.Now().AddDate(0, 0, -25)
	os.Chtimes(expiringFile, expiringTime, expiringTime)

	expiring, err := rm.FindExpiringFiles()
	if err != nil {
		t.Fatalf("FindExpiringFiles failed: %v", err)
	}

	if len(expiring) != 1 {
		t.Errorf("expected 1 expiring file, got %d", len(expiring))
	}
}

func TestCleanup_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	policy := &RetentionPolicy{RetentionDays: 1, NotifyBeforeDays: 0}
	rm := NewRetentionManager(tmpDir, policy)

	// Create an old file
	oldFile := filepath.Join(tmpDir, "backlog-old.json")
	os.WriteFile(oldFile, []byte(`{"test": true}`), 0644)
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldFile, oldTime, oldTime)

	result, err := rm.Cleanup(true) // dryRun
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if result.FilesFound != 1 {
		t.Errorf("expected 1 file found, got %d", result.FilesFound)
	}
	if result.FilesDeleted != 0 {
		t.Errorf("expected 0 deleted in dry run, got %d", result.FilesDeleted)
	}

	// File should still exist
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		t.Error("file should still exist after dry run")
	}
}

func TestCleanup_Real(t *testing.T) {
	tmpDir := t.TempDir()
	policy := &RetentionPolicy{RetentionDays: 1, NotifyBeforeDays: 0}
	rm := NewRetentionManager(tmpDir, policy)

	oldFile := filepath.Join(tmpDir, "backlog-old.json")
	os.WriteFile(oldFile, []byte(`{"test": true}`), 0644)
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldFile, oldTime, oldTime)

	result, err := rm.Cleanup(false)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if result.FilesDeleted != 1 {
		t.Errorf("expected 1 deleted, got %d", result.FilesDeleted)
	}

	// File should be deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("file should be deleted after cleanup")
	}
}

func TestAddException(t *testing.T) {
	rm := NewRetentionManager("/tmp/test", nil)
	rm.AddException("*.important")

	if len(rm.policy.Exceptions) != 1 {
		t.Errorf("expected 1 exception, got %d", len(rm.policy.Exceptions))
	}
}

func TestIsException(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRetentionManager(tmpDir, nil)
	rm.AddException(filepath.Join(tmpDir, "backlog-keep.json"))

	if !rm.isException(filepath.Join(tmpDir, "backlog-keep.json")) {
		t.Error("should match exception")
	}
	if rm.isException(filepath.Join(tmpDir, "backlog-other.json")) {
		t.Error("should not match exception")
	}
}

func TestGetPolicy(t *testing.T) {
	policy := &RetentionPolicy{RetentionDays: 60}
	rm := NewRetentionManager("/tmp/test", policy)

	got := rm.GetPolicy()
	if got.RetentionDays != 60 {
		t.Errorf("expected 60 days, got %d", got.RetentionDays)
	}
}

func TestCleanupResult_Report_DryRun(t *testing.T) {
	cr := &CleanupResult{
		DryRun:     true,
		FilesFound: 3,
		BytesFreed: 1024,
	}

	report := cr.Report()
	if !strings.Contains(report, "SIMULAÇÃO") {
		t.Error("dry run report should mention simulation")
	}
	if !strings.Contains(report, "3") {
		t.Error("should mention file count")
	}
}

func TestCleanupResult_Report_Real(t *testing.T) {
	cr := &CleanupResult{
		DryRun:       false,
		FilesFound:   2,
		FilesDeleted: 2,
		BytesFreed:   2048,
	}

	report := cr.Report()
	if !strings.Contains(report, "CLEANUP EXECUTADO") {
		t.Error("real report should mention cleanup executed")
	}
}

func TestCleanupResult_Report_WithErrors(t *testing.T) {
	cr := &CleanupResult{
		DryRun:       false,
		FilesFound:   1,
		FilesDeleted: 0,
		Errors:       []string{"permission denied"},
	}

	report := cr.Report()
	if !strings.Contains(report, "Erros") {
		t.Error("should mention errors")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d): expected %q, got %q", tt.bytes, tt.expected, result)
		}
	}
}

func TestScheduleCleanup_Disabled(t *testing.T) {
	rm := NewRetentionManager("/tmp/test", nil)
	// Default policy has AutoCleanup=false → no-op, returns nil
	err := rm.ScheduleCleanup()
	if err != nil {
		t.Errorf("expected nil error when auto-cleanup disabled, got: %v", err)
	}
}

func TestScheduleCleanup_Enabled(t *testing.T) {
	tmpDir := t.TempDir()
	policy := &RetentionPolicy{
		RetentionDays:    0, // Everything is expired
		AutoCleanup:      true,
		NotifyBeforeDays: 0,
	}
	rm := NewRetentionManager(tmpDir, policy)

	err := rm.ScheduleCleanup()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
