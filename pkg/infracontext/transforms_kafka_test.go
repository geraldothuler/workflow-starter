package infracontext

import (
	"testing"
)

// --- kafkaClustersToTopology ---

func TestKafkaClustersToTopology(t *testing.T) {
	items := []any{
		map[string]any{
			"id": "lkc-abc123",
			"spec": map[string]any{
				"display_name":  "prod-cluster",
				"availability":  "HIGH",
				"cloud":         "AWS",
				"region":        "us-east-1",
				"http_endpoint": "https://pkc-abc123.us-east-1.aws.confluent.cloud:443",
			},
			"status": map[string]any{
				"phase": "PROVISIONED",
			},
		},
		map[string]any{
			"id": "lkc-def456",
			"spec": map[string]any{
				"display_name": "staging-cluster",
				"availability": "LOW",
				"cloud":        "GCP",
				"region":       "us-central1",
			},
			"status": map[string]any{
				"phase": "PROVISIONED",
			},
		},
	}

	result, err := kafkaClustersToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodes := result.([]TopologyNode)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	if nodes[0].Kind != "KafkaCluster" {
		t.Errorf("kind = %q, want KafkaCluster", nodes[0].Kind)
	}
	if nodes[0].Name != "prod-cluster" {
		t.Errorf("name = %q, want prod-cluster", nodes[0].Name)
	}
	if nodes[0].Status != "PROVISIONED" {
		t.Errorf("status = %q, want PROVISIONED", nodes[0].Status)
	}
	if nodes[0].Metadata["cloud"] != "AWS" {
		t.Errorf("cloud = %q, want AWS", nodes[0].Metadata["cloud"])
	}
	if nodes[0].Metadata["region"] != "us-east-1" {
		t.Errorf("region = %q, want us-east-1", nodes[0].Metadata["region"])
	}
	if nodes[0].Metadata["availability"] != "HIGH" {
		t.Errorf("availability = %q, want HIGH", nodes[0].Metadata["availability"])
	}
	if nodes[0].Metadata["id"] != "lkc-abc123" {
		t.Errorf("id = %q, want lkc-abc123", nodes[0].Metadata["id"])
	}
}

func TestKafkaClustersToTopology_Empty(t *testing.T) {
	result, err := kafkaClustersToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestKafkaClustersToTopology_MissingID(t *testing.T) {
	items := []any{
		map[string]any{
			"spec": map[string]any{"display_name": "no-id-cluster"},
		},
	}
	result, err := kafkaClustersToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for missing id, got %d", len(nodes))
	}
}

func TestKafkaClustersToTopology_MissingDisplayName(t *testing.T) {
	items := []any{
		map[string]any{
			"id":   "lkc-xyz",
			"spec": map[string]any{},
		},
	}
	result, err := kafkaClustersToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	// Should fall back to ID as name
	if nodes[0].Name != "lkc-xyz" {
		t.Errorf("name = %q, want lkc-xyz (fallback to id)", nodes[0].Name)
	}
}

func TestKafkaClustersToTopology_InvalidItem(t *testing.T) {
	items := []any{"not-a-map", 42}
	result, err := kafkaClustersToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for invalid items, got %d", len(nodes))
	}
}

// --- kafkaBrokersToTopology ---

func TestKafkaBrokersToTopology(t *testing.T) {
	items := []any{
		map[string]any{
			"broker_id":  "0",
			"host":       "broker-0.example.com",
			"port":       "9092",
			"rack":       "az1",
			"cluster_id": "lkc-abc123",
		},
		map[string]any{
			"broker_id":  "1",
			"host":       "broker-1.example.com",
			"port":       "9092",
			"rack":       "az2",
			"cluster_id": "lkc-abc123",
		},
	}

	result, err := kafkaBrokersToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodes := result.([]TopologyNode)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	if nodes[0].Kind != "KafkaBroker" {
		t.Errorf("kind = %q, want KafkaBroker", nodes[0].Kind)
	}
	if nodes[0].Name != "broker-0.example.com" {
		t.Errorf("name = %q, want broker-0.example.com", nodes[0].Name)
	}
	if nodes[0].Metadata["broker_id"] != "0" {
		t.Errorf("broker_id = %q, want 0", nodes[0].Metadata["broker_id"])
	}
	if nodes[0].Metadata["rack"] != "az1" {
		t.Errorf("rack = %q, want az1", nodes[0].Metadata["rack"])
	}
	if nodes[0].Metadata["port"] != "9092" {
		t.Errorf("port = %q, want 9092", nodes[0].Metadata["port"])
	}
}

