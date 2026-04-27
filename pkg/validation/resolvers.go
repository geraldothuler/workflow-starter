package validation

import "github.com/Cobliteam/workflow-toolkit/pkg/types"

// AutoResolver resolve gaps automaticamente
type AutoResolver struct{}

// NewAutoResolver cria auto resolver
func NewAutoResolver(verbose ...bool) *AutoResolver {
	return &AutoResolver{}
}

// InteractiveResolver resolve gaps interativamente
type InteractiveResolver struct{}

// NewInteractiveResolver cria interactive resolver
func NewInteractiveResolver() *InteractiveResolver {
	return &InteractiveResolver{}
}

// Resolve resolve gaps automaticamente
func (r *AutoResolver) Resolve(gaps []types.Gap) ([]types.Resolution, error) {
	return []types.Resolution{}, nil
}

// Resolve resolve gaps interativamente
func (r *InteractiveResolver) Resolve(gaps []types.Gap) ([]types.Resolution, error) {
	return []types.Resolution{}, nil
}
