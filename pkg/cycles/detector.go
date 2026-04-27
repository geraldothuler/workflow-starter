package cycles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// CheckCycle evaluates heuristic signals in the given repo and returns a CycleResult.
// All decisions are deterministic (zero-LLM).
func CheckCycle(repoPath string) CycleResult {
	cfg, err := LoadCycleConfig(repoPath)
	if err != nil {
		return CycleResult{
			Summary: fmt.Sprintf("config error: %v", err),
			Cost:    "zero-llm",
		}
	}

	var signals []Signal
	totalScore := 0
	maxScore := 0
	var filesChanged []string
	var packagesTouched []string

	for _, sc := range cfg.Cycle.Signals {
		maxScore += sc.Weight
		var sig Signal

		switch sc.Name {
		case "git_changes":
			sig, filesChanged = evalGitChanges(repoPath, sc.Weight)
		case "tests_pass":
			sig = evalCommand(repoPath, "tests_pass", sc.Command, sc.Weight)
		case "build_pass":
			sig = evalCommand(repoPath, "build_pass", sc.Command, sc.Weight)
		case "time_elapsed":
			sig = evalTimeElapsed(repoPath, cfg.Cycle.Savepoint.Dir, sc.ThresholdMinutes, sc.Weight)
		case "packages_touched":
			sig, packagesTouched = evalPackagesTouched(repoPath, sc.Weight)
		default:
			sig = Signal{Name: sc.Name, Passed: false, Weight: sc.Weight, Detail: "unknown signal"}
		}

		if sig.Passed {
			totalScore += sig.Weight
		}
		signals = append(signals, sig)
	}

	shouldSave := totalScore >= cfg.Cycle.Threshold
	summary := fmt.Sprintf("score %d/%d (threshold: %d)", totalScore, maxScore, cfg.Cycle.Threshold)
	if shouldSave {
		summary += " — savepoint recommended"
	}

	return CycleResult{
		ShouldSavepoint: shouldSave,
		Score:           totalScore,
		MaxScore:        maxScore,
		Threshold:       cfg.Cycle.Threshold,
		Signals:         signals,
		Summary:         summary,
		FilesChanged:    filesChanged,
		PackagesTouched: packagesTouched,
		Cost:            "zero-llm",
	}
}

// WriteSavepoint renders the savepoint template and writes it to .workflow/savepoints/.
func WriteSavepoint(repoPath string, result CycleResult) (string, error) {
	cfg, err := LoadCycleConfig(repoPath)
	if err != nil {
		return "", fmt.Errorf("config error: %w", err)
	}

	dir := filepath.Join(repoPath, cfg.Cycle.Savepoint.Dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create savepoint dir: %w", err)
	}

	now := time.Now()
	filename := now.Format(cfg.Cycle.Savepoint.Format)
	outPath := filepath.Join(dir, filename)

	content, err := renderSavepoint(result, now)
	if err != nil {
		return "", fmt.Errorf("failed to render savepoint: %w", err)
	}

	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write savepoint: %w", err)
	}

	return outPath, nil
}

// --- signal evaluators ---

func evalGitChanges(repoPath string, weight int) (Signal, []string) {
	out, err := runGit(repoPath, "diff", "--stat", "HEAD")
	if err != nil {
		// Fallback: try diff against empty tree (initial commit scenario)
		out, err = runGit(repoPath, "diff", "--stat")
		if err != nil {
			return Signal{Name: "git_changes", Passed: false, Weight: weight, Detail: "git error"}, nil
		}
	}

	files := parseGitStatFiles(out)
	passed := len(files) >= 1
	detail := fmt.Sprintf("%d files changed", len(files))

	return Signal{Name: "git_changes", Passed: passed, Weight: weight, Detail: detail}, files
}

func evalCommand(repoPath, name, command string, weight int) Signal {
	if command == "" {
		return Signal{Name: name, Passed: false, Weight: weight, Detail: "no command configured"}
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Extract short detail from output
		detail := "failed"
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 {
			last := lines[len(lines)-1]
			if len(last) > 80 {
				last = last[:80] + "..."
			}
			detail = last
		}
		return Signal{Name: name, Passed: false, Weight: weight, Detail: detail}
	}

	detail := "ok"
	if name == "tests_pass" {
		detail = countTestPackages(string(output))
	} else if name == "build_pass" {
		detail = "compiled"
	}

	return Signal{Name: name, Passed: true, Weight: weight, Detail: detail}
}

