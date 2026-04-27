package repoindex

import (
	"fmt"
	"sort"
	"strings"
)

// Canvas is the consolidated view of a repo for migration planning.
type Canvas struct {
	Repo Repo

	// Pipeline position
	Consumes []CanvasEdge // topics this repo consumes, with their known producers
	Produces []CanvasEdge // topics this repo produces, with their known consumers

	// Dependency weight
	DBConnections []DBConnection
	ExternalAPIs  []ExternalAPI
	ProviderCount int  // grep count of *Provider classes
	HasBlockingIO bool // grep for Await.result / Thread.sleep / blocking patterns
	DepScore      string

	// State complexity
	StateModels     []Model // models with flink-* dialect
	SerializerCount int     // handlers with flink-serializer trigger_type
	StateScore      string

	// Blast radius
	DownstreamRepos []string // repos that lose input if this repo stops producing
	BlastScore      string

	// Merge candidates
	Upstream   []MergeCandidate // repos whose outputs feed this repo
	Downstream []MergeCandidate // repos that directly consume this repo's outputs

	// Proto contracts — schema-registry models referenced by this repo
	ProtoContracts []SchemaDep

	// Shared config keys — env/SSM keys used by this repo and at least one other
	SharedConfigKeys []SharedConfigKey

	// Public API surface — endpoints documented in the OpenAPI spec that map to this service
	PublicEndpoints []PublicEndpoint
	// HandlerGaps — handler paths that have no corresponding public_endpoint
	HandlerGaps []string
}

// CanvasEdge is a topic with its known peer repos (producers or consumers).
type CanvasEdge struct {
	Topic string
	Repos []string // producers or consumers
}

// MergeCandidate is a repo adjacent in the topology with a coupling assessment.
type MergeCandidate struct {
	Repo     string
	Via      string // topic name
	Coupling string // HIGH / MEDIUM / LOW
}

