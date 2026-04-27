package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestAutoDetectInput(t *testing.T) {
	input, err := AutoDetectInput()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.BacklogPath == "" {
		t.Error("expected non-empty BacklogPath")
	}
	if input.DeepDivesPath == "" {
		t.Error("expected non-empty DeepDivesPath")
	}
}

func TestAutoDetectInput_WithBasePath(t *testing.T) {
	input, err := AutoDetectInput("/custom/base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.BacklogPath != "/custom/base/backlog.json" {
		t.Errorf("expected '/custom/base/backlog.json', got %q", input.BacklogPath)
	}
}

func TestNewServer(t *testing.T) {
	data := &LensData{}
	server := NewServer(8080, data)

	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.port != 8080 {
		t.Errorf("expected port 8080, got %d", server.port)
	}
	if server.data != data {
		t.Error("expected data to be set")
	}
}

func TestNewExporter(t *testing.T) {
	data := &LensData{}
	exporter := NewExporter(data)

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}
	if exporter.data != data {
		t.Error("expected data to be set")
	}
}

func TestNewStaticExporter(t *testing.T) {
	data := &LensData{}
	exporter := NewStaticExporter(data)

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}
}

func TestReplaceFirst(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		old      string
		new      string
		expected string
	}{
		{"simple replace", "hello world", "world", "go", "hello go"},
		{"first only", "aaa", "a", "b", "baa"},
		{"no match", "hello", "xyz", "abc", "hello"},
		{"empty old", "hello", "", "", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceFirst(tt.s, tt.old, tt.new)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestLoadOrBuild_BacklogFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a backlog.json in old format (types.Backlog)
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Test Epic",
				Stories: []types.Story{
					{ID: "E1.1", Title: "Test Story"},
				},
			},
		},
	}

	backlogJSON, _ := json.MarshalIndent(backlog, "", "  ")
	backlogPath := filepath.Join(tmpDir, "backlog.json")
	os.WriteFile(backlogPath, backlogJSON, 0644)

	input := &Input{BacklogPath: backlogPath}
	data, err := LoadOrBuild(input)
	if err != nil {
		t.Fatalf("LoadOrBuild failed: %v", err)
	}

	if data == nil {
		t.Fatal("expected non-nil LensData")
	}
}

func TestConvertToLensData(t *testing.T) {
	backlog := &types.Backlog{
		Meta: types.Metadata{
			TotalEpics:   1,
			TotalStories: 2,
			Stats: types.GenerationStats{
				TotalStoryPoints: 8,
				TotalCriteria:    3,
			},
		},
		Epics: []types.Epic{
			{
				ID:          "E1",
				Code:        "EPIC-E1",
				Title:       "Backend API Development",
				Description: "Create the backend API for the system with all necessary endpoints and integrations",
				Priority:    "high",
				Stories: []types.Story{
					{ID: "E1.1", Title: "User CRUD", Effort: 3, What: "Create API", Why: "For users"},
					{ID: "E1.2", Title: "Auth System", Effort: 5, What: "Add auth", Why: "Security"},
				},
			},
		},
	}

	deepDives := []types.DeepDive{
		{Term: "Go", WhatIs: "Programming language", WhyHere: "Performance"},
	}

	data := ConvertToLensData(backlog, deepDives)
	if data == nil {
		t.Fatal("expected non-nil LensData")
	}
	if len(data.Epics) != 1 {
		t.Errorf("expected 1 epic, got %d", len(data.Epics))
	}
	if len(data.DeepDives) != 1 {
		t.Errorf("expected 1 deep dive, got %d", len(data.DeepDives))
	}
	if data.Effort.TotalSPs != 8 {
		t.Errorf("expected 8 total SPs, got %d", data.Effort.TotalSPs)
	}
}

func TestCalculateRisk(t *testing.T) {
	tests := []struct {
		effort   int
		expected string
	}{
		{1, "low"},
		{3, "low"},
		{5, "medium"},
		{7, "medium"},
		{8, "high"},
		{13, "high"},
	}

	for _, tt := range tests {
		story := &types.Story{Effort: tt.effort}
		result := calculateRisk(story)
		if result != tt.expected {
			t.Errorf("effort %d: expected %q, got %q", tt.effort, tt.expected, result)
		}
	}
}

