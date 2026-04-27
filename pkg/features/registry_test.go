package features

import (
	"os"
	"path/filepath"
	"testing"
)

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from test file to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

func TestLoadRegistry(t *testing.T) {
	basePath := findProjectRoot(t)
	registry, err := LoadRegistry(basePath)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	if len(registry.Features) == 0 {
		t.Fatal("Expected features to be loaded, got 0")
	}

	if len(registry.Features) != 9 {
		t.Errorf("Expected 9 features, got %d", len(registry.Features))
	}
}

func TestGetFeature(t *testing.T) {
	basePath := findProjectRoot(t)
	registry, err := LoadRegistry(basePath)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	feature, ok := registry.GetFeature("generate_backlog")
	if !ok {
		t.Fatal("Expected to find generate_backlog feature")
	}

	if feature.Name != "Generate Backlog" {
		t.Errorf("Expected name 'Generate Backlog', got %q", feature.Name)
	}

	if feature.CLI.Command != "backlog generate" {
		t.Errorf("Expected CLI command 'backlog generate', got %q", feature.CLI.Command)
	}

	// Non-existent feature
	_, ok = registry.GetFeature("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent feature")
	}
}

func TestGetFeatureByCommand(t *testing.T) {
	basePath := findProjectRoot(t)
	registry, err := LoadRegistry(basePath)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	tests := []struct {
		command      string
		expectedName string
	}{
		{"backlog generate", "Generate Backlog"},
		{"backlog", "Generate Backlog"},
		{"spec build", "Build Specification"},
		{"spec", "Build Specification"},
		{"lens apply", "Apply Lens"},
		{"validate completion", "Validate Completion"},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			feature := registry.GetFeatureByCommand(tt.command)
			if feature == nil {
				t.Fatalf("Expected to find feature for command %q", tt.command)
			}
			if feature.Name != tt.expectedName {
				t.Errorf("For command %q, expected name %q, got %q", tt.command, tt.expectedName, feature.Name)
			}
		})
	}

	// Non-existent command
	feature := registry.GetFeatureByCommand("nonexistent")
	if feature != nil {
		t.Error("Expected nil for nonexistent command")
	}
}

func TestGetSkillsForCommand(t *testing.T) {
	basePath := findProjectRoot(t)
	registry, err := LoadRegistry(basePath)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	skills := registry.GetSkillsForCommand("backlog")
	if len(skills) != 2 {
		t.Fatalf("Expected 2 skills for backlog, got %d", len(skills))
	}

	expectedSkills := map[string]bool{
		"wtb-backlog":     false,
		"wtb-deep-dives":  false,
	}
	for _, s := range skills {
		expectedSkills[s] = true
	}
	for skill, found := range expectedSkills {
		if !found {
			t.Errorf("Expected skill %q not found", skill)
		}
	}

	// Command with no skills
	skills = registry.GetSkillsForCommand("lens")
	if len(skills) != 0 {
		t.Errorf("Expected 0 skills for lens, got %d", len(skills))
	}

	// Non-existent command
	skills = registry.GetSkillsForCommand("nonexistent")
	if skills != nil {
		t.Errorf("Expected nil skills for nonexistent, got %v", skills)
	}
}

func TestGetPatternsForCommand(t *testing.T) {
	basePath := findProjectRoot(t)
	registry, err := LoadRegistry(basePath)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	patterns := registry.GetPatternsForCommand("backlog")
	if len(patterns) != 1 {
		t.Fatalf("Expected 1 pattern for backlog, got %d", len(patterns))
	}
	if patterns[0] != "cobli-tech-stack" {
		t.Errorf("Expected pattern 'cobli-tech-stack', got %q", patterns[0])
	}
}

func TestListFeatures(t *testing.T) {
	basePath := findProjectRoot(t)
	registry, err := LoadRegistry(basePath)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	features := registry.ListFeatures()
	if len(features) != 9 {
		t.Errorf("Expected 9 features, got %d", len(features))
	}
}
