package credentials

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// mockExecCommand creates a function that simulates command execution.
// It returns a command that runs the test binary with TEST_HELPER_PROCESS=1,
// which triggers the TestHelperProcess function below.
func mockExecCommand(exitCode int, stdout, stderr string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{
			"TEST_HELPER_PROCESS=1",
			fmt.Sprintf("TEST_EXIT_CODE=%d", exitCode),
			fmt.Sprintf("TEST_STDOUT=%s", stdout),
			fmt.Sprintf("TEST_STDERR=%s", stderr),
		}
		return cmd
	}
}

// TestHelperProcess is a helper that simulates external commands in tests.
// It is invoked by the test binary when TEST_HELPER_PROCESS=1 is set.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("TEST_HELPER_PROCESS") != "1" {
		return
	}

	stdout := os.Getenv("TEST_STDOUT")
	stderr := os.Getenv("TEST_STDERR")
	exitCode := 0
	fmt.Sscanf(os.Getenv("TEST_EXIT_CODE"), "%d", &exitCode)

	if stdout != "" {
		fmt.Fprint(os.Stdout, stdout)
	}
	if stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}
	os.Exit(exitCode)
}

func newTestSpec() CommandProviderSpec {
	return CommandProviderSpec{
		ID:   "test-pass",
		Name: "Test Pass Provider",
		Resolve: CommandSpec{
			Command: "pass",
			Args:    []string{"show", "wtb/{{.name}}"},
			Parse:   "first_line",
			Timeout: 5 * time.Second,
		},
		Store: CommandSpec{
			Command: "pass",
			Args:    []string{"insert", "-f", "wtb/{{.name}}"},
			Input:   "{{.value}}\n{{.value}}\n",
			Timeout: 5 * time.Second,
		},
		Available: CommandSpec{
			Command: "pass",
			Args:    []string{"ls"},
			Timeout: 5 * time.Second,
		},
		SetupGuide: "Install pass: brew install pass",
	}
}

func TestCommandProvider_Name(t *testing.T) {
	p := NewCommandProvider(newTestSpec())
	if p.Name() != "test-pass" {
		t.Errorf("expected 'test-pass', got %q", p.Name())
	}
}

func TestCommandProvider_Resolve_Success(t *testing.T) {
	spec := newTestSpec()
	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(0, "my-secret-value\nsome-other-line\n", "")

	cred, err := p.Resolve(context.Background(), "MY_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "my-secret-value" {
		t.Errorf("expected 'my-secret-value', got %q", cred.Value)
	}
	if cred.Source != "test-pass" {
		t.Errorf("expected source 'test-pass', got %q", cred.Source)
	}
	if cred.Name != "MY_TOKEN" {
		t.Errorf("expected name 'MY_TOKEN', got %q", cred.Name)
	}
}

func TestCommandProvider_Resolve_NotFound(t *testing.T) {
	spec := newTestSpec()
	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(1, "", "not found")

	_, err := p.Resolve(context.Background(), "NONEXISTENT")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCommandProvider_Resolve_EmptyOutput(t *testing.T) {
	spec := newTestSpec()
	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(0, "", "")

	_, err := p.Resolve(context.Background(), "EMPTY_TOKEN")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for empty output, got %v", err)
	}
}

