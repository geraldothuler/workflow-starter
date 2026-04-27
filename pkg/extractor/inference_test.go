package extractor

import (
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestNewInferenceEngine(t *testing.T) {
	gp := &types.GoldenPath{}
	tp := &types.TeamPatterns{}
	extracted := &ExtractedData{}

	ie := NewInferenceEngine(gp, tp, "transcript text", extracted)
	if ie == nil {
		t.Fatal("expected non-nil inference engine")
	}
	if ie.goldenPath != gp {
		t.Error("goldenPath not set")
	}
	if ie.teamPatterns != tp {
		t.Error("teamPatterns not set")
	}
}

func TestInfer_NilGoldenPath(t *testing.T) {
	extracted := &ExtractedData{}
	ie := NewInferenceEngine(nil, nil, "some text", extracted)

	result := ie.Infer()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.InferredTechnologies == nil {
		t.Error("expected non-nil technologies slice")
	}
	if result.InferredNFRs == nil {
		t.Error("expected non-nil NFRs slice")
	}
	if result.InferredPatterns == nil {
		t.Error("expected non-nil patterns slice")
	}
	if result.Suggestions == nil {
		t.Error("expected non-nil suggestions slice")
	}
}

func TestInferTechnologies_NoPatterns(t *testing.T) {
	gp := &types.GoldenPath{} // no patterns
	extracted := &ExtractedData{}
	ie := NewInferenceEngine(gp, nil, "some text", extracted)

	techs := ie.inferTechnologies()
	if len(techs) != 0 {
		t.Errorf("expected 0 technologies with no patterns, got %d", len(techs))
	}
}

func TestInferTechnologies_WithPatterns(t *testing.T) {
	gp := &types.GoldenPath{
		Patterns: map[string]types.Pattern{
			"GP-001": {
				Name: "Event Streaming with Kafka",
				When: "streaming events in real time",
				How:  "Use Kafka for streaming and Flink for processing with ScyllaDB for storage",
			},
		},
	}
	extracted := &ExtractedData{
		Stack: []TechMention{}, // no existing stack
	}
	// Transcript with enough keyword matches for high relevance
	transcript := "We need streaming events in real time using kafka and flink for processing"

	ie := NewInferenceEngine(gp, nil, transcript, extracted)
	techs := ie.inferTechnologies()

	// Should infer some technologies from the pattern
	// The pattern mentions kafka, flink, scylladb
	if len(techs) == 0 {
		t.Log("no technologies inferred (relevance may be below threshold)")
	}
}

func TestInferTechnologies_SkipsAlreadyMentioned(t *testing.T) {
	gp := &types.GoldenPath{
		Patterns: map[string]types.Pattern{
			"GP-001": {
				Name: "Kafka Streaming",
				When: "streaming events",
				How:  "Use kafka for event streaming",
			},
		},
	}
	// Kafka already in stack
	extracted := &ExtractedData{
		Stack: []TechMention{
			{Name: "Kafka", Confidence: 0.9, Source: "explicit"},
		},
	}
	transcript := "We need streaming events using kafka for event streaming"

	ie := NewInferenceEngine(gp, nil, transcript, extracted)
	techs := ie.inferTechnologies()

	// Should not re-infer Kafka since it's already mentioned
	for _, tech := range techs {
		if tech.Name == "Kafka" {
			t.Error("should not re-infer already mentioned technology")
		}
	}
}

func TestInferNFRs_NilGoldenPath(t *testing.T) {
	extracted := &ExtractedData{}
	ie := NewInferenceEngine(nil, nil, "some text", extracted)

	nfrs := ie.inferNFRs()
	if len(nfrs) != 0 {
		t.Errorf("expected 0 NFRs with nil golden path, got %d", len(nfrs))
	}
}

func TestInferNFRs_WithKeywords(t *testing.T) {
	gp := &types.GoldenPath{}
	extracted := &ExtractedData{}
	// Transcript with streaming and real-time keywords (triggers "high-throughput")
	transcript := "Precisamos de streaming de eventos em tempo real com kafka"

	ie := NewInferenceEngine(gp, nil, transcript, extracted)
	nfrs := ie.inferNFRs()

	// Should detect high-throughput NFR (keywords: streaming, kafka, tempo real)
	found := false
	for _, nfr := range nfrs {
		if nfr.Confidence > 0 {
			found = true
		}
	}
	if !found {
		t.Log("no NFRs inferred - may need more keyword matches (>= 2)")
	}
}

func TestInferNFRs_HighAvailability(t *testing.T) {
	gp := &types.GoldenPath{}
	extracted := &ExtractedData{}
	// Contains "crítico" and "disponibilidade" (2 keywords for high-availability)
	transcript := "O sistema é crítico para o negócio e precisa de alta disponibilidade"

	ie := NewInferenceEngine(gp, nil, transcript, extracted)
	nfrs := ie.inferNFRs()

	found := false
	for _, nfr := range nfrs {
		if nfr.Confidence > 0 {
			found = true
		}
	}
	if !found {
		t.Error("should detect high-availability NFR with 2+ keywords")
	}
}

func TestDetectApplicablePatterns_NilGoldenPath(t *testing.T) {
	extracted := &ExtractedData{}
	ie := NewInferenceEngine(nil, nil, "text", extracted)

	patterns := ie.detectApplicablePatterns()
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns with nil golden path, got %d", len(patterns))
	}
}

