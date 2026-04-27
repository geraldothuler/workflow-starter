package testenv

import (
	"os/exec"
	"strings"
)

// DetectRuntime returns the name of the active Docker runtime
// ("docker-desktop", "colima") or "" if none is running.
func DetectRuntime(cfg *TestEnvConfig) string {
	for _, r := range cfg.Runtimes {
		if isRuntimeRunning(r) {
			return r.Name
		}
	}
	return ""
}

// RuntimeInstallInstructions returns the install URL for each known runtime.
func RuntimeInstallInstructions(cfg *TestEnvConfig) []string {
	lines := []string{"No Docker runtime detected. Install one of:"}
	for _, r := range cfg.Runtimes {
		lines = append(lines, "  • "+r.Name+": "+r.InstallURL)
	}
	return lines
}

func isRuntimeRunning(r RuntimeConfig) bool {
	parts := strings.Fields(r.CheckCmd)
	if len(parts) == 0 {
		return false
	}
	out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput()
	if err != nil {
		return false
	}
	return r.CheckPattern == "" || strings.Contains(string(out), r.CheckPattern)
}
