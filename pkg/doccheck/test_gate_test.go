package doccheck

import (
	"testing"
)

func TestLoadTestGateRules(t *testing.T) {
	rules, err := LoadTestGateRules()
	if err != nil {
		t.Fatalf("LoadTestGateRules() error: %v", err)
	}
	if rules == nil {
		t.Fatal("LoadTestGateRules() returned nil")
	}
	if len(rules.TestGate.Steps) == 0 {
		t.Error("expected non-empty steps")
	}
	if len(rules.TestGate.PackageClassification) != 3 {
		t.Errorf("expected 3 classifications, got %d", len(rules.TestGate.PackageClassification))
	}
	if len(rules.TestGate.Rules) == 0 {
		t.Error("expected non-empty rules")
	}
}

func TestClassifyPackage(t *testing.T) {
	rules, err := LoadTestGateRules()
	if err != nil {
		t.Fatalf("LoadTestGateRules() error: %v", err)
	}

	tests := []struct {
		pkgPath string
		want    string
	}{
		{"pkg/types/", "testable"},
		{"pkg/parser/", "testable"},
		{"pkg/backlog/", "testable"},
		{"pkg/doccheck/", "testable"},
		{"pkg/scaffold/", "testable"},
		{"pkg/compliance/", "partial"},
		{"pkg/render/", "partial"},
		{"pkg/journey/", "partial"},
		{"pkg/mcp/", "partial"},
		{"pkg/llm/", "io_limited"},
		{"pkg/ui/", "io_limited"},
		{"pkg/server/", "io_limited"},
		{"pkg/transport/", "io_limited"},
		{"pkg/unknown/", "testable"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.pkgPath, func(t *testing.T) {
			got := ClassifyPackage(tt.pkgPath, rules)
			if got != tt.want {
				t.Errorf("ClassifyPackage(%q) = %q, want %q", tt.pkgPath, got, tt.want)
			}
		})
	}
}

func TestCheckTestGate(t *testing.T) {
	rules, err := LoadTestGateRules()
	if err != nil {
		t.Fatalf("LoadTestGateRules() error: %v", err)
	}

	changedFiles := []string{
		"pkg/types/story.go",
		"pkg/types/story_test.go", // should be skipped (test file)
		"pkg/compliance/consent.go",
		"pkg/llm/client.go",       // io_limited — no gap
		"pkg/parser/markdown.go",
		"README.md",               // non-Go — should be skipped
	}

	gaps := CheckTestGate(changedFiles, rules)

	// Expect gaps for: pkg/types/, pkg/compliance/, pkg/parser/
	// NOT for: pkg/llm/ (io_limited), test files, non-Go files
	if len(gaps) != 3 {
		t.Fatalf("expected 3 gaps, got %d: %+v", len(gaps), gaps)
	}

	// Build a map for easier assertion
	gapMap := make(map[string]TestGap)
	for _, g := range gaps {
		gapMap[g.Package] = g
	}

	if g, ok := gapMap["pkg/types/"]; !ok {
		t.Error("expected gap for pkg/types/")
	} else if g.Classification != "testable" {
		t.Errorf("pkg/types/ classification = %q, want %q", g.Classification, "testable")
	}

	if g, ok := gapMap["pkg/compliance/"]; !ok {
		t.Error("expected gap for pkg/compliance/")
	} else if g.Classification != "partial" {
		t.Errorf("pkg/compliance/ classification = %q, want %q", g.Classification, "partial")
	}

	if g, ok := gapMap["pkg/parser/"]; !ok {
		t.Error("expected gap for pkg/parser/")
	} else if g.Classification != "testable" {
		t.Errorf("pkg/parser/ classification = %q, want %q", g.Classification, "testable")
	}

	if _, ok := gapMap["pkg/llm/"]; ok {
		t.Error("pkg/llm/ should not produce a gap (io_limited)")
	}
}

func TestCheckTestGateDeduplication(t *testing.T) {
	rules, err := LoadTestGateRules()
	if err != nil {
		t.Fatalf("LoadTestGateRules() error: %v", err)
	}

	changedFiles := []string{
		"pkg/types/story.go",
		"pkg/types/epic.go",
		"pkg/types/backlog.go",
	}

	gaps := CheckTestGate(changedFiles, rules)
	if len(gaps) != 1 {
		t.Errorf("expected 1 gap (deduplicated), got %d: %+v", len(gaps), gaps)
	}
}

func TestCheckTestGateEmpty(t *testing.T) {
	rules, err := LoadTestGateRules()
	if err != nil {
		t.Fatalf("LoadTestGateRules() error: %v", err)
	}

	gaps := CheckTestGate(nil, rules)
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for nil input, got %d", len(gaps))
	}

	gaps = CheckTestGate([]string{}, rules)
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for empty input, got %d", len(gaps))
	}
}
