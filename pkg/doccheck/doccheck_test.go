package doccheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/features"
)

// platformContent returns CLAUDE.md + REFERENCE.md combined.
// Technical details live in REFERENCE.md since the 3-scope split (CLAUDE.md = contracts,
// REFERENCE.md = tech reference). Tests that enforce "documented somewhere in platform docs"
// should use this instead of reading CLAUDE.md alone.
func platformContent(t *testing.T, root string) string {
	t.Helper()
	var sb strings.Builder
	for _, rel := range []string{"CLAUDE.md", "docs/workflow/platform/REFERENCE.md"} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err == nil {
			sb.Write(data)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// rootDir finds the project root by walking up from the test file location.
func rootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Walk up to find go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// --- Go Version ---

func TestGoVersion_CLAUDEmd(t *testing.T) {
	root := rootDir(t)
	version, err := GoVersionFromMod(root)
	if err != nil {
		t.Fatal(err)
	}

	// CLAUDE.md or REFERENCE.md must reference the correct Go version.
	// Technical details live in REFERENCE.md since the 3-scope split.
	content := platformContent(t, root)
	if !strings.Contains(content, "Go "+version) {
		t.Errorf("platform docs do not reference Go %s (from go.mod) — add to CLAUDE.md or REFERENCE.md", version)
	}
}

func TestGoVersion_README(t *testing.T) {
	root := rootDir(t)
	version, err := GoVersionFromMod(root)
	if err != nil {
		t.Fatal(err)
	}

	found, err := DocContains(root, "README.md", "Go "+version)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Errorf("README.md does not reference Go %s (from go.mod)", version)
	}
}

// --- Package Coverage ---

func TestAllPackages_DocumentedInCLAUDEmd(t *testing.T) {
	root := rootDir(t)
	packages, err := FindGoPackages(root)
	if err != nil {
		t.Fatal(err)
	}

	// Since the 3-scope split, package inventory lives in REFERENCE.md.
	// Accept mention in either CLAUDE.md or REFERENCE.md.
	content := platformContent(t, root)

	var missing []string
	for _, pkg := range packages {
		if !strings.Contains(content, pkg+"/") && !strings.Contains(content, pkg+"→") && !strings.Contains(content, "├── "+pkg) && !strings.Contains(content, "└── "+pkg) {
			missing = append(missing, pkg)
		}
	}
	if len(missing) > 0 {
		t.Errorf("platform docs (CLAUDE.md + REFERENCE.md) missing documentation for packages: %v", missing)
	}
}

// TestAllPackages_DocumentedInREADME removed: README.md is now a narrative gateway.
// Package inventory is validated by TestAllPackages_DocumentedInCLAUDEmd (above).

// --- Command Coverage ---

// TestAllCommands_DocumentedInCLAUDEmd verifies that CLAUDE.md references cmd/wtb/
// as the CLI entry point. Individual file inventory belongs in README.md, not CLAUDE.md
// (CLAUDE.md is an architectural overview, not a file listing).
func TestAllCommands_DocumentedInCLAUDEmd(t *testing.T) {
	root := rootDir(t)
	content := platformContent(t, root)
	if !strings.Contains(content, "cmd/wtb/") {
		t.Error("platform docs (CLAUDE.md or REFERENCE.md) must reference cmd/wtb/ as the CLI entry point")
	}
}

// TestAllCommands_DocumentedInREADME was redirected to CLAUDE.md.
// README.md is a narrative gateway — CLI subcommand inventory lives in CLAUDE.md (sec 10).
func TestAllCommands_DocumentedInCLAUDEmd_Subcommands(t *testing.T) {
	root := rootDir(t)
	commands, err := FindCommandFiles(root)
	if err != nil {
		t.Fatal(err)
	}

	// Since the 3-scope split, subcommand inventory lives in REFERENCE.md.
	content := platformContent(t, root)

	// main.go is the entry point, not a subcommand — skip.
	skip := map[string]bool{"main.go": true}

	var missing []string
	for _, cmd := range commands {
		if skip[cmd] {
			continue
		}
		subName := strings.TrimSuffix(cmd, ".go")
		if !strings.Contains(content, subName) {
			missing = append(missing, cmd)
		}
	}
	if len(missing) > 0 {
		t.Errorf("platform docs (CLAUDE.md + REFERENCE.md) missing documentation for subcommands: %v", missing)
	}
}

// --- No Ghost References ---

func TestCLAUDEmd_NoGhostCommands(t *testing.T) {
	root := rootDir(t)
	commands, err := FindCommandFiles(root)
	if err != nil {
		t.Fatal(err)
	}

	// Build set of actual command files
	actual := make(map[string]bool)
	for _, cmd := range commands {
		actual[cmd] = true
	}
	actual["root.go"] = true // root.go is legitimate

	claudeData, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(claudeData)

	// Check for common ghost commands that were documented but don't exist
	ghosts := []string{"reverse.go", "diff.go", "merge.go"}
	for _, ghost := range ghosts {
		if strings.Contains(content, ghost) && !actual[ghost] {
			t.Errorf("CLAUDE.md references non-existent command file: %s", ghost)
		}
	}
}

// --- Skill Coverage ---

func TestAllSkills_DocumentedInCLAUDEmd(t *testing.T) {
	root := rootDir(t)
	skills, err := SkillDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) == 0 {
		t.Skip("no skills directory found")
	}

	claudeData, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(claudeData)

	var missing []string
	for _, skill := range skills {
		if !strings.Contains(content, skill) {
			missing = append(missing, skill)
		}
	}
	if len(missing) > 0 {
		t.Errorf("CLAUDE.md is missing documentation for skills: %v", missing)
	}
}

