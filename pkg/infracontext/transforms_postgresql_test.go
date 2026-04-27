package infracontext

import (
	"testing"
)

// --- pg_database_info_to_topology ---

func TestPgDatabaseInfoToTopology_Basic(t *testing.T) {
	items := []any{
		map[string]any{
			"name":               "fusca_production",
			"version":            "PostgreSQL 15.4",
			"started_at":         "2026-02-20T10:30:00+00:00",
			"size_bytes":         5.36870912e9,
			"active_connections": float64(42),
			"max_connections":    float64(200),
		},
	}
	result, err := pgDatabaseInfoToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.Kind != "PostgresDatabase" {
		t.Errorf("expected kind PostgresDatabase, got %q", n.Kind)
	}
	if n.Name != "fusca_production" {
		t.Errorf("expected name fusca_production, got %q", n.Name)
	}
	if n.Status != "running" {
		t.Errorf("expected status running, got %q", n.Status)
	}
	if n.Metadata["version"] != "PostgreSQL 15.4" {
		t.Errorf("expected version PostgreSQL 15.4, got %q", n.Metadata["version"])
	}
	if n.Metadata["active_connections"] != "42" {
		t.Errorf("expected active_connections 42, got %q", n.Metadata["active_connections"])
	}
}

func TestPgDatabaseInfoToTopology_Empty(t *testing.T) {
	result, err := pgDatabaseInfoToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}

// --- pg_database_info_to_metrics ---

func TestPgDatabaseInfoToMetrics_Basic(t *testing.T) {
	items := []any{
		map[string]any{
			"name":               "mydb",
			"active_connections": float64(50),
			"max_connections":    float64(200),
			"size_bytes":         float64(1073741824),
		},
	}
	result, err := pgDatabaseInfoToMetrics(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)
	// Expect: active_connections, max_connections, database_size_bytes, connection_utilization_pct
	if len(metrics) != 4 {
		t.Fatalf("expected 4 metrics, got %d", len(metrics))
	}

	byName := make(map[string]Metric)
	for _, m := range metrics {
		byName[m.Name] = m
	}

	if m, ok := byName["active_connections"]; !ok || m.Value != 50 {
		t.Errorf("expected active_connections=50, got %v", byName["active_connections"])
	}
	if m, ok := byName["connection_utilization_pct"]; !ok || m.Value != 25 {
		t.Errorf("expected connection_utilization_pct=25, got %v", m.Value)
	}
}

func TestPgDatabaseInfoToMetrics_ZeroMaxConnections(t *testing.T) {
	items := []any{
		map[string]any{
			"name":               "testdb",
			"active_connections": float64(5),
			"max_connections":    float64(0),
			"size_bytes":         float64(1024),
		},
	}
	result, err := pgDatabaseInfoToMetrics(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)
	// Should not include utilization_pct when max_connections=0
	for _, m := range metrics {
		if m.Name == "connection_utilization_pct" {
			t.Error("should not include utilization when max_connections=0")
		}
	}
}

