package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ── E2E Fixture Helpers ─────────────────────────────────────────────────────

// backlogFixture creates a realistic Backlog with technologies that have
// pre-defined templates in pkg/techref (Go, PostgreSQL, Docker).
// This allows the techref engine to generate real deep dives in template mode.
func backlogFixture() types.Backlog {
	return types.Backlog{
		Meta: types.Metadata{
			GeneratedAt:  "2026-02-23T10:00:00Z",
			Provider:     "fixture",
			InputFile:    "fixture-narrative.md",
			ProjectTitle: "E2E Test Project",
			Lang:         "pt-BR",
			TotalEpics:   2,
			TotalStories: 3,
		},
		Epics: []types.Epic{
			{
				ID:          "E1",
				Code:        "E1",
				Title:       "API Gateway",
				Description: "Implement the main API gateway using Go and PostgreSQL",
				Stories: []types.Story{
					{
						ID:     "E1.S1",
						EpicID: "E1",
						Title:  "Database Connection Pool",
						What:   "Set up PostgreSQL connection pooling with health checks",
						Why:    "Reliable database access under load",
						AcceptanceCriteria: []string{
							"Pool configured with pgbouncer",
							"Health check endpoint verifies DB connectivity",
						},
						Effort: 5,
					},
					{
						ID:     "E1.S2",
						EpicID: "E1",
						Title:  "HTTP Router",
						What:   "Implement Go HTTP router with middleware",
						Why:    "Clean API routing with auth middleware",
						AcceptanceCriteria: []string{
							"RESTful routes registered",
							"Auth middleware applied to protected endpoints",
						},
						Effort: 3,
					},
				},
			},
			{
				ID:          "E2",
				Code:        "E2",
				Title:       "Containerization",
				Description: "Docker-based deployment pipeline",
				Stories: []types.Story{
					{
						ID:     "E2.S1",
						EpicID: "E2",
						Title:  "Multi-stage Build",
						What:   "Create Docker multi-stage build for Go service",
						Why:    "Minimal production images",
						AcceptanceCriteria: []string{
							"Build stage compiles Go binary",
							"Runtime stage uses distroless base",
							"Image size under 50MB",
						},
						Effort: 3,
					},
				},
			},
		},
	}
}

// writeBacklogFixture writes the backlog JSON to the conventional path.
func writeBacklogFixture(t *testing.T, repoDir string, bl types.Backlog) string {
	t.Helper()
	data, err := json.MarshalIndent(bl, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal backlog fixture: %v", err)
	}
	backlogPath := filepath.Join(repoDir, ".workflow", "backlog.json")
	if err := os.MkdirAll(filepath.Dir(backlogPath), 0755); err != nil {
		t.Fatalf("failed to create backlog dir: %v", err)
	}
	if err := os.WriteFile(backlogPath, data, 0644); err != nil {
		t.Fatalf("failed to write backlog fixture: %v", err)
	}
	return backlogPath
}

// backlogDefinitionYAML is the pipeline definition for the backlog use-case,
// identical to use-cases/backlog/definition.yml but with engines wired to
// our hybrid registry (mock LLM steps + real heuristic steps).
const backlogE2EDefinitionYAML = `
id: backlog
type: pipeline
name: Technical Backlog Generation
version: "1.0"

inputs:
  - name: narrative
    description: Input narrative file
    required: true
  - name: provider
    description: LLM provider
    required: false
    default: claude

steps:
  - name: extract_spec
    engine: pkg/extractor
    description: Extract structured spec from narrative
    output: .workflow/spec.json
    input_mapping:
      provider: "${provider:-claude}"

  - name: generate_backlog
    engine: pkg/backlog
    description: Generate epics and stories
    output: .workflow/backlog.json
    input_mapping:
      provider: "${provider:-claude}"

  - name: generate_deep_dives
    engine: pkg/techref
    description: Generate tech ref documentation (TechRegistry)
    output: .workflow/deep-dives/
    optional: true
    input_mapping:
      mode: "${mode:-template}"
      provider: "${provider:-claude}"

  - name: enrich_infra
    engine: pkg/infracontext
    description: Enrich deep dives with live infra context
    output: .workflow/infra-context.json
    optional: true

  - name: visualize
    engine: pkg/render
    description: Generate visual output
    output: .workflow/render/
    optional: true
    input_mapping:
      format: "${format:-html}"

artefacts:
  - name: backlog
    format: backlog.json
    destination: ".workflow/"

chain:
  from: []
  to: [review]
`

