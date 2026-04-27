package playbook

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
)

func init() {
	RegisterAnalyzer("analyze_inactive_slots", analyzeInactiveSlots)
	RegisterAnalyzer("analyze_wal_lag", analyzeWALLag)
	RegisterAnalyzer("analyze_connection_saturation", analyzeConnectionSaturation)
	RegisterAnalyzer("analyze_failed_connectors", analyzeFailedConnectors)
	RegisterAnalyzer("analyze_empty_consumer_groups", analyzeEmptyConsumerGroups)
	RegisterAnalyzer("analyze_unhealthy_pods", analyzeUnhealthyPods)
}

// analyzeInactiveSlots detects inactive PostgreSQL replication slots.
// Looks at Health entries with Kind=ReplicationSlot and status != healthy.
func analyzeInactiveSlots(ic *infracontext.InfraContext, args map[string]any, acc []Finding) ([]Finding, error) {
	var findings []Finding
	for _, h := range ic.Health {
		if h.Kind != "ReplicationSlot" {
			continue
		}
		if h.Status == infracontext.HealthStatusHealthy {
			continue
		}

		// Extract evidence from topology metadata
		evidence := fmt.Sprintf("slot=%s, status=%s", h.Component, h.Status)
		for _, node := range ic.Topology {
			if node.Kind == "ReplicationSlot" && node.Name == h.Component {
				if lag, ok := node.Metadata["wal_lag_bytes"]; ok {
					evidence += fmt.Sprintf(", wal_lag_bytes=%s", lag)
				}
				if walStatus, ok := node.Metadata["wal_status"]; ok {
					evidence += fmt.Sprintf(", wal_status=%s", walStatus)
				}
			}
		}

		findings = append(findings, Finding{
			ID:           fmt.Sprintf("inactive-slot-%s", h.Component),
			AnalyzerName: "analyze_inactive_slots",
			Severity:     SeverityCritical,
			Title:        fmt.Sprintf("Inactive replication slot: %s", h.Component),
			Detail:       fmt.Sprintf("Replication slot %q is %s. CDC consumers using this slot cannot receive changes.", h.Component, h.Status),
			Evidence:     evidence,
			Recommendation: fmt.Sprintf("Check if the consumer (Debezium/Airbyte) for slot %q is running. "+
				"If intentionally removed, drop the slot to prevent WAL accumulation.", h.Component),
			Timestamp: time.Now(),
		})
	}
	return findings, nil
}

// analyzeWALLag detects excessive WAL replication lag.
// Looks at Topology entries with Kind=WALSender and metadata replay_lag_bytes.
func analyzeWALLag(ic *infracontext.InfraContext, args map[string]any, acc []Finding) ([]Finding, error) {
	warningThreshold := int64(100 * 1024 * 1024)  // 100MB
	criticalThreshold := int64(1024 * 1024 * 1024) // 1GB

	if v, ok := args["warning_threshold_bytes"]; ok {
		if n, err := toInt64(v); err == nil {
			warningThreshold = n
		}
	}
	if v, ok := args["critical_threshold_bytes"]; ok {
		if n, err := toInt64(v); err == nil {
			criticalThreshold = n
		}
	}

	var findings []Finding
	for _, node := range ic.Topology {
		if node.Kind != "WALSender" {
			continue
		}

		lagStr, ok := node.Metadata["replay_lag_bytes"]
		if !ok {
			continue
		}
		lagBytes, err := strconv.ParseInt(lagStr, 10, 64)
		if err != nil {
			continue
		}

		if lagBytes < warningThreshold {
			continue
		}

		severity := SeverityWarning
		if lagBytes >= criticalThreshold {
			severity = SeverityCritical
		}

		findings = append(findings, Finding{
			ID:           fmt.Sprintf("wal-lag-%s", node.Name),
			AnalyzerName: "analyze_wal_lag",
			Severity:     severity,
			Title:        fmt.Sprintf("Excessive WAL lag on %s", node.Name),
			Detail: fmt.Sprintf("WAL sender %q has %s replay lag. This indicates the consumer "+
				"is falling behind, which may trigger slot deactivation.",
				node.Name, formatBytes(lagBytes)),
			Evidence:       fmt.Sprintf("sender=%s, replay_lag_bytes=%d (%s)", node.Name, lagBytes, formatBytes(lagBytes)),
			Recommendation: "Investigate consumer throughput. Consider increasing max_wal_senders or wal_keep_size.",
			Timestamp:      time.Now(),
		})
	}
	return findings, nil
}

