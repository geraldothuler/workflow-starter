package types

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBacklogStruct(t *testing.T) {
	backlog := Backlog{
		Epics: []Epic{
			{
				ID:          "E1",
				Code:        "EPIC-E1",
				Title:       "Backend API",
				Description: "APIs do sistema",
				Stories: []Story{
					{
						ID:                 "E1.1",
						EpicID:             "E1",
						Title:              "User CRUD",
						What:               "Criar API de usuarios",
						Why:                "Para gerenciar cadastros",
						AcceptanceCriteria: []string{"Deve funcionar"},
						Effort:             5,
					},
				},
			},
		},
		Meta: Metadata{
			GeneratedAt: "2026-02-20",
			Provider:    "claude",
		},
	}

	if len(backlog.Epics) != 1 {
		t.Errorf("expected 1 epic, got %d", len(backlog.Epics))
	}
	if backlog.Epics[0].ID != "E1" {
		t.Errorf("expected 'E1', got %q", backlog.Epics[0].ID)
	}
	if len(backlog.Epics[0].Stories) != 1 {
		t.Errorf("expected 1 story, got %d", len(backlog.Epics[0].Stories))
	}
	if backlog.Epics[0].Stories[0].Effort != 5 {
		t.Errorf("expected effort=5, got %d", backlog.Epics[0].Stories[0].Effort)
	}
}

func TestBacklog_JSON(t *testing.T) {
	backlog := Backlog{
		Epics: []Epic{
			{ID: "E1", Title: "Test Epic"},
		},
		Meta: Metadata{Provider: "claude"},
	}

	data, err := json.Marshal(backlog)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Backlog
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Epics[0].Title != "Test Epic" {
		t.Errorf("expected 'Test Epic', got %q", decoded.Epics[0].Title)
	}
}

func TestBacklog_SaveToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "output", "backlog.json")

	backlog := &Backlog{
		Epics: []Epic{{ID: "E1", Title: "Test"}},
	}

	err := backlog.SaveToFile(outPath)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	var loaded Backlog
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal saved file: %v", err)
	}
	if loaded.Epics[0].ID != "E1" {
		t.Errorf("expected 'E1', got %q", loaded.Epics[0].ID)
	}
}

func TestProjectInput(t *testing.T) {
	pi := ProjectInput{
		Context:   "Test context",
		Volumetry: "1000 users",
		Stack:     "Go",
		NFRs:      "99% uptime",
		Metadata:  map[string]string{"file": "test.md"},
	}

	if pi.Context != "Test context" {
		t.Error("context mismatch")
	}
	if pi.Metadata["file"] != "test.md" {
		t.Error("metadata mismatch")
	}
}

func TestSpecification(t *testing.T) {
	spec := Specification{
		Epics: []Epic{{ID: "E1"}},
		StackDecisions: map[string]interface{}{
			"database": "PostgreSQL",
		},
	}

	if len(spec.Epics) != 1 {
		t.Error("expected 1 epic")
	}
	if spec.StackDecisions["database"] != "PostgreSQL" {
		t.Error("stack decision mismatch")
	}
}

func TestDeepDive(t *testing.T) {
	dd := DeepDive{
		Term:    "PostgreSQL",
		WhatIs:  "Database relacional",
		WhyHere: "Para persistencia de dados",
		Patterns: []string{"CQRS"},
	}

	if dd.Term != "PostgreSQL" {
		t.Error("term mismatch")
	}
	if len(dd.Patterns) != 1 {
		t.Error("expected 1 pattern")
	}
}

func TestGenerationStats(t *testing.T) {
	stats := GenerationStats{
		TotalEpics:       3,
		TotalStories:     12,
		TotalCriteria:    36,
		TotalDeepDives:   5,
		TotalStoryPoints: 45,
	}

	if stats.TotalEpics != 3 {
		t.Error("TotalEpics mismatch")
	}
	if stats.TotalStoryPoints != 45 {
		t.Error("TotalStoryPoints mismatch")
	}
}

func TestGap(t *testing.T) {
	gap := Gap{
		Type:     "missing_info",
		Message:  "Missing volumetry",
		Severity: "high",
	}

	if gap.Type != "missing_info" {
		t.Error("type mismatch")
	}
}

func TestPattern(t *testing.T) {
	pattern := Pattern{
		ID:          "GP-001",
		Name:        "API Gateway",
		Description: "Use API Gateway for routing",
		When:        "Multiple microservices",
	}

	if pattern.ID != "GP-001" {
		t.Error("ID mismatch")
	}
}