// e2eRegistry builds a hybrid engine registry:
//   - pkg/extractor: mock that writes a fixture spec.json
//   - pkg/backlog:   mock that writes a fixture backlog.json
//   - pkg/techref:   REAL engine (template mode, zero-LLM)
//   - pkg/infracontext: REAL engine (gracefully skips when no providers available)
//   - pkg/render:    REAL engine
//
// This approach tests the real heuristic/template pipeline end-to-end while
// isolating LLM-dependent steps behind deterministic fixtures.
func e2eRegistry(t *testing.T, repoDir string) map[string]StepExecutor {
	t.Helper()

	bl := backlogFixture()

	return map[string]StepExecutor{
		// Mock: writes fixture spec JSON (simulates extractor output)
		"pkg/extractor": func(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
			result := StepResult{StepName: step.Name}
			specData := map[string]string{
				"ProjectDefinition": "# E2E Test Project\n\nAPI Gateway with PostgreSQL and Docker deployment.",
			}
			if step.Output != "" {
				outPath := resolveOutput(step.Output, opts)
				if err := saveJSON(outPath, specData); err != nil {
					result.Error = err
					return result
				}
				result.Outputs = map[string]string{"spec": outPath}
			}
			return result
		},

		// Mock: writes fixture backlog JSON (simulates backlog generator output)
		"pkg/backlog": func(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
			result := StepResult{StepName: step.Name}
			if step.Output != "" {
				outPath := resolveOutput(step.Output, opts)
				data, _ := json.MarshalIndent(bl, "", "  ")
				if err := writeFile(outPath, data); err != nil {
					result.Error = err
					return result
				}
				result.Outputs = map[string]string{"backlog": outPath}
			}
			return result
		},

		// REAL: techref engine uses template mode (zero-LLM)
		"pkg/techref": techrefEngine,

		// REAL: infracontext engine (gracefully skips without providers)
		"pkg/infracontext": infracontextEngine,

		// REAL: render engine
		"pkg/render": renderEngine,
	}
}

// ── E2E Tests ───────────────────────────────────────────────────────────────

// TestE2E_BacklogPipeline_TemplateMode runs the full backlog pipeline:
// narrative → spec extraction → backlog generation → deep dives → render.
// LLM-dependent steps (extractor, backlog) use fixture mocks.
// Template/heuristic steps (techref, render) use REAL engines.
func TestE2E_BacklogPipeline_TemplateMode(t *testing.T) {
	repoDir := t.TempDir()
	workflowHome := t.TempDir()

	// Write the backlog pipeline definition
	writeFixture(t, workflowHome, "backlog", backlogE2EDefinitionYAML)

	def, err := LoadDefinition(workflowHome, "backlog")
	if err != nil {
		t.Fatalf("failed to load backlog definition: %v", err)
	}

	// Create a dummy narrative file (the mock extractor reads the path but
	// doesn't actually process it — it writes fixture data instead).
	narrativePath := filepath.Join(repoDir, "narrative.md")
	if err := os.WriteFile(narrativePath, []byte("# Meeting Notes\nWe need an API gateway."), 0644); err != nil {
		t.Fatalf("failed to write narrative fixture: %v", err)
	}

	registry := e2eRegistry(t, repoDir)
	term := &mockTerminal{responses: []bool{true, true, true}} // confirm all optional steps
	opts := RunOptions{
		RepoPath:     repoDir,
		WorkflowHome: workflowHome,
	}
	runner := New(def, registry, opts).WithTerminal(term)

	inputs := RunInputs{
		"narrative": narrativePath,
		"mode":      "template",
		"format":    "markdown",
	}

	results, err := runner.Run(inputs)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// ── Verify step count ──────────────────────────────────────────────
	if len(results) != 5 {
		t.Errorf("expected 5 step results, got %d", len(results))
	}

	// ── Verify spec output ─────────────────────────────────────────────
	specPath := filepath.Join(repoDir, ".workflow", "spec.json")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Errorf("spec.json not created at %s", specPath)
	}

	// ── Verify backlog output ──────────────────────────────────────────
	backlogPath := filepath.Join(repoDir, ".workflow", "backlog.json")
	if _, err := os.Stat(backlogPath); os.IsNotExist(err) {
		t.Errorf("backlog.json not created at %s", backlogPath)
	} else {
		// Verify backlog is valid JSON with expected structure
		data, err := os.ReadFile(backlogPath)
		if err != nil {
			t.Fatalf("failed to read backlog.json: %v", err)
		}
		var bl types.Backlog
		if err := json.Unmarshal(data, &bl); err != nil {
			t.Errorf("backlog.json is not valid Backlog JSON: %v", err)
		}
		if bl.Meta.TotalEpics != 2 {
			t.Errorf("expected 2 epics in backlog, got %d", bl.Meta.TotalEpics)
		}
		if bl.Meta.ProjectTitle != "E2E Test Project" {
			t.Errorf("expected project title 'E2E Test Project', got %q", bl.Meta.ProjectTitle)
		}
	}

	// ── Verify deep dives output (REAL techref template engine) ────────
	deepDivesPath := filepath.Join(repoDir, ".workflow", "deep-dives", "deep-dives.json")
	if _, err := os.Stat(deepDivesPath); os.IsNotExist(err) {
		t.Errorf("deep-dives.json not created at %s", deepDivesPath)
	} else {
		data, err := os.ReadFile(deepDivesPath)
		if err != nil {
			t.Fatalf("failed to read deep-dives.json: %v", err)
		}
		var dives []types.DeepDive
		if err := json.Unmarshal(data, &dives); err != nil {
			t.Errorf("deep-dives.json is not valid JSON: %v", err)
		}
		// The backlog mentions PostgreSQL, Go, and Docker — all have templates.
		// At least some deep dives should be generated.
		if len(dives) == 0 {
			t.Error("expected at least one deep dive generated from templates")
		}
		// Verify at least one dive has source=template
		hasTemplate := false
		for _, d := range dives {
			if d.Source.Source == "template" {
				hasTemplate = true
				break
			}
		}
		if !hasTemplate {
			t.Error("expected at least one deep dive with source=template")
		}
	}

	// ── Verify render output (REAL markdown renderer) ──────────────────
	mdPath := filepath.Join(repoDir, ".workflow", "render", "backlog.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Errorf("markdown render not created at %s", mdPath)
	} else {
		data, err := os.ReadFile(mdPath)
		if err != nil {
			t.Fatalf("failed to read backlog.md: %v", err)
		}
		content := string(data)
		if len(content) == 0 {
			t.Error("rendered markdown is empty")
		}
		// Check that rendered markdown contains epic titles from fixture
		if !strings.Contains(content, "API Gateway") {
			t.Error("rendered markdown should contain epic title 'API Gateway'")
		}
		if !strings.Contains(content, "Containerization") {
			t.Error("rendered markdown should contain epic title 'Containerization'")
		}
	}

	// ── Verify infra step skipped gracefully (no kubectl/kafka/psql) ───
	infraResult := results[3] // enrich_infra is step 4 (index 3)
	if infraResult.Error != nil {
		t.Errorf("infra step should skip gracefully, got error: %v", infraResult.Error)
	}

	// ── Verify no step had errors ──────────────────────────────────────
	for i, r := range results {
		if r.Error != nil {
			t.Errorf("step %d (%s) had unexpected error: %v", i, r.StepName, r.Error)
		}
	}
}