// analyzeConnectionSaturation detects high PostgreSQL connection utilization.
// Looks at Metrics with name "connection_utilization_pct".
func analyzeConnectionSaturation(ic *infracontext.InfraContext, args map[string]any, acc []Finding) ([]Finding, error) {
	warningPct := 80.0
	criticalPct := 95.0

	if v, ok := args["warning_pct"]; ok {
		if f, err := toFloat64(v); err == nil {
			warningPct = f
		}
	}
	if v, ok := args["critical_pct"]; ok {
		if f, err := toFloat64(v); err == nil {
			criticalPct = f
		}
	}

	var findings []Finding
	for _, m := range ic.Metrics {
		if m.Name != "connection_utilization_pct" {
			continue
		}
		if m.Value < warningPct {
			continue
		}

		severity := SeverityWarning
		if m.Value >= criticalPct {
			severity = SeverityCritical
		}

		db := m.Labels["database"]
		if db == "" {
			db = "unknown"
		}

		findings = append(findings, Finding{
			ID:             fmt.Sprintf("conn-saturation-%s", db),
			AnalyzerName:   "analyze_connection_saturation",
			Severity:       severity,
			Title:          fmt.Sprintf("Connection saturation: %.1f%%", m.Value),
			Detail:         fmt.Sprintf("Database %q is using %.1f%% of available connections. High connection usage can prevent new clients (including replication) from connecting.", db, m.Value),
			Evidence:       fmt.Sprintf("database=%s, utilization=%.1f%%", db, m.Value),
			Recommendation: "Review connection pooling (PgBouncer), close idle connections, or increase max_connections.",
			Timestamp:      time.Now(),
		})
	}
	return findings, nil
}

// analyzeFailedConnectors detects unhealthy Kafka connectors.
// Looks at Health entries with Kind=KafkaConnector and status unhealthy.
func analyzeFailedConnectors(ic *infracontext.InfraContext, args map[string]any, acc []Finding) ([]Finding, error) {
	var findings []Finding
	for _, h := range ic.Health {
		if h.Kind != "KafkaConnector" {
			continue
		}
		if h.Status == infracontext.HealthStatusHealthy {
			continue
		}

		findings = append(findings, Finding{
			ID:             fmt.Sprintf("failed-connector-%s", h.Component),
			AnalyzerName:   "analyze_failed_connectors",
			Severity:       SeverityCritical,
			Title:          fmt.Sprintf("Failed Kafka connector: %s", h.Component),
			Detail:         fmt.Sprintf("Kafka connector %q is %s. CDC data flow through this connector is interrupted.", h.Component, h.Status),
			Evidence:       fmt.Sprintf("connector=%s, status=%s, message=%s", h.Component, h.Status, h.Message),
			Recommendation: fmt.Sprintf("Check connector logs: kafka-connect GET /connectors/%s/status. Restart if transient.", h.Component),
			Timestamp:      time.Now(),
		})
	}
	return findings, nil
}

// analyzeEmptyConsumerGroups detects Kafka consumer groups with no active consumers.
// Looks at Health entries with Kind=ConsumerGroup and status degraded.
func analyzeEmptyConsumerGroups(ic *infracontext.InfraContext, args map[string]any, acc []Finding) ([]Finding, error) {
	var findings []Finding
	for _, h := range ic.Health {
		if h.Kind != "ConsumerGroup" {
			continue
		}
		if h.Status != infracontext.HealthStatusDegraded {
			continue
		}

		findings = append(findings, Finding{
			ID:             fmt.Sprintf("empty-cg-%s", h.Component),
			AnalyzerName:   "analyze_empty_consumer_groups",
			Severity:       SeverityWarning,
			Title:          fmt.Sprintf("Empty consumer group: %s", h.Component),
			Detail:         fmt.Sprintf("Consumer group %q has no active consumers. Data is accumulating on topics without being processed.", h.Component),
			Evidence:       fmt.Sprintf("consumer_group=%s, status=%s", h.Component, h.Status),
			Recommendation: "Verify that consuming application pods are running and connected to the correct consumer group.",
			Timestamp:      time.Now(),
		})
	}
	return findings, nil
}

// analyzeUnhealthyPods detects unhealthy Kubernetes pods matching name patterns.
// Filters by args["name_patterns"] (e.g., ["airbyte", "debezium", "cdc"]).
func analyzeUnhealthyPods(ic *infracontext.InfraContext, args map[string]any, acc []Finding) ([]Finding, error) {
	patterns := extractStringSlice(args, "name_patterns")

	var findings []Finding
	for _, h := range ic.Health {
		if h.Kind != "Pod" {
			continue
		}
		if h.Status == infracontext.HealthStatusHealthy {
			continue
		}

		if len(patterns) > 0 && !matchesAnyPattern(h.Component, patterns) {
			continue
		}

		findings = append(findings, Finding{
			ID:             fmt.Sprintf("unhealthy-pod-%s", h.Component),
			AnalyzerName:   "analyze_unhealthy_pods",
			Severity:       SeverityWarning,
			Title:          fmt.Sprintf("Unhealthy pod: %s", h.Component),
			Detail:         fmt.Sprintf("Pod %q is %s. This may affect pipeline data flow.", h.Component, h.Status),
			Evidence:       fmt.Sprintf("pod=%s, status=%s, message=%s", h.Component, h.Status, h.Message),
			Recommendation: fmt.Sprintf("Check pod logs: kubectl logs %s. Describe: kubectl describe pod %s", h.Component, h.Component),
			Timestamp:      time.Now(),
		})
	}
	return findings, nil
}

// --- Helpers ---

func extractStringSlice(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func matchesAnyPattern(name string, patterns []string) bool {
	lower := strings.ToLower(name)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func toInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int64:
		return val, nil
	case float64:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to int64", v)
}

func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to float64", v)
}
