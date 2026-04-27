package credentials

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// InteractiveRunner executes commands with terminal access (stdin/stdout/stderr attached).
// Used for session credential refresh flows that require user interaction
// (browser SSO, MFA prompts, etc.).
type InteractiveRunner struct {
	// isTerminal checks if file descriptor is a terminal.
	// Defaults to isattyCheck; override in tests.
	isTerminal func(fd uintptr) bool

	// execCommand creates the exec.Cmd. Defaults to exec.CommandContext.
	execCommand func(ctx context.Context, name string, args ...string) *exec.Cmd

	// stdin/stdout/stderr for the interactive command.
	// Defaults to os.Stdin, os.Stdout, os.Stderr.
	stdin  *os.File
	stdout *os.File
	stderr *os.File
}

// NewInteractiveRunner creates a runner for interactive commands.
func NewInteractiveRunner() *InteractiveRunner {
	return &InteractiveRunner{
		isTerminal:  isattyCheck,
		execCommand: exec.CommandContext,
		stdin:       os.Stdin,
		stdout:      os.Stdout,
		stderr:      os.Stderr,
	}
}

// Run executes a command interactively with terminal attached.
// Returns ErrNoTerminal if no TTY is available (e.g., in CI environments).
func (r *InteractiveRunner) Run(ctx context.Context, command string, args []string) error {
	// Verify terminal availability
	if !r.isTerminal(r.stdin.Fd()) {
		return fmt.Errorf("%w: stdin is not a terminal (running in CI or pipe?)", ErrNoTerminal)
	}

	cmd := r.execCommand(ctx, command, args...)
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("interactive command %q failed: %w", command, err)
	}

	return nil
}

// RunNonInteractive executes a command without terminal (stdout/stderr captured).
// Used for pre_check commands that just need to verify session status.
func (r *InteractiveRunner) RunNonInteractive(ctx context.Context, command string, args []string) error {
	cmd := r.execCommand(ctx, command, args...)
	// Suppress output — we only care about exit code
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run()
}

// isattyCheck checks if a file descriptor is a terminal.
// Uses a simple heuristic: Stat the fd — if it's a character device, it's likely a terminal.
func isattyCheck(fd uintptr) bool {
	file := os.NewFile(fd, "")
	if file == nil {
		return false
	}
	fi, err := file.Stat()
	if err != nil {
		return false
	}
	// Character devices (terminals) have ModeCharDevice set
	return fi.Mode()&os.ModeCharDevice != 0
}
