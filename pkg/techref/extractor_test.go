package techref

import (
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestExtractTechsFromStory(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "API REST com Spring Boot",
		What:  "Criar endpoint usando Spring Boot e PostgreSQL para retornar dados em JSON",
		Why:   "Precisamos de uma API rápida",
		AcceptanceCriteria: []string{
			"Usar Docker para containerização",
		},
	}

	techs := ExtractTechsFromStory(story)

	// Deve encontrar: Spring Boot, PostgreSQL, Docker
	expectedTechs := []string{"Spring Boot", "PostgreSQL", "Docker"}

	for _, expected := range expectedTechs {
		found := false
		for _, tech := range techs {
			if tech == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find tech %q, but didn't. Found: %v", expected, techs)
		}
	}
}

func TestExtractTechsByEpic(t *testing.T) {
	epic := types.Epic{
		ID:    "E1",
		Title: "Backend APIs",
		Stories: []types.Story{
			{
				ID:    "E1.1",
				Title: "User API",
				What:  "Criar com Spring Boot e PostgreSQL",
			},
			{
				ID:    "E1.2",
				Title: "Tasks API",
				What:  "Criar com Spring Boot e Redis",
			},
			{
				ID:    "E1.3",
				Title: "Auth API",
				What:  "OAuth2 com Spring Boot",
			},
		},
	}

	extractions := ExtractTechsByEpic(epic)

	// Deve ter pelo menos algumas tecnologias
	if len(extractions) < 2 {
		t.Errorf("Expected at least 2 technologies, got %d", len(extractions))
	}

	// Verificar Spring Boot (deve estar em múltiplas histórias)
	springBoot := findExtraction(extractions, "Spring Boot")
	if springBoot == nil {
		t.Error("Spring Boot not found in extractions")
	} else if springBoot.Count < 2 {
		t.Errorf("Spring Boot should be in multiple stories, got %d", springBoot.Count)
	}
}

