package infracontext

import (
	"fmt"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/sources"
)

func init() {
	RegisterTransform("pg_database_info_to_topology", pgDatabaseInfoToTopology)
	RegisterTransform("pg_database_info_to_metrics", pgDatabaseInfoToMetrics)
	RegisterTransform("pg_replication_slots_to_topology", pgReplicationSlotsToTopology)
	RegisterTransform("pg_replication_slots_to_health", pgReplicationSlotsToHealth)
	RegisterTransform("pg_replication_stats_to_topology", pgReplicationStatsToTopology)
	RegisterTransform("pg_replication_stats_to_health", pgReplicationStatsToHealth)
	RegisterTransform("pg_connections_to_topology", pgConnectionsToTopology)
	RegisterTransform("pg_connections_to_metrics", pgConnectionsToMetrics)
	RegisterTransform("pg_tables_to_topology", pgTablesToTopology)
	RegisterTransform("pg_tables_to_metrics", pgTablesToMetrics)
}

// pgDatabaseInfoToTopology extracts database info as topology nodes.
func pgDatabaseInfoToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		name := sources.ExtractString(item, "name")
		version := sources.ExtractString(item, "version")
		startedAt := sources.ExtractString(item, "started_at")
		sizeBytes := extractFloat(item, "size_bytes")
		activeConns := extractFloat(item, "active_connections")
		maxConns := extractFloat(item, "max_connections")

		node := TopologyNode{
			Kind:   "PostgresDatabase",
			Name:   name,
			Status: "running",
			Metadata: map[string]string{
				"version":            version,
				"started_at":         startedAt,
				"size_bytes":         fmt.Sprintf("%.0f", sizeBytes),
				"active_connections": fmt.Sprintf("%.0f", activeConns),
				"max_connections":    fmt.Sprintf("%.0f", maxConns),
			},
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// pgDatabaseInfoToMetrics extracts database-level metrics.
func pgDatabaseInfoToMetrics(rawItems []any, args map[string]any) (any, error) {
	var metrics []Metric
	for _, item := range rawItems {
		name := sources.ExtractString(item, "name")
		activeConns := extractFloat(item, "active_connections")
		maxConns := extractFloat(item, "max_connections")
		sizeBytes := extractFloat(item, "size_bytes")

		labels := map[string]string{"database": name}

		metrics = append(metrics,
			Metric{Name: "active_connections", Value: activeConns, Unit: "connections", Labels: labels},
			Metric{Name: "max_connections", Value: maxConns, Unit: "connections", Labels: labels},
			Metric{Name: "database_size_bytes", Value: sizeBytes, Unit: "bytes", Labels: labels},
		)

		if maxConns > 0 {
			utilPct := (activeConns / maxConns) * 100
			metrics = append(metrics, Metric{
				Name:   "connection_utilization_pct",
				Value:  utilPct,
				Unit:   "percent",
				Labels: labels,
			})
		}
	}
	return metrics, nil
}

// pgReplicationSlotsToTopology extracts replication slots as topology nodes.
func pgReplicationSlotsToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		slotName := sources.ExtractString(item, "slot_name")
		plugin := sources.ExtractString(item, "plugin")
		slotType := sources.ExtractString(item, "slot_type")
		active := extractBool(item, "active")
		restartLSN := sources.ExtractString(item, "restart_lsn")
		confirmedFlushLSN := sources.ExtractString(item, "confirmed_flush_lsn")
		walStatus := sources.ExtractString(item, "wal_status")
		lagBytes := extractFloat(item, "slot_lag_bytes")

		status := "inactive"
		if active {
			status = "active"
		}

		node := TopologyNode{
			Kind:   "ReplicationSlot",
			Name:   slotName,
			Status: status,
			Metadata: map[string]string{
				"plugin":              plugin,
				"slot_type":           slotType,
				"restart_lsn":         restartLSN,
				"confirmed_flush_lsn": confirmedFlushLSN,
				"wal_status":          walStatus,
				"slot_lag_bytes":      fmt.Sprintf("%.0f", lagBytes),
			},
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// pgReplicationSlotsToHealth extracts health checks from replication slots.
// Active slot → healthy; inactive + lag > 1GB → unhealthy; inactive + lag > 100MB → degraded;
// inactive + no lag → degraded (cleanup candidate).
func pgReplicationSlotsToHealth(rawItems []any, args map[string]any) (any, error) {
	var checks []HealthCheck
	for _, item := range rawItems {
		slotName := sources.ExtractString(item, "slot_name")
		active := extractBool(item, "active")
		lagBytes := extractFloat(item, "slot_lag_bytes")

		status, message := mapReplicationSlotHealth(active, lagBytes)

		checks = append(checks, HealthCheck{
			Component: slotName,
			Kind:      "ReplicationSlot",
			Status:    status,
			Message:   message,
			LastCheck: time.Now(),
		})
	}
	return checks, nil
}

// pgReplicationStatsToTopology extracts WAL sender info as topology nodes.
func pgReplicationStatsToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		appName := sources.ExtractString(item, "application_name")
		clientAddr := sources.ExtractString(item, "client_addr")
		state := sources.ExtractString(item, "state")
		syncState := sources.ExtractString(item, "sync_state")
		sentLSN := sources.ExtractString(item, "sent_lsn")
		replayLSN := sources.ExtractString(item, "replay_lsn")
		replayLagBytes := extractFloat(item, "replay_lag_bytes")

		name := appName
		if name == "" {
			name = clientAddr
		}

		node := TopologyNode{
			Kind:   "WALSender",
			Name:   name,
			Status: state,
			Metadata: map[string]string{
				"application_name": appName,
				"client_addr":     clientAddr,
				"sync_state":      syncState,
				"sent_lsn":        sentLSN,
				"replay_lsn":      replayLSN,
				"replay_lag_bytes": fmt.Sprintf("%.0f", replayLagBytes),
			},
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// pgReplicationStatsToHealth extracts health from WAL sender state.
// streaming → healthy; catchup → degraded; startup/backup → unhealthy.
func pgReplicationStatsToHealth(rawItems []any, args map[string]any) (any, error) {
	var checks []HealthCheck
	for _, item := range rawItems {
		appName := sources.ExtractString(item, "application_name")
		clientAddr := sources.ExtractString(item, "client_addr")
		state := sources.ExtractString(item, "state")

		name := appName
		if name == "" {
			name = clientAddr
		}

		status, message := mapReplicationState(state)

		checks = append(checks, HealthCheck{
			Component: name,
			Kind:      "WALSender",
			Status:    status,
			Message:   message,
			LastCheck: time.Now(),
		})
	}
	return checks, nil
}

// pgConnectionsToTopology extracts unique app_name+state combinations as topology nodes.
func pgConnectionsToTopology(rawItems []any, args map[string]any) (any, error) {
	// Aggregate by application_name
	type connGroup struct {
		appName    string
		clientAddr string
		count      int
		states     map[string]int
	}
	groups := make(map[string]*connGroup)

	for _, item := range rawItems {
		appName := sources.ExtractString(item, "application_name")
		clientAddr := sources.ExtractString(item, "client_addr")
		state := sources.ExtractString(item, "state")

		key := appName
		if key == "" {
			key = clientAddr
		}
		if key == "" {
			key = "unknown"
		}

		g, ok := groups[key]
		if !ok {
			g = &connGroup{
				appName:    appName,
				clientAddr: clientAddr,
				states:     make(map[string]int),
			}
			groups[key] = g
		}
		g.count++
		g.states[state]++
	}

	var nodes []TopologyNode
	for key, g := range groups {
		// Determine dominant state
		dominantState := ""
		maxCount := 0
		for s, c := range g.states {
			if c > maxCount {
				dominantState = s
				maxCount = c
			}
		}

		metadata := map[string]string{
			"connection_count": fmt.Sprintf("%d", g.count),
			"dominant_state":   dominantState,
		}
		if g.clientAddr != "" {
			metadata["client_addr"] = g.clientAddr
		}

		nodes = append(nodes, TopologyNode{
			Kind:     "Connection",
			Name:     key,
			Status:   dominantState,
			Metadata: metadata,
		})
		_ = key // used as Name above
	}
	return nodes, nil
}

// pgConnectionsToMetrics extracts connection count metrics by state.
func pgConnectionsToMetrics(rawItems []any, args map[string]any) (any, error) {
	stateCounts := make(map[string]int)
	longRunning := 0

	for _, item := range rawItems {
		state := sources.ExtractString(item, "state")
		stateCounts[state]++

		duration := extractFloat(item, "state_duration_seconds")
		if state == "active" && duration > 60 {
			longRunning++
		}
	}

	var metrics []Metric
	total := 0
	for state, count := range stateCounts {
		total += count
		metrics = append(metrics, Metric{
			Name:   "connections_" + state,
			Value:  float64(count),
			Unit:   "connections",
			Labels: map[string]string{"state": state},
		})
	}

	metrics = append(metrics,
		Metric{Name: "connections_total", Value: float64(total), Unit: "connections"},
		Metric{Name: "connections_long_running", Value: float64(longRunning), Unit: "connections"},
	)

	return metrics, nil
}

// pgTablesToTopology extracts top tables as topology nodes.
func pgTablesToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		schema := sources.ExtractString(item, "schemaname")
		relname := sources.ExtractString(item, "relname")
		liveTup := extractFloat(item, "n_live_tup")
		deadTup := extractFloat(item, "n_dead_tup")
		lastVacuum := sources.ExtractString(item, "last_vacuum")
		lastAutovacuum := sources.ExtractString(item, "last_autovacuum")
		totalSize := extractFloat(item, "total_size_bytes")

		name := schema + "." + relname

		node := TopologyNode{
			Kind:   "Table",
			Name:   name,
			Status: "active",
			Metadata: map[string]string{
				"schema":           schema,
				"n_live_tup":       fmt.Sprintf("%.0f", liveTup),
				"n_dead_tup":       fmt.Sprintf("%.0f", deadTup),
				"last_vacuum":      lastVacuum,
				"last_autovacuum":  lastAutovacuum,
				"total_size_bytes": fmt.Sprintf("%.0f", totalSize),
			},
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// pgTablesToMetrics extracts table-level metrics (dead tuple ratio, sizes).
func pgTablesToMetrics(rawItems []any, args map[string]any) (any, error) {
	var metrics []Metric
	var totalDeadTuples, totalLiveTuples float64
	var totalSize float64

	for _, item := range rawItems {
		schema := sources.ExtractString(item, "schemaname")
		relname := sources.ExtractString(item, "relname")
		liveTup := extractFloat(item, "n_live_tup")
		deadTup := extractFloat(item, "n_dead_tup")
		size := extractFloat(item, "total_size_bytes")

		tableName := schema + "." + relname
		labels := map[string]string{"table": tableName}

		totalDeadTuples += deadTup
		totalLiveTuples += liveTup
		totalSize += size

		metrics = append(metrics, Metric{
			Name:   "table_size_bytes",
			Value:  size,
			Unit:   "bytes",
			Labels: labels,
		})

		if liveTup+deadTup > 0 {
			ratio := deadTup / (liveTup + deadTup) * 100
			metrics = append(metrics, Metric{
				Name:   "dead_tuple_ratio",
				Value:  ratio,
				Unit:   "percent",
				Labels: labels,
			})
		}
	}

	// Aggregate metrics
	metrics = append(metrics,
		Metric{Name: "total_tables", Value: float64(len(rawItems)), Unit: "tables"},
		Metric{Name: "total_dead_tuples", Value: totalDeadTuples, Unit: "tuples"},
		Metric{Name: "total_database_size", Value: totalSize, Unit: "bytes"},
	)

	if totalLiveTuples+totalDeadTuples > 0 {
		overallRatio := totalDeadTuples / (totalLiveTuples + totalDeadTuples) * 100
		metrics = append(metrics, Metric{
			Name: "overall_dead_tuple_ratio",
			Value: overallRatio,
			Unit:  "percent",
		})
	}

	return metrics, nil
}

// --- Helpers ---

// mapReplicationSlotHealth maps replication slot state to health status.
// Active → healthy; inactive + lag > 1GB → unhealthy; inactive + lag > 100MB → degraded;
// inactive + no/low lag → degraded (cleanup candidate).
func mapReplicationSlotHealth(active bool, lagBytes float64) (string, string) {
	if active {
		return HealthStatusHealthy, "slot active"
	}
	const (
		oneGB  = 1024 * 1024 * 1024 // 1 GiB
		hundMB = 100 * 1024 * 1024  // 100 MiB
	)
	if lagBytes > float64(oneGB) {
		return HealthStatusUnhealthy, fmt.Sprintf("slot inactive, WAL lag %.0f bytes (>1GB)", lagBytes)
	}
	if lagBytes > float64(hundMB) {
		return HealthStatusDegraded, fmt.Sprintf("slot inactive, WAL lag %.0f bytes (>100MB)", lagBytes)
	}
	return HealthStatusDegraded, "slot inactive"
}

// mapReplicationState maps WAL sender state to health status.
// streaming → healthy; catchup → degraded; startup/backup → unhealthy.
func mapReplicationState(state string) (string, string) {
	switch state {
	case "streaming":
		return HealthStatusHealthy, "streaming"
	case "catchup":
		return HealthStatusDegraded, "catching up"
	case "startup":
		return HealthStatusUnhealthy, "starting up"
	case "backup":
		return HealthStatusUnhealthy, "backup in progress"
	default:
		return HealthStatusUnknown, state
	}
}

// extractFloat safely extracts a float64 from a nested map.
func extractFloat(item any, field string) float64 {
	val := sources.ExtractPath(item, field)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		var f float64
		fmt.Sscanf(v, "%f", &f)
		return f
	default:
		return 0
	}
}

// extractBool safely extracts a bool from a nested map.
func extractBool(item any, field string) bool {
	val := sources.ExtractPath(item, field)
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "t"
	default:
		return false
	}
}
