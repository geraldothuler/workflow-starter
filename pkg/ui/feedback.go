package ui

import (
	"fmt"
	"strings"
	"time"
)

// Colors for terminal output
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Bold    = "\033[1m"
)

// Box draws a box around content
func Box(title string) string {
	width := 53
	top := "┌" + strings.Repeat("─", width) + "┐"
	middle := fmt.Sprintf("│  %s%-*s│", Bold, width-2, title+Reset)
	bottom := "└" + strings.Repeat("─", width) + "┘"
	return top + "\n" + middle + "\n" + bottom
}

// Section creates a section header
func Section(number, total int, title string) string {
	separator := strings.Repeat("━", 53)
	return fmt.Sprintf("\n[%d/%d] %s\n%s\n", number, total, title, separator)
}

// Success prints a success message
func Success(msg string) {
	fmt.Printf("%s✓%s %s\n", Green, Reset, msg)
}

// Error prints an error message
func Error(msg string) {
	fmt.Printf("%s✗%s %s\n", Red, Reset, msg)
}

// Warning prints a warning message
func Warning(msg string) {
	fmt.Printf("%s⚠️ %s %s\n", Yellow, Reset, msg)
}

// Info prints an info message
func Info(msg string) {
	fmt.Printf("%s💡%s %s\n", Cyan, Reset, msg)
}

// Step represents a process step
type Step struct {
	Number      int
	Total       int
	Title       string
	Description string
	Status      string // "pending", "running", "done", "failed"
}

// Render renders the step
func (s *Step) Render() {
	fmt.Print(Section(s.Number, s.Total, s.Title))
	if s.Description != "" {
		fmt.Printf("\n  %s\n", s.Description)
	}
}

// ProgressBar shows progress visually
type ProgressBar struct {
	Total   int
	Current int
	Label   string
}

// Render renders the progress bar
func (p *ProgressBar) Render() string {
	if p.Total == 0 {
		return ""
	}
	percent := float64(p.Current) / float64(p.Total)
	filled := int(percent * 40)
	empty := 40 - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("  ⏳ [%s] %s", bar, p.Label)
}

// Update updates and re-renders the progress bar
func (p *ProgressBar) Update(current int, label string) {
	p.Current = current
	p.Label = label
	fmt.Printf("\r%s", p.Render())
}

// Complete marks the progress bar as complete
func (p *ProgressBar) Complete(label string) {
	p.Current = p.Total
	p.Label = label
	fmt.Printf("\r%s\n", p.Render())
}

// NextStep represents a suggested next action
type NextStep struct {
	Command     string
	Description string
	Rationale   string
	Actions     []Action
}

// Action represents an actionable choice
type Action struct {
	Key         string
	Description string
	Command     string
}

// Display shows the next step suggestion
func (n *NextStep) Display() {
	fmt.Println()
	fmt.Println(Box("💡 RECOMMENDED NEXT STEP"))
	fmt.Println()
	fmt.Printf("%s\n\n", n.Description)
	
	if n.Command != "" {
		fmt.Printf("  $ %s%s%s\n", Cyan, n.Command, Reset)
	}
	
	if n.Rationale != "" {
		fmt.Printf("\n%s\n", n.Rationale)
	}
	
	if len(n.Actions) > 0 {
		fmt.Println()
		fmt.Println(Box("🎯 QUICK ACTIONS"))
		fmt.Println()
		for _, action := range n.Actions {
			fmt.Printf("  [%s%s%s] %s\n", Yellow, action.Key, Reset, action.Description)
			if action.Command != "" {
				fmt.Printf("      $ %s%s%s\n", Cyan, action.Command, Reset)
			}
		}
		fmt.Println()
	}
}

// Summary shows a summary of results
type Summary struct {
	Title   string
	Items   []SummaryItem
	Metrics []Metric
}

// SummaryItem is a key-value summary item
type SummaryItem struct {
	Label string
	Value string
	Icon  string
}

// Metric shows a metric with value
type Metric struct {
	Label string
	Value string
}

