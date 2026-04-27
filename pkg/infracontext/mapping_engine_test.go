package infracontext

import (
	"encoding/json"
	"os"
	"testing"
)

func loadFixture(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse fixture %s: %v", path, err)
	}
	return result
}

func TestMappingEngine_MapTopology_Pods(t *testing.T) {
	engine := NewMappingEngine(nil)
	rawData := loadFixture(t, "testdata/pods.json")

	section := &InfraMappingSection{
		SourcePath: "items",
		Each: map[string]any{
			"kind":      map[string]any{"value": "Pod"},
			"name":      map[string]any{"path": "metadata.name"},
			"namespace": map[string]any{"path": "metadata.namespace"},
			"status":    map[string]any{"path": "status.phase"},
			"labels":    map[string]any{"path": "metadata.labels"},
			"containers": map[string]any{
				"source_path": "spec.containers",
				"each": map[string]any{
					"name":        map[string]any{"path": "name"},
					"image":       map[string]any{"path": "image"},
					"cpu_request": map[string]any{"path": "resources.requests.cpu"},
					"cpu_limit":   map[string]any{"path": "resources.limits.cpu"},
					"mem_request": map[string]any{"path": "resources.requests.memory"},
					"mem_limit":   map[string]any{"path": "resources.limits.memory"},
				},
				"ready_from": "status.containerStatuses",
			},
		},
	}

	nodes, err := engine.MapTopology(rawData, section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 4 {
		t.Fatalf("expected 4 pods, got %d", len(nodes))
	}

	// Check first pod
	pod := nodes[0]
	if pod.Kind != "Pod" {
		t.Errorf("kind = %q, want Pod", pod.Kind)
	}
	if pod.Name != "api-server-7b8f9c6d4-x2k9p" {
		t.Errorf("name = %q, want api-server-7b8f9c6d4-x2k9p", pod.Name)
	}
	if pod.Namespace != "default" {
		t.Errorf("namespace = %q, want default", pod.Namespace)
	}
	if pod.Status != "Running" {
		t.Errorf("status = %q, want Running", pod.Status)
	}
	if pod.Labels["app"] != "api-server" {
		t.Errorf("label app = %q, want api-server", pod.Labels["app"])
	}

	// Check containers
	if len(pod.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(pod.Containers))
	}
	c := pod.Containers[0]
	if c.Name != "api" {
		t.Errorf("container name = %q, want api", c.Name)
	}
	if c.Image != "myregistry/api-server:v1.2.0" {
		t.Errorf("container image = %q", c.Image)
	}
	if c.CPURequest != "250m" {
		t.Errorf("cpu_request = %q, want 250m", c.CPURequest)
	}
	if !c.Ready {
		t.Error("container should be ready")
	}

	// Check failed pod
	failedPod := nodes[3]
	if failedPod.Status != "Failed" {
		t.Errorf("failed pod status = %q, want Failed", failedPod.Status)
	}
}