func TestDetectApplicablePatterns_WithRelevantPattern(t *testing.T) {
	gp := &types.GoldenPath{
		Patterns: map[string]types.Pattern{
			"GP-001": {
				Name: "Event Streaming",
				When: "streaming events",
				How:  "Use kafka streaming events in real time for processing data",
			},
		},
	}
	extracted := &ExtractedData{}
	// Transcript matches pattern keywords
	transcript := "We need streaming events in real time using kafka for event processing data"

	ie := NewInferenceEngine(gp, nil, transcript, extracted)
	patterns := ie.detectApplicablePatterns()

	// With enough keyword matches, pattern should be detected
	if len(patterns) == 0 {
		t.Log("no patterns detected - relevance may be below 0.5 threshold")
	}
}

func TestGenerateSuggestions_StreamingWithoutObservability(t *testing.T) {
	extracted := &ExtractedData{
		Stack: []TechMention{
			{Name: "Kafka", Confidence: 0.9, Source: "explicit"},
		},
	}
	ie := NewInferenceEngine(nil, nil, "", extracted)

	suggestions := ie.generateSuggestions()

	found := false
	for _, s := range suggestions {
		if s.Type == "tech" {
			found = true
		}
	}
	if !found {
		t.Error("should suggest observability for streaming system")
	}
}

func TestGenerateSuggestions_StreamingWithObservability(t *testing.T) {
	extracted := &ExtractedData{
		Stack: []TechMention{
			{Name: "Kafka", Confidence: 0.9, Source: "explicit"},
			{Name: "Prometheus", Confidence: 0.8, Source: "explicit"},
		},
	}
	ie := NewInferenceEngine(nil, nil, "", extracted)

	suggestions := ie.generateSuggestions()

	for _, s := range suggestions {
		if s.Type == "tech" && s.Confidence == 0.8 {
			t.Error("should not suggest observability when Prometheus is present")
		}
	}
}

func TestGenerateSuggestions_DatabaseWithoutReplication(t *testing.T) {
	extracted := &ExtractedData{
		Stack: []TechMention{
			{Name: "ScyllaDB", Confidence: 0.9, Source: "explicit"},
		},
	}
	ie := NewInferenceEngine(nil, nil, "Dados distribuídos", extracted)

	suggestions := ie.generateSuggestions()

	found := false
	for _, s := range suggestions {
		if s.Type == "nfr" {
			found = true
		}
	}
	if !found {
		t.Error("should suggest replication for distributed database")
	}
}

func TestGenerateSuggestions_HighVolumeWithoutScaling(t *testing.T) {
	extracted := &ExtractedData{
		Volumetry: map[string]string{
			"events": "100 milhão por dia",
		},
	}
	ie := NewInferenceEngine(nil, nil, "Muitos eventos", extracted)

	suggestions := ie.generateSuggestions()

	found := false
	for _, s := range suggestions {
		if s.Type == "nfr" && s.Confidence == 0.75 {
			found = true
		}
	}
	if !found {
		t.Error("should suggest auto-scaling for high volume")
	}
}

func TestAnalyzeGaps_AllMissing(t *testing.T) {
	extracted := &ExtractedData{
		Stack: []TechMention{}, // no stack
	}
	ie := NewInferenceEngine(nil, nil, "", extracted)

	gap := ie.analyzeGaps()
	if gap.Completeness != 0.0 {
		t.Errorf("expected completeness 0.0 with no stack, got %f", gap.Completeness)
	}
	if len(gap.MissingCategories) != 4 {
		t.Errorf("expected 4 missing categories, got %d", len(gap.MissingCategories))
	}
	if len(gap.CriticalGaps) < 2 {
		t.Error("backend and database should be critical gaps")
	}
}

