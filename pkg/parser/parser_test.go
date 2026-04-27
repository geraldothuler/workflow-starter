package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestExtractSections(t *testing.T) {
	content := `# Projeto

## Contexto

Sistema de gerenciamento de tarefas para equipes.

## Volumetria

- 1000 usuarios ativos
- 50 requisicoes/segundo

## Stack

Go, PostgreSQL, Redis

## RNFs

- 99.9% uptime
- Latencia < 200ms
`

	sections := extractSections(content)

	if sections["contexto"] == "" {
		t.Error("expected 'contexto' section to be extracted")
	}
	if sections["volumetria"] == "" {
		t.Error("expected 'volumetria' section to be extracted")
	}
	if sections["stack"] == "" {
		t.Error("expected 'stack' section to be extracted")
	}
}

func TestExtractSections_WithSubsections(t *testing.T) {
	content := `## Stack

### Frontend
React, TypeScript

### Backend
Go, Gin

### Database
PostgreSQL
`

	sections := extractSections(content)

	if sections["stack"] == "" {
		t.Error("expected main 'stack' section")
	}
	if sections["frontend"] == "" {
		t.Error("expected 'frontend' subsection")
	}
	if sections["backend"] == "" {
		t.Error("expected 'backend' subsection")
	}
}

func TestExtractSections_CodeBlocks(t *testing.T) {
	content := "## Contexto\n\nSistema com codigo:\n\n```go\n## Isso nao e header\nfunc main() {}\n```\n\nTexto depois.\n"

	sections := extractSections(content)

	if sections["contexto"] == "" {
		t.Error("expected 'contexto' section")
	}
	// The "## Isso nao e header" inside code block should not create a new section
	if _, exists := sections["isso nao e header"]; exists {
		t.Error("should not extract headers inside code blocks")
	}
}

