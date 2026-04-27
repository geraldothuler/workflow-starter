package techref

import "testing"

func TestIsTrivial(t *testing.T) {
	tests := []struct {
		term     string
		expected bool
	}{
		// Triviais óbvios
		{"HTTP", true},
		{"JSON", true},
		{"GET", true},
		{"POST", true},
		{"API", true},
		{"REST", true},
		{"CRUD", true},

		// Case-insensitive
		{"http", true},
		{"json", true},
		{"Api", true},

		// Com espaços
		{" HTTP ", true},
		{" JSON ", true},

		// Vazio
		{"", true},
		{"   ", true},

		// NÃO triviais (frameworks, libs específicas)
		{"Spring Boot", false},
		{"React", false},
		{"Kafka", false},
		{"Redis", false},
		{"PostgreSQL", false},
		{"Docker", false},
		{"Kubernetes", false},
		{"JWT", false},
		{"OAuth2", false},
		{"GraphQL", false},

		// Padrões arquiteturais (NÃO triviais)
		{"Event Sourcing", false},
		{"CQRS", false},
		{"Circuit Breaker", false},
		{"Saga Pattern", false},

		// Conceitos básicos (triviais)
		{"endpoint", true},
		{"rota", true},
		{"controller", true},
		{"service", true},

		// Estruturas de dados básicas (triviais)
		{"array", true},
		{"list", true},
		{"map", true},
		{"string", true},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			result := IsTrivial(tt.term)
			if result != tt.expected {
				t.Errorf("IsTrivial(%q) = %v, expected %v", tt.term, result, tt.expected)
			}
		})
	}
}

func TestIsTrivialWithContext(t *testing.T) {
	tests := []struct {
		term     string
		context  string
		expected bool
	}{
		// API é trivial normalmente
		{"API", "criar uma API REST simples", true},

		// Mas não se contexto menciona otimização
		{"API", "otimizar performance da API com cache customizado", false},

		// JSON é trivial normalmente
		{"JSON", "retornar JSON na resposta", true},

		// Mas não com configuração especial
		{"JSON", "JSON com serialização customizada", false},

		// HTTP é trivial
		{"HTTP", "requisição HTTP GET", true},

		// Mas não com padrão específico
		{"HTTP", "arquitetura HTTP com padrão específico", false},

		// Login é trivial
		{"login", "tela de login", true},

		// Mas não com migração
		{"login", "migração do sistema de login para OAuth2", false},
	}

	for _, tt := range tests {
		t.Run(tt.term+"_with_context", func(t *testing.T) {
			result := IsTrivialWithContext(tt.term, tt.context)
			if result != tt.expected {
				t.Errorf("IsTrivialWithContext(%q, %q) = %v, expected %v",
					tt.term, tt.context, result, tt.expected)
			}
		})
	}
}

func TestFilterTrivialTerms(t *testing.T) {
	input := []string{
		"HTTP",        // trivial
		"JSON",        // trivial
		"Kafka",       // NÃO trivial
		"REST",        // trivial
		"Spring Boot", // NÃO trivial
		"API",         // trivial
		"Redis",       // NÃO trivial
	}

	result := FilterTrivialTerms(input)

	expected := []string{"Kafka", "Spring Boot", "Redis"}

	if len(result) != len(expected) {
		t.Errorf("FilterTrivialTerms() returned %d items, expected %d", len(result), len(expected))
	}

	for _, exp := range expected {
		found := false
		for _, r := range result {
			if r == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("FilterTrivialTerms() missing expected term: %q", exp)
		}
	}
}

func TestFilterTrivialTermsWithContext(t *testing.T) {
	terms := []string{
		"HTTP",
		"JSON",
		"Kafka",
		"API",
	}

	// Contexto simples: tudo trivial exceto Kafka
	context1 := "criar API REST simples com JSON"
	result1 := FilterTrivialTermsWithContext(terms, context1)
	if len(result1) != 1 || result1[0] != "Kafka" {
		t.Errorf("FilterTrivialTermsWithContext() with simple context failed, got %v", result1)
	}

	// Contexto com otimização: API pode se tornar relevante
	context2 := "otimização da API com cache customizado"
	result2 := FilterTrivialTermsWithContext(terms, context2)
	found := false
	for _, r := range result2 {
		if r == "API" {
			found = true
		}
	}
	if !found {
		t.Errorf("FilterTrivialTermsWithContext() should keep 'API' with optimization context")
	}
}

