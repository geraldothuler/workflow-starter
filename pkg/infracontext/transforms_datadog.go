package infracontext

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/sources"
)

func init() {
	RegisterTransform("dd_integrations_to_topology", ddIntegrationsToTopology)
	RegisterTransform("dd_services_to_topology", ddServicesToTopology)
	RegisterTransform("dd_hosts_to_topology", ddHostsToTopology)
	RegisterTransform("dd_monitors_to_alerts", ddMonitorsToAlerts)
	RegisterTransform("dd_host_metrics", ddHostMetrics)
	RegisterTransform("dd_dependencies_to_topology", ddDependenciesToTopology)
}

// ddIntegrationsToTopology converts Datadog integrations list to TopologyNodes.
// Input: array of integration objects with name, installed, type fields.
func ddIntegrationsToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		// Filter to installed integrations only
		installed, _ := m["installed"].(bool)
		if !installed {
			continue
		}

		name := sources.ExtractString(item, "name")
		if name == "" {
			continue
		}

		intType := sources.ExtractString(item, "type")
		status := "Active"
		if intType == "" {
			intType = "integration"
		}

		node := TopologyNode{
			Kind:   "Integration",
			Name:   name,
			Status: status,
			Metadata: map[string]string{
				"type": intType,
			},
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// ddServicesToTopology converts Datadog service definitions to TopologyNodes.
// Input: array of service definition objects.
func ddServicesToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		schema := sources.ExtractPath(item, "schema")
		if schema == nil {
			// Flat structure (no schema wrapper)
			schema = item
		}

		name := sources.ExtractString(schema, "dd-service")
		if name == "" {
			name = sources.ExtractString(m, "name")
		}
		if name == "" {
			continue
		}

		node := TopologyNode{
			Kind:     "Service",
			Name:     name,
			Status:   "Active",
			Metadata: make(map[string]string),
		}

		// Extract languages
		if langs := sources.ExtractSlice(schema, "languages"); len(langs) > 0 {
			var langStrs []string
			for _, l := range langs {
				if s, ok := l.(string); ok {
					langStrs = append(langStrs, s)
				}
			}
			if len(langStrs) > 0 {
				node.Metadata["languages"] = strings.Join(langStrs, ",")
			}
		}

		if svcType := sources.ExtractString(schema, "type"); svcType != "" {
			node.Metadata["type"] = svcType
		}
		if tier := sources.ExtractString(schema, "tier"); tier != "" {
			node.Metadata["tier"] = tier
		}
		if team := sources.ExtractString(schema, "team"); team != "" {
			node.Metadata["team"] = team
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

// ddHostsToTopology converts Datadog hosts list to TopologyNodes.
// Input: array of host objects from /api/v1/hosts.
func ddHostsToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name := sources.ExtractString(item, "name")
		if name == "" {
			name = sources.ExtractString(item, "host_name")
		}
		if name == "" {
			continue
		}

		// Determine status from is_muted and up fields
		status := "Active"
		if up, ok := m["up"].(bool); ok && !up {
			status = "Down"
		}
		if muted, ok := m["is_muted"].(bool); ok && muted {
			status = "Muted"
		}

		node := TopologyNode{
			Kind:     "Host",
			Name:     name,
			Status:   status,
			Metadata: make(map[string]string),
		}

		// Extract apps
		if apps := sources.ExtractSlice(item, "apps"); len(apps) > 0 {
			var appStrs []string
			for _, a := range apps {
				if s, ok := a.(string); ok {
					appStrs = append(appStrs, s)
				}
			}
			if len(appStrs) > 0 {
				node.Metadata["apps"] = strings.Join(appStrs, ",")
			}
		}

		// Extract agent version
		if ver := sources.ExtractString(item, "meta.agent_version"); ver != "" {
			node.Metadata["agent_version"] = ver
		}

		// Extract platform
		if platform := sources.ExtractString(item, "meta.platform"); platform != "" {
			node.Metadata["platform"] = platform
		}

		// Extract cloud provider from tags
		if tags := sources.ExtractSlice(item, "tags_by_source.Datadog"); tags != nil {
			for _, tag := range tags {
				if s, ok := tag.(string); ok {
					if strings.HasPrefix(s, "cloud_provider:") {
						node.Metadata["cloud"] = strings.TrimPrefix(s, "cloud_provider:")
					}
				}
			}
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

// ddMonitorsToAlerts converts Datadog monitors to Alerts.
// Input: array of monitor objects.
func ddMonitorsToAlerts(rawItems []any, args map[string]any) (any, error) {
	var alerts []Alert
	for _, item := range rawItems {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name := sources.ExtractString(item, "name")
		if name == "" {
			continue
		}

		// Map priority to severity
		severity := AlertSeverityInfo
		if priority, ok := m["priority"].(float64); ok {
			switch {
			case priority <= 2:
				severity = AlertSeverityCritical
			case priority <= 3:
				severity = AlertSeverityWarning
			default:
				severity = AlertSeverityInfo
			}
		}
		// Also check tags for priority
		if tags, ok := m["tags"].([]any); ok {
			for _, tag := range tags {
				if s, ok := tag.(string); ok {
					if strings.HasPrefix(s, "priority:") {
						p := strings.TrimPrefix(s, "priority:")
						switch p {
						case "P1", "P2", "critical":
							severity = AlertSeverityCritical
						case "P3", "warning":
							severity = AlertSeverityWarning
						}
					}
				}
			}
		}

		// Map overall_state to status
		overallState := sources.ExtractString(item, "overall_state")
		status := mapDDMonitorState(overallState)

		message := sources.ExtractString(item, "message")

		alert := Alert{
			Name:     name,
			Severity: severity,
			Status:   status,
			Message:  message,
		}

		// Try to parse created timestamp
		if created := sources.ExtractString(item, "created"); created != "" {
			if t, err := time.Parse(time.RFC3339, created); err == nil {
				alert.FiredAt = t
			}
		}

		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func mapDDMonitorState(state string) string {
	switch strings.ToLower(state) {
	case "alert":
		return AlertStatusFiring
	case "warn":
		return AlertStatusPending
	case "ok", "no data":
		return AlertStatusResolved
	default:
		return AlertStatusPending
	}
}

// ddHostMetrics extracts host metrics (CPU, memory) from Datadog hosts.
// Input: array of host objects with meta.gohai fields.
func ddHostMetrics(rawItems []any, args map[string]any) (any, error) {
	var metrics []Metric
	for _, item := range rawItems {
		name := sources.ExtractString(item, "name")
		if name == "" {
			name = sources.ExtractString(item, "host_name")
		}
		if name == "" {
			continue
		}

		labels := map[string]string{"host": name}

		// Extract CPU cores from gohai
		cpuCores := sources.ExtractString(item, "meta.gohai.cpu.cpu_cores")
		if cpuCores == "" {
			cpuCores = sources.ExtractString(item, "meta.cpuCores")
		}
		if cpuCores != "" {
			metrics = append(metrics, Metric{
				Name:   "cpu_cores",
				Value:  parseFloat(cpuCores),
				Unit:   "cores",
				Labels: labels,
			})
		}

		// Extract total memory from gohai
		memTotal := sources.ExtractString(item, "meta.gohai.memory.total")
		if memTotal == "" {
			memTotal = sources.ExtractString(item, "meta.totalMemory")
		}
		if memTotal != "" {
			metrics = append(metrics, Metric{
				Name:   "memory_total",
				Value:  parseFloat(memTotal),
				Unit:   "bytes",
				Labels: labels,
			})
		}
	}
	return metrics, nil
}

// ddDependenciesToTopology converts Datadog service dependencies to TopologyNodes.
// Input: array of dependency objects with service, deps fields.
func ddDependenciesToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	seen := make(map[string]bool)

	for _, item := range rawItems {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		serviceName := sources.ExtractString(item, "service")
		if serviceName == "" {
			// Try alternative field names
			serviceName = sources.ExtractString(item, "name")
		}
		if serviceName == "" {
			continue
		}

		// Extract dependencies
		var connectsTo []string
		if deps, ok := m["deps"].([]any); ok {
			for _, dep := range deps {
				if s, ok := dep.(string); ok {
					connectsTo = append(connectsTo, s)
				}
			}
		}
		if calls := sources.ExtractSlice(item, "calls"); calls != nil {
			for _, call := range calls {
				if callMap, ok := call.(map[string]any); ok {
					if callee := sources.ExtractString(callMap, "service"); callee != "" {
						connectsTo = append(connectsTo, callee)
					}
				}
			}
		}

		if !seen[serviceName] {
			seen[serviceName] = true
			node := TopologyNode{
				Kind:       "ServiceDependency",
				Name:       serviceName,
				Status:     "Active",
				ConnectsTo: connectsTo,
			}
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// argFloat extracts a float64 from args map.
func argFloat(args map[string]any, key string) float64 {
	if args == nil {
		return 0
	}
	v, ok := args[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		var f float64
		fmt.Sscanf(fmt.Sprintf("%v", v), "%f", &f)
		return f
	}
}
