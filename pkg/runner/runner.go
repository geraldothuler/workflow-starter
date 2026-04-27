package runner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RunInputs holds the key→value inputs provided by the user for a run.
type RunInputs map[string]string

// Get returns the input value or its declared default.
func (ri RunInputs) Get(spec InputSpec) string {
	if v, ok := ri[spec.Name]; ok {
		return v
	}
	return spec.Default
}

// RunOptions controls execution behaviour.
type RunOptions struct {
	DryRun       bool   // print steps without executing
	AutoSkip     bool   // skip optional steps without asking
	WorkflowHome string // path to ~/workflow/
	RepoPath     string // target repo where artefacts are written
}

// StepResult captures the outcome of a single step execution.
type StepResult struct {
	StepName string
	Outputs  map[string]string // output key → resolved path
	Skipped  bool
	Error    error
}

// StepExecutor is the function signature for engine implementations.
// Each entry in the engine registry must satisfy this type.
type StepExecutor func(step StepSpec, inputs RunInputs, opts RunOptions) StepResult

// TerminalIO abstracts user interaction for human checkpoints.
type TerminalIO interface {
	Confirm(prompt string) bool
	Printf(format string, args ...any)
	Ask(prompt string, whyAsk string) string
	CanAsk() bool
}

// stdTerminal reads from os.Stdin and writes to w.
type stdTerminal struct{ w io.Writer }

func (t *stdTerminal) Confirm(prompt string) bool {
	fmt.Fprintf(t.w, "%s [y/N] ", prompt)
	var resp string
	fmt.Fscanln(os.Stdin, &resp)
	return strings.EqualFold(resp, "y") || strings.EqualFold(resp, "yes")
}

func (t *stdTerminal) Printf(format string, args ...any) {
	fmt.Fprintf(t.w, format, args...)
}

