package infracontext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/transport"
)

// --- Helper: create a mock HTTP server from fixture files ---

func newMockDatadogServer(t *testing.T) *httptest.Server {
	t.Helper()

	fixtures := map[string]string{
		"/api/v1/integration":             "testdata/dd_integrations.json",
		"/api/v2/services/definitions":    "testdata/dd_service_definitions.json",
		"/api/v1/hosts":                   "testdata/dd_hosts.json",
		"/api/v1/service_dependencies":    "testdata/dd_dependencies.json",
		"/api/v2/containers":              "testdata/dd_containers.json",
		"/api/v1/monitor":                 "testdata/dd_monitors.json",
		"/api/v1/dashboard":               "testdata/dd_dashboards.json",
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture, ok := fixtures[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found: " + r.URL.Path})
			return
		}

		data, err := os.ReadFile(fixture)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
}

func newDatadogSpec(baseURL string) *InfraProviderSpec {
	spec, err := LoadProviderConfig("datadog", "")
	if err != nil {
		panic(fmt.Sprintf("failed to load datadog config: %v", err))
	}
	// Override the base_url to point to our test server
	spec.Transport.HTTP.BaseURL = baseURL
	spec.Transport.HTTP.Timeout = 10 * time.Second
	return spec
}

// --- Full pipeline test ---

func TestConfigProvider_HTTP_FullPipeline(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", map[string]string{
		"DD-API-KEY":         "test-key",
		"DD-APPLICATION-KEY": "test-app-key",
		"Content-Type":       "application/json",
	}, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("full pipeline fetch error: %v", err)
	}

	// Provider and cluster
	if ic.Provider != "datadog" {
		t.Errorf("provider = %q, want datadog", ic.Provider)
	}
	if ic.Cluster != "datadog" {
		t.Errorf("cluster = %q, want datadog", ic.Cluster)
	}

	// Topology should contain hosts, integrations, services, deps, containers, dashboards
	if len(ic.Topology) == 0 {
		t.Error("expected topology nodes")
	}

	kinds := make(map[string]int)
	for _, node := range ic.Topology {
		kinds[node.Kind]++
	}

	// Verify we have different kinds
	if kinds["Host"] != 3 {
		t.Errorf("hosts = %d, want 3", kinds["Host"])
	}
	if kinds["Integration"] != 4 { // 4 installed out of 5
		t.Errorf("integrations = %d, want 4", kinds["Integration"])
	}
	if kinds["Service"] != 3 {
		t.Errorf("services = %d, want 3", kinds["Service"])
	}
	if kinds["ServiceDependency"] != 3 {
		t.Errorf("service deps = %d, want 3", kinds["ServiceDependency"])
	}
	if kinds["Container"] != 3 {
		t.Errorf("containers = %d, want 3", kinds["Container"])
	}
	if kinds["Dashboard"] != 2 {
		t.Errorf("dashboards = %d, want 2", kinds["Dashboard"])
	}

	// Alerts from monitors
	if len(ic.Alerts) != 3 {
		t.Errorf("alerts = %d, want 3", len(ic.Alerts))
	}

	// Metrics from hosts
	if len(ic.Metrics) == 0 {
		t.Error("expected metrics from hosts")
	}

	// Verify summary works
	summary := ic.Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

// --- Individual step tests ---

func TestConfigProvider_HTTP_Hosts(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"hosts"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	// Only hosts topology + metrics
	if len(ic.Topology) != 3 {
		t.Errorf("topology = %d, want 3 hosts", len(ic.Topology))
	}
	for _, node := range ic.Topology {
		if node.Kind != "Host" {
			t.Errorf("unexpected kind %q when filtering to hosts", node.Kind)
		}
	}

	// Should have host metrics
	if len(ic.Metrics) == 0 {
		t.Error("expected host metrics")
	}
}

func TestConfigProvider_HTTP_Monitors(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"monitors"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Alerts) != 3 {
		t.Errorf("alerts = %d, want 3", len(ic.Alerts))
	}

	// Verify alert severities
	severities := make(map[string]int)
	for _, a := range ic.Alerts {
		severities[a.Severity]++
	}
	if severities[AlertSeverityCritical] != 1 {
		t.Errorf("critical alerts = %d, want 1", severities[AlertSeverityCritical])
	}
}

