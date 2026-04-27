package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ── AllCommands ────────────────────────────────────────────────────────────────

func TestAllCommands_SingleCommand(t *testing.T) {
	s := StepSpec{Command: "plan-new"}
	got := s.AllCommands()
	if len(got) != 1 || got[0] != "plan-new" {
		t.Errorf("expected [plan-new], got %v", got)
	}
}

func TestAllCommands_CommandsSlice(t *testing.T) {
	s := StepSpec{Commands: []string{"auth", "db-health", "k8s-status"}}
	got := s.AllCommands()
	if len(got) != 3 {
		t.Errorf("expected 3 commands, got %d: %v", len(got), got)
	}
}

func TestAllCommands_CommandTakesPriority(t *testing.T) {
	// If both Command and Commands are set, Command wins.
	s := StepSpec{Command: "plan-new", Commands: []string{"auth", "db-health"}}
	got := s.AllCommands()
	if len(got) != 1 || got[0] != "plan-new" {
		t.Errorf("expected Command to take priority, got %v", got)
	}
}

func TestAllCommands_Empty(t *testing.T) {
	s := StepSpec{}
	got := s.AllCommands()
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestStepSpec_AllCommands_CommandSpecs(t *testing.T) {
	s := StepSpec{
		CommandSpecs: []CommandSpec{
			{Name: "auth"},
			{Name: "db-health"},
			{Name: "k8s-status"},
		},
	}
	got := s.AllCommands()
	if len(got) != 3 {
		t.Errorf("expected 3 commands from CommandSpecs, got %d: %v", len(got), got)
	}
	if got[0] != "auth" || got[1] != "db-health" || got[2] != "k8s-status" {
		t.Errorf("unexpected command names: %v", got)
	}
}

// ── DefaultRegistry keys ───────────────────────────────────────────────────────

func TestDefaultRegistry_HasExpectedEngines(t *testing.T) {
	reg := DefaultRegistry()
	want := []string{
		"pkg/extractor",
		"pkg/backlog",
		"pkg/techref",
		"pkg/infracontext",
		"pkg/render",
		"pkg/playbook",
		"pkg/ops",
	}
	for _, key := range want {
		if _, ok := reg[key]; !ok {
			t.Errorf("DefaultRegistry missing engine %q", key)
		}
	}
}

// ── opsPlanExecute ────────────────────────────────────────────────────────────

func TestOpsPlanExecute_NoPlanFile_ReturnsError(t *testing.T) {
	// RepoPath points to an empty temp dir → plan.md doesn't exist → error status.
	r := opsPlanExecute(RunInputs{}, RunOptions{RepoPath: t.TempDir()})
	if r.Status != "error" {
		t.Errorf("expected status 'error' when plan file missing, got %q", r.Status)
	}
}

func TestOpsPlanExecute_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	planPath := dir + "/.workflow/ops/plan.md"
	if err := os.MkdirAll(dir+"/.workflow/ops", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte("not: valid: yaml: ["), 0644); err != nil {
		t.Fatal(err)
	}
	r := opsPlanExecute(RunInputs{}, RunOptions{RepoPath: dir})
	if r.Status != "error" {
		t.Errorf("expected status 'error' for invalid YAML, got %q", r.Status)
	}
}

// ── opsEngine ─────────────────────────────────────────────────────────────────

func TestOpsEngine_NoCommands_ReturnsError(t *testing.T) {
	step := StepSpec{Name: "probe", Engine: "pkg/ops"} // no Command or Commands
	result := opsEngine(step, RunInputs{}, RunOptions{})
	if result.Error == nil {
		t.Error("expected error when no commands specified")
	}
}