func (t *stdTerminal) Ask(prompt string, whyAsk string) string {
	if whyAsk != "" {
		fmt.Fprintf(t.w, "  %s\n", whyAsk)
	}
	fmt.Fprintf(t.w, "  %s: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func (t *stdTerminal) CanAsk() bool { return true }

// Runner orchestrates step execution for a pipeline use-case.
type Runner struct {
	def       *UseCaseDefinition
	registry  map[string]StepExecutor
	opts      RunOptions
	term      TerminalIO
	resolvers *ResolverRegistry
}

// New creates a Runner with the given definition, engine registry, and options.
// Use DefaultRegistry() for the standard engine set.
func New(def *UseCaseDefinition, registry map[string]StepExecutor, opts RunOptions) *Runner {
	return &Runner{
		def:      def,
		registry: registry,
		opts:     opts,
		term:     &stdTerminal{w: os.Stdout},
	}
}

// WithTerminal replaces the default terminal (useful in tests).
func (r *Runner) WithTerminal(term TerminalIO) *Runner {
	r.term = term
	return r
}

// WithResolvers sets the YAML-driven Socratic input resolver registry.
func (r *Runner) WithResolvers(reg *ResolverRegistry) *Runner {
	r.resolvers = reg
	return r
}

// ValidateInputs checks that all required inputs are present.
func (r *Runner) ValidateInputs(inputs RunInputs) error {
	var missing []string
	for _, spec := range r.def.RequiredInputs() {
		if inputs.Get(spec) == "" {
			missing = append(missing, spec.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required inputs: %s", strings.Join(missing, ", "))
	}
	return nil
}

// CollectInputs walks each required input through its resolve chain.
// Flag-provided inputs (already in the map) are always checked first.
// If no resolver succeeds for a required input, returns error.
func (r *Runner) CollectInputs(inputs RunInputs) (RunInputs, error) {
	resolved := make(RunInputs, len(inputs))
	for k, v := range inputs {
		resolved[k] = v
	}

	ctx := ResolveContext{
		WorkflowHome: r.opts.WorkflowHome,
		PersonalDir:  personalDir(),
		RepoPath:     r.opts.RepoPath,
		Definition:   r.def,
		Term:         r.term,
		Config:       r.resolvers.config,
	}

	for _, spec := range r.def.Inputs {
		// Already provided via flags
		if v, ok := resolved[spec.Name]; ok && v != "" {
			continue
		}

		// Walk resolve chain
		chain := r.resolvers.ResolveChain(spec)
		found := false
		for _, strategyName := range chain {
			resolver := r.resolvers.Get(strategyName)
			if resolver == nil {
				continue
			}
			val, ok := resolver.Resolve(spec, resolved, ctx)
			if ok && val != "" {
				resolved[spec.Name] = val
				found = true
				break
			}
		}

		if !found && spec.Required && spec.Default == "" {
			return nil, fmt.Errorf("could not resolve required input %q (tried: %s)",
				spec.Name, strings.Join(chain, " -> "))
		}
	}

	return resolved, nil
}

// personalDir returns ~/.workflow/ path.
func personalDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".workflow")
}

// Run executes all steps in the definition sequentially.
// Returns results for each step executed (or skipped).
func (r *Runner) Run(inputs RunInputs) ([]StepResult, error) {
	if r.resolvers != nil {
		resolved, err := r.CollectInputs(inputs)
		if err != nil {
			return nil, err
		}
		inputs = resolved
	} else {
		if err := r.ValidateInputs(inputs); err != nil {
			return nil, err
		}
	}

	if r.def.IsAgent() {
		return nil, fmt.Errorf("use-case %q is an agent — use 'wtb agent spec %s' to inspect it, or call workflow_run via MCP",
			r.def.ID, r.def.ID)
	}

	if !r.def.IsPipeline() {
		// Documentary use-cases have no steps — scaffolding is handled by wtb new.
		return nil, fmt.Errorf("use-case %q (type: %s) has no executable steps; use 'wtb new %s' to scaffold artefacts",
			r.def.ID, r.def.Type, r.def.ID)
	}

	var results []StepResult

	for _, step := range r.def.Steps {
		result, err := r.executeStep(step, inputs)
		results = append(results, result)
		if err != nil {
			return results, fmt.Errorf("step %q failed: %w", step.Name, err)
		}
	}

	return results, nil
}

func (r *Runner) executeStep(step StepSpec, inputs RunInputs) (StepResult, error) {
	result := StepResult{StepName: step.Name}

	// Optional steps: ask or auto-skip.
	if step.Optional {
		if r.opts.AutoSkip || r.opts.DryRun {
			result.Skipped = true
			r.term.Printf("  [skip] %s (optional)\n", step.Name)
			return result, nil
		}
		if !r.term.Confirm(fmt.Sprintf("  Run optional step %q? (%s)", step.Name, step.Description)) {
			result.Skipped = true
			return result, nil
		}
	}

	// Human checkpoint: mandatory pause before execution.
	if step.HumanCheckpoint && !r.opts.DryRun {
		r.term.Printf("\n  ⚠ Checkpoint — step: %s\n  %s\n", step.Name, step.Description)
		if !r.term.Confirm("  Proceed?") {
			return result, fmt.Errorf("aborted by user at checkpoint %q", step.Name)
		}
	}

	// Apply YAML-declared input_mapping (aliases + defaults) before dispatch.
	resolvedInputs := ResolveInputs(step.InputMapping, inputs)

	// Dry-run: describe what would happen without executing.
	if r.opts.DryRun {
		r.term.Printf("  [dry-run] %s → engine: %s", step.Name, step.Engine)
		if cmds := step.AllCommands(); len(cmds) > 0 {
			r.term.Printf(" commands: %s", strings.Join(cmds, ", "))
		}
		if step.Output != "" {
			r.term.Printf(" → %s", step.Output)
		}
		r.term.Printf("\n")
		return result, nil
	}

	// Dispatch to engine.
	exec, ok := r.registry[step.Engine]
	if !ok {
		return result, fmt.Errorf("no engine registered for %q — register it in runner.DefaultRegistry()", step.Engine)
	}

	stepResult := exec(step, resolvedInputs, r.opts)
	stepResult.StepName = step.Name
	return stepResult, stepResult.Error
}
