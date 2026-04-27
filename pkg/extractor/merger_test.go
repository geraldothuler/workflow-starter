package extractor

import (
	"strings"
	"testing"
)

func TestNewMeetingMerger(t *testing.T) {
	mm := NewMeetingMerger()
	if mm == nil {
		t.Fatal("expected non-nil merger")
	}
	if len(mm.extractions) != 0 {
		t.Error("expected empty extractions")
	}
}

func TestAddExtraction(t *testing.T) {
	mm := NewMeetingMerger()

	result := &ExtractionResult{
		ProjectDefinition: "test",
		Metadata:          ExtractionMetadata{Source: "meeting1.md"},
	}
	mm.AddExtraction(result, "transcript text")

	if len(mm.extractions) != 1 {
		t.Errorf("expected 1 extraction, got %d", len(mm.extractions))
	}
	if len(mm.transcripts) != 1 {
		t.Errorf("expected 1 transcript, got %d", len(mm.transcripts))
	}
}

func TestMerge_Empty(t *testing.T) {
	mm := NewMeetingMerger()

	_, err := mm.Merge()
	if err == nil {
		t.Error("expected error for empty merger")
	}
}

func TestMerge_SingleMeeting(t *testing.T) {
	mm := NewMeetingMerger()

	result := &ExtractionResult{
		ProjectDefinition: "Single meeting result",
		Metadata: ExtractionMetadata{
			Source: "meeting1.md",
		},
	}
	mm.AddExtraction(result, "transcript")

	merged, err := mm.Merge()
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}
	if merged.MeetingCount != 1 {
		t.Errorf("expected meeting count 1, got %d", merged.MeetingCount)
	}
	if merged.ProjectDefinition != "Single meeting result" {
		t.Error("expected original project definition for single meeting")
	}
	if len(merged.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(merged.Sources))
	}
}

func TestMerge_MultipleMeetings(t *testing.T) {
	mm := NewMeetingMerger()

	ext1 := &ExtractionResult{
		Metadata: ExtractionMetadata{
			Source:            "meeting1.md",
			OverallConfidence: 0.8,
			SpeakersDetected:  []string{"Alice"},
			Warnings:          []string{"Low volumetry confidence"},
		},
		Extractions: map[string]interface{}{
			"context":   "Sistema de gestão v1",
			"problem":   "Precisa organizar",
			"objectives": []string{"Criar sistema"},
			"nfrs":      []string{"99% uptime"},
		},
	}

	ext2 := &ExtractionResult{
		Metadata: ExtractionMetadata{
			Source:            "meeting2.md",
			OverallConfidence: 0.9,
			SpeakersDetected:  []string{"Bob"},
			Warnings:          []string{"Stack needs review"},
		},
		Extractions: map[string]interface{}{
			"context":   "Sistema de gestão v2 com novas funcionalidades",
			"problem":   "Precisa organizar tarefas de forma eficiente para a equipe toda",
			"objectives": []string{"Automatizar processos"},
			"nfrs":      []string{"P99 < 200ms"},
		},
	}

	mm.AddExtraction(ext1, "transcript1")
	mm.AddExtraction(ext2, "transcript2")

	merged, err := mm.Merge()
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if merged.MeetingCount != 2 {
		t.Errorf("expected meeting count 2, got %d", merged.MeetingCount)
	}
	if len(merged.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(merged.Sources))
	}
	if merged.ProjectDefinition == "" {
		t.Error("expected non-empty project definition")
	}
}

func TestMergeContext_UsesLastMeeting(t *testing.T) {
	mm := NewMeetingMerger()

	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"context": "First context",
		},
	}, "")
	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"context": "Updated context",
		},
	}, "")

	ctx := mm.mergeContext()
	if ctx != "Updated context" {
		t.Errorf("expected last context, got %q", ctx)
	}
}

func TestMergeProblem_UsesLongest(t *testing.T) {
	mm := NewMeetingMerger()

	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"problem": "Short problem",
		},
	}, "")
	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"problem": "A much longer and more detailed description of the problem that the team is facing",
		},
	}, "")

	problem := mm.mergeProblem()
	if !strings.Contains(problem, "much longer") {
		t.Error("expected longest problem description")
	}
}

func TestMergeObjectives_Union(t *testing.T) {
	mm := NewMeetingMerger()

	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"objectives": []string{"Build API", "Add auth"},
		},
	}, "")
	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"objectives": []string{"Build API", "Deploy to cloud"}, // "Build API" is duplicate
		},
	}, "")

	objectives := mm.mergeObjectives()
	if len(objectives) != 3 {
		t.Errorf("expected 3 unique objectives, got %d: %v", len(objectives), objectives)
	}
}

func TestMergeNFRs_Union(t *testing.T) {
	mm := NewMeetingMerger()

	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"nfrs": []string{"99% uptime"},
		},
	}, "")
	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"nfrs": []string{"99% uptime", "P99 < 200ms"}, // duplicate
		},
	}, "")

	nfrs := mm.mergeNFRs()
	if len(nfrs) != 2 {
		t.Errorf("expected 2 unique NFRs, got %d: %v", len(nfrs), nfrs)
	}
}