func TestMappingEngine_MapTopology_Services(t *testing.T) {
	engine := NewMappingEngine(nil)
	rawData := loadFixture(t, "testdata/services.json")

	section := &InfraMappingSection{
		SourcePath: "items",
		Each: map[string]any{
			"kind":      map[string]any{"value": "Service"},
			"name":      map[string]any{"path": "metadata.name"},
			"namespace": map[string]any{"path": "metadata.namespace"},
			"status":    map[string]any{"value": "Active"},
			"labels":    map[string]any{"path": "metadata.labels"},
			"metadata": map[string]any{
				"cluster_ip": map[string]any{"path": "spec.clusterIP"},
				"type":       map[string]any{"path": "spec.type"},
			},
		},
	}

	nodes, err := engine.MapTopology(rawData, section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 4 {
		t.Fatalf("expected 4 services, got %d", len(nodes))
	}

	svc := nodes[0]
	if svc.Kind != "Service" {
		t.Errorf("kind = %q, want Service", svc.Kind)
	}
	if svc.Status != "Active" {
		t.Errorf("status = %q, want Active", svc.Status)
	}
	if svc.Metadata["cluster_ip"] != "10.96.100.1" {
		t.Errorf("cluster_ip = %q, want 10.96.100.1", svc.Metadata["cluster_ip"])
	}
	if svc.Metadata["type"] != "ClusterIP" {
		t.Errorf("type = %q, want ClusterIP", svc.Metadata["type"])
	}
}

func TestMappingEngine_MapTopology_Deployments(t *testing.T) {
	engine := NewMappingEngine(nil)
	rawData := loadFixture(t, "testdata/deployments.json")

	section := &InfraMappingSection{
		SourcePath: "items",
		Each: map[string]any{
			"kind":      map[string]any{"value": "Deployment"},
			"name":      map[string]any{"path": "metadata.name"},
			"namespace": map[string]any{"path": "metadata.namespace"},
			"status": map[string]any{
				"transform": "deployment_status",
				"args":      map[string]any{"conditions_path": "status.conditions"},
			},
			"labels": map[string]any{"path": "metadata.labels"},
			"replicas": map[string]any{
				"desired":   map[string]any{"path": "spec.replicas"},
				"ready":     map[string]any{"path": "status.readyReplicas", "default": 0},
				"available": map[string]any{"path": "status.availableReplicas", "default": 0},
			},
			"containers": map[string]any{
				"source_path": "spec.template.spec.containers",
				"each": map[string]any{
					"name":  map[string]any{"path": "name"},
					"image": map[string]any{"path": "image"},
				},
			},
		},
	}

	nodes, err := engine.MapTopology(rawData, section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 deployments, got %d", len(nodes))
	}

	// api-server deployment
	deploy := nodes[0]
	if deploy.Status != "Available" {
		t.Errorf("api-server status = %q, want Available", deploy.Status)
	}
	if deploy.Replicas == nil {
		t.Fatal("expected replicas to be set")
	}
	if deploy.Replicas.Desired != 3 {
		t.Errorf("desired = %d, want 3", deploy.Replicas.Desired)
	}
	if deploy.Replicas.Ready != 3 {
		t.Errorf("ready = %d, want 3", deploy.Replicas.Ready)
	}

	// redis-cache deployment (unavailable)
	redis := nodes[1]
	if redis.Status != "Unavailable" {
		t.Errorf("redis-cache status = %q, want Unavailable", redis.Status)
	}
	if redis.Replicas.Ready != 1 {
		t.Errorf("redis ready = %d, want 1", redis.Replicas.Ready)
	}
}

func TestMappingEngine_MapTopology_Nodes(t *testing.T) {
	engine := NewMappingEngine(nil)
	rawData := loadFixture(t, "testdata/nodes.json")

	section := &InfraMappingSection{
		SourcePath: "items",
		Each: map[string]any{
			"kind":   map[string]any{"value": "Node"},
			"name":   map[string]any{"path": "metadata.name"},
			"status": map[string]any{"transform": "node_status", "args": map[string]any{"conditions_path": "status.conditions"}},
			"labels": map[string]any{"path": "metadata.labels"},
			"metadata": map[string]any{
				"os":      map[string]any{"path": "status.nodeInfo.osImage"},
				"kubelet": map[string]any{"path": "status.nodeInfo.kubeletVersion"},
			},
		},
	}

	nodes, err := engine.MapTopology(rawData, section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	if nodes[0].Status != "Ready" {
		t.Errorf("node-1 status = %q, want Ready", nodes[0].Status)
	}
	if nodes[2].Status != "NotReady" {
		t.Errorf("node-3 status = %q, want NotReady", nodes[2].Status)
	}
	if nodes[0].Metadata["os"] != "Ubuntu 22.04.3 LTS" {
		t.Errorf("node-1 os = %q", nodes[0].Metadata["os"])
	}
}

func TestMappingEngine_MapHealth_Pods(t *testing.T) {
	engine := NewMappingEngine(nil)
	rawData := loadFixture(t, "testdata/pods.json")

	section := &InfraMappingSection{
		SourcePath: "items",
		Transform:  "health_from_pod_status",
		Args: map[string]any{
			"name_path":       "metadata.name",
			"phase_path":      "status.phase",
			"conditions_path": "status.conditions",
			"ready_type":      "Ready",
		},
	}

	checks, err := engine.MapHealth(rawData, section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(checks) != 4 {
		t.Fatalf("expected 4 health checks, got %d", len(checks))
	}

	// Running and ready
	if checks[0].Status != HealthStatusHealthy {
		t.Errorf("pod 0 status = %q, want healthy", checks[0].Status)
	}
	// Running but not ready
	if checks[2].Status != HealthStatusDegraded {
		t.Errorf("pod 2 status = %q, want degraded", checks[2].Status)
	}
	// Failed
	if checks[3].Status != HealthStatusUnhealthy {
		t.Errorf("pod 3 status = %q, want unhealthy", checks[3].Status)
	}
}

func TestMappingEngine_MapHealth_Deployments(t *testing.T) {
	engine := NewMappingEngine(nil)
	rawData := loadFixture(t, "testdata/deployments.json")

	section := &InfraMappingSection{
		SourcePath: "items",
		Transform:  "health_from_conditions",
		Args: map[string]any{
			"name_path":       "metadata.name",
			"kind":            "Deployment",
			"conditions_path": "status.conditions",
			"ready_type":      "Available",
		},
	}

	checks, err := engine.MapHealth(rawData, section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(checks) != 3 {
		t.Fatalf("expected 3 health checks, got %d", len(checks))
	}

	if checks[0].Component != "api-server" || checks[0].Status != HealthStatusHealthy {
		t.Errorf("api-server: component=%q status=%q", checks[0].Component, checks[0].Status)
	}
	if checks[1].Component != "redis-cache" || checks[1].Status != HealthStatusUnhealthy {
		t.Errorf("redis-cache: component=%q status=%q", checks[1].Component, checks[1].Status)
	}
}

func TestMappingEngine_MapMetrics_Nodes(t *testing.T) {
	engine := NewMappingEngine(nil)
	rawData := loadFixture(t, "testdata/nodes.json")

	section := &InfraMappingSection{
		SourcePath: "items",
		Transform:  "node_allocatable_metrics",
		Args: map[string]any{
			"name_path":        "metadata.name",
			"allocatable_path": "status.allocatable",
		},
	}

	metrics, err := engine.MapMetrics(rawData, section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 nodes × 2 metrics each = 6
	if len(metrics) != 6 {
		t.Fatalf("expected 6 metrics, got %d", len(metrics))
	}

	// First node CPU
	if metrics[0].Name != "allocatable_cpu" || metrics[0].Value != 4.0 {
		t.Errorf("metrics[0] = {%s, %f}", metrics[0].Name, metrics[0].Value)
	}
}

func TestMappingEngine_MapTextMetrics(t *testing.T) {
	engine := NewMappingEngine(nil)
	rawText, err := os.ReadFile("testdata/top_pods.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	section := &InfraMappingSection{
		Transform: "parse_kubectl_top",
		Args: map[string]any{
			"format": "name cpu_millicores mem_bytes",
		},
	}

	metrics, err := engine.MapTextMetrics(string(rawText), section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4 pods × 2 metrics each = 8
	if len(metrics) != 8 {
		t.Fatalf("expected 8 metrics, got %d", len(metrics))
	}
}

func TestMappingEngine_MapTopology_NilSection(t *testing.T) {
	engine := NewMappingEngine(nil)
	nodes, err := engine.MapTopology(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodes != nil {
		t.Error("expected nil for nil section")
	}
}

func TestMappingEngine_MapHealth_NilSection(t *testing.T) {
	engine := NewMappingEngine(nil)
	checks, err := engine.MapHealth(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checks != nil {
		t.Error("expected nil for nil section")
	}
}

func TestMappingEngine_MapMetrics_NilSection(t *testing.T) {
	engine := NewMappingEngine(nil)
	metrics, err := engine.MapMetrics(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics != nil {
		t.Error("expected nil for nil section")
	}
}