func TestPgDatabaseInfoToMetrics_Empty(t *testing.T) {
	result, err := pgDatabaseInfoToMetrics([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for empty input, got %d", len(metrics))
	}
}

// --- pg_replication_slots_to_topology ---

func TestPgReplicationSlotsToTopology_Basic(t *testing.T) {
	items := []any{
		map[string]any{
			"slot_name":           "cdc_slot",
			"plugin":              "pgoutput",
			"slot_type":           "logical",
			"active":              true,
			"restart_lsn":         "0/F000000",
			"confirmed_flush_lsn": "0/F000100",
			"wal_status":          "reserved",
			"slot_lag_bytes":      float64(1024),
		},
		map[string]any{
			"slot_name":           "broken_slot",
			"plugin":              "pgoutput",
			"slot_type":           "logical",
			"active":              false,
			"restart_lsn":         "0/A000000",
			"confirmed_flush_lsn": "0/A000000",
			"wal_status":          "reserved",
			"slot_lag_bytes":      float64(2147483648),
		},
	}
	result, err := pgReplicationSlotsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	if nodes[0].Name != "cdc_slot" || nodes[0].Status != "active" {
		t.Errorf("expected cdc_slot/active, got %s/%s", nodes[0].Name, nodes[0].Status)
	}
	if nodes[0].Kind != "ReplicationSlot" {
		t.Errorf("expected kind ReplicationSlot, got %q", nodes[0].Kind)
	}
	if nodes[0].Metadata["plugin"] != "pgoutput" {
		t.Errorf("expected plugin pgoutput, got %q", nodes[0].Metadata["plugin"])
	}

	if nodes[1].Name != "broken_slot" || nodes[1].Status != "inactive" {
		t.Errorf("expected broken_slot/inactive, got %s/%s", nodes[1].Name, nodes[1].Status)
	}
}

func TestPgReplicationSlotsToTopology_Empty(t *testing.T) {
	result, err := pgReplicationSlotsToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}

// --- pg_replication_slots_to_health ---

func TestPgReplicationSlotsToHealth_Active(t *testing.T) {
	items := []any{
		map[string]any{
			"slot_name":      "healthy_slot",
			"active":         true,
			"slot_lag_bytes": float64(512),
		},
	}
	result, err := pgReplicationSlotsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Status != HealthStatusHealthy {
		t.Errorf("expected healthy, got %q", checks[0].Status)
	}
	if checks[0].Kind != "ReplicationSlot" {
		t.Errorf("expected kind ReplicationSlot, got %q", checks[0].Kind)
	}
}

func TestPgReplicationSlotsToHealth_InactiveHighLag(t *testing.T) {
	items := []any{
		map[string]any{
			"slot_name":      "broken_cdc",
			"active":         false,
			"slot_lag_bytes": float64(2147483648), // 2 GB
		},
	}
	result, err := pgReplicationSlotsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if checks[0].Status != HealthStatusUnhealthy {
		t.Errorf("expected unhealthy for 2GB lag, got %q", checks[0].Status)
	}
	if checks[0].Component != "broken_cdc" {
		t.Errorf("expected component broken_cdc, got %q", checks[0].Component)
	}
}

func TestPgReplicationSlotsToHealth_InactiveMediumLag(t *testing.T) {
	items := []any{
		map[string]any{
			"slot_name":      "degraded_slot",
			"active":         false,
			"slot_lag_bytes": float64(524288000), // ~500 MB
		},
	}
	result, err := pgReplicationSlotsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if checks[0].Status != HealthStatusDegraded {
		t.Errorf("expected degraded for 500MB lag, got %q", checks[0].Status)
	}
}

func TestPgReplicationSlotsToHealth_InactiveNoLag(t *testing.T) {
	items := []any{
		map[string]any{
			"slot_name":      "old_slot",
			"active":         false,
			"slot_lag_bytes": float64(0),
		},
	}
	result, err := pgReplicationSlotsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if checks[0].Status != HealthStatusDegraded {
		t.Errorf("expected degraded for inactive slot with no lag, got %q", checks[0].Status)
	}
}

func TestPgReplicationSlotsToHealth_Empty(t *testing.T) {
	result, err := pgReplicationSlotsToHealth([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if len(checks) != 0 {
		t.Errorf("expected 0 checks, got %d", len(checks))
	}
}

func TestPgReplicationSlotsToHealth_ThresholdBoundary(t *testing.T) {
	tests := []struct {
		name       string
		lagBytes   float64
		wantStatus string
	}{
		{"exactly 100MB", float64(100 * 1024 * 1024), HealthStatusDegraded}, // at boundary, not over
		{"just above 100MB", float64(100*1024*1024 + 1), HealthStatusDegraded},
		{"exactly 1GB", float64(1024 * 1024 * 1024), HealthStatusDegraded}, // at boundary, not over
		{"just above 1GB", float64(1024*1024*1024 + 1), HealthStatusUnhealthy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := []any{
				map[string]any{
					"slot_name":      "test_slot",
					"active":         false,
					"slot_lag_bytes": tt.lagBytes,
				},
			}
			result, err := pgReplicationSlotsToHealth(items, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			checks := result.([]HealthCheck)
			if checks[0].Status != tt.wantStatus {
				t.Errorf("lag=%.0f: expected %q, got %q", tt.lagBytes, tt.wantStatus, checks[0].Status)
			}
		})
	}
}

// --- pg_replication_stats_to_topology ---

func TestPgReplicationStatsToTopology_Basic(t *testing.T) {
	items := []any{
		map[string]any{
			"application_name": "debezium",
			"client_addr":      "10.0.1.50",
			"state":            "streaming",
			"sync_state":       "async",
			"sent_lsn":         "0/F000100",
			"replay_lsn":       "0/F000100",
			"replay_lag_bytes":  float64(0),
		},
	}
	result, err := pgReplicationStatsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Kind != "WALSender" {
		t.Errorf("expected kind WALSender, got %q", nodes[0].Kind)
	}
	if nodes[0].Name != "debezium" {
		t.Errorf("expected name debezium, got %q", nodes[0].Name)
	}
	if nodes[0].Status != "streaming" {
		t.Errorf("expected status streaming, got %q", nodes[0].Status)
	}
}

func TestPgReplicationStatsToTopology_FallbackToClientAddr(t *testing.T) {
	items := []any{
		map[string]any{
			"application_name": "",
			"client_addr":      "10.0.2.10",
			"state":            "catchup",
			"sync_state":       "async",
		},
	}
	result, err := pgReplicationStatsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if nodes[0].Name != "10.0.2.10" {
		t.Errorf("expected fallback to client_addr, got %q", nodes[0].Name)
	}
}

func TestPgReplicationStatsToTopology_Empty(t *testing.T) {
	result, err := pgReplicationStatsToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

// --- pg_replication_stats_to_health ---

func TestPgReplicationStatsToHealth_Streaming(t *testing.T) {
	items := []any{
		map[string]any{
			"application_name": "debezium",
			"client_addr":      "10.0.1.50",
			"state":            "streaming",
		},
	}
	result, err := pgReplicationStatsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if checks[0].Status != HealthStatusHealthy {
		t.Errorf("expected healthy for streaming, got %q", checks[0].Status)
	}
}

func TestPgReplicationStatsToHealth_Catchup(t *testing.T) {
	items := []any{
		map[string]any{
			"application_name": "standby",
			"client_addr":      "10.0.2.10",
			"state":            "catchup",
		},
	}
	result, err := pgReplicationStatsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if checks[0].Status != HealthStatusDegraded {
		t.Errorf("expected degraded for catchup, got %q", checks[0].Status)
	}
}

func TestPgReplicationStatsToHealth_Startup(t *testing.T) {
	items := []any{
		map[string]any{
			"application_name": "new_replica",
			"client_addr":      "10.0.3.10",
			"state":            "startup",
		},
	}
	result, err := pgReplicationStatsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if checks[0].Status != HealthStatusUnhealthy {
		t.Errorf("expected unhealthy for startup, got %q", checks[0].Status)
	}
}

func TestPgReplicationStatsToHealth_UnknownState(t *testing.T) {
	items := []any{
		map[string]any{
			"application_name": "mystery",
			"client_addr":      "10.0.4.10",
			"state":            "some_new_state",
		},
	}
	result, err := pgReplicationStatsToHealth(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if checks[0].Status != HealthStatusUnknown {
		t.Errorf("expected unknown for unknown state, got %q", checks[0].Status)
	}
}

func TestPgReplicationStatsToHealth_Empty(t *testing.T) {
	result, err := pgReplicationStatsToHealth([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := result.([]HealthCheck)
	if len(checks) != 0 {
		t.Errorf("expected 0 checks, got %d", len(checks))
	}
}

// --- pg_connections_to_topology ---

func TestPgConnectionsToTopology_Basic(t *testing.T) {
	items := []any{
		map[string]any{
			"application_name": "fusca-api",
			"client_addr":      "10.0.1.10",
			"state":            "active",
		},
		map[string]any{
			"application_name": "fusca-api",
			"client_addr":      "10.0.1.11",
			"state":            "idle",
		},
		map[string]any{
			"application_name": "metabase",
			"client_addr":      "10.0.3.5",
			"state":            "active",
		},
	}
	result, err := pgConnectionsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 groups (fusca-api, metabase), got %d", len(nodes))
	}

	byName := make(map[string]TopologyNode)
	for _, n := range nodes {
		byName[n.Name] = n
	}

	if n, ok := byName["fusca-api"]; ok {
		if n.Kind != "Connection" {
			t.Errorf("expected kind Connection, got %q", n.Kind)
		}
		if n.Metadata["connection_count"] != "2" {
			t.Errorf("expected connection_count=2, got %q", n.Metadata["connection_count"])
		}
	} else {
		t.Error("expected fusca-api group")
	}

	if n, ok := byName["metabase"]; ok {
		if n.Metadata["connection_count"] != "1" {
			t.Errorf("expected connection_count=1, got %q", n.Metadata["connection_count"])
		}
	} else {
		t.Error("expected metabase group")
	}
}

func TestPgConnectionsToTopology_NoAppName(t *testing.T) {
	items := []any{
		map[string]any{
			"application_name": "",
			"client_addr":      "10.0.5.1",
			"state":            "idle",
		},
	}
	result, err := pgConnectionsToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "10.0.5.1" {
		t.Errorf("expected fallback to client_addr, got %q", nodes[0].Name)
	}
}

func TestPgConnectionsToTopology_Empty(t *testing.T) {
	result, err := pgConnectionsToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

// --- pg_connections_to_metrics ---

func TestPgConnectionsToMetrics_Basic(t *testing.T) {
	items := []any{
		map[string]any{"state": "active", "state_duration_seconds": float64(0.5)},
		map[string]any{"state": "active", "state_duration_seconds": float64(90.0)},
		map[string]any{"state": "idle", "state_duration_seconds": float64(120.0)},
		map[string]any{"state": "idle_in_transaction", "state_duration_seconds": float64(45.0)},
	}
	result, err := pgConnectionsToMetrics(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)

	byName := make(map[string]Metric)
	for _, m := range metrics {
		byName[m.Name] = m
	}

	if m, ok := byName["connections_total"]; !ok || m.Value != 4 {
		t.Errorf("expected total=4, got %v", m.Value)
	}
	if m, ok := byName["connections_long_running"]; !ok || m.Value != 1 {
		t.Errorf("expected long_running=1 (only active >60s), got %v", m.Value)
	}
	if m, ok := byName["connections_active"]; !ok || m.Value != 2 {
		t.Errorf("expected active=2, got %v", m.Value)
	}
}

func TestPgConnectionsToMetrics_Empty(t *testing.T) {
	result, err := pgConnectionsToMetrics([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)
	// Should still have total and long_running (both 0)
	byName := make(map[string]Metric)
	for _, m := range metrics {
		byName[m.Name] = m
	}
	if m := byName["connections_total"]; m.Value != 0 {
		t.Errorf("expected total=0, got %v", m.Value)
	}
}

func TestPgConnectionsToMetrics_NoLongRunning(t *testing.T) {
	items := []any{
		map[string]any{"state": "active", "state_duration_seconds": float64(10.0)},
		map[string]any{"state": "active", "state_duration_seconds": float64(30.0)},
	}
	result, err := pgConnectionsToMetrics(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)
	byName := make(map[string]Metric)
	for _, m := range metrics {
		byName[m.Name] = m
	}
	if m := byName["connections_long_running"]; m.Value != 0 {
		t.Errorf("expected long_running=0 (all under 60s), got %v", m.Value)
	}
}

// --- pg_tables_to_topology ---

func TestPgTablesToTopology_Basic(t *testing.T) {
	items := []any{
		map[string]any{
			"schemaname":       "public",
			"relname":          "orders",
			"n_live_tup":       float64(1200000),
			"n_dead_tup":       float64(15000),
			"last_vacuum":      "2026-02-21T03:00:00+00:00",
			"last_autovacuum":  "2026-02-22T02:00:00+00:00",
			"total_size_bytes": float64(2147483648),
		},
		map[string]any{
			"schemaname":       "audit",
			"relname":          "change_log",
			"n_live_tup":       float64(10000000),
			"n_dead_tup":       float64(0),
			"last_vacuum":      "2026-02-22T04:00:00+00:00",
			"last_autovacuum":  "2026-02-22T04:00:00+00:00",
			"total_size_bytes": float64(8589934592),
		},
	}
	result, err := pgTablesToTopology(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	if nodes[0].Name != "public.orders" {
		t.Errorf("expected public.orders, got %q", nodes[0].Name)
	}
	if nodes[0].Kind != "Table" {
		t.Errorf("expected kind Table, got %q", nodes[0].Kind)
	}
	if nodes[0].Metadata["schema"] != "public" {
		t.Errorf("expected schema public, got %q", nodes[0].Metadata["schema"])
	}

	if nodes[1].Name != "audit.change_log" {
		t.Errorf("expected audit.change_log, got %q", nodes[1].Name)
	}
}

func TestPgTablesToTopology_Empty(t *testing.T) {
	result, err := pgTablesToTopology([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes := result.([]TopologyNode)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

// --- pg_tables_to_metrics ---

func TestPgTablesToMetrics_Basic(t *testing.T) {
	items := []any{
		map[string]any{
			"schemaname":       "public",
			"relname":          "orders",
			"n_live_tup":       float64(1000),
			"n_dead_tup":       float64(100),
			"total_size_bytes": float64(1048576),
		},
		map[string]any{
			"schemaname":       "public",
			"relname":          "users",
			"n_live_tup":       float64(500),
			"n_dead_tup":       float64(50),
			"total_size_bytes": float64(524288),
		},
	}
	result, err := pgTablesToMetrics(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)

	byName := make(map[string]Metric)
	for _, m := range metrics {
		if m.Labels == nil {
			byName[m.Name] = m
		}
	}

	if m := byName["total_tables"]; m.Value != 2 {
		t.Errorf("expected total_tables=2, got %v", m.Value)
	}
	if m := byName["total_dead_tuples"]; m.Value != 150 {
		t.Errorf("expected total_dead_tuples=150, got %v", m.Value)
	}
	if m := byName["total_database_size"]; m.Value != 1572864 {
		t.Errorf("expected total_database_size=1572864, got %v", m.Value)
	}
}

func TestPgTablesToMetrics_DeadTupleRatio(t *testing.T) {
	items := []any{
		map[string]any{
			"schemaname":       "public",
			"relname":          "bloated",
			"n_live_tup":       float64(900),
			"n_dead_tup":       float64(100),
			"total_size_bytes": float64(1024),
		},
	}
	result, err := pgTablesToMetrics(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)

	var deadRatio float64
	for _, m := range metrics {
		if m.Name == "dead_tuple_ratio" && m.Labels["table"] == "public.bloated" {
			deadRatio = m.Value
		}
	}
	// 100 / (900 + 100) * 100 = 10%
	if deadRatio != 10 {
		t.Errorf("expected dead_tuple_ratio=10%%, got %v%%", deadRatio)
	}
}

func TestPgTablesToMetrics_ZeroTuples(t *testing.T) {
	items := []any{
		map[string]any{
			"schemaname":       "public",
			"relname":          "empty_table",
			"n_live_tup":       float64(0),
			"n_dead_tup":       float64(0),
			"total_size_bytes": float64(8192),
		},
	}
	result, err := pgTablesToMetrics(items, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)
	// Should not include dead_tuple_ratio when live+dead=0
	for _, m := range metrics {
		if m.Name == "dead_tuple_ratio" && m.Labels["table"] == "public.empty_table" {
			t.Error("should not include dead_tuple_ratio when live+dead=0")
		}
	}
}

func TestPgTablesToMetrics_Empty(t *testing.T) {
	result, err := pgTablesToMetrics([]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metrics := result.([]Metric)
	// Should still have aggregate metrics
	byName := make(map[string]Metric)
	for _, m := range metrics {
		if m.Labels == nil {
			byName[m.Name] = m
		}
	}
	if m := byName["total_tables"]; m.Value != 0 {
		t.Errorf("expected total_tables=0, got %v", m.Value)
	}
}

// --- Helper tests ---

func TestMapReplicationSlotHealth(t *testing.T) {
	tests := []struct {
		name       string
		active     bool
		lagBytes   float64
		wantStatus string
	}{
		{"active healthy", true, 0, HealthStatusHealthy},
		{"active with lag", true, 5368709120, HealthStatusHealthy},
		{"inactive no lag", false, 0, HealthStatusDegraded},
		{"inactive low lag", false, 50000000, HealthStatusDegraded},
		{"inactive high lag", false, 2147483648, HealthStatusUnhealthy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := mapReplicationSlotHealth(tt.active, tt.lagBytes)
			if status != tt.wantStatus {
				t.Errorf("expected %q, got %q", tt.wantStatus, status)
			}
		})
	}
}

func TestMapReplicationState(t *testing.T) {
	tests := []struct {
		state      string
		wantStatus string
	}{
		{"streaming", HealthStatusHealthy},
		{"catchup", HealthStatusDegraded},
		{"startup", HealthStatusUnhealthy},
		{"backup", HealthStatusUnhealthy},
		{"something_else", HealthStatusUnknown},
		{"", HealthStatusUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			status, _ := mapReplicationState(tt.state)
			if status != tt.wantStatus {
				t.Errorf("state=%q: expected %q, got %q", tt.state, tt.wantStatus, status)
			}
		})
	}
}

func TestExtractFloat(t *testing.T) {
	tests := []struct {
		name string
		item any
		want float64
	}{
		{"float64", map[string]any{"val": float64(42.5)}, 42.5},
		{"int", map[string]any{"val": 10}, 10},
		{"string", map[string]any{"val": "3.14"}, 3.14},
		{"nil", map[string]any{}, 0},
		{"missing field", map[string]any{"other": 1}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFloat(tt.item, "val")
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestExtractBool(t *testing.T) {
	tests := []struct {
		name string
		item any
		want bool
	}{
		{"true", map[string]any{"val": true}, true},
		{"false", map[string]any{"val": false}, false},
		{"string true", map[string]any{"val": "true"}, true},
		{"string t", map[string]any{"val": "t"}, true},
		{"string false", map[string]any{"val": "false"}, false},
		{"nil", map[string]any{}, false},
		{"missing", map[string]any{"other": true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBool(tt.item, "val")
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
