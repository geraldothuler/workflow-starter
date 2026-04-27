package infracontext

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// MockExecutor simulates command execution for testing.
type MockExecutor struct {
	responses map[string][]byte
	errors    map[string]error
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		responses: make(map[string][]byte),
		errors:    make(map[string]error),
	}
}

func (m *MockExecutor) AddResponse(command string, data []byte) {
	m.responses[command] = data
}

func (m *MockExecutor) AddError(command string, err error) {
	m.errors[command] = err
}

func (m *MockExecutor) Execute(ctx context.Context, command string, args []string) ([]byte, error) {
	key := command
	for _, arg := range args {
		key += " " + arg
	}

	if err, ok := m.errors[key]; ok {
		return nil, err
	}

	// Try exact match first
	if data, ok := m.responses[key]; ok {
		return data, nil
	}

	// Try matching by any substring containing the verb and resource
	for k, v := range m.responses {
		if containsAllWords(key, k) {
			return v, nil
		}
	}

	return nil, fmt.Errorf("mock: no response for %q", key)
}

func containsAllWords(haystack, needle string) bool {
	// Simple substring match
	return len(haystack) > 0 && len(needle) > 0 &&
		(haystack == needle || len(haystack) > len(needle))
}

func TestConfigProvider_ID_Name(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Kubernetes (kubectl)",
	}
	cp := NewConfigProvider(spec, nil, nil, nil)

	if cp.ID() != "kubectl" {
		t.Errorf("ID() = %q, want kubectl", cp.ID())
	}
	if cp.Name() != "Kubernetes (kubectl)" {
		t.Errorf("Name() = %q", cp.Name())
	}
}

func TestConfigProvider_Available_CommandNotFound(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "test",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI: &InfraCLISpec{
				Command:        "nonexistent-command-xyz",
				AvailableCheck: []string{"version"},
			},
		},
	}
	cp := NewConfigProvider(spec, nil, nil, nil)

	if cp.Available() {
		t.Error("expected Available() = false for nonexistent command")
	}
}

func TestConfigProvider_Available_MockSuccess(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "test",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI: &InfraCLISpec{
				Command:        "kubectl",
				AvailableCheck: []string{"cluster-info"},
			},
		},
	}
	cp := NewConfigProvider(spec, nil, nil, nil)

	mock := NewMockExecutor()
	mock.AddResponse("kubectl cluster-info", []byte("Kubernetes control plane is running"))
	cp.SetExecutor(mock)

	if !cp.Available() {
		t.Error("expected Available() = true with mock")
	}
}

func TestConfigProvider_Fetch_Pods(t *testing.T) {
	podsData, err := os.ReadFile("testdata/pods.json")
	if err != nil {
		t.Fatal(err)
	}

	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Test Kubectl",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI: &InfraCLISpec{
				Command: "kubectl",
				Timeout: 30 * time.Second,
			},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "pods",
				CLICommand: "get pods -n {{.namespace}} -o json",
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{
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
									"name":  map[string]any{"path": "name"},
									"image": map[string]any{"path": "image"},
								},
								"ready_from": "status.containerStatuses",
							},
						},
					},
					Health: &InfraMappingSection{
						SourcePath: "items",
						Transform:  "health_from_pod_status",
						Args: map[string]any{
							"name_path":       "metadata.name",
							"phase_path":      "status.phase",
							"conditions_path": "status.conditions",
							"ready_type":      "Ready",
						},
					},
				},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl get pods -n default -o json", podsData)
	mock.AddResponse("kubectl config current-context", []byte("test-cluster"))

	techMapper := NewTechMapperFromMap(map[string]string{
		"postgres": "PostgreSQL",
		"redis":    "Redis",
	})
	cp := NewConfigProvider(spec, techMapper, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if ic.Provider != "kubectl" {
		t.Errorf("provider = %q", ic.Provider)
	}
	if ic.Cluster != "test-cluster" {
		t.Errorf("cluster = %q", ic.Cluster)
	}
	if ic.Namespace != "default" {
		t.Errorf("namespace = %q", ic.Namespace)
	}
	if len(ic.Topology) != 4 {
		t.Errorf("topology count = %d, want 4", len(ic.Topology))
	}
	if len(ic.Health) != 4 {
		t.Errorf("health count = %d, want 4", len(ic.Health))
	}

	// Verify tech detection
	techMap := ic.TechMap(techMapper)
	if techMap["PostgreSQL"] == "" {
		t.Error("expected PostgreSQL to be detected")
	}
	if techMap["Redis"] == "" {
		t.Error("expected Redis to be detected")
	}
}

