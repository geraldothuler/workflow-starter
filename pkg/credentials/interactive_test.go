package credentials

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestInteractiveRunner_NoTerminal(t *testing.T) {
	runner := NewInteractiveRunner()
	// Force no-terminal check
	runner.isTerminal = func(fd uintptr) bool { return false }

	err := runner.Run(context.Background(), "echo", []string{"hello"})
	if err == nil {
		t.Fatal("expected error for no terminal")
	}
	if !errors.Is(err, ErrNoTerminal) {
		t.Errorf("expected ErrNoTerminal, got %v", err)
	}
}

func TestInteractiveRunner_WithTerminal(t *testing.T) {
	runner := NewInteractiveRunner()
	// Force terminal available
	runner.isTerminal = func(fd uintptr) bool { return true }
	// Use a mock exec that succeeds
	runner.execCommand = mockExecCommand(0, "", "")
	// Use /dev/null as stdin to avoid actual terminal interaction
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	runner.stdin = devNull

	err = runner.Run(context.Background(), "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInteractiveRunner_CommandFailure(t *testing.T) {
	runner := NewInteractiveRunner()
	runner.isTerminal = func(fd uintptr) bool { return true }
	runner.execCommand = mockExecCommand(1, "", "auth failed")
	devNull, _ := os.Open(os.DevNull)
	defer devNull.Close()
	runner.stdin = devNull

	err := runner.Run(context.Background(), "gh", []string{"auth", "login"})
	if err == nil {
		t.Fatal("expected error for failed command")
	}
}

func TestInteractiveRunner_NonInteractive(t *testing.T) {
	runner := NewInteractiveRunner()
	runner.execCommand = mockExecCommand(0, "", "")

	// Non-interactive doesn't need terminal check
	err := runner.RunNonInteractive(context.Background(), "gh", []string{"auth", "status"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInteractiveRunner_NonInteractiveFailure(t *testing.T) {
	runner := NewInteractiveRunner()
	runner.execCommand = mockExecCommand(1, "", "not authenticated")

	err := runner.RunNonInteractive(context.Background(), "gh", []string{"auth", "status"})
	if err == nil {
		t.Fatal("expected error for failed non-interactive command")
	}
}

func TestInteractiveRunner_Timeout(t *testing.T) {
	runner := NewInteractiveRunner()
	runner.isTerminal = func(fd uintptr) bool { return true }
	// Use a command that takes long
	runner.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}
	devNull, _ := os.Open(os.DevNull)
	defer devNull.Close()
	runner.stdin = devNull

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := runner.Run(ctx, "sleep", []string{"60"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