// BuildCanvas constructs a Canvas for the named repo.
func BuildCanvas(db *DB, repoName string) (*Canvas, error) {
	snap, err := GetSnapshot(db, repoName)
	if err != nil {
		return nil, err
	}

	nodes, err := BuildTopology(db)
	if err != nil {
		return nil, err
	}

	c := &Canvas{Repo: snap.Repo}

	// ── pipeline position ──────────────────────────────────────────────────
	topicMap := map[string]*TopicNode{}
	for i := range nodes {
		topicMap[nodes[i].Topic] = &nodes[i]
	}

	for _, ev := range snap.Events {
		if isConfigKeyBusName(ev.BusName) || ev.BusName == "" {
			continue
		}
		node := topicMap[ev.BusName]
		switch {
		case strings.Contains(ev.EventType, "consumer") || ev.EventType == "consumed":
			edge := CanvasEdge{Topic: ev.BusName}
			if node != nil {
				for _, p := range node.Producers {
					if p != repoName {
						edge.Repos = append(edge.Repos, p)
					}
				}
			}
			c.Consumes = append(c.Consumes, edge)
		case strings.Contains(ev.EventType, "producer") || ev.EventType == "produced":
			edge := CanvasEdge{Topic: ev.BusName}
			if node != nil {
				for _, con := range node.Consumers {
					if con != repoName {
						edge.Repos = append(edge.Repos, con)
					}
				}
			}
			c.Produces = append(c.Produces, edge)
		}
	}
	deduplicateEdges(&c.Consumes)
	deduplicateEdges(&c.Produces)

	// ── dependency weight ──────────────────────────────────────────────────
	c.DBConnections = snap.DBConnections
	c.ExternalAPIs = snap.ExternalAPIs

	// grep for Provider classes and blocking I/O patterns
	provMatches, _ := GrepRepos(db, GrepOptions{
		Pattern:    `class\s+\w+Provider|object\s+\w+Provider`,
		RepoFilter: repoName,
		ExtFilter:  []string{".scala", ".kt"},
		MaxMatches: 100,
	})
	seen := map[string]bool{}
	for _, m := range provMatches {
		key := m.File + ":" + m.Line[:min(40, len(m.Line))]
		if !seen[key] {
			seen[key] = true
			c.ProviderCount++
		}
	}

	blockingMatches, _ := GrepRepos(db, GrepOptions{
		Pattern:    `Await\.result|\.get\(\)|Thread\.sleep|runBlocking`,
		RepoFilter: repoName,
		ExtFilter:  []string{".scala", ".kt"},
		MaxMatches: 20,
	})
	c.HasBlockingIO = len(blockingMatches) > 0

	c.DepScore = depScore(len(c.DBConnections), len(c.ExternalAPIs), c.ProviderCount, c.HasBlockingIO)

	// ── state complexity ───────────────────────────────────────────────────
	for _, m := range snap.Models {
		if strings.HasPrefix(m.Dialect, "flink") {
			c.StateModels = append(c.StateModels, m)
		}
	}
	for _, h := range snap.Handlers {
		if h.TriggerType == "flink-serializer" {
			c.SerializerCount++
		}
	}
	c.StateScore = stateScore(len(c.StateModels), c.SerializerCount)

	// ── blast radius ───────────────────────────────────────────────────────
	downstreamSet := map[string]bool{}
	for _, edge := range c.Produces {
		for _, r := range edge.Repos {
			downstreamSet[r] = true
		}
	}
	for r := range downstreamSet {
		c.DownstreamRepos = append(c.DownstreamRepos, r)
	}
	sort.Strings(c.DownstreamRepos)
	c.BlastScore = blastScore(c.DownstreamRepos, nodes)

	// ── merge candidates ───────────────────────────────────────────────────
	upstreamSet := map[string]bool{}
	for _, edge := range c.Consumes {
		for _, r := range edge.Repos {
			upstreamSet[r] = true
		}
	}
	for r := range upstreamSet {
		via := topicVia(c.Consumes, r)
		c.Upstream = append(c.Upstream, MergeCandidate{
			Repo:     r,
			Via:      via,
			Coupling: couplingScore(via, nodes),
		})
	}
	for r := range downstreamSet {
		via := topicVia(c.Produces, r)
		c.Downstream = append(c.Downstream, MergeCandidate{
			Repo:     r,
			Via:      via,
			Coupling: couplingScore(via, nodes),
		})
	}
	sort.Slice(c.Upstream, func(i, j int) bool { return c.Upstream[i].Repo < c.Upstream[j].Repo })
	sort.Slice(c.Downstream, func(i, j int) bool { return c.Downstream[i].Repo < c.Downstream[j].Repo })

	// ── proto contracts ────────────────────────────────────────────────────────
	c.ProtoContracts, _ = QuerySchemaDepsByRepo(db, repoName)

	// ── shared config keys ─────────────────────────────────────────────────────
	c.SharedConfigKeys, _ = detectSharedConfigKeys(db, repoName)

	// ── public API surface ─────────────────────────────────────────────────────
	c.PublicEndpoints, _ = QueryPublicEndpointsForRepo(db, repoName)
	c.HandlerGaps, _ = QueryHandlerGaps(db, repoName)

	return c, nil
}