// LastSavepointMarker is the path (relative to repoRoot) of the marker file
// written by wtb cycle-check --save to record when the last savepoint was created.
const LastSavepointMarker = ".workflow/last-savepoint"

func evalTimeElapsed(repoPath, _ string, thresholdMinutes, weight int) Signal {
	markerPath := filepath.Join(repoPath, LastSavepointMarker)
	info, err := os.Stat(markerPath)
	if err != nil {
		// No marker yet — treat as elapsed
		return Signal{Name: "time_elapsed", Passed: true, Weight: weight, Detail: "no prior savepoints"}
	}

	elapsed := time.Since(info.ModTime())
	minutes := int(elapsed.Minutes())
	passed := minutes >= thresholdMinutes
	detail := fmt.Sprintf("%dmin (threshold: %dmin)", minutes, thresholdMinutes)

	return Signal{Name: "time_elapsed", Passed: passed, Weight: weight, Detail: detail}
}

func evalPackagesTouched(repoPath string, weight int) (Signal, []string) {
	out, err := runGit(repoPath, "diff", "--stat", "HEAD")
	if err != nil {
		out, err = runGit(repoPath, "diff", "--stat")
		if err != nil {
			return Signal{Name: "packages_touched", Passed: false, Weight: weight, Detail: "git error"}, nil
		}
	}

	files := parseGitStatFiles(out)
	pkgs := uniquePackages(files)
	passed := len(pkgs) >= 1
	detail := fmt.Sprintf("%d packages", len(pkgs))

	return Signal{Name: "packages_touched", Passed: passed, Weight: weight, Detail: detail}, pkgs
}

// --- helpers ---

func runGit(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	return string(out), err
}

// parseGitStatFiles parses `git diff --stat` output to extract file paths.
func parseGitStatFiles(stat string) []string {
	var files []string
	for _, line := range strings.Split(stat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}
		// git diff --stat lines: " path/to/file | N +++ ---"
		// After TrimSpace: "path/to/file | N +++ ---"
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			f := strings.TrimSpace(parts[0])
			if f != "" {
				files = append(files, f)
			}
		}
	}
	return files
}

// uniquePackages extracts unique Go package directories from file paths.
func uniquePackages(files []string) []string {
	seen := map[string]bool{}
	var pkgs []string
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		dir := filepath.Dir(f)
		if !seen[dir] {
			seen[dir] = true
			pkgs = append(pkgs, dir)
		}
	}
	return pkgs
}

// countTestPackages extracts a summary from go test output.
func countTestPackages(output string) string {
	ok := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "ok") {
			ok++
		}
	}
	if ok > 0 {
		return fmt.Sprintf("%d packages ok", ok)
	}
	return "ok"
}

const savepointTmpl = `---
type: dev-cycle
date: {{.Date}}
time: {{.Time}}
score: {{.Score}}/{{.MaxScore}}
threshold: {{.Threshold}}
---

# Savepoint — {{.Date}} {{.Time}}

## Signals
{{range .Signals}}
- {{if .Passed}}pass{{else}}fail{{end}} {{.Name}} ({{.Detail}})
{{- end}}

## Files Changed
{{range .FilesChanged}}
- {{.}}
{{- end}}
{{if not .FilesChanged}}
- (none)
{{- end}}

## Packages Touched
{{range .PackagesTouched}}
- {{.}}
{{- end}}
{{if not .PackagesTouched}}
- (none)
{{- end}}
`

type savepointData struct {
	Date            string
	Time            string
	Score           int
	MaxScore        int
	Threshold       int
	Signals         []Signal
	FilesChanged    []string
	PackagesTouched []string
}

// RenderSavepoint renders the savepoint markdown for the given result and time.
// The caller is responsible for persisting the returned content.
func RenderSavepoint(result CycleResult, now time.Time) (string, error) {
	return renderSavepoint(result, now)
}

func renderSavepoint(result CycleResult, now time.Time) (string, error) {
	tmpl, err := template.New("savepoint").Parse(savepointTmpl)
	if err != nil {
		return "", err
	}

	data := savepointData{
		Date:            now.Format("2006-01-02"),
		Time:            now.Format("15:04:05"),
		Score:           result.Score,
		MaxScore:        result.MaxScore,
		Threshold:       result.Threshold,
		Signals:         result.Signals,
		FilesChanged:    result.FilesChanged,
		PackagesTouched: result.PackagesTouched,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