func TestConfigProvider_HTTP_Integrations(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"integrations"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	// 4 installed integrations (mysql is not installed)
	if len(ic.Topology) != 4 {
		t.Errorf("topology = %d, want 4 installed integrations", len(ic.Topology))
	}
}

func TestConfigProvider_HTTP_ServiceDefinitions(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"service_definitions"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Topology) != 3 {
		t.Errorf("topology = %d, want 3 services", len(ic.Topology))
	}
}

func TestConfigProvider_HTTP_Containers(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"containers"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Topology) != 3 {
		t.Errorf("topology = %d, want 3 containers", len(ic.Topology))
	}
	for _, node := range ic.Topology {
		if node.Kind != "Container" {
			t.Errorf("unexpected kind %q", node.Kind)
		}
	}
}

func TestConfigProvider_HTTP_Dashboards(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"dashboards"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Topology) != 2 {
		t.Errorf("topology = %d, want 2 dashboards", len(ic.Topology))
	}
	for _, node := range ic.Topology {
		if node.Kind != "Dashboard" {
			t.Errorf("unexpected kind %q", node.Kind)
		}
	}
}

func TestConfigProvider_HTTP_Dependencies(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"service_dependencies"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Topology) != 3 {
		t.Errorf("topology = %d, want 3 service dependencies", len(ic.Topology))
	}

	// Verify ConnectsTo
	for _, node := range ic.Topology {
		if node.Name == "api-gateway" {
			if len(node.ConnectsTo) != 2 {
				t.Errorf("api-gateway connects_to = %d, want 2", len(node.ConnectsTo))
			}
		}
	}
}

// --- Error cases ---

func TestConfigProvider_HTTP_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	spec := &InfraProviderSpec{
		ID:   "datadog",
		Name: "Datadog",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP: &InfraHTTPSpec{
				BaseURL: server.URL,
				Timeout: 10 * time.Second,
			},
		},
		Defaults: InfraDefaultsSpec{
			Namespace: "",
			TTL:       5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "hosts",
				HTTPMethod: "GET",
				HTTPPath:   "/api/v1/hosts",
				Mapping:    InfraMapping{},
			},
		},
	}

	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestConfigProvider_HTTP_OptionalStepFails(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/api/v1/hosts" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"host_list": []}`))
		} else {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`))
		}
	}))
	defer server.Close()

	spec := &InfraProviderSpec{
		ID:   "datadog",
		Name: "Datadog",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP: &InfraHTTPSpec{
				BaseURL: server.URL,
				Timeout: 10 * time.Second,
			},
		},
		Defaults: InfraDefaultsSpec{
			TTL: 5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "hosts",
				HTTPMethod: "GET",
				HTTPPath:   "/api/v1/hosts",
				Mapping:    InfraMapping{},
			},
			{
				ID:         "integrations",
				HTTPMethod: "GET",
				HTTPPath:   "/api/v1/integration",
				Optional:   true,
				Mapping:    InfraMapping{},
			},
		},
	}

	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch should succeed even if optional step fails: %v", err)
	}
	if ic == nil {
		t.Fatal("expected non-nil InfraContext")
	}
}

// --- Available tests ---

func TestConfigProvider_HTTP_Available_WithTransport(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "datadog",
		Name: "Datadog",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP: &InfraHTTPSpec{
				BaseURL: "https://api.datadoghq.com",
				Timeout: 30 * time.Second,
			},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	// Without injected transport or resolver, should be unavailable
	if cp.Available() {
		t.Error("expected Available() = false without credentials or transport")
	}

	// With injected transport, should be available
	tr := transport.NewHTTPTransport("https://api.datadoghq.com", "", "", nil, 0)
	cp.SetHTTPTransport(tr)
	if !cp.Available() {
		t.Error("expected Available() = true with injected transport")
	}
}

// --- Cache test ---

func TestConfigProvider_HTTP_WithCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"host_list": []}`))
	}))
	defer server.Close()

	spec := &InfraProviderSpec{
		ID:   "datadog",
		Name: "Datadog",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP: &InfraHTTPSpec{
				BaseURL: server.URL,
				Timeout: 10 * time.Second,
			},
		},
		Defaults: InfraDefaultsSpec{
			TTL: 5 * time.Minute,
		},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "hosts",
				HTTPMethod: "GET",
				HTTPPath:   "/api/v1/hosts",
				Mapping:    InfraMapping{},
			},
		},
	}

	cache := NewCache("")
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, cache, nil)
	cp.SetHTTPTransport(tr)

	// First fetch
	ic1, err := cp.Fetch(context.Background(), FetchOptions{UseCache: true})
	if err != nil {
		t.Fatalf("first fetch error: %v", err)
	}

	// Second fetch should hit cache
	ic2, err := cp.Fetch(context.Background(), FetchOptions{UseCache: true})
	if err != nil {
		t.Fatalf("second fetch error: %v", err)
	}

	if ic1.FetchedAt != ic2.FetchedAt {
		t.Error("second fetch should return cached result")
	}
	if callCount != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", callCount)
	}
}

