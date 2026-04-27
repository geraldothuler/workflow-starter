package infracontext

import (
	"fmt"

	"github.com/Cobliteam/workflow-toolkit/pkg/sources"
)

// MappingEngine interprets the mapping YAML to transform raw JSON data
// into typed InfraContext data (topology, health, metrics).
type MappingEngine struct {
	techMapper *TechMapper
}

// NewMappingEngine creates a new mapping engine with the given tech mapper.
func NewMappingEngine(techMapper *TechMapper) *MappingEngine {
	return &MappingEngine{techMapper: techMapper}
}

// MapTopology maps raw JSON data into TopologyNodes using the mapping section config.
// Supports both transform-based (returns []TopologyNode) and each-based (field-by-field) mapping.
func (e *MappingEngine) MapTopology(rawData any, section *InfraMappingSection) ([]TopologyNode, error) {
	if section == nil {
		return nil, nil
	}

	// Transform-based mapping
	if section.Transform != "" {
		items := extractItems(rawData, section.SourcePath)
		result, err := CallTransform(section.Transform, items, section.Args)
		if err != nil {
			return nil, fmt.Errorf("transform %q: %w", section.Transform, err)
		}
		if nodes, ok := result.([]TopologyNode); ok {
			return nodes, nil
		}
		return nil, fmt.Errorf("transform %q returned unexpected type %T", section.Transform, result)
	}

	// Each-based mapping (field-by-field)
	items := extractItems(rawData, section.SourcePath)
	if items == nil {
		return nil, nil
	}

	var nodes []TopologyNode
	for _, item := range items {
		node, err := e.mapOneTopologyNode(item, section.Each)
		if err != nil {
			return nil, fmt.Errorf("mapping topology node: %w", err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// MapHealth maps raw JSON data into HealthChecks using the mapping section config.
func (e *MappingEngine) MapHealth(rawData any, section *InfraMappingSection) ([]HealthCheck, error) {
	if section == nil {
		return nil, nil
	}

	// Transform-based mapping
	if section.Transform != "" {
		items := extractItems(rawData, section.SourcePath)
		result, err := CallTransform(section.Transform, items, section.Args)
		if err != nil {
			return nil, fmt.Errorf("transform %q: %w", section.Transform, err)
		}
		if checks, ok := result.([]HealthCheck); ok {
			return checks, nil
		}
		return nil, fmt.Errorf("transform %q returned unexpected type %T", section.Transform, result)
	}

	return nil, nil
}

// MapMetrics maps raw JSON data into Metrics using the mapping section config.
func (e *MappingEngine) MapMetrics(rawData any, section *InfraMappingSection) ([]Metric, error) {
	if section == nil {
		return nil, nil
	}

	// Transform-based mapping
	if section.Transform != "" {
		items := extractItems(rawData, section.SourcePath)
		result, err := CallTransform(section.Transform, items, section.Args)
		if err != nil {
			return nil, fmt.Errorf("transform %q: %w", section.Transform, err)
		}
		if metrics, ok := result.([]Metric); ok {
			return metrics, nil
		}
		return nil, fmt.Errorf("transform %q returned unexpected type %T", section.Transform, result)
	}

	return nil, nil
}

// MapAlerts maps raw JSON data into Alerts using the mapping section config.
func (e *MappingEngine) MapAlerts(rawData any, section *InfraMappingSection) ([]Alert, error) {
	if section == nil {
		return nil, nil
	}

	// Transform-based mapping
	if section.Transform != "" {
		items := extractItems(rawData, section.SourcePath)
		result, err := CallTransform(section.Transform, items, section.Args)
		if err != nil {
			return nil, fmt.Errorf("transform %q: %w", section.Transform, err)
		}
		if alerts, ok := result.([]Alert); ok {
			return alerts, nil
		}
		return nil, fmt.Errorf("transform %q returned unexpected type %T", section.Transform, result)
	}

	return nil, nil
}

// MapTextMetrics maps raw text (non-JSON) data into Metrics using a transform.
func (e *MappingEngine) MapTextMetrics(rawText string, section *InfraMappingSection) ([]Metric, error) {
	if section == nil || section.Transform == "" {
		return nil, nil
	}

	result, err := CallTransform(section.Transform, []any{rawText}, section.Args)
	if err != nil {
		return nil, fmt.Errorf("transform %q: %w", section.Transform, err)
	}
	if metrics, ok := result.([]Metric); ok {
		return metrics, nil
	}
	return nil, fmt.Errorf("transform %q returned unexpected type %T", section.Transform, result)
}

// mapOneTopologyNode maps a single raw item into a TopologyNode using the "each" config.
func (e *MappingEngine) mapOneTopologyNode(item any, each map[string]any) (TopologyNode, error) {
	node := TopologyNode{}

	node.Kind = e.resolveStringField(item, each, "kind")
	node.Name = e.resolveStringField(item, each, "name")
	node.Namespace = e.resolveStringField(item, each, "namespace")
	node.Status = e.resolveStatusField(item, each)
	node.Labels = e.resolveLabelsField(item, each, "labels")
	node.Metadata = e.resolveMetadataField(item, each)
	node.Replicas = e.resolveReplicasField(item, each)
	node.Containers = e.resolveContainersField(item, each)

	return node, nil
}

// resolveStringField extracts a string value from a field config.
// Supports: { value: "literal" } or { path: "dot.path" }
func (e *MappingEngine) resolveStringField(item any, each map[string]any, fieldName string) string {
	fieldCfg, ok := each[fieldName]
	if !ok {
		return ""
	}

	cfgMap, ok := fieldCfg.(map[string]any)
	if !ok {
		return ""
	}

	// Literal value
	if v, ok := cfgMap["value"]; ok {
		return fmt.Sprintf("%v", v)
	}

	// Path extraction
	if path, ok := cfgMap["path"].(string); ok {
		return sources.ExtractString(item, path)
	}

	return ""
}

// resolveStatusField handles status which can be a simple path or a transform.
func (e *MappingEngine) resolveStatusField(item any, each map[string]any) string {
	fieldCfg, ok := each["status"]
	if !ok {
		return ""
	}

	cfgMap, ok := fieldCfg.(map[string]any)
	if !ok {
		return ""
	}

	// Simple value or path
	if v, ok := cfgMap["value"]; ok {
		return fmt.Sprintf("%v", v)
	}
	if path, ok := cfgMap["path"].(string); ok {
		return sources.ExtractString(item, path)
	}

	// Transform-based
	if transformName, ok := cfgMap["transform"].(string); ok {
		transformArgs, _ := cfgMap["args"].(map[string]any)
		result, err := CallTransform(transformName, []any{item}, transformArgs)
		if err != nil {
			return "Unknown"
		}
		if s, ok := result.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", result)
	}

	return ""
}

// resolveLabelsField extracts a map[string]string from the item.
func (e *MappingEngine) resolveLabelsField(item any, each map[string]any, fieldName string) map[string]string {
	fieldCfg, ok := each[fieldName]
	if !ok {
		return nil
	}
	cfgMap, ok := fieldCfg.(map[string]any)
	if !ok {
		return nil
	}
	path, ok := cfgMap["path"].(string)
	if !ok {
		return nil
	}
	raw := sources.ExtractPath(item, path)
	return toStringMap(raw)
}

// resolveMetadataField extracts metadata from nested field configs.
func (e *MappingEngine) resolveMetadataField(item any, each map[string]any) map[string]string {
	metaCfg, ok := each["metadata"]
	if !ok {
		return nil
	}
	metaMap, ok := metaCfg.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]string)
	for key, fieldCfg := range metaMap {
		cfgMap, ok := fieldCfg.(map[string]any)
		if !ok {
			continue
		}
		if path, ok := cfgMap["path"].(string); ok {
			val := sources.ExtractString(item, path)
			if val != "" {
				result[key] = val
			}
		}
		if transformName, ok := cfgMap["transform"].(string); ok {
			raw := sources.ExtractPath(item, argString(cfgMap, "path"))
			transformArgs, _ := cfgMap["args"].(map[string]any)
			var items []any
			if rawSlice, ok := raw.([]any); ok {
				items = rawSlice
			} else if raw != nil {
				items = []any{raw}
			}
			res, err := CallTransform(transformName, items, transformArgs)
			if err == nil {
				result[key] = fmt.Sprintf("%v", res)
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// resolveReplicasField extracts replica info from nested field configs.
func (e *MappingEngine) resolveReplicasField(item any, each map[string]any) *ReplicaInfo {
	repCfg, ok := each["replicas"]
	if !ok {
		return nil
	}
	repMap, ok := repCfg.(map[string]any)
	if !ok {
		return nil
	}

	info := &ReplicaInfo{}
	info.Desired = resolveIntField(item, repMap, "desired")
	info.Ready = resolveIntField(item, repMap, "ready")
	info.Available = resolveIntField(item, repMap, "available")
	return info
}

// resolveContainersField extracts container info from the item.
func (e *MappingEngine) resolveContainersField(item any, each map[string]any) []ContainerInfo {
	contCfg, ok := each["containers"]
	if !ok {
		return nil
	}
	contMap, ok := contCfg.(map[string]any)
	if !ok {
		return nil
	}

	sourcePath, _ := contMap["source_path"].(string)
	if sourcePath == "" {
		return nil
	}

	containers := sources.ExtractSlice(item, sourcePath)
	if containers == nil {
		return nil
	}

	eachCfg, ok := contMap["each"].(map[string]any)
	if !ok {
		return nil
	}

	// Get ready status from a separate path if specified
	readyFrom, _ := contMap["ready_from"].(string)
	var containerStatuses []any
	if readyFrom != "" {
		containerStatuses = sources.ExtractSlice(item, readyFrom)
	}

	var result []ContainerInfo
	for i, c := range containers {
		ci := ContainerInfo{}
		ci.Name = resolveStringFromCfg(c, eachCfg, "name")
		ci.Image = resolveStringFromCfg(c, eachCfg, "image")
		ci.CPURequest = resolveStringFromCfg(c, eachCfg, "cpu_request")
		ci.CPULimit = resolveStringFromCfg(c, eachCfg, "cpu_limit")
		ci.MemRequest = resolveStringFromCfg(c, eachCfg, "mem_request")
		ci.MemLimit = resolveStringFromCfg(c, eachCfg, "mem_limit")

		// Resolve ready from container statuses
		if containerStatuses != nil && i < len(containerStatuses) {
			if statusMap, ok := containerStatuses[i].(map[string]any); ok {
				if ready, ok := statusMap["ready"].(bool); ok {
					ci.Ready = ready
				}
			}
		}

		result = append(result, ci)
	}
	return result
}

// --- Helpers ---

func extractItems(rawData any, sourcePath string) []any {
	if sourcePath == "" {
		if items, ok := rawData.([]any); ok {
			return items
		}
		return nil
	}
	return sources.ExtractSlice(rawData, sourcePath)
}

func resolveStringFromCfg(item any, each map[string]any, fieldName string) string {
	fieldCfg, ok := each[fieldName]
	if !ok {
		return ""
	}
	cfgMap, ok := fieldCfg.(map[string]any)
	if !ok {
		return ""
	}
	if path, ok := cfgMap["path"].(string); ok {
		return sources.ExtractString(item, path)
	}
	if v, ok := cfgMap["value"]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func resolveIntField(item any, each map[string]any, fieldName string) int {
	fieldCfg, ok := each[fieldName]
	if !ok {
		return 0
	}
	cfgMap, ok := fieldCfg.(map[string]any)
	if !ok {
		return 0
	}

	if path, ok := cfgMap["path"].(string); ok {
		raw := sources.ExtractPath(item, path)
		return toInt(raw, cfgMap)
	}
	return 0
}

func toInt(v any, cfgMap map[string]any) int {
	if v == nil {
		if d, ok := cfgMap["default"]; ok {
			return toIntRaw(d)
		}
		return 0
	}
	return toIntRaw(v)
}

func toIntRaw(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		var i int
		fmt.Sscanf(fmt.Sprintf("%v", v), "%d", &i)
		return i
	}
}

func toStringMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, val := range m {
		result[k] = fmt.Sprintf("%v", val)
	}
	return result
}