// --- Package Count Sanity ---

func TestPackageCount_ReasonablyDocumented(t *testing.T) {
	root := rootDir(t)
	packages, err := FindGoPackages(root)
	if err != nil {
		t.Fatal(err)
	}

	claudeData, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(claudeData)

	// Check the documented count is reasonable
	actualCount := len(packages)
	countStr := fmt.Sprintf("%d pacotes", actualCount)
	if !strings.Contains(content, countStr) {
		t.Logf("CLAUDE.md package count may be outdated (actual: %d packages)", actualCount)
		// Not a hard failure, just informational
	}
}

// ==========================================================================
// DRIFT PREVENTION TESTS (added after README audit)
// Prevents: broken links, wrong counts, missing registrations, stale paths
// ==========================================================================

// --- Broken Links ---

func TestREADME_NoBrokenLinks(t *testing.T) {
	root := rootDir(t)
	broken, err := FindBrokenLinks(root, "README.md")
	if err != nil {
		t.Fatal(err)
	}

	if len(broken) > 0 {
		for _, link := range broken {
			t.Errorf("README.md line %d: broken link [%s](%s) — file does not exist",
				link.Line, link.Text, link.Path)
		}
	}
}

func TestCLAUDEmd_NoBrokenLinks(t *testing.T) {
	root := rootDir(t)
	broken, err := FindBrokenLinks(root, "CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}

	if len(broken) > 0 {
		for _, link := range broken {
			t.Errorf("CLAUDE.md line %d: broken link [%s](%s) — file does not exist",
				link.Line, link.Text, link.Path)
		}
	}
}

// --- Provider Count ---

// TestProviderCount_README removed: README.md is now a narrative gateway.
// Provider count is validated by TestProviderCount_CLAUDEmd (below).

func TestProviderCount_CLAUDEmd(t *testing.T) {
	root := rootDir(t)

	providers, err := FindLLMProviders(root)
	if err != nil {
		t.Fatal(err)
	}

	// Since the 3-scope split, provider inventory lives in REFERENCE.md.
	content := platformContent(t, root)

	var missing []string
	providerNames := map[string]string{
		"claude":  "Claude",
		"chatgpt": "ChatGPT",
		"gemini":  "Gemini",
		"ollama":  "Ollama",
		"azure":   "Azure",
	}

	for _, p := range providers {
		displayName, ok := providerNames[p]
		if !ok {
			displayName = p
		}
		if !strings.Contains(content, displayName) {
			missing = append(missing, displayName)
		}
	}
	if len(missing) > 0 {
		t.Errorf("platform docs (CLAUDE.md + REFERENCE.md) missing LLM provider references: %v", missing)
	}
}

// --- STATUS.yml Skills Integrity ---

func TestStatusYml_SkillsCountMatchesFilesystem(t *testing.T) {
	root := rootDir(t)

	// Count skills on filesystem
	skills, err := SkillDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	actualCount := len(skills)

	// Read count from STATUS.yml
	statusCount, err := StatusYmlSkillsCount(root)
	if err != nil {
		t.Fatal(err)
	}

	if statusCount != actualCount {
		t.Errorf("STATUS.yml says skills: %d but filesystem has %d skill directories: %v",
			statusCount, actualCount, skills)
	}
}

func TestAllSkills_RegisteredInStatusYml(t *testing.T) {
	root := rootDir(t)

	// Skills on disk
	skills, err := SkillDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) == 0 {
		t.Skip("no skills directory found")
	}

	// Skills registered in STATUS.yml
	registered, err := StatusYmlRegisteredSkills(root)
	if err != nil {
		t.Fatal(err)
	}
	registeredSet := make(map[string]bool)
	for _, s := range registered {
		registeredSet[s] = true
	}

	var missing []string
	for _, skill := range skills {
		if !registeredSet[skill] {
			missing = append(missing, skill)
		}
	}
	if len(missing) > 0 {
		t.Errorf("STATUS.yml is missing registration for skills: %v (registered: %v)", missing, registered)
	}
}

