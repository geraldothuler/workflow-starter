package ops

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// StepResult records the outcome of a single plan step execution.
type StepResult struct {
	StepID    int    `json:"step_id"`
	Tool      string `json:"tool"`
	Owner     string `json:"owner"`
	Status    string `json:"status"`  // ok | warn | critical | error | skipped | pending
	Signal    string `json:"signal"`
	Skipped   bool   `json:"skipped,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	Duration  string `json:"duration,omitempty"`
}

// PlanExecuteConfig holds parameters for plan execution.
type PlanExecuteConfig struct {
	DryRun    bool      // print steps without executing
	BinaryPath string   // path to wtb binary (default: os.Executable)
	Stdout    io.Writer // where to write human-facing output (default: os.Stdout)
	Stdin     io.Reader // where to read human confirmations (default: os.Stdin)
}

// ExecutePlan runs each step of the plan in sequence.
//
// owner=auto steps are executed automatically (using the wtb binary or shell).
// owner=human steps print the tool string and wait for Enter before proceeding.
// Returns an OpsResult summarising the execution.
func ExecutePlan(plan Plan, cfg PlanExecuteConfig) OpsResult {
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.BinaryPath == "" {
		if exe, err := os.Executable(); err == nil {
			cfg.BinaryPath = exe
		} else {
			cfg.BinaryPath = "wtb"
		}
	}

	b := plan.Plan
	var results []StepResult
	var aborted bool
	var abortReason string

	fmt.Fprintf(cfg.Stdout, "\n┌─ Executing plan: %s\n", b.ID)
	fmt.Fprintf(cfg.Stdout, "│  Scenario: %s\n", b.Scenario)
	if b.Context != "" {
		fmt.Fprintf(cfg.Stdout, "│  Context:  %s\n", b.Context)
	}
	fmt.Fprintf(cfg.Stdout, "│  Risk: %s  Steps: %d\n", b.Risk, len(b.Steps))
	fmt.Fprintf(cfg.Stdout, "└─────────────────────────────────────\n\n")

	for _, step := range b.Steps {
		if aborted {
			results = append(results, StepResult{
				StepID:  step.ID,
				Tool:    step.Tool,
				Owner:   step.Owner,
				Status:  "skipped",
				Skipped: true,
				Signal:  "aborted before this step",
			})
			continue
		}

		ownerTag := "[auto]"
		if step.Owner == "human" {
			ownerTag = "[human]"
		}
		fmt.Fprintf(cfg.Stdout, "── Step %d %s: %s\n", step.ID, ownerTag, step.Purpose)
		fmt.Fprintf(cfg.Stdout, "   %s\n", step.Tool)
		if step.ProceedIf != "" {
			fmt.Fprintf(cfg.Stdout, "   proceed_if: %s\n", step.ProceedIf)
		}

		if cfg.DryRun {
			fmt.Fprintf(cfg.Stdout, "   [dry-run — not executed]\n\n")
			results = append(results, StepResult{
				StepID:  step.ID,
				Tool:    step.Tool,
				Owner:   step.Owner,
				Status:  "skipped",
				Skipped: true,
				Signal:  "dry-run",
			})
			continue
		}

		var sr StepResult
		start := time.Now()

		if step.Owner == "human" {
			sr = executeHumanStep(step, cfg)
		} else {
			sr = executeAutoStep(step, cfg.BinaryPath)
		}

		sr.Duration = fmt.Sprintf("%.1fs", time.Since(start).Seconds())
		sr.StartedAt = start.UTC().Format(time.RFC3339)
		results = append(results, sr)

		printStepResult(cfg.Stdout, sr)

		// Abort plan if a step fails critically.
		if sr.Status == "error" || sr.Status == "critical" {
			aborted = true
			abortReason = fmt.Sprintf("step %d failed: %s", step.ID, sr.Signal)
		}
	}

	return buildExecutionResult(b, results, aborted, abortReason)
}

// executeAutoStep runs an auto step by invoking the wtb binary.
// Tool strings starting with "#" are treated as informational and skipped automatically.
func executeAutoStep(step PlanStep, binaryPath string) StepResult {
	tool := strings.TrimSpace(step.Tool)

	// Comments / manual SQL — cannot be auto-executed.
	if strings.HasPrefix(tool, "#") {
		return StepResult{
			StepID:  step.ID,
			Tool:    tool,
			Owner:   step.Owner,
			Status:  "skipped",
			Skipped: true,
			Signal:  "passo manual declarado como auto — altere owner para human",
		}
	}

	// Split "wtb ops ..." into args for the binary.
	parts := strings.Fields(tool)
	if len(parts) == 0 {
		return StepResult{StepID: step.ID, Tool: tool, Owner: "auto", Status: "error", Signal: "tool string vazia"}
	}

	// Replace "wtb" with the actual binary path for self-invocation.
	var cmdArgs []string
	if parts[0] == "wtb" {
		cmdArgs = append([]string{binaryPath}, parts[1:]...)
	} else {
		// Non-wtb commands: run via sh -c for flexibility.
		cmdArgs = []string{"sh", "-c", tool}
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	if err != nil {
		return StepResult{
			StepID: step.ID,
			Tool:   tool,
			Owner:  "auto",
			Status: "error",
			Signal: fmt.Sprintf("falha: %v — %s", err, truncate(outStr, 200)),
		}
	}

	// Extract signal from the output (first non-empty line).
	signal := firstLine(outStr)
	return StepResult{
		StepID: step.ID,
		Tool:   tool,
		Owner:  "auto",
		Status: "ok",
		Signal: signal,
	}
}

// executeHumanStep pauses and waits for human confirmation.
func executeHumanStep(step PlanStep, cfg PlanExecuteConfig) StepResult {
	fmt.Fprintf(cfg.Stdout, "\n   ⚠️  PASSO HUMANO — aprovação necessária\n")
	if step.Rollback != "" {
		fmt.Fprintf(cfg.Stdout, "   Rollback: %s\n", step.Rollback)
	}
	fmt.Fprintf(cfg.Stdout, "\n   Execute o comando acima manualmente, depois pressione:\n")
	fmt.Fprintf(cfg.Stdout, "   [Enter] para continuar  |  [s] + Enter para pular  |  Ctrl+C para abortar\n")
	fmt.Fprintf(cfg.Stdout, "   > ")

	scanner := bufio.NewScanner(cfg.Stdin)
	if scanner.Scan() {
		input := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if input == "s" || input == "skip" {
			fmt.Fprintf(cfg.Stdout, "   Passo pulado.\n\n")
			return StepResult{
				StepID:  step.ID,
				Tool:    step.Tool,
				Owner:   "human",
				Status:  "skipped",
				Skipped: true,
				Signal:  "pulado pelo operador",
			}
		}
	}

	fmt.Fprintf(cfg.Stdout, "   Passo marcado como concluído.\n\n")
	return StepResult{
		StepID: step.ID,
		Tool:   step.Tool,
		Owner:  "human",
		Status: "ok",
		Signal: "concluído pelo operador",
	}
}

func printStepResult(w io.Writer, sr StepResult) {
	icon := "✅"
	switch sr.Status {
	case "error", "critical":
		icon = "❌"
	case "warn":
		icon = "⚠️ "
	case "skipped":
		icon = "⏭️ "
	}
	dur := ""
	if sr.Duration != "" {
		dur = " (" + sr.Duration + ")"
	}
	fmt.Fprintf(w, "   %s %s%s\n\n", icon, sr.Signal, dur)
}

func buildExecutionResult(b PlanBody, results []StepResult, aborted bool, abortReason string) OpsResult {
	total := len(results)
	ok, skipped, failed := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "ok", "warn":
			ok++
		case "skipped":
			skipped++
		default:
			failed++
		}
	}

	status := "ok"
	var signal string

	if aborted {
		status = "critical"
		signal = fmt.Sprintf("plan '%s' abortado — %s (%d/%d passos ok)", b.ID, abortReason, ok, total)
	} else if failed > 0 {
		status = "warn"
		signal = fmt.Sprintf("plan '%s' concluído com %d falha(s) — %d/%d passos ok", b.ID, failed, ok, total)
	} else {
		signal = fmt.Sprintf("plan '%s' concluído — %d/%d passos ok, %d pulados", b.ID, ok, total, skipped)
	}

	return OpsResult{
		Status:  status,
		Signal:  signal,
		Data:    map[string]any{"plan_id": b.ID, "scenario": b.Scenario, "steps": results, "total": total, "ok": ok, "skipped": skipped, "failed": failed},
		Actions: []string{},
		Cost:    "zero-llm",
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx != -1 {
		return s[:idx]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
