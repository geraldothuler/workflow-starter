package credentials

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

// CommandProvider is a YAML-driven credential provider that executes external commands.
// Each instance is configured by a CommandProviderSpec loaded from a YAML file.
// This enables providers like pass, 1password, aws-ssm, keyring without any Go code.
type CommandProvider struct {
	config CommandProviderSpec

	// execCommand is the function used to create commands.
	// Defaults to exec.CommandContext; override in tests.
	execCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewCommandProvider creates a command-based provider from a YAML-loaded spec.
func NewCommandProvider(config CommandProviderSpec) *CommandProvider {
	return &CommandProvider{
		config:      config,
		execCommand: exec.CommandContext,
	}
}

// Name returns the provider identifier from config.
func (p *CommandProvider) Name() string { return p.config.ID }

// Resolve executes the resolve command and parses stdout to get the credential value.
func (p *CommandProvider) Resolve(ctx context.Context, name string) (*Credential, error) {
	spec := p.config.Resolve

	output, err := RunCommand(ctx, p.execCommand, spec, map[string]string{"name": name})
	if err != nil {
		return nil, err
	}

	value, err := parseOutput(output, spec.Parse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if value == "" {
		return nil, ErrNotFound
	}

	return &Credential{
		Name:   name,
		Value:  value,
		Source: p.config.ID,
	}, nil
}

// RunCommand executes a CommandSpec with template-expanded args and returns stdout.
// This is a shared helper used by both CommandProvider and SessionCommandProvider.
func RunCommand(ctx context.Context, execFn func(ctx context.Context, name string, args ...string) *exec.Cmd, spec CommandSpec, vars map[string]string) (string, error) {
	args, err := expandArgs(spec.Args, vars)
	if err != nil {
		return "", fmt.Errorf("failed to expand args: %w", err)
	}

	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := execFn(ctx, spec.Command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Command failure means credential not found or provider error
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() != 0 {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("command %q failed: %w (stderr: %s)", spec.Command, err, stderr.String())
	}

	return stdout.String(), nil
}

// Store executes the store command, passing the value via stdin or args template.
func (p *CommandProvider) Store(ctx context.Context, name, value string) error {
	spec := p.config.Store
	if spec.Command == "" {
		return ErrUnsupported
	}

	vars := map[string]string{"name": name, "value": value}

	args, err := expandArgs(spec.Args, vars)
	if err != nil {
		return fmt.Errorf("failed to expand store args: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()

	cmd := p.execCommand(ctx, spec.Command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Provide stdin if configured
	if spec.Input != "" {
		input, err := expandTemplate(spec.Input, vars)
		if err != nil {
			return fmt.Errorf("failed to expand store input: %w", err)
		}
		cmd.Stdin = strings.NewReader(input)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("store command %q failed: %w (stderr: %s)", spec.Command, err, stderr.String())
	}

	return nil
}

// Available checks if the external tool is installed and working.
func (p *CommandProvider) Available() bool {
	spec := p.config.Available

	ctx, cancel := context.WithTimeout(context.Background(), spec.Timeout)
	defer cancel()

	cmd := p.execCommand(ctx, spec.Command, spec.Args...)
	var devNull bytes.Buffer
	cmd.Stdout = &devNull
	cmd.Stderr = &devNull

	return cmd.Run() == nil
}

// --- Template helpers ---

// expandArgs applies Go template expansion to each argument.
// Supports {{.name}} and {{.value}} placeholders.
func expandArgs(args []string, vars map[string]string) ([]string, error) {
	result := make([]string, len(args))
	for i, arg := range args {
		expanded, err := expandTemplate(arg, vars)
		if err != nil {
			return nil, fmt.Errorf("arg[%d] %q: %w", i, arg, err)
		}
		result[i] = expanded
	}
	return result, nil
}

// expandTemplate applies Go template expansion to a string.
func expandTemplate(tmpl string, vars map[string]string) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil // fast path: no templates
	}

	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid template %q: %w", tmpl, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	return buf.String(), nil
}

// parseOutput extracts the credential value from command stdout.
// Modes:
//   - "trim": strings.TrimSpace
//   - "first_line": first non-empty line
//   - "json:.field.path": JSON field extraction
func parseOutput(output, mode string) (string, error) {
	switch {
	case mode == "trim" || mode == "":
		return strings.TrimSpace(output), nil

	case mode == "first_line":
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				return trimmed, nil
			}
		}
		return "", nil

	case strings.HasPrefix(mode, "json:"):
		path := mode[5:] // e.g., ".Parameter.Value"
		return extractJSONField(output, path)

	default:
		return "", fmt.Errorf("unsupported parse mode: %q", mode)
	}
}

// extractJSONField extracts a field from JSON output using a dot-separated path.
// Path format: ".field.subfield" or "field.subfield"
func extractJSONField(jsonStr, path string) (string, error) {
	path = strings.TrimPrefix(path, ".")
	parts := strings.Split(path, ".")

	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON output: %w", err)
	}

	current := data
	for _, part := range parts {
		obj, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("expected object at %q, got %T", part, current)
		}
		current, ok = obj[part]
		if !ok {
			return "", fmt.Errorf("field %q not found", part)
		}
	}

	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%v", v), nil
	default:
		// Marshal back to JSON for complex types
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}