// --- ClusterName test ---

func TestConfigProvider_HTTP_ClusterName(t *testing.T) {
	spec := &InfraProviderSpec{
		ID:   "datadog",
		Name: "Datadog",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP: &InfraHTTPSpec{
				BaseURL: "https://api.datadoghq.com",
				Timeout: 30 * time.Second,
			},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)

	// Without KubeContext, HTTP providers use provider ID
	name := cp.clusterName(FetchOptions{})
	if name != "datadog" {
		t.Errorf("clusterName = %q, want datadog", name)
	}

	// With KubeContext, should use it
	name = cp.clusterName(FetchOptions{KubeContext: "my-context"})
	if name != "my-context" {
		t.Errorf("clusterName = %q, want my-context", name)
	}
}

// --- Template expansion tests ---

func TestExpandAnyTemplate(t *testing.T) {
	tests := []struct {
		tmpl string
		data map[string]string
		want string
	}{
		{
			"https://{{.DD_SITE}}",
			map[string]string{"DD_SITE": "api.datadoghq.com"},
			"https://api.datadoghq.com",
		},
		{
			"{{.DD_API_KEY}}",
			map[string]string{"DD_API_KEY": "abc123"},
			"abc123",
		},
	}

	for _, tt := range tests {
		got, err := expandAnyTemplate(tt.tmpl, tt.data)
		if err != nil {
			t.Fatalf("expandAnyTemplate(%q) error: %v", tt.tmpl, err)
		}
		if got != tt.want {
			t.Errorf("expandAnyTemplate(%q) = %q, want %q", tt.tmpl, got, tt.want)
		}
	}
}

// --- WrapForMapping test ---

func TestWrapForMapping(t *testing.T) {
	items := []any{"a", "b", "c"}

	// With path
	result := wrapForMapping(items, "data")
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	inner, ok := m["data"].([]any)
	if !ok {
		t.Fatalf("expected []any at data, got %T", m["data"])
	}
	if len(inner) != 3 {
		t.Errorf("inner len = %d, want 3", len(inner))
	}

	// Without path
	result2 := wrapForMapping(items, "")
	arr, ok := result2.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result2)
	}
	if len(arr) != 3 {
		t.Errorf("arr len = %d, want 3", len(arr))
	}

	// Nested path
	result3 := wrapForMapping(items, "meta.results")
	m3, ok := result3.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result3)
	}
	inner3, ok := m3["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map at meta, got %T", m3["meta"])
	}
	if _, ok := inner3["results"].([]any); !ok {
		t.Errorf("expected []any at meta.results")
	}

	// Dot path (root)
	result4 := wrapForMapping(items, ".")
	arr4, ok := result4.([]any)
	if !ok {
		t.Fatalf("expected []any for dot path, got %T", result4)
	}
	if len(arr4) != 3 {
		t.Errorf("arr4 len = %d, want 3", len(arr4))
	}
}

// --- JSON serialization test ---

func TestConfigProvider_HTTP_JSONSerialization(t *testing.T) {
	server := newMockDatadogServer(t)
	defer server.Close()

	spec := newDatadogSpec(server.URL)
	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	data, err := json.Marshal(ic)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}

	// Verify it round-trips
	var ic2 InfraContext
	if err := json.Unmarshal(data, &ic2); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if ic2.Provider != "datadog" {
		t.Errorf("provider = %q after round-trip", ic2.Provider)
	}
}

// =====================================================================
// Step-chaining tests
// =====================================================================