// --- Skills in README ---

func TestAllSkills_DocumentedInREADME(t *testing.T) {
	root := rootDir(t)
	skills, err := SkillDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) == 0 {
		t.Skip("no skills directory found")
	}

	readmeData, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(readmeData)

	var missing []string
	for _, skill := range skills {
		if !strings.Contains(content, skill) {
			missing = append(missing, skill)
		}
	}
	if len(missing) > 0 {
		t.Errorf("README.md is missing documentation for skills: %v", missing)
	}
}

// --- Example Directory Paths ---

func TestREADME_ExamplePathsExist(t *testing.T) {
	root := rootDir(t)

	// Find all example dirs on filesystem
	exampleDirs, err := FindExampleDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(exampleDirs) == 0 {
		t.Skip("no examples directory found")
	}

	existsSet := make(map[string]bool)
	for _, d := range exampleDirs {
		existsSet[d] = true
	}

	// Read README and find all references to examples/
	readmeData, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	// Match patterns like "examples/something/" or "examples/something"
	re := regexp.MustCompile(`examples/([\w-]+)`)
	matches := re.FindAllStringSubmatch(string(readmeData), -1)

	seen := make(map[string]bool)
	var broken []string
	for _, m := range matches {
		dirName := m[1]
		if seen[dirName] {
			continue
		}
		seen[dirName] = true

		if !existsSet[dirName] {
			broken = append(broken, "examples/"+dirName)
		}
	}
	if len(broken) > 0 {
		t.Errorf("README.md references non-existent example directories: %v (available: %v)",
			broken, exampleDirs)
	}
}

// --- No Ghost Skills ---

