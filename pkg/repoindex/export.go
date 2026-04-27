package repoindex

import (
	"fmt"
	"strings"
)

// ExportSummary renders a repo snapshot as markdown (architecture-compatible)
// or Datadog Service Catalog YAML. Format: "markdown" | "datadog".
func ExportSummary(db *DB, repoName, format string) (string, error) {
	snap, err := GetSnapshot(db, repoName)
	if err != nil {
		return "", err
	}
	switch format {
	case "datadog":
		return renderDatadog(snap), nil
	default:
		return renderMarkdown(snap), nil
	}
}

// renderMarkdown produces a summary.md compatible with Cobliteam/architecture format.
func renderMarkdown(snap *RepoSnapshot) string {
	var b strings.Builder
	r := snap.Repo

	b.WriteString(fmt.Sprintf("# %s - Summary\n\n", r.Name))

	if r.Owner != "" {
		b.WriteString(fmt.Sprintf("**Owner:** %s\n", r.Owner))
	}
	if r.Lang != "" {
		b.WriteString(fmt.Sprintf("**Technology:** %s / %s\n", r.Lang, r.Framework))
	}
	b.WriteString("\n---\n\n")

	// reads_from: kafka consumers + db connections + external APIs
	var reads []string
	for _, e := range snap.Events {
		if e.EventType == "kafka-consumer" && isConcreteTopicName(e.BusName) {
			reads = append(reads, fmt.Sprintf("- Kafka topic `%s`", e.BusName))
		}
	}
	for _, d := range snap.DBConnections {
		if d.Dialect != "" {
			reads = append(reads, fmt.Sprintf("- %s (%s)", d.Dialect, d.HostVar))
		}
	}
	for _, a := range snap.ExternalAPIs {
		if a.URL != "" {
			reads = append(reads, fmt.Sprintf("- %s (%s)", a.Name, a.URL))
		}
	}

	// writes_to: kafka producers + db connections (write side)
	var writes []string
	seen := map[string]bool{}
	for _, e := range snap.Events {
		if e.EventType == "kafka-producer" && isConcreteTopicName(e.BusName) && !seen[e.BusName] {
			seen[e.BusName] = true
			writes = append(writes, fmt.Sprintf("- Kafka topic `%s`", e.BusName))
		}
	}
	for _, d := range snap.DBConnections {
		if d.Dialect != "" {
			writes = append(writes, fmt.Sprintf("- %s (%s)", d.Dialect, d.HostVar))
		}
	}

	b.WriteString("<reads_from>\n")
	if len(reads) > 0 {
		b.WriteString(strings.Join(reads, "\n") + "\n")
	}
	b.WriteString("</reads_from>\n\n")

	b.WriteString("<writes_to>\n")
	if len(writes) > 0 {
		b.WriteString(strings.Join(writes, "\n") + "\n")
	}
	b.WriteString("</writes_to>\n\n")

	// Kafka topics summary table
	consumers := topicsByType(snap.Events, "kafka-consumer")
	producers := topicsByType(snap.Events, "kafka-producer")
	kafkaTopic := topicsByType(snap.Events, "kafka-topic")
	producers = append(producers, kafkaTopic...)

	if len(producers) > 0 || len(consumers) > 0 {
		b.WriteString("---\n\n## Kafka Topics Summary\n\n")
		if len(producers) > 0 {
			b.WriteString("### Produced Topics\n| Topic | Description |\n|-------|-------------|\n")
			for _, t := range producers {
				b.WriteString(fmt.Sprintf("| %s | %s |\n", t.BusName, t.Description))
			}
			b.WriteString("\n")
		}
		if len(consumers) > 0 {
			b.WriteString("### Consumed Topics\n| Topic | Description |\n|-------|-------------|\n")
			for _, t := range consumers {
				b.WriteString(fmt.Sprintf("| %s | %s |\n", t.BusName, t.Description))
			}
			b.WriteString("\n")
		}
	}

	// DB tables
	if len(snap.Models) > 0 {
		b.WriteString("---\n\n## Database Tables\n")
		for _, m := range snap.Models {
			b.WriteString(fmt.Sprintf("- %s\n", m.TableName))
		}
		b.WriteString("\n")
	}

	// External dependencies
	if len(snap.ExternalAPIs) > 0 {
		b.WriteString("---\n\n## External Service Dependencies\n")
		for _, a := range snap.ExternalAPIs {
			b.WriteString(fmt.Sprintf("- %s\n", a.Name))
		}
		b.WriteString("\n")
	}

	// Service metrics — incident triage checklist (business + key middleware)
	if len(snap.ServiceMetrics) > 0 {
		byCategory := map[string][]string{}
		for _, m := range snap.ServiceMetrics {
			byCategory[m.Category] = append(byCategory[m.Category], m.MetricName)
		}
		b.WriteString("---\n\n## Incident Triage — Key Metrics\n\n")
		for _, cat := range []string{"business", "apm", "middleware", "kafka", "flink", "jvm"} {
			names := byCategory[cat]
			if len(names) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("### %s\n", strings.Title(cat)))
			for _, n := range names {
				b.WriteString(fmt.Sprintf("- `%s`\n", n))
			}
			b.WriteString("\n")
		}
	}

	// Deployment resources from Helm charts
	hasResources := false
	for _, cs := range snap.ChartSnapshots {
		if len(cs.Resources) > 0 {
			hasResources = true
			break
		}
	}
	if hasResources {
		b.WriteString("---\n\n## Deployment Resources\n\n")
		b.WriteString("| Deployment | CPU req | CPU lim | Mem lim | Replicas | Heap |\n")
		b.WriteString("|------------|---------|---------|---------|----------|------|\n")
		for _, cs := range snap.ChartSnapshots {
			for _, r := range cs.Resources {
				rMin, rMax := "", ""
				if r.ReplicasMin > 0 {
					rMin = fmt.Sprintf("%d", r.ReplicasMin)
				}
				if r.ReplicasMax > 0 {
					rMax = fmt.Sprintf("%d", r.ReplicasMax)
				}
				replicas := ""
				if rMin != "" || rMax != "" {
					replicas = rMin + "–" + rMax
				}
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
					r.Container, r.CPURequest, r.CPULimit, r.MemLimit, replicas, r.HeapSize))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderDatadog produces a Datadog Service Catalog YAML (schema-version: v2.2).
func renderDatadog(snap *RepoSnapshot) string {
	var b strings.Builder
	r := snap.Repo

	b.WriteString("schema-version: v2.2\n")
	b.WriteString("dd-service: " + r.Name + "\n")
	if r.Owner != "" {
		b.WriteString("team: " + r.Owner + "\n")
	}
	b.WriteString(fmt.Sprintf("languages:\n  - %s\n", r.Lang))
	b.WriteString("type: web\n")
	b.WriteString("description: |\n")
	b.WriteString(fmt.Sprintf("  %s/%s service\n", r.Lang, r.Framework))

	// Tags
	b.WriteString("tags:\n")
	b.WriteString(fmt.Sprintf("  - lang:%s\n", r.Lang))
	b.WriteString(fmt.Sprintf("  - framework:%s\n", r.Framework))
	if r.Owner != "" {
		b.WriteString(fmt.Sprintf("  - team:%s\n", r.Owner))
	}
	// finops.org/service — cost attribution tag (Ivan's DD catalog convention)
	b.WriteString(fmt.Sprintf("  - finops.org/service:%s\n", r.Name))
	// image tag from latest chart snapshot (skip common placeholder values)
	for _, cs := range snap.ChartSnapshots {
		tag := cs.ImageTag
		if tag != "" && tag != "<nil>" && tag != "change-me" && tag != "latest" && !strings.HasPrefix(tag, "$(") {
			b.WriteString(fmt.Sprintf("  - image_tag:%s\n", tag))
			break
		}
	}

	// Integrations: Kafka — prefer architecture-enriched topics (with serialization),
	// fall back to LLM-extracted events if no enrichment available.
	enrichedProducers := topicEnrichmentsByDir(snap.TopicEnrichments, "produces")
	enrichedConsumers := topicEnrichmentsByDir(snap.TopicEnrichments, "consumes")
	fallbackProducers := topicsByType(snap.Events, "kafka-producer")
	fallbackConsumers := topicsByType(snap.Events, "kafka-consumer")

	hasKafka := len(enrichedProducers) > 0 || len(enrichedConsumers) > 0 ||
		len(fallbackProducers) > 0 || len(fallbackConsumers) > 0
	if hasKafka {
		b.WriteString("integrations:\n  kafka:\n")
		if len(enrichedProducers) > 0 {
			b.WriteString("    produces:\n")
			seen := map[string]bool{}
			for _, t := range enrichedProducers {
				if seen[t.Topic] {
					continue
				}
				seen[t.Topic] = true
				if t.Serialization != "" {
					b.WriteString(fmt.Sprintf("      - topic: %s\n        serialization: %s\n", t.Topic, t.Serialization))
				} else {
					b.WriteString(fmt.Sprintf("      - topic: %s\n", t.Topic))
				}
			}
		} else if len(fallbackProducers) > 0 {
			b.WriteString("    produces:\n")
			for _, t := range fallbackProducers {
				b.WriteString(fmt.Sprintf("      - topic: %s\n", t.BusName))
			}
		}
		if len(enrichedConsumers) > 0 {
			b.WriteString("    consumes:\n")
			seen := map[string]bool{}
			for _, t := range enrichedConsumers {
				if seen[t.Topic] {
					continue
				}
				seen[t.Topic] = true
				b.WriteString(fmt.Sprintf("      - topic: %s\n", t.Topic))
				if t.Serialization != "" {
					b.WriteString(fmt.Sprintf("        serialization: %s\n", t.Serialization))
				}
				if t.ConsumerGroup != "" {
					b.WriteString(fmt.Sprintf("        consumer-group: %s\n", t.ConsumerGroup))
				}
			}
		} else if len(fallbackConsumers) > 0 {
			b.WriteString("    consumes:\n")
			for _, t := range fallbackConsumers {
				b.WriteString(fmt.Sprintf("      - topic: %s\n", t.BusName))
			}
		}
	}

	// Deployment units
	if len(snap.DeploymentUnits) > 0 {
		b.WriteString("deployment-units:\n")
		for _, u := range snap.DeploymentUnits {
			if u.Deprecated {
				b.WriteString(fmt.Sprintf("  - %s [deprecated]\n", u.Name))
			} else {
				b.WriteString(fmt.Sprintf("  - %s\n", u.Name))
			}
		}
	}

	// Dependencies: external APIs + DBs
	var deps []string
	for _, d := range snap.DBConnections {
		if d.Dialect != "" {
			deps = append(deps, d.Dialect)
		}
	}
	for _, a := range snap.ExternalAPIs {
		if a.Name != "" {
			deps = append(deps, a.Name)
		}
	}
	if len(deps) > 0 {
		b.WriteString("dependencies:\n")
		seen := map[string]bool{}
		for _, d := range deps {
			if !seen[d] {
				seen[d] = true
				b.WriteString(fmt.Sprintf("  - %s\n", d))
			}
		}
	}

	// DD Monitors
	if len(snap.DDMonitors) > 0 {
		b.WriteString("monitoring:\n  monitors:\n")
		for _, m := range snap.DDMonitors {
			url := m.URL
			if url == "" {
				url = fmt.Sprintf("https://app.datadoghq.com/monitors/%s", m.MonitorID)
			}
			b.WriteString(fmt.Sprintf("    - id: %s\n      name: %s\n      type: %s\n      status: %s\n      url: %s\n",
				m.MonitorID, m.Name, m.Type, m.Status, url))
		}
	}

	return b.String()
}

// topicEnrichmentsByDir filters TopicEnrichments by direction (produces/consumes).
func topicEnrichmentsByDir(topics []TopicEnrichment, dir string) []TopicEnrichment {
	var result []TopicEnrichment
	for _, t := range topics {
		if t.Direction == dir {
			result = append(result, t)
		}
	}
	return result
}

// isConcreteTopicName returns false for placeholder values — delegates to
// isConcreteTopic in sanitize.go so both the DB layer and export use the same rule.
func isConcreteTopicName(name string) bool {
	return isConcreteTopic(name)
}

// topicsByType returns unique events with a concrete bus_name for the given event_type.
func topicsByType(events []Event, eventType string) []Event {
	seen := map[string]bool{}
	var result []Event
	for _, e := range events {
		if e.EventType == eventType && isConcreteTopicName(e.BusName) && !seen[e.BusName] {
			seen[e.BusName] = true
			result = append(result, e)
		}
	}
	return result
}
