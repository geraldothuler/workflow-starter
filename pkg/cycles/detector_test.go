package cycles

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseGitStatFiles(t *testing.T) {
	stat := ` pkg/cycles/detector.go | 120 +++++++++
 pkg/cycles/types.go    |  25 ++
 cmd/wtb/main.go        |   3 +-
 3 files changed, 148 insertions(+)
`
	files := parseGitStatFiles(stat)
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(files), files)
	}
	expected := []string{"pkg/cycles/detector.go", "pkg/cycles/types.go", "cmd/wtb/main.go"}
	for i, f := range expected {
		if files[i] != f {
			t.Errorf("file[%d]: expected %q, got %q", i, f, files[i])
		}
	}
}

func TestUniquePackages(t *testing.T) {
	files := []string{
		"pkg/cycles/detector.go",
		"pkg/cycles/types.go",
		"cmd/wtb/main.go",
		"README.md",
	}
	pkgs := uniquePackages(files)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %v", len(pkgs), pkgs)
	}
	// Order: pkg/cycles first, then cmd/wtb
	expected := map[string]bool{"pkg/cycles": true, "cmd/wtb": true}
	for _, p := range pkgs {
		if !expected[p] {
			t.Errorf("unexpected package: %q", p)
		}
	}
}

func TestCountTestPackages(t *testing.T) {
	output := `ok  	github.com/Cobliteam/workflow-toolkit/pkg/cycles	0.003s
ok  	github.com/Cobliteam/workflow-toolkit/pkg/ops	0.021s
ok  	github.com/Cobliteam/workflow-toolkit/pkg/runner	0.015s
`
	result := countTestPackages(output)
	if result != "3 packages ok" {
		t.Errorf("expected '3 packages ok', got %q", result)
	}
}