// Display renders the summary
func (s *Summary) Display() {
	fmt.Println()
	if s.Title != "" {
		fmt.Println(Box(s.Title))
		fmt.Println()
	}
	
	if len(s.Items) > 0 {
		fmt.Println("📁 Created files:")
		for _, item := range s.Items {
			icon := item.Icon
			if icon == "" {
				icon = "✓"
			}
			fmt.Printf("   %s%s%s %-20s %s\n", Green, icon, Reset, item.Label, item.Value)
		}
		fmt.Println()
	}
	
	if len(s.Metrics) > 0 {
		fmt.Println("📊 Summary:")
		for _, metric := range s.Metrics {
			fmt.Printf("   %-20s %s\n", metric.Label+":", metric.Value)
		}
		fmt.Println()
	}
}

// QualityReport shows input/output quality analysis
type QualityReport struct {
	OverallScore int
	Items        []QualityItem
	Suggestions  []string
}

// QualityItem represents a quality metric
type QualityItem struct {
	Name  string
	Score int
	Icon  string
}

// Display renders the quality report
func (q *QualityReport) Display() {
	fmt.Println()
	fmt.Println(Box("📊 QUALITY REPORT"))
	fmt.Println()
	
	for _, item := range q.Items {
		icon := "✓"
		color := Green
		if item.Score < 50 {
			icon = "⚠️ "
			color = Red
		} else if item.Score < 80 {
			icon = "⚠️ "
			color = Yellow
		}
		
		fmt.Printf("  %s%s%s %-20s %s%d%%%s\n", 
			color, icon, Reset, item.Name+":", color, item.Score, Reset)
	}
	
	fmt.Println()
	fmt.Printf("🎯 Overall Quality: %s%d%%%s", 
		scoreColor(q.OverallScore), q.OverallScore, Reset)
	
	if q.OverallScore < 70 {
		fmt.Printf(" %s(needs refinement)%s\n", Yellow, Reset)
	} else if q.OverallScore < 85 {
		fmt.Printf(" %s(good with minor refinement)%s\n", Green, Reset)
	} else {
		fmt.Printf(" %s(excellent)%s\n", Green, Reset)
	}
	fmt.Println()
	
	if len(q.Suggestions) > 0 {
		fmt.Println(strings.Repeat("━", 53))
		fmt.Println()
		fmt.Println("💡 RECOMMENDATIONS:")
		fmt.Println()
		for i, suggestion := range q.Suggestions {
			fmt.Printf("%d. %s\n", i+1, suggestion)
		}
		fmt.Println()
	}
}

// scoreColor returns color based on score
func scoreColor(score int) string {
	if score < 50 {
		return Red
	} else if score < 80 {
		return Yellow
	}
	return Green
}

// ValidationError represents a validation error with context
type ValidationError struct {
	File    string
	Line    int
	Column  int
	Message string
	Current string
	Suggest string
}

// Display renders the validation error
func (v *ValidationError) Display() {
	fmt.Printf("%s❌ VALIDATION ERROR%s\n\n", Red+Bold, Reset)
	fmt.Printf("%s:\n", v.File)
	if v.Line > 0 {
		fmt.Printf("  Line %d: %s\n", v.Line, v.Message)
	} else {
		fmt.Printf("  %s\n", v.Message)
	}
	
	if v.Current != "" {
		fmt.Printf("\n  Current:\n")
		fmt.Printf("    %s%s%s\n", Red, v.Current, Reset)
	}
	
	if v.Suggest != "" {
		fmt.Printf("\n  Suggested:\n")
		fmt.Printf("    %s%s%s\n", Green, v.Suggest, Reset)
	}
	fmt.Println()
}

// Timer tracks and displays elapsed time
type Timer struct {
	start time.Time
	label string
}

// NewTimer creates a new timer
func NewTimer(label string) *Timer {
	return &Timer{
		start: time.Now(),
		label: label,
	}
}

// Elapsed returns formatted elapsed time
func (t *Timer) Elapsed() string {
	duration := time.Since(t.start)
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	}
	return fmt.Sprintf("%dm %ds", int(duration.Minutes()), int(duration.Seconds())%60)
}

// Stop stops the timer and displays result
func (t *Timer) Stop() {
	fmt.Printf("  %sTime:%s %s\n", Cyan, Reset, t.Elapsed())
}

// Example shows an example (good or bad)
type Example struct {
	Title   string
	Content string
	IsGood  bool
	Reasons []string
}

