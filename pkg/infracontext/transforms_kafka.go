package infracontext

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/sources"
)

func init() {
	RegisterTransform("kafka_clusters_to_topology", kafkaClustersToTopology)
	RegisterTransform("kafka_brokers_to_topology", kafkaBrokersToTopology)
	RegisterTransform("kafka_topics_to_topology", kafkaTopicsToTopology)
	RegisterTransform("kafka_consumer_groups_to_topology", kafkaConsumerGroupsToTopology)
	RegisterTransform("kafka_consumer_groups_to_health", kafkaConsumerGroupsToHealth)
	RegisterTransform("kafka_connectors_to_topology", kafkaConnectorsToTopology)
	RegisterTransform("kafka_connectors_to_health", kafkaConnectorsToHealth)
}

// kafkaClustersToTopology converts Confluent Cloud clusters to TopologyNodes.
// Input: array of cluster objects from /cmk/v2/clusters.
func kafkaClustersToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		_, ok := item.(map[string]any)
		if !ok {
			continue
		}

		id := sources.ExtractString(item, "id")
		if id == "" {
			continue
		}

		name := sources.ExtractString(item, "spec.display_name")
		if name == "" {
			name = id
		}

		status := sources.ExtractString(item, "status.phase")
		if status == "" {
			status = "Unknown"
		}

		node := TopologyNode{
			Kind:     "KafkaCluster",
			Name:     name,
			Status:   status,
			Metadata: map[string]string{"id": id},
		}

		if avail := sources.ExtractString(item, "spec.availability"); avail != "" {
			node.Metadata["availability"] = avail
		}
		if cloud := sources.ExtractString(item, "spec.cloud"); cloud != "" {
			node.Metadata["cloud"] = cloud
		}
		if region := sources.ExtractString(item, "spec.region"); region != "" {
			node.Metadata["region"] = region
		}
		if endpoint := sources.ExtractString(item, "spec.http_endpoint"); endpoint != "" {
			node.Metadata["endpoint"] = endpoint
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

// kafkaBrokersToTopology converts Kafka broker list to TopologyNodes.
// Input: array of broker objects from /kafka/v3/clusters/{id}/brokers.
func kafkaBrokersToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		_, ok := item.(map[string]any)
		if !ok {
			continue
		}

		brokerID := sources.ExtractString(item, "broker_id")
		if brokerID == "" {
			continue
		}

		host := sources.ExtractString(item, "host")
		name := fmt.Sprintf("broker-%s", brokerID)
		if host != "" {
			name = host
		}

		node := TopologyNode{
			Kind:     "KafkaBroker",
			Name:     name,
			Status:   "Active",
			Metadata: map[string]string{"broker_id": brokerID},
		}

		if host != "" {
			node.Metadata["host"] = host
		}
		if port := sources.ExtractString(item, "port"); port != "" {
			node.Metadata["port"] = port
		}
		if rack := sources.ExtractString(item, "rack"); rack != "" {
			node.Metadata["rack"] = rack
		}
		if clusterID := sources.ExtractString(item, "cluster_id"); clusterID != "" {
			node.Metadata["cluster_id"] = clusterID
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

// kafkaTopicsToTopology converts Kafka topics to TopologyNodes.
// Input: array of topic objects from /kafka/v3/clusters/{id}/topics.
func kafkaTopicsToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		_, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name := sources.ExtractString(item, "topic_name")
		if name == "" {
			continue
		}

		// Skip internal topics by default
		isInternal := sources.ExtractString(item, "is_internal")
		if isInternal == "true" {
			continue
		}

		node := TopologyNode{
			Kind:     "KafkaTopic",
			Name:     name,
			Status:   "Active",
			Metadata: make(map[string]string),
		}

		if partitions := sources.ExtractString(item, "partitions_count"); partitions != "" {
			node.Metadata["partitions"] = partitions
		}
		if rf := sources.ExtractString(item, "replication_factor"); rf != "" {
			node.Metadata["replication_factor"] = rf
		}
		if clusterID := sources.ExtractString(item, "cluster_id"); clusterID != "" {
			node.Metadata["cluster_id"] = clusterID
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

// kafkaConsumerGroupsToTopology converts Kafka consumer groups to TopologyNodes.
// Input: array of consumer group objects from /kafka/v3/clusters/{id}/consumer-groups.
func kafkaConsumerGroupsToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		_, ok := item.(map[string]any)
		if !ok {
			continue
		}

		groupID := sources.ExtractString(item, "consumer_group_id")
		if groupID == "" {
			continue
		}

		state := sources.ExtractString(item, "state")
		status := mapKafkaGroupState(state)

		node := TopologyNode{
			Kind:     "ConsumerGroup",
			Name:     groupID,
			Status:   status,
			Metadata: map[string]string{"state": state},
		}

		if coordinator := sources.ExtractString(item, "coordinator.related"); coordinator != "" {
			node.Metadata["coordinator"] = coordinator
		}
		if clusterID := sources.ExtractString(item, "cluster_id"); clusterID != "" {
			node.Metadata["cluster_id"] = clusterID
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

// kafkaConsumerGroupsToHealth converts consumer group states to HealthChecks.
func kafkaConsumerGroupsToHealth(rawItems []any, args map[string]any) (any, error) {
	var checks []HealthCheck
	for _, item := range rawItems {
		_, ok := item.(map[string]any)
		if !ok {
			continue
		}

		groupID := sources.ExtractString(item, "consumer_group_id")
		if groupID == "" {
			continue
		}

		state := sources.ExtractString(item, "state")
		healthStatus := mapKafkaGroupStateToHealth(state)

		checks = append(checks, HealthCheck{
			Component: groupID,
			Kind:      "ConsumerGroup",
			Status:    healthStatus,
			Message:   state,
			LastCheck: time.Now(),
		})
	}
	return checks, nil
}

// kafkaConnectorsToTopology converts Kafka Connect connectors to TopologyNodes.
// Input: connectors list from /connect/v1/.../connectors?expand=info,status.
// Confluent Cloud returns a map[connectorName]->object with info/status.
func kafkaConnectorsToTopology(rawItems []any, args map[string]any) (any, error) {
	var nodes []TopologyNode
	for _, item := range rawItems {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name := sources.ExtractString(item, "info.name")
		if name == "" {
			// Try flat structure
			name = sources.ExtractString(item, "name")
		}
		if name == "" {
			continue
		}

		connType := sources.ExtractString(item, "info.type")
		if connType == "" {
			connType = sources.ExtractString(item, "type")
		}

		connState := sources.ExtractString(item, "status.connector.state")
		if connState == "" {
			connState = sources.ExtractString(item, "status.state")
		}
		status := mapKafkaConnectorState(connState)

		node := TopologyNode{
			Kind:   "KafkaConnector",
			Name:   name,
			Status: status,
			Metadata: map[string]string{
				"state": connState,
			},
		}

		if connType != "" {
			node.Metadata["type"] = strings.ToLower(connType)
		}

		// Extract connector class if available
		if class, ok := m["info"].(map[string]any); ok {
			if config, ok := class["config"].(map[string]any); ok {
				if cls, ok := config["connector.class"].(string); ok {
					node.Metadata["class"] = cls
				}
			}
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

// kafkaConnectorsToHealth converts connector states to HealthChecks.
func kafkaConnectorsToHealth(rawItems []any, args map[string]any) (any, error) {
	var checks []HealthCheck
	for _, item := range rawItems {
		_, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name := sources.ExtractString(item, "info.name")
		if name == "" {
			name = sources.ExtractString(item, "name")
		}
		if name == "" {
			continue
		}

		connState := sources.ExtractString(item, "status.connector.state")
		if connState == "" {
			connState = sources.ExtractString(item, "status.state")
		}
		healthStatus := mapKafkaConnectorStateToHealth(connState)

		checks = append(checks, HealthCheck{
			Component: name,
			Kind:      "KafkaConnector",
			Status:    healthStatus,
			Message:   connState,
			LastCheck: time.Now(),
		})
	}
	return checks, nil
}

// --- Kafka state mapping helpers ---

func mapKafkaGroupState(state string) string {
	switch strings.ToUpper(state) {
	case "STABLE":
		return "Stable"
	case "PREPARING_REBALANCE", "COMPLETING_REBALANCE", "REBALANCING":
		return "Rebalancing"
	case "DEAD":
		return "Dead"
	case "EMPTY":
		return "Empty"
	default:
		return "Unknown"
	}
}

func mapKafkaGroupStateToHealth(state string) string {
	switch strings.ToUpper(state) {
	case "STABLE":
		return HealthStatusHealthy
	case "PREPARING_REBALANCE", "COMPLETING_REBALANCE", "REBALANCING":
		return HealthStatusDegraded
	case "DEAD":
		return HealthStatusUnhealthy
	case "EMPTY":
		return HealthStatusDegraded
	default:
		return HealthStatusUnknown
	}
}

func mapKafkaConnectorState(state string) string {
	switch strings.ToUpper(state) {
	case "RUNNING":
		return "Running"
	case "PAUSED":
		return "Paused"
	case "FAILED":
		return "Failed"
	case "UNASSIGNED":
		return "Unassigned"
	default:
		return "Unknown"
	}
}

func mapKafkaConnectorStateToHealth(state string) string {
	switch strings.ToUpper(state) {
	case "RUNNING":
		return HealthStatusHealthy
	case "PAUSED":
		return HealthStatusDegraded
	case "FAILED":
		return HealthStatusUnhealthy
	case "UNASSIGNED":
		return HealthStatusDegraded
	default:
		return HealthStatusUnknown
	}
}
