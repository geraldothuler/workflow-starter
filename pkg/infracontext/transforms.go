package infracontext

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/sources"
)

// TransformFunc is a named transform function that converts raw data into
// typed InfraContext data ([]HealthCheck, []Metric, or a string).
// rawItems is the slice of raw objects, args contains configuration from YAML.
type TransformFunc func(rawItems []any, args map[string]any) (any, error)

// TransformRegistry holds named transform functions.
// The YAML references them by name; Go code registers them here.
var transformRegistry = map[string]TransformFunc{
	"health_from_conditions":  healthFromConditions,
	"health_from_pod_status":  healthFromPodStatus,
	"deployment_status":       deploymentStatus,
	"node_status":             nodeStatus,
	"format_ports":            formatPorts,
	"node_allocatable_metrics": nodeAllocatableMetrics,
	"parse_kubectl_top":       parseKubectlTop,
	"tech_from_image":         techFromImage,
}

// CallTransform invokes a named transform function.
func CallTransform(name string, rawItems []any, args map[string]any) (any, error) {
	fn, ok := transformRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unknown transform function: %q", name)
	}
	return fn(rawItems, args)
}

// RegisterTransform adds a custom transform (for extensibility or testing).
func RegisterTransform(name string, fn TransformFunc) {
	transformRegistry[name] = fn
}

// ListTransforms returns all registered transform names.
func ListTransforms() []string {
	names := make([]string, 0, len(transformRegistry))
	for name := range transformRegistry {
		names = append(names, name)
	}
	return names
}

// --- Transform implementations ---

// healthFromConditions extracts health checks from Kubernetes conditions arrays.
// Used for Deployments, Nodes, StatefulSets.
// Args: name_path, kind, conditions_path, ready_type
func healthFromConditions(rawItems []any, args map[string]any) (any, error) {
	namePath := argString(args, "name_path")
	kind := argString(args, "kind")
	conditionsPath := argString(args, "conditions_path")
	readyType := argString(args, "ready_type")

	var checks []HealthCheck
	for _, item := range rawItems {
		name := sources.ExtractString(item, namePath)
		conditions := sources.ExtractSlice(item, conditionsPath)

		status := HealthStatusUnknown
		message := ""
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]any)
			if !ok {
				continue
			}
			condType, _ := condMap["type"].(string)
			condStatus, _ := condMap["status"].(string)
			if condType == readyType {
				if condStatus == "True" {
					status = HealthStatusHealthy
				} else {
					status = HealthStatusUnhealthy
					if msg, ok := condMap["message"].(string); ok {
						message = msg
					}
				}
				break
			}
		}

		checks = append(checks, HealthCheck{
			Component: name,
			Kind:      kind,
			Status:    status,
			Message:   message,
			LastCheck: time.Now(),
		})
	}
	return checks, nil
}

// healthFromPodStatus extracts health checks from pod status and conditions.
// Args: name_path, phase_path, conditions_path, ready_type
func healthFromPodStatus(rawItems []any, args map[string]any) (any, error) {
	namePath := argString(args, "name_path")
	phasePath := argString(args, "phase_path")
	conditionsPath := argString(args, "conditions_path")
	readyType := argString(args, "ready_type")

	var checks []HealthCheck
	for _, item := range rawItems {
		name := sources.ExtractString(item, namePath)
		phase := sources.ExtractString(item, phasePath)

		status := HealthStatusUnknown
		message := ""

		switch phase {
		case "Running":
			status = HealthStatusHealthy
			// Check if Ready condition exists and is true
			conditions := sources.ExtractSlice(item, conditionsPath)
			for _, cond := range conditions {
				condMap, ok := cond.(map[string]any)
				if !ok {
					continue
				}
				condType, _ := condMap["type"].(string)
				condStatus, _ := condMap["status"].(string)
				if condType == readyType && condStatus != "True" {
					status = HealthStatusDegraded
					if msg, ok := condMap["message"].(string); ok {
						message = msg
					}
				}
			}
		case "Succeeded":
			status = HealthStatusHealthy
			message = "completed"
		case "Pending":
			status = HealthStatusDegraded
			message = "pending"
		case "Failed":
			status = HealthStatusUnhealthy
			message = "failed"
		default:
			status = HealthStatusUnknown
			message = phase
		}

		checks = append(checks, HealthCheck{
			Component: name,
			Kind:      "Pod",
			Status:    status,
			Message:   message,
			LastCheck: time.Now(),
		})
	}
	return checks, nil
}

// deploymentStatus derives a status string from deployment conditions.
// Args: conditions_path
func deploymentStatus(rawItems []any, args map[string]any) (any, error) {
	conditionsPath := argString(args, "conditions_path")
	if len(rawItems) == 0 {
		return "Unknown", nil
	}
	// deploymentStatus is called per-item, rawItems[0] is the item itself
	item := rawItems[0]
	conditions := sources.ExtractSlice(item, conditionsPath)
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]any)
		if !ok {
			continue
		}
		condType, _ := condMap["type"].(string)
		condStatus, _ := condMap["status"].(string)
		if condType == "Available" {
			if condStatus == "True" {
				return "Available", nil
			}
			return "Unavailable", nil
		}
	}
	return "Unknown", nil
}