func TestConvertDeepDives(t *testing.T) {
	deepDives := []types.DeepDive{
		{
			Term:           "Kafka",
			WhatIs:         "Event streaming platform",
			WhyHere:        "High throughput",
			StoryID:        "E1.1",
			SourcePatterns: []string{"GP-001"},
		},
		{
			Term:   "Redis",
			WhatIs: "In-memory store",
		},
	}

	result := convertDeepDives(deepDives)
	if len(result) != 2 {
		t.Fatalf("expected 2 deep dives, got %d", len(result))
	}

	// Term-based key: storyID:term for contextualized
	dd1, ok := result["E1.1:Kafka"]
	if !ok {
		t.Fatal("expected key 'E1.1:Kafka'")
	}
	if dd1.Term != "Kafka" {
		t.Errorf("expected 'Kafka', got %q", dd1.Term)
	}
	if dd1.StoryID != "E1.1" {
		t.Errorf("expected 'E1.1', got %q", dd1.StoryID)
	}

	// Term-based key: just term for global
	_, ok2 := result["Redis"]
	if !ok2 {
		t.Fatal("expected key 'Redis'")
	}
}

func TestConvertDeepDives_Empty(t *testing.T) {
	result := convertDeepDives(nil)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestCalculateEffort(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				ID: "E1",
				Stories: []types.Story{
					{Effort: 3},
					{Effort: 5},
				},
			},
			{
				ID: "E2",
				Stories: []types.Story{
					{Effort: 8},
				},
			},
		},
	}

	effort := calculateEffort(backlog)
	if effort.TotalStories != 3 {
		t.Errorf("expected 3 stories, got %d", effort.TotalStories)
	}
	if effort.TotalSPs != 16 {
		t.Errorf("expected 16 SPs, got %d", effort.TotalSPs)
	}
	if len(effort.ByEpic) != 2 {
		t.Errorf("expected 2 epic efforts, got %d", len(effort.ByEpic))
	}

	e1 := effort.ByEpic["E1"]
	if e1.SPs != 8 {
		t.Errorf("E1: expected 8 SPs, got %d", e1.SPs)
	}
}

func TestCalculateEffort_Empty(t *testing.T) {
	backlog := &types.Backlog{}
	effort := calculateEffort(backlog)
	if effort.TotalSPs != 0 {
		t.Errorf("expected 0 SPs, got %d", effort.TotalSPs)
	}
}

func TestParseParallelismConfig_WithLimits(t *testing.T) {
	content := `# Team Patterns

## Limites de Paralelismo

**Histórias Grandes/Complexas** (5+ SPs): Limite: 1
**Histórias Médias** (3-4 SPs): Limite: 2
**Histórias Pequenas** (1-2 SPs): Limite: 3
`

	config := parseParallelismConfig(content)
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.ParallelismLimits.LargeStories != 1 {
		t.Errorf("expected large=1, got %d", config.ParallelismLimits.LargeStories)
	}
	if config.ParallelismLimits.MediumStories != 2 {
		t.Errorf("expected medium=2, got %d", config.ParallelismLimits.MediumStories)
	}
	if config.ParallelismLimits.SmallStories != 3 {
		t.Errorf("expected small=3, got %d", config.ParallelismLimits.SmallStories)
	}
}

func TestParseParallelismConfig_NoSection(t *testing.T) {
	config := parseParallelismConfig("No relevant content here")
	if config != nil {
		t.Error("expected nil config without parallelism section")
	}
}

func TestExtractLimit(t *testing.T) {
	content := `**Histórias Grandes/Complexas** (5+ SPs): Limite: 2`
	result := extractLimit(content, "Grandes/Complexas", "5\\+ SPs")
	if result != 2 {
		t.Errorf("expected 2, got %d", result)
	}
}