func TestSpecification_SaveToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "spec.md")

	spec := &Specification{
		Epics: []Epic{{ID: "E1", Title: "Test"}},
	}

	err := spec.SaveToFile(outPath)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
}

func TestMetadata_ProjectTitle_JSON(t *testing.T) {
	meta := Metadata{
		GeneratedAt:  "2026-02-20",
		Provider:     "claude",
		ProjectTitle: "SPUI Alerts System",
		TotalEpics:   3,
		TotalStories: 12,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Metadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ProjectTitle != "SPUI Alerts System" {
		t.Errorf("expected 'SPUI Alerts System', got %q", decoded.ProjectTitle)
	}
}

func TestMetadata_ProjectTitle_OmitEmpty(t *testing.T) {
	meta := Metadata{Provider: "claude"}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// project_title should be omitted when empty
	jsonStr := string(data)
	if strings.Contains(jsonStr, "project_title") {
		t.Error("expected project_title to be omitted when empty")
	}
}

func TestGenerationMetrics_JSON(t *testing.T) {
	metrics := GenerationMetrics{
		TotalTechsExtracted:   15,
		TrivialFiltered:       5,
		ClassificationStats:   map[string]int{"trivial": 5, "standard": 6, "specific": 3, "critical": 1},
		CrossEpicGlobalDives:  2,
		CrossEpicDeduplicated: 3,
		LLMCallsMade:          8,
		LLMCallsSaved:         12,
		ReductionPercent:      60.0,
		TotalInputTokens:      5000,
		TotalOutputTokens:     2000,
		TotalCost:             0.05,
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded GenerationMetrics
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.TotalTechsExtracted != 15 {
		t.Errorf("expected TotalTechsExtracted=15, got %d", decoded.TotalTechsExtracted)
	}
	if decoded.TrivialFiltered != 5 {
		t.Errorf("expected TrivialFiltered=5, got %d", decoded.TrivialFiltered)
	}
	if decoded.ClassificationStats["critical"] != 1 {
		t.Errorf("expected classification_stats.critical=1, got %d", decoded.ClassificationStats["critical"])
	}
	if decoded.ReductionPercent != 60.0 {
		t.Errorf("expected ReductionPercent=60.0, got %f", decoded.ReductionPercent)
	}
	if decoded.TotalCost != 0.05 {
		t.Errorf("expected TotalCost=0.05, got %f", decoded.TotalCost)
	}
}

func TestMetadata_WithMetrics_JSON(t *testing.T) {
	meta := Metadata{
		Provider:     "claude",
		ProjectTitle: "Test Project",
		Metrics: &GenerationMetrics{
			TotalTechsExtracted: 10,
			TotalCost:           0.03,
		},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Metadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Metrics == nil {
		t.Fatal("expected Metrics to be non-nil")
	}
	if decoded.Metrics.TotalTechsExtracted != 10 {
		t.Errorf("expected TotalTechsExtracted=10, got %d", decoded.Metrics.TotalTechsExtracted)
	}
}

func TestMetadata_NilMetrics_OmitEmpty(t *testing.T) {
	meta := Metadata{Provider: "claude"}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, "metrics") {
		t.Error("expected metrics to be omitted when nil")
	}
}

func TestDeepDive_Classification_JSON(t *testing.T) {
	dd := DeepDive{
		Term:           "Kafka",
		WhatIs:         "Event streaming platform",
		Classification: "critical",
		Scope:          "global",
	}

	data, err := json.Marshal(dd)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded DeepDive
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Classification != "critical" {
		t.Errorf("expected Classification='critical', got %q", decoded.Classification)
	}
	if decoded.Scope != "global" {
		t.Errorf("expected Scope='global', got %q", decoded.Scope)
	}
}

func TestDeepDive_Classification_OmitEmpty(t *testing.T) {
	dd := DeepDive{Term: "PostgreSQL", WhatIs: "DB"}

	data, err := json.Marshal(dd)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, "classification") {
		t.Error("expected classification to be omitted when empty")
	}
	if strings.Contains(jsonStr, `"scope"`) {
		t.Error("expected scope to be omitted when empty")
	}
}

func TestGoldenPath(t *testing.T) {
	gp := GoldenPath{
		Patterns: map[string]Pattern{
			"GP-001": {ID: "GP-001", Name: "Test Pattern"},
		},
	}

	if len(gp.Patterns) != 1 {
		t.Error("expected 1 pattern")
	}
	if gp.Patterns["GP-001"].Name != "Test Pattern" {
		t.Error("pattern name mismatch")
	}
}
