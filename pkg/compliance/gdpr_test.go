package compliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGDPRExporter(t *testing.T) {
	ge := NewGDPRExporter("/tmp/test")
	if ge == nil {
		t.Fatal("expected non-nil exporter")
	}
	if ge.configDir != "/tmp/test" {
		t.Errorf("expected configDir '/tmp/test', got %q", ge.configDir)
	}
}

func TestExportMyData_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	ge := NewGDPRExporter(tmpDir)

	outputFile := filepath.Join(tmpDir, "export.json")
	err := ge.ExportMyData(outputFile)
	if err != nil {
		t.Fatalf("ExportMyData failed: %v", err)
	}

	// Verify output file exists and is valid JSON
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	var export GDPRExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(export.Files) != 0 {
		t.Errorf("expected 0 files for empty dir, got %d", len(export.Files))
	}
	if export.DataController != "Workflow Platform User" {
		t.Errorf("expected 'Workflow Platform User', got %q", export.DataController)
	}
}

func TestExportMyData_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	ge := NewGDPRExporter(tmpDir)

	// Create consent file
	consentPath := filepath.Join(tmpDir, "consent.json")
	os.WriteFile(consentPath, []byte(`{"consent_given": true}`), 0644)

	// Create backlog file
	backlogPath := filepath.Join(tmpDir, "backlog.json")
	os.WriteFile(backlogPath, []byte(`{"epics": []}`), 0644)

	// Create deep dives file
	ddPath := filepath.Join(tmpDir, "deep-dives.json")
	os.WriteFile(ddPath, []byte(`[]`), 0644)

	outputFile := filepath.Join(tmpDir, "export.json")
	err := ge.ExportMyData(outputFile)
	if err != nil {
		t.Fatalf("ExportMyData failed: %v", err)
	}

	data, _ := os.ReadFile(outputFile)
	var export GDPRExport
	json.Unmarshal(data, &export)

	if len(export.DataCategories) < 2 {
		t.Errorf("expected at least 2 categories, got %d", len(export.DataCategories))
	}
	if len(export.Files) < 2 {
		t.Errorf("expected at least 2 files, got %d", len(export.Files))
	}
	if export.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestForgetMe_NoConfirm(t *testing.T) {
	tmpDir := t.TempDir()
	ge := NewGDPRExporter(tmpDir)

	_, err := ge.ForgetMe(false)
	if err == nil {
		t.Error("expected error without confirmation")
	}
}

func TestForgetMe_Confirm(t *testing.T) {
	tmpDir := t.TempDir()
	ge := NewGDPRExporter(tmpDir)

	// Create files to delete
	consentPath := filepath.Join(tmpDir, "consent.json")
	os.WriteFile(consentPath, []byte(`{}`), 0644)

	backlogPath := filepath.Join(tmpDir, "backlog.json")
	os.WriteFile(backlogPath, []byte(`{}`), 0644)

	result, err := ge.ForgetMe(true)
	if err != nil {
		t.Fatalf("ForgetMe failed: %v", err)
	}

	if result.FilesDeleted < 2 {
		t.Errorf("expected at least 2 files deleted, got %d", result.FilesDeleted)
	}

	// Files should be gone
	if _, err := os.Stat(consentPath); !os.IsNotExist(err) {
		t.Error("consent file should be deleted")
	}
}

func TestForgetMeResult_Report(t *testing.T) {
	result := &ForgetMeResult{
		FilesDeleted: 3,
		Errors:       []string{},
	}

	report := result.Report()
	if !strings.Contains(report, "RIGHT TO BE FORGOTTEN") {
		t.Error("report should mention right to be forgotten")
	}
	if !strings.Contains(report, "3") {
		t.Error("report should mention files deleted count")
	}
	if !strings.Contains(report, "sucesso") {
		t.Error("report should mention success when no errors")
	}
}

func TestForgetMeResult_Report_WithErrors(t *testing.T) {
	result := &ForgetMeResult{
		FilesDeleted: 1,
		Errors:       []string{"permission denied on file.json"},
	}

	report := result.Report()
	if !strings.Contains(report, "Erros") {
		t.Error("report should mention errors")
	}
	if !strings.Contains(report, "permission denied") {
		t.Error("report should contain error details")
	}
}

func TestPortabilityReport(t *testing.T) {
	tmpDir := t.TempDir()
	ge := NewGDPRExporter(tmpDir)

	outputFile := filepath.Join(tmpDir, "portability.json")
	err := ge.PortabilityReport(outputFile)
	if err != nil {
		t.Fatalf("PortabilityReport failed: %v", err)
	}

	data, _ := os.ReadFile(outputFile)
	var report PortabilityReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if report.Format != "JSON" {
		t.Errorf("expected format 'JSON', got %q", report.Format)
	}
	if len(report.StandardCompliance) < 2 {
		t.Errorf("expected at least 2 compliance standards, got %d", len(report.StandardCompliance))
	}
	if len(report.DataCategories) < 3 {
		t.Errorf("expected at least 3 data categories, got %d", len(report.DataCategories))
	}
}

func TestCheckConsent_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManager(tmpDir, false)

	// Write invalid JSON to consent file
	consentPath := filepath.Join(tmpDir, "consent.json")
	os.WriteFile(consentPath, []byte("not json"), 0644)

	_, err := cm.CheckConsent("claude")
	if err == nil {
		t.Error("expected error for corrupted consent file")
	}
}
