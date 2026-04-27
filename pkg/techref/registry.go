// Package techref — TechRegistry provides configurable domain knowledge
// for technology extraction. Data is loaded from embedded YAML defaults
// and can be overridden per-project via .workflow/deep-dives/*.yml.
package techref

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TYPES
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// KnownTech represents a known technology with optional aliases.
type KnownTech struct {
	Name     string   `yaml:"name"`
	Category string   `yaml:"-"` // set during loading from parent key
	Aliases  []string `yaml:"aliases,omitempty"`
}

// TechGroup represents a family of related technologies that share a single deep dive.
type TechGroup struct {
	Name    string   `yaml:"name"`
	Primary string   `yaml:"primary"`
	Members []string `yaml:"members"`
}

// CompoundConfig holds compound validation patterns.
type CompoundConfig struct {
	KnownWords      []string `yaml:"known_words"`
	KnownTechs      []string `yaml:"known_techs"`
	TechModifiers   []string `yaml:"tech_modifiers"`
	VerbsPT         []string `yaml:"verbs_pt"`
	VerbsEN         []string `yaml:"verbs_en"`
	BusinessTermsPT []string `yaml:"business_terms_pt"`
	BusinessTermsEN []string `yaml:"business_terms_en"`
}

// ConfidenceConfig holds scoring thresholds and adjustments.
type ConfidenceConfig struct {
	Thresholds struct {
		Min      float64 `yaml:"min"`
		High     float64 `yaml:"high"`
		VeryHigh float64 `yaml:"very_high"`
	} `yaml:"thresholds"`
	LayerScores      map[string]float64 `yaml:"layer_scores"`
	Penalties        map[string]float64 `yaml:"penalties"`
	Bonuses          map[string]float64 `yaml:"bonuses"`
	SpecificKeywords []string           `yaml:"specific_keywords"`
}

// TechRegistry holds all configurable domain knowledge for extraction.
type TechRegistry struct {
	// Core data
	KnownTechs       []KnownTech         // Sorted by name length DESC
	CanonicalForms    map[string]string   // lowercase variation → canonical
	TrivialTerms      map[string][]string // category → terms
	CommonWords       map[string][]string // category → words
	Verbs             map[string][]string // locale → verbs
	AcronymBlacklist  map[string]bool     // uppercase → true
	CompoundPatterns  CompoundConfig
	Confidence        ConfidenceConfig
	TechGroups        []TechGroup // Tech families

	// Template store (loaded from config/deep_dive_templates.yml)
	templateStore *templateStore

	// Derived caches (built once on load)
	techGroupMap         map[string]*TechGroup // lowercase member → group (flattened)
	trivialSet           map[string]bool // lowercase → true (flattened)
	commonWordSet        map[string]bool // exact case → true (flattened)
	verbSet              map[string]bool // lowercase → true (flattened)
	knownTechSet         map[string]bool // exact case → true (for context)
	relevanceKeywords    []string
	compoundWordsSet     map[string]bool // cached from CompoundPatterns
	compoundTechsSet     map[string]bool
	compoundModifiersSet map[string]bool
	compoundVerbsSet     map[string]bool
	compoundBusinessSet  map[string]bool
}

// RegistryOption configures a TechRegistry during creation.
type RegistryOption func(*TechRegistry)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// EMBEDDED DEFAULTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

