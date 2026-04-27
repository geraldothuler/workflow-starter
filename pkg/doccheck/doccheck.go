// Package doccheck validates documentation against actual codebase state.
// Used as a Go test guardrail to catch drift between docs and code.
package doccheck

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FindGoPackages returns all package directory names under pkg/.
func FindGoPackages(rootDir string) ([]string, error) {
	pkgDir := filepath.Join(rootDir, "pkg")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}

	var packages []string
	for _, e := range entries {
		if e.IsDir() {
			// Verify it contains at least one .go file
			goFiles, _ := filepath.Glob(filepath.Join(pkgDir, e.Name(), "*.go"))
			if len(goFiles) > 0 {
				packages = append(packages, e.Name())
			}
		}
	}
	return packages, nil
}

// FindCommandFiles returns all .go command files (excluding root.go and test files).
func FindCommandFiles(rootDir string) ([]string, error) {
	cmdDir := filepath.Join(rootDir, "cmd", "wtb")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil, err
	}

	var commands []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") && name != "root.go" {
			commands = append(commands, name)
		}
	}
	return commands, nil
}

// GoVersionFromMod reads the Go version from go.mod.
func GoVersionFromMod(rootDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)
	matches := re.FindSubmatch(data)
	if len(matches) < 2 {
		return "", nil
	}
	return string(matches[1]), nil
}

// DocContains checks if a documentation file contains a given string.
func DocContains(rootDir, docPath, needle string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, docPath))
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), needle), nil
}

// SkillDirs returns all skill directory names under skills/.
func SkillDirs(rootDir string) ([]string, error) {
	skillsDir := filepath.Join(rootDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []string
	for _, e := range entries {
		if e.IsDir() {
			skills = append(skills, e.Name())
		}
	}
	return skills, nil
}

// MarkdownLink represents a link found in a markdown document.
type MarkdownLink struct {
	Text string
	Path string
	Line int
}

// FindMarkdownLinks extracts all markdown links [text](path) from a document.
// Only returns relative links (not http/https/mailto/# anchors).
func FindMarkdownLinks(rootDir, docPath string) ([]MarkdownLink, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, docPath))
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	lines := strings.Split(string(data), "\n")

	var links []MarkdownLink
	for i, line := range lines {
		matches := re.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			path := m[2]
			// Skip external links, anchors, and image badges
			if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") ||
				strings.HasPrefix(path, "mailto:") || strings.HasPrefix(path, "#") {
				continue
			}
			// Strip anchor from path (e.g., "file.md#section" -> "file.md")
			if idx := strings.Index(path, "#"); idx > 0 {
				path = path[:idx]
			}
			links = append(links, MarkdownLink{
				Text: m[1],
				Path: path,
				Line: i + 1,
			})
		}
	}
	return links, nil
}

// FindBrokenLinks returns markdown links from a document that point to non-existent files.
func FindBrokenLinks(rootDir, docPath string) ([]MarkdownLink, error) {
	links, err := FindMarkdownLinks(rootDir, docPath)
	if err != nil {
		return nil, err
	}

	// Resolve links relative to the document's directory
	docDir := filepath.Dir(filepath.Join(rootDir, docPath))

	var broken []MarkdownLink
	for _, link := range links {
		// Resolve absolute vs relative
		var target string
		if filepath.IsAbs(link.Path) {
			target = link.Path
		} else {
			target = filepath.Join(docDir, link.Path)
		}
		target = filepath.Clean(target)

		if _, err := os.Stat(target); os.IsNotExist(err) {
			broken = append(broken, link)
		}
	}
	return broken, nil
}

// FindLLMProviders reads pkg/llm/client.go and returns provider constant values.
// Parses lines like: ProviderClaude Provider = "claude"
func FindLLMProviders(rootDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, "pkg", "llm", "client.go"))
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`Provider\w+\s+Provider\s*=\s*"(\w+)"`)
	matches := re.FindAllStringSubmatch(string(data), -1)

	var providers []string
	for _, m := range matches {
		providers = append(providers, m[1])
	}
	return providers, nil
}