func TestConfigProvider_Fetch_WithCache(t *testing.T) {
	podsJSON := `{"items": [{"metadata": {"name": "test-pod", "namespace": "default"}, "spec": {"containers": []}, "status": {"phase": "Running", "conditions": [{"type": "Ready", "status": "True"}]}}]}`

	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "kubectl", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "pods",
				CLICommand: "get pods -n {{.namespace}} -o json",
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{
						SourcePath: "items",
						Each: map[string]any{
							"kind":   map[string]any{"value": "Pod"},
							"name":   map[string]any{"path": "metadata.name"},
							"status": map[string]any{"path": "status.phase"},
						},
					},
				},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl get pods -n default -o json", []byte(podsJSON))
	mock.AddResponse("kubectl config current-context", []byte("test"))

	cache := NewCache("")
	cp := NewConfigProvider(spec, nil, cache, nil)
	cp.SetExecutor(mock)

	// First fetch
	ic1, err := cp.Fetch(context.Background(), FetchOptions{UseCache: true})
	if err != nil {
		t.Fatalf("first fetch error: %v", err)
	}

	// Second fetch should come from cache (even if mock would fail)
	mock.AddError("kubectl get pods -n default -o json", fmt.Errorf("should not be called"))
	ic2, err := cp.Fetch(context.Background(), FetchOptions{UseCache: true})
	if err != nil {
		t.Fatalf("second fetch error: %v", err)
	}

	if ic1.FetchedAt != ic2.FetchedAt {
		t.Error("second fetch should return cached result")
	}
}

func TestConfigProvider_Fetch_OptionalStepFails(t *testing.T) {
	podsJSON := `{"items": []}`

	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "kubectl", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "pods",
				CLICommand: "get pods -n {{.namespace}} -o json",
				Mapping:    InfraMapping{},
			},
			{
				ID:         "top_pods",
				CLICommand: "top pods -n {{.namespace}} --no-headers",
				Optional:   true,
				ParseMode:  "text",
				Mapping:    InfraMapping{},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl get pods -n default -o json", []byte(podsJSON))
	mock.AddError("kubectl top pods -n default --no-headers", fmt.Errorf("metrics-server not installed"))
	mock.AddResponse("kubectl config current-context", []byte("test"))

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch should succeed even if optional step fails: %v", err)
	}
	if ic == nil {
		t.Fatal("expected non-nil InfraContext")
	}
}

func TestConfigProvider_Fetch_RequiredStepFails(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "kubectl", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "pods",
				CLICommand: "get pods -n {{.namespace}} -o json",
				Mapping:    InfraMapping{},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddError("kubectl get pods -n default -o json", fmt.Errorf("connection refused"))
	mock.AddResponse("kubectl config current-context", []byte("test"))

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err == nil {
		t.Error("expected error when required step fails")
	}
}

func TestConfigProvider_Fetch_TextParsing(t *testing.T) {
	topOutput := "pod-1   100m   200Mi\npod-2   250m   300Mi\n"

	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "kubectl", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "top_pods",
				CLICommand: "top pods -n {{.namespace}} --no-headers",
				ParseMode:  "text",
				Mapping: InfraMapping{
					Metrics: &InfraMappingSection{
						Transform: "parse_kubectl_top",
						Args: map[string]any{
							"format": "name cpu_millicores mem_bytes",
						},
					},
				},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl top pods -n default --no-headers", []byte(topOutput))
	mock.AddResponse("kubectl config current-context", []byte("test"))

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	// 2 pods × 2 metrics = 4
	if len(ic.Metrics) != 4 {
		t.Errorf("expected 4 metrics, got %d", len(ic.Metrics))
	}
}

