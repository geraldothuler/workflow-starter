package doccheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── extractMermaidBlock ────────────────────────────────────────────────────

func TestExtractMermaidBlock_found(t *testing.T) {
	content := "# README\n\n```mermaid\nflowchart LR\n    a --> b\n```\n\nrest"
	got := extractMermaidBlock(content)
	want := "flowchart LR\n    a --> b"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractMermaidBlock_notFound(t *testing.T) {
	got := extractMermaidBlock("# README\n\nno diagram here")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractMermaidBlock_empty(t *testing.T) {
	got := extractMermaidBlock("")
	if got != "" {
		t.Errorf("expected empty on empty input, got %q", got)
	}
}

// ── CheckUseCasesWithoutDefinition ─────────────────────────────────────────

func TestCheckUseCasesWithoutDefinition_allPresent(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "use-cases", "incident", "definition.yml"), "id: incident")
	createFile(t, filepath.Join(root, "use-cases", "postmortem", "definition.yml"), "id: postmortem")

	result := CheckUseCasesWithoutDefinition(root)
	if !result.Passed {
		t.Errorf("expected pass, got failure: %s", result.Detail)
	}
}

func TestCheckUseCasesWithoutDefinition_missing(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "use-cases", "incident", "definition.yml"), "id: incident")
	// postmortem dir exists but no definition.yml
	if err := os.MkdirAll(filepath.Join(root, "use-cases", "postmortem"), 0755); err != nil {
		t.Fatal(err)
	}

	result := CheckUseCasesWithoutDefinition(root)
	if result.Passed {
		t.Error("expected failure for missing definition.yml")
	}
	if !strings.Contains(result.Detail, "postmortem") {
		t.Errorf("detail should mention 'postmortem', got: %s", result.Detail)
	}
	if result.Check != "usecase-definition" {
		t.Errorf("wrong check name: %s", result.Check)
	}
}

func TestCheckUseCasesWithoutDefinition_noUseCasesDir(t *testing.T) {
	root := t.TempDir()
	result := CheckUseCasesWithoutDefinition(root)
	if !result.Passed {
		t.Errorf("expected pass when use-cases/ is absent, got: %s", result.Detail)
	}
}

// ── CheckLLMInOpsPackage ───────────────────────────────────────────────────

func TestCheckLLMInOpsPackage_clean(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "pkg", "ops", "probe.go"),
		`package ops

import "fmt"

func Probe() string { return fmt.Sprintf("ok") }
`)
	result := CheckLLMInOpsPackage(root)
	if !result.Passed {
		t.Errorf("expected pass for clean ops file, got: %s", result.Detail)
	}
}

func TestCheckLLMInOpsPackage_violation(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "pkg", "ops", "probe.go"),
		`package ops

import "github.com/Cobliteam/workflow-toolkit/pkg/llm"

func Probe() string { return llm.Generate("x") }
`)
	result := CheckLLMInOpsPackage(root)
	if result.Passed {
		t.Error("expected failure for LLM import in ops")
	}
	if !strings.Contains(result.Detail, "zero-llm-ops") {
		t.Errorf("detail should mention check name, got: %s", result.Detail)
	}
	if !strings.Contains(result.Detail, "probe.go") {
		t.Errorf("detail should mention offending file, got: %s", result.Detail)
	}
}

func TestCheckLLMInOpsPackage_overrideRespected(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "pkg", "ops", "probe.go"),
		`package ops

// wtb-noguard: zero-llm — this probe orchestrates LLM for plan generation, see ADR-007
import "github.com/Cobliteam/workflow-toolkit/pkg/llm"

func Probe() string { return llm.Generate("x") }
`)
	result := CheckLLMInOpsPackage(root)
	if !result.Passed {
		t.Errorf("expected pass when override marker present, got: %s", result.Detail)
	}
}

func TestCheckLLMInOpsPackage_noOpsDir(t *testing.T) {
	root := t.TempDir()
	result := CheckLLMInOpsPackage(root)
	if !result.Passed {
		t.Errorf("expected pass when pkg/ops/ is absent, got: %s", result.Detail)
	}
}

// ── CheckChainDiagramSync override ─────────────────────────────────────────

func TestCheckChainDiagramSync_noguardOverride(t *testing.T) {
	root := t.TempDir()
	// Minimal definition so LoadUseCases returns > 0 items
	createFile(t, filepath.Join(root, "use-cases", "incident", "definition.yml"),
		"id: incident\nname: Incident\ntype: documentary\nchain: []\n")

	// README has override marker but deliberately mismatched diagram
	readmeContent := "# Test\n\n<!-- wtb-noguard: chain-diagram-sync — simplified view for readability -->\n\n```mermaid\nflowchart TD\n    a --> b\n```\n"
	createFile(t, filepath.Join(root, "README.md"), readmeContent)

	result := CheckChainDiagramSync(root)
	if !result.Passed {
		t.Errorf("expected pass when noguard override present, got: %s", result.Detail)
	}
}

// ── RunAll ─────────────────────────────────────────────────────────────────

func TestRunAll_returnsAllResults(t *testing.T) {
	root := t.TempDir()
	results := RunAll(root)
	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}
	checks := []string{
		"chain-diagram-sync", "zero-llm-ops", "usecase-definition",
		"anonymization-in-docs", "docs-html-standard", "docs-html-links",
		"memory-index-bloat", "memory-content-leak",
		"memory-index-orphan", "context-json-drift",
	}
	for i, want := range checks {
		if results[i].Check != want {
			t.Errorf("results[%d].Check = %q, want %q", i, results[i].Check, want)
		}
	}
}