func TestExtractProvides_SingleField(t *testing.T) {
	rawData := map[string]any{
		"data": []any{
			map[string]any{"id": "c1", "name": "cluster-1"},
			map[string]any{"id": "c2", "name": "cluster-2"},
		},
	}

	provides := map[string]*InfraProvidesSpec{
		"clusters": {
			SourcePath: "data",
			Field:      "id",
		},
	}

	result := extractProvides(rawData, provides)
	items := result["clusters"]
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].(string) != "c1" {
		t.Errorf("items[0] = %v, want c1", items[0])
	}
	if items[1].(string) != "c2" {
		t.Errorf("items[1] = %v, want c2", items[1])
	}
}

func TestExtractProvides_MultiField(t *testing.T) {
	rawData := map[string]any{
		"data": []any{
			map[string]any{
				"id": "c1",
				"spec": map[string]any{
					"http_endpoint": "https://cluster1.example.com",
					"display_name":  "cluster-1",
				},
			},
			map[string]any{
				"id": "c2",
				"spec": map[string]any{
					"http_endpoint": "https://cluster2.example.com",
					"display_name":  "cluster-2",
				},
			},
		},
	}

	provides := map[string]*InfraProvidesSpec{
		"clusters": {
			SourcePath: "data",
			Fields: map[string]string{
				"id":       "id",
				"endpoint": "spec.http_endpoint",
				"name":     "spec.display_name",
			},
		},
	}

	result := extractProvides(rawData, provides)
	items := result["clusters"]
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	first := items[0].(map[string]string)
	if first["id"] != "c1" {
		t.Errorf("first.id = %q, want c1", first["id"])
	}
	if first["endpoint"] != "https://cluster1.example.com" {
		t.Errorf("first.endpoint = %q", first["endpoint"])
	}
}

func TestExtractProvides_EmptyData(t *testing.T) {
	rawData := map[string]any{
		"data": []any{},
	}

	provides := map[string]*InfraProvidesSpec{
		"clusters": {
			SourcePath: "data",
			Field:      "id",
		},
	}

	result := extractProvides(rawData, provides)
	if len(result["clusters"]) != 0 {
		t.Errorf("expected 0 items, got %d", len(result["clusters"]))
	}
}

func TestApplyItemToTemplateData_String(t *testing.T) {
	data := map[string]string{"namespace": "default"}
	applyItemToTemplateData(data, "cluster-1")

	if data["item"] != "cluster-1" {
		t.Errorf("item = %q, want cluster-1", data["item"])
	}
	if data["namespace"] != "default" {
		t.Errorf("namespace should be preserved")
	}
}

func TestApplyItemToTemplateData_Map(t *testing.T) {
	data := map[string]string{"namespace": "default"}
	applyItemToTemplateData(data, map[string]string{
		"id":       "lkc-abc",
		"endpoint": "https://cluster.example.com",
	})

	if data["item_id"] != "lkc-abc" {
		t.Errorf("item_id = %q, want lkc-abc", data["item_id"])
	}
	if data["item_endpoint"] != "https://cluster.example.com" {
		t.Errorf("item_endpoint = %q", data["item_endpoint"])
	}
}

func TestCloneTemplateData(t *testing.T) {
	orig := map[string]string{"a": "1", "b": "2"}
	clone := cloneTemplateData(orig)
	clone["c"] = "3"

	if _, ok := orig["c"]; ok {
		t.Error("clone should not modify original")
	}
	if clone["a"] != "1" {
		t.Error("clone should have original values")
	}
}

func TestStepChaining_SingleField(t *testing.T) {
	// Server provides a list of items on /discover, and per-item data on /items/{id}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/discover":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": "a"},
					map[string]any{"id": "b"},
				},
			})
		case "/items/a":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []any{
					map[string]any{"name": "item-a1", "kind": "Thing"},
				},
			})
		case "/items/b":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []any{
					map[string]any{"name": "item-b1", "kind": "Thing"},
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	// Register a test transform
	RegisterTransform("test_thing_topology", func(rawItems []any, args map[string]any) (any, error) {
		var nodes []TopologyNode
		for _, item := range rawItems {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			nodes = append(nodes, TopologyNode{Kind: "Thing", Name: name, Status: "Active"})
		}
		return nodes, nil
	})

	spec := &InfraProviderSpec{
		ID:   "test-chaining",
		Name: "Test Chaining",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP:    &InfraHTTPSpec{BaseURL: server.URL, Timeout: 10 * time.Second},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "discover",
				HTTPMethod: "GET",
				HTTPPath:   "/discover",
				Provides: map[string]*InfraProvidesSpec{
					"things": {SourcePath: "items", Field: "id"},
				},
				Mapping: InfraMapping{},
			},
			{
				ID:         "details",
				ForEach:    "things",
				HTTPMethod: "GET",
				HTTPPath:   "/items/{{.item}}",
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{
						SourcePath: "data",
						Transform:  "test_thing_topology",
					},
				},
			},
		},
	}

	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Topology) != 2 {
		t.Fatalf("expected 2 topology nodes, got %d", len(ic.Topology))
	}
	if ic.Topology[0].Name != "item-a1" {
		t.Errorf("node[0].Name = %q, want item-a1", ic.Topology[0].Name)
	}
	if ic.Topology[1].Name != "item-b1" {
		t.Errorf("node[1].Name = %q, want item-b1", ic.Topology[1].Name)
	}
}

