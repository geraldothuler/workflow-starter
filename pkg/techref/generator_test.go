package techref

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// Mock LLM caller
func mockLLMCaller(prompt string) (string, error) {
	return "Mocked deep dive response", nil
}

func mockLLMCallerError(prompt string) (string, error) {
	return "", errors.New("LLM error")
}

func TestGenerateDeepDivesOptimized_DryRun(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend API",
				Stories: []types.Story{
					{ID: "E1.1", Title: "User API", What: "Criar com Spring Boot e PostgreSQL"},
					{ID: "E1.2", Title: "Tasks API", What: "Criar com Spring Boot e Redis"},
				},
			},
		},
	}

	config := GetDefaultGenerationConfig()
	config.DryRun = true
	config.Verbose = false

	result := GenerateDeepDivesOptimized(backlog, config)

	// Deve ter gerado dives
	if len(result.DeepDives) == 0 {
		t.Error("Expected some deep dives in dry-run")
	}
}

func TestGenerateDeepDivesOptimized_WithLLM(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend",
				Stories: []types.Story{
					{ID: "E1.1", What: "Spring Boot PostgreSQL"},
					{ID: "E1.2", What: "Spring Boot Redis"},
				},
			},
		},
	}

	config := GetDefaultGenerationConfig()
	config.LLMCaller = mockLLMCaller
	config.Verbose = false

	result := GenerateDeepDivesOptimized(backlog, config)

	// Deve ter feito algumas chamadas
	if result.Metrics.LLMCallsMade == 0 {
		t.Error("Expected LLM calls, got 0")
	}

	// Deve ter gerado dives
	if len(result.DeepDives) == 0 {
		t.Error("Expected deep dives")
	}

	// Deve ter economia
	if result.Metrics.ReductionPercent <= 0 {
		t.Error("Expected reduction percentage > 0")
	}
}

func TestGenerationMetrics(t *testing.T) {
	metrics := GenerationMetrics{
		TotalTechsExtracted: 10,
		TrivialFiltered:     5,
		StandardCount:       2,
		SpecificCount:       2,
		CriticalCount:       1,
		LLMCallsMade:        5,
	}

	if metrics.TotalTechsExtracted != 10 {
		t.Error("Wrong total techs")
	}

	if metrics.TrivialFiltered != 5 {
		t.Error("Wrong trivial count")
	}
}

func TestBuildEpicPrompt(t *testing.T) {
	epic := types.Epic{
		ID:    "E1",
		Title: "Backend APIs",
		Stories: []types.Story{
			{ID: "E1.1", Title: "User API", What: "Create user API"},
			{ID: "E1.2", Title: "Tasks API", What: "Create tasks API"},
		},
	}

	classification := TechClassification{
		Term:   "Spring Boot",
		Scope:  "epic",
		EpicID: "E1",
	}

	prompt := buildEpicPrompt(epic, classification)

	if !strings.Contains(prompt, "Spring Boot") {
		t.Error("Prompt should mention Spring Boot")
	}

	if !strings.Contains(prompt, "E1") {
		t.Error("Prompt should mention epic ID")
	}

	if !strings.Contains(prompt, "Backend APIs") {
		t.Error("Prompt should mention epic title")
	}
}

func TestBuildStoryPrompt(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "User API",
		What:  "Create REST API for users",
		Why:   "Need user management",
		AcceptanceCriteria: []string{
			"Must support CRUD operations",
		},
	}

	classification := TechClassification{
		Term:    "Spring Boot",
		Scope:   "story",
		StoryID: "E1.1",
	}

	prompt := buildStoryPrompt(story, classification)

	if !strings.Contains(prompt, "Spring Boot") {
		t.Error("Prompt should mention Spring Boot")
	}

	if !strings.Contains(prompt, "E1.1") {
		t.Error("Prompt should mention story code")
	}

	if !strings.Contains(prompt, "CRUD operations") {
		t.Error("Prompt should mention acceptance criteria")
	}
}

func TestCalculateOldApproachCalls(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				Stories: []types.Story{
					{ID: "E1.1"},
					{ID: "E1.2"},
					{ID: "E1.3"},
				},
			},
		},
	}

	calls := calculateOldApproachCalls(backlog)

	// 3 stories × ~6 techs = ~18 calls
	if calls < 10 || calls > 30 {
		t.Errorf("Expected ~18 calls, got %d", calls)
	}
}

