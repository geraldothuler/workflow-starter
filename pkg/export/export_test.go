package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestExportBacklogJSON(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "backlog.json")

	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Title: "Epic 1", Stories: []types.Story{{Title: "Story 1"}}},
		},
	}

	err := ExportBacklogJSON(backlog, outPath)
	if err != nil {
		t.Fatalf("ExportBacklogJSON failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	var loaded types.Backlog
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(loaded.Epics) != 1 {
		t.Errorf("expected 1 epic, got %d", len(loaded.Epics))
	}
}

func TestExportBacklogJSON_EmptyBacklog(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "empty.json")

	err := ExportBacklogJSON(&types.Backlog{}, outPath)
	if err != nil {
		t.Fatalf("ExportBacklogJSON failed for empty backlog: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if !json.Valid(data) {
		t.Error("output is not valid JSON")
	}
}

func TestExportBacklogMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "backlog.md")

	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				Title:       "Auth System",
				Code:        "E-001",
				Description: "Authentication and authorization",
				Stories: []types.Story{
					{
						Title:              "Login Flow",
						What:               "Implement user login",
						Why:                "Users need to access the system",
						AcceptanceCriteria: []string{"User can login with email", "Session is created"},
						Effort:             5,
					},
				},
			},
		},
		Meta: types.Metadata{
			Provider:     "claude",
			TotalEpics:   1,
			TotalStories: 1,
		},
	}

	err := ExportBacklogMarkdown(backlog, outPath)
	if err != nil {
		t.Fatalf("ExportBacklogMarkdown failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	content := string(data)

	// Verify markdown structure
	checks := []string{
		"# Backlog",
		"## Auth System [E-001]",
		"### Login Flow",
		"**What:** Implement user login",
		"**Why:** Users need to access the system",
		"**Acceptance Criteria:**",
		"- User can login with email",
		"**Effort:** 5 points",
		"**Provider:** claude",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected markdown to contain %q", check)
		}
	}
}

func TestFormatBacklogMarkdown_Empty(t *testing.T) {
	md := FormatBacklogMarkdown(&types.Backlog{})
	if !strings.Contains(md, "# Backlog") {
		t.Error("expected header even for empty backlog")
	}
}

func TestFormatBacklogMarkdown_WithDeepDives(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Title: "Epic 1"},
		},
		DeepDives: []types.DeepDive{
			{
				Term:   "Kafka",
				WhatIs: "Distributed event streaming platform",
				WhyHere: "High throughput event processing",
				Patterns: []string{"Event Sourcing", "CQRS"},
			},
		},
	}

	md := FormatBacklogMarkdown(backlog)

	checks := []string{
		"## Deep Dives",
		"### Deep Dive: Kafka",
		"Distributed event streaming platform",
		"High throughput event processing",
		"- Event Sourcing",
		"- CQRS",
	}
	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("expected markdown to contain %q", check)
		}
	}
}

func TestFormatBacklogMarkdown_WithTasks(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				Title: "Setup",
				Stories: []types.Story{
					{
						Title: "Setup CI",
						Tasks: []types.Task{
							{Description: "Configure pipeline", Effort: "2h"},
							{Description: "Add tests"},
						},
					},
				},
			},
		},
	}

	md := FormatBacklogMarkdown(backlog)

	if !strings.Contains(md, "- [ ] Configure pipeline (2h)") {
		t.Error("expected task with effort")
	}
	if !strings.Contains(md, "- [ ] Add tests") {
		t.Error("expected task without effort")
	}
}

func TestExportStatic(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a source backlog file
	srcPath := filepath.Join(tmpDir, "source.json")
	os.WriteFile(srcPath, []byte(`{"title":"test"}`), 0644)

	outDir := filepath.Join(tmpDir, "output")
	err := ExportStatic(srcPath, outDir)
	if err != nil {
		t.Fatalf("ExportStatic failed: %v", err)
	}

	outFile := filepath.Join(outDir, "backlog.json")
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(data) != `{"title":"test"}` {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestExportStatic_NonExistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	err := ExportStatic("/nonexistent/file.json", filepath.Join(tmpDir, "out"))
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}