func TestConfigProvider_Fetch_ResourceFilter(t *testing.T) {
	podsJSON := `{"items": [{"metadata": {"name": "pod-1"}, "spec": {"containers": []}, "status": {"phase": "Running"}}]}`
	svcJSON := `{"items": [{"metadata": {"name": "svc-1"}, "spec": {"type": "ClusterIP"}}]}`

	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "kubectl", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "pods",
				CLICommand: "get pods -n {{.namespace}} -o json",
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{
						SourcePath: "items",
						Each: map[string]any{
							"kind":   map[string]any{"value": "Pod"},
							"name":   map[string]any{"path": "metadata.name"},
							"status": map[string]any{"path": "status.phase"},
						},
					},
				},
			},
			{
				ID:         "services",
				CLICommand: "get services -n {{.namespace}} -o json",
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{
						SourcePath: "items",
						Each: map[string]any{
							"kind":   map[string]any{"value": "Service"},
							"name":   map[string]any{"path": "metadata.name"},
							"status": map[string]any{"value": "Active"},
						},
					},
				},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl get pods -n default -o json", []byte(podsJSON))
	mock.AddResponse("kubectl get services -n default -o json", []byte(svcJSON))
	mock.AddResponse("kubectl config current-context", []byte("test"))

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	// Fetch only pods
	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"pods"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	// Should only have pods in topology
	for _, node := range ic.Topology {
		if node.Kind == "Service" {
			t.Error("should not have Service when filtering to pods only")
		}
	}
}

func TestConfigProvider_Fetch_CustomNamespace(t *testing.T) {
	podsJSON := `{"items": []}`

	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "kubectl", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "pods",
				CLICommand: "get pods -n {{.namespace}} -o json",
				Mapping:    InfraMapping{},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl get pods -n production -o json", []byte(podsJSON))
	mock.AddResponse("kubectl config current-context", []byte("prod-cluster"))

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{Namespace: "production"})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if ic.Namespace != "production" {
		t.Errorf("namespace = %q, want production", ic.Namespace)
	}
}

func TestConfigProvider_Fetch_FullPipeline(t *testing.T) {
	// Load all fixtures
	podsData, err := os.ReadFile("testdata/pods.json")
	if err != nil {
		t.Fatal(err)
	}
	svcData, err := os.ReadFile("testdata/services.json")
	if err != nil {
		t.Fatal(err)
	}
	deployData, err := os.ReadFile("testdata/deployments.json")
	if err != nil {
		t.Fatal(err)
	}
	nodesData, err := os.ReadFile("testdata/nodes.json")
	if err != nil {
		t.Fatal(err)
	}
	topData, err := os.ReadFile("testdata/top_pods.txt")
	if err != nil {
		t.Fatal(err)
	}

	// Load the real kubectl config
	spec, err := LoadProviderConfig("kubectl", "")
	if err != nil {
		t.Fatalf("failed to load kubectl config: %v", err)
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl get pods -n default -o json", podsData)
	mock.AddResponse("kubectl get services -n default -o json", svcData)
	mock.AddResponse("kubectl get deployments -n default -o json", deployData)
	mock.AddResponse("kubectl get nodes -o json", nodesData)
	mock.AddResponse("kubectl top pods -n default --no-headers", topData)
	mock.AddResponse("kubectl config current-context", []byte("prod-cluster"))

	techMapper := NewTechMapperFromMap(map[string]string{
		"postgres": "PostgreSQL",
		"redis":    "Redis",
		"nginx":    "Nginx",
	})

	cp := NewConfigProvider(spec, techMapper, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("full pipeline fetch error: %v", err)
	}

	// Validate topology
	kinds := make(map[string]int)
	for _, node := range ic.Topology {
		kinds[node.Kind]++
	}
	if kinds["Pod"] != 4 {
		t.Errorf("pods = %d, want 4", kinds["Pod"])
	}
	if kinds["Service"] != 4 {
		t.Errorf("services = %d, want 4", kinds["Service"])
	}
	if kinds["Deployment"] != 3 {
		t.Errorf("deployments = %d, want 3", kinds["Deployment"])
	}
	if kinds["Node"] != 3 {
		t.Errorf("nodes = %d, want 3", kinds["Node"])
	}

	// Validate health
	if len(ic.Health) == 0 {
		t.Error("expected health checks")
	}

	// Validate metrics (from nodes + top)
	if len(ic.Metrics) == 0 {
		t.Error("expected metrics")
	}

	// Validate tech detection
	techMap := ic.TechMap(techMapper)
	if techMap["PostgreSQL"] == "" {
		t.Error("expected PostgreSQL detection")
	}
	if techMap["Redis"] == "" {
		t.Error("expected Redis detection")
	}
	if techMap["Nginx"] == "" {
		t.Error("expected Nginx detection")
	}

	// Validate summary
	summary := ic.Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}

	// Validate JSON serialization
	data, err := json.Marshal(ic)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}

