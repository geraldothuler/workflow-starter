package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/backlog"
	"github.com/Cobliteam/workflow-toolkit/pkg/export"
	"github.com/Cobliteam/workflow-toolkit/pkg/extractor"
	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/ops"
	"github.com/Cobliteam/workflow-toolkit/pkg/parser"
	"github.com/Cobliteam/workflow-toolkit/pkg/playbook"
	"github.com/Cobliteam/workflow-toolkit/pkg/render"
	"github.com/Cobliteam/workflow-toolkit/pkg/techref"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// DefaultRegistry returns the platform engine registry.
//
// Phase 5: real engine wiring.
// Phase 7: plan-execute wired to ops.ExecutePlan (subprocess wiring complete).
//   - pkg/ops      → CheckAWSAuth, CheckDBHealth, CheckK8sStatus, CheckKafkaStatus, CheckLogsAnalyze, NewPlanFromTemplate, ExecutePlan
//   - pkg/playbook → LoadPlaybookConfigs + Executor.Execute
//   - pkg/extractor → TranscriptExtractor.Extract (narrative → spec JSON)
//   - pkg/backlog   → Generator.Generate (spec JSON → backlog markdown)
//   - pkg/techref      → template-first tech ref generation (Phase 8)
//   - pkg/infracontext → live infra context fetch (Phase 8)
//   - pkg/render       → multi-format backlog rendering (Phase 8)
func DefaultRegistry() map[string]StepExecutor {
	return map[string]StepExecutor{
		"pkg/extractor":    extractorEngine,
		"pkg/backlog":      backlogEngine,
		"pkg/techref":      techrefEngine,
		"pkg/infracontext": infracontextEngine,
		"pkg/render":       renderEngine,
		"pkg/playbook":     playbookEngine,
		"pkg/ops":          opsEngine,
	}
}

// ── pkg/ops engine ────────────────────────────────────────────────────────────

// opsEngine dispatches each command in step.AllCommands() to the corresponding
// pkg/ops function, accumulates results, and serialises them to step.Output.
// Auth-blocking chain: if a critical dependency (e.g. auth) fails, downstream
// commands that depend on it are skipped automatically with a skip signal.
func opsEngine(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
	result := StepResult{StepName: step.Name}

	cmds := step.AllCommands()
	if len(cmds) == 0 {
		result.Error = fmt.Errorf("pkg/ops: no commands specified in step %q", step.Name)
		return result
	}

	// Load critical commands config for dependency tracking
	critCfg, _ := LoadCriticalCommandsConfig()
	cmdStatuses := map[string]string{} // cmd -> status

	var allResults []ops.OpsResult
	for _, cmd := range cmds {
		spec := MergedCommandSpec(cmd, critCfg, step.CommandSpecs)

		// Check: should skip because a dependency failed?
		if spec != nil && shouldSkip(spec, cmdStatuses) {
			skipSignal := renderSkipSignal(spec.SkipSignal, spec, cmdStatuses)
			skipResult := ops.OpsResult{
				Status: "skipped",
				Signal: skipSignal,
				Cost:   "zero-llm",
			}
			printOpsResult(cmd, skipResult)
			allResults = append(allResults, skipResult)
			cmdStatuses[cmd] = "skipped"
			continue
		}

		r, err := dispatchOpsCommand(cmd, inputs, opts)
		if err != nil {
			result.Error = err
			return result
		}
		printOpsResult(cmd, r)
		allResults = append(allResults, r)
		cmdStatuses[cmd] = r.Status
	}

	// Persist combined results if output path is specified.
	if step.Output != "" {
		outPath := resolveOutput(step.Output, opts)
		if err := saveJSON(outPath, allResults); err != nil {
			fmt.Printf("  ⚠ could not save output to %s: %v\n", outPath, err)
		} else {
			result.Outputs = map[string]string{"results": outPath}
			fmt.Printf("  ✓ saved: %s\n", outPath)
		}
	}

	return result
}