func TestStepChaining_MultiField(t *testing.T) {
	// Server: discover returns items with id+endpoint, detail uses per-item endpoint
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []any{
				map[string]any{"name": "res-1"},
			},
		})
	}))
	defer mainServer.Close()

	discoverServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"clusters": []any{
				map[string]any{
					"id": "c1",
					"spec": map[string]any{
						"endpoint": mainServer.URL,
					},
				},
			},
		})
	}))
	defer discoverServer.Close()

	RegisterTransform("test_res_topology", func(rawItems []any, args map[string]any) (any, error) {
		var nodes []TopologyNode
		for _, item := range rawItems {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			nodes = append(nodes, TopologyNode{Kind: "Resource", Name: name, Status: "Active"})
		}
		return nodes, nil
	})

	spec := &InfraProviderSpec{
		ID:   "test-multi",
		Name: "Test Multi-Field",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP:    &InfraHTTPSpec{BaseURL: discoverServer.URL, Timeout: 10 * time.Second},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "discover",
				HTTPMethod: "GET",
				HTTPPath:   "/clusters",
				Provides: map[string]*InfraProvidesSpec{
					"clusters": {
						SourcePath: "clusters",
						Fields: map[string]string{
							"id":       "id",
							"endpoint": "spec.endpoint",
						},
					},
				},
				Mapping: InfraMapping{},
			},
			{
				ID:          "resources",
				ForEach:     "clusters",
				HTTPBaseURL: "{{.item_endpoint}}",
				HTTPMethod:  "GET",
				HTTPPath:    "/v3/clusters/{{.item_id}}/resources",
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{
						SourcePath: "data",
						Transform:  "test_res_topology",
					},
				},
			},
		},
	}

	tr := transport.NewHTTPTransport(discoverServer.URL, "", "", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Topology) != 1 {
		t.Fatalf("expected 1 topology node, got %d", len(ic.Topology))
	}
	if ic.Topology[0].Name != "res-1" {
		t.Errorf("name = %q, want res-1", ic.Topology[0].Name)
	}
}

func TestStepChaining_HTTPBaseURLOverride(t *testing.T) {
	// cluster1 and cluster2 are separate servers
	cluster1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []any{
				map[string]any{"name": "from-cluster1"},
			},
		})
	}))
	defer cluster1.Close()

	cluster2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []any{
				map[string]any{"name": "from-cluster2"},
			},
		})
	}))
	defer cluster2.Close()

	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []any{
				map[string]any{
					"id":       "c1",
					"endpoint": cluster1.URL,
				},
				map[string]any{
					"id":       "c2",
					"endpoint": cluster2.URL,
				},
			},
		})
	}))
	defer mgmtServer.Close()

	RegisterTransform("test_simple_topology", func(rawItems []any, args map[string]any) (any, error) {
		var nodes []TopologyNode
		for _, item := range rawItems {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			nodes = append(nodes, TopologyNode{Kind: "Item", Name: name, Status: "Active"})
		}
		return nodes, nil
	})

	spec := &InfraProviderSpec{
		ID:   "test-override",
		Name: "Test Override",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP:    &InfraHTTPSpec{BaseURL: mgmtServer.URL, Timeout: 10 * time.Second},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "discover",
				HTTPMethod: "GET",
				HTTPPath:   "/clusters",
				Provides: map[string]*InfraProvidesSpec{
					"clusters": {
						SourcePath: "data",
						Fields: map[string]string{
							"id":       "id",
							"endpoint": "endpoint",
						},
					},
				},
				Mapping: InfraMapping{},
			},
			{
				ID:          "items",
				ForEach:     "clusters",
				HTTPBaseURL: "{{.item_endpoint}}",
				HTTPMethod:  "GET",
				HTTPPath:    "/items",
				Mapping: InfraMapping{
					Topology: &InfraMappingSection{
						SourcePath: "data",
						Transform:  "test_simple_topology",
					},
				},
			},
		},
	}

	tr := transport.NewHTTPTransport(mgmtServer.URL, "", "", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Topology) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(ic.Topology))
	}

	names := map[string]bool{}
	for _, n := range ic.Topology {
		names[n.Name] = true
	}
	if !names["from-cluster1"] {
		t.Error("missing from-cluster1")
	}
	if !names["from-cluster2"] {
		t.Error("missing from-cluster2")
	}
}