func TestCommandProvider_Resolve_TrimMode(t *testing.T) {
	spec := newTestSpec()
	spec.Resolve.Parse = "trim"
	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(0, "  secret-with-spaces  \n", "")

	cred, err := p.Resolve(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "secret-with-spaces" {
		t.Errorf("expected 'secret-with-spaces', got %q", cred.Value)
	}
}

func TestCommandProvider_Store_Success(t *testing.T) {
	spec := newTestSpec()
	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(0, "", "")

	err := p.Store(context.Background(), "MY_TOKEN", "new-value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommandProvider_Store_Unsupported(t *testing.T) {
	spec := newTestSpec()
	spec.Store = CommandSpec{} // no store command
	p := NewCommandProvider(spec)

	err := p.Store(context.Background(), "MY_TOKEN", "value")
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("expected ErrUnsupported, got %v", err)
	}
}

func TestCommandProvider_Store_Failure(t *testing.T) {
	spec := newTestSpec()
	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(1, "", "permission denied")

	err := p.Store(context.Background(), "MY_TOKEN", "value")
	if err == nil {
		t.Fatal("expected error for failed store")
	}
	if !strings.Contains(err.Error(), "store command") {
		t.Errorf("expected error about store command, got: %v", err)
	}
}

func TestCommandProvider_Available_True(t *testing.T) {
	spec := newTestSpec()
	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(0, "", "")

	if !p.Available() {
		t.Error("expected Available() = true")
	}
}

func TestCommandProvider_Available_False(t *testing.T) {
	spec := newTestSpec()
	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(1, "", "command not found")

	if p.Available() {
		t.Error("expected Available() = false")
	}
}

// --- Template expansion tests ---

func TestExpandArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		vars     map[string]string
		expected []string
	}{
		{
			name:     "no templates",
			args:     []string{"show", "literal"},
			vars:     map[string]string{"name": "TOKEN"},
			expected: []string{"show", "literal"},
		},
		{
			name:     "name template",
			args:     []string{"show", "wtb/{{.name}}"},
			vars:     map[string]string{"name": "MY_TOKEN"},
			expected: []string{"show", "wtb/MY_TOKEN"},
		},
		{
			name:     "value template",
			args:     []string{"insert", "--title={{.name}}", "credential={{.value}}"},
			vars:     map[string]string{"name": "TOKEN", "value": "secret123"},
			expected: []string{"insert", "--title=TOKEN", "credential=secret123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandArgs(tt.args, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d args, got %d", len(tt.expected), len(result))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("arg[%d]: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

// --- Parse output tests ---

func TestParseOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		mode     string
		expected string
		wantErr  bool
	}{
		{
			name:     "trim mode",
			output:   "  hello  \n",
			mode:     "trim",
			expected: "hello",
		},
		{
			name:     "first_line mode",
			output:   "secret\nmetadata\n",
			mode:     "first_line",
			expected: "secret",
		},
		{
			name:     "first_line skips empty",
			output:   "\n\nsecret\n",
			mode:     "first_line",
			expected: "secret",
		},
		{
			name:     "json field",
			output:   `{"Parameter":{"Value":"my-secret"}}`,
			mode:     "json:.Parameter.Value",
			expected: "my-secret",
		},
		{
			name:     "json field no dot prefix",
			output:   `{"token":"abc123"}`,
			mode:     "json:token",
			expected: "abc123",
		},
		{
			name:    "unsupported mode",
			output:  "anything",
			mode:    "regex:.*",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseOutput(tt.output, tt.mode)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// --- Validation tests ---

func TestCommandProviderSpec_Validate(t *testing.T) {
	t.Run("valid spec", func(t *testing.T) {
		spec := newTestSpec()
		if err := spec.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing ID", func(t *testing.T) {
		spec := newTestSpec()
		spec.ID = ""
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for missing ID")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		spec := newTestSpec()
		spec.Name = ""
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for missing name")
		}
	})

	t.Run("missing resolve command", func(t *testing.T) {
		spec := newTestSpec()
		spec.Resolve.Command = ""
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for missing resolve command")
		}
	})

	t.Run("missing available command", func(t *testing.T) {
		spec := newTestSpec()
		spec.Available.Command = ""
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for missing available command")
		}
	})

	t.Run("invalid parse mode", func(t *testing.T) {
		spec := newTestSpec()
		spec.Resolve.Parse = "invalid_mode"
		if err := spec.Validate(); err == nil {
			t.Fatal("expected error for invalid parse mode")
		}
	})

	t.Run("json parse mode accepted", func(t *testing.T) {
		spec := newTestSpec()
		spec.Resolve.Parse = "json:.Parameter.Value"
		if err := spec.Validate(); err != nil {
			t.Fatalf("unexpected error for json parse mode: %v", err)
		}
	})
}

// --- extractJSONField tests ---

func TestExtractJSONField(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple field",
			json:     `{"token":"abc"}`,
			path:     "token",
			expected: "abc",
		},
		{
			name:     "nested field",
			json:     `{"data":{"secret":"xyz"}}`,
			path:     "data.secret",
			expected: "xyz",
		},
		{
			name:     "numeric field",
			json:     `{"count":42}`,
			path:     "count",
			expected: "42",
		},
		{
			name:    "invalid JSON",
			json:    "not json",
			path:    "field",
			wantErr: true,
		},
		{
			name:    "missing field",
			json:    `{"other":"value"}`,
			path:    "missing",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractJSONField(tt.json, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// --- Integration-style test with inline exec mock ---

func TestCommandProvider_FullResolveFlow(t *testing.T) {
	// Simulates: command outputs JSON, we parse a field from it
	spec := CommandProviderSpec{
		ID:   "aws-ssm-test",
		Name: "Test AWS SSM",
		Resolve: CommandSpec{
			Command: "aws",
			Args:    []string{"ssm", "get-parameter", "--name", "/wtb/{{.name}}"},
			Parse:   "json:.Parameter.Value",
			Timeout: 5 * time.Second,
		},
		Available: CommandSpec{
			Command: "aws",
			Args:    []string{"sts", "get-caller-identity"},
			Timeout: 5 * time.Second,
		},
	}

	p := NewCommandProvider(spec)
	p.execCommand = mockExecCommand(0, `{"Parameter":{"Value":"ssm-secret-123"}}`, "")

	cred, err := p.Resolve(context.Background(), "DB_PASSWORD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Value != "ssm-secret-123" {
		t.Errorf("expected 'ssm-secret-123', got %q", cred.Value)
	}
	if cred.Source != "aws-ssm-test" {
		t.Errorf("expected source 'aws-ssm-test', got %q", cred.Source)
	}
}

// --- Test with mock that captures stdin ---

func TestExpandTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		vars     map[string]string
		expected string
		wantErr  bool
	}{
		{
			name:     "no template",
			tmpl:     "literal",
			vars:     map[string]string{"name": "x"},
			expected: "literal",
		},
		{
			name:     "name substitution",
			tmpl:     "wtb/{{.name}}",
			vars:     map[string]string{"name": "MY_TOKEN"},
			expected: "wtb/MY_TOKEN",
		},
		{
			name:     "value substitution with newlines",
			tmpl:     "{{.value}}\n{{.value}}\n",
			vars:     map[string]string{"value": "secret"},
			expected: "secret\nsecret\n",
		},
		{
			name:    "invalid template",
			tmpl:    "{{.invalid",
			vars:    map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandTemplate(tt.tmpl, tt.vars)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// --- Suppress unused import warnings ---
var _ = bytes.Buffer{}