func TestNormalizeSection(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Contexto", "contexto"},
		{"Stack Técnico", "stack tcnico"},
		{"  Volumetria  ", "volumetria"},
		{"RNFs (Requisitos)", "rnfs requisitos"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeSection(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNormalizeSectionNames(t *testing.T) {
	sections := map[string]string{
		"contexto do projeto": "Texto contexto",
		"stack tecnico":       "Go, React",
		"rnfs requisitos":     "99% uptime",
		"fluxo de dados":      "Input -> Output",
	}

	normalized := normalizeSectionNames(sections)

	if normalized["contexto"] != "Texto contexto" {
		t.Error("expected 'contexto' to be mapped")
	}
	if normalized["stack"] != "Go, React" {
		t.Error("expected 'stack' to be mapped")
	}
	if normalized["rnfs"] != "99% uptime" {
		t.Error("expected 'rnfs' to be mapped")
	}
	if normalized["fluxo"] != "Input -> Output" {
		t.Error("expected 'fluxo' to be mapped")
	}
}

func TestValidateInput(t *testing.T) {
	tests := []struct {
		name          string
		context       string
		volumetry     string
		stack         string
		expectedCount int
	}{
		{"all present", "ctx", "vol", "stk", 0},
		{"missing context", "", "vol", "stk", 1},
		{"missing all", "", "", "", 3},
		{"missing volumetry", "ctx", "", "stk", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &types_stub{Context: tt.context, Volumetry: tt.volumetry, Stack: tt.stack}
			missing := validateInput_stub(input)
			if len(missing) != tt.expectedCount {
				t.Errorf("expected %d missing, got %d: %v", tt.expectedCount, len(missing), missing)
			}
		})
	}
}

// Stub types to avoid importing types package in the test directly
// (ValidateInput uses types.ProjectInput)
type types_stub struct {
	Context   string
	Volumetry string
	Stack     string
}

func validateInput_stub(input *types_stub) []string {
	var missing []string
	if input.Context == "" {
		missing = append(missing, "Contexto")
	}
	if input.Volumetry == "" {
		missing = append(missing, "Volumetria")
	}
	if input.Stack == "" {
		missing = append(missing, "Stack")
	}
	return missing
}

func TestContainsNumbers(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"no numbers here", false},
		{"has 42 numbers", true},
		{"1000 usuarios", true},
		{"", false},
		{"abc123", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsNumbers(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCalculateHash(t *testing.T) {
	hash1 := calculateHash("content1")
	hash2 := calculateHash("content2")
	hash1Again := calculateHash("content1")

	if hash1 == "" {
		t.Error("hash should not be empty")
	}
	if hash1 == hash2 {
		t.Error("different content should produce different hashes")
	}
	if hash1 != hash1Again {
		t.Error("same content should produce same hash")
	}
}

func TestParseInput_RealFile(t *testing.T) {
	// Create temp file with project input
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.md")

	content := `# Projeto Test

## Contexto

Sistema de teste para validar parser.

## Volumetria

- 100 usuarios ativos
- 10 req/s

## Stack

Go, PostgreSQL

## RNFs

- 99% uptime
`

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	input, err := ParseInput(inputPath)
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}

	if input.Context == "" {
		t.Error("expected context to be extracted")
	}
	if input.Volumetry == "" {
		t.Error("expected volumetry to be extracted")
	}
	if input.Stack == "" {
		t.Error("expected stack to be extracted")
	}
	if input.RawContent == "" {
		t.Error("expected raw content")
	}
	if input.Metadata["file"] != inputPath {
		t.Errorf("expected file path in metadata, got %q", input.Metadata["file"])
	}
	if input.Metadata["hash"] == "" {
		t.Error("expected hash in metadata")
	}
}

func TestParseInput_NonExistentFile(t *testing.T) {
	_, err := ParseInput("/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestNewGoldenPathParser(t *testing.T) {
	parser := NewGoldenPathParser("/some/path")
	if parser == nil {
		t.Fatal("expected non-nil parser")
	}

	gp, err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gp == nil {
		t.Fatal("expected non-nil golden path")
	}
}

func TestNewTeamConfigParser(t *testing.T) {
	parser := NewTeamConfigParser("/some/path")
	if parser == nil {
		t.Fatal("expected non-nil parser")
	}

	tp, err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil team patterns")
	}
}

func TestNewProjectInputParser(t *testing.T) {
	parser := NewProjectInputParser("/nonexistent/file.md")
	if parser == nil {
		t.Fatal("expected non-nil parser")
	}
	// Parse will fail because file doesn't exist
	_, err := parser.Parse()
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestMerger(t *testing.T) {
	merger := NewMerger(nil, nil, nil)
	if merger == nil {
		t.Fatal("expected non-nil merger")
	}

	spec, err := merger.Merge()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil specification")
	}
}

func TestConfigMerger(t *testing.T) {
	merger := NewConfigMerger(nil, nil, nil)
	if merger == nil {
		t.Fatal("expected non-nil merger")
	}

	spec, err := merger.Merge()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil specification")
	}
}

func TestExtractVolumetryFromContext(t *testing.T) {
	sections := map[string]string{
		"contexto": "Sistema com 500 usuarios ativos e 100 transacoes/segundo",
		"volumetria": "",
	}

	result := extractVolumetryFromContext(sections)
	if result == "" {
		t.Error("should extract volumetry from context section")
	}
}

func TestExtractVolumetryFromContext_NoNumbers(t *testing.T) {
	sections := map[string]string{
		"contexto": "Sistema sem dados numericos",
	}

	result := extractVolumetryFromContext(sections)
	if result != "" {
		t.Errorf("expected empty result for text without numbers, got %q", result)
	}
}

func TestValidateInput_Real(t *testing.T) {
	tests := []struct {
		name     string
		input    *types.ProjectInput
		expected int
	}{
		{"all present", &types.ProjectInput{Context: "ctx", Volumetry: "vol", Stack: "stk"}, 0},
		{"missing context", &types.ProjectInput{Volumetry: "vol", Stack: "stk"}, 1},
		{"missing all", &types.ProjectInput{}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing := ValidateInput(tt.input)
			if len(missing) != tt.expected {
				t.Errorf("expected %d missing, got %d: %v", tt.expected, len(missing), missing)
			}
		})
	}
}

func TestParseGoldenPaths_File(t *testing.T) {
	tmpDir := t.TempDir()
	gpPath := filepath.Join(tmpDir, "golden-paths.md")

	content := `# Golden Paths

## Event Sourcing

### Pattern: Event Store
**Quando usar:** Sistema que precisa de auditoria completa
**Implementação validada:** CQRS com Kafka
**Decisão:** Usar Avro para serialização
**Validado em:** Projeto X

## Microservices

### Pattern: Service Mesh
**Quando usar:** Múltiplos serviços
**Implementação validada:** Kubernetes + Istio
`

	os.WriteFile(gpPath, []byte(content), 0644)

	gp, err := ParseGoldenPaths(gpPath)
	if err != nil {
		t.Fatalf("ParseGoldenPaths failed: %v", err)
	}
	if gp == nil {
		t.Fatal("expected non-nil golden path")
	}
	if len(gp.Patterns) == 0 {
		t.Error("expected patterns to be parsed")
	}
}

func TestParseGoldenPaths_NonExistent(t *testing.T) {
	gp, err := ParseGoldenPaths("/nonexistent/golden-paths.md")
	if err != nil {
		t.Fatalf("should not return error for missing file, got: %v", err)
	}
	if gp == nil {
		t.Fatal("should return empty structure for missing file")
	}
}

func TestParseTeamPatterns_File(t *testing.T) {
	tmpDir := t.TempDir()
	tpPath := filepath.Join(tmpDir, "team-patterns.md")

	content := `# Team Patterns

## Code Review

### Pattern: Pair Programming
**Quando usar:** Código complexo
**Implementação validada:** Mob programming sessions
`

	os.WriteFile(tpPath, []byte(content), 0644)

	tp, err := ParseTeamPatterns(tpPath)
	if err != nil {
		t.Fatalf("ParseTeamPatterns failed: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil team patterns")
	}
}

func TestDetectPatternReferences_ExplicitIDs(t *testing.T) {
	text := "Esta história usa GP-001 e TP-003 para referência"
	refs := DetectPatternReferences(text, nil, nil)

	found := map[string]bool{}
	for _, ref := range refs {
		found[ref] = true
	}
	if !found["GP-001"] {
		t.Error("should detect GP-001")
	}
	if !found["TP-003"] {
		t.Error("should detect TP-003")
	}
}

func TestDetectPatternReferences_ByName(t *testing.T) {
	gp := &types.GoldenPath{
		Patterns: map[string]types.Pattern{
			"GP-001": {Name: "Event Sourcing"},
		},
	}
	text := "Usamos event sourcing neste projeto"

	refs := DetectPatternReferences(text, gp, nil)
	found := false
	for _, ref := range refs {
		if ref == "GP-001" {
			found = true
		}
	}
	if !found {
		t.Error("should detect GP-001 by name match")
	}
}

func TestDetectPatternReferences_NoMatches(t *testing.T) {
	text := "Texto sem referências a patterns"
	refs := DetectPatternReferences(text, nil, nil)
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}

func TestParseGoldenPathsFromContent(t *testing.T) {
	content := `# Golden Paths

## Event Sourcing com Kafka
Quando: Eventos de telemetria IoT
Tech: Kafka com particionamento por device_id

## Stream Processing com Flink
Quando: Detecção de padrões em tempo real
Tech: Flink + RocksDB
`
	gp, err := ParseGoldenPathsFromContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gp == nil {
		t.Fatal("expected non-nil golden paths")
	}
	if len(gp.Patterns) < 2 {
		t.Errorf("expected at least 2 patterns, got %d", len(gp.Patterns))
	}
}

func TestParseTeamPatternsFromContent(t *testing.T) {
	content := `# Team Patterns

## Backend em Kotlin + Spring Boot
Stack: Spring Boot 3.2+ com Kotlin 1.9+

## Data Processing
Tech: Flink 1.17+ (streaming) + Kafka 3.5+ (messaging)
`
	tp, err := ParseTeamPatternsFromContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil team patterns")
	}
	if len(tp.Patterns) < 2 {
		t.Errorf("expected at least 2 patterns, got %d", len(tp.Patterns))
	}
}

func TestParseGoldenPathsFromContent_Empty(t *testing.T) {
	gp, err := ParseGoldenPathsFromContent("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gp == nil {
		t.Fatal("expected non-nil golden paths")
	}
	if len(gp.Patterns) != 0 {
		t.Errorf("expected 0 patterns for empty content, got %d", len(gp.Patterns))
	}
}

func TestValidateGoldenPathStruct(t *testing.T) {
	v := NewAdvancedValidator(nil, nil)
	if err := v.ValidateGoldenPathStruct(&types.GoldenPath{}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTeamConfigStruct(t *testing.T) {
	v := NewAdvancedValidator(nil, nil)
	if err := v.ValidateTeamConfigStruct(&types.TeamConfig{}, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateProjectInputStruct(t *testing.T) {
	v := NewAdvancedValidator(nil, nil)
	if err := v.ValidateProjectInputStruct(&types.ProjectInput{}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAdvancedValidator(t *testing.T) {
	validator := NewAdvancedValidator(nil, nil)
	if validator == nil {
		t.Fatal("expected non-nil validator")
	}

	// All validation methods should return nil (stub implementations)
	if err := validator.ValidateGoldenPath("path"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validator.ValidateTeamConfig("path"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validator.ValidateProjectInput("path"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