// dispatchOpsCommand routes a single ops command string to the matching pkg/ops function.
// Defaults are resolved via YAML input_mapping before this function is called
// (see runner.ResolveInputs and use-cases/ops-response/definition.yml).
func dispatchOpsCommand(cmd string, inputs RunInputs, opts RunOptions) (ops.OpsResult, error) {
	switch cmd {
	case "auth":
		return ops.CheckAWSAuth(inputs["profile"]), nil

	case "db-health":
		cfg := ops.DBHealthConfig{
			KubectlContext: inputs["kubectl-context"],
			Namespace:      inputs["namespace"],
			DBHost:         inputs["db-host"],
			DBPort:         inputs["db-port"],
			DBUser:         inputs["db-user"],
			DBPassword:     inputs["db-password"],
			DBPasswordSSM:  inputs["db-password-ssm"],
			AWSProfile:     inputs["profile"],
			DBName:         inputs["db-name"],
		}
		return ops.CheckDBHealth(cfg), nil

	case "k8s-status":
		cfg := ops.K8sConfig{
			KubectlContext: inputs["kubectl-context"],
			Namespace:      inputs["namespace"],
			Deployment:     inputs["deployment"],
			LabelSelector:  inputs["label-selector"],
		}
		return ops.CheckK8sStatus(cfg), nil

	case "kafka-status":
		cfg := ops.KafkaConfig{
			KubectlContext: inputs["kubectl-context"],
			Namespace:      inputs["namespace"],
			Deployment:     inputs["kafka-deployment"],
			LabelSelector:  inputs["kafka-label-selector"],
			Topic:          inputs["kafka-topic"],
			ConsumerGroup:  inputs["kafka-consumer-group"],
			Window:         inputs["window"],
			Source:         inputs["kafka-source"],
		}
		return ops.CheckKafkaStatus(cfg), nil

	case "logs-analyze":
		cfg := ops.LogsConfig{
			FilePath: inputs["logs-file"],
			Patterns: splitCSV(inputs["log-patterns"]),
			Limit:    50,
		}
		return ops.CheckLogsAnalyze(cfg), nil

	case "plan-new":
		return opsPlanNew(inputs, opts), nil

	case "plan-execute":
		return opsPlanExecute(inputs, opts), nil

	case "jira":
		return ops.CheckJira(jiraCfgFromInputs(inputs)), nil
	case "slack":
		return ops.CheckSlack(slackCfgFromInputs(inputs)), nil
	case "websearch":
		return ops.CheckWebSearch(webSearchCfgFromInputs(inputs)), nil
	case "snowflake":
		return ops.CheckSnowflake(snowflakeCfgFromInputs(inputs)), nil
	case "montecarlo":
		return ops.CheckMonteCarlo(monteCarloCfgFromInputs(inputs)), nil
	case "airbyte":
		return ops.CheckAirbyte(airbyteCfgFromInputs(inputs)), nil

	case "github":
		return ops.CheckGitHub(githubCfgFromInputs(inputs)), nil

	default:
		return ops.OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("unknown ops command %q — valid: auth, db-health, k8s-status, kafka-status, logs-analyze, plan-new, plan-execute, jira, slack, websearch, snowflake, montecarlo, airbyte, github", cmd),
			Cost:   "zero-llm",
		}, nil
	}
}