func TestStepChaining_EmptyProvides(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer server.Close()

	spec := &InfraProviderSpec{
		ID:   "test-empty",
		Name: "Test Empty",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP:    &InfraHTTPSpec{BaseURL: server.URL, Timeout: 10 * time.Second},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "discover",
				HTTPMethod: "GET",
				HTTPPath:   "/clusters",
				Provides: map[string]*InfraProvidesSpec{
					"clusters": {SourcePath: "data", Field: "id"},
				},
				Mapping: InfraMapping{},
			},
			{
				ID:         "details",
				ForEach:    "clusters",
				HTTPMethod: "GET",
				HTTPPath:   "/items/{{.item}}",
				Optional:   true,
				Mapping:    InfraMapping{},
			},
		},
	}

	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if ic == nil {
		t.Fatal("expected non-nil InfraContext")
	}
}

func TestStepChaining_NonOptionalForEachFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer server.Close()

	spec := &InfraProviderSpec{
		ID:   "test-fail",
		Name: "Test Fail",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP:    &InfraHTTPSpec{BaseURL: server.URL, Timeout: 10 * time.Second},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "discover",
				HTTPMethod: "GET",
				HTTPPath:   "/clusters",
				Provides: map[string]*InfraProvidesSpec{
					"clusters": {SourcePath: "data", Field: "id"},
				},
				Mapping: InfraMapping{},
			},
			{
				ID:         "details",
				ForEach:    "clusters",
				HTTPMethod: "GET",
				HTTPPath:   "/items/{{.item}}",
				Optional:   false, // required — should fail with empty provides
				Mapping:    InfraMapping{},
			},
		},
	}

	tr := transport.NewHTTPTransport(server.URL, "", "", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err == nil {
		t.Error("expected error for non-optional for_each with empty provides")
	}
}

// =====================================================================
// Auth type tests
// =====================================================================

func TestEnsureHTTPTransport_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Basic mykey:mysecret" {
			t.Errorf("Authorization = %q, want 'Basic mykey:mysecret'", auth)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	spec := &InfraProviderSpec{
		ID:   "test-basic",
		Name: "Test Basic Auth",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP: &InfraHTTPSpec{
				BaseURL:   server.URL,
				AuthType:  "basic",
				AuthValue: "mykey:mysecret",
				Timeout:   10 * time.Second,
			},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "test",
				HTTPMethod: "GET",
				HTTPPath:   "/test",
				Mapping:    InfraMapping{},
			},
		},
	}

	// Create transport with basic auth
	tr := transport.NewHTTPTransport(server.URL, "basic", "mykey:mysecret", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
}

func TestEnsureHTTPTransport_BearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-token-123" {
			t.Errorf("Authorization = %q, want 'Bearer my-token-123'", auth)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	tr := transport.NewHTTPTransport(server.URL, "bearer", "my-token-123", nil, 10*time.Second)
	spec := &InfraProviderSpec{
		ID:   "test-bearer",
		Name: "Test Bearer Auth",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP:    &InfraHTTPSpec{BaseURL: server.URL, Timeout: 10 * time.Second},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "test",
				HTTPMethod: "GET",
				HTTPPath:   "/test",
				Mapping:    InfraMapping{},
			},
		},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
}