func TestREADME_NoGhostSkills(t *testing.T) {
	root := rootDir(t)
	skills, err := SkillDirs(root)
	if err != nil {
		t.Fatal(err)
	}

	actualSet := make(map[string]bool)
	for _, s := range skills {
		actualSet[s] = true
	}

	readmeData, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	// Known wtb-* names that are NOT skills (e.g., CLI binary, packages, override markers)
	notSkills := map[string]bool{
		"wtb-agent":   true, // CLI wrapper binary name
		"wtb-noguard": true, // guardrail override marker (<!-- wtb-noguard: ... -->)
	}

	// Find all "wtb-*" references in README
	re := regexp.MustCompile(`wtb-([\w-]+)`)
	matches := re.FindAllString(string(readmeData), -1)

	seen := make(map[string]bool)
	var ghosts []string
	for _, m := range matches {
		if seen[m] || notSkills[m] {
			continue
		}
		seen[m] = true
		if !actualSet[m] {
			ghosts = append(ghosts, m)
		}
	}
	if len(ghosts) > 0 {
		t.Errorf("README.md references non-existent skills: %v", ghosts)
	}
}

// ==========================================================================
// CROSS-CHANNEL PARITY GUARDRAILS (v3.3)
// Prevents drift between CLI commands and MCP/Wrapper tool counts.
// ==========================================================================

func TestMCP_ToolCount(t *testing.T) {
	root := rootDir(t)
	count, err := FindMCPToolCount(root)
	if err != nil {
		t.Fatalf("failed to count MCP tools: %v", err)
	}

	if count < 7 {
		t.Errorf("MCP Server has %d tools (expected >= 7)", count)
	}
}

func TestWrapper_CommandCount(t *testing.T) {
	root := rootDir(t)
	wrapperPath := filepath.Join(root, "agent", "wrapper", "wtb-agent.sh")

	data, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("failed to read wrapper script: %v", err)
	}
	content := string(data)

	// Count case entries in the main router (between "case "$cmd" in" and "esac")
	caseRe := regexp.MustCompile(`^\s+\S+\)\s+cmd_\w+`)
	lines := strings.Split(content, "\n")

	commandCount := 0
	for _, line := range lines {
		if caseRe.MatchString(line) {
			commandCount++
		}
	}

	if commandCount < 11 {
		t.Errorf("Wrapper has %d commands (expected >= 11)", commandCount)
	}
}

func TestLens_ProjectTitleNotHardcoded(t *testing.T) {
	root := rootDir(t)
	converterPath := filepath.Join(root, "pkg", "render", "converter.go")

	data, err := os.ReadFile(converterPath)
	if err != nil {
		t.Fatalf("failed to read converter.go: %v", err)
	}
	content := string(data)

	// Verify that convertMeta references backlog.Meta.ProjectTitle
	if !strings.Contains(content, "backlog.Meta.ProjectTitle") {
		t.Error("converter.go should reference backlog.Meta.ProjectTitle (not hardcoded title)")
	}

	// Verify the hardcoded "Backlog Técnico" is only used as fallback, not as the primary title
	titleAssignments := regexp.MustCompile(`Title:\s*"Backlog Técnico"`)
	matches := titleAssignments.FindAllString(content, -1)
	if len(matches) > 0 {
		t.Error("converter.go still has hardcoded 'Backlog Técnico' as Title (should use ProjectTitle with fallback)")
	}
}

// ==========================================================================
// DOC VIVA GUARDRAILS (v3.5)
// Validates the living documentation chain:
// README → guides → cross-refs → breadcrumbs → mermaid → STATUS.yml
// ==========================================================================

// --- 4.1: MCP Tool Count Exact Match ---

func TestMCP_ToolCountExact(t *testing.T) {
	root := rootDir(t)
	tools, err := FindMCPToolNames(root)
	if err != nil {
		t.Fatal(err)
	}

	expected := 25 // Go MCP server (pkg/mcp/) has 25 tools (added ops_airbyte_schedule_map)
	if len(tools) != expected {
		t.Errorf("MCP Server has %d tools (expected exactly %d). Tools: %v", len(tools), expected, tools)
	}
}

// --- 4.2: STATUS.yml File Paths Exist ---

