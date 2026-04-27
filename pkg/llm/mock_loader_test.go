package llm

import (
	"strings"
	"testing"
)

func TestLoadMockConfig_HasAllKeys(t *testing.T) {
	cfg, err := LoadMockConfig()
	if err != nil {
		t.Fatalf("LoadMockConfig failed: %v", err)
	}

	required := []string{"extraction", "epics", "stories", "criteria", "technologies", "deep_dive", "heuristic_analysis"}
	for _, key := range required {
		if cfg.Responses[key] == "" {
			t.Errorf("expected non-empty response for key %q", key)
		}
	}
}

func TestLoadMockConfig_ResponsesAreValidJSON(t *testing.T) {
	cfg, err := LoadMockConfig()
	if err != nil {
		t.Fatalf("LoadMockConfig failed: %v", err)
	}

	for key, resp := range cfg.Responses {
		trimmed := strings.TrimSpace(resp)
		if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
			preview := trimmed
			if len(preview) > 20 {
				preview = preview[:20]
			}
			t.Errorf("response[%q] does not look like JSON (starts with %q)", key, preview)
		}
	}
}

func TestRouteMockResponse_Extraction(t *testing.T) {
	cfg, _ := LoadMockConfig()
	prompt := "TRANSCRIÇÃO DA REUNIÃO:\nTech Lead: vamos usar Kafka..."
	got := routeMockResponse(cfg.Responses, prompt)
	if got != cfg.Responses["extraction"] {
		t.Errorf("expected extraction response for TRANSCRIÇÃO prompt")
	}
}

func TestRouteMockResponse_Epics(t *testing.T) {
	cfg, _ := LoadMockConfig()
	prompt := `...FORMATO DE SAÍDA:\n[{"id":"E1","title":"...","complexity": 8}]`
	got := routeMockResponse(cfg.Responses, prompt)
	if got != cfg.Responses["epics"] {
		t.Errorf("expected epics response for complexity prompt")
	}
}

func TestRouteMockResponse_Stories(t *testing.T) {
	cfg, _ := LoadMockConfig()
	prompt := `ÉPICO: E1 - Backend API\n...\n"Como engenheiro, quero X para Y"`
	got := routeMockResponse(cfg.Responses, prompt)
	if got != cfg.Responses["stories"] {
		t.Errorf("expected stories response for 'Como engenheiro' prompt")
	}
}

func TestRouteMockResponse_Criteria(t *testing.T) {
	cfg, _ := LoadMockConfig()
	prompt := `...FORMATO:\n["Critério 1 com métrica específica", ...]`
	got := routeMockResponse(cfg.Responses, prompt)
	if got != cfg.Responses["criteria"] {
		t.Errorf("expected criteria response")
	}
}

func TestRouteMockResponse_Technologies(t *testing.T) {
	cfg, _ := LoadMockConfig()
	prompt := `...Use nomes canônicos (ex: "Kafka" não "Apache Kafka")...`
	got := routeMockResponse(cfg.Responses, prompt)
	if got != cfg.Responses["technologies"] {
		t.Errorf("expected technologies response")
	}
}

func TestRouteMockResponse_DeepDive(t *testing.T) {
	cfg, _ := LoadMockConfig()
	prompt := `...why_in_this_story": "Por que é necessária neste contexto...`
	got := routeMockResponse(cfg.Responses, prompt)
	if got != cfg.Responses["deep_dive"] {
		t.Errorf("expected deep_dive response")
	}
}

func TestNewMockProviderFromConfig_CompleteReturnsNonEmpty(t *testing.T) {
	mock, err := NewMockProviderFromConfig()
	if err != nil {
		t.Fatalf("NewMockProviderFromConfig failed: %v", err)
	}

	resp, usage, err := mock.CompleteWithUsage("TRANSCRIÇÃO DA REUNIÃO:\ntest", 1000)
	if err != nil {
		t.Fatalf("CompleteWithUsage failed: %v", err)
	}
	if resp == "" {
		t.Error("expected non-empty response")
	}
	if usage == nil || usage.Cost != 0.0 {
		t.Error("expected zero cost for mock provider")
	}
	if mock.ProviderName() != "mock" {
		t.Errorf("expected provider 'mock', got %q", mock.ProviderName())
	}
}

func TestNewProvider_MockProvider(t *testing.T) {
	p, err := NewProvider(ProviderConfig{Provider: "mock"})
	if err != nil {
		t.Fatalf("NewProvider mock failed: %v", err)
	}
	if p.ProviderName() != "mock" {
		t.Errorf("expected 'mock', got %q", p.ProviderName())
	}
}

func TestRouteMockResponse_HeuristicAnalysis(t *testing.T) {
	cfg, _ := LoadMockConfig()
	// Prompt template para análise de histórico SQLite — keyword: sugerir_regras
	prompt := `Analise o histórico de execuções abaixo e sugerir_regras YAML compatíveis com store_rules.yml`
	got := routeMockResponse(cfg.Responses, prompt)
	if got != cfg.Responses["heuristic_analysis"] {
		t.Errorf("expected heuristic_analysis response for sugerir_regras prompt")
	}
	// Valida que a resposta tem os campos esperados
	if !strings.Contains(got, "suggested_rules") {
		t.Errorf("heuristic_analysis response missing 'suggested_rules' field")
	}
	if !strings.Contains(got, "patterns") {
		t.Errorf("heuristic_analysis response missing 'patterns' field")
	}
}

func TestNewProvider_MockToggle_EnvVar(t *testing.T) {
	t.Setenv("WTB_MOCK_LLM", "1")

	// Mesmo com provider="claude" (sem key), deve retornar mock via toggle
	p, err := NewProvider(ProviderConfig{Provider: "claude"})
	if err != nil {
		t.Fatalf("NewProvider with WTB_MOCK_LLM=1 failed: %v", err)
	}
	if p.ProviderName() != "mock" {
		t.Errorf("expected 'mock' via toggle, got %q", p.ProviderName())
	}
}