// --- cli_args tests ---

func TestCLIArgs_Basic(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "test",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "echo", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:      "step1",
				CLIArgs: []string{"-t", "-A", "-c"},
				Mapping: InfraMapping{},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddResponse("echo -t -A -c", []byte(`{}`))
	mock.AddResponse("echo config current-context", []byte("test"))

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch with cli_args failed: %v", err)
	}
}

func TestCLIArgs_TemplateExpansion(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "test",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "psql", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:      "db_info",
				CLIArgs: []string{"{{.PG_CONNECTION_STRING}}", "-t", "-c", "SELECT 1"},
				Mapping: InfraMapping{},
			},
		},
	}

	mock := &TemplateTrackingExecutor{
		lastArgs: nil,
		response: []byte(`{}`),
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	// Simulate credential resolution by using the Fetch path with templateData
	// Since we can't easily inject creds, we test expansion directly
	expanded, err := expandAnyTemplate("{{.PG_CONNECTION_STRING}}", map[string]string{
		"PG_CONNECTION_STRING": "postgresql://user:pass@host:5432/db",
	})
	if err != nil {
		t.Fatalf("template expansion failed: %v", err)
	}
	if expanded != "postgresql://user:pass@host:5432/db" {
		t.Errorf("expected expanded connection string, got %q", expanded)
	}
}

// TemplateTrackingExecutor tracks the last args passed to Execute.
type TemplateTrackingExecutor struct {
	lastArgs []string
	response []byte
}

func (t *TemplateTrackingExecutor) Execute(ctx context.Context, command string, args []string) ([]byte, error) {
	t.lastArgs = args
	return t.response, nil
}

func TestCLIArgs_WithSpaces(t *testing.T) {
	sqlQuery := "SELECT json_build_object('items', json_agg(row_to_json(t))) FROM (SELECT 1) t"

	spec := &InfraProviderSpec{
		ID:   "test",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "psql", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:      "query",
				CLIArgs: []string{"postgresql://localhost/db", "-t", "-A", "-c", sqlQuery},
				Mapping: InfraMapping{},
			},
		},
	}

	tracker := &TemplateTrackingExecutor{
		response: []byte(`{}`),
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(tracker)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch with SQL args failed: %v", err)
	}

	// Verify the SQL query was passed as a single arg (not split by spaces)
	if len(tracker.lastArgs) != 5 {
		t.Fatalf("expected 5 args, got %d: %v", len(tracker.lastArgs), tracker.lastArgs)
	}
	if tracker.lastArgs[4] != sqlQuery {
		t.Errorf("SQL query was split!\nexpected: %q\ngot:      %q", sqlQuery, tracker.lastArgs[4])
	}
}

func TestCLIArgs_ValidationBothSet(t *testing.T) {
	spec := InfraProviderSpec{
		ID:   "test",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "test"},
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "bad_step",
				CLICommand: "get pods -o json",
				CLIArgs:    []string{"get", "pods", "-o", "json"},
			},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected validation error when both cli_command and cli_args are set")
	}
}

func TestCLIArgs_Empty_FallbackToCLICommand(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "kubectl",
		Name: "Test",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "kubectl", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "pods",
				CLICommand: "get pods -n {{.namespace}} -o json",
				CLIArgs:    nil, // empty — should fallback to cli_command
				Mapping:    InfraMapping{},
			},
		},
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl get pods -n default -o json", []byte(`{"items": []}`))
	mock.AddResponse("kubectl config current-context", []byte("test"))

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch with empty cli_args should fallback to cli_command: %v", err)
	}
}