// nodeStatus derives a status string from node conditions.
// Args: conditions_path
func nodeStatus(rawItems []any, args map[string]any) (any, error) {
	conditionsPath := argString(args, "conditions_path")
	if len(rawItems) == 0 {
		return "Unknown", nil
	}
	item := rawItems[0]
	conditions := sources.ExtractSlice(item, conditionsPath)
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]any)
		if !ok {
			continue
		}
		condType, _ := condMap["type"].(string)
		condStatus, _ := condMap["status"].(string)
		if condType == "Ready" {
			if condStatus == "True" {
				return "Ready", nil
			}
			return "NotReady", nil
		}
	}
	return "Unknown", nil
}

// formatPorts formats a Kubernetes service ports array into a string.
// Args: (none — rawItems[0] is the ports array)
func formatPorts(rawItems []any, args map[string]any) (any, error) {
	var parts []string
	for _, item := range rawItems {
		portMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		port := fmt.Sprintf("%v", portMap["port"])
		protocol, _ := portMap["protocol"].(string)
		name, _ := portMap["name"].(string)

		entry := port
		if protocol != "" {
			entry += "/" + protocol
		}
		if name != "" {
			entry = name + ":" + entry
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, ","), nil
}

// nodeAllocatableMetrics extracts allocatable resource metrics from nodes.
// Args: name_path, allocatable_path
func nodeAllocatableMetrics(rawItems []any, args map[string]any) (any, error) {
	namePath := argString(args, "name_path")
	allocatablePath := argString(args, "allocatable_path")

	var metrics []Metric
	for _, item := range rawItems {
		name := sources.ExtractString(item, namePath)
		alloc := sources.ExtractPath(item, allocatablePath)
		allocMap, ok := alloc.(map[string]any)
		if !ok {
			continue
		}

		if cpu, ok := allocMap["cpu"]; ok {
			metrics = append(metrics, Metric{
				Name:  "allocatable_cpu",
				Value: parseCPU(fmt.Sprintf("%v", cpu)),
				Unit:  "cores",
				Labels: map[string]string{"node": name},
			})
		}
		if mem, ok := allocMap["memory"]; ok {
			metrics = append(metrics, Metric{
				Name:  "allocatable_memory",
				Value: parseMemory(fmt.Sprintf("%v", mem)),
				Unit:  "bytes",
				Labels: map[string]string{"node": name},
			})
		}
	}
	return metrics, nil
}

// parseKubectlTop parses `kubectl top pods` text output into metrics.
// Args: format (e.g., "name cpu_millicores mem_bytes")
func parseKubectlTop(rawItems []any, args map[string]any) (any, error) {
	// rawItems[0] should be the raw text content as string
	if len(rawItems) == 0 {
		return []Metric{}, nil
	}
	text, ok := rawItems[0].(string)
	if !ok {
		return []Metric{}, nil
	}

	var metrics []Metric
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]

		cpuVal := parseCPUMillicores(fields[1])
		metrics = append(metrics, Metric{
			Name:   "cpu_usage",
			Value:  cpuVal,
			Unit:   "millicores",
			Labels: map[string]string{"pod": name},
		})

		memVal := parseMemoryMi(fields[2])
		metrics = append(metrics, Metric{
			Name:   "memory_usage",
			Value:  memVal,
			Unit:   "Mi",
			Labels: map[string]string{"pod": name},
		})
	}
	return metrics, nil
}

// techFromImage is a placeholder — actual logic is in TechMapper.
// This is used when referenced from YAML as a field-level transform.
func techFromImage(rawItems []any, args map[string]any) (any, error) {
	if len(rawItems) == 0 {
		return "", nil
	}
	image, ok := rawItems[0].(string)
	if !ok {
		return "", nil
	}
	// Simple prefix matching — full matching is done by TechMapper
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	name := strings.Split(last, ":")[0]
	return name, nil
}

// --- Helpers ---

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// parseCPU converts Kubernetes CPU values (e.g., "4", "500m") to cores.
func parseCPU(s string) float64 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "m") {
		v := parseFloat(strings.TrimSuffix(s, "m"))
		return v / 1000
	}
	return parseFloat(s)
}

// parseMemory converts Kubernetes memory values (e.g., "8Gi", "1024Mi") to bytes.
func parseMemory(s string) float64 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "Ki") {
		return parseFloat(strings.TrimSuffix(s, "Ki")) * 1024
	}
	if strings.HasSuffix(s, "Mi") {
		return parseFloat(strings.TrimSuffix(s, "Mi")) * 1024 * 1024
	}
	if strings.HasSuffix(s, "Gi") {
		return parseFloat(strings.TrimSuffix(s, "Gi")) * 1024 * 1024 * 1024
	}
	if strings.HasSuffix(s, "Ti") {
		return parseFloat(strings.TrimSuffix(s, "Ti")) * 1024 * 1024 * 1024 * 1024
	}
	return parseFloat(s)
}

// parseCPUMillicores parses "250m" or "1" into millicores as float64.
func parseCPUMillicores(s string) float64 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "m") {
		return parseFloat(strings.TrimSuffix(s, "m"))
	}
	// Plain number means cores, convert to millicores
	return parseFloat(s) * 1000
}

// parseMemoryMi parses "128Mi" or "1Gi" into MiB as float64.
func parseMemoryMi(s string) float64 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "Mi") {
		return parseFloat(strings.TrimSuffix(s, "Mi"))
	}
	if strings.HasSuffix(s, "Gi") {
		return parseFloat(strings.TrimSuffix(s, "Gi")) * 1024
	}
	if strings.HasSuffix(s, "Ki") {
		return parseFloat(strings.TrimSuffix(s, "Ki")) / 1024
	}
	return parseFloat(s)
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
