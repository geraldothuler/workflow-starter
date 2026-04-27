package doccheck

import (
	"testing"
)

func TestLoadFrontierRules(t *testing.T) {
	rules, err := LoadFrontierRules()
	if err != nil {
		t.Fatalf("LoadFrontierRules() error: %v", err)
	}
	if len(rules.Frontiers) == 0 {
		t.Fatal("LoadFrontierRules() returned zero frontiers")
	}

	// Verify a known frontier is present
	found := false
	for _, f := range rules.Frontiers {
		if f.ID == "pure_logic" {
			found = true
			if f.Name == "" {
				t.Error("pure_logic frontier has empty name")
			}
			if len(f.Detect) == 0 {
				t.Error("pure_logic frontier has no detect patterns")
			}
			if len(f.Required) == 0 {
				t.Error("pure_logic frontier has no required test types")
			}
			break
		}
	}
	if !found {
		t.Error("expected pure_logic frontier in rules")
	}
}

func TestEvalFrontier_PureLogic(t *testing.T) {
	rules, err := LoadFrontierRules()
	if err != nil {
		t.Fatal(err)
	}

	// Content that matches pure_logic: "deterministic" is one of its detect patterns
	content := `func Score(a, b int) int { return a + b } // deterministic`
	result := EvalFrontier("pkg/scoring/score.go", content, rules)

	if result == nil {
		t.Fatal("expected pure_logic match, got nil")
	}
	if result.FrontierID != "pure_logic" {
		t.Errorf("expected frontier pure_logic, got %s", result.FrontierID)
	}
	if result.FrontierName != "Logica Pura" {
		t.Errorf("expected name 'Logica Pura', got %s", result.FrontierName)
	}
	if len(result.RequiredTests) != 1 || result.RequiredTests[0] != "unit" {
		t.Errorf("expected required [unit], got %v", result.RequiredTests)
	}
}

func TestEvalFrontier_LLMCrossing(t *testing.T) {
	rules, err := LoadFrontierRules()
	if err != nil {
		t.Fatal(err)
	}

	content := `// calls llm.Client to generate backlog
client := llm.NewClient(); resp, err := client.Complete(ctx, prompt)`
	result := EvalFrontier("pkg/backlog/generator.go", content, rules)

	if result == nil {
		t.Fatal("expected llm_crossing match, got nil")
	}
	if result.FrontierID != "llm_crossing" {
		t.Errorf("expected frontier llm_crossing, got %s", result.FrontierID)
	}
}

func TestEvalFrontier_HTTPHandler(t *testing.T) {
	rules, err := LoadFrontierRules()
	if err != nil {
		t.Fatal(err)
	}

	content := `func handler(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }`
	result := EvalFrontier("pkg/server/handler.go", content, rules)

	if result == nil {
		t.Fatal("expected http_handler match, got nil")
	}
	if result.FrontierID != "http_handler" {
		t.Errorf("expected frontier http_handler, got %s", result.FrontierID)
	}
}

func TestEvalFrontier_Filesystem(t *testing.T) {
	rules, err := LoadFrontierRules()
	if err != nil {
		t.Fatal(err)
	}

	content := `err := os.WriteFile(path, data, 0644)`
	result := EvalFrontier("pkg/export/writer.go", content, rules)

	if result == nil {
		t.Fatal("expected filesystem match, got nil")
	}
	if result.FrontierID != "filesystem" {
		t.Errorf("expected frontier filesystem, got %s", result.FrontierID)
	}
	if len(result.RequiredTests) != 1 || result.RequiredTests[0] != "unit_with_tmpdir" {
		t.Errorf("expected required [unit_with_tmpdir], got %v", result.RequiredTests)
	}
}

func TestEvalFrontier_NoMatch(t *testing.T) {
	rules, err := LoadFrontierRules()
	if err != nil {
		t.Fatal(err)
	}

	content := `package main; func main() { println("hello") }`
	result := EvalFrontier("cmd/hello/main.go", content, rules)

	if result != nil {
		t.Errorf("expected nil for no match, got frontier %s", result.FrontierID)
	}
}