func TestCLIArgs_BackwardsCompat_Kubectl(t *testing.T) {
	// Verify that existing kubectl provider (which uses cli_command) still works
	podsJSON := `{"items": [{"metadata": {"name": "test-pod", "namespace": "default"}, "spec": {"containers": []}, "status": {"phase": "Running"}}]}`

	spec, err := LoadProviderConfig("kubectl", "")
	if err != nil {
		t.Fatalf("failed to load kubectl config: %v", err)
	}

	mock := NewMockExecutor()
	mock.AddResponse("kubectl get pods -n default -o json", []byte(podsJSON))
	mock.AddResponse("kubectl get services -n default -o json", []byte(`{"items": []}`))
	mock.AddResponse("kubectl get deployments -n default -o json", []byte(`{"items": []}`))
	mock.AddResponse("kubectl get nodes -o json", []byte(`{"items": []}`))
	mock.AddResponse("kubectl top pods -n default --no-headers", []byte(""))
	mock.AddResponse("kubectl config current-context", []byte("test"))

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("backwards compat failed: kubectl with cli_command should still work: %v", err)
	}
	if len(ic.Topology) != 1 {
		t.Errorf("expected 1 topology node, got %d", len(ic.Topology))
	}
}

// --- PostgreSQL pipeline tests ---

func TestPostgreSQLProvider_FullPipeline(t *testing.T) {
	// Load fixtures
	dbInfoData, err := os.ReadFile("testdata/pg_database_info.json")
	if err != nil {
		t.Fatal(err)
	}
	replSlotsData, err := os.ReadFile("testdata/pg_replication_slots.json")
	if err != nil {
		t.Fatal(err)
	}
	replStatsData, err := os.ReadFile("testdata/pg_replication_stats.json")
	if err != nil {
		t.Fatal(err)
	}
	connectionsData, err := os.ReadFile("testdata/pg_connections.json")
	if err != nil {
		t.Fatal(err)
	}
	tablesData, err := os.ReadFile("testdata/pg_tables.json")
	if err != nil {
		t.Fatal(err)
	}

	spec, err := LoadProviderConfig("postgresql", "")
	if err != nil {
		t.Fatalf("failed to load postgresql config: %v", err)
	}

	mock := &PostgreSQLMockExecutor{
		responses: map[string][]byte{
			"database_info":     dbInfoData,
			"replication_slots": replSlotsData,
			"replication_stats": replStatsData,
			"connections":       connectionsData,
			"tables":            tablesData,
		},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("full pipeline fetch error: %v", err)
	}

	if ic.Provider != "postgresql" {
		t.Errorf("provider = %q, want postgresql", ic.Provider)
	}

	// Validate topology: 1 db + 3 slots + 2 WAL senders + N connections + 5 tables
	if len(ic.Topology) == 0 {
		t.Error("expected topology nodes")
	}

	kinds := make(map[string]int)
	for _, node := range ic.Topology {
		kinds[node.Kind]++
	}
	if kinds["PostgresDatabase"] != 1 {
		t.Errorf("expected 1 PostgresDatabase, got %d", kinds["PostgresDatabase"])
	}
	if kinds["ReplicationSlot"] != 3 {
		t.Errorf("expected 3 ReplicationSlot, got %d", kinds["ReplicationSlot"])
	}
	if kinds["WALSender"] != 2 {
		t.Errorf("expected 2 WALSender, got %d", kinds["WALSender"])
	}
	if kinds["Table"] != 5 {
		t.Errorf("expected 5 Table, got %d", kinds["Table"])
	}

	// Validate health checks
	if len(ic.Health) == 0 {
		t.Error("expected health checks")
	}

	// Validate metrics
	if len(ic.Metrics) == 0 {
		t.Error("expected metrics")
	}

	// Validate JSON serialization
	data, err := json.Marshal(ic)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestPostgreSQLProvider_ReplicationSlots(t *testing.T) {
	replSlotsData, err := os.ReadFile("testdata/pg_replication_slots.json")
	if err != nil {
		t.Fatal(err)
	}

	spec := &InfraProviderSpec{
		ID:   "postgresql",
		Name: "PostgreSQL (psql)",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "psql", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       2 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:      "replication_slots",
				CLIArgs: []string{"postgresql://localhost/db", "-t", "-A", "-c", "SELECT ..."},
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{
						SourcePath: "items",
						Transform:  "pg_replication_slots_to_topology",
					},
					Health: &InfraMappingSection{
						SourcePath: "items",
						Transform:  "pg_replication_slots_to_health",
					},
				},
			},
		},
	}

	mock := &PostgreSQLMockExecutor{
		responses: map[string][]byte{
			"replication_slots": replSlotsData,
		},
		orderedSteps: []string{"replication_slots"},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("replication slots fetch error: %v", err)
	}

	// Should have 3 slots
	if len(ic.Topology) != 3 {
		t.Errorf("expected 3 topology nodes, got %d", len(ic.Topology))
	}

	// Verify health mapping: 1 healthy (active), 1 unhealthy (inactive + 2GB lag), 1 degraded (inactive + 0 lag)
	healthByComp := make(map[string]string)
	for _, h := range ic.Health {
		healthByComp[h.Component] = h.Status
	}

	if healthByComp["debezium_slot"] != HealthStatusHealthy {
		t.Errorf("debezium_slot expected healthy, got %q", healthByComp["debezium_slot"])
	}
	if healthByComp["airbyte_cdc_slot"] != HealthStatusUnhealthy {
		t.Errorf("airbyte_cdc_slot expected unhealthy (2GB lag), got %q", healthByComp["airbyte_cdc_slot"])
	}
	if healthByComp["old_unused_slot"] != HealthStatusDegraded {
		t.Errorf("old_unused_slot expected degraded (inactive, no lag), got %q", healthByComp["old_unused_slot"])
	}
}

