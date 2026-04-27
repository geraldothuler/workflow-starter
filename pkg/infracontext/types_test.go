package infracontext

import (
	"strings"
	"testing"
	"time"
)

func TestInfraContext_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		fetchedAt time.Time
		ttl       time.Duration
		want      bool
	}{
		{
			name:      "not expired",
			fetchedAt: time.Now(),
			ttl:       5 * time.Minute,
			want:      false,
		},
		{
			name:      "expired",
			fetchedAt: time.Now().Add(-10 * time.Minute),
			ttl:       5 * time.Minute,
			want:      true,
		},
		{
			name:      "zero TTL never expires",
			fetchedAt: time.Now().Add(-1 * time.Hour),
			ttl:       0,
			want:      false,
		},
		{
			name:      "just expired",
			fetchedAt: time.Now().Add(-5*time.Minute - time.Second),
			ttl:       5 * time.Minute,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := &InfraContext{
				FetchedAt: tt.fetchedAt,
				TTL:       tt.ttl,
			}
			got := ic.IsExpired()
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInfraContext_TechMap(t *testing.T) {
	mapper := NewTechMapperFromMap(map[string]string{
		"postgres": "PostgreSQL",
		"redis":    "Redis",
		"nginx":    "Nginx",
	})

	ic := &InfraContext{
		Topology: []TopologyNode{
			{
				Kind: "Pod",
				Name: "pg-0",
				Containers: []ContainerInfo{
					{Name: "postgres", Image: "postgres:15.4"},
				},
			},
			{
				Kind: "Pod",
				Name: "cache-0",
				Containers: []ContainerInfo{
					{Name: "redis", Image: "redis:7.2"},
				},
			},
			{
				Kind: "Pod",
				Name: "app-0",
				Containers: []ContainerInfo{
					{Name: "app", Image: "myregistry/myapp:latest"},
				},
			},
		},
	}

	techMap := ic.TechMap(mapper)
	if techMap["PostgreSQL"] != "postgres:15.4" {
		t.Errorf("expected PostgreSQL -> postgres:15.4, got %q", techMap["PostgreSQL"])
	}
	if techMap["Redis"] != "redis:7.2" {
		t.Errorf("expected Redis -> redis:7.2, got %q", techMap["Redis"])
	}
	if _, ok := techMap["myapp"]; ok {
		t.Error("expected no match for myapp")
	}
}

func TestInfraContext_TechMap_NilMapper(t *testing.T) {
	ic := &InfraContext{}
	techMap := ic.TechMap(nil)
	if len(techMap) != 0 {
		t.Errorf("expected empty map with nil mapper, got %d entries", len(techMap))
	}
}

func TestInfraContext_HealthByTech(t *testing.T) {
	ic := &InfraContext{
		Health: []HealthCheck{
			{Component: "api-deploy", Kind: "Deployment", Status: HealthStatusHealthy},
			{Component: "redis-deploy", Kind: "Deployment", Status: HealthStatusUnhealthy},
			{Component: "node-1", Kind: "Node", Status: HealthStatusHealthy},
		},
	}

	byTech := ic.HealthByTech()
	if len(byTech["Deployment"]) != 2 {
		t.Errorf("expected 2 Deployment health checks, got %d", len(byTech["Deployment"]))
	}
	if len(byTech["Node"]) != 1 {
		t.Errorf("expected 1 Node health check, got %d", len(byTech["Node"]))
	}
}

func TestInfraContext_HealthByComponent(t *testing.T) {
	ic := &InfraContext{
		Health: []HealthCheck{
			{Component: "api-deploy", Status: HealthStatusHealthy},
			{Component: "redis-deploy", Status: HealthStatusUnhealthy},
		},
	}

	byComp := ic.HealthByComponent()
	if byComp["api-deploy"].Status != HealthStatusHealthy {
		t.Errorf("expected api-deploy healthy, got %q", byComp["api-deploy"].Status)
	}
	if byComp["redis-deploy"].Status != HealthStatusUnhealthy {
		t.Errorf("expected redis-deploy unhealthy, got %q", byComp["redis-deploy"].Status)
	}
}

func TestInfraContext_Summary(t *testing.T) {
	ic := &InfraContext{
		Provider:  "kubectl",
		Cluster:   "prod-cluster",
		Namespace: "default",
		Topology: []TopologyNode{
			{Kind: "Node"},
			{Kind: "Node"},
			{Kind: "Pod"},
			{Kind: "Pod"},
			{Kind: "Pod"},
			{Kind: "Service"},
			{Kind: "Deployment"},
			{Kind: "Deployment"},
		},
		Health: []HealthCheck{
			{Status: HealthStatusHealthy},
			{Status: HealthStatusHealthy},
			{Status: HealthStatusDegraded},
			{Status: HealthStatusUnhealthy},
		},
		Metrics: []Metric{{Name: "cpu"}, {Name: "mem"}},
		Alerts:  []Alert{{Name: "high-cpu"}},
	}

	summary := ic.Summary()
	if !strings.Contains(summary, "kubectl/prod-cluster/default") {
		t.Errorf("expected provider/cluster/namespace in summary, got %q", summary)
	}
	if !strings.Contains(summary, "nodes=2") {
		t.Errorf("expected nodes=2 in summary, got %q", summary)
	}
	if !strings.Contains(summary, "pods=3") {
		t.Errorf("expected pods=3 in summary, got %q", summary)
	}
	if !strings.Contains(summary, "svcs=1") {
		t.Errorf("expected svcs=1 in summary, got %q", summary)
	}
	if !strings.Contains(summary, "deploys=2") {
		t.Errorf("expected deploys=2 in summary, got %q", summary)
	}
	if !strings.Contains(summary, "2 ok") {
		t.Errorf("expected '2 ok' in summary, got %q", summary)
	}
	if !strings.Contains(summary, "1 degraded") {
		t.Errorf("expected '1 degraded' in summary, got %q", summary)
	}
	if !strings.Contains(summary, "1 unhealthy") {
		t.Errorf("expected '1 unhealthy' in summary, got %q", summary)
	}
	if !strings.Contains(summary, "metrics=2") {
		t.Errorf("expected metrics=2 in summary, got %q", summary)
	}
	if !strings.Contains(summary, "alerts=1") {
		t.Errorf("expected alerts=1 in summary, got %q", summary)
	}
}

func TestInfraContext_Summary_NoNamespace(t *testing.T) {
	ic := &InfraContext{
		Provider: "kubectl",
		Cluster:  "dev",
	}
	summary := ic.Summary()
	if !strings.Contains(summary, "kubectl/dev") {
		t.Errorf("expected kubectl/dev in summary, got %q", summary)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{-5, "-5"},
	}
	for _, tt := range tests {
		got := itoa(tt.input)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