func TestStatusYml_FilePathsExist(t *testing.T) {
	root := rootDir(t)
	paths, err := StatusYmlFilePaths(root)
	if err != nil {
		t.Fatal(err)
	}

	var missing []string
	for _, p := range paths {
		fullPath := filepath.Join(root, p)
		// Handle glob patterns (e.g., "docs/research/implementation-examples/*.go.example")
		if strings.Contains(p, "*") {
			matches, _ := filepath.Glob(fullPath)
			if len(matches) == 0 {
				missing = append(missing, p)
			}
			continue
		}
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		t.Errorf("STATUS.yml references non-existent files: %v", missing)
	}
}

// --- 4.3: Dynamic Ghost Command Check ---

func TestCLAUDEmd_NoDynamicGhostCommands(t *testing.T) {
	root := rootDir(t)

	// Get actual command files
	actualCmds, err := FindCommandFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	actualSet := make(map[string]bool)
	for _, cmd := range actualCmds {
		actualSet[cmd] = true
	}
	actualSet["root.go"] = true
	actualSet["main.go"] = true

	// Get command refs from CLAUDE.md
	refs, err := FindDocCommandRefs(root, "CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}

	var ghosts []string
	for _, ref := range refs {
		if !actualSet[ref] {
			ghosts = append(ghosts, ref)
		}
	}
	if len(ghosts) > 0 {
		t.Errorf("CLAUDE.md references non-existent command files: %v", ghosts)
	}
}

// --- 4.4: Pattern File Existence ---

func TestStatusYml_PatternFilesExist(t *testing.T) {
	root := rootDir(t)
	patternsDir := filepath.Join(root, "patterns")

	if _, err := os.Stat(patternsDir); os.IsNotExist(err) {
		t.Skip("no patterns directory")
	}

	// Read STATUS.yml for pattern references
	data, err := os.ReadFile(filepath.Join(root, "docs", "STATUS.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Find pattern names from "patterns:" section
	// Only match first-level entries (exactly 2 spaces) — skip sub-keys like used_by:, when_to_load:
	re := regexp.MustCompile(`^  ([\w-]+):\s*$`)
	inPatterns := false
	var patternNames []string
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "patterns:" {
			inPatterns = true
			continue
		}
		// Stop at next top-level section (no leading spaces, not blank, not comment)
		if inPatterns && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
			break
		}
		if inPatterns {
			if m := re.FindStringSubmatch(line); len(m) > 1 {
				patternNames = append(patternNames, m[1])
			}
		}
	}

	// Verify each pattern has a file
	for _, name := range patternNames {
		patternFile := filepath.Join(patternsDir, name+".md")
		if _, err := os.Stat(patternFile); os.IsNotExist(err) {
			t.Errorf("STATUS.yml pattern %q has no file at patterns/%s.md", name, name)
		}
	}
}

// --- 4.5: STATUS.yml Stats Accurate ---

func TestStatusYml_StatsAccurate(t *testing.T) {
	root := rootDir(t)

	// Skills count
	skills, err := SkillDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	statusSkills, err := StatusYmlSkillsCount(root)
	if err != nil {
		t.Fatal(err)
	}
	if statusSkills != len(skills) {
		t.Errorf("STATUS.yml stats.skills=%d but filesystem has %d", statusSkills, len(skills))
	}

	// Patterns count
	patterns, err := PatternDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "STATUS.yml"))
	if err != nil {
		t.Fatal(err)
	}
	patternsRe := regexp.MustCompile(`(?m)^\s+patterns:\s+(\d+)`)
	if m := patternsRe.FindStringSubmatch(string(data)); len(m) > 1 {
		count, _ := strconv.Atoi(m[1])
		if count != len(patterns) {
			t.Errorf("STATUS.yml stats.patterns=%d but filesystem has %d", count, len(patterns))
		}
	}
}

// --- 4.6: Mermaid Syntax Validation ---