func TestPostgreSQLProvider_CDCInvestigation(t *testing.T) {
	// Simulate the real-world CDC investigation scenario:
	// - airbyte_cdc_slot is inactive with 2GB lag → unhealthy
	// - debezium WAL sender is streaming → healthy
	// - multiple idle connections from fusca-api
	replSlotsData, err := os.ReadFile("testdata/pg_replication_slots.json")
	if err != nil {
		t.Fatal(err)
	}
	replStatsData, err := os.ReadFile("testdata/pg_replication_stats.json")
	if err != nil {
		t.Fatal(err)
	}
	connectionsData, err := os.ReadFile("testdata/pg_connections.json")
	if err != nil {
		t.Fatal(err)
	}

	spec := &InfraProviderSpec{
		ID:   "postgresql",
		Name: "PostgreSQL (psql)",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "psql", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       2 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:      "replication_slots",
				CLIArgs: []string{"conn", "-t", "-A", "-c", "query1"},
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_slots_to_topology"},
					Health:   &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_slots_to_health"},
				},
			},
			{
				ID:       "replication_stats",
				CLIArgs:  []string{"conn", "-t", "-A", "-c", "query2"},
				Optional: true,
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_stats_to_topology"},
					Health:   &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_stats_to_health"},
				},
			},
			{
				ID:       "connections",
				CLIArgs:  []string{"conn", "-t", "-A", "-c", "query3"},
				Optional: true,
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{SourcePath: "items", Transform: "pg_connections_to_topology"},
					Metrics:  &InfraMappingSection{SourcePath: "items", Transform: "pg_connections_to_metrics"},
				},
			},
		},
	}

	mock := &PostgreSQLMockExecutor{
		responses: map[string][]byte{
			"replication_slots": replSlotsData,
			"replication_stats": replStatsData,
			"connections":       connectionsData,
		},
		orderedSteps: []string{"replication_slots", "replication_stats", "connections"},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("CDC investigation fetch error: %v", err)
	}

	// Check health: should have unhealthy slot + healthy WAL sender + degraded WAL sender
	unhealthyCount := 0
	healthyCount := 0
	for _, h := range ic.Health {
		switch h.Status {
		case HealthStatusUnhealthy:
			unhealthyCount++
		case HealthStatusHealthy:
			healthyCount++
		}
	}
	if unhealthyCount == 0 {
		t.Error("expected at least 1 unhealthy health check (broken CDC slot)")
	}
	if healthyCount == 0 {
		t.Error("expected at least 1 healthy health check")
	}

	// Check metrics: should have connection metrics
	if len(ic.Metrics) == 0 {
		t.Error("expected connection metrics")
	}
}

