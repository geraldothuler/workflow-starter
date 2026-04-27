package extractor

import (
	"testing"
)

func TestNewConfidenceScorer(t *testing.T) {
	extracted := &ExtractedData{}
	cs := NewConfidenceScorer("transcript", extracted)
	if cs == nil {
		t.Fatal("expected non-nil scorer")
	}
	if cs.transcript != "transcript" {
		t.Error("transcript not set")
	}
}

func TestScore_FullData(t *testing.T) {
	extracted := &ExtractedData{
		Context:    "Sistema de gestão de frota IoT para monitoramento de veículos em tempo real com objetivo de reduzir custos operacionais da plataforma para o cliente",
		Problem:    "Atualmente a empresa enfrenta dificuldade em rastrear veículos e o problema principal é a falta de dados em tempo real para tomada de decisão",
		Objectives: []string{"Reduzir tempo de 30min para 5s", "Aumentar cobertura para 95%", "Cortar custos em 40%"},
		Volumetry: map[string]string{
			"devices":      "500k",
			"transactions": "10M/day",
		},
		Stack: []TechMention{
			{Name: "Go", Confidence: 0.9, Source: "explicit"},
			{Name: "Kafka", Confidence: 0.9, Source: "explicit"},
			{Name: "ScyllaDB", Confidence: 0.9, Source: "explicit"},
			{Name: "Kubernetes", Confidence: 0.8, Source: "explicit"},
			{Name: "Prometheus", Confidence: 0.8, Source: "explicit"},
			{Name: "Redis", Confidence: 0.7, Source: "inferred"},
		},
		NFRs: []string{
			"Latência P99 < 200ms",
			"Uptime 99.9% SLA",
			"Security compliance LGPD",
		},
	}

	cs := NewConfidenceScorer("", extracted)
	result := cs.Score()

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.OverallScore <= 0 {
		t.Error("expected positive overall score for full data")
	}
	if len(result.SectionScores) != 6 {
		t.Errorf("expected 6 section scores, got %d", len(result.SectionScores))
	}
}

func TestScore_EmptyData(t *testing.T) {
	extracted := &ExtractedData{
		Volumetry: map[string]string{},
	}

	cs := NewConfidenceScorer("", extracted)
	result := cs.Score()

	if result.OverallScore >= 0.7 {
		t.Errorf("expected low overall score for empty data, got %f", result.OverallScore)
	}
}