// opsPlanNew creates a plan from template or blank and saves it to the output path.
// Defaults (symptom, namespace, profile, consumer-deployment) are resolved via
// YAML input_mapping before this function is called.
func opsPlanNew(inputs RunInputs, opts RunOptions) ops.OpsResult {
	templateID := inputs["plan_template"]
	scenario := inputs["symptom"]

	var plan ops.Plan
	if templateID != "" {
		vars := map[string]string{
			"kubectl-context":     inputs["kubectl-context"],
			"namespace":           inputs["namespace"],
			"db-host":             inputs["db-host"],
			"db-user":             inputs["db-user"],
			"ssm-path":            inputs["ssm-path"],
			"deployment":          inputs["deployment"],
			"consumer-deployment": inputs["consumer-deployment"],
			"kafka-topic":         inputs["kafka-topic"],
			"aws-profile":         inputs["profile"],
		}
		var ok bool
		plan, ok = ops.NewPlanFromTemplate(templateID, scenario, vars)
		if !ok {
			tpls := ops.ListTemplates()
			ids := make([]string, len(tpls))
			for i, t := range tpls {
				ids[i] = t.ID
			}
			return ops.OpsResult{
				Status:  "error",
				Signal:  fmt.Sprintf("plan template %q not found (available: %s)", templateID, strings.Join(ids, ", ")),
				Actions: []string{"use --input plan_template=<id>"},
				Cost:    "zero-llm",
			}
		}
	} else {
		plan = ops.NewBlankPlan(scenario, scenario, "medium")
	}

	// Save plan YAML to the standard location.
	planPath := resolveOutput(".workflow/ops/plan.md", opts)
	if err := os.MkdirAll(filepath.Dir(planPath), 0755); err == nil {
		data, err := ops.MarshalPlan(plan)
		if err == nil {
			_ = os.WriteFile(planPath, data, 0644)
		}
	}

	fmt.Printf("  Plan created: %s\n", planPath)
	fmt.Println(ops.PlanSummary(plan))

	return ops.OpsResult{
		Status: "ok",
		Signal: fmt.Sprintf("plan created: %s (%d steps)", plan.Plan.ID, len(plan.Plan.Steps)),
		Data:   map[string]any{"plan_path": planPath, "steps": len(plan.Plan.Steps), "risk": plan.Plan.Risk},
		Cost:   "zero-llm",
	}
}

// opsPlanExecute loads the plan from the standard location and executes it step-by-step.
// owner=auto steps invoke the wtb binary; owner=human steps wait for operator confirmation.
func opsPlanExecute(inputs RunInputs, opts RunOptions) ops.OpsResult {
	planPath := resolveOutput(".workflow/ops/plan.md", opts)
	data, err := os.ReadFile(planPath)
	if err != nil {
		return ops.OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("plan not found at %s — run create_plan step first", planPath),
			Actions: []string{"wtb run ops-response --input ... (re-run from create_plan step)"},
			Cost:    "zero-llm",
		}
	}

	plan, err := ops.UnmarshalPlan(data)
	if err != nil {
		return ops.OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("failed to parse plan at %s: %v", planPath, err),
			Cost:   "zero-llm",
		}
	}

	return ops.ExecutePlan(plan, ops.PlanExecuteConfig{
		DryRun: opts.DryRun,
	})
}

// printOpsResult prints a human-readable summary of an OpsResult.
func printOpsResult(cmd string, r ops.OpsResult) {
	icons := map[string]string{"ok": "✓", "warn": "⚠", "critical": "✗", "error": "✗"}
	icon := icons[r.Status]
	if icon == "" {
		icon = "•"
	}
	fmt.Printf("  [ops/%s] %s %s\n", cmd, icon, r.Signal)
	for _, a := range r.Actions {
		fmt.Printf("    → %s\n", a)
	}
}

// ── pkg/playbook engine ───────────────────────────────────────────────────────

// playbookEngine loads a playbook by ID (from inputs["playbook"]), builds an empty
// infracontext registry, and executes the playbook. Steps whose providers are not
// registered are skipped automatically.
func playbookEngine(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
	result := StepResult{StepName: step.Name}

	playbookID := inputs["playbook"]
	if playbookID == "" {
		result.Error = fmt.Errorf("pkg/playbook: 'playbook' input required (--input playbook=<id>)")
		return result
	}

	// Load playbook catalogue (embedded defaults + optional project overrides).
	specs, err := playbook.LoadPlaybookConfigs(opts.RepoPath)
	if err != nil {
		result.Error = fmt.Errorf("pkg/playbook: failed to load configs: %w", err)
		return result
	}

	spec, ok := specs[playbookID]
	if !ok {
		ids := make([]string, 0, len(specs))
		for id := range specs {
			ids = append(ids, id)
		}
		result.Error = fmt.Errorf("pkg/playbook: playbook %q not found (available: %s)", playbookID, strings.Join(ids, ", "))
		return result
	}

	// Build an infracontext registry. Providers that are unavailable cause their
	// steps to be skipped (if optional) or return an error (if required).
	reg := infracontext.NewRegistry()
	exec := playbook.NewExecutor(reg)

	report, err := exec.Execute(context.Background(), spec, playbook.ExecuteOptions{
		Namespace:   inputs["namespace"],
		KubeContext: inputs["kubectl-context"],
		Verbose:     true,
	})
	if err != nil {
		result.Error = fmt.Errorf("pkg/playbook: execution error: %w", err)
		return result
	}

	fmt.Printf("  Summary: %s\n", report.Summary)

	if step.Output != "" {
		outPath := resolveOutput(step.Output, opts)
		if err := writeFile(outPath, []byte(report.Markdown)); err != nil {
			fmt.Printf("  ⚠ could not save report to %s: %v\n", outPath, err)
		} else {
			result.Outputs = map[string]string{"report": outPath}
			fmt.Printf("  ✓ report saved: %s\n", outPath)
		}
	}

	return result
}

