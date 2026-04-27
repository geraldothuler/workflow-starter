// Package memory provides the context management subsystem for the workflow platform.
// Three storage layers:
//
//	Keychain     — credential values and operational IDs (security find-generic-password)
//	context.json — structured facts: thresholds, limits, config values (wtb memory set/get)
//	topic files  — narrative content: runbooks, heuristics, architecture notes (wtb memory get)
//
// The CLI is the single entry point for the LLM to read and write memory.
// It eliminates hallucination on recovery and rediscovery on structured data.
package memory

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed topic-map.yml
var defaultTopicMapYML []byte

// TopicMap maps short topic names to file paths relative to the repo root.
type TopicMap struct {
	Topics map[string]string `yaml:"topics"`
}

// LoadTopicMap reads the topic map from the repo's .claude/memory/topic-map.yml.
// Falls back to the embedded default if the file is not found.
func LoadTopicMap(repoRoot string) (*TopicMap, error) {
	custom := filepath.Join(repoRoot, ".claude", "memory", "topic-map.yml")
	data, err := os.ReadFile(custom)
	if err != nil {
		// Use embedded default
		data = defaultTopicMapYML
	}

	var tm TopicMap
	if err := yaml.Unmarshal(data, &tm); err != nil {
		return nil, fmt.Errorf("topic-map.yml: %w", err)
	}
	if tm.Topics == nil {
		tm.Topics = make(map[string]string)
	}
	return &tm, nil
}

// Resolve returns the absolute path for a topic name.
// Returns an error if the topic is not found in the map.
func (tm *TopicMap) Resolve(repoRoot, topic string) (string, error) {
	rel, ok := tm.Topics[topic]
	if !ok {
		// Build sorted suggestion list
		var known []string
		for k := range tm.Topics {
			known = append(known, k)
		}
		return "", fmt.Errorf("topic %q not found. Known topics: %s", topic, strings.Join(known, ", "))
	}

	// Memory topic files live in ~/.claude/projects/*/memory/ — resolve via home dir
	if strings.HasPrefix(rel, "memory/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home dir: %w", err)
		}
		pattern := filepath.Join(home, ".claude", "projects", "*workflow*", rel)
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return matches[0], nil
		}
		// Fall through to repo-relative resolution (topic file may also exist in repo)
	}

	abs := filepath.Join(repoRoot, rel)
	return abs, nil
}

// List returns all topic names in sorted order.
func (tm *TopicMap) List() []string {
	names := make([]string, 0, len(tm.Topics))
	for k := range tm.Topics {
		names = append(names, k)
	}
	return names
}