func TestCountTestPackagesEmpty(t *testing.T) {
	result := countTestPackages("")
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

func TestLoadCycleConfig(t *testing.T) {
	cfg, err := LoadCycleConfig("")
	if err != nil {
		t.Fatalf("LoadCycleConfig failed: %v", err)
	}
	if cfg.Cycle.Threshold != 6 {
		t.Errorf("expected threshold 6, got %d", cfg.Cycle.Threshold)
	}
	if len(cfg.Cycle.Signals) != 5 {
		t.Errorf("expected 5 signals, got %d", len(cfg.Cycle.Signals))
	}
	if cfg.Cycle.Savepoint.Dir != ".workflow/savepoints" {
		t.Errorf("expected savepoint dir '.workflow/savepoints', got %q", cfg.Cycle.Savepoint.Dir)
	}
}

func TestLoadCycleConfigOverride(t *testing.T) {
	tmp := t.TempDir()
	overrideDir := filepath.Join(tmp, ".workflow")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}
	override := `cycle:
  threshold: 8
`
	if err := os.WriteFile(filepath.Join(overrideDir, "cycle_rules.yml"), []byte(override), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadCycleConfig(tmp)
	if err != nil {
		t.Fatalf("LoadCycleConfig failed: %v", err)
	}
	if cfg.Cycle.Threshold != 8 {
		t.Errorf("expected overridden threshold 8, got %d", cfg.Cycle.Threshold)
	}
}

func TestEvalTimeElapsedNoSavepoints(t *testing.T) {
	tmp := t.TempDir()
	sig := evalTimeElapsed(tmp, ".workflow/savepoints", 30, 1)
	if !sig.Passed {
		t.Error("expected passed=true when no savepoints exist")
	}
	if sig.Detail != "no prior savepoints" {
		t.Errorf("unexpected detail: %q", sig.Detail)
	}
}

func TestEvalTimeElapsedRecent(t *testing.T) {
	tmp := t.TempDir()
	markerDir := filepath.Join(tmp, ".workflow")
	if err := os.MkdirAll(markerDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create the last-savepoint marker (written by wtb cycle-check --save)
	markerPath := filepath.Join(markerDir, "last-savepoint")
	if err := os.WriteFile(markerPath, []byte("savepoint-recent"), 0644); err != nil {
		t.Fatal(err)
	}
	// Touch it to now
	now := time.Now()
	if err := os.Chtimes(markerPath, now, now); err != nil {
		t.Fatal(err)
	}

	sig := evalTimeElapsed(tmp, ".workflow/savepoints", 30, 1)
	if sig.Passed {
		t.Error("expected passed=false for recent savepoint (< 30 min)")
	}
}

func TestEvalTimeElapsedOld(t *testing.T) {
	tmp := t.TempDir()
	markerDir := filepath.Join(tmp, ".workflow")
	if err := os.MkdirAll(markerDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create the last-savepoint marker with mtime 45 minutes ago
	markerPath := filepath.Join(markerDir, "last-savepoint")
	if err := os.WriteFile(markerPath, []byte("savepoint-old"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-45 * time.Minute)
	if err := os.Chtimes(markerPath, old, old); err != nil {
		t.Fatal(err)
	}

	sig := evalTimeElapsed(tmp, ".workflow/savepoints", 30, 1)
	if !sig.Passed {
		t.Error("expected passed=true for old savepoint (> 30 min)")
	}
}

func TestRenderSavepoint(t *testing.T) {
	result := CycleResult{
		Score:     9,
		MaxScore:  10,
		Threshold: 6,
		Signals: []Signal{
			{Name: "git_changes", Passed: true, Weight: 2, Detail: "3 files changed"},
			{Name: "tests_pass", Passed: true, Weight: 3, Detail: "39 packages ok"},
			{Name: "build_pass", Passed: true, Weight: 3, Detail: "compiled"},
			{Name: "time_elapsed", Passed: false, Weight: 1, Detail: "12min (threshold: 30min)"},
			{Name: "packages_touched", Passed: true, Weight: 1, Detail: "2 packages"},
		},
		FilesChanged:    []string{"pkg/cycles/detector.go", "cmd/wtb/main.go"},
		PackagesTouched: []string{"pkg/cycles", "cmd/wtb"},
	}

	now := time.Date(2026, 2, 24, 14, 30, 22, 0, time.UTC)
	content, err := renderSavepoint(result, now)
	if err != nil {
		t.Fatalf("renderSavepoint failed: %v", err)
	}

	// Check key fields in output
	checks := []string{
		"type: dev-cycle",
		"date: 2026-02-24",
		"time: 14:30:22",
		"score: 9/10",
		"threshold: 6",
		"pass git_changes",
		"fail time_elapsed",
		"pkg/cycles/detector.go",
	}
	for _, c := range checks {
		if !contains(content, c) {
			t.Errorf("savepoint missing %q", c)
		}
	}
}

func TestWriteSavepoint(t *testing.T) {
	tmp := t.TempDir()
	result := CycleResult{
		Score:     9,
		MaxScore:  10,
		Threshold: 6,
		Signals: []Signal{
			{Name: "git_changes", Passed: true, Weight: 2, Detail: "3 files"},
		},
		FilesChanged: []string{"a.go"},
	}

	path, err := WriteSavepoint(tmp, result)
	if err != nil {
		t.Fatalf("WriteSavepoint failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("savepoint file not created: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "type: dev-cycle") {
		t.Error("savepoint missing frontmatter")
	}
}

func TestMergeCycleConfig(t *testing.T) {
	base := &CycleConfig{}
	base.Cycle.Threshold = 6
	base.Cycle.Savepoint.Dir = ".workflow/savepoints"
	base.Cycle.Savepoint.Format = "savepoint-2006-01-02.md"

	override := &CycleConfig{}
	override.Cycle.Threshold = 8
	override.Cycle.Savepoint.Dir = "custom/dir"

	mergeCycleConfig(base, override)

	if base.Cycle.Threshold != 8 {
		t.Errorf("expected threshold 8, got %d", base.Cycle.Threshold)
	}
	if base.Cycle.Savepoint.Dir != "custom/dir" {
		t.Errorf("expected dir 'custom/dir', got %q", base.Cycle.Savepoint.Dir)
	}
	// Format should remain from base since override was empty
	if base.Cycle.Savepoint.Format != "savepoint-2006-01-02.md" {
		t.Errorf("expected format preserved, got %q", base.Cycle.Savepoint.Format)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsString(s, substr)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