// ── pkg/extractor engine ──────────────────────────────────────────────────────

// extractorEngine reads inputs["narrative"] (file path), runs TranscriptExtractor.Extract,
// and saves the ExtractionResult JSON to step.Output for use by backlogEngine.
func extractorEngine(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
	result := StepResult{StepName: step.Name}

	narrative := inputs["narrative"]
	if narrative == "" {
		result.Error = fmt.Errorf("pkg/extractor: 'narrative' input required (--input narrative=<file>)")
		return result
	}
	narrative = expandHomePath(narrative)
	if _, err := os.Stat(narrative); os.IsNotExist(err) {
		result.Error = fmt.Errorf("pkg/extractor: narrative file not found: %s", narrative)
		return result
	}

	providerID := inputs["provider"]
	p, err := llm.NewProvider(llm.ProviderConfig{Provider: providerID})
	if err != nil {
		result.Error = fmt.Errorf("pkg/extractor: failed to create extractor (provider=%s): %w", providerID, err)
		return result
	}
	ext, err := extractor.NewTranscriptExtractorWithProvider(p, "", "")
	if err != nil {
		result.Error = fmt.Errorf("pkg/extractor: failed to create extractor (provider=%s): %w", providerID, err)
		return result
	}

	fmt.Printf("  Extracting spec from: %s\n", narrative)
	extraction, err := ext.Extract(narrative)
	if err != nil {
		result.Error = fmt.Errorf("pkg/extractor: extraction failed: %w", err)
		return result
	}

	fmt.Printf("  ✓ spec extracted (%d chars)\n", len(extraction.ProjectDefinition))

	if step.Output != "" {
		outPath := resolveOutput(step.Output, opts)
		data, _ := json.MarshalIndent(extraction, "", "  ")
		if err := writeFile(outPath, data); err != nil {
			fmt.Printf("  ⚠ could not save spec to %s: %v\n", outPath, err)
		} else {
			result.Outputs = map[string]string{"spec": outPath}
			fmt.Printf("  ✓ spec saved: %s\n", outPath)
		}
	}

	return result
}

// ── pkg/backlog engine ────────────────────────────────────────────────────────

