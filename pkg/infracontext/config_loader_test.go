package infracontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProviderConfigs_Embedded(t *testing.T) {
	specs, err := LoadProviderConfigs("")
	if err != nil {
		t.Fatalf("failed to load configs: %v", err)
	}

	// Should have kubectl, datadog, kafka, and postgresql
	if len(specs) < 4 {
		t.Fatalf("expected at least 4 provider configs, got %d", len(specs))
	}

	// --- Verify kubectl ---
	kubectl, ok := specs["kubectl"]
	if !ok {
		t.Fatal("expected kubectl provider")
	}

	if kubectl.Name != "Kubernetes (kubectl)" {
		t.Errorf("kubectl name = %q", kubectl.Name)
	}
	if kubectl.Transport.Primary != "cli" {
		t.Errorf("kubectl transport primary = %q, want cli", kubectl.Transport.Primary)
	}
	if kubectl.Transport.CLI == nil {
		t.Fatal("expected CLI transport")
	}
	if kubectl.Transport.CLI.Command != "kubectl" {
		t.Errorf("cli command = %q, want kubectl", kubectl.Transport.CLI.Command)
	}
	if len(kubectl.FetchSteps) != 5 {
		t.Errorf("expected 5 fetch steps, got %d", len(kubectl.FetchSteps))
	}

	// Verify step IDs
	stepIDs := make(map[string]bool)
	for _, step := range kubectl.FetchSteps {
		stepIDs[step.ID] = true
	}
	for _, expected := range []string{"pods", "services", "deployments", "nodes", "top_pods"} {
		if !stepIDs[expected] {
			t.Errorf("missing fetch step: %s", expected)
		}
	}

	// Verify top_pods is optional
	for _, step := range kubectl.FetchSteps {
		if step.ID == "top_pods" {
			if !step.Optional {
				t.Error("top_pods step should be optional")
			}
			if step.ParseMode != "text" {
				t.Errorf("top_pods parse_mode = %q, want text", step.ParseMode)
			}
		}
	}

	// --- Verify datadog ---
	dd, ok := specs["datadog"]
	if !ok {
		t.Fatal("expected datadog provider")
	}
	if dd.Name != "Datadog" {
		t.Errorf("datadog name = %q", dd.Name)
	}
	if dd.Transport.Primary != "http" {
		t.Errorf("datadog transport primary = %q, want http", dd.Transport.Primary)
	}
	if dd.Transport.HTTP == nil {
		t.Fatal("expected HTTP transport for datadog")
	}
	if len(dd.FetchSteps) != 7 {
		t.Errorf("expected 7 datadog fetch steps, got %d", len(dd.FetchSteps))
	}
	// Verify some datadog step IDs
	ddStepIDs := make(map[string]bool)
	for _, step := range dd.FetchSteps {
		ddStepIDs[step.ID] = true
	}
	for _, expected := range []string{"integrations", "hosts", "monitors", "dashboards"} {
		if !ddStepIDs[expected] {
			t.Errorf("missing datadog fetch step: %s", expected)
		}
	}

	// --- Verify kafka ---
	kafka, ok := specs["kafka"]
	if !ok {
		t.Fatal("expected kafka provider")
	}
	if kafka.Name != "Kafka (Confluent Cloud)" {
		t.Errorf("kafka name = %q", kafka.Name)
	}
	if kafka.Transport.Primary != "http" {
		t.Errorf("kafka transport primary = %q, want http", kafka.Transport.Primary)
	}
	if kafka.Transport.HTTP == nil {
		t.Fatal("expected HTTP transport for kafka")
	}
	if kafka.Transport.HTTP.AuthType != "basic" {
		t.Errorf("kafka auth_type = %q, want basic", kafka.Transport.HTTP.AuthType)
	}
	if kafka.Transport.HTTP.AuthValue == "" {
		t.Error("kafka auth_value should not be empty")
	}
	if len(kafka.FetchSteps) != 5 {
		t.Errorf("expected 5 kafka fetch steps, got %d", len(kafka.FetchSteps))
	}
	// Verify step IDs and step-chaining
	kafkaStepIDs := make(map[string]bool)
	for _, step := range kafka.FetchSteps {
		kafkaStepIDs[step.ID] = true
	}
	for _, expected := range []string{"clusters", "brokers", "topics", "consumer_groups", "connectors"} {
		if !kafkaStepIDs[expected] {
			t.Errorf("missing kafka fetch step: %s", expected)
		}
	}
	// Verify clusters step has provides
	for _, step := range kafka.FetchSteps {
		if step.ID == "clusters" {
			if step.Provides == nil {
				t.Error("clusters step should have provides")
			}
			if _, ok := step.Provides["clusters"]; !ok {
				t.Error("clusters step should provide 'clusters'")
			}
		}
		if step.ID == "brokers" {
			if step.ForEach != "clusters" {
				t.Errorf("brokers for_each = %q, want clusters", step.ForEach)
			}
			if step.HTTPBaseURL == "" {
				t.Error("brokers step should have http_base_url")
			}
			if !step.Optional {
				t.Error("brokers step should be optional")
			}
		}
	}

	// --- Verify postgresql ---
	pg, ok := specs["postgresql"]
	if !ok {
		t.Fatal("expected postgresql provider")
	}
	if pg.Name != "PostgreSQL (psql)" {
		t.Errorf("postgresql name = %q", pg.Name)
	}
	if pg.Transport.Primary != "cli" {
		t.Errorf("postgresql transport primary = %q, want cli", pg.Transport.Primary)
	}
	if pg.Transport.CLI == nil {
		t.Fatal("expected CLI transport for postgresql")
	}
	if pg.Transport.CLI.Command != "psql" {
		t.Errorf("postgresql cli command = %q, want psql", pg.Transport.CLI.Command)
	}
	if len(pg.FetchSteps) != 5 {
		t.Errorf("expected 5 postgresql fetch steps, got %d", len(pg.FetchSteps))
	}
	// Verify step IDs
	pgStepIDs := make(map[string]bool)
	for _, step := range pg.FetchSteps {
		pgStepIDs[step.ID] = true
	}
	for _, expected := range []string{"database_info", "replication_slots", "replication_stats", "connections", "tables"} {
		if !pgStepIDs[expected] {
			t.Errorf("missing postgresql fetch step: %s", expected)
		}
	}
	// Verify all steps use cli_args (not cli_command)
	for _, step := range pg.FetchSteps {
		if step.CLICommand != "" {
			t.Errorf("postgresql step %q should use cli_args, not cli_command", step.ID)
		}
		if len(step.CLIArgs) == 0 {
			t.Errorf("postgresql step %q should have cli_args", step.ID)
		}
	}
	// Verify optional steps
	for _, step := range pg.FetchSteps {
		switch step.ID {
		case "replication_stats", "connections", "tables":
			if !step.Optional {
				t.Errorf("postgresql step %q should be optional", step.ID)
			}
		case "database_info", "replication_slots":
			if step.Optional {
				t.Errorf("postgresql step %q should not be optional", step.ID)
			}
		}
	}
}

