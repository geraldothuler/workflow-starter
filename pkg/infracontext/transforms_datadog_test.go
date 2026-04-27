package infracontext

import (
	"testing"
)

// --- dd_integrations_to_topology ---

func TestDDIntegrationsToTopology_HappyPath(t *testing.T) {
	items := []any{
		map[string]any{"name": "postgresql", "installed": true, "type": "check"},
		map[string]any{"name": "redis", "installed": true, "type": "check"},
		map[string]any{"name": "docker", "installed": true, "type": "crawler"},
	}

	result, err := CallTransform("dd_integrations_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	nodes, ok := result.([]TopologyNode)
	if !ok {
		t.Fatalf("expected []TopologyNode, got %T", result)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	if nodes[0].Kind != "Integration" {
		t.Errorf("kind = %q, want Integration", nodes[0].Kind)
	}
	if nodes[0].Name != "postgresql" {
		t.Errorf("name = %q, want postgresql", nodes[0].Name)
	}
}

func TestDDIntegrationsToTopology_FilterNotInstalled(t *testing.T) {
	items := []any{
		map[string]any{"name": "postgresql", "installed": true, "type": "check"},
		map[string]any{"name": "redis", "installed": false, "type": "check"},
	}

	result, err := CallTransform("dd_integrations_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Errorf("expected 1 installed integration, got %d", len(nodes))
	}
}

func TestDDIntegrationsToTopology_Empty(t *testing.T) {
	result, err := CallTransform("dd_integrations_to_topology", []any{}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestDDIntegrationsToTopology_MissingFields(t *testing.T) {
	items := []any{
		map[string]any{"installed": true}, // no name
		map[string]any{"name": "redis", "installed": true},
	}

	result, err := CallTransform("dd_integrations_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Errorf("expected 1 node (skip missing name), got %d", len(nodes))
	}
}

func TestDDIntegrationsToTopology_NonMapItems(t *testing.T) {
	items := []any{"not-a-map", 42}
	result, err := CallTransform("dd_integrations_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for non-map items, got %d", len(nodes))
	}
}

// --- dd_services_to_topology ---

func TestDDServicesToTopology_HappyPath(t *testing.T) {
	items := []any{
		map[string]any{
			"schema": map[string]any{
				"dd-service": "api-gateway",
				"languages":  []any{"go", "python"},
				"type":       "web",
				"tier":       "Tier 1",
				"team":       "platform",
			},
		},
	}

	result, err := CallTransform("dd_services_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "api-gateway" {
		t.Errorf("name = %q, want api-gateway", nodes[0].Name)
	}
	if nodes[0].Metadata["languages"] != "go,python" {
		t.Errorf("languages = %q", nodes[0].Metadata["languages"])
	}
	if nodes[0].Metadata["tier"] != "Tier 1" {
		t.Errorf("tier = %q", nodes[0].Metadata["tier"])
	}
}

func TestDDServicesToTopology_FlatStructure(t *testing.T) {
	items := []any{
		map[string]any{
			"name": "worker-service",
			"type": "background",
			"team": "data",
		},
	}

	result, err := CallTransform("dd_services_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "worker-service" {
		t.Errorf("name = %q, want worker-service", nodes[0].Name)
	}
}

func TestDDServicesToTopology_Empty(t *testing.T) {
	result, err := CallTransform("dd_services_to_topology", []any{}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestDDServicesToTopology_MissingName(t *testing.T) {
	items := []any{
		map[string]any{"schema": map[string]any{"type": "web"}},
	}
	result, err := CallTransform("dd_services_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for missing name, got %d", len(nodes))
	}
}

// --- dd_hosts_to_topology ---

func TestDDHostsToTopology_HappyPath(t *testing.T) {
	items := []any{
		map[string]any{
			"name":     "host-1.prod",
			"up":       true,
			"is_muted": false,
			"apps":     []any{"nginx", "postgresql"},
			"meta": map[string]any{
				"agent_version": "7.40.0",
				"platform":      "linux",
			},
		},
	}

	result, err := CallTransform("dd_hosts_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Kind != "Host" {
		t.Errorf("kind = %q, want Host", nodes[0].Kind)
	}
	if nodes[0].Status != "Active" {
		t.Errorf("status = %q, want Active", nodes[0].Status)
	}
	if nodes[0].Metadata["apps"] != "nginx,postgresql" {
		t.Errorf("apps = %q", nodes[0].Metadata["apps"])
	}
	if nodes[0].Metadata["agent_version"] != "7.40.0" {
		t.Errorf("agent_version = %q", nodes[0].Metadata["agent_version"])
	}
}

func TestDDHostsToTopology_DownHost(t *testing.T) {
	items := []any{
		map[string]any{"name": "host-down", "up": false, "is_muted": false},
	}
	result, err := CallTransform("dd_hosts_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if nodes[0].Status != "Down" {
		t.Errorf("status = %q, want Down", nodes[0].Status)
	}
}

func TestDDHostsToTopology_MutedHost(t *testing.T) {
	items := []any{
		map[string]any{"name": "host-muted", "up": true, "is_muted": true},
	}
	result, err := CallTransform("dd_hosts_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if nodes[0].Status != "Muted" {
		t.Errorf("status = %q, want Muted", nodes[0].Status)
	}
}

func TestDDHostsToTopology_Empty(t *testing.T) {
	result, err := CallTransform("dd_hosts_to_topology", []any{}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestDDHostsToTopology_HostNameFallback(t *testing.T) {
	items := []any{
		map[string]any{"host_name": "fallback-host", "up": true},
	}
	result, err := CallTransform("dd_hosts_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if nodes[0].Name != "fallback-host" {
		t.Errorf("name = %q, want fallback-host", nodes[0].Name)
	}
}

// --- dd_monitors_to_alerts ---

func TestDDMonitorsToAlerts_HappyPath(t *testing.T) {
	items := []any{
		map[string]any{
			"name":          "High CPU on api-server",
			"overall_state": "Alert",
			"priority":      1.0,
			"message":       "CPU > 90%",
			"created":       "2026-01-15T10:00:00Z",
		},
		map[string]any{
			"name":          "Disk Usage",
			"overall_state": "OK",
			"priority":      4.0,
			"message":       "Disk OK",
		},
	}

	result, err := CallTransform("dd_monitors_to_alerts", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	alerts := result.([]Alert)
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}

	// First: critical, firing
	if alerts[0].Severity != AlertSeverityCritical {
		t.Errorf("severity = %q, want critical", alerts[0].Severity)
	}
	if alerts[0].Status != AlertStatusFiring {
		t.Errorf("status = %q, want firing", alerts[0].Status)
	}
	if alerts[0].Message != "CPU > 90%" {
		t.Errorf("message = %q", alerts[0].Message)
	}
	if alerts[0].FiredAt.IsZero() {
		t.Error("expected non-zero FiredAt")
	}

	// Second: info, resolved
	if alerts[1].Severity != AlertSeverityInfo {
		t.Errorf("severity = %q, want info", alerts[1].Severity)
	}
	if alerts[1].Status != AlertStatusResolved {
		t.Errorf("status = %q, want resolved", alerts[1].Status)
	}
}

func TestDDMonitorsToAlerts_WarnState(t *testing.T) {
	items := []any{
		map[string]any{
			"name":          "Memory Warning",
			"overall_state": "Warn",
			"priority":      3.0,
			"message":       "Memory > 80%",
		},
	}
	result, err := CallTransform("dd_monitors_to_alerts", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	alerts := result.([]Alert)
	if alerts[0].Status != AlertStatusPending {
		t.Errorf("status = %q, want pending", alerts[0].Status)
	}
	if alerts[0].Severity != AlertSeverityWarning {
		t.Errorf("severity = %q, want warning", alerts[0].Severity)
	}
}

func TestDDMonitorsToAlerts_TagPriority(t *testing.T) {
	items := []any{
		map[string]any{
			"name":          "Critical via tag",
			"overall_state": "Alert",
			"tags":          []any{"priority:P1", "env:prod"},
		},
	}
	result, err := CallTransform("dd_monitors_to_alerts", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	alerts := result.([]Alert)
	if alerts[0].Severity != AlertSeverityCritical {
		t.Errorf("severity = %q, want critical (from tag)", alerts[0].Severity)
	}
}

func TestDDMonitorsToAlerts_Empty(t *testing.T) {
	result, err := CallTransform("dd_monitors_to_alerts", []any{}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	alerts := result.([]Alert)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestDDMonitorsToAlerts_MissingName(t *testing.T) {
	items := []any{
		map[string]any{"overall_state": "OK"},
	}
	result, err := CallTransform("dd_monitors_to_alerts", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	alerts := result.([]Alert)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for missing name, got %d", len(alerts))
	}
}

// --- dd_host_metrics ---

func TestDDHostMetrics_HappyPath(t *testing.T) {
	items := []any{
		map[string]any{
			"name": "host-1",
			"meta": map[string]any{
				"gohai": map[string]any{
					"cpu":    map[string]any{"cpu_cores": "4"},
					"memory": map[string]any{"total": "8589934592"},
				},
			},
		},
	}

	result, err := CallTransform("dd_host_metrics", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	metrics := result.([]Metric)
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	// CPU cores
	if metrics[0].Name != "cpu_cores" {
		t.Errorf("name = %q, want cpu_cores", metrics[0].Name)
	}
	if metrics[0].Value != 4.0 {
		t.Errorf("cpu value = %f, want 4", metrics[0].Value)
	}
	if metrics[0].Labels["host"] != "host-1" {
		t.Errorf("host label = %q", metrics[0].Labels["host"])
	}

	// Memory
	if metrics[1].Name != "memory_total" {
		t.Errorf("name = %q, want memory_total", metrics[1].Name)
	}
}

func TestDDHostMetrics_Empty(t *testing.T) {
	result, err := CallTransform("dd_host_metrics", []any{}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	metrics := result.([]Metric)
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics, got %d", len(metrics))
	}
}

func TestDDHostMetrics_MissingGohai(t *testing.T) {
	items := []any{
		map[string]any{"name": "host-1", "meta": map[string]any{}},
	}
	result, err := CallTransform("dd_host_metrics", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	metrics := result.([]Metric)
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for missing gohai, got %d", len(metrics))
	}
}

func TestDDHostMetrics_FallbackFields(t *testing.T) {
	items := []any{
		map[string]any{
			"name": "host-1",
			"meta": map[string]any{
				"cpuCores":    "8",
				"totalMemory": "17179869184",
			},
		},
	}
	result, err := CallTransform("dd_host_metrics", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	metrics := result.([]Metric)
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics from fallback fields, got %d", len(metrics))
	}
	if metrics[0].Value != 8.0 {
		t.Errorf("cpu value = %f, want 8", metrics[0].Value)
	}
}

// --- dd_dependencies_to_topology ---

func TestDDDependenciesToTopology_HappyPath(t *testing.T) {
	items := []any{
		map[string]any{
			"service": "api-gateway",
			"deps":    []any{"user-service", "auth-service"},
		},
		map[string]any{
			"service": "user-service",
			"deps":    []any{"database"},
		},
	}

	result, err := CallTransform("dd_dependencies_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Kind != "ServiceDependency" {
		t.Errorf("kind = %q, want ServiceDependency", nodes[0].Kind)
	}
	if len(nodes[0].ConnectsTo) != 2 {
		t.Errorf("connectsTo = %d, want 2", len(nodes[0].ConnectsTo))
	}
}

func TestDDDependenciesToTopology_CallsFormat(t *testing.T) {
	items := []any{
		map[string]any{
			"service": "api",
			"calls": []any{
				map[string]any{"service": "db"},
				map[string]any{"service": "cache"},
			},
		},
	}

	result, err := CallTransform("dd_dependencies_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if len(nodes[0].ConnectsTo) != 2 {
		t.Errorf("connectsTo = %d, want 2", len(nodes[0].ConnectsTo))
	}
}

func TestDDDependenciesToTopology_Dedup(t *testing.T) {
	items := []any{
		map[string]any{"service": "api", "deps": []any{"db"}},
		map[string]any{"service": "api", "deps": []any{"cache"}}, // duplicate service
	}

	result, err := CallTransform("dd_dependencies_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Errorf("expected 1 deduplicated node, got %d", len(nodes))
	}
}

func TestDDDependenciesToTopology_Empty(t *testing.T) {
	result, err := CallTransform("dd_dependencies_to_topology", []any{}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestDDDependenciesToTopology_NameFallback(t *testing.T) {
	items := []any{
		map[string]any{"name": "svc-via-name", "deps": []any{"other"}},
	}
	result, err := CallTransform("dd_dependencies_to_topology", items, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "svc-via-name" {
		t.Errorf("name = %q, want svc-via-name", nodes[0].Name)
	}
}
