package techref

import (
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// REGRESSION TESTS: Validam que os 4 bugs conhecidos estão fixos
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ─────────────────────────────────────────────────────────────
// Fix 1: Substring Deduplication
// "Spring Boot" + "Spring" → só "Spring Boot"
// ─────────────────────────────────────────────────────────────

func TestDeduplicateSubstrings_SpringBootAndSpring(t *testing.T) {
	matches := []TechMatch{
		{Term: "Spring Boot", Layer: LayerKnown, Confidence: 1.0},
		{Term: "Spring", Layer: LayerKnown, Confidence: 1.0},
	}

	result := deduplicateSubstrings(matches)

	if len(result) != 1 {
		t.Errorf("expected 1 match after substring dedup, got %d: %v", len(result), techNames(result))
	}
	if len(result) > 0 && result[0].Term != "Spring Boot" {
		t.Errorf("expected 'Spring Boot' to survive, got %q", result[0].Term)
	}
}

func TestDeduplicateSubstrings_ReactNativeAndReact(t *testing.T) {
	matches := []TechMatch{
		{Term: "React", Layer: LayerKnown, Confidence: 1.0},
		{Term: "React Native", Layer: LayerKnown, Confidence: 1.0},
	}

	result := deduplicateSubstrings(matches)

	if len(result) != 1 {
		t.Errorf("expected 1 match after substring dedup, got %d: %v", len(result), techNames(result))
	}
	if len(result) > 0 && result[0].Term != "React Native" {
		t.Errorf("expected 'React Native' to survive, got %q", result[0].Term)
	}
}

func TestDeduplicateSubstrings_BootAndSpringBoot(t *testing.T) {
	matches := []TechMatch{
		{Term: "Boot", Layer: LayerIsolated, Confidence: 0.5},
		{Term: "Spring Boot", Layer: LayerKnown, Confidence: 1.0},
	}

	result := deduplicateSubstrings(matches)

	if len(result) != 1 {
		t.Errorf("expected 1 match, got %d: %v", len(result), techNames(result))
	}
	if len(result) > 0 && result[0].Term != "Spring Boot" {
		t.Errorf("expected 'Spring Boot', got %q", result[0].Term)
	}
}

func TestDeduplicateSubstrings_IndependentTermsPreserved(t *testing.T) {
	matches := []TechMatch{
		{Term: "Redis", Layer: LayerKnown, Confidence: 1.0},
		{Term: "Docker", Layer: LayerKnown, Confidence: 1.0},
		{Term: "Kafka", Layer: LayerKnown, Confidence: 1.0},
	}

	result := deduplicateSubstrings(matches)

	if len(result) != 3 {
		t.Errorf("expected 3 independent matches preserved, got %d: %v", len(result), techNames(result))
	}
}

func TestDeduplicateSubstrings_EmptyAndSingle(t *testing.T) {
	// Empty
	result := deduplicateSubstrings([]TechMatch{})
	if len(result) != 0 {
		t.Errorf("expected 0 for empty input, got %d", len(result))
	}

	// Single
	single := []TechMatch{{Term: "PostgreSQL", Layer: LayerKnown, Confidence: 1.0}}
	result = deduplicateSubstrings(single)
	if len(result) != 1 {
		t.Errorf("expected 1 for single input, got %d", len(result))
	}
}

func TestDeduplicateSubstrings_CaseInsensitive(t *testing.T) {
	matches := []TechMatch{
		{Term: "spring", Layer: LayerIsolated, Confidence: 0.5},
		{Term: "Spring Boot", Layer: LayerKnown, Confidence: 1.0},
	}

	result := deduplicateSubstrings(matches)

	if len(result) != 1 {
		t.Errorf("expected 1 match (case-insensitive dedup), got %d: %v", len(result), techNames(result))
	}
}

// ─────────────────────────────────────────────────────────────
// Fix 2: Context Validation — Verbs Rejected
// ─────────────────────────────────────────────────────────────

func TestIsVerb_CaseInsensitive(t *testing.T) {
	tests := []struct {
		word     string
		expected bool
	}{
		{"Retornar", true},
		{"retornar", true},
		{"RETORNAR", true}, // all-caps also matched (case-insensitive)
		{"Integrar", true},
		{"integrar", true},
		{"Integra", true},  // conjugation
		{"integra", true},  // conjugation lowercase
		{"Retorna", true},  // conjugation
		{"Processa", true}, // conjugation
		{"Create", true},
		{"create", true},
		{"Deploy", true},
		{"deploy", true},
		// Non-verbs
		{"PostgreSQL", false},
		{"Redis", false},
		{"Spring", false},
		{"Kafka", false},
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			result := isVerb(tt.word)
			if result != tt.expected {
				t.Errorf("isVerb(%q) = %v, expected %v", tt.word, result, tt.expected)
			}
		})
	}
}

