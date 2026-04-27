package docs

// UseCaseDef is the subset of use-cases/*/definition.yml relevant to docs generation.
type UseCaseDef struct {
	ID    string   `yaml:"id"`
	Name  string   `yaml:"name"`
	Type  string   `yaml:"type"` // "documentary" | "pipeline"
	Chain ChainDef `yaml:"chain"`
}

// ChainDef represents the chain section of a definition.yml.
type ChainDef struct {
	From []string `yaml:"from"`
	To   []string `yaml:"to"`
}