// backlogEngine reads the extraction JSON written by extractorEngine, parses the
// ProjectDefinition into a ProjectInput, runs the Generator, and exports Markdown.
//
// It looks for the extraction JSON at:
//  1. inputs["spec-file"]
//  2. {repoPath}/.workflow/backlog/spec.json (convention — written by extract_spec step)
func backlogEngine(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
	result := StepResult{StepName: step.Name}

	specPath := coalesce(
		inputs["spec-file"],
		resolveOutput(".workflow/spec.json", opts),        // written by extract_spec step
		resolveOutput(".workflow/backlog/spec.json", opts), // legacy path
	)

	data, err := os.ReadFile(specPath)
	if err != nil {
		result.Error = fmt.Errorf("pkg/backlog: spec file not found at %s — run extract_spec step first", specPath)
		return result
	}

	var extraction extractor.ExtractionResult
	if err := json.Unmarshal(data, &extraction); err != nil {
		result.Error = fmt.Errorf("pkg/backlog: failed to parse spec JSON: %w", err)
		return result
	}

	// Write ProjectDefinition as a temp markdown file for parser.ParseInput.
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("wtb-spec-%d.md", os.Getpid()))
	if err := os.WriteFile(tmpFile, []byte(extraction.ProjectDefinition), 0644); err != nil {
		result.Error = fmt.Errorf("pkg/backlog: failed to write temp spec file: %w", err)
		return result
	}
	defer os.Remove(tmpFile)

	projectInput, err := parser.ParseInput(tmpFile)
	if err != nil {
		result.Error = fmt.Errorf("pkg/backlog: failed to parse project input: %w", err)
		return result
	}

	providerID := inputs["provider"]
	bp, bpErr := llm.NewProvider(llm.ProviderConfig{Provider: providerID})
	if bpErr != nil {
		result.Error = fmt.Errorf("pkg/backlog: failed to create provider (provider=%s): %w", providerID, bpErr)
		return result
	}
	spec := &types.Specification{}
	gen := backlog.NewGeneratorWithProvider(bp, spec, projectInput)

	fmt.Printf("  Generating backlog (provider=%s)...\n", providerID)
	bl, err := gen.Generate(projectInput, backlog.GenerateOptions{
		SkipDeepDive: inputs["skip-deep-dive"] == "true",
	})
	if err != nil {
		result.Error = fmt.Errorf("pkg/backlog: generation failed: %w", err)
		return result
	}

	fmt.Printf("  ✓ %d epics, %d stories generated\n", bl.Meta.TotalEpics, bl.Meta.TotalStories)

	if step.Output != "" {
		outPath := resolveOutput(step.Output, opts)
		if err := export.ExportBacklogMarkdown(bl, outPath); err != nil {
			fmt.Printf("  ⚠ could not save backlog to %s: %v\n", outPath, err)
		} else {
			result.Outputs = map[string]string{"backlog": outPath}
			fmt.Printf("  ✓ backlog saved: %s\n", outPath)
		}
	}

	return result
}

// ── pkg/techref engine ────────────────────────────────────────────────────────

// techrefEngine generates tech ref documents for technologies found in the backlog.
// Mode "template" (default) uses zero-LLM templates; "hybrid" falls back to LLM for unknowns.
func techrefEngine(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
	result := StepResult{StepName: step.Name}

	backlogPath := coalesce(
		inputs["backlog-file"],
		resolveOutput(".workflow/backlog.json", opts),
	)

	data, err := os.ReadFile(backlogPath)
	if err != nil {
		result.Error = fmt.Errorf("pkg/techref: backlog file not found at %s — run generate_backlog step first", backlogPath)
		return result
	}

	var bl types.Backlog
	if err := json.Unmarshal(data, &bl); err != nil {
		result.Error = fmt.Errorf("pkg/techref: failed to parse backlog JSON: %w", err)
		return result
	}

	reg, err := techref.NewTechRegistry()
	if err != nil {
		reg = techref.DefaultRegistry()
	}

	config := techref.GetDefaultGenerationConfig()

	mode := coalesce(inputs["mode"], "template")
	switch mode {
	case "template":
		config.TemplateOnly = true
		fmt.Println("  Mode: template-only (zero-LLM)")
	case "hybrid":
		config.TemplateOnly = false
		providerID := llm.Provider(coalesce(inputs["provider"], "claude"))
		client, cErr := llm.NewClient(providerID)
		if cErr == nil {
			config.LLMCaller = func(prompt string) (string, error) {
				return client.Complete(prompt, 4096)
			}
			fmt.Printf("  Mode: hybrid (template + LLM fallback, provider=%s)\n", providerID)
		} else {
			config.TemplateOnly = true
			fmt.Printf("  Mode: template-only (LLM provider %s unavailable: %v)\n", providerID, cErr)
		}
	default:
		config.TemplateOnly = true
		fmt.Printf("  Mode: template-only (unknown mode %q, defaulting to template)\n", mode)
	}

	genResult := techref.GenerateDeepDivesOptimizedWithRegistry(reg, bl, config)

	fmt.Printf("  Generated %d tech refs (%d errors)\n", len(genResult.DeepDives), len(genResult.Errors))

	if step.Output != "" {
		outPath := resolveOutput(step.Output, opts)
		outFile := filepath.Join(outPath, "deep-dives.json")
		if err := saveJSON(outFile, genResult.DeepDives); err != nil {
			fmt.Printf("  could not save output to %s: %v\n", outFile, err)
		} else {
			result.Outputs = map[string]string{"deep-dives": outFile}
			fmt.Printf("  saved: %s\n", outFile)
		}
	}

	return result
}

