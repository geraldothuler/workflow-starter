package techref

import (
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestClassifyTech_Trivial(t *testing.T) {
	config := DefaultClassifierConfig()
	epic := types.Epic{
		ID:    "E1",
		Title: "API Backend",
		Stories: []types.Story{
			{ID: "E1.1", Title: "Criar endpoint HTTP", What: "Retornar JSON"},
		},
	}

	// HTTP é trivial
	result := ClassifyTech("HTTP", epic, config)

	if result.Relevance != TRIVIAL {
		t.Errorf("Expected TRIVIAL for HTTP, got %v", result.Relevance)
	}

	if result.Scope != "none" {
		t.Errorf("Expected scope 'none' for trivial, got %v", result.Scope)
	}
}

func TestClassifyTech_Critical(t *testing.T) {
	config := DefaultClassifierConfig()
	config.CoreTechnologies = []string{"Kafka", "PostgreSQL"}

	epic := types.Epic{
		ID:    "E1",
		Title: "Event Streaming",
		Stories: []types.Story{
			{ID: "E1.1", Title: "Setup Kafka", What: "Configurar Kafka"},
			{ID: "E1.2", Title: "Consumer", What: "Consumir de Kafka"},
		},
	}

	result := ClassifyTech("Kafka", epic, config)

	if result.Relevance != CRITICAL {
		t.Errorf("Expected CRITICAL for core tech Kafka, got %v", result.Relevance)
	}

	if result.Scope != "epic" {
		t.Errorf("Expected scope 'epic' for critical, got %v", result.Scope)
	}

	if result.EpicID != "E1" {
		t.Errorf("Expected EpicID 'E1', got %v", result.EpicID)
	}
}

func TestClassifyTech_Specific_SingleStory(t *testing.T) {
	config := DefaultClassifierConfig()

	epic := types.Epic{
		ID:    "E1",
		Title: "Backend APIs",
		Stories: []types.Story{
			{ID: "E1.1", Title: "User API", What: "Criar API REST"},
			{ID: "E1.2", Title: "Redis Cache", What: "Implementar cache com Redis"},
			{ID: "E1.3", Title: "Tasks API", What: "Criar API REST"},
		},
	}

	// Redis aparece em apenas 1 história → SPECIFIC
	result := ClassifyTech("Redis", epic, config)

	if result.Relevance != SPECIFIC {
		t.Errorf("Expected SPECIFIC for Redis (single story), got %v", result.Relevance)
	}

	if result.Scope != "story" {
		t.Errorf("Expected scope 'story' for specific, got %v", result.Scope)
	}

	if result.StoryID != "E1.2" {
		t.Errorf("Expected StoryID 'E1.2', got %v", result.StoryID)
	}
}

func TestClassifyTech_Standard(t *testing.T) {
	config := DefaultClassifierConfig()

	epic := types.Epic{
		ID:    "E1",
		Title: "Backend APIs",
		Stories: []types.Story{
			{ID: "E1.1", Title: "User API", What: "Criar com Spring Boot"},
			{ID: "E1.2", Title: "Tasks API", What: "Criar com Spring Boot"},
			{ID: "E1.3", Title: "Auth API", What: "Criar com Spring Boot"},
		},
	}

	// Spring Boot usado em múltiplas histórias → STANDARD
	result := ClassifyTech("Spring Boot", epic, config)

	if result.Relevance != STANDARD {
		t.Errorf("Expected STANDARD for Spring Boot (multiple stories), got %v", result.Relevance)
	}

	if result.Scope != "epic" {
		t.Errorf("Expected scope 'epic' for standard, got %v", result.Scope)
	}
}

func TestClassifyAllTechsInEpic(t *testing.T) {
	config := DefaultClassifierConfig()
	config.CoreTechnologies = []string{"PostgreSQL"}

	epic := types.Epic{
		ID:    "E1",
		Title: "Backend",
		Stories: []types.Story{
			{ID: "E1.1", Title: "API", What: "HTTP JSON PostgreSQL"},
			{ID: "E1.2", Title: "Cache", What: "Redis cache"},
		},
	}

	// Testar classificação individual
	httpClass := ClassifyTech("HTTP", epic, config)
	if httpClass.Relevance != TRIVIAL {
		t.Errorf("HTTP should be TRIVIAL")
	}

	pgClass := ClassifyTech("PostgreSQL", epic, config)
	if pgClass.Relevance != CRITICAL {
		t.Errorf("PostgreSQL should be CRITICAL (core tech)")
	}

	redisClass := ClassifyTech("Redis", epic, config)
	if redisClass.Relevance != SPECIFIC {
		t.Errorf("Redis should be SPECIFIC (single story)")
	}
}

func TestFilterByRelevance(t *testing.T) {
	classifications := []TechClassification{
		{Term: "HTTP", Relevance: TRIVIAL},
		{Term: "Kafka", Relevance: CRITICAL},
		{Term: "Redis", Relevance: SPECIFIC},
		{Term: "Spring Boot", Relevance: STANDARD},
	}

	critical := FilterByRelevance(classifications, CRITICAL)
	if len(critical) != 1 || critical[0].Term != "Kafka" {
		t.Errorf("FilterByRelevance(CRITICAL) failed")
	}

	trivial := FilterByRelevance(classifications, TRIVIAL)
	if len(trivial) != 1 || trivial[0].Term != "HTTP" {
		t.Errorf("FilterByRelevance(TRIVIAL) failed")
	}
}

func TestGroupByScope_Internal(t *testing.T) {
	classifications := []TechClassification{
		{Term: "HTTP", Relevance: TRIVIAL, Scope: "none"},
		{Term: "Kafka", Relevance: CRITICAL, Scope: "epic"},
		{Term: "Redis", Relevance: SPECIFIC, Scope: "story"},
		{Term: "Spring Boot", Relevance: STANDARD, Scope: "epic"},
	}

	grouped := groupByScope(classifications)

	if len(grouped["none"]) != 1 {
		t.Errorf("Expected 1 in 'none' scope, got %d", len(grouped["none"]))
	}

	if len(grouped["epic"]) != 2 {
		t.Errorf("Expected 2 in 'epic' scope, got %d", len(grouped["epic"]))
	}

	if len(grouped["story"]) != 1 {
		t.Errorf("Expected 1 in 'story' scope, got %d", len(grouped["story"]))
	}
}

func TestGetStatistics(t *testing.T) {
	classifications := []TechClassification{
		{Term: "HTTP", Relevance: TRIVIAL},
		{Term: "JSON", Relevance: TRIVIAL},
		{Term: "Kafka", Relevance: CRITICAL},
		{Term: "Redis", Relevance: SPECIFIC},
		{Term: "Spring Boot", Relevance: STANDARD},
		{Term: "PostgreSQL", Relevance: STANDARD},
	}

	stats := GetStatistics(classifications)

	if stats["total"] != 6 {
		t.Errorf("Expected total 6, got %d", stats["total"])
	}

	if stats["trivial"] != 2 {
		t.Errorf("Expected 2 trivial, got %d", stats["trivial"])
	}

	if stats["critical"] != 1 {
		t.Errorf("Expected 1 critical, got %d", stats["critical"])
	}

	if stats["specific"] != 1 {
		t.Errorf("Expected 1 specific, got %d", stats["specific"])
	}

	if stats["standard"] != 2 {
		t.Errorf("Expected 2 standard, got %d", stats["standard"])
	}
}

func TestShouldGenerateDeepDive_Internal(t *testing.T) {
	tests := []struct {
		relevance TechRelevance
		expected  bool
	}{
		{TRIVIAL, false},
		{STANDARD, true},
		{SPECIFIC, true},
		{CRITICAL, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.relevance), func(t *testing.T) {
			classification := TechClassification{Relevance: tt.relevance}
			result := shouldGenerateDeepDive(classification)

			if result != tt.expected {
				t.Errorf("shouldGenerateDeepDive(%v) = %v, expected %v",
					tt.relevance, result, tt.expected)
			}
		})
	}
}

func TestGetDeepDiveScope_Internal(t *testing.T) {
	tests := []struct {
		relevance TechRelevance
		expected  string
	}{
		{TRIVIAL, "none"},
		{STANDARD, "epic"},
		{SPECIFIC, "story"},
		{CRITICAL, "epic"},
	}

	for _, tt := range tests {
		t.Run(string(tt.relevance), func(t *testing.T) {
			classification := TechClassification{Relevance: tt.relevance}
			result := getDeepDiveScope(classification)

			if result != tt.expected {
				t.Errorf("getDeepDiveScope(%v) = %v, expected %v",
					tt.relevance, result, tt.expected)
			}
		})
	}
}

// Benchmark
func BenchmarkClassifyTech(b *testing.B) {
	config := DefaultClassifierConfig()
	epic := types.Epic{
		ID:    "E1",
		Title: "Test",
		Stories: []types.Story{
			{ID: "E1.1", Title: "Story 1", What: "Spring Boot API"},
			{ID: "E1.2", Title: "Story 2", What: "Spring Boot service"},
		},
	}

	for i := 0; i < b.N; i++ {
		ClassifyTech("Spring Boot", epic, config)
	}
}