func TestOpsEngine_UnknownCommand_NoStepError(t *testing.T) {
	// An unknown command causes opsEngine to return an OpsResult with Status "error",
	// but does NOT set StepResult.Error (so the step itself doesn't fail the pipeline).
	step := StepSpec{Name: "probe", Engine: "pkg/ops", Command: "unknown-cmd"}
	result := opsEngine(step, RunInputs{}, RunOptions{})
	if result.Error != nil {
		t.Errorf("unknown command should not fail the step, got: %v", result.Error)
	}
}

// ── playbookEngine ────────────────────────────────────────────────────────────

func TestPlaybookEngine_MissingInput(t *testing.T) {
	step := StepSpec{Name: "run_playbook", Engine: "pkg/playbook"}
	// No "playbook" input → engine must return error.
	result := playbookEngine(step, RunInputs{}, RunOptions{})
	if result.Error == nil {
		t.Error("expected error when 'playbook' input is missing")
	}
}

// ── extractorEngine ───────────────────────────────────────────────────────────

func TestExtractorEngine_MissingInput(t *testing.T) {
	step := StepSpec{Name: "extract_spec", Engine: "pkg/extractor"}
	result := extractorEngine(step, RunInputs{}, RunOptions{})
	if result.Error == nil {
		t.Error("expected error when 'narrative' input is missing")
	}
}

func TestExtractorEngine_FileNotFound(t *testing.T) {
	step := StepSpec{Name: "extract_spec", Engine: "pkg/extractor"}
	result := extractorEngine(step, RunInputs{"narrative": "/nonexistent/file.md"}, RunOptions{})
	if result.Error == nil {
		t.Error("expected error when narrative file does not exist")
	}
}

// ── backlogEngine ─────────────────────────────────────────────────────────────

func TestBacklogEngine_MissingSpec(t *testing.T) {
	step := StepSpec{Name: "generate_backlog", Engine: "pkg/backlog"}
	// No spec-file and no repoPath → spec.json not found → error.
	result := backlogEngine(step, RunInputs{}, RunOptions{RepoPath: t.TempDir()})
	if result.Error == nil {
		t.Error("expected error when spec JSON does not exist")
	}
}

// ── coalesce helper ───────────────────────────────────────────────────────────

func TestCoalesce_FirstNonEmpty(t *testing.T) {
	if got := coalesce("", "", "third"); got != "third" {
		t.Errorf("expected 'third', got %q", got)
	}
}

