package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

func TestNewTranscriptExtractorWithClient(t *testing.T) {
	mock := llm.NewMockClient("response")

	ext := NewTranscriptExtractorWithClient(mock, nil, nil)

	if ext == nil {
		t.Fatal("expected non-nil extractor")
	}
	if ext.llmClient == nil {
		t.Error("expected llmClient to be set")
	}
}

func TestExtractJSON_Extractor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain json",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json with markdown",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "with whitespace",
			input:    "  {\"key\": \"value\"}  ",
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtract_BasicTranscript(t *testing.T) {
	// Mock response with valid JSON that matches ExtractedData structure
	llmResponse := `{
		"context": "Sistema de gerenciamento de tarefas",
		"problem": "Empresa precisa organizar projetos",
		"objectives": ["Criar sistema web", "Permitir colaboracao"],
		"volumetry": {"usuarios": "500", "projetos": "100"},
		"stack": [{"name": "Go", "confidence": 0.9, "source": "explicit"}],
		"nfrs": ["99% uptime"],
		"overall_confidence": 0.85,
		"section_confidence": {"context": 0.9, "problem": 0.8, "volumetry": 0.7, "stack": 0.9, "nfrs": 0.8},
		"speakers": ["PM", "Tech Lead"],
		"warnings": []
	}`

	mock := llm.NewMockClient(llmResponse)
	ext := NewTranscriptExtractorWithClient(mock, nil, nil)

	// Create temp transcript file
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.md")
	transcript := `# Reuniao de Planejamento

PM: Precisamos de um sistema de gerenciamento de tarefas.
Tech Lead: Vamos usar Go no backend com PostgreSQL.
PM: Esperamos 500 usuarios inicialmente.
`
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	result, err := ext.Extract(transcriptPath)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ProjectDefinition == "" {
		t.Error("expected non-empty project definition")
	}
	if result.Metadata.OverallConfidence == 0 {
		t.Error("expected non-zero confidence")
	}
	if result.Metadata.Source != transcriptPath {
		t.Errorf("expected source %q, got %q", transcriptPath, result.Metadata.Source)
	}

	// Verify LLM was called
	if mock.CallCount == 0 {
		t.Error("expected LLM to be called")
	}
}

func TestExtract_NonExistentFile(t *testing.T) {
	mock := llm.NewMockClient("response")
	ext := NewTranscriptExtractorWithClient(mock, nil, nil)

	_, err := ext.Extract("/nonexistent/transcript.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExtract_FallbackPrompt(t *testing.T) {
	// First response is invalid JSON, second (simplified) is valid
	invalidJSON := "This is not valid JSON at all"
	validJSON := `{
		"context": "Sistema basico",
		"problem": "Precisa de organizacao",
		"objectives": ["Criar sistema"],
		"volumetry": {},
		"stack": [],
		"nfrs": [],
		"overall_confidence": 0.6,
		"section_confidence": {"context": 0.7, "problem": 0.6, "volumetry": 0.0, "stack": 0.0, "nfrs": 0.0},
		"speakers": [],
		"warnings": ["Baixa confianca"]
	}`

	mock := llm.NewMockClient(invalidJSON, validJSON)
	ext := NewTranscriptExtractorWithClient(mock, nil, nil)

	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.md")
	if err := os.WriteFile(transcriptPath, []byte("Reuniao simples"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	result, err := ext.Extract(transcriptPath)
	if err != nil {
		t.Fatalf("Extract with fallback failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should have called LLM twice (first failed JSON parse, fallback worked)
	if mock.CallCount != 2 {
		t.Errorf("expected 2 LLM calls (primary + fallback), got %d", mock.CallCount)
	}
}

func TestBuildContextInfo(t *testing.T) {
	mock := llm.NewMockClient()
	ext := NewTranscriptExtractorWithClient(mock, nil, nil)

	info := ext.buildContextInfo()

	// Should return some context info from pattern layers
	// Even without golden paths, it should return essentials
	if info == "" {
		t.Error("expected non-empty context info")
	}
}

func TestGenerateMarkdown(t *testing.T) {
	mock := llm.NewMockClient()
	ext := NewTranscriptExtractorWithClient(mock, nil, nil)

	data := &ExtractedData{
		Context:   "Sistema de tarefas",
		Problem:   "Precisamos organizar trabalho",
		Objectives: []string{"Criar sistema", "Automatizar"},
		Stack: []TechMention{
			{Name: "Go", Confidence: 0.9, Source: "explicit"},
			{Name: "Redis", Confidence: 0.7, Source: "inferred", Rationale: "Caching"},
		},
		NFRs: []string{"99% uptime"},
		SectionConfidence: map[string]float64{
			"context":   0.9,
			"problem":   0.8,
			"volumetry": 0.0,
			"stack":     0.85,
			"nfrs":      0.7,
		},
		Warnings:         []string{"Revisar volumetria"},
		OverallConfidence: 0.8,
	}

	md := ext.generateMarkdown(data)

	if md == "" {
		t.Fatal("expected non-empty markdown")
	}
	if !contains(md, "Contexto") {
		t.Error("markdown should contain Contexto section")
	}
	if !contains(md, "Stack Técnico") {
		t.Error("markdown should contain Stack section")
	}
	if !contains(md, "Go") {
		t.Error("markdown should mention Go")
	}
	if !contains(md, "Redis") {
		t.Error("markdown should mention Redis")
	}
	if !contains(md, "Confirmado") {
		t.Error("markdown should have 'Confirmado' section for explicit techs")
	}
	if !contains(md, "Sugerido") {
		t.Error("markdown should have 'Sugerido' section for inferred techs")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	expected := "test content"
	if err := os.WriteFile(filePath, []byte(expected), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	content, err := readFile(filePath)
	if err != nil {
		t.Fatalf("readFile failed: %v", err)
	}
	if content != expected {
		t.Errorf("expected %q, got %q", expected, content)
	}
}

func TestReadFile_NonExistent(t *testing.T) {
	_, err := readFile("/nonexistent/file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