// StatusYmlSkillsCount reads the "skills:" stat from STATUS.yml.
func StatusYmlSkillsCount(rootDir string) (int, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, "docs", "STATUS.yml"))
	if err != nil {
		return 0, err
	}

	// Match the stats section "skills: N" (with possible comment after)
	re := regexp.MustCompile(`(?m)^\s+skills:\s+(\d+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) < 2 {
		return 0, nil
	}
	count, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	return count, nil
}

// StatusYmlRegisteredSkills extracts skill names from the skills section in STATUS.yml.
// Looks for lines with "wtb-*:" pattern under the skills_detail or skills section.
func StatusYmlRegisteredSkills(rootDir string) ([]string, error) {
	f, err := os.Open(filepath.Join(rootDir, "docs", "STATUS.yml"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Look for "wtb-*:" entries throughout the file
	re := regexp.MustCompile(`^\s+(wtb-[\w-]+):`)
	scanner := bufio.NewScanner(f)
	seen := make(map[string]bool)
	var skills []string

	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			name := m[1]
			if !seen[name] {
				seen[name] = true
				skills = append(skills, name)
			}
		}
	}
	return skills, scanner.Err()
}

// FindExampleDirs returns all directory names under examples/.
func FindExampleDirs(rootDir string) ([]string, error) {
	exDir := filepath.Join(rootDir, "examples")
	entries, err := os.ReadDir(exDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}

// FindMCPToolNames scans the Go MCP server source (pkg/mcp/tools_*.go)
// for s.AddTool() calls and returns the count. Tool names are extracted
// from the first argument pattern: toolXxxYyy() → xxx_yyy.
func FindMCPToolNames(rootDir string) ([]string, error) {
	pattern := filepath.Join(rootDir, "pkg", "mcp", "tools_*.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`s\.AddTool\(tool(\w+)\(\)`)
	var tools []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, m := range matches {
			tools = append(tools, m[1])
		}
	}
	return tools, nil
}

// StatusYmlFilePaths extracts all "file:" entries from STATUS.yml.
func StatusYmlFilePaths(rootDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, "docs", "STATUS.yml"))
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?m)^\s+(?:file|-):\s*(.+\.(?:md|yml|go))`)
	matches := re.FindAllStringSubmatch(string(data), -1)

	seen := make(map[string]bool)
	var paths []string
	for _, m := range matches {
		p := strings.TrimSpace(m[1])
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}

	// Also extract from "- path" entries in files: arrays
	re2 := regexp.MustCompile(`(?m)^\s+-\s+([\w/.-]+\.(?:md|yml|go\.example))`)
	matches2 := re2.FindAllStringSubmatch(string(data), -1)
	for _, m := range matches2 {
		p := strings.TrimSpace(m[1])
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// FindDocCommandRefs extracts .go filenames referenced in a doc's CLI commands section.
func FindDocCommandRefs(rootDir, docPath string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, docPath))
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?:├──|└──)\s+(\w+\.go)`)
	matches := re.FindAllStringSubmatch(string(data), -1)

	seen := make(map[string]bool)
	var refs []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			refs = append(refs, name)
		}
	}
	return refs, nil
}

// MermaidBlock represents a mermaid code block found in a markdown file.
type MermaidBlock struct {
	Content string
	Line    int
	File    string
}

// FindMermaidBlocks extracts mermaid code blocks from a markdown file.
func FindMermaidBlocks(filePath string) ([]MermaidBlock, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var blocks []MermaidBlock
	inBlock := false
	var current strings.Builder
	startLine := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```mermaid" {
			inBlock = true
			startLine = i + 1
			current.Reset()
			continue
		}
		if inBlock && trimmed == "```" {
			blocks = append(blocks, MermaidBlock{
				Content: current.String(),
				Line:    startLine,
				File:    filePath,
			})
			inBlock = false
			continue
		}
		if inBlock {
			current.WriteString(line)
			current.WriteString("\n")
		}
	}
	return blocks, nil
}