func TestGuides_MermaidBlocksValid(t *testing.T) {
	root := rootDir(t)
	validKeywords := []string{"flowchart", "graph", "sequenceDiagram", "stateDiagram", "pie", "classDiagram", "erDiagram", "gantt", "gitgraph"}

	// Check guides and README
	files := []string{filepath.Join(root, "README.md")}
	guideDir := filepath.Join(root, "docs", "guides")
	entries, err := os.ReadDir(guideDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			files = append(files, filepath.Join(guideDir, e.Name()))
		}
	}

	for _, file := range files {
		blocks, err := FindMermaidBlocks(file)
		if err != nil {
			t.Errorf("error reading %s: %v", file, err)
			continue
		}

		relPath, _ := filepath.Rel(root, file)
		for _, block := range blocks {
			content := strings.TrimSpace(block.Content)
			if content == "" {
				t.Errorf("%s line %d: empty mermaid block", relPath, block.Line)
				continue
			}

			// Check first word is a valid keyword
			firstWord := strings.Fields(content)[0]
			valid := false
			for _, kw := range validKeywords {
				if firstWord == kw {
					valid = true
					break
				}
			}
			// Also check for stateDiagram-v2
			if strings.HasPrefix(firstWord, "stateDiagram") {
				valid = true
			}
			if !valid {
				t.Errorf("%s line %d: mermaid block starts with %q (expected one of: %v)",
					relPath, block.Line, firstWord, validKeywords)
			}
		}
	}
}

// --- 4.7: Guide Files Exist ---

func TestGuides_ExistOnDisk(t *testing.T) {
	root := rootDir(t)
	guides, err := FindGuideFiles(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(guides) < 5 {
		t.Errorf("expected at least 5 guide files, found %d: %v", len(guides), guides)
	}

	// Verify each guide has content
	for _, g := range guides {
		path := filepath.Join(root, "docs", "guides", g)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("guide %s: %v", g, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("guide %s is empty", g)
		}
	}
}

// --- 4.8: README-Guide Link Integrity ---

func TestREADME_AllGuideLinksExist(t *testing.T) {
	root := rootDir(t)
	links, err := FindMarkdownLinks(root, "README.md")
	if err != nil {
		t.Fatal(err)
	}

	var brokenGuides []string
	for _, link := range links {
		if strings.Contains(link.Path, "docs/guides/") {
			fullPath := filepath.Join(root, link.Path)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				brokenGuides = append(brokenGuides, link.Path)
			}
		}
	}
	if len(brokenGuides) > 0 {
		t.Errorf("README.md has broken guide links: %v", brokenGuides)
	}
}

// --- 4.9: Guide Cross-References ---

func TestGuides_NoBrokenCrossReferences(t *testing.T) {
	root := rootDir(t)
	guidesDir := filepath.Join(root, "docs", "guides")
	entries, err := os.ReadDir(guidesDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		relDocPath := filepath.Join("docs", "guides", e.Name())
		broken, err := FindBrokenLinks(root, relDocPath)
		if err != nil {
			t.Errorf("error checking %s: %v", e.Name(), err)
			continue
		}

		for _, link := range broken {
			t.Errorf("%s line %d: broken link [%s](%s)",
				e.Name(), link.Line, link.Text, link.Path)
		}
	}
}

// --- 4.10: Breadcrumb Navigation ---

func TestGuides_HaveBreadcrumb(t *testing.T) {
	root := rootDir(t)
	guidesDir := filepath.Join(root, "docs", "guides")
	entries, err := os.ReadDir(guidesDir)
	if err != nil {
		t.Fatal(err)
	}

	breadcrumbRe := regexp.MustCompile(`>\s*📍\s*\[README\]\(../../README\.md\)`)

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(guidesDir, e.Name()))
		if err != nil {
			t.Errorf("error reading %s: %v", e.Name(), err)
			continue
		}

		// Check first 5 lines for breadcrumb
		lines := strings.Split(string(data), "\n")
		found := false
		limit := 5
		if len(lines) < limit {
			limit = len(lines)
		}
		for i := 0; i < limit; i++ {
			if breadcrumbRe.MatchString(lines[i]) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("%s: missing breadcrumb navigation (expected '> 📍 [README](../../README.md) > ...' in first 5 lines)", e.Name())
		}
	}
}