func TestGetDefaultGenerationConfig(t *testing.T) {
	config := GetDefaultGenerationConfig()

	if config.Verbose {
		t.Error("Default should not be verbose")
	}

	if config.DryRun {
		t.Error("Default should not be dry-run")
	}

	if config.LLMCaller != nil {
		t.Error("Default LLMCaller should be nil")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CROSS-EPIC DEDUPLICATION TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCrossEpicDedup_SameTechIn2Epics(t *testing.T) {
	// Spring Boot appears in both epics → should become 1 global DD
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend API",
				Stories: []types.Story{
					{ID: "E1.1", Title: "User API", What: "Criar com Spring Boot e PostgreSQL"},
					{ID: "E1.2", Title: "Tasks API", What: "Criar com Spring Boot e Redis"},
				},
			},
			{
				ID:    "E2",
				Title: "Auth Service",
				Stories: []types.Story{
					{ID: "E2.1", Title: "Login", What: "Implementar login com Spring Boot e JWT"},
				},
			},
		},
	}

	reg := DefaultRegistry()
	globalMap := buildCrossEpicMapCached(reg, backlog, make(map[string][]TechExtraction))

	// Spring Boot should be in global map (appears in E1 and E2)
	if _, ok := globalMap["Spring Boot"]; !ok {
		t.Error("Spring Boot should be in cross-epic global map")
		t.Logf("globalMap keys: %v", func() []string {
			keys := make([]string, 0, len(globalMap))
			for k := range globalMap {
				keys = append(keys, k)
			}
			return keys
		}())
	}

	// PostgreSQL should NOT be in global map (only in E1)
	if _, ok := globalMap["PostgreSQL"]; ok {
		t.Error("PostgreSQL should NOT be in cross-epic map (only in 1 epic)")
	}
}

func TestCrossEpicDedup_SingleEpicUnchanged(t *testing.T) {
	// Single epic → no cross-epic dedup
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend",
				Stories: []types.Story{
					{ID: "E1.1", What: "Spring Boot PostgreSQL"},
					{ID: "E1.2", What: "Spring Boot Redis"},
				},
			},
		},
	}

	reg := DefaultRegistry()
	globalMap := buildCrossEpicMapCached(reg, backlog, make(map[string][]TechExtraction))

	if len(globalMap) != 0 {
		t.Errorf("Single epic should have no cross-epic techs, got %d", len(globalMap))
	}
}

func TestCrossEpicDedup_DryRun_ReducesDuplicates(t *testing.T) {
	// Same tech in 3 epics → should produce only 1 global DD (not 3 epic-level DDs)
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "API",
				Stories: []types.Story{
					{ID: "E1.1", What: "Criar com Spring Boot e PostgreSQL"},
				},
			},
			{
				ID:    "E2",
				Title: "Auth",
				Stories: []types.Story{
					{ID: "E2.1", What: "Login com Spring Boot e JWT"},
				},
			},
			{
				ID:    "E3",
				Title: "Admin",
				Stories: []types.Story{
					{ID: "E3.1", What: "Painel admin com Spring Boot"},
				},
			},
		},
	}

	config := GetDefaultGenerationConfig()
	config.DryRun = true

	result := GenerateDeepDivesOptimized(backlog, config)

	// Count how many Spring Boot deep dives were generated
	springBootCount := 0
	for _, dd := range result.DeepDives {
		if dd.Term == "Spring Boot" {
			springBootCount++
		}
	}

	if springBootCount != 1 {
		t.Errorf("Expected exactly 1 Spring Boot DD (global), got %d", springBootCount)
		for _, dd := range result.DeepDives {
			t.Logf("  DD: %s - %s", dd.Term, dd.WhatIs[:min(60, len(dd.WhatIs))])
		}
	}

	// Should have cross-epic metrics
	if result.Metrics.CrossEpicGlobalDives == 0 {
		t.Error("Expected CrossEpicGlobalDives > 0")
	}

	if result.Metrics.CrossEpicDeduplicated == 0 {
		t.Error("Expected CrossEpicDeduplicated > 0 (Spring Boot skipped in individual epics)")
	}
}

func TestBuildGlobalPrompt(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend",
				Stories: []types.Story{
					{ID: "E1.1", Title: "User API", What: "Create with Spring Boot"},
				},
			},
			{
				ID:    "E2",
				Title: "Auth",
				Stories: []types.Story{
					{ID: "E2.1", Title: "Login", What: "Auth with Spring Boot"},
				},
			},
		},
	}

	crossTech := &crossEpicTech{
		Term:         "Spring Boot",
		EpicIDs:      []string{"E1", "E2"},
		TotalStories: 2,
		BestScope:    "global",
	}

	prompt := buildGlobalPrompt(backlog, crossTech)

	if !strings.Contains(prompt, "Spring Boot") {
		t.Error("Global prompt should mention tech")
	}
	if !strings.Contains(prompt, "GLOBAL") {
		t.Error("Global prompt should mention GLOBAL context")
	}
	if !strings.Contains(prompt, "2 épicos") {
		t.Error("Global prompt should mention number of epics")
	}
	if !strings.Contains(prompt, "E1") || !strings.Contains(prompt, "E2") {
		t.Error("Global prompt should include both epic contexts")
	}
}