func TestExtractLimit_NoMatch(t *testing.T) {
	result := extractLimit("nothing here", "Grandes/Complexas", "5\\+ SPs")
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}
}

func TestFindLargeStories(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				ID: "E1",
				Stories: []types.Story{
					{ID: "E1.1", Effort: 3, EpicID: "E1"},  // small
					{ID: "E1.2", Effort: 8, EpicID: "E1"},  // large
					{ID: "E1.3", Effort: 5, EpicID: "E1"},  // large
				},
			},
		},
	}

	limits := &ParallelismLimits{LargeStories: 1}
	large := findLargeStories(backlog, limits)

	if len(large) != 2 {
		t.Errorf("expected 2 large stories (effort >= 5), got %d", len(large))
	}
}

func TestFindLargeStories_NoLimits(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Stories: []types.Story{{Effort: 8}}},
		},
	}

	limits := &ParallelismLimits{LargeStories: 0} // no limit
	large := findLargeStories(backlog, limits)
	if len(large) != 0 {
		t.Errorf("expected 0 when no limit, got %d", len(large))
	}
}

func TestGenerateStoryReason(t *testing.T) {
	story := types.Story{Effort: 8, Tags: []string{"Kafka"}}
	limits := &ParallelismLimits{LargeStories: 1}

	reason := generateStoryReason(story, limits)
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestCountHighPriority(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{Priority: "high"},
			{Priority: "medium"},
			{Priority: "high"},
		},
	}

	count := countHighPriority(backlog)
	if count != 2 {
		t.Errorf("expected 2 high priority, got %d", count)
	}
}

func TestAnalyzeCriticalPath_NilLimits(t *testing.T) {
	result := analyzeCriticalPath(&types.Backlog{}, &EffortSummary{}, nil)
	if result != nil {
		t.Error("expected nil for nil limits")
	}
}

func TestAnalyzeCriticalPath_WithLargeStories(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend",
				Stories: []types.Story{
					{ID: "E1.1", Title: "Large Story", Effort: 8, EpicID: "E1"},
				},
			},
		},
	}

	effort := &EffortSummary{
		OptimisticDays: 4,
		ByEpic: map[string]EpicEffort{
			"E1": {SPs: 8},
		},
	}

	limits := &ParallelismLimits{LargeStories: 1}
	result := analyzeCriticalPath(backlog, effort, limits)

	if result == nil {
		t.Fatal("expected non-nil analysis")
	}
	if !result.Enabled {
		t.Error("expected enabled")
	}
	if len(result.LargeStories) != 1 {
		t.Errorf("expected 1 large story, got %d", len(result.LargeStories))
	}
}

func TestInferMilestones_TooFewEpics(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{{ID: "E1"}},
	}
	effort := &EffortSummary{ByEpic: map[string]EpicEffort{"E1": {SPs: 10}}}

	milestones := inferMilestones(backlog, effort)
	if len(milestones) != 0 {
		t.Errorf("expected 0 milestones for single epic, got %d", len(milestones))
	}
}

func TestInferMilestones_TwoEpics(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{ID: "E1", Title: "Backend"},
			{ID: "E2", Title: "Frontend"},
		},
	}
	effort := &EffortSummary{
		TotalSPs:  20,
		TotalDays: 10,
		ByEpic: map[string]EpicEffort{
			"E1": {SPs: 12, Days: 6},
			"E2": {SPs: 8, Days: 4},
		},
	}

	milestones := inferMilestones(backlog, effort)
	if len(milestones) != 2 {
		t.Errorf("expected 2 milestones for 2 epics, got %d", len(milestones))
	}
}

func TestLoadOrBuild_NonExistentFile(t *testing.T) {
	input := &Input{BacklogPath: "/nonexistent/backlog.json"}
	_, err := LoadOrBuild(input)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// DATA FOUNDATION TESTS (Fase 1: v3.3)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestConvertMeta_UsesProjectTitle(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{{ID: "E1"}},
		Meta: types.Metadata{
			ProjectTitle: "SPUI Alerts System",
			TotalEpics:   1,
			TotalStories: 5,
		},
	}

	meta := convertMeta(backlog)
	if meta.Title != "SPUI Alerts System" {
		t.Errorf("expected 'SPUI Alerts System', got %q", meta.Title)
	}
}