func TestScoreContext_Detailed(t *testing.T) {
	extracted := &ExtractedData{
		Context: "Sistema de gestão de frota IoT para monitoramento de veículos. O objetivo principal é permitir rastreamento em tempo real. Os clientes incluem empresas de transporte e logística que precisam otimizar operações.",
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreContext()
	if score.Score < 0.5 {
		t.Errorf("detailed context should score > 0.5, got %f", score.Score)
	}
	if len(score.Strengths) == 0 {
		t.Error("detailed context should have strengths")
	}
	if score.Level == "" {
		t.Error("level should not be empty")
	}
}

func TestScoreContext_Short(t *testing.T) {
	extracted := &ExtractedData{
		Context: "Um app",
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreContext()
	if score.Score >= 0.5 {
		t.Errorf("short context should score < 0.5, got %f", score.Score)
	}
	if len(score.Issues) == 0 {
		t.Error("short context should have issues")
	}
}

func TestScoreProblem_Detailed(t *testing.T) {
	extracted := &ExtractedData{
		Problem: "Atualmente a empresa enfrenta dificuldade em rastrear veículos e a dor principal é a falta de dados para decisão rápida",
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreProblem()
	if score.Score < 0.6 {
		t.Errorf("detailed problem should score >= 0.6, got %f", score.Score)
	}
}

func TestScoreProblem_Vague(t *testing.T) {
	extracted := &ExtractedData{
		Problem: "Precisa melhorar",
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreProblem()
	if score.Score >= 0.5 {
		t.Errorf("vague problem should score < 0.5, got %f", score.Score)
	}
}

func TestScoreObjectives_WithMeasurable(t *testing.T) {
	extracted := &ExtractedData{
		Objectives: []string{
			"Reduzir latência para 200ms",
			"Atingir 99.9% uptime",
			"Suportar 500k dispositivos",
		},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreObjectives()
	if score.Score < 0.7 {
		t.Errorf("measurable objectives should score >= 0.7, got %f", score.Score)
	}
}

func TestScoreObjectives_Empty(t *testing.T) {
	extracted := &ExtractedData{
		Objectives: []string{},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreObjectives()
	if score.Score >= 0.5 {
		t.Errorf("empty objectives should score < 0.5, got %f", score.Score)
	}
}

func TestScoreVolumetry_WithExplicitMetrics(t *testing.T) {
	extracted := &ExtractedData{
		Volumetry: map[string]string{
			"users":        "500k",
			"transactions": "10M/day",
			"devices":      "100k",
		},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreVolumetry()
	if score.Score < 0.5 {
		t.Errorf("explicit volumetry should score >= 0.5, got %f", score.Score)
	}
}

func TestScoreVolumetry_Empty(t *testing.T) {
	extracted := &ExtractedData{
		Volumetry: map[string]string{},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreVolumetry()
	if score.Score >= 0.5 {
		t.Errorf("empty volumetry should score < 0.5, got %f", score.Score)
	}
}

func TestScoreVolumetry_MostlyInferred(t *testing.T) {
	extracted := &ExtractedData{
		Volumetry: map[string]string{
			"users":  "estimado 500k",
			"events": "inferido 1M/day",
		},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreVolumetry()
	if len(score.Issues) == 0 {
		t.Error("mostly inferred volumetry should have issues")
	}
}

func TestScoreStack_Complete(t *testing.T) {
	extracted := &ExtractedData{
		Stack: []TechMention{
			{Name: "Spring Boot", Confidence: 0.9, Source: "explicit"},
			{Name: "PostgreSQL", Confidence: 0.9, Source: "explicit"},
			{Name: "Kafka", Confidence: 0.9, Source: "explicit"},
			{Name: "Prometheus", Confidence: 0.8, Source: "explicit"},
			{Name: "AWS", Confidence: 0.8, Source: "explicit"},
			{Name: "Redis", Confidence: 0.7, Source: "inferred"},
		},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreStack()
	if score.Score < 0.6 {
		t.Errorf("complete stack should score >= 0.6, got %f", score.Score)
	}
}

func TestScoreStack_Empty(t *testing.T) {
	extracted := &ExtractedData{
		Stack: []TechMention{},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreStack()
	if score.Score >= 0.5 {
		t.Errorf("empty stack should score < 0.5, got %f", score.Score)
	}
}

func TestScoreNFRs_Complete(t *testing.T) {
	extracted := &ExtractedData{
		NFRs: []string{
			"Latência P99 < 200ms",
			"Uptime 99.9% disponibilidade SLA",
			"Segurança: compliance LGPD",
		},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreNFRs()
	if score.Score < 0.6 {
		t.Errorf("complete NFRs should score >= 0.6, got %f", score.Score)
	}
}

func TestScoreNFRs_Empty(t *testing.T) {
	extracted := &ExtractedData{
		NFRs: []string{},
	}
	cs := NewConfidenceScorer("", extracted)

	score := cs.scoreNFRs()
	if score.Score >= 0.5 {
		t.Errorf("empty NFRs should score < 0.5, got %f", score.Score)
	}
}

func TestPerformFactChecks_AllGood(t *testing.T) {
	extracted := &ExtractedData{
		Volumetry: map[string]string{"users": "500k"},
		Stack:     []TechMention{{Name: "Go"}},
		NFRs:      []string{"99% uptime"},
	}
	cs := NewConfidenceScorer("", extracted)

	checks := cs.performFactChecks()
	if len(checks) == 0 {
		t.Fatal("expected at least one check")
	}
	if !checks[0].Passed {
		t.Error("expected consistency check to pass")
	}
}

func TestPerformFactChecks_VolumetryWithoutStack(t *testing.T) {
	extracted := &ExtractedData{
		Volumetry: map[string]string{"users": "500k"},
		Stack:     []TechMention{}, // empty
	}
	cs := NewConfidenceScorer("", extracted)

	checks := cs.performFactChecks()
	found := false
	for _, check := range checks {
		if check.Type == "completeness" && !check.Passed {
			found = true
		}
	}
	if !found {
		t.Error("should flag volumetry without stack")
	}
}

func TestPerformFactChecks_PerformanceNFRWithoutStack(t *testing.T) {
	extracted := &ExtractedData{
		NFRs:  []string{"Latência P99 < 100ms"},
		Stack: []TechMention{},
	}
	cs := NewConfidenceScorer("", extracted)

	checks := cs.performFactChecks()
	found := false
	for _, check := range checks {
		if check.Type == "consistency" && !check.Passed {
			found = true
		}
	}
	if !found {
		t.Error("should flag performance NFR without stack")
	}
}

func TestPerformFactChecks_UnrealisticVolumetry(t *testing.T) {
	extracted := &ExtractedData{
		Volumetry: map[string]string{"users": "1000000000 per second"},
		Stack:     []TechMention{{Name: "Go"}},
	}
	cs := NewConfidenceScorer("", extracted)

	checks := cs.performFactChecks()
	found := false
	for _, check := range checks {
		if check.Type == "realism" && !check.Passed {
			found = true
		}
	}
	if !found {
		t.Error("should flag unrealistic volumetry")
	}
}

func TestGenerateRecommendations_LowScores(t *testing.T) {
	result := &ScoringResult{
		OverallScore: 0.5,
		SectionScores: map[string]SectionScore{
			"context": {Score: 0.3},
			"stack":   {Score: 0.8},
		},
		FactChecks: []FactCheck{
			{Passed: false, Suggestion: "Add more details"},
		},
	}

	cs := NewConfidenceScorer("", &ExtractedData{})
	recs := cs.generateRecommendations(result)

	if len(recs) == 0 {
		t.Error("expected recommendations for low scores")
	}
}

func TestGenerateRecommendations_HighOverall(t *testing.T) {
	result := &ScoringResult{
		OverallScore: 0.85,
		SectionScores: map[string]SectionScore{
			"context": {Score: 0.8},
			"stack":   {Score: 0.9},
		},
		FactChecks: []FactCheck{
			{Passed: true},
		},
	}

	cs := NewConfidenceScorer("", &ExtractedData{})
	recs := cs.generateRecommendations(result)

	// Should not have the "overall < 70%" recommendation
	for _, rec := range recs {
		if rec == "📋 Confiança geral < 70% - recomendado validar com stakeholders antes de prosseguir" {
			t.Error("should not recommend stakeholder validation for high overall score")
		}
	}
}

func TestScoreLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.9, "high"},
		{0.8, "high"},
		{0.7, "medium"},
		{0.6, "medium"},
		{0.5, "low"},
		{0.0, "low"},
	}

	for _, tt := range tests {
		result := scoreLevel(tt.score)
		if result != tt.expected {
			t.Errorf("scoreLevel(%f) = %q, want %q", tt.score, result, tt.expected)
		}
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		value, min, max, expected float64
	}{
		{0.5, 0.0, 1.0, 0.5},
		{-0.5, 0.0, 1.0, 0.0},
		{1.5, 0.0, 1.0, 1.0},
		{0.0, 0.0, 1.0, 0.0},
		{1.0, 0.0, 1.0, 1.0},
	}

	for _, tt := range tests {
		result := clamp(tt.value, tt.min, tt.max)
		if result != tt.expected {
			t.Errorf("clamp(%f, %f, %f) = %f, want %f", tt.value, tt.min, tt.max, result, tt.expected)
		}
	}
}

func TestContainsAny_Scoring(t *testing.T) {
	if !containsAny("hello world", []string{"world"}) {
		t.Error("should find 'world'")
	}
	if containsAny("hello", []string{"world"}) {
		t.Error("should not find 'world'")
	}
	if !containsAny("Hello World", []string{"hello"}) {
		t.Error("should be case insensitive")
	}
}

func TestContainsNumbers_Scoring(t *testing.T) {
	if !containsNumbers("abc123") {
		t.Error("should find numbers")
	}
	if containsNumbers("abc") {
		t.Error("should not find numbers in text-only")
	}
}

func TestHasKey(t *testing.T) {
	m := map[string]string{
		"users":  "500k",
		"events": "1M",
	}

	if !hasKey(m, []string{"users"}) {
		t.Error("should find 'users' key")
	}
	if hasKey(m, []string{"missing"}) {
		t.Error("should not find 'missing' key")
	}
}

func TestCategorizeStack(t *testing.T) {
	cs := NewConfidenceScorer("", &ExtractedData{})

	stack := []TechMention{
		{Name: "Spring Boot"},
		{Name: "PostgreSQL"},
		{Name: "Kafka"},
		{Name: "Prometheus"},
		{Name: "AWS"},
	}

	categories := cs.categorizeStack(stack)
	if !categories["backend"] {
		t.Error("should detect backend")
	}
	if !categories["database"] {
		t.Error("should detect database")
	}
	if !categories["streaming"] {
		t.Error("should detect streaming")
	}
	if !categories["observability"] {
		t.Error("should detect observability")
	}
	if !categories["cloud"] {
		t.Error("should detect cloud")
	}
}

func TestCategorizeStack_Empty(t *testing.T) {
	cs := NewConfidenceScorer("", &ExtractedData{})

	categories := cs.categorizeStack([]TechMention{})
	if len(categories) != 0 {
		t.Errorf("expected 0 categories for empty stack, got %d", len(categories))
	}
}