func TestGetTrivialCategories(t *testing.T) {
	categories := GetTrivialCategories()

	expectedCategories := []string{
		"Protocols",
		"Formats",
		"HTTPMethods",
		"BasicConcepts",
		"DataStructures",
		"BasicAuth",
		"GenericDB",
		"BasicFrontend",
		"CommonAbbrev",
	}

	for _, cat := range expectedCategories {
		if _, exists := categories[cat]; !exists {
			t.Errorf("GetTrivialCategories() missing category: %q", cat)
		}
	}

	for cat, terms := range categories {
		if len(terms) == 0 {
			t.Errorf("GetTrivialCategories() category %q is empty", cat)
		}
	}
}

func TestTrivialTerms_NewCategories(t *testing.T) {
	// Testa que os novos termos adicionados nas 3 categorias são reconhecidos como trivial
	tests := []struct {
		term     string
		category string
	}{
		// devops_concepts
		{"CI", "devops"}, {"CD", "devops"}, {"pipeline", "devops"},
		{"deploy", "devops"}, {"staging", "devops"}, {"production", "devops"},
		{"build", "devops"}, {"release", "devops"}, {"rollback", "devops"},
		{"branch", "devops"}, {"merge", "devops"}, {"commit", "devops"},

		// testing_concepts
		{"test", "testing"}, {"teste", "testing"}, {"mock", "testing"},
		{"stub", "testing"}, {"fixture", "testing"}, {"TDD", "testing"},
		{"BDD", "testing"}, {"coverage", "testing"}, {"e2e", "testing"},

		// architecture_patterns
		{"MVC", "arch"}, {"MVP", "arch"}, {"MVVM", "arch"},
		{"microservices", "arch"}, {"monolith", "arch"},
		{"middleware", "arch"}, {"proxy", "arch"}, {"gateway", "arch"},
		{"cache", "arch"}, {"caching", "arch"},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			if !IsTrivial(tt.term) {
				t.Errorf("IsTrivial(%q) = false, expected true (category: %s)", tt.term, tt.category)
			}
		})
	}
}

func TestTrivialTerms_SpecificPatternsNotTrivial(t *testing.T) {
	// Padrões específicos continuam NÃO-triviais (merecem deep dive)
	nonTrivial := []string{
		"Event Sourcing", "CQRS", "Circuit Breaker", "Saga Pattern",
		"DDD", "Spring Boot", "React", "Kafka", "Redis",
	}

	for _, term := range nonTrivial {
		t.Run(term, func(t *testing.T) {
			if IsTrivial(term) {
				t.Errorf("IsTrivial(%q) = true, expected false (specific pattern should NOT be trivial)", term)
			}
		})
	}
}

func TestTrivialTerms_GenericPatterns(t *testing.T) {
	terms := []string{
		"ORM", "SDK", "CLI", "IDE", "CDN", "DNS", "ACL", "TTL",
		"ADR", "RFC", "SLA", "SLO", "SLI", "DSL", "IDL", "RPC",
		"ETL", "ELT",
	}

	for _, term := range terms {
		t.Run(term, func(t *testing.T) {
			if !IsTrivial(term) {
				t.Errorf("IsTrivial(%q) = false, expected true (generic pattern should be trivial)", term)
			}
		})
	}
}

// Benchmark
func BenchmarkIsTrivial(b *testing.B) {
	term := "HTTP"
	for i := 0; i < b.N; i++ {
		IsTrivial(term)
	}
}

func BenchmarkFilterTrivialTerms(b *testing.B) {
	terms := []string{
		"HTTP", "JSON", "Kafka", "REST", "Spring Boot",
		"API", "Redis", "PostgreSQL", "GET", "POST",
	}

	for i := 0; i < b.N; i++ {
		FilterTrivialTerms(terms)
	}
}
