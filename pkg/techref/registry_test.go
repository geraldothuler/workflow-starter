package techref

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// REGISTRY LOADING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestNewTechRegistry_LoadsEmbedded(t *testing.T) {
	reg, err := NewTechRegistry()
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// Verify known techs loaded
	if len(reg.KnownTechs) < 50 {
		t.Errorf("expected 50+ known techs, got %d", len(reg.KnownTechs))
	}

	// Verify canonical forms loaded
	if len(reg.CanonicalForms) < 100 {
		t.Errorf("expected 100+ canonical forms, got %d", len(reg.CanonicalForms))
	}

	// Verify trivial terms loaded
	if len(reg.trivialSet) < 50 {
		t.Errorf("expected 50+ trivial terms, got %d", len(reg.trivialSet))
	}

	// Verify verbs loaded
	if len(reg.verbSet) < 40 {
		t.Errorf("expected 40+ verbs, got %d", len(reg.verbSet))
	}

	// Verify common words loaded
	if len(reg.commonWordSet) < 60 {
		t.Errorf("expected 60+ common words, got %d", len(reg.commonWordSet))
	}

	// Verify acronym blacklist loaded
	if len(reg.AcronymBlacklist) < 5 {
		t.Errorf("expected 5+ blacklisted acronyms, got %d", len(reg.AcronymBlacklist))
	}

	// Verify confidence config loaded
	if reg.Confidence.Thresholds.Min == 0 {
		t.Error("confidence min threshold not loaded")
	}
	if len(reg.Confidence.LayerScores) < 4 {
		t.Errorf("expected 4 layer scores, got %d", len(reg.Confidence.LayerScores))
	}
}

func TestNewTechRegistry_KnownTechsSortedByLength(t *testing.T) {
	reg, err := NewTechRegistry()
	if err != nil {
		t.Fatal(err)
	}

	// Verify sorted DESC by name length
	for i := 1; i < len(reg.KnownTechs); i++ {
		if len(reg.KnownTechs[i].Name) > len(reg.KnownTechs[i-1].Name) {
			t.Errorf("not sorted DESC: [%d]=%q (len=%d) > [%d]=%q (len=%d)",
				i, reg.KnownTechs[i].Name, len(reg.KnownTechs[i].Name),
				i-1, reg.KnownTechs[i-1].Name, len(reg.KnownTechs[i-1].Name))
			break
		}
	}
}