//go:embed config/*.yml
var defaultConfigs embed.FS

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CONSTRUCTOR
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// NewTechRegistry creates a registry loaded with embedded defaults.
// Apply options for project-level overrides or custom additions.
func NewTechRegistry(opts ...RegistryOption) (*TechRegistry, error) {
	reg := &TechRegistry{
		CanonicalForms:   make(map[string]string),
		TrivialTerms:     make(map[string][]string),
		CommonWords:      make(map[string][]string),
		Verbs:            make(map[string][]string),
		AcronymBlacklist: make(map[string]bool),
	}

	// 1. Load embedded defaults
	if err := reg.loadEmbedded(); err != nil {
		return nil, err
	}

	// 2. Apply options (project overrides, additions)
	for _, opt := range opts {
		opt(reg)
	}

	// 3. Build derived caches
	reg.buildCaches()

	return reg, nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// OPTIONS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// WithProjectDir loads overrides from .workflow/deep-dives/*.yml.
func WithProjectDir(dir string) RegistryOption {
	return func(r *TechRegistry) {
		overrideDir := filepath.Join(dir, ".workflow", "deep-dives")
		if _, err := os.Stat(overrideDir); os.IsNotExist(err) {
			return // No overrides
		}
		r.mergeOverrides(overrideDir)
	}
}

// WithExtraTechs adds technologies to the registry.
func WithExtraTechs(techs ...KnownTech) RegistryOption {
	return func(r *TechRegistry) {
		r.KnownTechs = append(r.KnownTechs, techs...)
	}
}

// WithExtraCanonical adds canonical form mappings.
func WithExtraCanonical(forms map[string]string) RegistryOption {
	return func(r *TechRegistry) {
		for k, v := range forms {
			r.CanonicalForms[k] = v
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// LOADING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (r *TechRegistry) loadEmbedded() error {
	// Load known technologies
	if err := r.loadKnownTechs(); err != nil {
		return err
	}

	// Load canonical forms
	if err := r.loadCanonicalForms(); err != nil {
		return err
	}

	// Load trivial terms
	if err := r.loadTrivialTerms(); err != nil {
		return err
	}

	// Load common words
	if err := r.loadCommonWords(); err != nil {
		return err
	}

	// Load verbs
	if err := r.loadVerbs(); err != nil {
		return err
	}

	// Load acronym blacklist
	if err := r.loadAcronymBlacklist(); err != nil {
		return err
	}

	// Load compound patterns
	if err := r.loadCompoundPatterns(); err != nil {
		return err
	}

	// Load confidence config
	if err := r.loadConfidence(); err != nil {
		return err
	}

	// Load tech groups
	if err := r.loadTechGroups(); err != nil {
		return err
	}

	// Load deep dive templates (optional — no error on failure)
	r.loadTemplates()

	return nil
}

func (r *TechRegistry) loadKnownTechs() error {
	data, err := defaultConfigs.ReadFile("config/known_technologies.yml")
	if err != nil {
		return err
	}

	// Parse as map of category → list of techs
	var raw map[string][]KnownTech
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	for category, techs := range raw {
		for i := range techs {
			techs[i].Category = category
			r.KnownTechs = append(r.KnownTechs, techs[i])
		}
	}

	// Sort by name length DESC so longer names match first
	sort.Slice(r.KnownTechs, func(i, j int) bool {
		return len(r.KnownTechs[i].Name) > len(r.KnownTechs[j].Name)
	})

	return nil
}

func (r *TechRegistry) loadCanonicalForms() error {
	data, err := defaultConfigs.ReadFile("config/canonical_forms.yml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &r.CanonicalForms); err != nil {
		return err
	}
	return nil
}

func (r *TechRegistry) loadTrivialTerms() error {
	data, err := defaultConfigs.ReadFile("config/trivial_terms.yml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &r.TrivialTerms); err != nil {
		return err
	}
	return nil
}

func (r *TechRegistry) loadCommonWords() error {
	data, err := defaultConfigs.ReadFile("config/common_words.yml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &r.CommonWords); err != nil {
		return err
	}
	return nil
}

func (r *TechRegistry) loadVerbs() error {
	data, err := defaultConfigs.ReadFile("config/verbs.yml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &r.Verbs); err != nil {
		return err
	}
	return nil
}

func (r *TechRegistry) loadAcronymBlacklist() error {
	data, err := defaultConfigs.ReadFile("config/acronym_blacklist.yml")
	if err != nil {
		return err
	}

	var raw struct {
		Blacklist []string `yaml:"blacklist"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	for _, item := range raw.Blacklist {
		r.AcronymBlacklist[item] = true
	}
	return nil
}

func (r *TechRegistry) loadCompoundPatterns() error {
	data, err := defaultConfigs.ReadFile("config/compound_patterns.yml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &r.CompoundPatterns); err != nil {
		return err
	}
	return nil
}

func (r *TechRegistry) loadConfidence() error {
	data, err := defaultConfigs.ReadFile("config/confidence.yml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &r.Confidence); err != nil {
		return err
	}
	return nil
}

func (r *TechRegistry) loadTechGroups() error {
	data, err := defaultConfigs.ReadFile("config/tech_groups.yml")
	if err != nil {
		return err
	}

	var raw struct {
		Groups []TechGroup `yaml:"groups"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.TechGroups = raw.Groups
	return nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CACHE BUILDING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (r *TechRegistry) buildCaches() {
	// Flatten trivial terms into a set (lowercase)
	r.trivialSet = make(map[string]bool)
	for _, terms := range r.TrivialTerms {
		for _, term := range terms {
			r.trivialSet[strings.ToLower(term)] = true
		}
	}

	// Extract relevance keywords from trivial terms config
	if keywords, ok := r.TrivialTerms["relevance_keywords"]; ok {
		r.relevanceKeywords = keywords
		// Remove from trivial set (they're metadata, not trivial terms)
		for _, kw := range keywords {
			delete(r.trivialSet, strings.ToLower(kw))
		}
	}

	// Remove very_basic from trivial categories (add to trivialSet directly)
	if vb, ok := r.TrivialTerms["very_basic"]; ok {
		for _, term := range vb {
			r.trivialSet[strings.ToLower(term)] = true
		}
	}

	// Flatten common words into a set (exact case)
	r.commonWordSet = make(map[string]bool)
	for _, words := range r.CommonWords {
		for _, word := range words {
			r.commonWordSet[word] = true
		}
	}

	// Flatten verbs into a set (lowercase)
	r.verbSet = make(map[string]bool)
	for _, verbs := range r.Verbs {
		for _, verb := range verbs {
			r.verbSet[strings.ToLower(verb)] = true
		}
	}

	// Build known tech set for context analysis (exact case)
	r.knownTechSet = make(map[string]bool)
	for _, tech := range r.KnownTechs {
		r.knownTechSet[tech.Name] = true
		// Also add each word from multi-word techs
		for _, word := range strings.Fields(tech.Name) {
			if len(word) >= 3 {
				r.knownTechSet[word] = true
			}
		}
	}

	// Pre-compute compound pattern sets (avoid allocating per call)
	r.compoundWordsSet = make(map[string]bool, len(r.CompoundPatterns.KnownWords))
	for _, w := range r.CompoundPatterns.KnownWords {
		r.compoundWordsSet[w] = true
	}
	r.compoundTechsSet = make(map[string]bool, len(r.CompoundPatterns.KnownTechs))
	for _, t := range r.CompoundPatterns.KnownTechs {
		r.compoundTechsSet[t] = true
	}
	r.compoundModifiersSet = make(map[string]bool, len(r.CompoundPatterns.TechModifiers))
	for _, m := range r.CompoundPatterns.TechModifiers {
		r.compoundModifiersSet[m] = true
	}
	r.compoundVerbsSet = make(map[string]bool)
	for _, v := range r.CompoundPatterns.VerbsPT {
		r.compoundVerbsSet[v] = true
	}
	for _, v := range r.CompoundPatterns.VerbsEN {
		r.compoundVerbsSet[v] = true
	}
	r.compoundBusinessSet = make(map[string]bool)
	for _, t := range r.CompoundPatterns.BusinessTermsPT {
		r.compoundBusinessSet[t] = true
	}
	for _, t := range r.CompoundPatterns.BusinessTermsEN {
		r.compoundBusinessSet[t] = true
	}

	// Build tech group lookup map (lowercase member → group)
	r.techGroupMap = make(map[string]*TechGroup)
	for i := range r.TechGroups {
		group := &r.TechGroups[i]
		for _, member := range group.Members {
			r.techGroupMap[strings.ToLower(member)] = group
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// OVERRIDES (project-level merge)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (r *TechRegistry) mergeOverrides(dir string) {
	// Merge known technologies
	if data, err := os.ReadFile(filepath.Join(dir, "known_technologies.yml")); err == nil {
		var raw map[string][]KnownTech
		if err := yaml.Unmarshal(data, &raw); err != nil {
			log.Printf("⚠️  TechRegistry: erro ao parsear %s/known_technologies.yml: %v", dir, err)
		} else {
			for category, techs := range raw {
				for i := range techs {
					techs[i].Category = category
					r.KnownTechs = append(r.KnownTechs, techs[i])
				}
			}
			sort.Slice(r.KnownTechs, func(i, j int) bool {
				return len(r.KnownTechs[i].Name) > len(r.KnownTechs[j].Name)
			})
		}
	}

	// Merge canonical forms
	if data, err := os.ReadFile(filepath.Join(dir, "canonical_forms.yml")); err == nil {
		var extra map[string]string
		if err := yaml.Unmarshal(data, &extra); err != nil {
			log.Printf("⚠️  TechRegistry: erro ao parsear %s/canonical_forms.yml: %v", dir, err)
		} else {
			for k, v := range extra {
				r.CanonicalForms[k] = v
			}
		}
	}

	// Merge trivial terms
	if data, err := os.ReadFile(filepath.Join(dir, "trivial_terms.yml")); err == nil {
		var extra map[string][]string
		if err := yaml.Unmarshal(data, &extra); err != nil {
			log.Printf("⚠️  TechRegistry: erro ao parsear %s/trivial_terms.yml: %v", dir, err)
		} else {
			for category, terms := range extra {
				r.TrivialTerms[category] = append(r.TrivialTerms[category], terms...)
			}
		}
	}

	// Merge tech groups
	if data, err := os.ReadFile(filepath.Join(dir, "tech_groups.yml")); err == nil {
		var raw struct {
			Groups []TechGroup `yaml:"groups"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			log.Printf("⚠️  TechRegistry: erro ao parsear %s/tech_groups.yml: %v", dir, err)
		} else {
			r.TechGroups = append(r.TechGroups, raw.Groups...)
		}
	}

	// Merge confidence
	if data, err := os.ReadFile(filepath.Join(dir, "confidence.yml")); err == nil {
		var extra ConfidenceConfig
		if err := yaml.Unmarshal(data, &extra); err != nil {
			log.Printf("⚠️  TechRegistry: erro ao parsear %s/confidence.yml: %v", dir, err)
		} else {
			if extra.Thresholds.Min > 0 {
				r.Confidence.Thresholds.Min = extra.Thresholds.Min
			}
			if extra.Thresholds.High > 0 {
				r.Confidence.Thresholds.High = extra.Thresholds.High
			}
			if extra.Thresholds.VeryHigh > 0 {
				r.Confidence.Thresholds.VeryHigh = extra.Thresholds.VeryHigh
			}
			for k, v := range extra.LayerScores {
				r.Confidence.LayerScores[k] = v
			}
			for k, v := range extra.Penalties {
				r.Confidence.Penalties[k] = v
			}
			for k, v := range extra.Bonuses {
				r.Confidence.Bonuses[k] = v
			}
			if len(extra.SpecificKeywords) > 0 {
				r.Confidence.SpecificKeywords = append(r.Confidence.SpecificKeywords, extra.SpecificKeywords...)
			}
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PUBLIC QUERY METHODS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// IsTrivial checks if a term is trivial (should not generate deep dive).
func (r *TechRegistry) IsTrivial(term string) bool {
	normalized := strings.TrimSpace(strings.ToLower(term))
	if normalized == "" {
		return true
	}
	return r.trivialSet[normalized]
}

// IsTrivialWithContext checks if a term is trivial considering context.
func (r *TechRegistry) IsTrivialWithContext(term string, context string) bool {
	if !r.IsTrivial(term) {
		return false
	}

	contextLower := strings.ToLower(context)
	for _, keyword := range r.relevanceKeywords {
		if strings.Contains(contextLower, strings.ToLower(keyword)) {
			return false // Context makes it relevant
		}
	}
	return true
}

// FilterTrivialTerms removes trivial terms from a list.
func (r *TechRegistry) FilterTrivialTerms(terms []string) []string {
	var filtered []string
	for _, term := range terms {
		if !r.IsTrivial(term) {
			filtered = append(filtered, term)
		}
	}
	return filtered
}

// NormalizeToCanonical converts a term to its canonical form.
func (r *TechRegistry) NormalizeToCanonical(tech string) string {
	lower := strings.ToLower(strings.TrimSpace(tech))
	if canonical, exists := r.CanonicalForms[lower]; exists {
		return canonical
	}
	return tech
}

// IsVerb checks if a word is a verb (for context analysis).
func (r *TechRegistry) IsVerb(word string) bool {
	return r.verbSet[strings.ToLower(word)]
}

// IsKnownTech checks if a word is a known technology (for context scoring).
func (r *TechRegistry) IsKnownTech(word string) bool {
	return r.knownTechSet[word]
}

// IsCommonWord checks if a capitalized word is a common non-tech word.
func (r *TechRegistry) IsCommonWord(word string) bool {
	return r.commonWordSet[word]
}

// IsBlacklistedAcronym checks if an acronym should be excluded.
func (r *TechRegistry) IsBlacklistedAcronym(acronym string) bool {
	return r.AcronymBlacklist[acronym]
}

// KnownTechsSorted returns known techs sorted by name length DESC.
// Useful for longest-match-first extraction.
func (r *TechRegistry) KnownTechsSorted() []KnownTech {
	// Already sorted during loading
	return r.KnownTechs
}

// MinConfidence returns the minimum confidence threshold.
func (r *TechRegistry) MinConfidence() float64 {
	if r.Confidence.Thresholds.Min > 0 {
		return r.Confidence.Thresholds.Min
	}
	return 0.60 // default
}

// LayerScore returns the base confidence score for a layer.
func (r *TechRegistry) LayerScore(layer string) float64 {
	if score, ok := r.Confidence.LayerScores[layer]; ok {
		return score
	}
	// Defaults
	switch layer {
	case "known":
		return 1.0
	case "acronym":
		return 0.85
	case "compound":
		return 0.70
	case "isolated":
		return 0.50
	}
	return 0.0
}

// Penalty returns a penalty value by name.
func (r *TechRegistry) Penalty(name string) float64 {
	if val, ok := r.Confidence.Penalties[name]; ok {
		return val
	}
	return -0.15 // default
}

// Bonus returns a bonus value by name.
func (r *TechRegistry) Bonus(name string) float64 {
	if val, ok := r.Confidence.Bonuses[name]; ok {
		return val
	}
	return 0.05 // default
}

// SpecificKeywords returns keywords that indicate specific usage.
func (r *TechRegistry) SpecificKeywords() []string {
	return r.Confidence.SpecificKeywords
}

// FindGroup returns the tech group for a technology, or nil if it doesn't belong to any.
func (r *TechRegistry) FindGroup(tech string) *TechGroup {
	return r.techGroupMap[strings.ToLower(strings.TrimSpace(tech))]
}

// CompoundKnownWords returns the cached set of known compound words.
func (r *TechRegistry) CompoundKnownWords() map[string]bool {
	return r.compoundWordsSet
}

// CompoundKnownTechs returns the cached set of known tech names for compounds.
func (r *TechRegistry) CompoundKnownTechs() map[string]bool {
	return r.compoundTechsSet
}

// CompoundTechModifiers returns the cached set of valid tech modifiers.
func (r *TechRegistry) CompoundTechModifiers() map[string]bool {
	return r.compoundModifiersSet
}

// CompoundVerbs returns the cached set of verbs that invalidate compounds.
func (r *TechRegistry) CompoundVerbs() map[string]bool {
	return r.compoundVerbsSet
}

// CompoundBusinessTerms returns the cached set of business terms that invalidate compounds.
func (r *TechRegistry) CompoundBusinessTerms() map[string]bool {
	return r.compoundBusinessSet
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// DEFAULT SINGLETON
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

var (
	defaultRegistry     *TechRegistry
	defaultRegistryOnce sync.Once
	defaultRegistryErr  error
)

// DefaultRegistry returns the singleton default registry (embedded defaults only).
func DefaultRegistry() *TechRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry, defaultRegistryErr = NewTechRegistry()
	})
	if defaultRegistryErr != nil {
		// Fallback: return empty registry (should never happen with embedded)
		return &TechRegistry{
			CanonicalForms:   make(map[string]string),
			TrivialTerms:     make(map[string][]string),
			CommonWords:      make(map[string][]string),
			Verbs:            make(map[string][]string),
			AcronymBlacklist: make(map[string]bool),
			trivialSet:       make(map[string]bool),
			commonWordSet:    make(map[string]bool),
			verbSet:          make(map[string]bool),
			knownTechSet:     make(map[string]bool),
		}
	}
	return defaultRegistry
}

// ResetDefaultRegistry clears the singleton (useful for testing).
func ResetDefaultRegistry() {
	defaultRegistryOnce = sync.Once{}
	defaultRegistry = nil
	defaultRegistryErr = nil
}
