package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// DiscoverCompose finds the docker-compose file in repoRoot using discovery order from config.
// Returns the resolved path or "" if not found.
func DiscoverCompose(repoRoot string, cfg *TestEnvConfig, override string) string {
	if override != "" {
		p := filepath.Join(repoRoot, override)
		if fileExists(p) {
			return p
		}
		return ""
	}
	for _, candidate := range cfg.Compose.Discovery {
		p := filepath.Join(repoRoot, candidate)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// StopConflictingContainers stops any running containers that occupy the given ports.
// Returns the list of containers stopped.
func StopConflictingContainers(ports []int) ([]string, error) {
	running, err := runningContainerPorts()
	if err != nil {
		return nil, err
	}
	var stopped []string
	for _, port := range ports {
		if id, ok := running[port]; ok {
			if err := stopContainer(id); err != nil {
				return stopped, fmt.Errorf("stopping container %s (port %d): %w", id, port, err)
			}
			stopped = append(stopped, fmt.Sprintf("%s (port %d)", id[:12], port))
		}
	}
	return stopped, nil
}

// ComposeUp runs docker-compose up -d for the given compose file.
func ComposeUp(composeFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ComposeDown runs docker-compose down for the given compose file.
func ComposeDown(composeFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runningContainerPorts returns a map of host port → container ID.
var portRe = regexp.MustCompile(`0\.0\.0\.0:(\d+)->`)

func runningContainerPorts() (map[int]string, error) {
	out, err := exec.Command("docker", "ps", "--format", "{{.ID}} {{.Ports}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	result := make(map[int]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		id := parts[0]
		ports := ""
		if len(parts) > 1 {
			ports = parts[1]
		}
		for _, m := range portRe.FindAllStringSubmatch(ports, -1) {
			var p int
			fmt.Sscanf(m[1], "%d", &p)
			if p > 0 {
				result[p] = id
			}
		}
	}
	return result, nil
}

func stopContainer(id string) error {
	return exec.Command("docker", "stop", id).Run()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
