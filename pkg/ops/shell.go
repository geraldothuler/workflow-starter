package ops

import (
	"context"
	"os"
	"os/exec"
)

// shellExec runs a command with context and returns combined output.
// Package-level variable to allow test injection without real kubectl/aws dependencies.
var shellExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// shellExecEnv runs a command with context, inheriting the current env and appending
// extraEnv entries ("KEY=VALUE"). Use to pass secrets via env var instead of CLI args
// (e.g. SNOWSQL_PWD) so they do not appear in the process argument list.
// Package-level variable to allow test injection.
var shellExecEnv = func(ctx context.Context, extraEnv []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	return cmd.CombinedOutput()
}

// shellOutput runs a command and returns stdout only.
// Package-level variable to allow test injection.
var shellOutput = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}
