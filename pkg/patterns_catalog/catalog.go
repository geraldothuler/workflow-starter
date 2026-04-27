// Package patterns_catalog provides an AI-powered architecture pattern catalog
// with hierarchy-based precedence. Data is loaded from embedded YAML defaults
// and can be overridden per-project via .workflow/patterns-catalog/*.yml.
//
// Hierarchy (highest priority wins): project > team > company > universal
package patterns_catalog

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TYPES
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// CatalogEntry represents a single pattern or anti-pattern in the catalog.
type CatalogEntry struct {
	ID              string         `yaml:"id"               json:"id"`
	Name            string         `yaml:"name"             json:"name"`
	Type            string         `yaml:"type"             json:"type"`            // "pattern" or "anti-pattern"
	Level           string         `yaml:"level"            json:"level"`           // "universal", "company", "team", "project"
	Category        string         `yaml:"category"         json:"category"`
	Source          EntrySource    `yaml:"source"           json:"source"`
	Description     string         `yaml:"description"      json:"description"`
	WhenToUse       []string       `yaml:"when_to_use"      json:"when_to_use,omitempty"`
	AntiPatterns    []string       `yaml:"anti_patterns"    json:"anti_patterns,omitempty"`
	Signs           []string       `yaml:"signs"            json:"signs,omitempty"`           // For anti-patterns
	Remediation     []string       `yaml:"remediation"      json:"remediation,omitempty"`     // For anti-patterns
	RelatedPatterns []string       `yaml:"related_patterns" json:"related_patterns,omitempty"`
	Keywords        []string       `yaml:"keywords"         json:"keywords,omitempty"`
}

// EntrySource tracks the origin of a catalog entry.
type EntrySource struct {
	Reference string `yaml:"reference" json:"reference"`
	Author    string `yaml:"author"    json:"author"`
}

// Category represents a grouping of patterns.
type Category struct {
	ID   string `yaml:"id"   json:"id"`
	Name string `yaml:"name" json:"name"`
}

