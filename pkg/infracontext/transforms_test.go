package infracontext

import (
	"testing"
)

func TestHealthFromConditions(t *testing.T) {
	tests := []struct {
		name       string
		items      []any
		args       map[string]any
		wantCount  int
		wantStatus string
	}{
		{
			name: "healthy deployment",
			items: []any{
				map[string]any{
					"metadata": map[string]any{"name": "api-server"},
					"status": map[string]any{
						"conditions": []any{
							map[string]any{"type": "Available", "status": "True"},
						},
					},
				},
			},
			args: map[string]any{
				"name_path":       "metadata.name",
				"kind":            "Deployment",
				"conditions_path": "status.conditions",
				"ready_type":      "Available",
			},
			wantCount:  1,
			wantStatus: HealthStatusHealthy,
		},
		{
			name: "unhealthy deployment",
			items: []any{
				map[string]any{
					"metadata": map[string]any{"name": "redis-cache"},
					"status": map[string]any{
						"conditions": []any{
							map[string]any{"type": "Available", "status": "False", "message": "not enough replicas"},
						},
					},
				},
			},
			args: map[string]any{
				"name_path":       "metadata.name",
				"kind":            "Deployment",
				"conditions_path": "status.conditions",
				"ready_type":      "Available",
			},
			wantCount:  1,
			wantStatus: HealthStatusUnhealthy,
		},
		{
			name: "node with no ready condition",
			items: []any{
				map[string]any{
					"metadata": map[string]any{"name": "node-1"},
					"status": map[string]any{
						"conditions": []any{
							map[string]any{"type": "MemoryPressure", "status": "False"},
						},
					},
				},
			},
			args: map[string]any{
				"name_path":       "metadata.name",
				"kind":            "Node",
				"conditions_path": "status.conditions",
				"ready_type":      "Ready",
			},
			wantCount:  1,
			wantStatus: HealthStatusUnknown,
		},
		{
			name:      "empty items",
			items:     []any{},
			args:      map[string]any{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := healthFromConditions(tt.items, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			checks := result.([]HealthCheck)
			if len(checks) != tt.wantCount {
				t.Fatalf("got %d checks, want %d", len(checks), tt.wantCount)
			}
			if tt.wantCount > 0 && checks[0].Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", checks[0].Status, tt.wantStatus)
			}
		})
	}
}

func TestHealthFromPodStatus(t *testing.T) {
	tests := []struct {
		name       string
		items      []any
		wantStatus string
		wantMsg    string
	}{
		{
			name: "running and ready",
			items: []any{
				map[string]any{
					"metadata": map[string]any{"name": "pod-1"},
					"status": map[string]any{
						"phase": "Running",
						"conditions": []any{
							map[string]any{"type": "Ready", "status": "True"},
						},
					},
				},
			},
			wantStatus: HealthStatusHealthy,
		},
		{
			name: "running but not ready",
			items: []any{
				map[string]any{
					"metadata": map[string]any{"name": "pod-2"},
					"status": map[string]any{
						"phase": "Running",
						"conditions": []any{
							map[string]any{"type": "Ready", "status": "False", "message": "container not ready"},
						},
					},
				},
			},
			wantStatus: HealthStatusDegraded,
			wantMsg:    "container not ready",
		},
		{
			name: "pending pod",
			items: []any{
				map[string]any{
					"metadata": map[string]any{"name": "pod-3"},
					"status": map[string]any{
						"phase":      "Pending",
						"conditions": []any{},
					},
				},
			},
			wantStatus: HealthStatusDegraded,
			wantMsg:    "pending",
		},
		{
			name: "failed pod",
			items: []any{
				map[string]any{
					"metadata": map[string]any{"name": "pod-4"},
					"status": map[string]any{
						"phase":      "Failed",
						"conditions": []any{},
					},
				},
			},
			wantStatus: HealthStatusUnhealthy,
			wantMsg:    "failed",
		},
		{
			name: "succeeded pod",
			items: []any{
				map[string]any{
					"metadata": map[string]any{"name": "pod-5"},
					"status": map[string]any{
						"phase":      "Succeeded",
						"conditions": []any{},
					},
				},
			},
			wantStatus: HealthStatusHealthy,
			wantMsg:    "completed",
		},
	}

	args := map[string]any{
		"name_path":       "metadata.name",
		"phase_path":      "status.phase",
		"conditions_path": "status.conditions",
		"ready_type":      "Ready",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := healthFromPodStatus(tt.items, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			checks := result.([]HealthCheck)
			if len(checks) != 1 {
				t.Fatalf("got %d checks, want 1", len(checks))
			}
			if checks[0].Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", checks[0].Status, tt.wantStatus)
			}
			if tt.wantMsg != "" && checks[0].Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", checks[0].Message, tt.wantMsg)
			}
		})
	}
}