func TestConvertMeta_Fallback(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{{ID: "E1"}},
		Meta:  types.Metadata{}, // No ProjectTitle
	}

	meta := convertMeta(backlog)
	if meta.Title != "Backlog Técnico" {
		t.Errorf("expected fallback 'Backlog Técnico', got %q", meta.Title)
	}
}

func TestConvertToLensData_PassesMetrics(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{ID: "E1", Description: "Test epic description that is long enough for summary", Stories: []types.Story{{Effort: 3}}},
		},
		Meta: types.Metadata{
			ProjectTitle: "Test",
			TotalEpics:   1,
			TotalStories: 1,
			Metrics: &types.GenerationMetrics{
				TotalTechsExtracted: 15,
				TrivialFiltered:     5,
				TotalCost:           0.05,
			},
		},
	}

	result := ConvertToLensData(backlog, nil)
	if result.Metrics == nil {
		t.Fatal("expected Metrics to be non-nil in LensData")
	}
	if result.Metrics.TotalTechsExtracted != 15 {
		t.Errorf("expected TotalTechsExtracted=15, got %d", result.Metrics.TotalTechsExtracted)
	}
	if result.Metrics.TrivialFiltered != 5 {
		t.Errorf("expected TrivialFiltered=5, got %d", result.Metrics.TrivialFiltered)
	}
	if result.Metrics.TotalCost != 0.05 {
		t.Errorf("expected TotalCost=0.05, got %f", result.Metrics.TotalCost)
	}
}

func TestConvertToLensData_NilMetrics(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{
			{ID: "E1", Description: "Test epic description for testing", Stories: []types.Story{{Effort: 3}}},
		},
		Meta: types.Metadata{TotalEpics: 1, TotalStories: 1},
	}

	result := ConvertToLensData(backlog, nil)
	if result.Metrics != nil {
		t.Error("expected Metrics to be nil when backlog has no metrics")
	}
}

func TestConvertDeepDives_PassesClassification(t *testing.T) {
	deepDives := []types.DeepDive{
		{
			Term:           "Kafka",
			WhatIs:         "Event streaming",
			Classification: "critical",
			Scope:          "global",
		},
		{
			Term:           "Redis",
			WhatIs:         "Cache",
			StoryID:        "E1.1",
			Classification: "specific",
			Scope:          "story",
		},
	}

	result := convertDeepDives(deepDives)

	// Term-based key: global → just term
	dd1 := result["Kafka"]
	if dd1.Classification != "critical" {
		t.Errorf("expected Classification='critical', got %q", dd1.Classification)
	}
	if dd1.Scope != "global" {
		t.Errorf("expected Scope='global', got %q", dd1.Scope)
	}

	// Term-based key: contextualized → storyID:term
	dd2 := result["E1.1:Redis"]
	if dd2.Classification != "specific" {
		t.Errorf("expected Classification='specific', got %q", dd2.Classification)
	}
	if dd2.Scope != "story" {
		t.Errorf("expected Scope='story', got %q", dd2.Scope)
	}
}