// catalogFile represents the raw YAML structure.
type catalogFile struct {
	Categories []Category     `yaml:"categories"`
	Patterns   []CatalogEntry `yaml:"patterns"`
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PATTERN CATALOG
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// PatternCatalog holds all patterns and anti-patterns with hierarchy support.
type PatternCatalog struct {
	entries    map[string]CatalogEntry // ID → entry (merged, highest level wins)
	categories []Category
}

// CatalogOption configures a PatternCatalog during creation.
type CatalogOption func(*PatternCatalog)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EMBEDDED DEFAULTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

//go:embed config/*.yml
var defaultCatalogConfigs embed.FS

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HIERARCHY
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// levelPriority returns the numeric priority of a hierarchy level.
// Higher number = higher priority.
var levelPriority = map[string]int{
	"universal": 0,
	"company":   1,
	"team":      2,
	"project":   3,
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CONSTRUCTOR
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// NewPatternCatalog creates a catalog loaded with embedded defaults.
// Apply options for project-level overrides.
func NewPatternCatalog(opts ...CatalogOption) (*PatternCatalog, error) {
	pc := &PatternCatalog{
		entries: make(map[string]CatalogEntry),
	}

	// 1. Load embedded defaults
	if err := pc.loadEmbedded(); err != nil {
		return nil, fmt.Errorf("loading embedded catalog: %w", err)
	}

	// 2. Apply options (project overrides)
	for _, opt := range opts {
		opt(pc)
	}

	return pc, nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// OPTIONS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// WithProjectOverrides loads overrides from .workflow/patterns-catalog/*.yml.
// Override behavior:
//   - Same ID: higher level wins for `type` field; other fields fully replaced
//   - New ID: appended to catalog
//   - New categories: appended to categories list
func WithProjectOverrides(projectDir string) CatalogOption {
	return func(pc *PatternCatalog) {
		overrideDir := filepath.Join(projectDir, ".workflow", "patterns-catalog")
		if _, err := os.Stat(overrideDir); os.IsNotExist(err) {
			return // No overrides directory
		}
		pc.loadOverrides(overrideDir)
	}
}

// WithEntries adds entries directly (useful for testing).
func WithEntries(entries ...CatalogEntry) CatalogOption {
	return func(pc *PatternCatalog) {
		for _, entry := range entries {
			pc.mergeEntry(entry)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ACCESSORS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// GetByID returns a pattern entry by ID, or nil if not found.
func (pc *PatternCatalog) GetByID(id string) *CatalogEntry {
	entry, ok := pc.entries[id]
	if !ok {
		return nil
	}
	return &entry
}

// PatternCount returns the total number of entries (patterns + anti-patterns).
func (pc *PatternCatalog) PatternCount() int {
	return len(pc.entries)
}

// Patterns returns all entries with type "pattern", sorted by ID.
func (pc *PatternCatalog) Patterns() []CatalogEntry {
	return pc.filterByType("pattern")
}

// AntiPatterns returns all entries with type "anti-pattern", sorted by ID.
func (pc *PatternCatalog) AntiPatterns() []CatalogEntry {
	return pc.filterByType("anti-pattern")
}

// AllEntries returns all entries sorted by ID.
func (pc *PatternCatalog) AllEntries() []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(pc.entries))
	for _, e := range pc.entries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
	return entries
}

// Categories returns all categories.
func (pc *PatternCatalog) Categories() []Category {
	return pc.categories
}

// EffectiveType returns the type of an entry considering hierarchy.
// If a higher-level override exists, its type takes precedence.
func (pc *PatternCatalog) EffectiveType(id string) string {
	entry, ok := pc.entries[id]
	if !ok {
		return ""
	}
	return entry.Type
}

// EntriesByCategory returns entries filtered by category ID.
func (pc *PatternCatalog) EntriesByCategory(categoryID string) []CatalogEntry {
	var result []CatalogEntry
	for _, e := range pc.entries {
		if e.Category == categoryID {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// EntriesByLevel returns entries filtered by hierarchy level.
func (pc *PatternCatalog) EntriesByLevel(level string) []CatalogEntry {
	var result []CatalogEntry
	for _, e := range pc.entries {
		if e.Level == level {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FORMAT FOR LLM PROMPT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// FormatForPrompt generates a compact representation of the catalog
// suitable for inclusion in an LLM prompt. Targets ~40 tokens per entry.
func (pc *PatternCatalog) FormatForPrompt() string {
	var sb strings.Builder

	sb.WriteString("# Architecture Pattern Catalog\n\n")

	// Group by category
	categoryEntries := make(map[string][]CatalogEntry)
	for _, e := range pc.entries {
		categoryEntries[e.Category] = append(categoryEntries[e.Category], e)
	}

	// Sort entries within each category
	for cat := range categoryEntries {
		sort.Slice(categoryEntries[cat], func(i, j int) bool {
			return categoryEntries[cat][i].ID < categoryEntries[cat][j].ID
		})
	}

	// Output by category order
	for _, cat := range pc.categories {
		entries, ok := categoryEntries[cat.ID]
		if !ok || len(entries) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n", cat.Name))

		for _, e := range entries {
			typeLabel := "P"
			if e.Type == "anti-pattern" {
				typeLabel = "AP"
			}

			sb.WriteString(fmt.Sprintf("- **%s** [%s|%s|%s]: %s",
				e.ID, typeLabel, e.Level, e.Source.Reference, e.Description))

			if e.Type == "anti-pattern" && len(e.Signs) > 0 {
				sb.WriteString(fmt.Sprintf(" Signs: %s.", strings.Join(e.Signs, "; ")))
			}
			if len(e.Remediation) > 0 {
				sb.WriteString(fmt.Sprintf(" Fix: %s.", strings.Join(e.Remediation, ", ")))
			}
			if e.Type == "pattern" && len(e.WhenToUse) > 0 {
				// Only first when_to_use for compactness
				sb.WriteString(fmt.Sprintf(" When: %s.", e.WhenToUse[0]))
			}
			if len(e.Keywords) > 0 {
				sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(e.Keywords, ", ")))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// LOADING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (pc *PatternCatalog) loadEmbedded() error {
	entries, err := defaultCatalogConfigs.ReadDir("config")
	if err != nil {
		return fmt.Errorf("failed to read embedded catalog configs: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := defaultCatalogConfigs.ReadFile("config/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read embedded config %s: %w", entry.Name(), err)
		}
		if err := pc.parseCatalogFile(data, entry.Name()); err != nil {
			return fmt.Errorf("failed to parse embedded config %s: %w", entry.Name(), err)
		}
	}

	return nil
}

func (pc *PatternCatalog) loadOverrides(dir string) {
	overrideFiles, err := os.ReadDir(dir)
	if err != nil {
		return // silently skip on read error
	}

	for _, entry := range overrideFiles {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // silently skip on read error
		}
		// Parse and merge (override entries by ID)
		_ = pc.parseCatalogFile(data, entry.Name())
	}
}

func (pc *PatternCatalog) parseCatalogFile(data []byte, filename string) error {
	var cf catalogFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return fmt.Errorf("YAML parse error in %s: %w", filename, err)
	}

	// Merge categories (append new ones)
	existingCats := make(map[string]bool)
	for _, c := range pc.categories {
		existingCats[c.ID] = true
	}
	for _, c := range cf.Categories {
		if !existingCats[c.ID] {
			pc.categories = append(pc.categories, c)
			existingCats[c.ID] = true
		}
	}

	// Merge entries
	for _, entry := range cf.Patterns {
		if entry.ID == "" {
			continue // skip entries without ID
		}
		pc.mergeEntry(entry)
	}

	return nil
}

// mergeEntry adds or overrides an entry. Higher hierarchy level wins.
func (pc *PatternCatalog) mergeEntry(entry CatalogEntry) {
	existing, exists := pc.entries[entry.ID]
	if !exists {
		// New entry: just add
		pc.entries[entry.ID] = entry
		return
	}

	// Entry exists: higher level wins
	existingPriority := levelPriority[existing.Level]
	newPriority := levelPriority[entry.Level]

	if newPriority >= existingPriority {
		// Override: new entry takes precedence
		pc.entries[entry.ID] = entry
	}
	// Otherwise: keep existing (it has higher priority)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (pc *PatternCatalog) filterByType(entryType string) []CatalogEntry {
	var result []CatalogEntry
	for _, e := range pc.entries {
		if e.Type == entryType {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yml" || ext == ".yaml"
}