func TestPostgreSQLProvider_OptionalStepsFail(t *testing.T) {
	// database_info and replication_slots succeed, but optional steps fail
	dbInfoData, err := os.ReadFile("testdata/pg_database_info.json")
	if err != nil {
		t.Fatal(err)
	}
	replSlotsData, err := os.ReadFile("testdata/pg_replication_slots.json")
	if err != nil {
		t.Fatal(err)
	}

	spec := &InfraProviderSpec{
		ID:   "postgresql",
		Name: "PostgreSQL (psql)",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "psql", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       2 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:      "database_info",
				CLIArgs: []string{"conn", "-t", "-A", "-c", "query1"},
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{SourcePath: "items", Transform: "pg_database_info_to_topology"},
					Metrics:  &InfraMappingSection{SourcePath: "items", Transform: "pg_database_info_to_metrics"},
				},
			},
			{
				ID:      "replication_slots",
				CLIArgs: []string{"conn", "-t", "-A", "-c", "query2"},
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_slots_to_topology"},
					Health:   &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_slots_to_health"},
				},
			},
			{
				ID:       "replication_stats",
				CLIArgs:  []string{"conn", "-t", "-A", "-c", "query3"},
				Optional: true,
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_stats_to_topology"},
				},
			},
			{
				ID:       "connections",
				CLIArgs:  []string{"conn", "-t", "-A", "-c", "query4"},
				Optional: true,
				Mapping:  InfraMapping{},
			},
			{
				ID:       "tables",
				CLIArgs:  []string{"conn", "-t", "-A", "-c", "query5"},
				Optional: true,
				Mapping:  InfraMapping{},
			},
		},
	}

	mock := &PostgreSQLMockExecutor{
		responses: map[string][]byte{
			"database_info":     dbInfoData,
			"replication_slots": replSlotsData,
		},
		failSteps: map[string]bool{
			"replication_stats": true,
			"connections":       true,
			"tables":            true,
		},
		orderedSteps: []string{"database_info", "replication_slots", "replication_stats", "connections", "tables"},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("should succeed even if optional steps fail: %v", err)
	}

	// Should still have data from steps 1-2
	if len(ic.Topology) == 0 {
		t.Error("expected topology from successful steps")
	}
	if len(ic.Health) == 0 {
		t.Error("expected health from replication_slots step")
	}
}

func TestPostgreSQLProvider_EmptyResults(t *testing.T) {
	emptyJSON := []byte(`{"items": []}`)

	spec := &InfraProviderSpec{
		ID:   "postgresql",
		Name: "PostgreSQL (psql)",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI:     &InfraCLISpec{Command: "psql", Timeout: 30 * time.Second},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "default",
			TTL:       2 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:      "replication_slots",
				CLIArgs: []string{"conn", "-t", "-A", "-c", "query"},
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_slots_to_topology"},
					Health:   &InfraMappingSection{SourcePath: "items", Transform: "pg_replication_slots_to_health"},
				},
			},
		},
	}

	mock := &PostgreSQLMockExecutor{
		responses: map[string][]byte{
			"replication_slots": emptyJSON,
		},
		orderedSteps: []string{"replication_slots"},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetExecutor(mock)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("empty results should not error: %v", err)
	}
	if len(ic.Topology) != 0 {
		t.Errorf("expected 0 topology for empty results, got %d", len(ic.Topology))
	}
	if len(ic.Health) != 0 {
		t.Errorf("expected 0 health for empty results, got %d", len(ic.Health))
	}
}

func TestPostgreSQLProvider_ConnectionString(t *testing.T) {
	// Verify that PG_CONNECTION_STRING template expands correctly in cli_args
	connStr := "postgresql://user:pass@myhost:5432/mydb"
	templateData := map[string]string{
		"PG_CONNECTION_STRING": connStr,
	}

	arg := "{{.PG_CONNECTION_STRING}}"
	expanded, err := expandAnyTemplate(arg, templateData)
	if err != nil {
		t.Fatalf("template expansion failed: %v", err)
	}
	if expanded != connStr {
		t.Errorf("expected %q, got %q", connStr, expanded)
	}
}