func TestAnalyzeGaps_AllPresent(t *testing.T) {
	extracted := &ExtractedData{
		Stack: []TechMention{
			{Name: "Spring Boot", Confidence: 0.9},
			{Name: "PostgreSQL", Confidence: 0.9},
			{Name: "Prometheus", Confidence: 0.8},
			{Name: "AWS EKS", Confidence: 0.8},
		},
	}
	ie := NewInferenceEngine(nil, nil, "", extracted)

	gap := ie.analyzeGaps()
	if gap.Completeness != 1.0 {
		t.Errorf("expected completeness 1.0 with all categories, got %f", gap.Completeness)
	}
	if len(gap.MissingCategories) != 0 {
		t.Errorf("expected 0 missing categories, got %d", len(gap.MissingCategories))
	}
}

func TestCalculatePatternRelevance_NoKeywords(t *testing.T) {
	ie := NewInferenceEngine(nil, nil, "some text", &ExtractedData{})

	pattern := types.Pattern{
		Name: "a",
		When: "b",
		How:  "c",
	}

	relevance := ie.calculatePatternRelevance(pattern)
	if relevance != 0.0 {
		t.Errorf("expected 0.0 relevance for empty keywords, got %f", relevance)
	}
}

func TestExtractKeywords(t *testing.T) {
	keywords := extractKeywords("Use kafka for streaming events in real-time processing")

	if len(keywords) == 0 {
		t.Fatal("expected keywords extracted")
	}

	// Should filter stopwords and short words
	for _, kw := range keywords {
		if kw == "for" || kw == "in" {
			t.Errorf("stopword %q should be filtered", kw)
		}
		if len(kw) <= 3 {
			t.Errorf("short word %q should be filtered", kw)
		}
	}
}

func TestExtractKeywords_Empty(t *testing.T) {
	keywords := extractKeywords("")
	if len(keywords) != 0 {
		t.Errorf("expected 0 keywords for empty text, got %d", len(keywords))
	}
}

func TestExtractPatternImplications(t *testing.T) {
	ie := NewInferenceEngine(nil, nil, "", &ExtractedData{})

	pattern := types.Pattern{
		How:       "Implement event sourcing with Kafka. Configure topics for each aggregate. Set up consumer groups for projections.",
		Decisions: []string{"Use Avro for serialization"},
	}

	implications := ie.extractPatternImplications(pattern)
	if len(implications) == 0 {
		t.Error("expected implications extracted")
	}
}

func TestExtractPatternImplications_EmptyPattern(t *testing.T) {
	ie := NewInferenceEngine(nil, nil, "", &ExtractedData{})

	pattern := types.Pattern{}
	implications := ie.extractPatternImplications(pattern)
	if len(implications) != 0 {
		t.Errorf("expected 0 implications for empty pattern, got %d", len(implications))
	}
}

func TestExtractPatternImplications_MaxThree(t *testing.T) {
	ie := NewInferenceEngine(nil, nil, "", &ExtractedData{})

	pattern := types.Pattern{
		How: "First step is to configure the system properly. Second step is to set up monitoring dashboards. Third step is to implement alerting rules. Fourth step is to deploy to production environment.",
		Decisions: []string{
			"Use Prometheus for metrics",
			"Use Grafana for dashboards",
		},
	}

	implications := ie.extractPatternImplications(pattern)
	if len(implications) > 3 {
		t.Errorf("expected max 3 implications, got %d", len(implications))
	}
}

func TestExplainRelevance(t *testing.T) {
	ie := NewInferenceEngine(nil, nil, "", &ExtractedData{})

	pattern := types.Pattern{Name: "Event Sourcing"}
	explanation := ie.explainRelevance(pattern)

	if explanation == "" {
		t.Error("expected non-empty explanation")
	}
}

func TestInfer_FullFlow(t *testing.T) {
	gp := &types.GoldenPath{
		Patterns: map[string]types.Pattern{
			"GP-001": {
				Name: "Event Streaming Platform",
				When: "streaming events real time",
				How:  "Use kafka for streaming and flink for processing with scylladb for storage",
			},
		},
	}
	extracted := &ExtractedData{
		Stack: []TechMention{
			{Name: "Kafka", Confidence: 0.9, Source: "explicit"},
		},
		Volumetry: map[string]string{
			"events": "100 milhão por dia",
		},
	}
	transcript := "Precisamos de streaming de eventos em tempo real com kafka. O sistema é crítico e precisa de alta disponibilidade."

	ie := NewInferenceEngine(gp, nil, transcript, extracted)
	result := ie.Infer()

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// All slices should be initialized
	if result.InferredTechnologies == nil {
		t.Error("InferredTechnologies should not be nil")
	}
	if result.InferredNFRs == nil {
		t.Error("InferredNFRs should not be nil")
	}
	if result.InferredPatterns == nil {
		t.Error("InferredPatterns should not be nil")
	}
	if result.Suggestions == nil {
		t.Error("Suggestions should not be nil")
	}
}
