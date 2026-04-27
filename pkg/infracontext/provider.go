package infracontext

import "context"

// Provider is the interface for infrastructure context providers.
// Implementations fetch data from external systems (kubectl, Datadog, etc.)
// and normalize it into an InfraContext.
type Provider interface {
	// ID returns the unique provider identifier (e.g., "kubectl", "datadog").
	ID() string

	// Name returns the human-readable provider name.
	Name() string

	// Available checks if this provider can run on the current system.
	// For kubectl, this checks if the binary is installed and a cluster is accessible.
	Available() bool

	// Fetch retrieves infrastructure context with the given options.
	// Returns a normalized InfraContext or an error.
	Fetch(ctx context.Context, opts FetchOptions) (*InfraContext, error)
}

// FetchOptions configures what to fetch from the provider.
type FetchOptions struct {
	// Namespace restricts the query to a specific namespace.
	// Empty means use the provider's default.
	Namespace string

	// KubeContext selects a specific kubectl context.
	// Empty means use the current context.
	KubeContext string

	// ResourceTypes limits which resources to fetch (e.g., ["pods", "services"]).
	// Empty means fetch all configured resource types.
	ResourceTypes []string

	// UseCache enables TTL-based caching of results.
	UseCache bool

	// Verbose enables detailed logging.
	Verbose bool
}