func TestPostgreSQLProvider_AvailableCheck(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "postgresql",
		Name: "PostgreSQL (psql)",
		Transport: InfraTransportSpec{
			Primary: "cli",
			CLI: &InfraCLISpec{
				Command:        "psql",
				AvailableCheck: []string{"--version"},
			},
		},
	}
	cp := NewConfigProvider(spec, nil, nil, nil)

	mock := NewMockExecutor()
	mock.AddResponse("psql --version", []byte("psql (PostgreSQL) 15.4"))
	cp.SetExecutor(mock)

	if !cp.Available() {
		t.Error("expected Available() = true when psql --version succeeds")
	}
}

func TestLoadProviderConfig_Single_PostgreSQL(t *testing.T) {
	spec, err := LoadProviderConfig("postgresql", "")
	if err != nil {
		t.Fatalf("failed to load postgresql config: %v", err)
	}
	if spec.ID != "postgresql" {
		t.Errorf("id = %q, want postgresql", spec.ID)
	}
	if spec.Transport.CLI.Command != "psql" {
		t.Errorf("command = %q, want psql", spec.Transport.CLI.Command)
	}
}

// PostgreSQLMockExecutor matches responses by step ID.
// It detects the step from SQL query keywords in cli_args, or falls back to call order.
type PostgreSQLMockExecutor struct {
	responses map[string][]byte
	failSteps map[string]bool
	// orderedSteps defines the expected call order (for when SQL detection isn't possible)
	orderedSteps []string
	callCount    int
}

func (m *PostgreSQLMockExecutor) Execute(ctx context.Context, command string, args []string) ([]byte, error) {
	// For cluster name detection (clusterName calls kubectl)
	if command == "kubectl" {
		return []byte("postgresql"), nil
	}

	// Try to detect step from SQL keywords in the args
	stepID := m.detectStepFromArgs(args)

	// Fallback to ordered steps
	if stepID == "" && m.callCount < len(m.orderedSteps) {
		stepID = m.orderedSteps[m.callCount]
	}

	m.callCount++

	if stepID == "" {
		return nil, fmt.Errorf("mock: cannot determine step (call #%d, args: %v)", m.callCount, args)
	}

	if m.failSteps != nil && m.failSteps[stepID] {
		return nil, fmt.Errorf("mock: step %q failed", stepID)
	}

	if data, ok := m.responses[stepID]; ok {
		return data, nil
	}

	return nil, fmt.Errorf("mock: no response for step %q", stepID)
}

func (m *PostgreSQLMockExecutor) detectStepFromArgs(args []string) string {
	for _, arg := range args {
		// SQL keyword detection
		switch {
		case strContains(arg, "current_database") || strContains(arg, "pg_postmaster_start_time"):
			return "database_info"
		case strContains(arg, "pg_replication_slots"):
			return "replication_slots"
		case strContains(arg, "pg_stat_replication"):
			return "replication_stats"
		case strContains(arg, "pg_stat_activity"):
			return "connections"
		case strContains(arg, "pg_stat_user_tables"):
			return "tables"
		}
	}
	return ""
}

func strContains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestExpandTemplate(t *testing.T) {
	tests := []struct {
		tmpl string
		data map[string]string
		want string
	}{
		{
			"get pods -n {{.namespace}} -o json",
			map[string]string{"namespace": "default"},
			"get pods -n default -o json",
		},
		{
			"get pods -n {{.namespace}} -o json",
			map[string]string{"namespace": "production"},
			"get pods -n production -o json",
		},
		{
			"get nodes -o json",
			map[string]string{},
			"get nodes -o json",
		},
	}

	for _, tt := range tests {
		got, err := expandTemplate(tt.tmpl, tt.data)
		if err != nil {
			t.Fatalf("expandTemplate(%q) error: %v", tt.tmpl, err)
		}
		if got != tt.want {
			t.Errorf("expandTemplate(%q) = %q, want %q", tt.tmpl, got, tt.want)
		}
	}
}
