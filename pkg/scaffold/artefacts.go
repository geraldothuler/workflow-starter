// Package scaffold provides helpers for creating and listing workflow artefacts.
// Extracted from cmd/wtb/main.go so that pkg/mcp and other packages can reuse
// the logic without importing main.
package scaffold

import (
	"fmt"
	"os"
	"sort"
)

// WorkflowTypes lists the organisational workflow types that support scaffolding
// via wtb new. Technical use-cases (backlog, investigation, ops-response) are
// run via wtb run and do not use templates.
var WorkflowTypes = []string{"incident", "postmortem", "review", "1on1"}

// IsValidType reports whether t is a supported organisational workflow type.
func IsValidType(t string) bool {
	for _, v := range WorkflowTypes {
		if v == t {
			return true
		}
	}
	return false
}

// NextNNN returns the zero-padded next sequence number for artefacts in dir.
// If dir does not exist, returns "001".
func NextNNN(dir string) (string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "001", nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	count := 0
	for _, e := range entries {
		name := e.Name()
		if name == "INDEX.md" {
			continue
		}
		if len(name) >= 3 && isDigit(name[0]) && isDigit(name[1]) && isDigit(name[2]) {
			count++
		}
	}
	return fmt.Sprintf("%03d", count+1), nil
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// ListArtefacts returns a sorted slice of artefact names (files/dirs with NNN
// prefix) found in typeDir, excluding INDEX.md.
func ListArtefacts(typeDir string) ([]string, error) {
	entries, err := os.ReadDir(typeDir)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		name := e.Name()
		if name == "INDEX.md" {
			continue
		}
		if len(name) >= 3 && isDigit(name[0]) && isDigit(name[1]) && isDigit(name[2]) {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result, nil
}

// ExtractNNN returns the 3-char NNN prefix of an artefact name.
func ExtractNNN(name string) string {
	if len(name) >= 3 {
		return name[:3]
	}
	return name
}
