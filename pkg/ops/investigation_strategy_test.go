package ops

import "testing"

// 1. Trigger numérico dispara quando acima do threshold
func TestSelectStrategies_NumericTrigger_Match(t *testing.T) {
	strategies := LoadStrategies("strategies")
	if strategies == nil {
		t.Fatal("LoadStrategies returned nil — YAML não encontrado")
	}
	data := map[string]any{"dataset_rows": 100}
	matched := SelectStrategies(data, strategies)
	found := false
	for _, s := range matched {
		if s.Name == "database_source_evaluation" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("database_source_evaluation deveria ter sido selecionada para dataset_rows=100")
	}
}

// 2. Trigger numérico NÃO dispara quando abaixo do threshold (10 não é > 10)
func TestSelectStrategies_NumericTrigger_NoMatch(t *testing.T) {
	strategies := LoadStrategies("strategies")
	data := map[string]any{"dataset_rows": 10}
	matched := SelectStrategies(data, strategies)
	for _, s := range matched {
		if s.Name == "database_source_evaluation" {
			t.Errorf("database_source_evaluation não deveria ser selecionada para dataset_rows=10")
		}
	}
}

// 2b. Trigger has_db_access dispara para qualquer acesso a banco, independente do volume
func TestSelectStrategies_DbAccess_Match(t *testing.T) {
	strategies := LoadStrategies("strategies")
	data := map[string]any{"has_db_access": 1}
	matched := SelectStrategies(data, strategies)
	found := false
	for _, s := range matched {
		if s.Name == "database_source_evaluation" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("database_source_evaluation deveria ser selecionada para has_db_access=1")
	}
}

// 3. Trigger string dispara quando data_source == "scylladb"
func TestSelectStrategies_StringTrigger_Match(t *testing.T) {
	strategies := LoadStrategies("strategies")
	data := map[string]any{"data_source": "scylladb"}
	matched := SelectStrategies(data, strategies)
	found := false
	for _, s := range matched {
		if s.Name == "scylladb_access" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("scylladb_access deveria ter sido selecionada para data_source=scylladb")
	}
}

// 4. Estratégias com llm_usage=none vêm antes das demais
func TestSelectStrategies_SortedByLLMUsage(t *testing.T) {
	strategies := []InvestigationStrategy{
		{Name: "needs_llm", LLMUsage: "required",
			Triggers: []StrategyCond{{Field: "x", Op: ">", Value: 0}}},
		{Name: "no_llm", LLMUsage: "none",
			Triggers: []StrategyCond{{Field: "x", Op: ">", Value: 0}}},
		{Name: "minimal_llm", LLMUsage: "minimal",
			Triggers: []StrategyCond{{Field: "x", Op: ">", Value: 0}}},
	}
	data := map[string]any{"x": 1}
	matched := SelectStrategies(data, strategies)
	if len(matched) != 3 {
		t.Fatalf("expected 3 matched, got %d", len(matched))
	}
	if matched[0].LLMUsage != "none" {
		t.Errorf("primeiro resultado deveria ser llm_usage=none, got %q", matched[0].LLMUsage)
	}
	if matched[1].LLMUsage != "minimal" {
		t.Errorf("segundo resultado deveria ser llm_usage=minimal, got %q", matched[1].LLMUsage)
	}
	if matched[2].LLMUsage != "required" {
		t.Errorf("terceiro resultado deveria ser llm_usage=required, got %q", matched[2].LLMUsage)
	}
}

// 5. Nenhuma trigger ativa → retorna slice vazio (não nil panic)
func TestSelectStrategies_NoMatch_EmptySlice(t *testing.T) {
	strategies := LoadStrategies("strategies")
	data := map[string]any{"unrelated_field": 999}
	matched := SelectStrategies(data, strategies)
	if len(matched) != 0 {
		t.Errorf("esperado 0 estratégias para contexto sem triggers, got %d", len(matched))
	}
}

// 6. Prefer e Avoid estão populados nas estratégias do YAML
func TestSelectStrategies_PreferAndAvoidPopulated(t *testing.T) {
	strategies := LoadStrategies("strategies")
	data := map[string]any{"data_source": "scylladb"}
	matched := SelectStrategies(data, strategies)
	for _, s := range matched {
		if s.Name == "scylladb_access" {
			if s.Prefer.Tool == "" {
				t.Errorf("scylladb_access.prefer.tool não deve ser vazio")
			}
			if len(s.Avoid) == 0 {
				t.Errorf("scylladb_access.avoid não deve ser vazio")
			}
			if s.LLMUsage != "none" {
				t.Errorf("scylladb_access.llm_usage deveria ser 'none', got %q", s.LLMUsage)
			}
			return
		}
	}
	t.Error("scylladb_access não encontrada nos resultados")
}
