package repoindex

import (
	"fmt"
	"sort"
	"strings"
)

// TopicNode represents a Kafka topic with its known producers and consumers (repo names).
type TopicNode struct {
	Topic     string
	Producers []string
	Consumers []string
}

// ImpactResult describes what changes when a set of repos are merged into one.
type ImpactResult struct {
	MergedRepos     []string
	InternalTopics  []string // produced AND consumed within merged set — can be eliminated
	ExternalInputs  []string // consumed by merged set, produced outside — must remain
	ExternalOutputs []string // produced by merged set, consumed outside — must remain
	OrphanTopics    []string // produced by merged set, no consumers anywhere in DB

	// HTTP synchronous coupling between repos in the merged set
	InternalHTTPCalls []InternalHTTPCall

	// DB tables accessed by 2+ repos in the merged set
	SharedDBTables []SharedDBTable

	// Proto models referenced by 2+ repos in the merged set
	SharedProtoContracts []SharedProtoContract
}

// BuildTopology returns the full Kafka topic dependency graph across all indexed repos.
// Topics with config-key bus_names (app.*, ${...}, <dynamic>) are excluded.
func BuildTopology(db *DB) ([]TopicNode, error) {
	rows, err := db.sql.Query(`
		SELECT r.name, e.event_type, e.bus_name
		FROM events e
		JOIN repos r ON r.id = e.repo_id
		WHERE e.bus_name != ''
		ORDER BY e.bus_name, r.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type entry struct {
		producers map[string]bool
		consumers map[string]bool
	}
	topics := map[string]*entry{}

	for rows.Next() {
		var repoName, eventType, busName string
		if err := rows.Scan(&repoName, &eventType, &busName); err != nil {
			continue
		}
		if isConfigKeyBusName(busName) {
			continue
		}
		if _, ok := topics[busName]; !ok {
			topics[busName] = &entry{
				producers: map[string]bool{},
				consumers: map[string]bool{},
			}
		}
		switch {
		case strings.Contains(eventType, "producer") || eventType == "produced":
			topics[busName].producers[repoName] = true
		case strings.Contains(eventType, "consumer") || eventType == "consumed":
			topics[busName].consumers[repoName] = true
		}
	}

	var nodes []TopicNode
	for topic, e := range topics {
		node := TopicNode{Topic: topic}
		for r := range e.producers {
			node.Producers = append(node.Producers, r)
		}
		for r := range e.consumers {
			node.Consumers = append(node.Consumers, r)
		}
		sort.Strings(node.Producers)
		sort.Strings(node.Consumers)
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Topic < nodes[j].Topic })
	return nodes, nil
}

// isConfigKeyBusName returns true for bus_names that are config placeholders, not real topic names.
func isConfigKeyBusName(s string) bool {
	return strings.HasPrefix(s, "app.") ||
		strings.HasPrefix(s, "${") ||
		strings.Contains(s, "<dynamic") ||
		strings.Contains(s, ".queue.") ||
		strings.Contains(s, ".topic.")
}

// FilterTopology returns nodes relevant to the given repo or topic filters.
func FilterTopology(nodes []TopicNode, repoFilter, topicFilter string) []TopicNode {
	var out []TopicNode
	for _, n := range nodes {
		if topicFilter != "" && !strings.Contains(n.Topic, topicFilter) {
			continue
		}
		if repoFilter != "" && !strSliceContains(n.Producers, repoFilter) && !strSliceContains(n.Consumers, repoFilter) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// RenderTopologyTable renders topology as an ASCII table.
func RenderTopologyTable(nodes []TopicNode) string {
	cols := []string{"topic", "producers", "consumers"}
	var rows [][]string
	for _, n := range nodes {
		producers := strings.Join(n.Producers, ", ")
		consumers := strings.Join(n.Consumers, ", ")
		if producers == "" {
			producers = "?"
		}
		if consumers == "" {
			consumers = "(none indexed)"
		}
		rows = append(rows, []string{n.Topic, producers, consumers})
	}
	if len(rows) == 0 {
		return "(no results)\n"
	}
	return RenderTable(cols, rows)
}

// RenderTopologyDOT renders topology as a Graphviz DOT digraph.
// Repos are boxes (blue), topics are ellipses (yellow).
func RenderTopologyDOT(nodes []TopicNode) string {
	var sb strings.Builder
	sb.WriteString("digraph kafka_topology {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [fontname=\"Helvetica\" fontsize=11];\n\n")

	// Collect unique repos and topics
	repoSet := map[string]bool{}
	topicSet := map[string]bool{}
	for _, n := range nodes {
		topicSet[n.Topic] = true
		for _, r := range n.Producers {
			repoSet[r] = true
		}
		for _, r := range n.Consumers {
			repoSet[r] = true
		}
	}

	// Topic nodes
	sb.WriteString("  // topics\n")
	sortedTopics := sortedKeys(topicSet)
	for _, t := range sortedTopics {
		sb.WriteString(fmt.Sprintf("  %q [shape=ellipse style=filled fillcolor=lightyellow];\n", t))
	}

	// Repo nodes
	sb.WriteString("\n  // repos\n")
	sortedRepos := sortedKeys(repoSet)
	for _, r := range sortedRepos {
		sb.WriteString(fmt.Sprintf("  %q [shape=box style=filled fillcolor=lightblue];\n", r))
	}

	// Edges: repo→topic (produce) and topic→repo (consume)
	sb.WriteString("\n  // edges\n")
	for _, n := range nodes {
		for _, prod := range n.Producers {
			sb.WriteString(fmt.Sprintf("  %q -> %q;\n", prod, n.Topic))
		}
		for _, cons := range n.Consumers {
			sb.WriteString(fmt.Sprintf("  %q -> %q;\n", n.Topic, cons))
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}

// ImpactAnalysis returns what changes when the given repos are merged into one.
func ImpactAnalysis(db *DB, repoNames []string) (*ImpactResult, error) {
	nodes, err := BuildTopology(db)
	if err != nil {
		return nil, err
	}

	merged := map[string]bool{}
	for _, r := range repoNames {
		merged[r] = true
	}

	result := &ImpactResult{MergedRepos: repoNames}

	for _, node := range nodes {
		producedByMerged := anyInSet(node.Producers, merged)
		consumedByMerged := anyInSet(node.Consumers, merged)
		hasExternalConsumers := anyNotInSet(node.Consumers, merged)
		hasExternalProducers := anyNotInSet(node.Producers, merged)

		switch {
		case producedByMerged && consumedByMerged && !hasExternalConsumers && !hasExternalProducers:
			// Both sides are within the merged set and nobody outside uses it
			result.InternalTopics = append(result.InternalTopics, node.Topic)
		case producedByMerged && !consumedByMerged && len(node.Consumers) == 0:
			result.OrphanTopics = append(result.OrphanTopics, node.Topic)
		case producedByMerged && hasExternalConsumers:
			result.ExternalOutputs = append(result.ExternalOutputs, node.Topic)
		case consumedByMerged && hasExternalProducers:
			result.ExternalInputs = append(result.ExternalInputs, node.Topic)
		}
	}

	sort.Strings(result.InternalTopics)
	sort.Strings(result.ExternalInputs)
	sort.Strings(result.ExternalOutputs)
	sort.Strings(result.OrphanTopics)

	// Gap 3: internal HTTP calls (non-fatal — skip on error)
	result.InternalHTTPCalls, _ = detectInternalHTTPCalls(db, repoNames)

	// Gap 2: shared DB tables (non-fatal)
	result.SharedDBTables, _ = detectSharedDBTables(db, repoNames)

	// Gap 1: shared proto contracts (non-fatal)
	result.SharedProtoContracts, _ = detectSharedProtoContracts(db, repoNames)

	return result, nil
}

// RenderImpact renders an ImpactResult as a human-readable summary.
func RenderImpact(r *ImpactResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Merge impact: %s\n\n", strings.Join(r.MergedRepos, " + ")))
	sb.WriteString(fmt.Sprintf("Jobs eliminated:  %d  (from %d → 1)\n", len(r.MergedRepos)-1, len(r.MergedRepos)))

	if len(r.InternalTopics) > 0 {
		sb.WriteString(fmt.Sprintf("\nInternal topics eliminated (%d):\n", len(r.InternalTopics)))
		for _, t := range r.InternalTopics {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
	} else {
		sb.WriteString("\nInternal topics eliminated: none\n")
	}

	if len(r.ExternalInputs) > 0 {
		sb.WriteString(fmt.Sprintf("\nExternal inputs (must remain, %d):\n", len(r.ExternalInputs)))
		for _, t := range r.ExternalInputs {
			sb.WriteString(fmt.Sprintf("  < %s\n", t))
		}
	}

	if len(r.ExternalOutputs) > 0 {
		sb.WriteString(fmt.Sprintf("\nExternal outputs (must remain, %d):\n", len(r.ExternalOutputs)))
		for _, t := range r.ExternalOutputs {
			sb.WriteString(fmt.Sprintf("  > %s\n", t))
		}
	}

	if len(r.OrphanTopics) > 0 {
		sb.WriteString(fmt.Sprintf("\nOrphan topics (no consumers indexed, %d):\n", len(r.OrphanTopics)))
		for _, t := range r.OrphanTopics {
			sb.WriteString(fmt.Sprintf("  ? %s\n", t))
		}
	}

	if len(r.InternalHTTPCalls) > 0 {
		sb.WriteString(fmt.Sprintf("\nInternal HTTP calls (sync coupling, %d):\n", len(r.InternalHTTPCalls)))
		for _, c := range r.InternalHTTPCalls {
			sb.WriteString(fmt.Sprintf("  ⇄ %s → %s  [%s]  %s\n", c.FromRepo, c.ToRepo, c.Via, truncateDetail(c.Detail, 60)))
		}
		sb.WriteString("  → merging eliminates these network hops\n")
	}

	if len(r.SharedDBTables) > 0 {
		sb.WriteString(fmt.Sprintf("\nShared DB tables (write coupling warning, %d):\n", len(r.SharedDBTables)))
		for _, t := range r.SharedDBTables {
			ext := ""
			if len(t.ExternalRepos) > 0 {
				ext = fmt.Sprintf("  [also: %s]", strings.Join(t.ExternalRepos, ", "))
			}
			sb.WriteString(fmt.Sprintf("  ! %-35s %s%s\n", t.TableName, strings.Join(t.MergedRepos, " + "), ext))
		}
	}

	if len(r.SharedProtoContracts) > 0 {
		sb.WriteString(fmt.Sprintf("\nShared proto contracts (remain external, %d):\n", len(r.SharedProtoContracts)))
		for _, c := range r.SharedProtoContracts {
			sb.WriteString(fmt.Sprintf("  < %-40s %s\n", c.ModelName, strings.Join(c.Repos, " + ")))
		}
	}

	return sb.String()
}

func truncateDetail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func anyInSet(slice []string, set map[string]bool) bool {
	for _, s := range slice {
		if set[s] {
			return true
		}
	}
	return false
}

func anyNotInSet(slice []string, set map[string]bool) bool {
	for _, s := range slice {
		if !set[s] {
			return true
		}
	}
	return false
}

func strSliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
