package patterns_catalog

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// MOCK LLM
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type mockCompleter struct {
	response string
	err      error
}

func (m *mockCompleter) Complete(prompt string, maxTokens int) (string, error) {
	return m.response, m.err
}

func (m *mockCompleter) CompleteWithUsage(prompt string, maxTokens int) (string, *llm.Usage, error) {
	return m.response, nil, m.err
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PARSE RESPONSE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestParseSuggestionResponse_ValidJSON(t *testing.T) {
	input := `[
		{
			"pattern_id": "cqrs",
			"confidence": 0.85,
			"reasoning": "Separate read and write models needed",
			"affected_epics": ["Dashboard", "Orders"]
		},
		{
			"pattern_id": "distributed-monolith",
			"confidence": 0.7,
			"reasoning": "Services appear tightly coupled",
			"affected_epics": ["Core"]
		}
	]`

	suggestions, err := parseSuggestionResponse(input)
	if err != nil {
		t.Fatalf("parseSuggestionResponse() error: %v", err)
	}

	if len(suggestions) != 2 {
		t.Fatalf("got %d suggestions, want 2", len(suggestions))
	}

	if suggestions[0].PatternID != "cqrs" {
		t.Errorf("suggestions[0].PatternID = %q, want cqrs", suggestions[0].PatternID)
	}
	if suggestions[0].Confidence != 0.85 {
		t.Errorf("suggestions[0].Confidence = %f, want 0.85", suggestions[0].Confidence)
	}
}

func TestParseSuggestionResponse_JSONWithSurroundingText(t *testing.T) {
	input := `Here are my suggestions:
[{"pattern_id": "saga", "confidence": 0.8, "reasoning": "Distributed transactions", "affected_epics": ["Payments"]}]
Hope this helps!`

	suggestions, err := parseSuggestionResponse(input)
	if err != nil {
		t.Fatalf("parseSuggestionResponse() error: %v", err)
	}

	if len(suggestions) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(suggestions))
	}
	if suggestions[0].PatternID != "saga" {
		t.Errorf("PatternID = %q, want saga", suggestions[0].PatternID)
	}
}

func TestParseSuggestionResponse_EmptyArray(t *testing.T) {
	suggestions, err := parseSuggestionResponse("[]")
	if err != nil {
		t.Fatalf("parseSuggestionResponse() error: %v", err)
	}
	if len(suggestions) != 0 {
		t.Errorf("got %d suggestions, want 0", len(suggestions))
	}
}

func TestParseSuggestionResponse_NoJSON(t *testing.T) {
	_, err := parseSuggestionResponse("No patterns found for this backlog.")
	if err == nil {
		t.Error("expected error for no JSON, got nil")
	}
}

func TestParseSuggestionResponse_InvalidJSON(t *testing.T) {
	_, err := parseSuggestionResponse("[{invalid json}]")
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// VALIDATE AGAINST CATALOG
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestValidateAgainstCatalog_DropsHallucinated(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	raw := []rawSuggestion{
		{PatternID: "cqrs", Confidence: 0.8, Reasoning: "Valid"},
		{PatternID: "hallucinated-pattern", Confidence: 0.9, Reasoning: "Fake"},
		{PatternID: "saga", Confidence: 0.7, Reasoning: "Also valid"},
	}

	result := validateAgainstCatalog(raw, pc)
	if len(result) != 2 {
		t.Fatalf("got %d results, want 2 (hallucinated should be dropped)", len(result))
	}
	if result[0].PatternID != "cqrs" {
		t.Errorf("result[0].PatternID = %q, want cqrs", result[0].PatternID)
	}
}

func TestValidateAgainstCatalog_EnrichesMetadata(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	raw := []rawSuggestion{
		{PatternID: "cqrs", Confidence: 0.8, Reasoning: "CQRS fits", AffectedEpics: []string{"Dashboard"}},
	}

	result := validateAgainstCatalog(raw, pc)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}

	s := result[0]
	if s.PatternName != "CQRS" {
		t.Errorf("PatternName = %q, want CQRS", s.PatternName)
	}
	if s.Type != "pattern" {
		t.Errorf("Type = %q, want pattern", s.Type)
	}
	if s.Category != "data" {
		t.Errorf("Category = %q, want data", s.Category)
	}
	if s.Source != "Microservices Patterns" {
		t.Errorf("Source = %q, want Microservices Patterns", s.Source)
	}
	if s.Level != "universal" {
		t.Errorf("Level = %q, want universal", s.Level)
	}
}