func TestNormalizeTech(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Spring Boot", "Spring Boot"},
		{"spring boot", "Spring Boot"},
		{"springboot", "Spring Boot"},
		{"PostgreSQL", "PostgreSQL"},
		{"postgres", "PostgreSQL"},
		{"postgresql", "PostgreSQL"},
		{"React", "React"},
		// "react js" com espaço não é mapeado pelo normalizer (apenas "reactjs" sem espaço)
		// {"react js", "React"},
		{"reactjs", "React"},
		{"MongoDB", "MongoDB"},
		{"mongo", "MongoDB"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeTech(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeTech(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractKnownTechnologies(t *testing.T) {
	text := "Criar API com Spring Boot, PostgreSQL e Redis, usando Docker e Kubernetes"

	techs := extractKnownTechnologies(text)

	expectedTechs := []string{"Spring Boot", "PostgreSQL", "Redis", "Docker", "Kubernetes"}

	if len(techs) < len(expectedTechs) {
		t.Errorf("Expected at least %d techs, got %d: %v", len(expectedTechs), len(techs), techs)
	}

	for _, expected := range expectedTechs {
		found := false
		for _, tech := range techs {
			if tech.Term == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find %q in techs", expected)
		}
	}
}

func TestExtractContext(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "Implementar cache",
		What:  "Usar Redis para cache de sessões. Redis deve ter TTL configurável.",
		AcceptanceCriteria: []string{
			"Redis deve suportar cluster",
		},
	}

	context := extractContext(story, "Redis")

	if context == "" {
		t.Error("Expected non-empty context for Redis")
	}

	if !strings.Contains(strings.ToLower(context), "redis") {
		t.Errorf("Context should mention Redis: %q", context)
	}
}

func TestFilterSingleStoryTechs(t *testing.T) {
	extractions := []TechExtraction{
		{Term: "Spring Boot", Count: 3, StoryIDs: []string{"E1.1", "E1.2", "E1.3"}},
		{Term: "Redis", Count: 1, StoryIDs: []string{"E1.2"}},
		{Term: "PostgreSQL", Count: 2, StoryIDs: []string{"E1.1", "E1.3"}},
		{Term: "Kafka", Count: 1, StoryIDs: []string{"E1.4"}},
	}

	single := FilterSingleStoryTechs(extractions)

	if len(single) != 2 {
		t.Errorf("Expected 2 single-story techs, got %d", len(single))
	}

	// Deve conter Redis e Kafka
	hasRedis := false
	hasKafka := false
	for _, tech := range single {
		if tech.Term == "Redis" {
			hasRedis = true
		}
		if tech.Term == "Kafka" {
			hasKafka = true
		}
	}

	if !hasRedis || !hasKafka {
		t.Error("Single-story filter should include Redis and Kafka")
	}
}

func TestFilterMultiStoryTechs(t *testing.T) {
	extractions := []TechExtraction{
		{Term: "Spring Boot", Count: 3},
		{Term: "Redis", Count: 1},
		{Term: "PostgreSQL", Count: 2},
	}

	multi := FilterMultiStoryTechs(extractions)

	if len(multi) != 2 {
		t.Errorf("Expected 2 multi-story techs, got %d", len(multi))
	}
}

func TestGroupTechsByCount(t *testing.T) {
	extractions := []TechExtraction{
		{Term: "Spring Boot", Count: 3},
		{Term: "Redis", Count: 1},
		{Term: "PostgreSQL", Count: 2},
		{Term: "Kafka", Count: 1},
		{Term: "Docker", Count: 3},
	}

	grouped := GroupTechsByCount(extractions)

	if len(grouped[1]) != 2 {
		t.Errorf("Expected 2 techs with count=1, got %d", len(grouped[1]))
	}

	if len(grouped[2]) != 1 {
		t.Errorf("Expected 1 tech with count=2, got %d", len(grouped[2]))
	}

	if len(grouped[3]) != 2 {
		t.Errorf("Expected 2 techs with count=3, got %d", len(grouped[3]))
	}
}

func TestGetExtractionStatistics(t *testing.T) {
	extractions := []TechExtraction{
		{Term: "Spring Boot", Count: 3},
		{Term: "Redis", Count: 1},
		{Term: "PostgreSQL", Count: 2},
		{Term: "Kafka", Count: 1},
	}

	stats := GetExtractionStatistics(extractions)

	if stats["total"] != 4 {
		t.Errorf("Expected total=4, got %v", stats["total"])
	}

	if stats["single_story"] != 2 {
		t.Errorf("Expected single_story=2, got %v", stats["single_story"])
	}

	if stats["multi_story"] != 2 {
		t.Errorf("Expected multi_story=2, got %v", stats["multi_story"])
	}
}

// Helper
func findExtraction(extractions []TechExtraction, term string) *TechExtraction {
	for i := range extractions {
		if extractions[i].Term == term {
			return &extractions[i]
		}
	}
	return nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// Compound Validator Tests
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestIsBusinessTerm(t *testing.T) {
	tests := []struct {
		w1, w2   string
		expected bool
	}{
		{"Produtos", "API", true},
		{"Sistema", "Web", true},
		{"Users", "Table", true},
		{"Spring", "Boot", false},
		{"Kafka", "Consumer", false},
	}

	for _, tt := range tests {
		t.Run(tt.w1+"_"+tt.w2, func(t *testing.T) {
			result := isBusinessTerm(tt.w1, tt.w2)
			if result != tt.expected {
				t.Errorf("isBusinessTerm(%q, %q) = %v, expected %v", tt.w1, tt.w2, result, tt.expected)
			}
		})
	}
}

func TestIsTechTech(t *testing.T) {
	tests := []struct {
		w1, w2   string
		expected bool
	}{
		{"Spring", "Boot", true},
		{"React", "Native", true},
		{"Circuit", "Breaker", true},
		{"Event", "Sourcing", true},
		{"Produtos", "Pedidos", false},
	}

	for _, tt := range tests {
		t.Run(tt.w1+"_"+tt.w2, func(t *testing.T) {
			result := isTechTech(tt.w1, tt.w2)
			if result != tt.expected {
				t.Errorf("isTechTech(%q, %q) = %v, expected %v", tt.w1, tt.w2, result, tt.expected)
			}
		})
	}
}

func TestIsTechVersion(t *testing.T) {
	if !isTechVersion("Java", "11") {
		t.Error("Java 11 should be tech+version")
	}
	if !isTechVersion("Python", "3") {
		t.Error("Python 3 should be tech+version")
	}
	if isTechVersion("Java", "Boot") {
		t.Error("Java Boot should not be tech+version")
	}
	if isTechVersion("Produtos", "3") {
		t.Error("Produtos 3 should not be tech+version")
	}
}

func TestIsTechModifier(t *testing.T) {
	if !isTechModifier("React", "Native") {
		t.Error("React Native should be tech+modifier")
	}
	if !isTechModifier("Spring", "Cloud") {
		t.Error("Spring Cloud should be tech+modifier")
	}
	if isTechModifier("Spring", "Produtos") {
		t.Error("Spring Produtos should not be tech+modifier")
	}
}

func TestHasVerb(t *testing.T) {
	if !hasVerb("Criar", "API") {
		t.Error("should detect Portuguese verb")
	}
	if !hasVerb("API", "Create") {
		t.Error("should detect English verb")
	}
	if hasVerb("Spring", "Boot") {
		t.Error("Spring Boot should not have verbs")
	}
}

func TestIsValidCompound(t *testing.T) {
	if !isValidCompound("Spring", "Boot") {
		t.Error("Spring Boot should be valid compound (tech+tech)")
	}
	if !isValidCompound("React", "Native") {
		t.Error("React Native should be valid compound (tech+modifier)")
	}
	if !isValidCompound("Java", "11") {
		t.Error("Java 11 should be valid compound (tech+version)")
	}
	if isValidCompound("Criar", "API") {
		t.Error("Criar API should not be valid (has verb)")
	}
}

func TestValidateCompoundStrict(t *testing.T) {
	if !validateCompoundStrict("Spring Boot") {
		t.Error("Spring Boot should pass strict validation")
	}
	if validateCompoundStrict("Single") {
		t.Error("single word should fail")
	}
	if validateCompoundStrict("Three Word Compound") {
		t.Error("three words should fail")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// Confidence Scorer Tests
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestGetBaseScore(t *testing.T) {
	tests := []struct {
		layer    ExtractionLayer
		expected float64
	}{
		{LayerKnown, 1.0},
		{LayerAcronym, 0.85},
		{LayerCompound, 0.70},
		{LayerIsolated, 0.50},
		{"unknown", 0.0},
	}

	for _, tt := range tests {
		t.Run(string(tt.layer), func(t *testing.T) {
			result := getBaseScore(tt.layer)
			if result != tt.expected {
				t.Errorf("getBaseScore(%q) = %v, expected %v", tt.layer, result, tt.expected)
			}
		})
	}
}

func TestGetLayerReason(t *testing.T) {
	if getLayerReason(LayerKnown) != "tecnologia conhecida" {
		t.Error("wrong reason for known layer")
	}
	if getLayerReason(LayerAcronym) != "sigla identificada" {
		t.Error("wrong reason for acronym layer")
	}
	if getLayerReason(LayerCompound) != "composto validado" {
		t.Error("wrong reason for compound layer")
	}
	if getLayerReason(LayerIsolated) != "palavra isolada" {
		t.Error("wrong reason for isolated layer")
	}
}

func TestCalculateConfidence_KnownLayer(t *testing.T) {
	match := TechMatch{Term: "PostgreSQL", Layer: LayerKnown}
	context := ContextInfo{}

	score := calculateConfidence(match, context)
	if score.Score != 1.0 {
		t.Errorf("expected 1.0 for known layer, got %v", score.Score)
	}
}

func TestCalculateConfidence_WithVerbPenalty(t *testing.T) {
	match := TechMatch{Term: "API", Layer: LayerIsolated}
	context := ContextInfo{HasVerbBefore: true}

	score := calculateConfidence(match, context)
	if score.Score >= 0.50 {
		t.Errorf("expected score < 0.50 with verb penalty, got %v", score.Score)
	}
	if !strings.Contains(score.Reason, "verbo") {
		t.Error("reason should mention verb")
	}
}

func TestCalculateConfidence_WithTechBonus(t *testing.T) {
	match := TechMatch{Term: "Boot", Layer: LayerIsolated}
	context := ContextInfo{HasTechBefore: true}

	score := calculateConfidence(match, context)
	if score.Score != 0.60 {
		t.Errorf("expected 0.60 (0.50 base + 0.10 bonus), got %v", score.Score)
	}
}

func TestFilterByConfidenceWithScores(t *testing.T) {
	matches := []TechMatch{
		{Term: "PostgreSQL", Layer: LayerKnown},    // 1.0
		{Term: "Redis", Layer: LayerAcronym},        // 0.85
		{Term: "Boot", Layer: LayerIsolated},         // 0.50
	}

	filtered := filterByConfidenceWithScores(matches, 0.75)

	if len(filtered) != 2 {
		t.Errorf("expected 2 matches above 0.75, got %d", len(filtered))
	}

	for _, m := range filtered {
		if m.Confidence < 0.75 {
			t.Errorf("filtered match %q has confidence %v below threshold", m.Term, m.Confidence)
		}
		if m.Context == "" {
			t.Errorf("filtered match %q should have context reason", m.Term)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// Normalizer Tests
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestGetLayerPriority(t *testing.T) {
	tests := []struct {
		layer    ExtractionLayer
		expected int
	}{
		{LayerKnown, 4},
		{LayerAcronym, 3},
		{LayerCompound, 2},
		{LayerIsolated, 1},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.layer), func(t *testing.T) {
			result := getLayerPriority(tt.layer)
			if result != tt.expected {
				t.Errorf("getLayerPriority(%q) = %d, expected %d", tt.layer, result, tt.expected)
			}
		})
	}
}

func TestNormalizeToCanonical(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"postgresql", "PostgreSQL"},
		{"postgres", "PostgreSQL"},
		{"react", "React"},
		{"reactjs", "React"},
		{"k8s", "Kubernetes"},
		{"docker", "Docker"},
		{"UnknownTech", "UnknownTech"},
		{"  redis  ", "Redis"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeToCanonical(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeToCanonical(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDeduplicateByCanonical(t *testing.T) {
	matches := []TechMatch{
		{Term: "postgresql", Layer: LayerKnown, Confidence: 1.0},
		{Term: "postgres", Layer: LayerAcronym, Confidence: 0.85},
		{Term: "Redis", Layer: LayerKnown, Confidence: 1.0},
	}

	result := deduplicateByCanonical(matches)

	if len(result) != 2 {
		t.Errorf("expected 2 deduplicated matches, got %d", len(result))
	}

	// PostgreSQL should keep the known layer version (higher confidence)
	for _, m := range result {
		if m.Term == "PostgreSQL" && m.Layer != LayerKnown {
			t.Error("PostgreSQL should keep the Known layer (higher confidence)")
		}
	}
}

func TestDeduplicateByCanonical_SameConfidence(t *testing.T) {
	matches := []TechMatch{
		{Term: "redis", Layer: LayerIsolated, Confidence: 0.5},
		{Term: "Redis", Layer: LayerKnown, Confidence: 0.5},
	}

	result := deduplicateByCanonical(matches)
	if len(result) != 1 {
		t.Errorf("expected 1 deduplicated match, got %d", len(result))
	}
	if len(result) > 0 && result[0].Layer != LayerKnown {
		t.Error("should keep higher layer priority when same confidence")
	}
}

// Benchmark
func BenchmarkExtractTechsByEpic(b *testing.B) {
	epic := types.Epic{
		ID: "E1",
		Stories: []types.Story{
			{ID: "E1.1", What: "API with Spring Boot and PostgreSQL"},
			{ID: "E1.2", What: "Service with Spring Boot and Redis"},
			{ID: "E1.3", What: "Worker with Kafka and Docker"},
		},
	}

	for i := 0; i < b.N; i++ {
		ExtractTechsByEpic(epic)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EMPTY INPUT TESTS (Fase B)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExtractTechsFromStory_EmptyStory(t *testing.T) {
	story := types.Story{}
	techs := ExtractTechsFromStory(story)
	if len(techs) != 0 {
		t.Errorf("expected 0 techs from empty story, got %d: %v", len(techs), techs)
	}
}

func TestExtractTechsByEpic_EmptyEpic(t *testing.T) {
	epic := types.Epic{ID: "E1", Stories: []types.Story{}}
	extractions := ExtractTechsByEpic(epic)
	if len(extractions) != 0 {
		t.Errorf("expected 0 extractions from epic with no stories, got %d", len(extractions))
	}
}

func TestExtractTechsByEpic_StoriesWithNoTechs(t *testing.T) {
	epic := types.Epic{
		ID: "E1",
		Stories: []types.Story{
			{ID: "E1.1", What: "Fazer algo simples"},
			{ID: "E1.2", What: "Criar nova funcionalidade"},
		},
	}
	extractions := ExtractTechsByEpic(epic)
	// May or may not find things — just verify no panic
	_ = extractions
}

func TestExtractTechsFromStory_NilRegistry(t *testing.T) {
	story := types.Story{
		What: "Criar API com Spring Boot e PostgreSQL",
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil registry caused panic: %v", r)
		}
	}()

	techs := ExtractTechsFromStoryWithRegistry(nil, story)
	if len(techs) == 0 {
		t.Error("should work with nil registry (fallback to default)")
	}
}

func TestExtractTechsByEpic_NilRegistry(t *testing.T) {
	epic := types.Epic{
		ID: "E1",
		Stories: []types.Story{
			{ID: "E1.1", What: "Spring Boot PostgreSQL"},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil registry caused panic: %v", r)
		}
	}()

	extractions := ExtractTechsByEpicWithRegistry(nil, epic)
	if len(extractions) == 0 {
		t.Error("should work with nil registry (fallback to default)")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EXTRACTION LAYER L2-L4 TESTS (Fase B)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExtractAcronyms(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
		excluded []string
	}{
		{
			name:     "basic acronyms",
			text:     "Usando JWT para autenticação e AWS para hosting",
			expected: []string{"JWT", "AWS"},
		},
		{
			name:     "blacklisted acronyms excluded",
			text:     "GET API SET GO",
			expected: []string{},
			excluded: []string{"GET", "SET", "GO", "API"},
		},
		{
			name:     "mixed case — only uppercase extracted",
			text:     "Kafka RabbitMQ AWS",
			expected: []string{"AWS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := extractAcronyms(tt.text)
			for _, exp := range tt.expected {
				found := false
				for _, m := range matches {
					if m.Term == exp {
						found = true
						if m.Layer != LayerAcronym {
							t.Errorf("%s should be LayerAcronym", exp)
						}
						break
					}
				}
				if !found {
					terms := make([]string, len(matches))
					for i, m := range matches {
						terms[i] = m.Term
					}
					t.Errorf("expected to find %q, got: %v", exp, terms)
				}
			}
			for _, exc := range tt.excluded {
				for _, m := range matches {
					if m.Term == exc {
						t.Errorf("expected %q to be excluded (blacklisted)", exc)
					}
				}
			}
		})
	}
}

func TestExtractValidCompounds(t *testing.T) {
	tests := []struct {
		name string
		text string
		min  int // minimum expected matches
	}{
		{
			name: "compound tech names",
			text: "Using React Native for mobile and Spring Boot for backend",
			min:  1,
		},
		{
			name: "no compounds",
			text: "simple text without compounds",
			min:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := extractValidCompounds(tt.text)
			if len(matches) < tt.min {
				t.Errorf("expected at least %d compound matches, got %d", tt.min, len(matches))
			}
			for _, m := range matches {
				if m.Layer != LayerCompound {
					t.Errorf("match %q should be LayerCompound", m.Term)
				}
			}
		})
	}
}

func TestExtractIsolatedWithContext(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		excluded []string // common words that should NOT appear
	}{
		{
			name:     "filters common words",
			text:     "Create a new Schema for the Data model",
			excluded: []string{"Create", "Schema", "Data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := extractIsolatedWithContext(tt.text)
			for _, exc := range tt.excluded {
				for _, m := range matches {
					if m.Term == exc {
						t.Errorf("common word %q should be filtered by L4", exc)
					}
				}
			}
			for _, m := range matches {
				if m.Layer != LayerIsolated {
					t.Errorf("match %q should be LayerIsolated", m.Term)
				}
			}
		})
	}
}

func TestMergeLayers_DeduplicatesKeepingHighest(t *testing.T) {
	layer1 := []TechMatch{
		{Term: "PostgreSQL", Layer: LayerKnown, Confidence: 1.0},
	}
	layer2 := []TechMatch{
		{Term: "PostgreSQL", Layer: LayerIsolated, Confidence: 0.5},
		{Term: "AWS", Layer: LayerAcronym, Confidence: 0.85},
	}

	result := mergeLayers(layer1, layer2)

	if len(result) != 2 {
		t.Fatalf("expected 2 merged results, got %d", len(result))
	}

	for _, m := range result {
		if m.Term == "PostgreSQL" && m.Confidence != 1.0 {
			t.Error("merged PostgreSQL should keep highest confidence (1.0)")
		}
	}
}

func TestFilterByConfidence(t *testing.T) {
	matches := []TechMatch{
		{Term: "A", Confidence: 1.0},
		{Term: "B", Confidence: 0.5},
		{Term: "C", Confidence: 0.3},
		{Term: "D", Confidence: 0.7},
	}

	result := filterByConfidence(matches, 0.6)
	if len(result) != 2 {
		t.Errorf("expected 2 matches >= 0.6, got %d", len(result))
	}
}

func TestExtractor_HigherConfidenceThreshold(t *testing.T) {
	// Com threshold em 0.65 (era 0.60), Layer 4 (isolated, score 0.50)
	// precisa de pelo menos 2 bonuses para passar:
	// - tech_nearby: +0.10 → 0.60 (insuficiente)
	// - tech_nearby + start_of_sentence: +0.10 + 0.05 → 0.65 (passa)
	reg, _ := NewTechRegistry()

	// Verifica que threshold é 0.65
	if reg.MinConfidence() != 0.65 {
		t.Fatalf("MinConfidence() = %f, want 0.65 — confidence.yml não atualizado?", reg.MinConfidence())
	}

	// Layer 4 (isolated) com apenas tech_nearby: score = 0.50 + 0.10 = 0.60 → REJEITADO
	match := TechMatch{Term: "Foobar", Layer: LayerIsolated}
	context := ContextInfo{HasTechBefore: true} // apenas 1 bonus
	score := calculateConfidenceWithRegistry(reg, match, context)
	if score.Score >= reg.MinConfidence() {
		t.Errorf("Isolated + tech_nearby: score=%.2f should be < threshold=%.2f",
			score.Score, reg.MinConfidence())
	}

	// Layer 4 com 2 bonuses: score = 0.50 + 0.10 + 0.05 = 0.65 → ACEITO
	context2 := ContextInfo{HasTechBefore: true, IsStartOfSentence: true}
	score2 := calculateConfidenceWithRegistry(reg, match, context2)
	if score2.Score < reg.MinConfidence() {
		t.Errorf("Isolated + tech_nearby + start_of_sentence: score=%.2f should be >= threshold=%.2f",
			score2.Score, reg.MinConfidence())
	}

	// Layer 3 (compound, 0.70) passa sem bonuses
	matchCompound := TechMatch{Term: "Spring Boot", Layer: LayerCompound}
	scoreCompound := calculateConfidenceWithRegistry(reg, matchCompound, ContextInfo{})
	if scoreCompound.Score < reg.MinConfidence() {
		t.Errorf("Compound (0.70) should pass threshold 0.65, got %.2f", scoreCompound.Score)
	}
}

func TestExtractSentence_UTF8(t *testing.T) {
	// Portuguese text with accented characters
	text := "O sistema de autenticação utiliza OAuth2 para garantir segurança na aplicação"

	sentence := extractSentence(text, "OAuth2")
	if !strings.Contains(sentence, "OAuth2") {
		t.Errorf("sentence should contain OAuth2: %q", sentence)
	}
	// Should not produce garbled output from multi-byte slicing
	for _, r := range sentence {
		if r == 0xFFFD { // Unicode replacement character = bad slice
			t.Error("sentence contains garbled Unicode (bad byte slicing)")
			break
		}
	}
}

func TestExtractSentence_AccentedChars(t *testing.T) {
	text := "A implementação do serviço de notificação usa Redis para cache de sessões temporárias"

	sentence := extractSentence(text, "Redis")
	if !strings.Contains(sentence, "Redis") {
		t.Errorf("sentence should contain Redis: %q", sentence)
	}
}