func TestIsValidIsolated_VerbsRejected(t *testing.T) {
	// Words with verbs before should be rejected
	ctx := ContextInfo{HasVerbBefore: true}
	if isValidIsolated("Retornar", ctx) {
		t.Error("isValidIsolated should reject word with verb before")
	}

	// Words with verbs after should be rejected
	ctx = ContextInfo{HasVerbAfter: true}
	if isValidIsolated("Dados", ctx) {
		t.Error("isValidIsolated should reject word with verb after")
	}
}

func TestIsValidIsolated_KnownTechAccepted(t *testing.T) {
	ctx := ContextInfo{} // no verb context
	if !isValidIsolated("PostgreSQL", ctx) {
		t.Error("isValidIsolated should accept known tech PostgreSQL")
	}
	if !isValidIsolated("Redis", ctx) {
		t.Error("isValidIsolated should accept known tech Redis")
	}
}

func TestIsValidIsolated_UnknownWordRejected(t *testing.T) {
	// Unknown word with no positive signals should be rejected (conservative default)
	ctx := ContextInfo{}
	if isValidIsolated("Blargfoo", ctx) {
		t.Error("isValidIsolated should reject unknown word with no positive signals")
	}
}

func TestIsValidIsolated_BetweenTechsAccepted(t *testing.T) {
	ctx := ContextInfo{HasTechBefore: true, HasTechAfter: true}
	if !isValidIsolated("SomeWord", ctx) {
		t.Error("isValidIsolated should accept word between known techs")
	}
}

// ─────────────────────────────────────────────────────────────
// Fix 3: False Positives
// ─────────────────────────────────────────────────────────────

func TestExtraction_GoNotExtractedAsVerb(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "Go implement the user service",
		What:  "We need to go ahead and implement the REST API with PostgreSQL",
	}

	techs := ExtractTechsFromStory(story)

	for _, tech := range techs {
		if tech == "Go" {
			t.Error("'Go' should NOT be extracted when used as a verb")
		}
	}
}

func TestExtraction_GolangExtracted(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "Criar API em Golang",
		What:  "Usar Golang e PostgreSQL para criar microserviço",
	}

	techs := ExtractTechsFromStory(story)

	found := false
	for _, tech := range techs {
		// Golang alias → canonical name "Go"
		if tech == "Go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'Golang' should be extracted as 'Go'. Got: %v", techs)
	}
}

func TestExtraction_GoExtractedAsTech(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "Backend com Go",
		What:  "Criar projeto Go com PostgreSQL como banco principal",
	}

	techs := ExtractTechsFromStory(story)

	found := false
	for _, tech := range techs {
		if tech == "Go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'Go' should be extracted when used as tech name. Got: %v", techs)
	}
}

func TestExtraction_GoWithGolang_Dedup(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "Backend com Go e Golang",
		What:  "Usar Golang (Go) para criar o backend",
	}

	techs := ExtractTechsFromStory(story)

	count := 0
	for _, tech := range techs {
		if tech == "Go" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("'Go' and 'Golang' should deduplicate to 1 entry, got %d. Techs: %v", count, techs)
	}
	if count == 0 {
		t.Errorf("'Go' should be extracted at least once. Got: %v", techs)
	}
}