func TestValidateAgainstCatalog_EnrichesAntiPatternRemediation(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	raw := []rawSuggestion{
		{PatternID: "distributed-monolith", Confidence: 0.75, Reasoning: "Tight coupling detected"},
	}

	result := validateAgainstCatalog(raw, pc)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}

	s := result[0]
	if s.Type != "anti-pattern" {
		t.Errorf("Type = %q, want anti-pattern", s.Type)
	}
	if len(s.Remediation) == 0 {
		t.Error("Remediation should be enriched from catalog")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FILTERING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestFilterSuggestions_ConfidenceThreshold(t *testing.T) {
	suggestions := []types.PatternSuggestion{
		{PatternID: "cqrs", Confidence: 0.9},
		{PatternID: "saga", Confidence: 0.3},   // Below threshold
		{PatternID: "facade", Confidence: 0.6},
	}

	config := SuggestionConfig{MinConfidence: 0.5, MaxPatterns: 10}
	filtered := filterSuggestions(suggestions, config)

	if len(filtered) != 2 {
		t.Fatalf("got %d filtered, want 2", len(filtered))
	}

	// Should be sorted by confidence desc
	if filtered[0].PatternID != "cqrs" {
		t.Errorf("filtered[0] = %q, want cqrs (highest confidence)", filtered[0].PatternID)
	}
}

func TestFilterSuggestions_MaxLimit(t *testing.T) {
	suggestions := []types.PatternSuggestion{
		{PatternID: "a", Confidence: 0.9},
		{PatternID: "b", Confidence: 0.8},
		{PatternID: "c", Confidence: 0.7},
		{PatternID: "d", Confidence: 0.6},
	}

	config := SuggestionConfig{MinConfidence: 0.0, MaxPatterns: 2}
	filtered := filterSuggestions(suggestions, config)

	if len(filtered) != 2 {
		t.Fatalf("got %d filtered, want 2 (max limit)", len(filtered))
	}
}

func TestClampConfidence(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{0.5, 0.5},
		{-0.1, 0.0},
		{1.5, 1.0},
		{0.0, 0.0},
		{1.0, 1.0},
	}

	for _, tt := range tests {
		got := clampConfidence(tt.input)
		if got != tt.want {
			t.Errorf("clampConfidence(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// END-TO-END (MOCK LLM)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestSuggestPatterns_EndToEnd(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				Title: "Order Processing",
				Stories: []types.Story{
					{Title: "Create order API"},
					{Title: "Payment processing"},
				},
			},
			{
				Title: "Dashboard Analytics",
				Stories: []types.Story{
					{Title: "Real-time metrics"},
					{Title: "Historical reports"},
				},
			},
		},
	}

	// Mock LLM response
	mockResponse := `[
		{"pattern_id": "cqrs", "confidence": 0.85, "reasoning": "Separate read/write models", "affected_epics": ["Dashboard Analytics"]},
		{"pattern_id": "circuit-breaker", "confidence": 0.7, "reasoning": "Payment API needs resilience", "affected_epics": ["Order Processing"]},
		{"pattern_id": "hallucinated-id", "confidence": 0.9, "reasoning": "Should be dropped", "affected_epics": []},
		{"pattern_id": "facade", "confidence": 0.3, "reasoning": "Below threshold", "affected_epics": []}
	]`

	mock := &mockCompleter{response: mockResponse}
	config := DefaultSuggestionConfig() // 0.5 threshold, 10 max

	suggestions, err := SuggestPatterns(pc, backlog, mock, config)
	if err != nil {
		t.Fatalf("SuggestPatterns() error: %v", err)
	}

	// Should have 2: cqrs (0.85) + circuit-breaker (0.7)
	// facade (0.3) is below threshold, hallucinated-id dropped
	if len(suggestions) != 2 {
		t.Fatalf("got %d suggestions, want 2", len(suggestions))
	}

	// Sorted by confidence desc
	if suggestions[0].PatternID != "cqrs" {
		t.Errorf("suggestions[0].PatternID = %q, want cqrs", suggestions[0].PatternID)
	}
	if suggestions[1].PatternID != "circuit-breaker" {
		t.Errorf("suggestions[1].PatternID = %q, want circuit-breaker", suggestions[1].PatternID)
	}

	// Verify metadata enrichment
	if suggestions[0].PatternName != "CQRS" {
		t.Errorf("PatternName = %q, want CQRS", suggestions[0].PatternName)
	}
	if suggestions[0].Source != "Microservices Patterns" {
		t.Errorf("Source = %q, want Microservices Patterns", suggestions[0].Source)
	}
}