func TestCoalesce_AllEmpty(t *testing.T) {
	if got := coalesce("", "", ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestCoalesce_FirstWins(t *testing.T) {
	if got := coalesce("first", "second"); got != "first" {
		t.Errorf("expected 'first', got %q", got)
	}
}

// ── splitCSV helper ───────────────────────────────────────────────────────────

func TestSplitCSV_Multiple(t *testing.T) {
	got := splitCSV("kafka, lock, oom")
	if len(got) != 3 || got[0] != "kafka" || got[1] != "lock" || got[2] != "oom" {
		t.Errorf("unexpected splitCSV result: %v", got)
	}
}

func TestSplitCSV_Single(t *testing.T) {
	got := splitCSV("all")
	if len(got) != 1 || got[0] != "all" {
		t.Errorf("expected [all], got %v", got)
	}
}

func TestSplitCSV_Empty(t *testing.T) {
	got := splitCSV("")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// ── resolveOutput helper ──────────────────────────────────────────────────────

func TestResolveOutput_WithRepoPath(t *testing.T) {
	got := resolveOutput(".workflow/ops/plan.md", RunOptions{RepoPath: "/home/user/repo"})
	want := "/home/user/repo/.workflow/ops/plan.md"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveOutput_NoRepoPath(t *testing.T) {
	got := resolveOutput(".workflow/ops/plan.md", RunOptions{})
	if got != ".workflow/ops/plan.md" {
		t.Errorf("expected path unchanged, got %q", got)
	}
}

func TestResolveOutput_NonWorkflowPath(t *testing.T) {
	got := resolveOutput("docs/workflow/backlog.md", RunOptions{RepoPath: "/repo"})
	if got != "docs/workflow/backlog.md" {
		t.Errorf("expected path unchanged for non-.workflow/ prefix, got %q", got)
	}
}

// ── techrefEngine ─────────────────────────────────────────────────────────────

func TestTechrefEngine_MissingBacklog(t *testing.T) {
	step := StepSpec{Name: "generate_deep_dives", Engine: "pkg/techref"}
	result := techrefEngine(step, RunInputs{}, RunOptions{RepoPath: t.TempDir()})
	if result.Error == nil {
		t.Error("expected error when backlog file is missing")
	}
}

func TestTechrefEngine_TemplateMode(t *testing.T) {
	dir := t.TempDir()

	// Write a minimal backlog with a known technology.
	bl := types.Backlog{
		Meta: types.Metadata{TotalEpics: 1, TotalStories: 1},
		Epics: []types.Epic{
			{
				ID:    "E1",
				Title: "Auth System",
				Stories: []types.Story{
					{ID: "E1.1", Title: "Login", What: "Implement PostgreSQL connection pool"},
				},
			},
		},
	}
	data, _ := json.Marshal(bl)
	backlogPath := filepath.Join(dir, ".workflow", "backlog.json")
	os.MkdirAll(filepath.Dir(backlogPath), 0755)
	os.WriteFile(backlogPath, data, 0644)

	outDir := filepath.Join(dir, ".workflow", "deep-dives")
	step := StepSpec{
		Name:   "generate_deep_dives",
		Engine: "pkg/techref",
		Output: ".workflow/deep-dives/",
	}
	result := techrefEngine(step, RunInputs{"mode": "template"}, RunOptions{RepoPath: dir})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	// Verify output file was created.
	outFile := filepath.Join(outDir, "deep-dives.json")
	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		t.Errorf("expected output file at %s", outFile)
	}
}

// ── renderEngine ──────────────────────────────────────────────────────────────

func TestRenderEngine_MissingBacklog(t *testing.T) {
	step := StepSpec{Name: "visualize", Engine: "pkg/render"}
	result := renderEngine(step, RunInputs{}, RunOptions{RepoPath: t.TempDir()})
	if result.Error == nil {
		t.Error("expected error when backlog file is missing")
	}
}

func TestRenderEngine_MarkdownFormat(t *testing.T) {
	dir := t.TempDir()

	bl := types.Backlog{
		Meta: types.Metadata{TotalEpics: 1, TotalStories: 1, ProjectTitle: "Test Project"},
		Epics: []types.Epic{
			{ID: "E1", Title: "Epic One", Stories: []types.Story{
				{ID: "E1.1", Title: "Story One", What: "Do something"},
			}},
		},
	}
	data, _ := json.Marshal(bl)
	backlogPath := filepath.Join(dir, ".workflow", "backlog.json")
	os.MkdirAll(filepath.Dir(backlogPath), 0755)
	os.WriteFile(backlogPath, data, 0644)

	outDir := filepath.Join(dir, ".workflow", "render")
	step := StepSpec{
		Name:   "visualize",
		Engine: "pkg/render",
		Output: ".workflow/render/",
	}
	result := renderEngine(step, RunInputs{"format": "markdown"}, RunOptions{RepoPath: dir})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	mdPath := filepath.Join(outDir, "backlog.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Errorf("expected markdown file at %s", mdPath)
	}
}

// ── infracontextEngine ────────────────────────────────────────────────────────

func TestInfracontextEngine_NoProviders(t *testing.T) {
	// On a CI/dev machine without kubectl/kafka/psql, engine should gracefully skip.
	step := StepSpec{Name: "enrich_infra", Engine: "pkg/infracontext"}
	result := infracontextEngine(step, RunInputs{}, RunOptions{RepoPath: t.TempDir()})
	// Should NOT return error — graceful skip is expected.
	if result.Error != nil {
		t.Errorf("expected graceful skip, got error: %v", result.Error)
	}
}
