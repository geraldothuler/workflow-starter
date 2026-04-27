//go:build integration

package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
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

// getTestProvider returns the provider to use.
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

const minimalTranscript = `Reuniao de kickoff do projeto.

Pedro: Precisamos de uma API REST em Go com PostgreSQL para gerenciar usuarios.
Maria: Deve ter login com JWT e latencia abaixo de 200ms.
Pedro: Vamos ter uns 1000 usuarios no primeiro ano, com pico de 50 requisicoes por segundo.
Maria: E precisamos de 99% de uptime, com deploy automatizado.
Pedro: Vamos usar Docker e Kubernetes pra orquestrar.
Maria: Certo, e monitoramento com Prometheus e Grafana.`

// writeTranscript writes the transcript to a temp file and returns the path.
func writeTranscript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}
	return path
}

func TestIntegration_Extract_SimpleTranscript(t *testing.T) {
	provider := getTestProvider(t)
	transcriptPath := writeTranscript(t, minimalTranscript)

	extractor, err := NewTranscriptExtractor(provider, "", "")
	if err != nil {
		t.Fatalf("failed to create extractor: %v", err)
	}

	result, err := extractor.Extract(transcriptPath)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ProjectDefinition == "" {
		t.Error("expected non-empty ProjectDefinition markdown")
	}

	t.Logf("[%s] Extraction result: %d bytes markdown", provider, len(result.ProjectDefinition))
	t.Logf("[%s] Metadata source: %s", provider, result.Metadata.Source)
}

func TestIntegration_Extract_ConfidenceScores(t *testing.T) {
	provider := getTestProvider(t)
	transcriptPath := writeTranscript(t, minimalTranscript)

	extractor, err := NewTranscriptExtractor(provider, "", "")
	if err != nil {
		t.Fatalf("failed to create extractor: %v", err)
	}

	result, err := extractor.Extract(transcriptPath)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result.Scoring == nil {
		t.Skip("scoring not available in result")
	}

	if result.Scoring.OverallScore < 0.0 || result.Scoring.OverallScore > 1.0 {
		t.Errorf("overall confidence %.2f out of range [0.0, 1.0]", result.Scoring.OverallScore)
	}

	t.Logf("[%s] Overall confidence: %.2f", provider, result.Scoring.OverallScore)
	for section, score := range result.Scoring.SectionScores {
		if score.Score < 0.0 || score.Score > 1.0 {
			t.Errorf("section %s confidence %.2f out of range [0.0, 1.0]", section, score.Score)
		}
		t.Logf("[%s] Section %s: %.2f", provider, section, score.Score)
	}
}

func TestIntegration_Extract_StackDetection(t *testing.T) {
	provider := getTestProvider(t)
	transcriptPath := writeTranscript(t, minimalTranscript)

	extractor, err := NewTranscriptExtractor(provider, "", "")
	if err != nil {
		t.Fatalf("failed to create extractor: %v", err)
	}

	result, err := extractor.Extract(transcriptPath)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// The extraction should produce some content
	if result.ProjectDefinition == "" {
		t.Error("expected non-empty project definition")
	}

	// Check that the markdown output mentions at least some technologies from the transcript
	md := result.ProjectDefinition
	foundTech := false
	for _, tech := range []string{"Go", "PostgreSQL", "JWT", "Docker", "Kubernetes"} {
		if containsString(md, tech) {
			foundTech = true
			t.Logf("[%s] Found tech in output: %s", provider, tech)
		}
	}
	if !foundTech {
		t.Error("expected at least one technology from transcript to appear in output")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