func TestSuggestPatterns_LLMError(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockCompleter{err: fmt.Errorf("API rate limit exceeded")}
	config := DefaultSuggestionConfig()

	_, err = SuggestPatterns(pc, types.Backlog{}, mock, config)
	if err == nil {
		t.Error("expected error from LLM failure, got nil")
	}
	if !strings.Contains(err.Error(), "LLM call failed") {
		t.Errorf("error = %q, want to contain 'LLM call failed'", err.Error())
	}
}

func TestSuggestPatterns_EmptyResponse(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockCompleter{response: "[]"}
	config := DefaultSuggestionConfig()

	suggestions, err := SuggestPatterns(pc, types.Backlog{}, mock, config)
	if err != nil {
		t.Fatalf("SuggestPatterns() error: %v", err)
	}
	if len(suggestions) != 0 {
		t.Errorf("got %d suggestions, want 0", len(suggestions))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PROMPT CONSTRUCTION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBuildSuggestionPrompt_ContainsSOLID(t *testing.T) {
	prompt := BuildSuggestionPrompt("summary", "catalog", DefaultSuggestionConfig())

	solidPrinciples := []string{"SRP", "OCP", "LSP", "ISP", "DIP"}
	for _, p := range solidPrinciples {
		if !strings.Contains(prompt, p) {
			t.Errorf("prompt missing SOLID principle: %s", p)
		}
	}
}

func TestBuildSuggestionPrompt_ContainsConfig(t *testing.T) {
	config := SuggestionConfig{MinConfidence: 0.7, MaxPatterns: 5}
	prompt := BuildSuggestionPrompt("summary", "catalog", config)

	if !strings.Contains(prompt, "0.7") {
		t.Error("prompt missing min confidence")
	}
	if !strings.Contains(prompt, "5") {
		t.Error("prompt missing max patterns")
	}
}

func TestBuildBacklogSummary_WithEpics(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				Title:       "User Management",
				Description: "Handle user registration and auth",
				Stories: []types.Story{
					{Title: "Login API", Effort: 3},
					{Title: "Register API", Effort: 5},
				},
			},
		},
		DeepDives: []types.DeepDive{
			{Term: "PostgreSQL"},
			{Term: "Redis"},
		},
	}

	summary := BuildBacklogSummary(backlog)

	if !strings.Contains(summary, "User Management") {
		t.Error("summary missing epic title")
	}
	if !strings.Contains(summary, "Login API") {
		t.Error("summary missing story title")
	}
	if !strings.Contains(summary, "PostgreSQL") {
		t.Error("summary missing technology")
	}
}

func TestBuildBacklogSummary_EmptyBacklog(t *testing.T) {
	summary := BuildBacklogSummary(types.Backlog{})
	if summary == "" {
		t.Error("summary should not be empty for empty backlog")
	}
	if !strings.Contains(summary, "Epics:** 0") {
		t.Error("summary should show 0 epics")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANTI-PATTERN DETECTION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestSuggestPatterns_AntiPatternDetection(t *testing.T) {
	pc, err := NewPatternCatalog()
	if err != nil {
		t.Fatal(err)
	}

	// Simulate LLM detecting an anti-pattern
	rawResponse, _ := json.Marshal([]rawSuggestion{
		{
			PatternID:     "god-class",
			Confidence:    0.8,
			Reasoning:     "UserService handles auth, billing, and notifications",
			AffectedEpics: []string{"Core Service"},
		},
	})

	mock := &mockCompleter{response: string(rawResponse)}
	config := DefaultSuggestionConfig()

	suggestions, err := SuggestPatterns(pc, types.Backlog{}, mock, config)
	if err != nil {
		t.Fatalf("SuggestPatterns() error: %v", err)
	}

	if len(suggestions) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(suggestions))
	}

	s := suggestions[0]
	if s.Type != "anti-pattern" {
		t.Errorf("Type = %q, want anti-pattern", s.Type)
	}
	if len(s.Remediation) == 0 {
		t.Error("anti-pattern should have remediation patterns")
	}
}
