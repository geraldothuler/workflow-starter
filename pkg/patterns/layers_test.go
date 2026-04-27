package patterns

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetGoldenPathsCore(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetGoldenPathsCore()
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result, "DataDog") {
		t.Error("core should mention DataDog")
	}
	if !strings.Contains(result, "Kafka") {
		t.Error("core should mention Kafka")
	}
}

func TestGetTeamPatternsCore(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetTeamPatternsCore()
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result, "Kotlin") {
		t.Error("core should mention Kotlin")
	}
	if !strings.Contains(result, "CircleCI") {
		t.Error("core should mention CircleCI")
	}
}

func TestGetGoldenPathsEssentials(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetGoldenPathsEssentials()
	if !strings.Contains(result, "GP-001") {
		t.Error("essentials should contain GP-001")
	}
	if !strings.Contains(result, "GP-008") {
		t.Error("essentials should contain GP-008")
	}
}

func TestGetTeamPatternsEssentials(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetTeamPatternsEssentials()
	if !strings.Contains(result, "TP-001") {
		t.Error("essentials should contain TP-001")
	}
	if !strings.Contains(result, "TP-008") {
		t.Error("essentials should contain TP-008")
	}
}

func TestGetCombined_Core(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetCombined(Core)
	if !strings.Contains(result, "GOLDEN PATHS") {
		t.Error("combined core should contain golden paths")
	}
	if !strings.Contains(result, "TEAM PATTERNS") {
		t.Error("combined core should contain team patterns")
	}
}

func TestGetCombined_Essentials(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetCombined(Essentials)
	if !strings.Contains(result, "GP-001") {
		t.Error("combined essentials should contain GP references")
	}
	if !strings.Contains(result, "TP-001") {
		t.Error("combined essentials should contain TP references")
	}
}

func TestGetCombined_Full_NotEmpty(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetCombined(Full)
	if result == "" {
		t.Error("combined full should now return content from embedded files")
	}
	if !strings.Contains(result, "GP-001") {
		t.Error("full golden paths should contain GP-001")
	}
	if !strings.Contains(result, "TP-001") {
		t.Error("full team patterns should contain TP-001")
	}
}

func TestGetGoldenPathsFull(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetGoldenPathsFull()
	if result == "" {
		t.Fatal("expected non-empty full golden paths")
	}
	if !strings.Contains(result, "DOCUMENTAÇÃO COMPLETA") {
		t.Error("full should contain documentation header")
	}
}

func TestGetTeamPatternsFull(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetTeamPatternsFull()
	if result == "" {
		t.Fatal("expected non-empty full team patterns")
	}
	if !strings.Contains(result, "DOCUMENTAÇÃO COMPLETA") {
		t.Error("full should contain documentation header")
	}
}

func TestLoadEmbedded(t *testing.T) {
	result := loadEmbedded("golden-paths-core.md")
	if result == "" {
		t.Error("expected content from embedded file")
	}

	result = loadEmbedded("nonexistent.md")
	if result != "" {
		t.Error("expected empty for nonexistent file")
	}
}

func TestGetRecommendedLayer(t *testing.T) {
	pl := &PatternLayer{}

	tests := []struct {
		phase    string
		expected Layer
	}{
		{"extraction", Essentials},
		{"epics", Essentials},
		{"stories", Essentials},
		{"criteria", Core},
		{"deep-dive-tech", Full},
		{"deep-dive-story", Essentials},
		{"unknown", Core},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			result := pl.GetRecommendedLayer(tt.phase)
			if result != tt.expected {
				t.Errorf("expected layer %d for phase %q, got %d", tt.expected, tt.phase, result)
			}
		})
	}
}

func TestGetEstimatedTokens(t *testing.T) {
	pl := &PatternLayer{}

	coreTokens := pl.GetEstimatedTokens(Core)
	essTokens := pl.GetEstimatedTokens(Essentials)
	fullTokens := pl.GetEstimatedTokens(Full)

	if coreTokens >= essTokens {
		t.Errorf("core (%d) should be < essentials (%d)", coreTokens, essTokens)
	}
	if essTokens >= fullTokens {
		t.Errorf("essentials (%d) should be < full (%d)", essTokens, fullTokens)
	}
}

func TestGetEstimatedTokens_Unknown(t *testing.T) {
	pl := &PatternLayer{}
	result := pl.GetEstimatedTokens(Layer(99))
	if result != 0 {
		t.Errorf("expected 0 for unknown layer, got %d", result)
	}
}

func TestFormatWithHeader(t *testing.T) {
	pl := &PatternLayer{}

	tests := []struct {
		layer   Layer
		keyword string
	}{
		{Core, "OBRIGATÓRIOS"},
		{Essentials, "GOLDEN PATHS"},
		{Full, "DOCUMENTAÇÃO COMPLETA"},
	}

	for _, tt := range tests {
		result := pl.FormatWithHeader("test content", tt.layer)
		if !strings.Contains(result, tt.keyword) {
			t.Errorf("header for layer %d should contain %q", tt.layer, tt.keyword)
		}
		if !strings.Contains(result, "test content") {
			t.Error("should contain the patterns content")
		}
	}
}

func TestLayerConstants(t *testing.T) {
	if Core != 0 {
		t.Errorf("Core should be 0, got %d", Core)
	}
	if Essentials != 1 {
		t.Errorf("Essentials should be 1, got %d", Essentials)
	}
	if Full != 2 {
		t.Errorf("Full should be 2, got %d", Full)
	}
}

// --- Custom patterns tests ---

func TestLoadCustomPatterns_FromDir(t *testing.T) {
	dir := t.TempDir()

	// Create a custom pattern file
	content := "# Custom Pattern\nThis is a custom pattern."
	if err := os.WriteFile(filepath.Join(dir, "custom.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a non-md file (should be ignored)
	if err := os.WriteFile(filepath.Join(dir, "ignore.go"), []byte("package x"), 0644); err != nil {
		t.Fatal(err)
	}

	pl := &PatternLayer{}
	if err := pl.LoadCustomPatterns(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := pl.GetCombined(Core)
	if !strings.Contains(result, "Custom Pattern") {
		t.Error("combined should include custom patterns")
	}
}

func TestLoadCustomPatterns_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	pl := &PatternLayer{}
	if err := pl.LoadCustomPatterns(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still work with no custom patterns
	result := pl.GetCombined(Core)
	if result == "" {
		t.Error("combined should still return embedded patterns")
	}
}

func TestLoadCustomPatterns_InvalidDir(t *testing.T) {
	pl := &PatternLayer{}
	err := pl.LoadCustomPatterns("/nonexistent/dir")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
