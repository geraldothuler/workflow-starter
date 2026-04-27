package context

import (
	"embed"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed config/*.yml
var embeddedMarkers embed.FS

// RepoStack holds detected technology categories for a repository.
type RepoStack struct {
	Backend        []string
	Database       []string
	Queue          []string
	Infrastructure []string
	CICD           []string
	Frontend       []string
}

// markerConfig mirrors the YAML structure in repo_markers.yml.
type markerConfig struct {
	Markers map[string][]marker `yaml:"markers"`
}

type marker struct {
	ID             string            `yaml:"id"`
	Name           string            `yaml:"name"`
	Files          []string          `yaml:"files"`
	Patterns       map[string]string `yaml:"patterns"`
	EnvPatterns    []string          `yaml:"env_patterns"`
	DockerPatterns []string          `yaml:"docker_patterns"`
}

// DetectStack scans repoPath for technology markers and returns the detected stack.
func DetectStack(repoPath string) (*RepoStack, error) {
	data, err := embeddedMarkers.ReadFile("config/repo_markers.yml")
	if err != nil {
		return nil, err
	}
	var cfg markerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	stack := &RepoStack{}
	categoryField := map[string]*[]string{
		"backend":        &stack.Backend,
		"database":       &stack.Database,
		"queue":          &stack.Queue,
		"infrastructure": &stack.Infrastructure,
		"cicd":           &stack.CICD,
		"frontend":       &stack.Frontend,
	}

	for category, markers := range cfg.Markers {
		field, ok := categoryField[category]
		if !ok {
			continue
		}
		for _, m := range markers {
			if matchMarker(repoPath, m) {
				*field = append(*field, m.Name)
			}
		}
	}
	return stack, nil
}

// matchMarker returns true if any of the marker's conditions match in the repo.
func matchMarker(repoPath string, m marker) bool {
	// Check file/dir existence
	for _, f := range m.Files {
		if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
			return true
		}
	}
	// Check content patterns (read file, match regex)
	for file, pattern := range m.Patterns {
		content, err := os.ReadFile(filepath.Join(repoPath, file))
		if err == nil {
			if matched, _ := regexp.MatchString(pattern, string(content)); matched {
				return true
			}
		}
	}
	// Scan env files for env_patterns
	if len(m.EnvPatterns) > 0 {
		envContent := readFiles(repoPath, ".env", "docker-compose.yml")
		for _, pat := range m.EnvPatterns {
			if strings.Contains(envContent, pat) {
				return true
			}
		}
	}
	// Scan docker files for docker_patterns
	if len(m.DockerPatterns) > 0 {
		dockerContent := readFiles(repoPath, "Dockerfile", "docker-compose.yml")
		for _, pat := range m.DockerPatterns {
			if strings.Contains(dockerContent, pat) {
				return true
			}
		}
	}
	return false
}

// readFiles concatenates contents of the given filenames under repoPath.
func readFiles(repoPath string, names ...string) string {
	var sb strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(repoPath, name))
		if err == nil {
			sb.Write(data)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