func TestExtraction_GoShortWordBoundary(t *testing.T) {
	// "go" appearing inside other words should NOT be extracted
	story := types.Story{
		ID:    "E1.1",
		Title: "Google Cloud Platform integration",
		What:  "The ongoing cargo management uses MongoDB for storage",
	}

	techs := ExtractTechsFromStory(story)

	for _, tech := range techs {
		if tech == "Go" {
			t.Errorf("'Go' should NOT be extracted from inside other words (going, cargo, google). Got: %v", techs)
		}
	}
}

func TestExtraction_iOSNotExtracted(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "App iOS de pedidos",
		What:  "Criar app iOS para gerenciar pedidos usando Flutter",
	}

	techs := ExtractTechsFromStory(story)

	for _, tech := range techs {
		if tech == "iOS" {
			t.Error("'iOS' should NOT be extracted (it's a platform, treated as trivial)")
		}
	}

	// But Flutter should be extracted
	hasFlutter := false
	for _, tech := range techs {
		if tech == "Flutter" {
			hasFlutter = true
		}
	}
	if !hasFlutter {
		t.Errorf("Flutter should be extracted. Got: %v", techs)
	}
}

func TestExtraction_AndroidNotExtracted(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "App Android",
		What:  "Criar aplicativo Android com React Native",
	}

	techs := ExtractTechsFromStory(story)

	for _, tech := range techs {
		if tech == "Android" {
			t.Error("'Android' should NOT be extracted (it's a platform, treated as trivial)")
		}
	}
}

func TestExtraction_SchemaNotExtracted(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "Database Schema design",
		What:  "Design the Schema for users table with PostgreSQL",
	}

	techs := ExtractTechsFromStory(story)

	for _, tech := range techs {
		if tech == "Schema" {
			t.Error("'Schema' should NOT be extracted (it's a common word, not a technology)")
		}
	}
}