// ── pkg/render engine ─────────────────────────────────────────────────────────

// renderEngine converts backlog data to visual formats (HTML, Markdown, or both).
func renderEngine(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
	result := StepResult{StepName: step.Name}

	backlogPath := coalesce(
		inputs["backlog-file"],
		resolveOutput(".workflow/backlog.json", opts),
	)

	data, err := os.ReadFile(backlogPath)
	if err != nil {
		result.Error = fmt.Errorf("pkg/render: backlog file not found at %s — run generate_backlog step first", backlogPath)
		return result
	}

	var bl types.Backlog
	if err := json.Unmarshal(data, &bl); err != nil {
		result.Error = fmt.Errorf("pkg/render: failed to parse backlog JSON: %w", err)
		return result
	}

	// Load deep dives (optional).
	var deepDives []types.DeepDive
	ddPath := coalesce(
		inputs["deep-dives-file"],
		resolveOutput(".workflow/deep-dives/deep-dives.json", opts),
	)
	if ddData, err := os.ReadFile(ddPath); err == nil {
		_ = json.Unmarshal(ddData, &deepDives)
	}

	lensData := render.ConvertToLensData(&bl, deepDives)

	format := coalesce(inputs["format"], "html")
	outDir := resolveOutput(step.Output, opts)
	if outDir == "" {
		outDir = resolveOutput(".workflow/render/", opts)
	}

	switch format {
	case "html":
		exporter := render.NewStaticExporter(lensData)
		if err := exporter.Export(outDir); err != nil {
			result.Error = fmt.Errorf("pkg/render: HTML export failed: %w", err)
			return result
		}
		result.Outputs = map[string]string{"html": outDir}
		fmt.Printf("  HTML exported: %s\n", outDir)

	case "markdown":
		md, err := render.RenderMarkdown(lensData)
		if err != nil {
			result.Error = fmt.Errorf("pkg/render: Markdown render failed: %w", err)
			return result
		}
		mdPath := filepath.Join(outDir, "backlog.md")
		if err := writeFile(mdPath, []byte(md)); err != nil {
			result.Error = fmt.Errorf("pkg/render: failed to write markdown: %w", err)
			return result
		}
		result.Outputs = map[string]string{"markdown": mdPath}
		fmt.Printf("  Markdown exported: %s\n", mdPath)

	case "both":
		exporter := render.NewStaticExporter(lensData)
		if err := exporter.Export(outDir); err != nil {
			result.Error = fmt.Errorf("pkg/render: HTML export failed: %w", err)
			return result
		}
		md, err := render.RenderMarkdown(lensData)
		if err != nil {
			result.Error = fmt.Errorf("pkg/render: Markdown render failed: %w", err)
			return result
		}
		mdPath := filepath.Join(outDir, "backlog.md")
		if err := writeFile(mdPath, []byte(md)); err != nil {
			fmt.Printf("  could not save markdown: %v\n", err)
		}
		result.Outputs = map[string]string{"html": outDir, "markdown": mdPath}
		fmt.Printf("  HTML + Markdown exported: %s\n", outDir)

	default:
		result.Error = fmt.Errorf("pkg/render: unknown format %q (valid: html, markdown, both)", format)
		return result
	}

	return result
}

// ── pkg/infracontext engine ──────────────────────────────────────────────────

