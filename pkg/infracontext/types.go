// Package infracontext provides a normalized abstraction for infrastructure context
// (metrics, topology, health, alerts) from multiple providers (kubectl, Datadog, etc.).
// All configuration is YAML-driven; Go code is the engine that interprets the YAMLs.
package infracontext

import (
	"time"
)

// InfraContext is the normalized infrastructure context.
// All providers map their data to this schema.
type InfraContext struct {
	Provider  string        `json:"provider" yaml:"provider"`
	Cluster   string        `json:"cluster" yaml:"cluster"`
	Namespace string        `json:"namespace" yaml:"namespace"`
	FetchedAt time.Time     `json:"fetched_at" yaml:"fetched_at"`
	TTL       time.Duration `json:"ttl" yaml:"ttl"`
	Metrics   []Metric      `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Topology  []TopologyNode `json:"topology,omitempty" yaml:"topology,omitempty"`
	Health    []HealthCheck `json:"health,omitempty" yaml:"health,omitempty"`
	Alerts    []Alert       `json:"alerts,omitempty" yaml:"alerts,omitempty"`
}

// IsExpired returns true if the context data has exceeded its TTL.
func (ic *InfraContext) IsExpired() bool {
	if ic.TTL <= 0 {
		return false
	}
	return time.Since(ic.FetchedAt) > ic.TTL
}

// TechMap returns a map of detected technologies by scanning container images
// in the topology against the provided tech mapping.
func (ic *InfraContext) TechMap(mapper *TechMapper) map[string]string {
	result := make(map[string]string)
	if mapper == nil {
		return result
	}
	for _, node := range ic.Topology {
		for _, c := range node.Containers {
			if tech := mapper.ExtractTechFromImage(c.Image); tech != "" {
				result[tech] = c.Image
			}
		}
	}
	return result
}

// HealthByTech groups health checks by component kind (e.g., "Deployment", "Pod").
func (ic *InfraContext) HealthByTech() map[string][]HealthCheck {
	result := make(map[string][]HealthCheck)
	for _, h := range ic.Health {
		result[h.Kind] = append(result[h.Kind], h)
	}
	return result
}

// HealthByComponent returns health checks indexed by component name.
func (ic *InfraContext) HealthByComponent() map[string]HealthCheck {
	result := make(map[string]HealthCheck)
	for _, h := range ic.Health {
		result[h.Component] = h
	}
	return result
}

// Summary returns a human-readable summary of the infrastructure context.
func (ic *InfraContext) Summary() string {
	var pods, services, deployments, nodes int
	for _, n := range ic.Topology {
		switch n.Kind {
		case "Pod":
			pods++
		case "Service":
			services++
		case "Deployment":
			deployments++
		case "Node":
			nodes++
		}
	}

	healthy, degraded, unhealthy := 0, 0, 0
	for _, h := range ic.Health {
		switch h.Status {
		case HealthStatusHealthy:
			healthy++
		case HealthStatusDegraded:
			degraded++
		case HealthStatusUnhealthy:
			unhealthy++
		}
	}

	summary := ic.Provider + "/" + ic.Cluster
	if ic.Namespace != "" {
		summary += "/" + ic.Namespace
	}
	summary += " | "
	summary += "nodes=" + itoa(nodes) + " pods=" + itoa(pods) + " svcs=" + itoa(services) + " deploys=" + itoa(deployments)
	summary += " | health: " + itoa(healthy) + " ok, " + itoa(degraded) + " degraded, " + itoa(unhealthy) + " unhealthy"
	summary += " | metrics=" + itoa(len(ic.Metrics)) + " alerts=" + itoa(len(ic.Alerts))
	return summary
}

// Metric represents a single infrastructure metric.
type Metric struct {
	Name      string            `json:"name" yaml:"name"`
	Value     float64           `json:"value" yaml:"value"`
	Unit      string            `json:"unit,omitempty" yaml:"unit,omitempty"`
	Labels    map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

// TopologyNode represents a Kubernetes resource (Pod, Service, Deployment, Node).
type TopologyNode struct {
	Kind       string            `json:"kind" yaml:"kind"`
	Name       string            `json:"name" yaml:"name"`
	Namespace  string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Status     string            `json:"status" yaml:"status"`
	Labels     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Replicas   *ReplicaInfo      `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Containers []ContainerInfo   `json:"containers,omitempty" yaml:"containers,omitempty"`
	ConnectsTo []string          `json:"connects_to,omitempty" yaml:"connects_to,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ReplicaInfo holds replica counts for scalable resources.
type ReplicaInfo struct {
	Desired   int `json:"desired" yaml:"desired"`
	Ready     int `json:"ready" yaml:"ready"`
	Available int `json:"available" yaml:"available"`
}

// ContainerInfo holds container specification details.
type ContainerInfo struct {
	Name       string `json:"name" yaml:"name"`
	Image      string `json:"image" yaml:"image"`
	Ready      bool   `json:"ready" yaml:"ready"`
	CPURequest string `json:"cpu_request,omitempty" yaml:"cpu_request,omitempty"`
	CPULimit   string `json:"cpu_limit,omitempty" yaml:"cpu_limit,omitempty"`
	MemRequest string `json:"mem_request,omitempty" yaml:"mem_request,omitempty"`
	MemLimit   string `json:"mem_limit,omitempty" yaml:"mem_limit,omitempty"`
}

// HealthStatus constants for HealthCheck.Status.
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusDegraded  = "degraded"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusUnknown   = "unknown"
)

// HealthCheck represents the health status of a component.
type HealthCheck struct {
	Component string    `json:"component" yaml:"component"`
	Kind      string    `json:"kind,omitempty" yaml:"kind,omitempty"`
	Status    string    `json:"status" yaml:"status"`
	Message   string    `json:"message,omitempty" yaml:"message,omitempty"`
	LastCheck time.Time `json:"last_check,omitempty" yaml:"last_check,omitempty"`
}

// AlertSeverity constants for Alert.Severity.
const (
	AlertSeverityCritical = "critical"
	AlertSeverityWarning  = "warning"
	AlertSeverityInfo     = "info"
)

// AlertStatus constants for Alert.Status.
const (
	AlertStatusFiring   = "firing"
	AlertStatusResolved = "resolved"
	AlertStatusPending  = "pending"
)

// Alert represents an infrastructure alert.
type Alert struct {
	Name      string    `json:"name" yaml:"name"`
	Severity  string    `json:"severity" yaml:"severity"`
	Component string    `json:"component,omitempty" yaml:"component,omitempty"`
	Message   string    `json:"message,omitempty" yaml:"message,omitempty"`
	FiredAt   time.Time `json:"fired_at,omitempty" yaml:"fired_at,omitempty"`
	Status    string    `json:"status" yaml:"status"`
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