func TestCalculateOldApproachCalls_RealExtraction(t *testing.T) {
	// With real content, the calculation should be based on actual extraction
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID: "E1",
				Stories: []types.Story{
					{ID: "E1.1", What: "Criar API com Spring Boot e PostgreSQL usando Docker"},
					{ID: "E1.2", What: "Implementar cache com Redis e monitorar com Kafka"},
				},
			},
		},
	}

	reg := DefaultRegistry()
	calls := calculateOldApproachCallsWithRegistry(reg, backlog)

	// Should be > 0 and based on actual techs (not hardcoded 6)
	if calls == 0 {
		t.Error("Expected > 0 old approach calls")
	}

	// Should be reasonable (not 6*2=12 hardcoded)
	t.Logf("Old approach calls: %d", calls)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CROSS-EPIC EDGE CASES (Fase B)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCrossEpicDedup_EmptyBacklog(t *testing.T) {
	reg := DefaultRegistry()
	backlog := types.Backlog{Epics: []types.Epic{}}
	cache := make(map[string][]TechExtraction)

	globalMap := buildCrossEpicMapCached(reg, backlog, cache)
	if len(globalMap) != 0 {
		t.Errorf("empty backlog should have no cross-epic techs, got %d", len(globalMap))
	}
	if len(cache) != 0 {
		t.Errorf("empty backlog should have empty cache, got %d entries", len(cache))
	}
}

func TestCrossEpicDedup_AllTechsInAllEpics(t *testing.T) {
	// Same tech in ALL epics — should still produce only 1 global DD
	backlog := types.Backlog{
		Epics: []types.Epic{
			{ID: "E1", Stories: []types.Story{{ID: "E1.1", What: "Spring Boot API"}}},
			{ID: "E2", Stories: []types.Story{{ID: "E2.1", What: "Spring Boot service"}}},
			{ID: "E3", Stories: []types.Story{{ID: "E3.1", What: "Spring Boot worker"}}},
			{ID: "E4", Stories: []types.Story{{ID: "E4.1", What: "Spring Boot admin"}}},
		},
	}

	reg := DefaultRegistry()
	cache := make(map[string][]TechExtraction)
	globalMap := buildCrossEpicMapCached(reg, backlog, cache)

	if _, ok := globalMap["Spring Boot"]; !ok {
		t.Fatal("Spring Boot should be in cross-epic map")
	}

	if len(globalMap["Spring Boot"].EpicIDs) != 4 {
		t.Errorf("expected 4 epic IDs, got %d", len(globalMap["Spring Boot"].EpicIDs))
	}

	// Cache should have entries for all 4 epics
	if len(cache) != 4 {
		t.Errorf("expected 4 cache entries, got %d", len(cache))
	}
}

func TestCrossEpicDedup_CachePopulated(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{ID: "E1", Stories: []types.Story{
				{ID: "E1.1", What: "Spring Boot PostgreSQL"},
			}},
			{ID: "E2", Stories: []types.Story{
				{ID: "E2.1", What: "Spring Boot Redis"},
			}},
		},
	}

	reg := DefaultRegistry()
	cache := make(map[string][]TechExtraction)
	_ = buildCrossEpicMapCached(reg, backlog, cache)

	// Cache should have entries for both epics
	if _, ok := cache["E1"]; !ok {
		t.Error("cache should contain E1 extractions")
	}
	if _, ok := cache["E2"]; !ok {
		t.Error("cache should contain E2 extractions")
	}

	// Extractions should contain actual techs
	if len(cache["E1"]) == 0 {
		t.Error("E1 cache should have extractions")
	}
}

func TestCrossEpicDedup_TrivialTermsExcluded(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{ID: "E1", Stories: []types.Story{{ID: "E1.1", What: "HTTP API with Spring Boot"}}},
			{ID: "E2", Stories: []types.Story{{ID: "E2.1", What: "HTTP service with Spring Boot"}}},
		},
	}

	reg := DefaultRegistry()
	globalMap := buildCrossEpicMapCached(reg, backlog, make(map[string][]TechExtraction))

	if _, ok := globalMap["HTTP"]; ok {
		t.Error("trivial term HTTP should NOT be in cross-epic map")
	}
}

func TestCrossEpicDedup_ManyEpics(t *testing.T) {
	epics := make([]types.Epic, 10)
	for i := range epics {
		epics[i] = types.Epic{
			ID: fmt.Sprintf("E%d", i+1),
			Stories: []types.Story{
				{ID: fmt.Sprintf("E%d.1", i+1), What: "Serviço com Spring Boot"},
			},
		}
	}

	backlog := types.Backlog{Epics: epics}
	config := GetDefaultGenerationConfig()
	config.DryRun = true

	result := GenerateDeepDivesOptimized(backlog, config)

	springBootCount := 0
	for _, dd := range result.DeepDives {
		if dd.Term == "Spring Boot" {
			springBootCount++
		}
	}

	if springBootCount != 1 {
		t.Errorf("expected exactly 1 Spring Boot DD across 10 epics, got %d", springBootCount)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ERROR PROPAGATION TESTS (Fase B)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestGenerateDeepDivesOptimized_LLMErrors_Propagated(t *testing.T) {
	// Use tech names that have NO pre-defined template so the LLM path is exercised.
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID: "E1",
				Stories: []types.Story{
					{ID: "E1.1", What: "CockroachDB Pulsar"},
					{ID: "E1.2", What: "CockroachDB Flink"},
				},
			},
		},
	}

	config := GetDefaultGenerationConfig()
	config.LLMCaller = mockLLMCallerError
	config.Verbose = false

	result := GenerateDeepDivesOptimizedWithRegistry(DefaultRegistry(), backlog, config)

	if len(result.Errors) == 0 {
		t.Error("expected errors to be propagated from LLM failures")
	}
}

