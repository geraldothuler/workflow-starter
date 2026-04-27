package playbook

import (
	"context"
	"fmt"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
)

// Executor runs a playbook against an infrastructure registry.
type Executor struct {
	registry *infracontext.Registry
}

// NewExecutor creates a new playbook executor.
func NewExecutor(registry *infracontext.Registry) *Executor {
	return &Executor{registry: registry}
}

// Execute runs all steps in the playbook sequentially.
func (e *Executor) Execute(ctx context.Context, spec *PlaybookSpec, opts ExecuteOptions) (*InvestigationReport, error) {
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid playbook: %w", err)
	}

	// Validate required providers
	for _, req := range spec.RequiredProviders {
		p, err := e.registry.Get(req.ID)
		if err != nil {
			return nil, fmt.Errorf("required provider %q not registered: %w", req.ID, err)
		}
		if !p.Available() {
			return nil, fmt.Errorf("required provider %q is not available", req.ID)
		}
	}

	report := &InvestigationReport{
		PlaybookID:    spec.ID,
		PlaybookTitle: spec.Title,
		StartedAt:     time.Now(),
	}

	var allFindings []Finding
	findingCounter := 0

	for _, step := range spec.Steps {
		stepStart := time.Now()
		result := StepResult{
			StepID:   step.ID,
			Title:    step.Title,
			Provider: step.Provider,
		}

		// Resolve provider
		provider, err := e.registry.Get(step.Provider)
		if err != nil {
			if step.Optional {
				result.Status = StepStatusSkipped
				result.Error = fmt.Sprintf("provider not registered: %v", err)
				result.Duration = time.Since(stepStart)
				report.StepResults = append(report.StepResults, result)
				report.StepsSkipped++
				continue
			}
			return nil, fmt.Errorf("step %q: provider %q not registered: %w", step.ID, step.Provider, err)
		}

		if !provider.Available() {
			if step.Optional {
				result.Status = StepStatusSkipped
				result.Error = "provider not available"
				result.Duration = time.Since(stepStart)
				report.StepResults = append(report.StepResults, result)
				report.StepsSkipped++
				continue
			}
			return nil, fmt.Errorf("step %q: provider %q is not available", step.ID, step.Provider)
		}

		// Fetch infrastructure context
		ic, err := provider.Fetch(ctx, infracontext.FetchOptions{
			Namespace:     opts.Namespace,
			KubeContext:   opts.KubeContext,
			ResourceTypes: step.ResourceTypes,
			UseCache:      true,
			Verbose:       opts.Verbose,
		})
		if err != nil {
			if step.Optional {
				result.Status = StepStatusSkipped
				result.Error = fmt.Sprintf("fetch failed: %v", err)
				result.Duration = time.Since(stepStart)
				report.StepResults = append(report.StepResults, result)
				report.StepsSkipped++
				continue
			}
			result.Status = StepStatusError
			result.Error = fmt.Sprintf("fetch failed: %v", err)
			result.Duration = time.Since(stepStart)
			report.StepResults = append(report.StepResults, result)
			report.StepsExecuted++
			continue
		}

		// Run analyzers
		var stepFindings []Finding
		for _, ref := range step.Analyzers {
			findings, err := CallAnalyzer(ref.Name, ic, ref.Args, allFindings)
			if err != nil {
				if opts.Verbose {
					fmt.Printf("  analyzer %q error: %v\n", ref.Name, err)
				}
				continue
			}
			for i := range findings {
				findingCounter++
				if findings[i].ID == "" {
					findings[i].ID = fmt.Sprintf("f%d", findingCounter)
				}
				findings[i].StepID = step.ID
			}
			stepFindings = append(stepFindings, findings...)
		}

		allFindings = append(allFindings, stepFindings...)

		result.Status = StepStatusSuccess
		result.Findings = make([]string, len(stepFindings))
		for i, f := range stepFindings {
			result.Findings[i] = f.ID
		}
		result.Duration = time.Since(stepStart)
		report.StepResults = append(report.StepResults, result)
		report.StepsExecuted++
	}

	report.Findings = allFindings
	report.CausalChain = BuildCausalChain(allFindings, DefaultCausalRules())
	report.CompletedAt = time.Now()
	report.Duration = report.CompletedAt.Sub(report.StartedAt)
	report.Summary = buildSummary(allFindings)
	report.Markdown = RenderMarkdown(report, spec)

	return report, nil
}

func buildSummary(findings []Finding) string {
	if len(findings) == 0 {
		return "No issues found. All checks passed."
	}

	critical, warning, info := 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warning++
		case SeverityInfo:
			info++
		}
	}

	summary := fmt.Sprintf("Found %d issue(s)", len(findings))
	parts := []string{}
	if critical > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", critical))
	}
	if warning > 0 {
		parts = append(parts, fmt.Sprintf("%d warning", warning))
	}
	if info > 0 {
		parts = append(parts, fmt.Sprintf("%d info", info))
	}

	if len(parts) > 0 {
		summary += ": "
		for i, p := range parts {
			if i > 0 {
				summary += ", "
			}
			summary += p
		}
	}

	return summary
}
