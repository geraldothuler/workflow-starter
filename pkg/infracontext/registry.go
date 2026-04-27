package infracontext

import (
	"fmt"
	"sync"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
)

// Registry manages available infrastructure providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID()] = p
}

// Get returns a provider by ID.
func (r *Registry) Get(id string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("infra provider %q not found", id)
	}
	return p, nil
}

// Available returns providers that are available on the current system.
func (r *Registry) Available() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Provider
	for _, p := range r.providers {
		if p.Available() {
			result = append(result, p)
		}
	}
	return result
}

// All returns all registered providers regardless of availability.
func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

// IDs returns all registered provider IDs.
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

// NewDefaultRegistry loads embedded provider configs and creates a registry
// with ConfigProvider instances for each.
func NewDefaultRegistry(projectDir string, credResolver *credentials.Resolver) (*Registry, error) {
	specs, err := LoadProviderConfigs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("loading provider configs: %w", err)
	}

	techMapper, err := NewTechMapper(projectDir)
	if err != nil {
		return nil, fmt.Errorf("loading tech mapper: %w", err)
	}

	cacheDir := ""
	if projectDir != "" {
		cacheDir = projectDir + "/.workflow/cache/infra"
	}
	cache := NewCache(cacheDir)

	registry := NewRegistry()
	for _, spec := range specs {
		provider := NewConfigProvider(spec, techMapper, cache, credResolver)
		registry.Register(provider)
	}

	return registry, nil
}