func TestEnsureHTTPTransport_NoAuthType(t *testing.T) {
	// Backwards compat: no auth_type means auth via headers (Datadog pattern)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("should not have Authorization header when no auth_type")
		}
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Errorf("DD-API-KEY = %q, want test-key", r.Header.Get("DD-API-KEY"))
		}
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	tr := transport.NewHTTPTransport(server.URL, "", "", map[string]string{
		"DD-API-KEY": "test-key",
	}, 10*time.Second)

	spec := &InfraProviderSpec{
		ID:   "test-noauth",
		Name: "Test No Auth",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP:    &InfraHTTPSpec{BaseURL: server.URL, Timeout: 10 * time.Second},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "test",
				HTTPMethod: "GET",
				HTTPPath:   "/test",
				Mapping:    InfraMapping{},
			},
		},
	}

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
}

// =====================================================================
// Kafka full pipeline tests
// =====================================================================

func newMockKafkaServer(t *testing.T) (*httptest.Server, *httptest.Server) {
	t.Helper()

	// Per-cluster REST API server
	clusterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixtures := map[string]string{
			"/kafka/v3/clusters/lkc-abc123/brokers":         "testdata/kafka_brokers.json",
			"/kafka/v3/clusters/lkc-abc123/topics":          "testdata/kafka_topics.json",
			"/kafka/v3/clusters/lkc-abc123/consumer-groups": "testdata/kafka_consumer_groups.json",
		}

		fixture, ok := fixtures[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found: " + r.URL.Path})
			return
		}

		data, err := os.ReadFile(fixture)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))

	// Management API server — returns clusters pointing to the cluster server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cmk/v2/clusters":
			// Return clusters with http_endpoint pointing to our test cluster server
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data": []any{
					map[string]any{
						"id": "lkc-abc123",
						"spec": map[string]any{
							"display_name":  "production-cluster",
							"availability":  "HIGH",
							"cloud":         "AWS",
							"region":        "us-east-1",
							"http_endpoint": clusterServer.URL,
						},
						"status": map[string]any{
							"phase": "PROVISIONED",
						},
					},
				},
				"metadata": map[string]any{"total_size": 1},
			})
		default:
			// Connectors endpoint
			if r.URL.Path == "/connect/v1/environments/_/clusters/lkc-abc123/connectors" {
				data, err := os.ReadFile("testdata/kafka_connectors.json")
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}))

	return mgmtServer, clusterServer
}

func TestKafkaProvider_FullPipeline(t *testing.T) {
	mgmtServer, clusterServer := newMockKafkaServer(t)
	defer mgmtServer.Close()
	defer clusterServer.Close()

	spec, err := LoadProviderConfig("kafka", "")
	if err != nil {
		t.Fatalf("failed to load kafka config: %v", err)
	}

	// Override base URL to our test server
	spec.Transport.HTTP.BaseURL = mgmtServer.URL

	tr := transport.NewHTTPTransport(mgmtServer.URL, "basic", "testkey:testsecret", map[string]string{
		"Content-Type": "application/json",
	}, 10*time.Second)

	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)
	cp.httpAuthType = "basic"
	cp.httpAuthValue = "testkey:testsecret"

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("full pipeline fetch error: %v", err)
	}

	if ic.Provider != "kafka" {
		t.Errorf("provider = %q, want kafka", ic.Provider)
	}

	// Count topology by kind
	kinds := make(map[string]int)
	for _, node := range ic.Topology {
		kinds[node.Kind]++
	}

	if kinds["KafkaCluster"] != 1 {
		t.Errorf("clusters = %d, want 1", kinds["KafkaCluster"])
	}
	if kinds["KafkaBroker"] != 3 {
		t.Errorf("brokers = %d, want 3", kinds["KafkaBroker"])
	}
	// 4 topics total - 1 internal = 3 user topics
	if kinds["KafkaTopic"] != 3 {
		t.Errorf("topics = %d, want 3", kinds["KafkaTopic"])
	}
	if kinds["ConsumerGroup"] != 3 {
		t.Errorf("consumer_groups = %d, want 3", kinds["ConsumerGroup"])
	}
	if kinds["KafkaConnector"] != 3 {
		t.Errorf("connectors = %d, want 3", kinds["KafkaConnector"])
	}

	// Health checks from consumer groups + connectors
	if len(ic.Health) != 6 { // 3 consumer groups + 3 connectors
		t.Errorf("health = %d, want 6", len(ic.Health))
	}

	// Verify some health statuses
	healthByComponent := make(map[string]string)
	for _, h := range ic.Health {
		healthByComponent[h.Component] = h.Status
	}
	if healthByComponent["order-processor"] != HealthStatusHealthy {
		t.Errorf("order-processor health = %q, want healthy", healthByComponent["order-processor"])
	}
	if healthByComponent["payment-handler"] != HealthStatusDegraded {
		t.Errorf("payment-handler health = %q, want degraded", healthByComponent["payment-handler"])
	}
	if healthByComponent["dead-letter-consumer"] != HealthStatusUnhealthy {
		t.Errorf("dead-letter-consumer health = %q, want unhealthy", healthByComponent["dead-letter-consumer"])
	}
	if healthByComponent["orders-s3-sink"] != HealthStatusHealthy {
		t.Errorf("orders-s3-sink health = %q, want healthy", healthByComponent["orders-s3-sink"])
	}
}