func TestMergeStack_AveragesConfidence(t *testing.T) {
	mm := NewMeetingMerger()

	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"stack": []TechMention{
				{Name: "Go", Confidence: 0.8, Source: "explicit"},
			},
		},
	}, "")
	mm.AddExtraction(&ExtractionResult{
		Extractions: map[string]interface{}{
			"stack": []TechMention{
				{Name: "Go", Confidence: 1.0, Source: "explicit"},
				{Name: "Redis", Confidence: 0.7, Source: "inferred"},
			},
		},
	}, "")

	stack := mm.mergeStack()
	if len(stack) != 2 {
		t.Errorf("expected 2 techs (Go averaged, Redis new), got %d", len(stack))
	}

	// Check Go confidence was averaged
	for _, tech := range stack {
		if strings.ToLower(tech.Name) == "go" {
			expected := 0.9 // (0.8 + 1.0) / 2
			if tech.Confidence != expected {
				t.Errorf("expected Go confidence %f, got %f", expected, tech.Confidence)
			}
		}
	}
}

func TestMergeSpeakers_Union(t *testing.T) {
	mm := NewMeetingMerger()

	mm.AddExtraction(&ExtractionResult{
		Metadata: ExtractionMetadata{
			SpeakersDetected: []string{"Alice", "Bob"},
		},
	}, "")
	mm.AddExtraction(&ExtractionResult{
		Metadata: ExtractionMetadata{
			SpeakersDetected: []string{"Bob", "Charlie"}, // Bob is duplicate
		},
	}, "")

	speakers := mm.mergeSpeakers()
	if len(speakers) != 3 {
		t.Errorf("expected 3 unique speakers, got %d: %v", len(speakers), speakers)
	}
}

func TestMergeWarnings(t *testing.T) {
	mm := NewMeetingMerger()

	mm.AddExtraction(&ExtractionResult{
		Metadata: ExtractionMetadata{
			Warnings: []string{"Warning 1"},
		},
	}, "")
	mm.AddExtraction(&ExtractionResult{
		Metadata: ExtractionMetadata{
			Warnings: []string{"Warning 2"},
		},
	}, "")

	warnings := mm.mergeWarnings()
	// Should have combined warning + individual warnings
	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings, got %d", len(warnings))
	}
	// First warning should mention number of meetings
	if !strings.Contains(warnings[0], "2") {
		t.Error("first warning should mention meeting count")
	}
}

func TestCalculateMergedConfidence(t *testing.T) {
	mm := NewMeetingMerger()

	mm.AddExtraction(&ExtractionResult{
		Metadata: ExtractionMetadata{OverallConfidence: 0.8},
	}, "")
	mm.AddExtraction(&ExtractionResult{
		Metadata: ExtractionMetadata{OverallConfidence: 0.6},
	}, "")

	confidence := mm.calculateMergedConfidence()
	expected := 0.7 // (0.8 + 0.6) / 2
	if confidence != expected {
		t.Errorf("expected confidence %f, got %f", expected, confidence)
	}
}

func TestCalculateMergedConfidence_Empty(t *testing.T) {
	mm := NewMeetingMerger()

	confidence := mm.calculateMergedConfidence()
	if confidence != 0.0 {
		t.Errorf("expected 0.0 for empty, got %f", confidence)
	}
}

func TestGenerateMergedMarkdown(t *testing.T) {
	mm := NewMeetingMerger()
	mm.AddExtraction(&ExtractionResult{}, "")
	mm.AddExtraction(&ExtractionResult{}, "")

	md := mm.generateMergedMarkdown(
		"Test context",
		"Test problem",
		[]string{"Obj1", "Obj2"},
		map[string]string{"users": "500k"},
		[]TechMention{{Name: "Go", Confidence: 0.9}},
		[]string{"99% uptime"},
		0.85,
		[]string{"Alice"},
		[]string{},
	)

	if !strings.Contains(md, "Projeto Consolidado") {
		t.Error("should contain title")
	}
	if !strings.Contains(md, "Test context") {
		t.Error("should contain context")
	}
	if !strings.Contains(md, "Test problem") {
		t.Error("should contain problem")
	}
	if !strings.Contains(md, "Obj1") {
		t.Error("should contain objectives")
	}
	if !strings.Contains(md, "500k") {
		t.Error("should contain volumetry")
	}
	if !strings.Contains(md, "Go") {
		t.Error("should contain stack")
	}
	if !strings.Contains(md, "99% uptime") {
		t.Error("should contain NFRs")
	}
	if !strings.Contains(md, "85%") {
		t.Error("should contain confidence percentage")
	}
}

func TestFormatConflictsReport_NoConflicts(t *testing.T) {
	report := FormatConflictsReport(nil)
	if !strings.Contains(report, "Nenhum conflito") {
		t.Error("should report no conflicts")
	}
}

func TestFormatConflictsReport_WithConflicts(t *testing.T) {
	conflicts := []Conflict{
		{
			Type:        "volumetry",
			Field:       "users",
			Values:      []string{"500k", "1M"},
			Resolution:  "Usando última reunião: 1M",
			NeedsReview: true,
		},
	}

	report := FormatConflictsReport(conflicts)
	if !strings.Contains(report, "CONFLITOS") {
		t.Error("should mention conflicts")
	}
	if !strings.Contains(report, "volumetry") {
		t.Error("should mention conflict type")
	}
	if !strings.Contains(report, "REVISÃO MANUAL") {
		t.Error("should mention needs review")
	}
}

func TestGetContext_StringValue(t *testing.T) {
	mm := NewMeetingMerger()

	ext := &ExtractionResult{
		Extractions: map[string]interface{}{
			"context": "Test context value",
		},
	}

	ctx := mm.getContext(ext)
	if ctx != "Test context value" {
		t.Errorf("expected 'Test context value', got %q", ctx)
	}
}

func TestGetContext_MissingValue(t *testing.T) {
	mm := NewMeetingMerger()

	ext := &ExtractionResult{
		Extractions: map[string]interface{}{},
	}

	ctx := mm.getContext(ext)
	if ctx != "" {
		t.Errorf("expected empty string, got %q", ctx)
	}
}