// ==========================================================================
// SECTION 5: Feature ↔ Pattern Traceability Guardrails
// ==========================================================================

// --- 5.1: Architecture Patterns Reference Valid Files ---

func TestFeaturesYml_ArchPatternsExist(t *testing.T) {
	root := rootDir(t)
	patternsDir := filepath.Join(root, "patterns")

	registry, err := features.LoadRegistry(root)
	if err != nil {
		t.Fatalf("failed to load FEATURES.yml: %v", err)
	}

	for name, feature := range registry.Features {
		for _, ref := range feature.Architecture.Patterns {
			patternFile := filepath.Join(patternsDir, ref.ID+".md")
			if _, err := os.Stat(patternFile); os.IsNotExist(err) {
				t.Errorf("feature %q references architecture pattern %q but patterns/%s.md does not exist",
					name, ref.ID, ref.ID)
			}
			if ref.Why == "" {
				t.Errorf("feature %q architecture pattern %q has empty 'why' — justification required",
					name, ref.ID)
			}
		}
	}
}

// --- 5.2: Claude Patterns Reference Valid Files ---

func TestFeaturesYml_ClaudePatternsExist(t *testing.T) {
	root := rootDir(t)
	patternsDir := filepath.Join(root, "patterns")

	registry, err := features.LoadRegistry(root)
	if err != nil {
		t.Fatalf("failed to load FEATURES.yml: %v", err)
	}

	for name, feature := range registry.Features {
		for _, patternID := range feature.Claude.Patterns {
			patternFile := filepath.Join(patternsDir, patternID+".md")
			if _, err := os.Stat(patternFile); os.IsNotExist(err) {
				t.Errorf("feature %q references claude pattern %q but patterns/%s.md does not exist",
					name, patternID, patternID)
			}
		}
	}
}

// --- 5.3: No Orphan Patterns (every pattern referenced by at least 1 feature) ---

func TestFeaturesYml_NoOrphanPatterns(t *testing.T) {
	root := rootDir(t)

	// Get all pattern files on disk
	patternFiles, err := PatternDirs(root)
	if err != nil {
		t.Fatal(err)
	}

	// Get all patterns referenced in FEATURES.yml
	registry, err := features.LoadRegistry(root)
	if err != nil {
		t.Fatalf("failed to load FEATURES.yml: %v", err)
	}

	referenced := make(map[string]bool)
	for _, feature := range registry.Features {
		for _, ref := range feature.Architecture.Patterns {
			referenced[ref.ID] = true
		}
		for _, p := range feature.Claude.Patterns {
			referenced[p] = true
		}
	}

	for _, patternName := range patternFiles {
		if !referenced[patternName] {
			t.Errorf("pattern %q exists on disk but is not referenced by any feature in FEATURES.yml (orphan)",
				patternName)
		}
	}
}

// --- 5.4: Every Feature Has Architecture Patterns ---

func TestFeaturesYml_AllFeaturesHaveArchPatterns(t *testing.T) {
	root := rootDir(t)

	registry, err := features.LoadRegistry(root)
	if err != nil {
		t.Fatalf("failed to load FEATURES.yml: %v", err)
	}

	for name, feature := range registry.Features {
		if len(feature.Architecture.Patterns) == 0 {
			t.Errorf("feature %q has no architecture.patterns — every feature should document its architectural composition",
				name)
		}
	}
}

// ── Cláusula Pétrea guardrail ──────────────────────────────────────────────

// TestClausulaPatrea_InMetacognition verifies that _core/metacognition.md
// contains all mandatory elements of the Options Scoring Cláusula Pétrea.
// Removing any of these elements must break CI — this is the enforcement.
func TestClausulaPatrea_InMetacognition(t *testing.T) {
	root := rootDir(t)
	if err := CheckClausulaPatrea(root); err != nil {
		t.Errorf("Cláusula Pétrea guardrail failed: %v", err)
	}
}
