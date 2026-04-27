package playbook

import (
	"testing"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
)

// --- analyze_inactive_slots ---

func TestAnalyzeInactiveSlots_NoFindings(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "slot1", Kind: "ReplicationSlot", Status: infracontext.HealthStatusHealthy},
			{Component: "deploy1", Kind: "Deployment", Status: infracontext.HealthStatusHealthy},
		},
	}
	findings, err := CallAnalyzer("analyze_inactive_slots", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestAnalyzeInactiveSlots_Detected(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "debezium_slot", Kind: "ReplicationSlot", Status: infracontext.HealthStatusUnhealthy},
			{Component: "active_slot", Kind: "ReplicationSlot", Status: infracontext.HealthStatusHealthy},
		},
		Topology: []infracontext.TopologyNode{
			{
				Kind: "ReplicationSlot", Name: "debezium_slot",
				Metadata: map[string]string{"wal_lag_bytes": "524288000", "wal_status": "reserved"},
			},
		},
	}
	findings, err := CallAnalyzer("analyze_inactive_slots", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != SeverityCritical {
		t.Errorf("severity = %q, want critical", f.Severity)
	}
	if f.AnalyzerName != "analyze_inactive_slots" {
		t.Errorf("analyzer = %q", f.AnalyzerName)
	}
	if !containsStr(f.Evidence, "wal_lag_bytes=524288000") {
		t.Errorf("evidence should contain wal_lag_bytes, got: %s", f.Evidence)
	}
}

// --- analyze_wal_lag ---

func TestAnalyzeWALLag_NoFindings(t *testing.T) {
	ic := &infracontext.InfraContext{
		Topology: []infracontext.TopologyNode{
			{Kind: "WALSender", Name: "sender1", Metadata: map[string]string{"replay_lag_bytes": "1024"}},
		},
	}
	findings, err := CallAnalyzer("analyze_wal_lag", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestAnalyzeWALLag_Detected(t *testing.T) {
	ic := &infracontext.InfraContext{
		Topology: []infracontext.TopologyNode{
			{Kind: "WALSender", Name: "sender1", Metadata: map[string]string{"replay_lag_bytes": "2147483648"}}, // 2GB
			{Kind: "WALSender", Name: "sender2", Metadata: map[string]string{"replay_lag_bytes": "209715200"}},  // 200MB
		},
	}
	findings, err := CallAnalyzer("analyze_wal_lag", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	// sender1 should be critical (2GB > 1GB threshold)
	var critCount, warnCount int
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			critCount++
		}
		if f.Severity == SeverityWarning {
			warnCount++
		}
	}
	if critCount != 1 {
		t.Errorf("expected 1 critical finding, got %d", critCount)
	}
	if warnCount != 1 {
		t.Errorf("expected 1 warning finding, got %d", warnCount)
	}
}

// --- analyze_connection_saturation ---

func TestAnalyzeConnectionSaturation_NoFindings(t *testing.T) {
	ic := &infracontext.InfraContext{
		Metrics: []infracontext.Metric{
			{Name: "connection_utilization_pct", Value: 50.0, Labels: map[string]string{"database": "mydb"}},
		},
	}
	findings, err := CallAnalyzer("analyze_connection_saturation", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestAnalyzeConnectionSaturation_Detected(t *testing.T) {
	ic := &infracontext.InfraContext{
		Metrics: []infracontext.Metric{
			{Name: "connection_utilization_pct", Value: 97.5, Labels: map[string]string{"database": "production"}},
			{Name: "connection_utilization_pct", Value: 85.0, Labels: map[string]string{"database": "analytics"}},
			{Name: "cpu_usage", Value: 50.0}, // should be ignored
		},
	}
	findings, err := CallAnalyzer("analyze_connection_saturation", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	var critCount, warnCount int
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			critCount++
		}
		if f.Severity == SeverityWarning {
			warnCount++
		}
	}
	if critCount != 1 {
		t.Errorf("expected 1 critical, got %d", critCount)
	}
	if warnCount != 1 {
		t.Errorf("expected 1 warning, got %d", warnCount)
	}
}

// --- analyze_failed_connectors ---

func TestAnalyzeFailedConnectors_NoFindings(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "cdc-source", Kind: "KafkaConnector", Status: infracontext.HealthStatusHealthy},
		},
	}
	findings, err := CallAnalyzer("analyze_failed_connectors", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestAnalyzeFailedConnectors_Detected(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "cdc-source", Kind: "KafkaConnector", Status: infracontext.HealthStatusUnhealthy, Message: "task failed"},
			{Component: "sink", Kind: "KafkaConnector", Status: infracontext.HealthStatusHealthy},
		},
	}
	findings, err := CallAnalyzer("analyze_failed_connectors", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("severity = %q, want critical", findings[0].Severity)
	}
}