// RenderCanvas renders a Canvas as a human-readable report.
func RenderCanvas(c *Canvas) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("══ CANVAS: %s (%s / %s) ══\n\n", c.Repo.Name, c.Repo.Lang, c.Repo.Framework))

	// pipeline position
	sb.WriteString("── PIPELINE POSITION ──────────────────────────────────────────\n")
	if len(c.Consumes) == 0 {
		sb.WriteString("  ← (no consumers indexed)\n")
	}
	for _, e := range c.Consumes {
		producers := "?"
		if len(e.Repos) > 0 {
			producers = strings.Join(e.Repos, ", ")
		}
		sb.WriteString(fmt.Sprintf("  ← [%s] ← %s\n", e.Topic, producers))
	}
	sb.WriteString(fmt.Sprintf("       %s\n", c.Repo.Name))
	if len(c.Produces) == 0 {
		sb.WriteString("  → (no producers indexed)\n")
	}
	for _, e := range c.Produces {
		consumers := "(none indexed)"
		if len(e.Repos) > 0 {
			consumers = strings.Join(e.Repos, ", ")
		}
		sb.WriteString(fmt.Sprintf("  → [%s] → %s\n", e.Topic, consumers))
	}

	// dependency weight
	sb.WriteString(fmt.Sprintf("\n── DEPENDENCY WEIGHT: %s ─────────────────────────────────────\n", c.DepScore))
	if len(c.DBConnections) > 0 {
		var dbs []string
		for _, d := range c.DBConnections {
			dbs = append(dbs, d.Dialect)
		}
		sb.WriteString(fmt.Sprintf("  DB connections : %s\n", strings.Join(dbs, ", ")))
	}
	if len(c.ExternalAPIs) > 0 {
		sb.WriteString(fmt.Sprintf("  External APIs  : %d\n", len(c.ExternalAPIs)))
	}
	if c.ProviderCount > 0 {
		sb.WriteString(fmt.Sprintf("  Provider classes: %d\n", c.ProviderCount))
	}
	if c.HasBlockingIO {
		sb.WriteString("  Blocking I/O   : YES (Await.result / runBlocking detected)\n")
	}
	if len(c.DBConnections) == 0 && len(c.ExternalAPIs) == 0 && c.ProviderCount == 0 {
		sb.WriteString("  (no external dependencies indexed)\n")
	}

	// state complexity
	sb.WriteString(fmt.Sprintf("\n── STATE COMPLEXITY: %s ──────────────────────────────────────\n", c.StateScore))
	if len(c.StateModels) > 0 {
		for _, m := range c.StateModels {
			sb.WriteString(fmt.Sprintf("  FSM state : %s (%s, %d fields)\n", m.Name, m.Dialect, len(m.Fields)))
		}
	}
	if c.SerializerCount > 0 {
		sb.WriteString(fmt.Sprintf("  Custom serializers: %d\n", c.SerializerCount))
	}
	if len(c.StateModels) == 0 && c.SerializerCount == 0 {
		sb.WriteString("  (stateless or no state indexed)\n")
	}

	// blast radius
	sb.WriteString(fmt.Sprintf("\n── BLAST RADIUS: %s ──────────────────────────────────────────\n", c.BlastScore))
	if len(c.DownstreamRepos) > 0 {
		sb.WriteString(fmt.Sprintf("  If this job stops: %s\n", strings.Join(c.DownstreamRepos, ", ")))
	} else {
		sb.WriteString("  No downstream consumers indexed\n")
	}

	// merge candidates
	sb.WriteString("\n── MERGE CANDIDATES ───────────────────────────────────────────\n")
	if len(c.Upstream) == 0 && len(c.Downstream) == 0 {
		sb.WriteString("  (no adjacent repos in topology)\n")
	}
	for _, m := range c.Upstream {
		sb.WriteString(fmt.Sprintf("  ↑ upstream  : %-35s via %-30s coupling: %s\n", m.Repo, m.Via, m.Coupling))
	}
	for _, m := range c.Downstream {
		sb.WriteString(fmt.Sprintf("  ↓ downstream: %-35s via %-30s coupling: %s\n", m.Repo, m.Via, m.Coupling))
	}

	// proto contracts
	if len(c.ProtoContracts) > 0 {
		sb.WriteString("\n── PROTO CONTRACTS ─────────────────────────────────────────────\n")
		for _, d := range c.ProtoContracts {
			sb.WriteString(fmt.Sprintf("  %-40s %d refs\n", d.ModelName, d.MatchCount))
		}
	}

	// shared config keys
	if len(c.SharedConfigKeys) > 0 {
		sb.WriteString("\n── SHARED CONFIG KEYS ──────────────────────────────────────────\n")
		sb.WriteString("  (operational coupling — changing these values affects multiple services)\n")
		for _, k := range c.SharedConfigKeys {
			others := make([]string, 0, len(k.Repos)-1)
			for _, r := range k.Repos {
				if r != c.Repo.Name {
					others = append(others, r)
				}
			}
			sb.WriteString(fmt.Sprintf("  ~ %-40s also: %s\n", k.Key, strings.Join(others, ", ")))
		}
	}

	// public API surface
	if len(c.PublicEndpoints) > 0 || len(c.HandlerGaps) > 0 {
		sb.WriteString("\n── PUBLIC API SURFACE ──────────────────────────────────────────\n")
		for _, ep := range c.PublicEndpoints {
			authLabel := ep.AuthType
			opID := ep.OperationID
			if opID == "" {
				opID = ep.Summary
			}
			sb.WriteString(fmt.Sprintf("  %-6s %-50s %-30s [%s]\n", ep.Method, ep.Path, opID, authLabel))
		}
		if len(c.HandlerGaps) > 0 {
			sb.WriteString(fmt.Sprintf("  GAP: %d handler(s) sem cobertura pública:\n", len(c.HandlerGaps)))
			for _, g := range c.HandlerGaps {
				sb.WriteString(fmt.Sprintf("    %s\n", g))
			}
		}
		if len(c.PublicEndpoints) == 0 {
			sb.WriteString("  (nenhum endpoint documentado no api-docs para este serviço)\n")
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// ── scoring helpers ────────────────────────────────────────────────────────────

func depScore(dbConns, apis, providers int, blocking bool) string {
	if blocking || providers >= 4 || dbConns+apis >= 4 {
		return "HEAVY"
	}
	if providers >= 2 || dbConns+apis >= 2 {
		return "MEDIUM"
	}
	return "LOW"
}

func stateScore(stateModels, serializers int) string {
	if stateModels >= 3 || serializers >= 3 {
		return "COMPLEX"
	}
	if stateModels >= 1 || serializers >= 1 {
		return "MODERATE"
	}
	return "SIMPLE"
}

func blastScore(downstreamRepos []string, nodes []TopicNode) string {
	if len(downstreamRepos) == 0 {
		return "LOW"
	}
	// Check if any downstream repo is itself a hub (produces to 3+ consumers)
	for _, r := range downstreamRepos {
		consumerCount := 0
		for _, n := range nodes {
			if strSliceContains(n.Producers, r) {
				consumerCount += len(n.Consumers)
			}
		}
		if consumerCount >= 3 {
			return "CRITICAL"
		}
	}
	if len(downstreamRepos) >= 3 {
		return "HIGH"
	}
	if len(downstreamRepos) >= 1 {
		return "MEDIUM"
	}
	return "LOW"
}

// couplingScore rates coupling based on how many other consumers the topic has.
// Exclusive (1 consumer) = HIGH coupling. Shared (many) = LOW.
func couplingScore(topic string, nodes []TopicNode) string {
	for _, n := range nodes {
		if n.Topic == topic {
			if len(n.Consumers) == 1 {
				return "HIGH"
			}
			if len(n.Consumers) <= 2 {
				return "MEDIUM"
			}
			return "LOW"
		}
	}
	return "MEDIUM"
}

func topicVia(edges []CanvasEdge, repo string) string {
	for _, e := range edges {
		for _, r := range e.Repos {
			if r == repo {
				return e.Topic
			}
		}
	}
	return "?"
}

func deduplicateEdges(edges *[]CanvasEdge) {
	seen := map[string]bool{}
	var out []CanvasEdge
	for _, e := range *edges {
		if !seen[e.Topic] {
			seen[e.Topic] = true
			out = append(out, e)
		}
	}
	*edges = out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