// FindGuideFiles returns all .md files in docs/guides/.
func FindGuideFiles(rootDir string) ([]string, error) {
	guidesDir := filepath.Join(rootDir, "docs", "guides")
	entries, err := os.ReadDir(guidesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var guides []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			guides = append(guides, e.Name())
		}
	}
	return guides, nil
}

// PatternDirs returns all pattern directory or file names under patterns/.
func PatternDirs(rootDir string) ([]string, error) {
	patternsDir := filepath.Join(rootDir, "patterns")
	entries, err := os.ReadDir(patternsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var patterns []string
	for _, e := range entries {
		name := e.Name()
		// Strip .md extension for matching
		name = strings.TrimSuffix(name, ".md")
		patterns = append(patterns, name)
	}
	return patterns, nil
}

// DocCountOf searches a documentation file for a pattern like "N <word>"
// (e.g., "5 providers") and returns the documented count.
// Returns -1 if pattern not found.
func DocCountOf(rootDir, docPath, word string) (int, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, docPath))
	if err != nil {
		return -1, err
	}

	re := regexp.MustCompile(`(\d+)\s+` + regexp.QuoteMeta(word))
	matches := re.FindStringSubmatch(string(data))
	if len(matches) < 2 {
		return -1, nil
	}
	count, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1, err
	}
	return count, nil
}

// FindMCPToolCount returns the number of tools registered in the Go MCP server
// by counting s.AddTool() calls in pkg/mcp/tools_*.go. This replaces the old
// TypeScript-specific description/category/annotation parsers which were removed
// when agent/mcp-server/ was deleted (MCP server is now pure Go in pkg/mcp/).
func FindMCPToolCount(rootDir string) (int, error) {
	tools, err := FindMCPToolNames(rootDir)
	if err != nil {
		return 0, err
	}
	return len(tools), nil
}

// guardrailContract is the schema for contract.yml files.
// Used by CheckClausulaPatrea and any future contract-based guardrails.
type guardrailContract struct {
	ID      string            `yaml:"id"`
	Version string            `yaml:"version"`
	Source  string            `yaml:"source"`  // path relative to rootDir
	Anchors []guardrailAnchor `yaml:"anchors"` // what must be present in source
}

type guardrailAnchor struct {
	ID          string `yaml:"id"`
	Contains    string `yaml:"contains"`    // substring that must exist in source
	Description string `yaml:"description"` // human-readable label for error messages
}

// CheckClausulaPatrea verifies the Cláusula Pétrea contract.
// Contract moved from skills/ to pkg/compliance/config/petrea_contract.yml (Phase 9.2f).
// Zero hardcoded strings in Go (P006, P008).
func CheckClausulaPatrea(rootDir string) error {
	contractPath := filepath.Join(rootDir, "pkg", "compliance", "config", "petrea_contract.yml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		return fmt.Errorf("CheckClausulaPatrea: contract not found at %s: %w", contractPath, err)
	}

	var contract guardrailContract
	if err := yaml.Unmarshal(data, &contract); err != nil {
		return fmt.Errorf("CheckClausulaPatrea: failed to parse contract: %w", err)
	}

	sourcePath := filepath.Join(rootDir, filepath.FromSlash(contract.Source))
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("CheckClausulaPatrea: source %q not found: %w", contract.Source, err)
	}

	for _, anchor := range contract.Anchors {
		if !strings.Contains(string(content), anchor.Contains) {
			return fmt.Errorf("CheckClausulaPatrea: missing %q (%s) in %s",
				anchor.Contains, anchor.Description, contract.Source)
		}
	}
	return nil
}

