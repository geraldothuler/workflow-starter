package testenv

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Run orchestrates the full testenv flow for the given repo root.
func Run(repoRoot string) (*TestEnvResult, error) {
	start := time.Now()

	globalCfg, err := LoadTestEnvConfig()
	if err != nil {
		return errorResult("failed to load config: "+err.Error(), ""), err
	}

	repoCfg, err := LoadRepoTestEnvConfig(repoRoot)
	if err != nil {
		return errorResult("failed to load .workflow/testenv.yml: "+err.Error(), ""), err
	}

	// 1. Detect runtime
	runtime := DetectRuntime(globalCfg)
	if runtime == "" {
		instructions := RuntimeInstallInstructions(globalCfg)
		return &TestEnvResult{
			Status:  "error",
			Signal:  "No Docker runtime detected",
			Actions: instructions,
		}, nil
	}

	// 2. Discover docker-compose
	composeFile := DiscoverCompose(repoRoot, globalCfg, repoCfg.Compose.File)
	if composeFile == "" {
		return &TestEnvResult{
			Status:  "error",
			Runtime: runtime,
			Signal:  "docker-compose file not found",
			Actions: []string{
				"Create docker-compose.yml in repo root",
				"Or set compose.file in .workflow/testenv.yml",
			},
		}, nil
	}

	// 3. Stop conflicting containers
	if globalCfg.Port.AutoStop && len(repoCfg.Services) > 0 {
		ports := servicePorts(repoCfg.Services)
		var stopped []string
		stopped, err = StopConflictingContainers(ports)
		if err != nil {
			return errorResult("stopping conflicting containers: "+err.Error(), runtime), err
		}
		if len(stopped) > 0 {
			fmt.Printf("Stopped conflicting containers: %s\n", strings.Join(stopped, ", "))
		}
	}

	// 4. docker-compose up
	fmt.Printf("Starting services via %s...\n", composeFile)
	if err := ComposeUp(composeFile); err != nil {
		return &TestEnvResult{
			Status:      "error",
			Runtime:     runtime,
			ComposeFile: composeFile,
			Signal:      "docker-compose up failed: " + err.Error(),
			Actions:     []string{"Check docker-compose.yml for syntax errors", "Run: docker compose -f " + composeFile + " up -d"},
		}, nil
	}

	// 5. Wait for services healthy
	var serviceResults []ServiceHealth
	if len(repoCfg.Services) > 0 {
		fmt.Printf("Waiting for %d service(s) to be healthy...\n", len(repoCfg.Services))
		serviceResults = WaitForServices(repoCfg.Services, &globalCfg.Health)
		for _, s := range serviceResults {
			fmt.Printf("  %s (:%d): %s [%s]\n", s.Name, s.Port, s.Status, s.Elapsed)
		}
		if anyUnhealthy(serviceResults) {
			return &TestEnvResult{
				Status:      "error",
				Runtime:     runtime,
				ComposeFile: composeFile,
				Services:    serviceResults,
				Signal:      "one or more services failed to become healthy",
				Actions:     []string{"Run: docker compose -f " + composeFile + " logs", "Check service configs in .workflow/testenv.yml"},
				Duration:    time.Since(start).Round(time.Millisecond).String(),
			}, nil
		}
	}

	// 6. Run tests
	if repoCfg.Tests.Command == "" {
		return &TestEnvResult{
			Status:      "skipped",
			Runtime:     runtime,
			ComposeFile: composeFile,
			Services:    serviceResults,
			Signal:      "services healthy — no tests.command set in .workflow/testenv.yml",
			Duration:    time.Since(start).Round(time.Millisecond).String(),
		}, nil
	}

	fmt.Printf("Running tests: %s\n", repoCfg.Tests.Command)
	output, exitCode := runCommand(repoCfg.Tests.Command, repoRoot, repoCfg.Tests.Env, repoCfg.Tests.TimeoutSeconds)

	status := "ok"
	signal := fmt.Sprintf("tests passed (exit %d)", exitCode)
	if exitCode != 0 {
		status = "error"
		signal = fmt.Sprintf("tests failed (exit %d)", exitCode)
	}

	return &TestEnvResult{
		Status:      status,
		Runtime:     runtime,
		ComposeFile: composeFile,
		Services:    serviceResults,
		TestOutput:  output,
		ExitCode:    exitCode,
		Signal:      signal,
		Duration:    time.Since(start).Round(time.Millisecond).String(),
	}, nil
}

func runCommand(command, dir string, env map[string]string, timeoutSecs int) (string, int) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", 1
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return buf.String(), exitCode
}

func servicePorts(services []ServiceConfig) []int {
	ports := make([]int, len(services))
	for i, s := range services {
		ports[i] = s.Port
	}
	return ports
}

func anyUnhealthy(services []ServiceHealth) bool {
	for _, s := range services {
		if s.Status != "healthy" {
			return true
		}
	}
	return false
}

func errorResult(msg, runtime string) *TestEnvResult {
	return &TestEnvResult{
		Status:  "error",
		Runtime: runtime,
		Signal:  msg,
	}
}