// --- analyze_empty_consumer_groups ---

func TestAnalyzeEmptyConsumerGroups_NoFindings(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "cg1", Kind: "ConsumerGroup", Status: infracontext.HealthStatusHealthy},
		},
	}
	findings, err := CallAnalyzer("analyze_empty_consumer_groups", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestAnalyzeEmptyConsumerGroups_Detected(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "cdc-consumers", Kind: "ConsumerGroup", Status: infracontext.HealthStatusDegraded},
			{Component: "active-cg", Kind: "ConsumerGroup", Status: infracontext.HealthStatusHealthy},
		},
	}
	findings, err := CallAnalyzer("analyze_empty_consumer_groups", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("severity = %q, want warning", findings[0].Severity)
	}
}

// --- analyze_unhealthy_pods ---

func TestAnalyzeUnhealthyPods_NoFindings(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "airbyte-worker-abc", Kind: "Pod", Status: infracontext.HealthStatusHealthy},
			{Component: "nginx-xyz", Kind: "Pod", Status: infracontext.HealthStatusUnhealthy}, // no match on pattern
		},
	}
	args := map[string]any{"name_patterns": []any{"airbyte", "debezium", "cdc"}}
	findings, err := CallAnalyzer("analyze_unhealthy_pods", ic, args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (nginx doesn't match patterns), got %d", len(findings))
	}
}

func TestAnalyzeUnhealthyPods_Detected(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "airbyte-worker-abc", Kind: "Pod", Status: infracontext.HealthStatusUnhealthy, Message: "CrashLoopBackOff"},
			{Component: "debezium-connect-xyz", Kind: "Pod", Status: infracontext.HealthStatusDegraded, Message: "pending"},
			{Component: "nginx-frontend", Kind: "Pod", Status: infracontext.HealthStatusUnhealthy}, // no match
		},
	}
	args := map[string]any{"name_patterns": []any{"airbyte", "debezium", "cdc"}}
	findings, err := CallAnalyzer("analyze_unhealthy_pods", ic, args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestAnalyzeUnhealthyPods_NoPatterns(t *testing.T) {
	ic := &infracontext.InfraContext{
		Health: []infracontext.HealthCheck{
			{Component: "any-pod", Kind: "Pod", Status: infracontext.HealthStatusUnhealthy},
		},
	}
	// No patterns = match all unhealthy pods
	findings, err := CallAnalyzer("analyze_unhealthy_pods", ic, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Errorf("expected 1 finding (no patterns = match all), got %d", len(findings))
	}
}

// --- Helpers ---

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{500, "500 B"},
		{2048, "2.0 KB"},
		{104857600, "100.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractStringSlice(t *testing.T) {
	// []any (from YAML unmarshaling)
	args := map[string]any{"patterns": []any{"a", "b", "c"}}
	got := extractStringSlice(args, "patterns")
	if len(got) != 3 {
		t.Errorf("expected 3 items, got %d", len(got))
	}

	// []string
	args2 := map[string]any{"patterns": []string{"x", "y"}}
	got2 := extractStringSlice(args2, "patterns")
	if len(got2) != 2 {
		t.Errorf("expected 2 items, got %d", len(got2))
	}

	// nil args
	got3 := extractStringSlice(nil, "patterns")
	if got3 != nil {
		t.Errorf("expected nil for nil args")
	}
}

// Suppress unused import warning for time
var _ = time.Now
