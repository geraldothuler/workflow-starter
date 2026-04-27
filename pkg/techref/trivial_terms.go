package techref

// TrivialTerms kept for backward compatibility.
// Prefer using registry.IsTrivial().
var TrivialTerms = struct {
	Protocols      []string
	Formats        []string
	HTTPMethods    []string
	BasicConcepts  []string
	DataStructures []string
	BasicAuth      []string
	GenericDB      []string
	BasicFrontend  []string
	Platforms      []string
	CommonAbbrev   []string
}{}

func init() {
	reg := DefaultRegistry()
	if reg == nil {
		return
	}
	// Populate from registry for backward compat
	TrivialTerms.Protocols = reg.TrivialTerms["protocols"]
	TrivialTerms.Formats = reg.TrivialTerms["formats"]
	TrivialTerms.HTTPMethods = reg.TrivialTerms["http_methods"]
	TrivialTerms.BasicConcepts = reg.TrivialTerms["basic_concepts"]
	TrivialTerms.DataStructures = reg.TrivialTerms["data_structures"]
	TrivialTerms.BasicAuth = reg.TrivialTerms["basic_auth"]
	TrivialTerms.GenericDB = reg.TrivialTerms["generic_db"]
	TrivialTerms.BasicFrontend = reg.TrivialTerms["basic_frontend"]
	TrivialTerms.Platforms = reg.TrivialTerms["platforms"]
	TrivialTerms.CommonAbbrev = reg.TrivialTerms["common_abbrev"]
}

// IsTrivial verifica se um termo é trivial (não precisa deep dive).
// Backward compat — delegates to registry.
func IsTrivial(term string) bool {
	return DefaultRegistry().IsTrivial(term)
}

// IsTrivialWithContext verifica se um termo é trivial considerando o contexto.
func IsTrivialWithContext(term string, context string) bool {
	return DefaultRegistry().IsTrivialWithContext(term, context)
}

// FilterTrivialTerms remove termos triviais de uma lista.
func FilterTrivialTerms(terms []string) []string {
	return DefaultRegistry().FilterTrivialTerms(terms)
}

// FilterTrivialTermsWithContext remove termos triviais considerando contexto.
func FilterTrivialTermsWithContext(terms []string, context string) []string {
	reg := DefaultRegistry()
	var filtered []string
	for _, term := range terms {
		if !reg.IsTrivialWithContext(term, context) {
			filtered = append(filtered, term)
		}
	}
	return filtered
}

// GetTrivialCategories retorna as categorias de termos triviais (para debug/config)
func GetTrivialCategories() map[string][]string {
	return map[string][]string{
		"Protocols":      TrivialTerms.Protocols,
		"Formats":        TrivialTerms.Formats,
		"HTTPMethods":    TrivialTerms.HTTPMethods,
		"BasicConcepts":  TrivialTerms.BasicConcepts,
		"DataStructures": TrivialTerms.DataStructures,
		"BasicAuth":      TrivialTerms.BasicAuth,
		"GenericDB":      TrivialTerms.GenericDB,
		"BasicFrontend":  TrivialTerms.BasicFrontend,
		"Platforms":      TrivialTerms.Platforms,
		"CommonAbbrev":   TrivialTerms.CommonAbbrev,
	}
}