func TestGenerateDeepDivesOptimized_NilRegistry(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{ID: "E1", Stories: []types.Story{{ID: "E1.1", What: "Spring Boot"}}},
		},
	}

	config := GetDefaultGenerationConfig()
	config.DryRun = true

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil registry caused panic: %v", r)
		}
	}()

	result := GenerateDeepDivesOptimizedWithRegistry(nil, backlog, config)
	if len(result.DeepDives) == 0 {
		t.Error("should work with nil registry (fallback to default)")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// BATCHING TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBatchPromptBuilder(t *testing.T) {
	epic := types.Epic{
		ID:    "E1",
		Title: "Backend APIs",
		Stories: []types.Story{
			{ID: "E1.1", Title: "User API", What: "Create user API"},
			{ID: "E1.2", Title: "Tasks API", What: "Create tasks API"},
		},
	}

	classifications := []TechClassification{
		{Term: "Spring Boot", Relevance: STANDARD, Scope: "epic"},
		{Term: "PostgreSQL", Relevance: STANDARD, Scope: "epic"},
		{Term: "Redis", Relevance: CRITICAL, Scope: "epic"},
	}

	prompt := buildBatchEpicPrompt(epic, classifications)

	// Deve conter todas as techs
	for _, c := range classifications {
		if !strings.Contains(prompt, c.Term) {
			t.Errorf("Batch prompt should mention %s", c.Term)
		}
	}

	// Deve conter contexto do épico
	if !strings.Contains(prompt, "E1") {
		t.Error("Batch prompt should mention epic ID")
	}
	if !strings.Contains(prompt, "Backend APIs") {
		t.Error("Batch prompt should mention epic title")
	}

	// Deve ter instruções de separação
	if !strings.Contains(prompt, "---") {
		t.Error("Batch prompt should mention separator")
	}
}

func TestBatchResponseParser(t *testing.T) {
	response := `### Spring Boot
Spring Boot é um framework Java para microsserviços.
---
### PostgreSQL
PostgreSQL é um banco relacional robusto.
---
### Redis
Redis é um banco in-memory para cache.`

	classifications := []TechClassification{
		{Term: "Spring Boot", Relevance: STANDARD, Scope: "epic"},
		{Term: "PostgreSQL", Relevance: STANDARD, Scope: "epic"},
		{Term: "Redis", Relevance: CRITICAL, Scope: "epic"},
	}

	dives := parseBatchResponse(response, classifications, "E1")

	if len(dives) != 3 {
		t.Fatalf("Expected 3 deep dives, got %d", len(dives))
	}

	// Cada dive deve ter Term e WhatIs preenchidos
	for i, d := range dives {
		if d.Term != classifications[i].Term {
			t.Errorf("Dive %d: Term=%q, expected %q", i, d.Term, classifications[i].Term)
		}
		if d.WhatIs == "" {
			t.Errorf("Dive %d: WhatIs should not be empty", i)
		}
		if d.Classification != string(classifications[i].Relevance) {
			t.Errorf("Dive %d: Classification=%q, expected %q", i, d.Classification, string(classifications[i].Relevance))
		}
	}

	// Spring Boot content should not contain PostgreSQL/Redis content
	if strings.Contains(dives[0].WhatIs, "PostgreSQL") {
		t.Error("Spring Boot content should not contain PostgreSQL")
	}
}

func TestBatchResponseParser_Fallback(t *testing.T) {
	// Resposta sem separadores claros → usa split por ---
	response := `Seção 1 sobre Spring Boot
---
Seção 2 sobre PostgreSQL`

	classifications := []TechClassification{
		{Term: "Spring Boot", Relevance: STANDARD, Scope: "epic"},
		{Term: "PostgreSQL", Relevance: STANDARD, Scope: "epic"},
	}

	dives := parseBatchResponse(response, classifications, "E1")

	if len(dives) != 2 {
		t.Fatalf("Expected 2 deep dives, got %d", len(dives))
	}

	// Ambas devem ter conteúdo
	for i, d := range dives {
		if d.WhatIs == "" {
			t.Errorf("Dive %d: WhatIs should not be empty", i)
		}
	}
}

