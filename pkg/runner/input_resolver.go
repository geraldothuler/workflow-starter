package runner

// InputResolver resolves a single input value from a specific source.
type InputResolver interface {
	Name() string
	Resolve(spec InputSpec, inputs RunInputs, ctx ResolveContext) (string, bool)
}

// ResolveContext provides access to environment for resolvers.
type ResolveContext struct {
	WorkflowHome string
	PersonalDir  string
	RepoPath     string
	Definition   *UseCaseDefinition
	Term         TerminalIO
	Config       *ResolveConfig
}

// ResolverRegistry holds named resolvers and their configuration.
type ResolverRegistry struct {
	resolvers map[string]InputResolver
	config    *ResolveConfig
}

// NewResolverRegistry creates an empty registry.
func NewResolverRegistry(cfg *ResolveConfig) *ResolverRegistry {
	return &ResolverRegistry{
		resolvers: make(map[string]InputResolver),
		config:    cfg,
	}
}

// Register adds a resolver to the registry.
func (r *ResolverRegistry) Register(resolver InputResolver) {
	r.resolvers[resolver.Name()] = resolver
}

// DefaultResolverRegistry creates a registry with all 5 built-in resolvers.
func DefaultResolverRegistry(cfg *ResolveConfig) *ResolverRegistry {
	reg := NewResolverRegistry(cfg)
	reg.Register(&SessionResolver{})
	reg.Register(&ChainResolver{})
	reg.Register(&HelmResolver{})
	reg.Register(&RepoResolver{})
	reg.Register(&AskResolver{})
	return reg
}

// ResolveChain returns the resolution chain for a given input spec.
// Uses spec.Resolve if set, otherwise falls back to config default chain.
func (r *ResolverRegistry) ResolveChain(spec InputSpec) []string {
	if len(spec.Resolve) > 0 {
		return spec.Resolve
	}
	return r.config.Resolve.DefaultChain
}

// Get returns a resolver by name, or nil if not found.
func (r *ResolverRegistry) Get(name string) InputResolver {
	return r.resolvers[name]
}
