//go:build integration

package backlog

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// skipIfNoKey skips the test if the API key for the given provider is not set.
func skipIfNoKey(t *testing.T, provider llm.Provider) {
	t.Helper()
	envVars := map[llm.Provider]string{
		llm.ProviderClaude:  "ANTHROPIC_API_KEY",
		llm.ProviderChatGPT: "OPENAI_API_KEY",
		llm.ProviderGemini:  "GEMINI_API_KEY",
	}
	key := envVars[provider]
	if os.Getenv(key) == "" {
		t.Skipf("skipping: %s not set", key)
	}
}

// getTestProvider returns the provider to use based on WTB_TEST_PROVIDER env var.
func getTestProvider(t *testing.T) llm.Provider {
	t.Helper()
	if p := os.Getenv("WTB_TEST_PROVIDER"); p != "" {
		provider := llm.Provider(p)
		skipIfNoKey(t, provider)
		return provider
	}
	for _, p := range []llm.Provider{llm.ProviderGemini, llm.ProviderClaude, llm.ProviderChatGPT} {
		envVars := map[llm.Provider]string{
			llm.ProviderClaude:  "ANTHROPIC_API_KEY",
			llm.ProviderChatGPT: "OPENAI_API_KEY",
			llm.ProviderGemini:  "GEMINI_API_KEY",
		}
		if os.Getenv(envVars[p]) != "" {
			return p
		}
	}
	t.Skip("no LLM provider configured")
	return ""
}

// minimalProjectInput creates a tiny ProjectInput to minimize cost.
func minimalProjectInput() *types.ProjectInput {
	return &types.ProjectInput{
		Context:       "API REST para cadastro de usuarios com autenticacao JWT",
		Stack:         "Go, PostgreSQL, JWT",
		NFRs:          "Latencia < 200ms, 99% uptime",
		Volumetry:     "1000 usuarios, 200 req/dia",
		BusinessRules: "Usuarios precisam de email unico",
		Metadata:      map[string]string{},
	}
}

func minimalSpec() *types.Specification {
	return &types.Specification{
		StackDecisions: map[string]interface{}{
			"backend":  "Go",
			"database": "PostgreSQL",
		},
	}
}

func TestIntegration_GenerateEpics(t *testing.T) {
	provider := getTestProvider(t)
	pi := minimalProjectInput()
	spec := minimalSpec()

	gen := NewGenerator(provider, spec, pi)
	gen.SetVerbose(true)

	epics, err := gen.generateEpics(pi)
	if err != nil {
		t.Fatalf("generateEpics failed: %v", err)
	}
	if len(epics) == 0 {
		t.Fatal("expected at least 1 epic")
	}

	for i, epic := range epics {
		if epic.ID == "" {
			t.Errorf("epic[%d] missing ID", i)
		}
		if epic.Title == "" {
			t.Errorf("epic[%d] missing Title", i)
		}
		t.Logf("Epic %d: %s - %s", i, epic.ID, epic.Title)
	}

	// Verify JSON serialization
	data, err := json.Marshal(epics)
	if err != nil {
		t.Fatalf("failed to marshal epics: %v", err)
	}
	t.Logf("[%s] Epics JSON length: %d bytes", provider, len(data))
}

func TestIntegration_GenerateStories(t *testing.T) {
	provider := getTestProvider(t)
	pi := minimalProjectInput()
	spec := minimalSpec()

	gen := NewGenerator(provider, spec, pi)
	gen.SetVerbose(true)

	// Create a minimal epic to generate stories for
	epic := &types.Epic{
		ID:          "E1",
		Title:       "API de Usuarios",
		Description: "CRUD basico de usuarios com autenticacao",
	}

	stories, err := gen.generateStories(pi, epic)
	if err != nil {
		t.Fatalf("generateStories failed: %v", err)
	}
	if len(stories) == 0 {
		t.Fatal("expected at least 1 story")
	}

	for i, story := range stories {
		if story.ID == "" {
			t.Errorf("story[%d] missing ID", i)
		}
		if story.Title == "" {
			t.Errorf("story[%d] missing Title", i)
		}
		t.Logf("Story %d: %s - %s (effort: %d)", i, story.ID, story.Title, story.Effort)
	}
}

func TestIntegration_FullPipeline_Minimal(t *testing.T) {
	provider := getTestProvider(t)
	pi := minimalProjectInput()
	spec := minimalSpec()

	gen := NewGenerator(provider, spec, pi)
	gen.SetVerbose(true)

	opts := GenerateOptions{
		SkipDeepDive:        true, // Skip deep dives to reduce cost
		GenerateTasks:       false,
		ComplexityThreshold: 5,
	}

	backlog, err := gen.Generate(pi, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if backlog == nil {
		t.Fatal("expected non-nil backlog")
	}
	if backlog.Meta.TotalEpics == 0 {
		t.Error("expected TotalEpics > 0")
	}
	if backlog.Meta.TotalStories == 0 {
		t.Error("expected TotalStories > 0")
	}
	if len(backlog.Epics) == 0 {
		t.Error("expected at least 1 epic")
	}

	for _, epic := range backlog.Epics {
		if len(epic.Stories) == 0 {
			t.Errorf("epic %s has no stories", epic.ID)
		}
	}

	t.Logf("[%s] Full pipeline: %d epics, %d stories",
		provider, backlog.Meta.TotalEpics, backlog.Meta.TotalStories)
}

func TestIntegration_CostTracking(t *testing.T) {
	provider := getTestProvider(t)
	pi := minimalProjectInput()
	spec := minimalSpec()

	gen := NewGenerator(provider, spec, pi)

	// Just generate epics to keep cost minimal
	_, err := gen.generateEpics(pi)
	if err != nil {
		t.Fatalf("generateEpics failed: %v", err)
	}

	if gen.totalInputTokens <= 0 {
		t.Errorf("expected totalInputTokens > 0, got %d", gen.totalInputTokens)
	}
	if gen.totalOutputTokens <= 0 {
		t.Errorf("expected totalOutputTokens > 0, got %d", gen.totalOutputTokens)
	}
	if gen.totalCost <= 0 {
		t.Errorf("expected totalCost > 0, got %f", gen.totalCost)
	}

	t.Logf("[%s] Cost: input=%d tokens, output=%d tokens, total=$%.6f",
		provider, gen.totalInputTokens, gen.totalOutputTokens, gen.totalCost)
}