// infracontextEngine fetches live infrastructure state from available providers
// (kubectl, kafka, postgres) and saves the context as JSON.
func infracontextEngine(step StepSpec, inputs RunInputs, opts RunOptions) StepResult {
	result := StepResult{StepName: step.Name}

	reg, err := infracontext.NewDefaultRegistry(opts.RepoPath, nil)
	if err != nil {
		// Fallback to empty registry.
		reg = infracontext.NewRegistry()
	}

	available := reg.Available()
	if len(available) == 0 {
		fmt.Println("  No infra providers available (kubectl, kafka, postgres not found)")
		fmt.Println("  Tip: install kubectl, kafka CLI, or psql to enable infra context enrichment")
		return result
	}

	fmt.Printf("  Available providers: %d\n", len(available))

	ctx := context.Background()
	fetchOpts := infracontext.FetchOptions{
		Namespace:   inputs["namespace"],
		KubeContext: inputs["kubectl-context"],
		UseCache:    true,
	}

	var contexts []*infracontext.InfraContext
	for _, p := range available {
		ic, err := p.Fetch(ctx, fetchOpts)
		if err != nil {
			fmt.Printf("  provider %s: fetch error: %v\n", p.Name(), err)
			continue
		}
		fmt.Printf("  %s\n", ic.Summary())
		contexts = append(contexts, ic)
	}

	if step.Output != "" && len(contexts) > 0 {
		outPath := resolveOutput(step.Output, opts)
		if err := saveJSON(outPath, contexts); err != nil {
			fmt.Printf("  could not save output to %s: %v\n", outPath, err)
		} else {
			result.Outputs = map[string]string{"infra-context": outPath}
			fmt.Printf("  saved: %s\n", outPath)
		}
	}

	return result
}

// ── shared helpers ────────────────────────────────────────────────────────────

// resolveOutput substitutes .workflow/ with the actual workflow data directory.
func resolveOutput(output string, opts RunOptions) string {
	if strings.HasPrefix(output, ".workflow/") && opts.RepoPath != "" {
		return strings.Replace(output, ".workflow/", opts.RepoPath+"/.workflow/", 1)
	}
	return output
}

// coalesce returns the first non-empty string from vals.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// splitCSV splits a comma-separated string into a trimmed slice.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// expandHomePath expands a leading ~/ in path.
func expandHomePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// saveJSON marshals v to JSON and writes it to path (creating parent dirs).
func saveJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

// writeFile writes data to path, creating parent directories as needed.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── extended integration config helpers ──────────────────────────────────────

func jiraCfgFromInputs(inputs RunInputs) ops.JiraConfig {
	return ops.JiraConfig{
		URL:      inputs["jira-url"],
		Email:    inputs["jira-email"],
		APIToken: inputs["jira-token"],
		Project:  inputs["jira-project"],
		JQL:      inputs["jira-jql"],
	}
}

func slackCfgFromInputs(inputs RunInputs) ops.SlackConfig {
	return ops.SlackConfig{
		Token:   inputs["slack-token"],
		Channel: inputs["slack-channel"],
		Query:   inputs["slack-query"],
		Window:  inputs["slack-window"],
	}
}

func webSearchCfgFromInputs(inputs RunInputs) ops.WebSearchConfig {
	return ops.WebSearchConfig{
		APIKey: inputs["websearch-api-key"],
		CSEID:  inputs["websearch-cse-id"],
		Query:  inputs["websearch-query"],
	}
}

func snowflakeCfgFromInputs(inputs RunInputs) ops.SnowflakeConfig {
	return ops.SnowflakeConfig{
		Account:   inputs["snowflake-account"],
		User:      inputs["snowflake-user"],
		Password:  inputs["snowflake-password"],
		Warehouse: inputs["snowflake-warehouse"],
		Database:  inputs["snowflake-database"],
		Schema:    inputs["snowflake-schema"],
		Query:     inputs["snowflake-query"],
	}
}

func monteCarloCfgFromInputs(inputs RunInputs) ops.MonteCarloConfig {
	return ops.MonteCarloConfig{
		APIKey:  inputs["montecarlo-api-key"],
		TableID: inputs["montecarlo-table-id"],
	}
}

func airbyteCfgFromInputs(inputs RunInputs) ops.AirbyteConfig {
	return ops.AirbyteConfig{
		URL:         inputs["airbyte-url"],
		APIKey:      inputs["airbyte-api-key"],
		WorkspaceID: inputs["airbyte-workspace-id"],
	}
}

func githubCfgFromInputs(inputs RunInputs) ops.GitHubConfig {
	pr := 0
	if v := inputs["github-pr"]; v != "" {
		fmt.Sscanf(v, "%d", &pr)
	}
	limit := 0
	if v := inputs["github-limit"]; v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	return ops.GitHubConfig{
		Repo:  inputs["github-repo"],
		PR:    pr,
		Query: inputs["github-query"],
		Scope: inputs["github-scope"],
		Limit: limit,
	}
}