func TestLoadProviderConfigs_WithOverride(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "infra-providers")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create override that replaces kubectl with custom config
	override := []byte(`provider:
  id: kubectl
  name: "Custom Kubectl"
  description: "Custom kubectl provider"
  transport:
    primary: cli
    cli:
      command: "kubectl"
      timeout: "60s"
  defaults:
    namespace: "production"
    ttl: "10m"
  fetch_steps:
    - id: pods
      cli_command: "get pods -n {{.namespace}} -o json"
      mapping:
        topology:
          source_path: "items"
          each:
            kind: { value: "Pod" }
            name: { path: "metadata.name" }
            status: { path: "status.phase" }
`)
	if err := os.WriteFile(filepath.Join(overrideDir, "kubectl.yml"), override, 0644); err != nil {
		t.Fatal(err)
	}

	specs, err := LoadProviderConfigs(dir)
	if err != nil {
		t.Fatalf("failed to load configs: %v", err)
	}

	kubectl, ok := specs["kubectl"]
	if !ok {
		t.Fatal("expected kubectl provider")
	}

	// Should be the override version
	if kubectl.Name != "Custom Kubectl" {
		t.Errorf("name = %q, want Custom Kubectl", kubectl.Name)
	}
	if kubectl.Defaults.Namespace != "production" {
		t.Errorf("namespace = %q, want production", kubectl.Defaults.Namespace)
	}
	if len(kubectl.FetchSteps) != 1 {
		t.Errorf("expected 1 fetch step, got %d", len(kubectl.FetchSteps))
	}
}

