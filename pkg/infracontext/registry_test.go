package infracontext

import (
	"context"
	"testing"
)

// stubProvider is a simple Provider implementation for testing.
type stubProvider struct {
	id        string
	name      string
	available bool
}

func (s *stubProvider) ID() string   { return s.id }
func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) Available() bool { return s.available }
func (s *stubProvider) Fetch(ctx context.Context, opts FetchOptions) (*InfraContext, error) {
	return &InfraContext{Provider: s.id}, nil
}

func TestRegistry_Register_Get(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubProvider{id: "kubectl", name: "Kubectl", available: true})
	reg.Register(&stubProvider{id: "datadog", name: "Datadog", available: false})

	// Get existing
	p, err := reg.Get("kubectl")
	if err != nil {
		t.Fatalf("Get(kubectl) error: %v", err)
	}
	if p.ID() != "kubectl" {
		t.Errorf("ID = %q, want kubectl", p.ID())
	}

	// Get non-existing
	_, err = reg.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestRegistry_Available(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubProvider{id: "kubectl", name: "Kubectl", available: true})
	reg.Register(&stubProvider{id: "datadog", name: "Datadog", available: false})
	reg.Register(&stubProvider{id: "prometheus", name: "Prometheus", available: true})

	available := reg.Available()
	if len(available) != 2 {
		t.Errorf("expected 2 available providers, got %d", len(available))
	}

	// Verify IDs
	ids := make(map[string]bool)
	for _, p := range available {
		ids[p.ID()] = true
	}
	if !ids["kubectl"] {
		t.Error("expected kubectl to be available")
	}
	if !ids["prometheus"] {
		t.Error("expected prometheus to be available")
	}
	if ids["datadog"] {
		t.Error("expected datadog to NOT be available")
	}
}

func TestRegistry_All(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubProvider{id: "kubectl", available: true})
	reg.Register(&stubProvider{id: "datadog", available: false})

	all := reg.All()
	if len(all) != 2 {
		t.Errorf("expected 2 providers, got %d", len(all))
	}
}

func TestRegistry_IDs(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubProvider{id: "kubectl"})
	reg.Register(&stubProvider{id: "datadog"})

	ids := reg.IDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}
}

func TestNewDefaultRegistry(t *testing.T) {
	registry, err := NewDefaultRegistry("", nil)
	if err != nil {
		t.Fatalf("NewDefaultRegistry error: %v", err)
	}

	// Should have at least kubectl
	ids := registry.IDs()
	if len(ids) == 0 {
		t.Fatal("expected at least 1 provider")
	}

	found := false
	for _, id := range ids {
		if id == "kubectl" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected kubectl in default registry")
	}
}