func TestDeploymentStatus(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "available",
			item: map[string]any{
				"status": map[string]any{
					"conditions": []any{
						map[string]any{"type": "Available", "status": "True"},
					},
				},
			},
			want: "Available",
		},
		{
			name: "unavailable",
			item: map[string]any{
				"status": map[string]any{
					"conditions": []any{
						map[string]any{"type": "Available", "status": "False"},
					},
				},
			},
			want: "Unavailable",
		},
		{
			name: "no available condition",
			item: map[string]any{
				"status": map[string]any{
					"conditions": []any{
						map[string]any{"type": "Progressing", "status": "True"},
					},
				},
			},
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := deploymentStatus([]any{tt.item}, map[string]any{
				"conditions_path": "status.conditions",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := result.(string)
			if got != tt.want {
				t.Errorf("deploymentStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeStatus(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "ready node",
			item: map[string]any{
				"status": map[string]any{
					"conditions": []any{
						map[string]any{"type": "Ready", "status": "True"},
					},
				},
			},
			want: "Ready",
		},
		{
			name: "not ready node",
			item: map[string]any{
				"status": map[string]any{
					"conditions": []any{
						map[string]any{"type": "Ready", "status": "False"},
					},
				},
			},
			want: "NotReady",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := nodeStatus([]any{tt.item}, map[string]any{
				"conditions_path": "status.conditions",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := result.(string)
			if got != tt.want {
				t.Errorf("nodeStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatPorts(t *testing.T) {
	tests := []struct {
		name  string
		items []any
		want  string
	}{
		{
			name: "single port",
			items: []any{
				map[string]any{"name": "http", "port": 8080, "protocol": "TCP"},
			},
			want: "http:8080/TCP",
		},
		{
			name: "multiple ports",
			items: []any{
				map[string]any{"name": "http", "port": 80, "protocol": "TCP"},
				map[string]any{"name": "https", "port": 443, "protocol": "TCP"},
			},
			want: "http:80/TCP,https:443/TCP",
		},
		{
			name: "port without name",
			items: []any{
				map[string]any{"port": 6379, "protocol": "TCP"},
			},
			want: "6379/TCP",
		},
		{
			name:  "empty",
			items: []any{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatPorts(tt.items, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := result.(string)
			if got != tt.want {
				t.Errorf("formatPorts() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeAllocatableMetrics(t *testing.T) {
	items := []any{
		map[string]any{
			"metadata": map[string]any{"name": "node-1"},
			"status": map[string]any{
				"allocatable": map[string]any{
					"cpu":    "4",
					"memory": "16Gi",
				},
			},
		},
	}

	result, err := nodeAllocatableMetrics(items, map[string]any{
		"name_path":        "metadata.name",
		"allocatable_path": "status.allocatable",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metrics := result.([]Metric)
	if len(metrics) != 2 {
		t.Fatalf("got %d metrics, want 2", len(metrics))
	}

	// Check CPU metric
	cpuMetric := metrics[0]
	if cpuMetric.Name != "allocatable_cpu" {
		t.Errorf("first metric name = %q, want allocatable_cpu", cpuMetric.Name)
	}
	if cpuMetric.Value != 4.0 {
		t.Errorf("cpu value = %f, want 4.0", cpuMetric.Value)
	}
	if cpuMetric.Labels["node"] != "node-1" {
		t.Errorf("cpu node label = %q, want node-1", cpuMetric.Labels["node"])
	}

	// Check memory metric
	memMetric := metrics[1]
	if memMetric.Name != "allocatable_memory" {
		t.Errorf("second metric name = %q, want allocatable_memory", memMetric.Name)
	}
	expectedMem := 16.0 * 1024 * 1024 * 1024
	if memMetric.Value != expectedMem {
		t.Errorf("memory value = %f, want %f", memMetric.Value, expectedMem)
	}
}

func TestParseKubectlTop(t *testing.T) {
	text := "api-server-abc   125m   180Mi\npostgres-0       450m   950Mi\n"

	result, err := parseKubectlTop([]any{text}, map[string]any{
		"format": "name cpu_millicores mem_bytes",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metrics := result.([]Metric)
	// 2 pods × 2 metrics each = 4
	if len(metrics) != 4 {
		t.Fatalf("got %d metrics, want 4", len(metrics))
	}

	// First pod CPU
	if metrics[0].Name != "cpu_usage" || metrics[0].Value != 125 {
		t.Errorf("metrics[0] = {%s, %f}, want {cpu_usage, 125}", metrics[0].Name, metrics[0].Value)
	}
	if metrics[0].Labels["pod"] != "api-server-abc" {
		t.Errorf("pod label = %q, want api-server-abc", metrics[0].Labels["pod"])
	}

	// First pod memory
	if metrics[1].Name != "memory_usage" || metrics[1].Value != 180 {
		t.Errorf("metrics[1] = {%s, %f}, want {memory_usage, 180}", metrics[1].Name, metrics[1].Value)
	}
}

func TestParseKubectlTop_Empty(t *testing.T) {
	result, err := parseKubectlTop([]any{""}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)
	if len(metrics) != 0 {
		t.Errorf("expected empty metrics, got %d", len(metrics))
	}
}

func TestCallTransform_Unknown(t *testing.T) {
	_, err := CallTransform("nonexistent", nil, nil)
	if err == nil {
		t.Error("expected error for unknown transform")
	}
}

func TestListTransforms(t *testing.T) {
	names := ListTransforms()
	if len(names) < 31 {
		t.Errorf("expected at least 31 transforms, got %d", len(names))
	}

	required := map[string]bool{
		"health_from_conditions":              true,
		"health_from_pod_status":              true,
		"deployment_status":                   true,
		"node_status":                         true,
		"format_ports":                        true,
		"node_allocatable_metrics":            true,
		"parse_kubectl_top":                   true,
		"tech_from_image":                     true,
		"dd_integrations_to_topology":         true,
		"dd_services_to_topology":             true,
		"dd_hosts_to_topology":                true,
		"dd_monitors_to_alerts":               true,
		"dd_host_metrics":                     true,
		"dd_dependencies_to_topology":         true,
		"kafka_clusters_to_topology":          true,
		"kafka_brokers_to_topology":           true,
		"kafka_topics_to_topology":            true,
		"kafka_consumer_groups_to_topology":   true,
		"kafka_consumer_groups_to_health":     true,
		"kafka_connectors_to_topology":        true,
		"kafka_connectors_to_health":          true,
		"pg_database_info_to_topology":        true,
		"pg_database_info_to_metrics":         true,
		"pg_replication_slots_to_topology":    true,
		"pg_replication_slots_to_health":      true,
		"pg_replication_stats_to_topology":    true,
		"pg_replication_stats_to_health":      true,
		"pg_connections_to_topology":          true,
		"pg_connections_to_metrics":           true,
		"pg_tables_to_topology":              true,
		"pg_tables_to_metrics":               true,
	}

	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}
	for name := range required {
		if !found[name] {
			t.Errorf("missing required transform: %q", name)
		}
	}
}

func TestParseCPU(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"4", 4.0},
		{"500m", 0.5},
		{"250m", 0.25},
		{"1000m", 1.0},
		{"0", 0.0},
	}
	for _, tt := range tests {
		got := parseCPU(tt.input)
		if got != tt.want {
			t.Errorf("parseCPU(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"1Ki", 1024},
		{"1Mi", 1024 * 1024},
		{"1Gi", 1024 * 1024 * 1024},
		{"16Gi", 16 * 1024 * 1024 * 1024},
		{"256", 256},
	}
	for _, tt := range tests {
		got := parseMemory(tt.input)
		if got != tt.want {
			t.Errorf("parseMemory(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestArgString(t *testing.T) {
	args := map[string]any{
		"name":  "test",
		"count": 42,
	}

	if got := argString(args, "name"); got != "test" {
		t.Errorf("argString(name) = %q, want test", got)
	}
	if got := argString(args, "missing"); got != "" {
		t.Errorf("argString(missing) = %q, want empty", got)
	}
	if got := argString(nil, "name"); got != "" {
		t.Errorf("argString(nil, name) = %q, want empty", got)
	}
	if got := argString(args, "count"); got != "" {
		t.Errorf("argString(count) = %q, want empty for non-string", got)
	}
}