// ── CheckMemoryIndexBloat / CheckMemoryContentLeak ────────────────────────

func TestCheckMemoryIndexBloat_noFile(t *testing.T) {
	// When MEMORY.md is not found (CI / different env), must pass gracefully.
	result := CheckMemoryIndexBloat()
	if result.Check != "memory-index-bloat" {
		t.Errorf("wrong check name: %s", result.Check)
	}
	// In CI there is no ~/.claude/projects/*workflow*/memory/MEMORY.md, so it passes.
	// If running locally and MEMORY.md exists, it must also pass (≤150 lines enforced).
}

func TestCheckMemoryContentLeak_noFile(t *testing.T) {
	result := CheckMemoryContentLeak()
	if result.Check != "memory-content-leak" {
		t.Errorf("wrong check name: %s", result.Check)
	}
}

// ── CheckDocsHtmlStandard ──────────────────────────────────────────────────

func TestCheckDocsHtmlStandard_pass(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "index.html"),
		`<a href="report.html">Report</a>`)
	createFile(t, filepath.Join(root, "docs", "report.html"),
		`<div class="footer"><a href="index.html">← Todos os documentos</a></div>`)

	result := CheckDocsHtmlStandard(root)
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Detail)
	}
}

func TestCheckDocsHtmlStandard_missingBreadcrumb(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "index.html"),
		`<a href="report.html">Report</a>`)
	createFile(t, filepath.Join(root, "docs", "report.html"),
		`<div class="footer">No link back</div>`)

	result := CheckDocsHtmlStandard(root)
	if result.Passed {
		t.Error("expected failure for missing breadcrumb")
	}
	if !strings.Contains(result.Detail, "breadcrumb") {
		t.Errorf("detail should mention breadcrumb, got: %s", result.Detail)
	}
}

func TestCheckDocsHtmlStandard_missingFromIndex(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "index.html"), `<p>no cards</p>`)
	createFile(t, filepath.Join(root, "docs", "report.html"),
		`<a href="index.html">← Todos os documentos</a>`)

	result := CheckDocsHtmlStandard(root)
	if result.Passed {
		t.Error("expected failure for doc not listed in index")
	}
	if !strings.Contains(result.Detail, "index.html") {
		t.Errorf("detail should mention index.html, got: %s", result.Detail)
	}
}

func TestCheckDocsHtmlStandard_overrideRespected(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "index.html"), `<p>no cards</p>`)
	createFile(t, filepath.Join(root, "docs", "draft.html"),
		`<!-- wtb-noguard: docs-html-standard — rascunho interno --><p>draft</p>`)

	result := CheckDocsHtmlStandard(root)
	if !result.Passed {
		t.Errorf("expected pass when override present, got: %s", result.Detail)
	}
}

func TestCheckDocsHtmlStandard_noDocs(t *testing.T) {
	root := t.TempDir()
	result := CheckDocsHtmlStandard(root)
	if !result.Passed {
		t.Errorf("expected pass when docs/ absent, got: %s", result.Detail)
	}
}

// ── CheckDocsHtmlLinks ─────────────────────────────────────────────────────

func TestCheckDocsHtmlLinks_pass(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "index.html"), `<p>home</p>`)
	createFile(t, filepath.Join(root, "docs", "report.html"),
		`<a href="index.html">home</a> <a href="https://github.com/x">external</a>`)

	result := CheckDocsHtmlLinks(root)
	if !result.Passed {
		t.Errorf("expected pass for valid links, got: %s", result.Detail)
	}
}

func TestCheckDocsHtmlLinks_brokenInternal(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "report.html"),
		`<a href="missing-page.html">broken</a>`)

	result := CheckDocsHtmlLinks(root)
	if result.Passed {
		t.Error("expected failure for broken internal link")
	}
	if !strings.Contains(result.Detail, "missing-page.html") {
		t.Errorf("detail should mention broken file, got: %s", result.Detail)
	}
}

func TestCheckDocsHtmlLinks_relativeMD(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "report.html"),
		`<a href="../README.md">readme</a>`)

	result := CheckDocsHtmlLinks(root)
	if result.Passed {
		t.Error("expected failure for relative .md link")
	}
	if !strings.Contains(result.Detail, ".md") {
		t.Errorf("detail should mention .md, got: %s", result.Detail)
	}
}

func TestCheckDocsHtmlLinks_absoluteMDAllowed(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "report.html"),
		`<a href="https://github.com/Cobliteam/workflow-toolkit/blob/main/README.md">readme</a>`)

	result := CheckDocsHtmlLinks(root)
	if !result.Passed {
		t.Errorf("expected pass for absolute GitHub MD link, got: %s", result.Detail)
	}
}

func TestCheckDocsHtmlLinks_skipAnchorsAndMailto(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "report.html"),
		`<a href="#section">anchor</a> <a href="mailto:x@y.com">mail</a>`)

	result := CheckDocsHtmlLinks(root)
	if !result.Passed {
		t.Errorf("expected pass for anchors and mailto, got: %s", result.Detail)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func createFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
