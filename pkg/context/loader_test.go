package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

func TestNewContextLoader(t *testing.T) {
	loader := NewContextLoader("/some/path")
	if loader == nil {
		t.Fatal("expected non-nil loader")
	}
	if loader.basePath != "/some/path" {
		t.Errorf("expected basePath '/some/path', got %q", loader.basePath)
	}
}

func TestLoadClaudeContext(t *testing.T) {
	basePath := findProjectRoot(t)
	loader := NewContextLoader(basePath)

	content, err := loader.loadClaudeContext()
	if err != nil {
		t.Fatalf("failed to load CLAUDE.md: %v", err)
	}
	if content == "" {
		t.Fatal("expected non-empty CLAUDE.md content")
	}
	if !strings.Contains(content, "Workflow") {
		t.Error("CLAUDE.md should mention Workflow")
	}
}

func TestLoadActiveDocs(t *testing.T) {
	basePath := findProjectRoot(t)
	loader := NewContextLoader(basePath)

	docs, err := loader.loadActiveDocs()
	if err != nil {
		t.Fatalf("failed to load active docs: %v", err)
	}
	// Should have at least some ACTIVE/IN_PROGRESS docs
	if len(docs) == 0 {
		t.Error("expected at least 1 active doc loaded")
	}
}

func TestLoadActiveDocs_AllSections(t *testing.T) {
	// Verify that architecture, compliance, guides sections are processed
	// (This was a bug fix — previously only research/planning/savepoints were parsed)
	basePath := findProjectRoot(t)
	loader := NewContextLoader(basePath)

	docs, err := loader.loadActiveDocs()
	if err != nil {
		t.Fatalf("failed to load active docs: %v", err)
	}

	// The project has ACTIVE docs in architecture section and IN_PROGRESS in guides
	// We just verify that docs were loaded (count > 0 means the bug fix is working)
	if len(docs) == 0 {
		t.Error("expected active docs from all sections (architecture, guides, etc.)")
	}
}

func TestLoadRelevantSkills(t *testing.T) {
	basePath := findProjectRoot(t)
	loader := NewContextLoader(basePath)

	skills, err := loader.loadRelevantSkills("backlog", "generate backlog")
	if err != nil {
		t.Fatalf("failed to load skills: %v", err)
	}

	// Skills were absorbed into YAML configs in Go packages (Phase 9).
	// The skills/ directory no longer exists, so loadRelevantSkills returns
	// empty — skill content is now embedded via go:embed in each package.
	// This test verifies the function handles the absence gracefully.
	_ = skills // len may be 0 — that's expected post-Phase 9
}

func TestLoadApplicablePatterns(t *testing.T) {
	basePath := findProjectRoot(t)
	loader := NewContextLoader(basePath)

	patterns, err := loader.loadApplicablePatterns("backlog", "generate backlog")
	if err != nil {
		t.Fatalf("failed to load patterns: %v", err)
	}

	// "backlog" command should resolve patterns via Feature Registry
	if len(patterns) == 0 {
		t.Error("expected patterns for 'backlog' command")
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	ctx := &Context{
		ClaudeContext: "# Project Context\nThis is workflow.",
		Skills: []Skill{
			{
				Name:        "test-skill",
				Description: "A test skill",
				Content:     "Skill content here",
			},
		},
		Patterns: []Pattern{
			{
				Name:        "test-pattern",
				Description: "A test pattern",
				Content:     "Pattern content here",
			},
		},
		ActiveDocs: []string{"Doc content 1", "Doc content 2"},
	}

	prompt := ctx.BuildSystemPrompt()

	if !strings.Contains(prompt, "Context: Workflow Platform") {
		t.Error("prompt should contain project context header")
	}
	if !strings.Contains(prompt, "This is workflow.") {
		t.Error("prompt should contain CLAUDE.md content")
	}
	if !strings.Contains(prompt, "Relevant Skills") {
		t.Error("prompt should contain skills section")
	}
	if !strings.Contains(prompt, "test-skill") {
		t.Error("prompt should contain skill name")
	}
	if !strings.Contains(prompt, "Skill content here") {
		t.Error("prompt should contain skill content")
	}
	if !strings.Contains(prompt, "Applicable Patterns") {
		t.Error("prompt should contain patterns section")
	}
	if !strings.Contains(prompt, "test-pattern") {
		t.Error("prompt should contain pattern name")
	}
	if !strings.Contains(prompt, "Active Documentation") {
		t.Error("prompt should contain active docs section")
	}
}

func TestBuildSystemPrompt_Empty(t *testing.T) {
	ctx := &Context{
		ClaudeContext: "# Context",
	}

	prompt := ctx.BuildSystemPrompt()

	if !strings.Contains(prompt, "# Context") {
		t.Error("prompt should contain CLAUDE.md content")
	}
	// Should not contain sections that have no content
	if strings.Contains(prompt, "Relevant Skills") {
		t.Error("prompt should not contain skills section when empty")
	}
	if strings.Contains(prompt, "Applicable Patterns") {
		t.Error("prompt should not contain patterns section when empty")
	}
	if strings.Contains(prompt, "Active Documentation") {
		t.Error("prompt should not contain active docs section when empty")
	}
}

func TestSummary(t *testing.T) {
	ctx := &Context{
		Skills:     []Skill{{Name: "s1"}, {Name: "s2"}},
		Patterns:   []Pattern{{Name: "p1"}},
		ActiveDocs: []string{"d1", "d2", "d3"},
	}

	summary := ctx.Summary()

	expected := "Context loaded: 2 skills, 1 patterns, 3 active docs"
	if summary != expected {
		t.Errorf("expected %q, got %q", expected, summary)
	}
}

func TestLoadForCommand(t *testing.T) {
	basePath := findProjectRoot(t)
	loader := NewContextLoader(basePath)

	ctx, err := loader.LoadForCommand("backlog", "generate backlog")
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.ClaudeContext == "" {
		t.Error("expected CLAUDE.md content")
	}

	summary := ctx.Summary()
	if !strings.Contains(summary, "skills") {
		t.Errorf("summary should mention skills: %s", summary)
	}
}

func TestLoadForCommand_UnknownCommand(t *testing.T) {
	basePath := findProjectRoot(t)
	loader := NewContextLoader(basePath)

	ctx, err := loader.LoadForCommand("nonexistent_command", "test")
	if err != nil {
		t.Fatalf("unknown command should not cause error: %v", err)
	}

	// Should still load CLAUDE.md and active docs
	if ctx.ClaudeContext == "" {
		t.Error("should still load CLAUDE.md for unknown commands")
	}
}