func TestKafkaBrokersToTopology_Empty(t *testing.T) {
	result, err := kafkaBrokersToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestKafkaBrokersToTopology_MissingBrokerID(t *testing.T) {
	items := []any{
		map[string]any{"host": "broker.example.com"},
	}
	result, err := kafkaBrokersToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestKafkaBrokersToTopology_NoHost(t *testing.T) {
	items := []any{
		map[string]any{"broker_id": "5"},
	}
	result, err := kafkaBrokersToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "broker-5" {
		t.Errorf("name = %q, want broker-5", nodes[0].Name)
	}
}

// --- kafkaTopicsToTopology ---

func TestKafkaTopicsToTopology(t *testing.T) {
	items := []any{
		map[string]any{
			"topic_name":         "orders",
			"partitions_count":   float64(12),
			"replication_factor": float64(3),
			"is_internal":        false,
			"cluster_id":         "lkc-abc123",
		},
		map[string]any{
			"topic_name":         "payments",
			"partitions_count":   float64(6),
			"replication_factor": float64(3),
			"is_internal":        false,
			"cluster_id":         "lkc-abc123",
		},
		map[string]any{
			"topic_name":         "__consumer_offsets",
			"partitions_count":   float64(50),
			"replication_factor": float64(3),
			"is_internal":        true,
			"cluster_id":         "lkc-abc123",
		},
	}

	result, err := kafkaTopicsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodes := result.([]TopologyNode)
	// Internal topics should be filtered out
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (internal filtered), got %d", len(nodes))
	}

	if nodes[0].Kind != "KafkaTopic" {
		t.Errorf("kind = %q, want KafkaTopic", nodes[0].Kind)
	}
	if nodes[0].Name != "orders" {
		t.Errorf("name = %q, want orders", nodes[0].Name)
	}
	if nodes[0].Metadata["partitions"] != "12" {
		t.Errorf("partitions = %q, want 12", nodes[0].Metadata["partitions"])
	}
	if nodes[0].Metadata["replication_factor"] != "3" {
		t.Errorf("replication_factor = %q, want 3", nodes[0].Metadata["replication_factor"])
	}
}

func TestKafkaTopicsToTopology_Empty(t *testing.T) {
	result, err := kafkaTopicsToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestKafkaTopicsToTopology_MissingName(t *testing.T) {
	items := []any{
		map[string]any{"partitions_count": float64(3)},
	}
	result, err := kafkaTopicsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestKafkaTopicsToTopology_AllInternal(t *testing.T) {
	items := []any{
		map[string]any{
			"topic_name":  "__consumer_offsets",
			"is_internal": true,
		},
		map[string]any{
			"topic_name":  "__transaction_state",
			"is_internal": true,
		},
	}
	result, err := kafkaTopicsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes (all internal), got %d", len(nodes))
	}
}

// --- kafkaConsumerGroupsToTopology ---

func TestKafkaConsumerGroupsToTopology(t *testing.T) {
	items := []any{
		map[string]any{
			"consumer_group_id": "order-processor",
			"state":             "STABLE",
			"cluster_id":        "lkc-abc123",
			"coordinator": map[string]any{
				"related": "https://broker-0:443/kafka/v3/clusters/lkc-abc123/brokers/0",
			},
		},
		map[string]any{
			"consumer_group_id": "payment-handler",
			"state":             "REBALANCING",
			"cluster_id":        "lkc-abc123",
		},
		map[string]any{
			"consumer_group_id": "dead-consumer",
			"state":             "DEAD",
			"cluster_id":        "lkc-abc123",
		},
	}

	result, err := kafkaConsumerGroupsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodes := result.([]TopologyNode)
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	if nodes[0].Kind != "ConsumerGroup" {
		t.Errorf("kind = %q, want ConsumerGroup", nodes[0].Kind)
	}
	if nodes[0].Name != "order-processor" {
		t.Errorf("name = %q, want order-processor", nodes[0].Name)
	}
	if nodes[0].Status != "Stable" {
		t.Errorf("status = %q, want Stable", nodes[0].Status)
	}

	if nodes[1].Status != "Rebalancing" {
		t.Errorf("status = %q, want Rebalancing", nodes[1].Status)
	}

	if nodes[2].Status != "Dead" {
		t.Errorf("status = %q, want Dead", nodes[2].Status)
	}
}

func TestKafkaConsumerGroupsToTopology_Empty(t *testing.T) {
	result, err := kafkaConsumerGroupsToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestKafkaConsumerGroupsToTopology_MissingGroupID(t *testing.T) {
	items := []any{
		map[string]any{"state": "STABLE"},
	}
	result, err := kafkaConsumerGroupsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

// --- kafkaConsumerGroupsToHealth ---

func TestKafkaConsumerGroupsToHealth(t *testing.T) {
	items := []any{
		map[string]any{
			"consumer_group_id": "order-processor",
			"state":             "STABLE",
		},
		map[string]any{
			"consumer_group_id": "payment-handler",
			"state":             "REBALANCING",
		},
		map[string]any{
			"consumer_group_id": "dead-consumer",
			"state":             "DEAD",
		},
	}

	result, err := kafkaConsumerGroupsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := result.([]HealthCheck)
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checks))
	}

	if checks[0].Status != HealthStatusHealthy {
		t.Errorf("STABLE status = %q, want healthy", checks[0].Status)
	}
	if checks[1].Status != HealthStatusDegraded {
		t.Errorf("REBALANCING status = %q, want degraded", checks[1].Status)
	}
	if checks[2].Status != HealthStatusUnhealthy {
		t.Errorf("DEAD status = %q, want unhealthy", checks[2].Status)
	}
}

func TestKafkaConsumerGroupsToHealth_Empty(t *testing.T) {
	result, err := kafkaConsumerGroupsToHealth([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if len(checks) != 0 {
		t.Errorf("expected 0 checks, got %d", len(checks))
	}
}

func TestKafkaConsumerGroupsToHealth_EmptyState(t *testing.T) {
	items := []any{
		map[string]any{
			"consumer_group_id": "group-empty",
			"state":             "EMPTY",
		},
	}
	result, err := kafkaConsumerGroupsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Status != HealthStatusDegraded {
		t.Errorf("EMPTY status = %q, want degraded", checks[0].Status)
	}
}

// --- kafkaConnectorsToTopology ---

func TestKafkaConnectorsToTopology(t *testing.T) {
	items := []any{
		map[string]any{
			"info": map[string]any{
				"name": "orders-s3-sink",
				"type": "sink",
				"config": map[string]any{
					"connector.class": "io.confluent.connect.s3.S3SinkConnector",
				},
			},
			"status": map[string]any{
				"connector": map[string]any{
					"state": "RUNNING",
				},
			},
		},
		map[string]any{
			"info": map[string]any{
				"name": "postgres-cdc-source",
				"type": "source",
			},
			"status": map[string]any{
				"connector": map[string]any{
					"state": "PAUSED",
				},
			},
		},
		map[string]any{
			"info": map[string]any{
				"name": "failed-connector",
				"type": "sink",
			},
			"status": map[string]any{
				"connector": map[string]any{
					"state": "FAILED",
				},
			},
		},
	}

	result, err := kafkaConnectorsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodes := result.([]TopologyNode)
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	if nodes[0].Kind != "KafkaConnector" {
		t.Errorf("kind = %q, want KafkaConnector", nodes[0].Kind)
	}
	if nodes[0].Name != "orders-s3-sink" {
		t.Errorf("name = %q, want orders-s3-sink", nodes[0].Name)
	}
	if nodes[0].Status != "Running" {
		t.Errorf("status = %q, want Running", nodes[0].Status)
	}
	if nodes[0].Metadata["type"] != "sink" {
		t.Errorf("type = %q, want sink", nodes[0].Metadata["type"])
	}
	if nodes[0].Metadata["class"] != "io.confluent.connect.s3.S3SinkConnector" {
		t.Errorf("class = %q, want S3SinkConnector", nodes[0].Metadata["class"])
	}

	if nodes[1].Status != "Paused" {
		t.Errorf("status = %q, want Paused", nodes[1].Status)
	}
	if nodes[1].Metadata["type"] != "source" {
		t.Errorf("type = %q, want source", nodes[1].Metadata["type"])
	}

	if nodes[2].Status != "Failed" {
		t.Errorf("status = %q, want Failed", nodes[2].Status)
	}
}

func TestKafkaConnectorsToTopology_Empty(t *testing.T) {
	result, err := kafkaConnectorsToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestKafkaConnectorsToTopology_FlatStructure(t *testing.T) {
	items := []any{
		map[string]any{
			"name": "flat-connector",
			"type": "sink",
			"status": map[string]any{
				"state": "RUNNING",
			},
		},
	}
	result, err := kafkaConnectorsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "flat-connector" {
		t.Errorf("name = %q, want flat-connector", nodes[0].Name)
	}
}

func TestKafkaConnectorsToTopology_MissingName(t *testing.T) {
	items := []any{
		map[string]any{
			"info": map[string]any{"type": "sink"},
		},
	}
	result, err := kafkaConnectorsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestKafkaConnectorsToTopology_InvalidItems(t *testing.T) {
	items := []any{"string", 42, nil}
	result, err := kafkaConnectorsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

// --- kafkaConnectorsToHealth ---

func TestKafkaConnectorsToHealth(t *testing.T) {
	items := []any{
		map[string]any{
			"info": map[string]any{"name": "running-conn"},
			"status": map[string]any{
				"connector": map[string]any{"state": "RUNNING"},
			},
		},
		map[string]any{
			"info": map[string]any{"name": "paused-conn"},
			"status": map[string]any{
				"connector": map[string]any{"state": "PAUSED"},
			},
		},
		map[string]any{
			"info": map[string]any{"name": "failed-conn"},
			"status": map[string]any{
				"connector": map[string]any{"state": "FAILED"},
			},
		},
	}

	result, err := kafkaConnectorsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := result.([]HealthCheck)
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checks))
	}

	if checks[0].Status != HealthStatusHealthy {
		t.Errorf("RUNNING status = %q, want healthy", checks[0].Status)
	}
	if checks[0].Component != "running-conn" {
		t.Errorf("component = %q, want running-conn", checks[0].Component)
	}

	if checks[1].Status != HealthStatusDegraded {
		t.Errorf("PAUSED status = %q, want degraded", checks[1].Status)
	}

	if checks[2].Status != HealthStatusUnhealthy {
		t.Errorf("FAILED status = %q, want unhealthy", checks[2].Status)
	}
}

func TestKafkaConnectorsToHealth_Empty(t *testing.T) {
	result, err := kafkaConnectorsToHealth([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if len(checks) != 0 {
		t.Errorf("expected 0 checks, got %d", len(checks))
	}
}

func TestKafkaConnectorsToHealth_Unassigned(t *testing.T) {
	items := []any{
		map[string]any{
			"info": map[string]any{"name": "unassigned-conn"},
			"status": map[string]any{
				"connector": map[string]any{"state": "UNASSIGNED"},
			},
		},
	}
	result, err := kafkaConnectorsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Status != HealthStatusDegraded {
		t.Errorf("UNASSIGNED status = %q, want degraded", checks[0].Status)
	}
}

// --- State mapping helpers ---

func TestMapKafkaGroupState(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"STABLE", "Stable"},
		{"REBALANCING", "Rebalancing"},
		{"PREPARING_REBALANCE", "Rebalancing"},
		{"COMPLETING_REBALANCE", "Rebalancing"},
		{"DEAD", "Dead"},
		{"EMPTY", "Empty"},
		{"", "Unknown"},
		{"UNKNOWN_STATE", "Unknown"},
	}

	for _, tt := range tests {
		got := mapKafkaGroupState(tt.state)
		if got != tt.want {
			t.Errorf("mapKafkaGroupState(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestMapKafkaGroupStateToHealth(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"STABLE", HealthStatusHealthy},
		{"REBALANCING", HealthStatusDegraded},
		{"DEAD", HealthStatusUnhealthy},
		{"EMPTY", HealthStatusDegraded},
		{"", HealthStatusUnknown},
	}

	for _, tt := range tests {
		got := mapKafkaGroupStateToHealth(tt.state)
		if got != tt.want {
			t.Errorf("mapKafkaGroupStateToHealth(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestMapKafkaConnectorState(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"RUNNING", "Running"},
		{"PAUSED", "Paused"},
		{"FAILED", "Failed"},
		{"UNASSIGNED", "Unassigned"},
		{"", "Unknown"},
	}

	for _, tt := range tests {
		got := mapKafkaConnectorState(tt.state)
		if got != tt.want {
			t.Errorf("mapKafkaConnectorState(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestMapKafkaConnectorStateToHealth(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"RUNNING", HealthStatusHealthy},
		{"PAUSED", HealthStatusDegraded},
		{"FAILED", HealthStatusUnhealthy},
		{"UNASSIGNED", HealthStatusDegraded},
		{"", HealthStatusUnknown},
	}

	for _, tt := range tests {
		got := mapKafkaConnectorStateToHealth(tt.state)
		if got != tt.want {
			t.Errorf("mapKafkaConnectorStateToHealth(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
