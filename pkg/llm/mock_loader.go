package llm

import (
	_ "embed"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/mock_responses.yml
var defaultMockResponsesYAML []byte

// MockConfig holds YAML-driven fixture responses for the mock LLM provider.
type MockConfig struct {
	Responses map[string]string `yaml:"responses"`
}

// LoadMockConfig loads the embedded mock response fixtures.
func LoadMockConfig() (MockConfig, error) {
	var cfg MockConfig
	err := yaml.Unmarshal(defaultMockResponsesYAML, &cfg)
	return cfg, err
}

// NewMockProviderFromConfig creates a MockClient with a smart prompt router.
// Routes each LLM call to the correct fixture response based on prompt keywords,
// so the full backlog pipeline runs end-to-end without any real API calls.
func NewMockProviderFromConfig() (*MockClient, error) {
	cfg, err := LoadMockConfig()
	if err != nil {
		return nil, err
	}

	mock := &MockClient{
		provider: "mock",
		model:    "mock-1.0",
	}

	mock.CompleteFunc = func(prompt string, maxTokens int) (string, *Usage, error) {
		response := routeMockResponse(cfg.Responses, prompt)
		usage := &Usage{
			InputTokens:  120,
			OutputTokens: 280,
			TotalTokens:  400,
			Cost:         0.0,
		}
		return response, usage, nil
	}

	return mock, nil
}

// routeMockResponse selects the fixture response that matches the prompt content.
// Uses unique keyword signatures from each pipeline step's prompt template.
func routeMockResponse(responses map[string]string, prompt string) string {
	// Keywords are unique per prompt template — checked in priority order.
	switch {
	case strings.Contains(prompt, "TRANSCRIÇÃO"):
		// pkg/extractor/prompts.go — extraction step
		return responses["extraction"]

	case strings.Contains(prompt, "sugerir_regras"):
		// pkg/store (future) — heuristic analysis step: receives SQLite ops history,
		// returns pattern observations + suggested trend_rules for store_rules.yml.
		return responses["heuristic_analysis"]

	case strings.Contains(prompt, "why_in_this_story"):
		// pkg/backlog/generator.go — deep dive step (JSON format contains this field)
		return responses["deep_dive"]

	case strings.Contains(prompt, "Use nomes canônicos"):
		// pkg/backlog/generator.go — technology extraction step
		return responses["technologies"]

	case strings.Contains(prompt, "Como engenheiro, quero"):
		// pkg/backlog/generator.go — stories generation step (example in format)
		return responses["stories"]

	case strings.Contains(prompt, "Critério 1 com métrica"):
		// pkg/backlog/generator.go — acceptance criteria step (example in format)
		return responses["criteria"]

	case strings.Contains(prompt, "complexity"):
		// pkg/backlog/generator.go — epics generation step (field in format)
		return responses["epics"]

	default:
		// Fallback: return deep_dive as a reasonable generic JSON response
		if r, ok := responses["deep_dive"]; ok {
			return r
		}
		return `{"mock": true}`
	}
}
