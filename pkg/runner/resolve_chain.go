package runner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChainResolver walks the artifact chain to find upstream values.
type ChainResolver struct{}

func (r *ChainResolver) Name() string { return "chain" }

func (r *ChainResolver) Resolve(spec InputSpec, inputs RunInputs, ctx ResolveContext) (string, bool) {
	if ctx.Definition == nil || ctx.RepoPath == "" {
		return "", false
	}

	// Get search fields for this input from config
	var searchFields []string
	if ctx.Config != nil {
		if s, ok := ctx.Config.Resolve.Strategies["chain"]; ok {
			searchFields = s.SearchFields[spec.Name]
		}
	}
	if len(searchFields) == 0 {
		searchFields = []string{spec.Name}
	}

	artifactDir := "docs/workflow/"
	if ctx.Config != nil {
		if s, ok := ctx.Config.Resolve.Strategies["chain"]; ok && s.ArtifactDir != "" {
			artifactDir = s.ArtifactDir
		}
	}

	// Walk upstream chain types
	for _, fromType := range ctx.Definition.Chain.From {
		typeDir := filepath.Join(ctx.RepoPath, artifactDir, fromType)
		val := scanArtifactsForField(typeDir, searchFields)
		if val != "" {
			return val, true
		}
	}
	return "", false
}

// scanArtifactsForField scans the most recent artifact in a type directory
// for YAML frontmatter fields. Searches both top-level .md files and .md
// files inside NNN-* subdirectories (convention: docs/workflow/<type>/NNN-<ctx>/).
func scanArtifactsForField(typeDir string, fields []string) string {
	entries, err := os.ReadDir(typeDir)
	if err != nil {
		return ""
	}

	// Sort entries by name descending to get most recent (NNN-based naming)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			// Descend into NNN-* subdirectories and scan their .md files
			if val := scanDirForField(filepath.Join(typeDir, entry.Name()), fields); val != "" {
				return val
			}
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		if val := scanFileForField(filepath.Join(typeDir, entry.Name()), fields); val != "" {
			return val
		}
	}
	return ""
}

// scanDirForField scans .md files inside a single directory for frontmatter fields.
func scanDirForField(dir string, fields []string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	// Sort descending (most recent savepoint first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if val := scanFileForField(filepath.Join(dir, entry.Name()), fields); val != "" {
			return val
		}
	}
	return ""
}

// scanFileForField reads a single .md file and returns the first matching
// frontmatter field value.
func scanFileForField(path string, fields []string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	fm := extractFrontmatter(string(data))
	if fm == nil {
		return ""
	}

	for _, field := range fields {
		if val, ok := fm[field]; ok && val != "" {
			return val
		}
	}
	return ""
}

// extractFrontmatter parses YAML frontmatter from --- delimited blocks.
func extractFrontmatter(content string) map[string]string {
	if !strings.HasPrefix(content, "---") {
		return nil
	}

	endIdx := strings.Index(content[3:], "---")
	if endIdx < 0 {
		return nil
	}

	fmContent := content[3 : endIdx+3]
	var fm map[string]string
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return nil
	}
	return fm
}
