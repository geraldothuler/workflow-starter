package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// MCPClientFactory creates MCP client sessions.
// In production: spawns a subprocess via CommandTransport.
// In tests: uses mock sessions with canned responses.
type MCPClientFactory interface {
	Connect(ctx context.Context, spec TransportSpec) (MCPSession, error)
}

// MCPSession wraps the MCP client session methods we use.
type MCPSession interface {
	// CallTool calls a named MCP tool with arguments and returns the raw JSON result.
	CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error)

	// Close gracefully shuts down the MCP session and subprocess.
	Close() error
}

// --- Production Implementation ---

// mcpProcessFactory spawns real MCP server subprocesses.
type mcpProcessFactory struct{}

// NewMCPProcessFactory creates a factory that spawns real npx subprocesses.
func NewMCPProcessFactory() MCPClientFactory {
	return &mcpProcessFactory{}
}

// Connect spawns an MCP server subprocess and connects to it via stdio.
func (f *mcpProcessFactory) Connect(ctx context.Context, spec TransportSpec) (MCPSession, error) {
	if spec.Command == "" {
		return nil, fmt.Errorf("transport command is required")
	}

	// Build command with context timeout
	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)

	cmd := exec.CommandContext(cmdCtx, spec.Command, spec.Args...)

	// Build environment: inherit current process env + add custom vars
	env := os.Environ()
	expanded := expandEnvMap(spec.Env)
	for k, v := range expanded {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// Get stdin/stdout pipes for MCP stdio communication
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the subprocess
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start MCP server %q: %w", spec.Command, err)
	}

	return &mcpProcessSession{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		cancel: cancel,
	}, nil
}

// mcpProcessSession wraps a running MCP server subprocess.
// Note: This is a simplified implementation. When the Go MCP SDK is added,
// this will be replaced with the SDK's proper session handling.
type mcpProcessSession struct {
	cmd    *exec.Cmd
	stdin  interface{ Write([]byte) (int, error) }
	stdout interface{ Read([]byte) (int, error) }
	cancel context.CancelFunc
}

// CallTool sends a tool call to the MCP server and reads the response.
// This is a placeholder that will be replaced by Go MCP SDK session.CallTool.
func (s *mcpProcessSession) CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	// TODO: Replace with Go MCP SDK session.CallTool when SDK is integrated in Phase 4
	return nil, fmt.Errorf("MCP SDK not yet integrated — use mock session for testing")
}

// Close shuts down the subprocess.
func (s *mcpProcessSession) Close() error {
	s.cancel()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.cmd.Wait()
}

// --- Mock Implementation (for testing) ---

// MockMCPFactory returns pre-configured mock sessions.
type MockMCPFactory struct {
	Session *MockMCPSession
	Err     error // error to return from Connect
}

// Connect returns the mock session.
func (f *MockMCPFactory) Connect(ctx context.Context, spec TransportSpec) (MCPSession, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Session, nil
}

// MockCall records a tool call for assertions.
type MockCall struct {
	Name string
	Args map[string]any
}

// MockMCPSession implements MCPSession with canned responses.
type MockMCPSession struct {
	Responses map[string]json.RawMessage // tool name -> response
	Errors    map[string]error           // tool name -> error
	Calls     []MockCall                 // recorded calls
	closed    bool
}

// NewMockMCPSession creates a mock session with the given canned responses.
func NewMockMCPSession(responses map[string]json.RawMessage) *MockMCPSession {
	return &MockMCPSession{
		Responses: responses,
		Errors:    map[string]error{},
		Calls:     nil,
	}
}

// CallTool returns the canned response for the tool name.
func (m *MockMCPSession) CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	m.Calls = append(m.Calls, MockCall{Name: name, Args: args})

	if err, ok := m.Errors[name]; ok {
		return nil, err
	}

	resp, ok := m.Responses[name]
	if !ok {
		return nil, fmt.Errorf("unexpected tool call: %q", name)
	}
	return resp, nil
}

// Close marks the session as closed.
func (m *MockMCPSession) Close() error {
	m.closed = true
	return nil
}

// IsClosed returns whether Close was called.
func (m *MockMCPSession) IsClosed() bool {
	return m.closed
}