func TestNewTechRegistry_CategoriesAssigned(t *testing.T) {
	reg, err := NewTechRegistry()
	if err != nil {
		t.Fatal(err)
	}

	// Every tech should have a category
	for _, tech := range reg.KnownTechs {
		if tech.Category == "" {
			t.Errorf("tech %q has no category", tech.Name)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TRIVIAL TERMS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryIsTrivial(t *testing.T) {
	reg, _ := NewTechRegistry()

	trivial := []string{"HTTP", "JSON", "API", "REST", "CRUD", "SQL", "GET", "POST"}
	for _, term := range trivial {
		if !reg.IsTrivial(term) {
			t.Errorf("expected %q to be trivial", term)
		}
	}

	nonTrivial := []string{"Kafka", "PostgreSQL", "Spring Boot", "React", "Docker"}
	for _, term := range nonTrivial {
		if reg.IsTrivial(term) {
			t.Errorf("expected %q to NOT be trivial", term)
		}
	}
}

func TestRegistryIsTrivial_CaseInsensitive(t *testing.T) {
	reg, _ := NewTechRegistry()

	if !reg.IsTrivial("http") {
		t.Error("expected 'http' (lowercase) to be trivial")
	}
	if !reg.IsTrivial("JSON") {
		t.Error("expected 'JSON' (uppercase) to be trivial")
	}
	if !reg.IsTrivial("Api") {
		t.Error("expected 'Api' (mixed case) to be trivial")
	}
}

func TestRegistryIsTrivial_Empty(t *testing.T) {
	reg, _ := NewTechRegistry()

	if !reg.IsTrivial("") {
		t.Error("empty string should be trivial")
	}
	if !reg.IsTrivial("   ") {
		t.Error("whitespace should be trivial")
	}
}

func TestRegistryIsTrivialWithContext(t *testing.T) {
	reg, _ := NewTechRegistry()

	// "API" is trivial without context
	if !reg.IsTrivialWithContext("API", "normal text") {
		t.Error("API should be trivial in normal context")
	}

	// "API" becomes relevant with optimization context
	if reg.IsTrivialWithContext("API", "otimização de performance da API") {
		t.Error("API should NOT be trivial in optimization context")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CANONICAL FORMS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryNormalize(t *testing.T) {
	reg, _ := NewTechRegistry()

	tests := []struct {
		input    string
		expected string
	}{
		{"postgres", "PostgreSQL"},
		{"PostgreSQL", "PostgreSQL"},
		{"spring boot", "Spring Boot"},
		{"springboot", "Spring Boot"},
		{"react", "React"},
		{"reactjs", "React"},
		{"k8s", "Kubernetes"},
		{"golang", "Go"},
		{"UnknownTech", "UnknownTech"}, // Passthrough
	}

	for _, tt := range tests {
		got := reg.NormalizeToCanonical(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeToCanonical(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// VERBS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryIsVerb(t *testing.T) {
	reg, _ := NewTechRegistry()

	verbs := []string{"criar", "Criar", "create", "implementar", "retorna", "integra"}
	for _, v := range verbs {
		if !reg.IsVerb(v) {
			t.Errorf("expected %q to be a verb", v)
		}
	}

	nonVerbs := []string{"Kafka", "Spring", "PostgreSQL"}
	for _, v := range nonVerbs {
		if reg.IsVerb(v) {
			t.Errorf("expected %q to NOT be a verb", v)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// COMMON WORDS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryIsCommonWord(t *testing.T) {
	reg, _ := NewTechRegistry()

	common := []string{"Criar", "Produtos", "Sistema", "Schema", "Create", "Data"}
	for _, w := range common {
		if !reg.IsCommonWord(w) {
			t.Errorf("expected %q to be a common word", w)
		}
	}

	uncommon := []string{"kafka", "spring"} // lowercase not in common words
	for _, w := range uncommon {
		if reg.IsCommonWord(w) {
			t.Errorf("expected %q to NOT be a common word", w)
		}
	}
}

func TestRegistryIsCommonWord_BusinessTerms(t *testing.T) {
	reg, _ := NewTechRegistry()

	// New business terms added via YAML
	business := []string{"Pagamento", "Payment", "Invoice", "Cart", "Notification"}
	for _, w := range business {
		if !reg.IsCommonWord(w) {
			t.Errorf("expected business term %q to be a common word", w)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CONFIDENCE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryConfidenceConfig(t *testing.T) {
	reg, _ := NewTechRegistry()

	if reg.MinConfidence() != 0.65 {
		t.Errorf("min confidence = %f, want 0.65", reg.MinConfidence())
	}

	if reg.LayerScore("known") != 1.0 {
		t.Errorf("known layer score = %f, want 1.0", reg.LayerScore("known"))
	}

	if reg.LayerScore("isolated") != 0.50 {
		t.Errorf("isolated layer score = %f, want 0.50", reg.LayerScore("isolated"))
	}

	if reg.Penalty("verb_before") != -0.15 {
		t.Errorf("verb_before penalty = %f, want -0.15", reg.Penalty("verb_before"))
	}

	if reg.Bonus("tech_nearby") != 0.10 {
		t.Errorf("tech_nearby bonus = %f, want 0.10", reg.Bonus("tech_nearby"))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// OPTIONS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryWithExtraTechs(t *testing.T) {
	reg, err := NewTechRegistry(
		WithExtraTechs(
			KnownTech{Name: "CustomDB", Category: "custom"},
			KnownTech{Name: "MyFramework", Category: "custom"},
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Should include the extras
	found := false
	for _, tech := range reg.KnownTechs {
		if tech.Name == "CustomDB" {
			found = true
			break
		}
	}
	if !found {
		t.Error("extra tech 'CustomDB' not found in registry")
	}
}

func TestRegistryWithExtraCanonical(t *testing.T) {
	reg, err := NewTechRegistry(
		WithExtraCanonical(map[string]string{
			"mydb": "MyDatabase",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if got := reg.NormalizeToCanonical("mydb"); got != "MyDatabase" {
		t.Errorf("NormalizeToCanonical(mydb) = %q, want MyDatabase", got)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SINGLETON
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestDefaultRegistry_Singleton(t *testing.T) {
	ResetDefaultRegistry()

	reg1 := DefaultRegistry()
	reg2 := DefaultRegistry()

	if reg1 != reg2 {
		t.Error("DefaultRegistry should return same instance")
	}

	if len(reg1.KnownTechs) < 50 {
		t.Errorf("default registry should have 50+ techs, got %d", len(reg1.KnownTechs))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// COMPOUND PATTERNS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryCompoundPatterns(t *testing.T) {
	reg, _ := NewTechRegistry()

	words := reg.CompoundKnownWords()
	if !words["Spring"] || !words["Boot"] {
		t.Error("expected Spring and Boot in compound known words")
	}

	techs := reg.CompoundKnownTechs()
	if !techs["Java"] || !techs["React"] {
		t.Error("expected Java and React in compound known techs")
	}

	modifiers := reg.CompoundTechModifiers()
	if !modifiers["Native"] || !modifiers["Cloud"] {
		t.Error("expected Native and Cloud in compound modifiers")
	}

	verbs := reg.CompoundVerbs()
	if !verbs["Criar"] || !verbs["Create"] {
		t.Error("expected Criar and Create in compound verbs")
	}

	business := reg.CompoundBusinessTerms()
	if !business["Produtos"] || !business["Products"] {
		t.Error("expected Produtos and Products in compound business terms")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ACRONYM BLACKLIST
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryAcronymBlacklist(t *testing.T) {
	reg, _ := NewTechRegistry()

	blacklisted := []string{"GO", "FOR", "SET", "GET", "APP", "API"}
	for _, a := range blacklisted {
		if !reg.IsBlacklistedAcronym(a) {
			t.Errorf("expected %q to be blacklisted", a)
		}
	}

	allowed := []string{"JWT", "AWS", "GCP", "MQTT"}
	for _, a := range allowed {
		if reg.IsBlacklistedAcronym(a) {
			t.Errorf("expected %q to NOT be blacklisted", a)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// DATA INTEGRITY
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestRegistryDataIntegrity_KnownTechsHaveCanonicalForms(t *testing.T) {
	reg, _ := NewTechRegistry()

	// Every known tech name should exist as a canonical form value
	canonicalValues := make(map[string]bool)
	for _, v := range reg.CanonicalForms {
		canonicalValues[v] = true
	}

	for _, tech := range reg.KnownTechs {
		// Tech name itself should normalize to itself
		normalized := reg.NormalizeToCanonical(tech.Name)
		if normalized != tech.Name {
			t.Errorf("known tech %q normalizes to %q instead of itself", tech.Name, normalized)
		}
	}
}

func TestRegistryDataIntegrity_AliasesMatchCanonical(t *testing.T) {
	reg, _ := NewTechRegistry()

	for _, tech := range reg.KnownTechs {
		for _, alias := range tech.Aliases {
			canonical := reg.NormalizeToCanonical(alias)
			if canonical != tech.Name {
				t.Errorf("alias %q of %q normalizes to %q", alias, tech.Name, canonical)
			}
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// WITH PROJECT DIR / MERGE OVERRIDES (Fase B)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestWithProjectDir_NoOverrideDir(t *testing.T) {
	// Non-existent dir → should not fail
	reg, err := NewTechRegistry(WithProjectDir("/nonexistent/path"))
	if err != nil {
		t.Fatalf("should not error with non-existent dir: %v", err)
	}
	if len(reg.KnownTechs) < 50 {
		t.Error("should still have embedded defaults")
	}
}

func TestWithProjectDir_MergesKnownTechs(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "deep-dives")
	os.MkdirAll(overrideDir, 0755)

	// Write override with extra known techs
	yaml := `custom_tools:
  - name: MyCustomDB
    aliases: [mycustomdb]
  - name: InternalTool
`
	os.WriteFile(filepath.Join(overrideDir, "known_technologies.yml"), []byte(yaml), 0644)

	reg, err := NewTechRegistry(WithProjectDir(dir))
	if err != nil {
		t.Fatal(err)
	}

	// Should include embedded techs + extras
	found := false
	for _, tech := range reg.KnownTechs {
		if tech.Name == "MyCustomDB" {
			found = true
			if tech.Category != "custom_tools" {
				t.Errorf("expected category 'custom_tools', got %q", tech.Category)
			}
			break
		}
	}
	if !found {
		t.Error("MyCustomDB not found after merging override")
	}

	// Embedded techs should still exist
	foundSpring := false
	for _, tech := range reg.KnownTechs {
		if tech.Name == "Spring Boot" {
			foundSpring = true
			break
		}
	}
	if !foundSpring {
		t.Error("embedded Spring Boot should still exist after merge")
	}
}

func TestWithProjectDir_MergesCanonicalForms(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "deep-dives")
	os.MkdirAll(overrideDir, 0755)

	yaml := `mydb: MyDatabase
customtool: CustomTool
`
	os.WriteFile(filepath.Join(overrideDir, "canonical_forms.yml"), []byte(yaml), 0644)

	reg, err := NewTechRegistry(WithProjectDir(dir))
	if err != nil {
		t.Fatal(err)
	}

	if got := reg.NormalizeToCanonical("mydb"); got != "MyDatabase" {
		t.Errorf("expected MyDatabase, got %q", got)
	}

	// Embedded forms should still work
	if got := reg.NormalizeToCanonical("postgres"); got != "PostgreSQL" {
		t.Errorf("expected PostgreSQL, got %q", got)
	}
}

func TestWithProjectDir_MergesTrivialTerms(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "deep-dives")
	os.MkdirAll(overrideDir, 0755)

	yaml := `project_specific:
  - InternalWidget
  - HelperTool
`
	os.WriteFile(filepath.Join(overrideDir, "trivial_terms.yml"), []byte(yaml), 0644)

	reg, err := NewTechRegistry(WithProjectDir(dir))
	if err != nil {
		t.Fatal(err)
	}

	if !reg.IsTrivial("InternalWidget") {
		t.Error("InternalWidget should be trivial after merge")
	}
	if !reg.IsTrivial("HTTP") {
		t.Error("embedded trivial HTTP should still work")
	}
}

func TestWithProjectDir_MergesConfidence(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "deep-dives")
	os.MkdirAll(overrideDir, 0755)

	yaml := `thresholds:
  min: 0.70
  high: 0.90
  very_high: 0.98
specific_keywords:
  - custom_keyword
`
	os.WriteFile(filepath.Join(overrideDir, "confidence.yml"), []byte(yaml), 0644)

	reg, err := NewTechRegistry(WithProjectDir(dir))
	if err != nil {
		t.Fatal(err)
	}

	if reg.MinConfidence() != 0.70 {
		t.Errorf("expected min=0.70 after override, got %f", reg.MinConfidence())
	}
	if reg.Confidence.Thresholds.VeryHigh != 0.98 {
		t.Errorf("expected very_high=0.98 after override, got %f", reg.Confidence.Thresholds.VeryHigh)
	}

	// Check specific keywords were merged
	found := false
	for _, kw := range reg.SpecificKeywords() {
		if kw == "custom_keyword" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custom_keyword should be in specific keywords after merge")
	}
}

func TestMergeOverrides_MalformedYAML_LogsWarning(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "deep-dives")
	os.MkdirAll(overrideDir, 0755)

	// Write invalid YAML
	os.WriteFile(filepath.Join(overrideDir, "known_technologies.yml"), []byte("{{invalid yaml"), 0644)

	// Should not fail — malformed YAML is logged as warning, not fatal
	reg, err := NewTechRegistry(WithProjectDir(dir))
	if err != nil {
		t.Fatal(err)
	}

	// Embedded defaults should still be there
	if len(reg.KnownTechs) < 50 {
		t.Error("should still have embedded defaults after malformed override")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// NIL GUARD TESTS (Fase B)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestNilRegistryGuard_PublicFunctions(t *testing.T) {
	// Public WithRegistry functions should handle nil by falling back to DefaultRegistry
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil registry caused panic: %v", r)
		}
	}()

	story := types.Story{What: "Spring Boot PostgreSQL"}
	epic := types.Epic{ID: "E1", Stories: []types.Story{story}}

	// These should all work with nil registry (fallback to DefaultRegistry)
	techs := ExtractTechsFromStoryWithRegistry(nil, story)
	if len(techs) == 0 {
		t.Error("ExtractTechsFromStoryWithRegistry(nil) should work")
	}

	extractions := ExtractTechsByEpicWithRegistry(nil, epic)
	if len(extractions) == 0 {
		t.Error("ExtractTechsByEpicWithRegistry(nil) should work")
	}

	_ = ClassifyTechWithRegistry(nil, "Spring Boot", epic, DefaultClassifierConfig())
	_ = ClassifyAllTechsInEpicWithRegistry(nil, epic, DefaultClassifierConfig())
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// COMPOUND CACHE TESTS (Fase B)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCompoundCaches_PreComputed(t *testing.T) {
	reg, _ := NewTechRegistry()

	// Verify caches are populated (not nil/empty)
	if len(reg.compoundWordsSet) == 0 {
		t.Error("compoundWordsSet should be pre-computed")
	}
	if len(reg.compoundTechsSet) == 0 {
		t.Error("compoundTechsSet should be pre-computed")
	}
	if len(reg.compoundModifiersSet) == 0 {
		t.Error("compoundModifiersSet should be pre-computed")
	}
	if len(reg.compoundVerbsSet) == 0 {
		t.Error("compoundVerbsSet should be pre-computed")
	}
	if len(reg.compoundBusinessSet) == 0 {
		t.Error("compoundBusinessSet should be pre-computed")
	}

	// Verify returned sets are same instance (not new alloc)
	set1 := reg.CompoundKnownWords()
	set2 := reg.CompoundKnownWords()
	// Both should reference same underlying map
	set1["__test_marker__"] = true
	if !set2["__test_marker__"] {
		t.Error("CompoundKnownWords should return cached set, not new allocation")
	}
	delete(set1, "__test_marker__")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TECH GROUPS TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestTechGroups_FindGroup(t *testing.T) {
	reg, _ := NewTechRegistry()

	// PostgreSQL pertence ao grupo "PostgreSQL Ecosystem"
	group := reg.FindGroup("PostgreSQL")
	if group == nil {
		t.Fatal("PostgreSQL should belong to a tech group")
	}
	if group.Primary != "PostgreSQL" {
		t.Errorf("PostgreSQL group primary = %q, want 'PostgreSQL'", group.Primary)
	}

	// pgx também pertence ao mesmo grupo
	pgxGroup := reg.FindGroup("pgx")
	if pgxGroup == nil {
		t.Fatal("pgx should belong to a tech group")
	}
	if pgxGroup.Name != group.Name {
		t.Errorf("pgx and PostgreSQL should be in the same group: %q vs %q",
			pgxGroup.Name, group.Name)
	}
}

func TestTechGroups_PrimaryMember(t *testing.T) {
	reg, _ := NewTechRegistry()

	// Docker e Docker Compose devem estar no mesmo grupo
	dockerGroup := reg.FindGroup("Docker")
	composeGroup := reg.FindGroup("Docker Compose")

	if dockerGroup == nil || composeGroup == nil {
		t.Fatal("Docker and Docker Compose should both have groups")
	}
	if dockerGroup.Name != composeGroup.Name {
		t.Errorf("Docker and Docker Compose should share group: %q vs %q",
			dockerGroup.Name, composeGroup.Name)
	}
	if dockerGroup.Primary != "Docker" {
		t.Errorf("Docker group primary = %q, want 'Docker'", dockerGroup.Primary)
	}
}

func TestTechGroups_NoGroup(t *testing.T) {
	reg, _ := NewTechRegistry()

	// Kafka não pertence a nenhum grupo
	group := reg.FindGroup("Kafka")
	if group != nil {
		t.Errorf("Kafka should not belong to any tech group, got %q", group.Name)
	}

	// Termos vazios ou aleatórios
	if reg.FindGroup("") != nil {
		t.Error("Empty string should not match any group")
	}
	if reg.FindGroup("FooBarBaz") != nil {
		t.Error("Unknown tech should not match any group")
	}
}

func TestTechGroups_LoadedFromConfig(t *testing.T) {
	reg, _ := NewTechRegistry()

	if len(reg.TechGroups) == 0 {
		t.Fatal("TechGroups should be loaded from config/tech_groups.yml")
	}

	// Deve ter pelo menos os 8 grupos definidos
	if len(reg.TechGroups) < 8 {
		t.Errorf("Expected at least 8 tech groups, got %d", len(reg.TechGroups))
	}

	// Cada grupo deve ter primary e members
	for _, g := range reg.TechGroups {
		if g.Name == "" {
			t.Error("Tech group should have a name")
		}
		if g.Primary == "" {
			t.Errorf("Tech group %q should have a primary", g.Name)
		}
		if len(g.Members) == 0 {
			t.Errorf("Tech group %q should have members", g.Name)
		}
	}
}