func TestBatching_SingleTech(t *testing.T) {
	// Com 1 tech, não deve usar batch (individual é mais direto)
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend",
				Stories: []types.Story{
					{ID: "E1.1", Title: "API", What: "Criar com Spring Boot"},
				},
			},
		},
	}

	config := GetDefaultGenerationConfig()
	config.EnableBatching = true
	config.LLMCaller = mockLLMCaller

	result := GenerateDeepDivesOptimized(backlog, config)

	// Deve funcionar com batching habilitado mas 1 tech
	if len(result.DeepDives) == 0 {
		t.Error("Expected at least 1 deep dive with single tech")
	}
}

func TestBatching_MultipleTechs(t *testing.T) {
	// Com múltiplas techs STANDARD num épico, batch deve usar 1 LLM call
	// Para ter múltiplas techs STANDARD, cada tech precisa aparecer em 2+ stories
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend",
				Stories: []types.Story{
					{ID: "E1.1", Title: "Users API", What: "Criar API com Spring Boot e PostgreSQL e Redis"},
					{ID: "E1.2", Title: "Tasks API", What: "Criar tasks com Spring Boot e PostgreSQL e Redis"},
					{ID: "E1.3", Title: "Cache API", What: "Implementar cache com Spring Boot e Redis e Kafka"},
				},
			},
		},
	}

	batchCaller := func(prompt string) (string, error) {
		return "### Spring Boot\nSB content\n---\n### PostgreSQL\nPG content\n---\n### Redis\nRedis content\n---\n### Kafka\nKafka content", nil
	}

	noBatchCaller := func(prompt string) (string, error) {
		return "Mocked individual response", nil
	}

	// Com batching
	config := GetDefaultGenerationConfig()
	config.EnableBatching = true
	config.LLMCaller = batchCaller
	result := GenerateDeepDivesOptimized(backlog, config)

	// Sem batching
	configNoBatch := GetDefaultGenerationConfig()
	configNoBatch.EnableBatching = false
	configNoBatch.LLMCaller = noBatchCaller
	resultNoBatch := GenerateDeepDivesOptimized(backlog, configNoBatch)

	t.Logf("Batching: %d calls, %d dives | NoBatch: %d calls, %d dives",
		result.Metrics.LLMCallsMade, len(result.DeepDives),
		resultNoBatch.Metrics.LLMCallsMade, len(resultNoBatch.DeepDives))

	// Batching deve usar MENOS LLM calls que sem batching
	if result.Metrics.LLMCallsMade >= resultNoBatch.Metrics.LLMCallsMade {
		t.Errorf("Batching should use fewer LLM calls: batch=%d, nobatch=%d",
			result.Metrics.LLMCallsMade, resultNoBatch.Metrics.LLMCallsMade)
	}
}

func TestBatching_GlobalDDs(t *testing.T) {
	// Global techs (cross-epic) devem ser batched em 1 call
	backlog := types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend",
				Stories: []types.Story{
					{ID: "E1.1", What: "Criar com Spring Boot e PostgreSQL"},
				},
			},
			{
				ID:    "E2",
				Title: "Auth",
				Stories: []types.Story{
					{ID: "E2.1", What: "Login com Spring Boot e PostgreSQL"},
				},
			},
		},
	}

	llmCalls := 0
	countingCaller := func(prompt string) (string, error) {
		llmCalls++
		return "### Spring Boot\nSpring Boot content\n---\n### PostgreSQL\nPG content", nil
	}

	config := GetDefaultGenerationConfig()
	config.EnableBatching = true
	config.LLMCaller = countingCaller

	result := GenerateDeepDivesOptimized(backlog, config)

	// Spring Boot + PostgreSQL are cross-epic → should be batched in 1 global call
	if result.Metrics.CrossEpicGlobalDives < 2 {
		t.Errorf("Expected at least 2 cross-epic global DDs, got %d", result.Metrics.CrossEpicGlobalDives)
	}

	t.Logf("Global DDs: %d, Total LLM calls: %d, Total dives: %d",
		result.Metrics.CrossEpicGlobalDives, result.Metrics.LLMCallsMade, len(result.DeepDives))
}

func TestTechFamilyConsolidation(t *testing.T) {
	// PostgreSQL e pgx são do mesmo grupo → devem ser consolidados
	reg := DefaultRegistry()

	classifications := []TechClassification{
		{Term: "PostgreSQL", Relevance: STANDARD, Scope: "epic", EpicID: "E1"},
		{Term: "pgx", Relevance: SPECIFIC, Scope: "story", EpicID: "E1", StoryID: "E1.1"},
		{Term: "Redis", Relevance: STANDARD, Scope: "epic", EpicID: "E1"},
	}

	consolidated := consolidateByTechGroup(reg, classifications)

	// PostgreSQL e pgx → 1 (consolidado), Redis → 1 = total 2
	if len(consolidated) != 2 {
		t.Errorf("Expected 2 consolidated classifications, got %d", len(consolidated))
		for _, c := range consolidated {
			t.Logf("  %s (%s) - %s", c.Term, c.Relevance, c.Reason)
		}
	}

	// O primary do grupo deve ser PostgreSQL
	foundPG := false
	foundRedis := false
	for _, c := range consolidated {
		if c.Term == "PostgreSQL" {
			foundPG = true
		}
		if c.Term == "Redis" {
			foundRedis = true
		}
		if c.Term == "pgx" {
			t.Error("pgx should be consolidated into PostgreSQL")
		}
	}

	if !foundPG {
		t.Error("PostgreSQL should be in consolidated list as primary")
	}
	if !foundRedis {
		t.Error("Redis should remain (not in PostgreSQL group)")
	}
}