func TestConvertMetrics_Nil(t *testing.T) {
	result := convertMetrics(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestConvertMetrics_Complete(t *testing.T) {
	metrics := &types.GenerationMetrics{
		TotalTechsExtracted:   20,
		TrivialFiltered:       8,
		ClassificationStats:   map[string]int{"trivial": 8, "standard": 7, "specific": 3, "critical": 2},
		CrossEpicGlobalDives:  3,
		CrossEpicDeduplicated: 4,
		LLMCallsMade:          10,
		LLMCallsSaved:         15,
		ReductionPercent:      60.0,
		TotalInputTokens:      5000,
		TotalOutputTokens:     2000,
		TotalCost:             0.07,
	}

	result := convertMetrics(metrics)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.TotalTechsExtracted != 20 {
		t.Errorf("expected 20, got %d", result.TotalTechsExtracted)
	}
	if result.TrivialFiltered != 8 {
		t.Errorf("expected 8, got %d", result.TrivialFiltered)
	}
	if result.CrossEpicGlobalDives != 3 {
		t.Errorf("expected 3, got %d", result.CrossEpicGlobalDives)
	}
	if result.ReductionPercent != 60.0 {
		t.Errorf("expected 60.0, got %f", result.ReductionPercent)
	}
	if result.TotalCost != 0.07 {
		t.Errorf("expected 0.07, got %f", result.TotalCost)
	}
	if result.ClassificationStats["critical"] != 2 {
		t.Errorf("expected critical=2, got %d", result.ClassificationStats["critical"])
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// v3.3: LENS UI POLISH TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestTruncateAtWord(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			"short string unchanged",
			"Hello world",
			120,
			"Hello world",
		},
		{
			"empty string",
			"",
			120,
			"",
		},
		{
			"truncates at word boundary",
			"Create the backend API for the system with all necessary endpoints and integrations for production deployment",
			50,
			"Create the backend API for the system with all...",
		},
		{
			"no spaces in long string",
			"abcdefghijklmnopqrstuvwxyz",
			10,
			"abcdefghij...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateAtWord(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestConvertMeta_LangFromBacklog(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{{ID: "E1"}},
		Meta: types.Metadata{
			ProjectTitle: "Test Project",
			Lang:         "en",
			TotalEpics:   1,
		},
	}

	meta := convertMeta(backlog)
	if meta.Lang != "en" {
		t.Errorf("expected Lang='en', got %q", meta.Lang)
	}
}

func TestConvertMeta_LangDefaultPtBR(t *testing.T) {
	backlog := &types.Backlog{
		Epics: []types.Epic{{ID: "E1"}},
		Meta:  types.Metadata{TotalEpics: 1},
	}

	meta := convertMeta(backlog)
	if meta.Lang != "pt-BR" {
		t.Errorf("expected Lang='pt-BR', got %q", meta.Lang)
	}
}

func TestConvertDeepDives_TermBasedKeys(t *testing.T) {
	deepDives := []types.DeepDive{
		{Term: "Go", WhatIs: "Language"},
		{Term: "Kafka", WhatIs: "Streaming", StoryID: "E1.1"},
		{Term: "Redis", WhatIs: "Cache", StoryID: "E2.1"},
	}

	result := convertDeepDives(deepDives)

	// Global key: just term
	if _, ok := result["Go"]; !ok {
		t.Error("expected key 'Go'")
	}

	// Contextualized keys: storyID:term
	if _, ok := result["E1.1:Kafka"]; !ok {
		t.Error("expected key 'E1.1:Kafka'")
	}
	if _, ok := result["E2.1:Redis"]; !ok {
		t.Error("expected key 'E2.1:Redis'")
	}

	// Old dd-N keys should NOT exist
	if _, ok := result["dd-1"]; ok {
		t.Error("should not have dd-1 key (old format)")
	}
}

func TestBuildExportJSON_IncludesNewFields(t *testing.T) {
	data := testLensData()
	jsData := buildExportJSON(data)

	// Check lang in meta
	meta := jsData["meta"].(map[string]interface{})
	if meta["lang"] != "pt-BR" {
		t.Errorf("expected lang='pt-BR', got %v", meta["lang"])
	}

	// Check metrics present
	if jsData["metrics"] == nil {
		t.Error("expected metrics in export JSON")
	}

	// Check effort present
	if jsData["effort"] == nil {
		t.Error("expected effort in export JSON")
	}

	// Check milestones present
	if jsData["milestones"] == nil {
		t.Error("expected milestones in export JSON")
	}

	// Check deep dive has classification
	dds := jsData["deep_dives"].(map[string]interface{})
	jwt, ok := dds["JWT"]
	if !ok {
		t.Fatal("expected deep dive key 'JWT'")
	}
	jwtMap := jwt.(map[string]interface{})
	if jwtMap["classification"] != "specific" {
		t.Errorf("expected classification='specific', got %v", jwtMap["classification"])
	}
}