func TestLoadProviderConfigs_AdditionalProvider(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "infra-providers")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Add a new provider via override
	newProvider := []byte(`provider:
  id: datadog
  name: "Datadog"
  description: "Datadog monitoring"
  transport:
    primary: mcp
    mcp:
      command: "npx"
      args: ["-y", "@datadog/mcp-server"]
  defaults:
    ttl: "5m"
  fetch_steps:
    - id: hosts
      mcp_tool: "list_hosts"
      mapping:
        topology:
          source_path: "host_list"
          each:
            kind: { value: "Host" }
            name: { path: "name" }
            status: { path: "status" }
`)
	if err := os.WriteFile(filepath.Join(overrideDir, "datadog.yml"), newProvider, 0644); err != nil {
		t.Fatal(err)
	}

	specs, err := LoadProviderConfigs(dir)
	if err != nil {
		t.Fatalf("failed to load configs: %v", err)
	}

	// Should have both kubectl (embedded) and datadog (override)
	if _, ok := specs["kubectl"]; !ok {
		t.Error("expected kubectl provider")
	}
	if _, ok := specs["datadog"]; !ok {
		t.Error("expected datadog provider")
	}
}

func TestLoadProviderConfig_Single(t *testing.T) {
	spec, err := LoadProviderConfig("kubectl", "")
	if err != nil {
		t.Fatalf("failed to load kubectl config: %v", err)
	}
	if spec.ID != "kubectl" {
		t.Errorf("id = %q, want kubectl", spec.ID)
	}
}

func TestLoadProviderConfig_NotFound(t *testing.T) {
	_, err := LoadProviderConfig("nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestValidate_Spec(t *testing.T) {
	tests := []struct {
		name    string
		spec    InfraProviderSpec
		wantErr bool
	}{
		{
			name: "valid",
			spec: InfraProviderSpec{
				ID:   "test",
				Name: "Test",
				Transport: InfraTransportSpec{
					Primary: "cli",
					CLI:     &InfraCLISpec{Command: "test-cmd"},
				},
				FetchSteps: []InfraFetchStep{
					{ID: "step1"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing id",
			spec: InfraProviderSpec{
				Name: "Test",
				Transport: InfraTransportSpec{
					Primary: "cli",
					CLI:     &InfraCLISpec{Command: "test"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			spec: InfraProviderSpec{
				ID: "test",
				Transport: InfraTransportSpec{
					Primary: "cli",
					CLI:     &InfraCLISpec{Command: "test"},
				},
			},
			wantErr: true,
		},
		{
			name: "valid http transport",
			spec: InfraProviderSpec{
				ID:   "test",
				Name: "Test",
				Transport: InfraTransportSpec{
					Primary: "http",
					HTTP:    &InfraHTTPSpec{BaseURL: "https://api.example.com"},
				},
				FetchSteps: []InfraFetchStep{
					{ID: "step1"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid transport primary",
			spec: InfraProviderSpec{
				ID:   "test",
				Name: "Test",
				Transport: InfraTransportSpec{
					Primary: "grpc",
				},
			},
			wantErr: true,
		},
		{
			name: "missing step id",
			spec: InfraProviderSpec{
				ID:   "test",
				Name: "Test",
				Transport: InfraTransportSpec{
					Primary: "cli",
					CLI:     &InfraCLISpec{Command: "test"},
				},
				FetchSteps: []InfraFetchStep{
					{ID: ""},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Defaults(t *testing.T) {
	d := InfraDefaultsSpec{}
	if err := d.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Namespace != "default" {
		t.Errorf("namespace = %q, want default", d.Namespace)
	}
	if d.TTL.Minutes() != 5 {
		t.Errorf("ttl = %v, want 5m", d.TTL)
	}

	d2 := InfraDefaultsSpec{TTLStr: "invalid"}
	if err := d2.Validate(); err == nil {
		t.Error("expected error for invalid TTL")
	}
}