func TestTechFamilyConsolidation_NoGroups(t *testing.T) {
	// Techs sem grupo devem permanecer inalteradas
	reg := DefaultRegistry()

	classifications := []TechClassification{
		{Term: "Kafka", Relevance: STANDARD, Scope: "epic"},
		{Term: "Redis", Relevance: STANDARD, Scope: "epic"},
	}

	consolidated := consolidateByTechGroup(reg, classifications)

	if len(consolidated) != 2 {
		t.Errorf("No groups → no consolidation: expected 2, got %d", len(consolidated))
	}
}

func TestBatchGlobalPromptBuilder(t *testing.T) {
	backlog := types.Backlog{
		Epics: []types.Epic{
			{ID: "E1", Title: "Backend", Stories: []types.Story{{ID: "E1.1", Title: "API", What: "Spring Boot"}}},
			{ID: "E2", Title: "Auth", Stories: []types.Story{{ID: "E2.1", Title: "Login", What: "Spring Boot"}}},
		},
	}

	crossTechs := []*crossEpicTech{
		{Term: "Spring Boot", EpicIDs: []string{"E1", "E2"}, TotalStories: 2},
		{Term: "PostgreSQL", EpicIDs: []string{"E1", "E2"}, TotalStories: 2},
	}

	prompt := buildBatchGlobalPrompt(backlog, crossTechs)

	// Deve conter ambas as techs
	if !strings.Contains(prompt, "Spring Boot") {
		t.Error("Global batch prompt should mention Spring Boot")
	}
	if !strings.Contains(prompt, "PostgreSQL") {
		t.Error("Global batch prompt should mention PostgreSQL")
	}
	// Deve mencionar cross-cutting
	if !strings.Contains(prompt, "cross-cutting") {
		t.Error("Global batch prompt should mention cross-cutting context")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ECONOMY TARGET TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// buildRepresentativeBacklog cria backlog representativo para validar economia.
// 5 épicos, 15 stories, mix de techs (cross-epic, families, triviais).
// Usa "Golang" no texto (normaliza para "Go" via canonical_forms).
// Inclui AcceptanceCriteria com menções adicionais de tecnologia.
func buildRepresentativeBacklog() types.Backlog {
	return types.Backlog{
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Backend API com Golang",
				Stories: []types.Story{
					{
						ID: "E1.1", Title: "Setup Golang",
						What:               "Criar projeto Golang com chi router e PostgreSQL como banco principal",
						AcceptanceCriteria: []string{"Usar Golang 1.24 com chi router", "PostgreSQL configurado com pgx driver"},
					},
					{
						ID: "E1.2", Title: "CRUD Users",
						What:               "Implementar CRUD de usuários com Golang e PostgreSQL usando pgx como driver principal",
						AcceptanceCriteria: []string{"Usar sqlx para queries complexas", "Redis para cache de sessão"},
					},
					{
						ID: "E1.3", Title: "CRUD Products",
						What:               "Implementar CRUD de produtos com Golang, PostgreSQL e Redis para cache de listagem",
						AcceptanceCriteria: []string{"Redis com TTL configurável", "Testes com testify"},
					},
				},
			},
			{
				ID:    "E2",
				Title: "Autenticação e Segurança",
				Stories: []types.Story{
					{
						ID: "E2.1", Title: "Login JWT",
						What:               "Implementar autenticação com Golang usando JWT tokens e bcrypt para hash de senhas",
						AcceptanceCriteria: []string{"JWT com RS256", "Refresh token com Redis"},
					},
					{
						ID: "E2.2", Title: "Middleware Auth",
						What:               "Middleware de autorização Golang para proteger rotas, validar JWT e rate limiting",
						AcceptanceCriteria: []string{"Integrar com chi router", "Rate limit com Redis"},
					},
					{
						ID: "E2.3", Title: "OAuth2 Google",
						What:               "Integrar OAuth2 do Google para login social com Golang e armazenar tokens no PostgreSQL",
						AcceptanceCriteria: []string{"OAuth2 callback handler em Golang", "Persistir no PostgreSQL"},
					},
				},
			},
			{
				ID:    "E3",
				Title: "Infraestrutura e Deploy",
				Stories: []types.Story{
					{
						ID: "E3.1", Title: "Docker",
						What:               "Containerizar aplicação Golang com Docker usando multi-stage build e Alpine",
						AcceptanceCriteria: []string{"Dockerfile multi-stage", "Imagem final < 20MB"},
					},
					{
						ID: "E3.2", Title: "Docker Compose",
						What:               "Criar Docker Compose para ambiente local com Golang, PostgreSQL e Redis conectados",
						AcceptanceCriteria: []string{"Docker Compose com healthcheck", "PostgreSQL e Redis como serviços"},
					},
					{
						ID: "E3.3", Title: "CI/CD Pipeline",
						What:               "Pipeline GitHub Actions para build Golang, testes, lint e deploy com Docker",
						AcceptanceCriteria: []string{"GitHub Actions com cache de módulos Golang", "Build Docker image"},
					},
				},
			},
			{
				ID:    "E4",
				Title: "Observabilidade",
				Stories: []types.Story{
					{
						ID: "E4.1", Title: "Métricas",
						What:               "Expor métricas da aplicação Golang com Prometheus e instrumentar endpoints críticos",
						AcceptanceCriteria: []string{"Prometheus metrics endpoint em Golang", "Histogramas de latência"},
					},
					{
						ID: "E4.2", Title: "Dashboards",
						What:               "Configurar dashboards Grafana para monitorar a API Golang com métricas do Prometheus",
						AcceptanceCriteria: []string{"Grafana com datasource Prometheus", "Dashboard de latência e throughput"},
					},
					{
						ID: "E4.3", Title: "Logging e Tracing",
						What:               "Implementar logging estruturado em Golang com Zap e distributed tracing com OpenTelemetry",
						AcceptanceCriteria: []string{"Zap logger configurado", "OpenTelemetry spans nos handlers"},
					},
				},
			},
			{
				ID:    "E5",
				Title: "Documentação e Qualidade",
				Stories: []types.Story{
					{
						ID: "E5.1", Title: "API Docs",
						What:               "Gerar documentação Swagger/OpenAPI da API Golang automaticamente a partir de annotations",
						AcceptanceCriteria: []string{"Swagger UI servido pelo Golang", "OpenAPI 3.0 spec gerada"},
					},
					{
						ID: "E5.2", Title: "Testes",
						What:               "Cobertura de testes em Golang com testify, mocks e testes de integração com PostgreSQL",
						AcceptanceCriteria: []string{"Testify para assertions", "PostgreSQL de teste via Docker"},
					},
					{
						ID: "E5.3", Title: "ADRs e Docs",
						What:               "Documentar decisões técnicas: Golang, PostgreSQL, Redis, Docker e arquitetura hexagonal",
						AcceptanceCriteria: []string{"ADR para escolha de Golang e PostgreSQL", "ADR para Redis como cache"},
					},
				},
			},
		},
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// BATCH RESPONSE PARSER EDGE CASES
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBatchResponseParser_EmptyResponse(t *testing.T) {
	classifications := []TechClassification{
		{Term: "Spring Boot", Relevance: STANDARD, Scope: "epic"},
		{Term: "PostgreSQL", Relevance: STANDARD, Scope: "epic"},
	}

	dives := parseBatchResponse("", classifications, "E1")

	// Empty response → should return dives with empty WhatIs
	if len(dives) != 2 {
		t.Fatalf("Expected 2 dives from empty response (graceful degradation), got %d", len(dives))
	}

	// Each dive should have Term from classifications
	for i, d := range dives {
		if d.Term != classifications[i].Term {
			t.Errorf("Dive %d: expected Term=%q, got %q", i, classifications[i].Term, d.Term)
		}
	}
}

func TestBatchResponseParser_SingleTech(t *testing.T) {
	response := "Spring Boot é um framework para criar aplicações Java rapidamente."

	classifications := []TechClassification{
		{Term: "Spring Boot", Relevance: STANDARD, Scope: "epic"},
	}

	dives := parseBatchResponse(response, classifications, "E1")

	if len(dives) != 1 {
		t.Fatalf("Expected 1 dive for single tech, got %d", len(dives))
	}

	if dives[0].Term != "Spring Boot" {
		t.Errorf("Expected Term='Spring Boot', got %q", dives[0].Term)
	}

	// WhatIs should have content (the whole response is used)
	if dives[0].WhatIs == "" {
		t.Error("WhatIs should not be empty for single tech response")
	}

	if dives[0].Classification != string(STANDARD) {
		t.Errorf("Expected Classification=%q, got %q", string(STANDARD), dives[0].Classification)
	}
}

func TestBatchResponseParser_MalformedSeparators(t *testing.T) {
	// Response with wrong separators (== instead of --- or ###)
	response := `Spring Boot é ótimo para microsserviços.
===========
PostgreSQL é robusto para dados relacionais.
===========
Redis é rápido para cache.`

	classifications := []TechClassification{
		{Term: "Spring Boot", Relevance: STANDARD, Scope: "epic"},
		{Term: "PostgreSQL", Relevance: STANDARD, Scope: "epic"},
		{Term: "Redis", Relevance: CRITICAL, Scope: "epic"},
	}

	dives := parseBatchResponse(response, classifications, "E1")

	// Should gracefully degrade: return dives even with wrong separators
	if len(dives) != 3 {
		t.Fatalf("Expected 3 dives with malformed separators (graceful degradation), got %d", len(dives))
	}

	// All dives should have Terms from classifications
	for i, d := range dives {
		if d.Term != classifications[i].Term {
			t.Errorf("Dive %d: expected Term=%q, got %q", i, classifications[i].Term, d.Term)
		}
	}

	// At least the first dive should have non-empty content
	// (the entire response is used as fallback when splitting fails)
	if dives[0].WhatIs == "" {
		t.Error("First dive WhatIs should not be empty")
	}
}

func TestEconomy_TargetReduction(t *testing.T) {
	backlog := buildRepresentativeBacklog()

	batchMockCaller := func(prompt string) (string, error) {
		// Simular resposta batched com múltiplas seções
		return "### Go\nGo é uma linguagem...\n---\n### PostgreSQL\nPostgreSQL é...\n---\n### Redis\nRedis é...\n---\n### JWT\nJWT tokens...\n---\n### Docker\nDocker é...\n---\n### Prometheus\nPrometheus é...\n---\n### Grafana\nGrafana é...\n---\n### OAuth2\nOAuth2 é...\n---\n### OpenAPI\nOpenAPI é...", nil
	}

	config := GetDefaultGenerationConfig()
	config.EnableBatching = true
	config.LLMCaller = batchMockCaller

	result := GenerateDeepDivesOptimized(backlog, config)

	t.Logf("Economy metrics:")
	t.Logf("  TotalTechsExtracted: %d", result.Metrics.TotalTechsExtracted)
	t.Logf("  TrivialFiltered: %d", result.Metrics.TrivialFiltered)
	t.Logf("  CrossEpicGlobalDives: %d", result.Metrics.CrossEpicGlobalDives)
	t.Logf("  CrossEpicDeduplicated: %d", result.Metrics.CrossEpicDeduplicated)
	t.Logf("  LLMCallsMade: %d", result.Metrics.LLMCallsMade)
	t.Logf("  LLMCallsSaved: %d", result.Metrics.LLMCallsSaved)
	t.Logf("  ReductionPercent: %.1f%%", result.Metrics.ReductionPercent)
	t.Logf("  TotalDives: %d", result.Metrics.TotalDives)
	t.Logf("  Classification: trivial=%d standard=%d specific=%d critical=%d",
		result.Metrics.TrivialCount, result.Metrics.StandardCount,
		result.Metrics.SpecificCount, result.Metrics.CriticalCount)

	// Meta: >= 60% de economia (margem de segurança vs meta real de 65-70%)
	if result.Metrics.ReductionPercent < 60.0 {
		t.Errorf("Economy target not met: got %.1f%%, want >= 60%%",
			result.Metrics.ReductionPercent)
	}

	// Deve ter gerado deep dives
	if result.Metrics.TotalDives == 0 {
		t.Error("Should have generated some deep dives")
	}

	// Deve ter economia vs abordagem antiga
	if result.Metrics.LLMCallsSaved <= 0 {
		t.Error("Should have saved some LLM calls")
	}
}

func TestEconomy_BatchingVsNoBatching(t *testing.T) {
	backlog := buildRepresentativeBacklog()

	batchCaller := func(prompt string) (string, error) {
		return "### Tech1\ncontent\n---\n### Tech2\ncontent", nil
	}
	noBatchCaller := func(prompt string) (string, error) {
		return "Individual response", nil
	}

	// Com batching
	configBatch := GetDefaultGenerationConfig()
	configBatch.EnableBatching = true
	configBatch.LLMCaller = batchCaller
	resultBatch := GenerateDeepDivesOptimized(backlog, configBatch)

	// Sem batching
	configNoBatch := GetDefaultGenerationConfig()
	configNoBatch.EnableBatching = false
	configNoBatch.LLMCaller = noBatchCaller
	resultNoBatch := GenerateDeepDivesOptimized(backlog, configNoBatch)

	t.Logf("Batching: %d calls, %.1f%% reduction | NoBatch: %d calls, %.1f%% reduction",
		resultBatch.Metrics.LLMCallsMade, resultBatch.Metrics.ReductionPercent,
		resultNoBatch.Metrics.LLMCallsMade, resultNoBatch.Metrics.ReductionPercent)

	// Batching deve resultar em MENOS LLM calls
	if resultBatch.Metrics.LLMCallsMade >= resultNoBatch.Metrics.LLMCallsMade {
		t.Errorf("Batching should use fewer LLM calls: batch=%d, nobatch=%d",
			resultBatch.Metrics.LLMCallsMade, resultNoBatch.Metrics.LLMCallsMade)
	}

	// Batching deve ter MAIOR economia
	if resultBatch.Metrics.ReductionPercent <= resultNoBatch.Metrics.ReductionPercent {
		t.Errorf("Batching should have higher reduction: batch=%.1f%%, nobatch=%.1f%%",
			resultBatch.Metrics.ReductionPercent, resultNoBatch.Metrics.ReductionPercent)
	}
}