func TestIsCommonWordHelper_ExpandedList(t *testing.T) {
	commonWords := []string{
		// DB/design terms
		"Schema", "Model", "Index", "Column", "Migration",
		"Query", "Entity", "Field", "Module", "Component",
		// Verbs (português)
		"Retornar", "Integrar", "Processar", "Validar",
		"Retorna", "Integra", "Processa",
		// Verbs (inglês)
		"Return", "Process", "Deploy", "Handle",
		// Business terms
		"Status", "Type", "Name", "Value", "Result",
	}

	for _, word := range commonWords {
		if !isCommonWordHelper(word) {
			t.Errorf("isCommonWordHelper(%q) should return true", word)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// End-to-End: Pipeline completo sem duplicatas
// ─────────────────────────────────────────────────────────────

func TestExtraction_NoSubstringDuplicates(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "API com Spring Boot e React Native",
		What:  "Criar API REST com Spring Boot, Spring Cloud e React Native. Usar PostgreSQL e Redis.",
		AcceptanceCriteria: []string{
			"Usar Docker e Kubernetes para deploy",
			"Monitorar com Prometheus e Grafana",
		},
	}

	techs := ExtractTechsFromStory(story)

	// "Spring" should NOT appear separately if "Spring Boot" or "Spring Cloud" is present
	for _, tech := range techs {
		if tech == "Spring" {
			t.Errorf("'Spring' should not appear separately when 'Spring Boot' or 'Spring Cloud' exists. Got: %v", techs)
		}
		if tech == "React" {
			t.Errorf("'React' should not appear separately when 'React Native' exists. Got: %v", techs)
		}
	}

	// These should be present
	expected := []string{"Spring Boot", "PostgreSQL", "Redis", "Docker", "Kubernetes"}
	for _, exp := range expected {
		found := false
		for _, tech := range techs {
			if tech == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %q in extraction. Got: %v", exp, techs)
		}
	}
}

func TestExtraction_VerbsNotExtracted(t *testing.T) {
	story := types.Story{
		ID:    "E1.1",
		Title: "Retornar dados do usuário",
		What:  "Integrar com API externa para Processar pedidos. Validar dados com Spring Boot.",
	}

	techs := ExtractTechsFromStory(story)

	// Verbs should NOT be in results
	verbs := []string{"Retornar", "Integrar", "Processar", "Validar"}
	for _, verb := range verbs {
		for _, tech := range techs {
			if tech == verb {
				t.Errorf("Verb %q should NOT be extracted as tech. Got: %v", verb, techs)
			}
		}
	}

	// Spring Boot SHOULD be extracted
	found := false
	for _, tech := range techs {
		if tech == "Spring Boot" {
			found = true
		}
	}
	if !found {
		t.Errorf("'Spring Boot' should be extracted. Got: %v", techs)
	}
}

func TestExtraction_ComplexScenarioEconomy(t *testing.T) {
	// Cenário complexo com muitas variações — testa economia
	story := types.Story{
		ID:    "E1.1",
		Title: "Backend com Spring Boot e PostgreSQL",
		What: "Criar microserviço usando Spring Boot com Spring Cloud para service discovery. " +
			"Armazenar dados em PostgreSQL com Redis para cache. " +
			"Integrar com Kafka para eventos assíncronos. " +
			"Monitorar com Prometheus e Grafana. " +
			"Containerizar com Docker e orquestrar com Kubernetes.",
		AcceptanceCriteria: []string{
			"Usar JWT para autenticação",
			"Documentar com Swagger",
		},
	}

	techs := ExtractTechsFromStory(story)

	// Não deve ter duplicatas por substring
	for i := 0; i < len(techs); i++ {
		for j := i + 1; j < len(techs); j++ {
			if techs[i] == techs[j] {
				t.Errorf("Duplicate found: %q at positions %d and %d. All: %v", techs[i], i, j, techs)
			}
		}
	}

	// Techs que devem estar presentes
	mustHave := []string{"Spring Boot", "PostgreSQL", "Redis", "Kafka", "Docker", "Kubernetes"}
	for _, tech := range mustHave {
		found := false
		for _, extracted := range techs {
			if extracted == tech {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %q in extraction result. Got: %v", tech, techs)
		}
	}

	// Verbs/common words should NOT be present
	shouldNotHave := []string{"Criar", "Armazenar", "Integrar", "Monitorar", "Usar", "Documentar"}
	for _, bad := range shouldNotHave {
		for _, tech := range techs {
			if tech == bad {
				t.Errorf("False positive: %q should NOT be extracted. Got: %v", bad, techs)
			}
		}
	}
}

func TestExtraction_AcronymFalsePositives(t *testing.T) {
	// Siglas que são conceitos genéricos, NÃO devem ser extraídas como tech
	story := types.Story{
		ID:    "E1.1",
		Title: "Documentação e infraestrutura",
		What: "Criar ADR para decisões técnicas. " +
			"Configurar TTL no cache. " +
			"Usar RPC para comunicação entre serviços. " +
			"Definir SLA para uptime. " +
			"Configurar DNS e CDN para o frontend. " +
			"Usar ORM para acesso ao banco. " +
			"Distribuir SDK para clientes.",
	}

	techs := ExtractTechsFromStory(story)

	falsePositives := []string{"ADR", "TTL", "RPC", "SLA", "DNS", "CDN", "ORM", "SDK"}
	for _, fp := range falsePositives {
		for _, tech := range techs {
			if tech == fp {
				t.Errorf("False positive: %q should NOT be extracted as a technology. Got: %v", fp, techs)
			}
		}
	}
}

func TestExtraction_RealAcronymsStillExtracted(t *testing.T) {
	// Siglas reais de tecnologias devem continuar sendo extraídas
	story := types.Story{
		ID:    "E1.1",
		Title: "Autenticação e mensageria",
		What: "Implementar autenticação com JWT tokens. " +
			"Usar MQTT para IoT devices. " +
			"Deploy na AWS com Terraform.",
	}

	techs := ExtractTechsFromStory(story)

	mustHave := []string{"JWT", "MQTT", "AWS", "Terraform"}
	for _, expected := range mustHave {
		found := false
		for _, tech := range techs {
			if tech == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Real tech %q should be extracted. Got: %v", expected, techs)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────

func techNames(matches []TechMatch) []string {
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Term
	}
	return names
}