// TestE2E_BacklogPipeline_MissingNarrative verifies the pipeline fails
// gracefully when no narrative input is provided.
func TestE2E_BacklogPipeline_MissingNarrative(t *testing.T) {
	workflowHome := t.TempDir()
	writeFixture(t, workflowHome, "backlog", backlogE2EDefinitionYAML)

	def, err := LoadDefinition(workflowHome, "backlog")
	if err != nil {
		t.Fatalf("failed to load definition: %v", err)
	}

	registry := e2eRegistry(t, t.TempDir())
	opts := RunOptions{RepoPath: t.TempDir()}
	runner := New(def, registry, opts).WithTerminal(&mockTerminal{})

	// Run without "narrative" input (which is required)
	_, err = runner.Run(RunInputs{})
	if err == nil {
		t.Fatal("expected error for missing required input 'narrative'")
	}
	if !strings.Contains(err.Error(), "narrative") {
		t.Errorf("error should mention missing 'narrative' input, got: %v", err)
	}
}

// TestE2E_BacklogPipeline_SkipOptionalSteps verifies that optional steps
// (deep dives, infra, render) are skipped gracefully with AutoSkip.
func TestE2E_BacklogPipeline_SkipOptionalSteps(t *testing.T) {
	repoDir := t.TempDir()
	workflowHome := t.TempDir()
	writeFixture(t, workflowHome, "backlog", backlogE2EDefinitionYAML)

	def, err := LoadDefinition(workflowHome, "backlog")
	if err != nil {
		t.Fatalf("failed to load definition: %v", err)
	}

	narrativePath := filepath.Join(repoDir, "narrative.md")
	if err := os.WriteFile(narrativePath, []byte("# Notes\nBuild something."), 0644); err != nil {
		t.Fatalf("failed to write narrative: %v", err)
	}

	registry := e2eRegistry(t, repoDir)
	term := &mockTerminal{}
	opts := RunOptions{
		RepoPath:     repoDir,
		WorkflowHome: workflowHome,
		AutoSkip:     true, // skip all optional steps
	}
	runner := New(def, registry, opts).WithTerminal(term)

	results, err := runner.Run(RunInputs{"narrative": narrativePath})
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 step results, got %d", len(results))
	}

	// Steps 0 (extract_spec) and 1 (generate_backlog) are required — should execute
	if results[0].Skipped {
		t.Error("extract_spec should not be skipped")
	}
	if results[1].Skipped {
		t.Error("generate_backlog should not be skipped")
	}

	// Steps 2, 3, 4 are optional — should be skipped
	optionalSteps := []struct {
		idx  int
		name string
	}{
		{2, "generate_deep_dives"},
		{3, "enrich_infra"},
		{4, "visualize"},
	}
	for _, s := range optionalSteps {
		if !results[s.idx].Skipped {
			t.Errorf("step %q (index %d) should be skipped with AutoSkip", s.name, s.idx)
		}
	}

	// Verify required step outputs still exist
	specPath := filepath.Join(repoDir, ".workflow", "spec.json")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Error("spec.json should still be created for required steps")
	}
	backlogPath := filepath.Join(repoDir, ".workflow", "backlog.json")
	if _, err := os.Stat(backlogPath); os.IsNotExist(err) {
		t.Error("backlog.json should still be created for required steps")
	}

	// Verify optional outputs do NOT exist (they were skipped)
	deepDivesPath := filepath.Join(repoDir, ".workflow", "deep-dives", "deep-dives.json")
	if _, err := os.Stat(deepDivesPath); err == nil {
		t.Error("deep-dives.json should NOT exist when deep dives step is skipped")
	}
}