// Display renders the example
func (e *Example) Display() {
	icon := "✅"
	label := "GOOD EXAMPLE"
	
	if !e.IsGood {
		icon = "❌"
		label = "BAD EXAMPLE"
	}
	
	fmt.Println()
	fmt.Println(Box(icon + " " + label))
	fmt.Println()
	
	if e.Title != "" {
		fmt.Printf("%s%s%s\n\n", Bold, e.Title, Reset)
	}
	
	fmt.Println(e.Content)
	fmt.Println()
	
	if len(e.Reasons) > 0 {
		if e.IsGood {
			fmt.Printf("%s✅ WHY THIS IS GOOD:%s\n", Green, Reset)
		} else {
			fmt.Printf("%s⚠️  WHY THIS IS BAD:%s\n", Red, Reset)
		}
		
		for _, reason := range e.Reasons {
			fmt.Printf("  • %s\n", reason)
		}
		fmt.Println()
	}
}

// Confirm asks for user confirmation
func Confirm(message string, defaultValue ...bool) bool {
	fmt.Printf("\n%s [Y/n] ", message)
	var response string
	fmt.Scanln(&response)
	return response == "" || strings.ToLower(response) == "y"
}

// Prompt asks for user input
func Prompt(message string) string {
	fmt.Printf("\n%s\n→ ", message)
	var response string
	fmt.Scanln(&response)
	return response
}

// Select shows a menu and returns selection
func Select(message string, options []string) int {
	fmt.Printf("\n%s\n", message)
	for i, option := range options {
		if i == 0 {
			fmt.Printf("  %s›%s %s\n", Cyan, Reset, option)
		} else {
			fmt.Printf("    %s\n", option)
		}
	}
	
	fmt.Printf("\n→ ")
	var choice int
	fmt.Scanln(&choice)
	
	if choice < 0 || choice >= len(options) {
		return 0
	}
	return choice
}

// Spinner shows a loading spinner
type Spinner struct {
	message string
	frames  []string
	current int
}

// NewSpinner creates a new spinner
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current: 0,
	}
}

// Tick advances the spinner
func (s *Spinner) Tick() {
	fmt.Printf("\r  %s %s", s.frames[s.current], s.message)
	s.current = (s.current + 1) % len(s.frames)
}

// Stop stops the spinner with a message
func (s *Spinner) Stop(message string) {
	fmt.Printf("\r  %s✓%s %s\n", Green, Reset, message)
}

// ProgressTracker rastreador de progresso
type ProgressTracker struct{}

// NewProgressTracker cria tracker
func NewProgressTracker(message string, total int) *ProgressTracker {
	return &ProgressTracker{}
}

// SuggestNext sugere próximo passo
func SuggestNext(message string) {
	fmt.Println("💡 " + message)
}

// Step marca passo
func (p *ProgressTracker) Step(step int, message string) {
	fmt.Println("⏳ " + message)
}

// Error marca erro
func (p *ProgressTracker) Error(message string) {
	fmt.Println("❌ " + message)
}

// Success marca sucesso
func (p *ProgressTracker) Success(message string) {
	fmt.Println("✅ " + message)
}

// Info mostra info
func (p *ProgressTracker) Info(message string) {
	fmt.Println("ℹ️  " + message)
}

// Warning mostra warning
func (p *ProgressTracker) Warning(message string) {
	fmt.Println("⚠️  " + message)
}

// Complete marca como completo
func (p *ProgressTracker) Complete() {
	fmt.Println("✅ Completo!")
}

// GetNextStep retorna próximo passo
func GetNextStep(context ...string) string {
	return "Continue com: wtb run backlog"
}

// InputMultiline input multiline
func InputMultiline(prompt string) (string, error) {
	fmt.Print(prompt + ": ")
	return "", nil
}

// SelectOption opção de seleção
type SelectOption struct {
	Label string
	Value string
}

// Input input simples
func Input(prompt string, defaultValue ...string) (string, error) {
	fmt.Print(prompt + ": ")
	return "", nil
}

// SelectString seleção que retorna string
func SelectString(message string, options []SelectOption) (SelectOption, error) {
	if len(options) == 0 {
		return SelectOption{}, nil
	}
	if len(options) > 0 { return options[0], nil }; return SelectOption{}, nil
}