func TestKafkaProvider_SingleCluster(t *testing.T) {
	mgmtServer, clusterServer := newMockKafkaServer(t)
	defer mgmtServer.Close()
	defer clusterServer.Close()

	spec, err := LoadProviderConfig("kafka", "")
	if err != nil {
		t.Fatalf("failed to load kafka config: %v", err)
	}

	spec.Transport.HTTP.BaseURL = mgmtServer.URL

	tr := transport.NewHTTPTransport(mgmtServer.URL, "basic", "k:s", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)
	cp.httpAuthType = "basic"
	cp.httpAuthValue = "k:s"

	// Only fetch clusters step
	ic, err := cp.Fetch(context.Background(), FetchOptions{
		ResourceTypes: []string{"clusters"},
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(ic.Topology) != 1 {
		t.Errorf("topology = %d, want 1 cluster", len(ic.Topology))
	}
	if ic.Topology[0].Kind != "KafkaCluster" {
		t.Errorf("kind = %q, want KafkaCluster", ic.Topology[0].Kind)
	}
}

func TestKafkaProvider_AuthBasic(t *testing.T) {
	authChecked := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Basic testkey:testsecret" {
			authChecked = true
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []any{},
		})
	}))
	defer server.Close()

	spec := &InfraProviderSpec{
		ID:   "kafka",
		Name: "Kafka",
		Transport: InfraTransportSpec{
			Primary: "http",
			HTTP: &InfraHTTPSpec{
				BaseURL:   server.URL,
				AuthType:  "basic",
				AuthValue: "testkey:testsecret",
				Timeout:   10 * time.Second,
			},
		},
		Defaults: InfraDefaultsSpec{TTL: 5 * time.Minute},
		FetchSteps: []InfraFetchStep{
			{
				ID:         "clusters",
				HTTPMethod: "GET",
				HTTPPath:   "/cmk/v2/clusters",
				Mapping:    InfraMapping{},
			},
		},
	}

	tr := transport.NewHTTPTransport(server.URL, "basic", "testkey:testsecret", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)

	_, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if !authChecked {
		t.Error("expected Basic auth header to be sent")
	}
}

func TestKafkaProvider_ConnectorsOptional(t *testing.T) {
	// Connectors endpoint fails but pipeline should continue
	clusterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer clusterServer.Close()

	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cmk/v2/clusters" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data": []any{
					map[string]any{
						"id": "lkc-test",
						"spec": map[string]any{
							"display_name":  "test",
							"http_endpoint": clusterServer.URL,
						},
						"status": map[string]any{"phase": "PROVISIONED"},
					},
				},
			})
		} else {
			// All other endpoints fail
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`))
		}
	}))
	defer mgmtServer.Close()

	spec, err := LoadProviderConfig("kafka", "")
	if err != nil {
		t.Fatalf("failed to load kafka config: %v", err)
	}
	spec.Transport.HTTP.BaseURL = mgmtServer.URL

	tr := transport.NewHTTPTransport(mgmtServer.URL, "basic", "k:s", nil, 10*time.Second)
	cp := NewConfigProvider(spec, nil, nil, nil)
	cp.SetHTTPTransport(tr)
	cp.httpAuthType = "basic"
	cp.httpAuthValue = "k:s"

	ic, err := cp.Fetch(context.Background(), FetchOptions{})
	if err != nil {
		t.Fatalf("fetch should succeed even if optional for_each steps fail: %v", err)
	}

	// Should have cluster topology at minimum
	if len(ic.Topology) < 1 {
		t.Error("expected at least 1 topology node (cluster)")
	}
}