// TestE2E_RenderFormats_HTML verifies that HTML output is generated correctly
// when the render engine is invoked with format=html.
func TestE2E_RenderFormats_HTML(t *testing.T) {
	repoDir := t.TempDir()
	bl := backlogFixture()

	// Pre-write backlog fixture (render engine reads it directly)
	writeBacklogFixture(t, repoDir, bl)

	step := StepSpec{
		Name:   "visualize",
		Engine: "pkg/render",
		Output: ".workflow/render/",
	}
	inputs := RunInputs{"format": "html"}
	opts := RunOptions{RepoPath: repoDir}

	result := renderEngine(step, inputs, opts)
	if result.Error != nil {
		t.Fatalf("render engine failed: %v", result.Error)
	}

	// Verify index.html was created
	htmlDir := filepath.Join(repoDir, ".workflow", "render")
	indexPath := filepath.Join(htmlDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Errorf("index.html not created at %s", indexPath)
	} else {
		data, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("failed to read index.html: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "<html") && !strings.Contains(content, "<!DOCTYPE") {
			t.Error("index.html should contain HTML markup")
		}
		if len(content) < 100 {
			t.Errorf("index.html seems too small (%d bytes), expected substantial HTML output", len(content))
		}
	}

	// Verify outputs map
	if result.Outputs == nil || result.Outputs["html"] == "" {
		t.Error("render result should have 'html' output path")
	}
}

// TestE2E_RenderFormats_Both verifies that both HTML and Markdown outputs
// are generated when format=both is requested.
func TestE2E_RenderFormats_Both(t *testing.T) {
	repoDir := t.TempDir()
	bl := backlogFixture()

	// Pre-write backlog fixture
	writeBacklogFixture(t, repoDir, bl)

	step := StepSpec{
		Name:   "visualize",
		Engine: "pkg/render",
		Output: ".workflow/render/",
	}
	inputs := RunInputs{"format": "both"}
	opts := RunOptions{RepoPath: repoDir}

	result := renderEngine(step, inputs, opts)
	if result.Error != nil {
		t.Fatalf("render engine failed: %v", result.Error)
	}

	renderDir := filepath.Join(repoDir, ".workflow", "render")

	// Verify HTML output
	indexPath := filepath.Join(renderDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("index.html not created for 'both' format")
	}

	// Verify Markdown output
	mdPath := filepath.Join(renderDir, "backlog.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Error("backlog.md not created for 'both' format")
	} else {
		data, err := os.ReadFile(mdPath)
		if err != nil {
			t.Fatalf("failed to read backlog.md: %v", err)
		}
		content := string(data)
		// Markdown should contain epic information from fixture
		if !strings.Contains(content, "API Gateway") {
			t.Error("markdown should contain 'API Gateway' from fixture backlog")
		}
	}

	// Verify outputs map has both keys
	if result.Outputs == nil {
		t.Fatal("render result should have outputs")
	}
	if result.Outputs["html"] == "" {
		t.Error("render result should have 'html' output path")
	}
	if result.Outputs["markdown"] == "" {
		t.Error("render result should have 'markdown' output path")
	}
}
