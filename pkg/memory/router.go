package memory

import (
	"fmt"
	"regexp"
	"strings"
)

// Destination classifies where a piece of knowledge should be stored.
type Destination int

const (
	DestKeychain    Destination = iota // credential or operational ID
	DestContextJSON                    // structured fact (threshold, limit, config)
	DestTopicFile                      // narrative content (heuristic, runbook, arch note)
)

// RouteResult is the output of the routing heuristic.
type RouteResult struct {
	Dest        Destination
	KeySuggestion string // suggested key name (snake_case)
	TopicFile   string   // path to the topic file (DestTopicFile only)
	TopicHint   string   // section hint for the topic file
	Command     string   // ready-to-run command suggestion
}

// String returns a human-readable routing decision.
func (r RouteResult) String() string {
	switch r.Dest {
	case DestKeychain:
		return fmt.Sprintf("KEYCHAIN\n  key: %s\n  cmd: bash ~/workflow/scripts/secret-set.sh %s <value>",
			r.KeySuggestion, r.KeySuggestion)
	case DestContextJSON:
		return fmt.Sprintf("CONTEXT_JSON\n  key: %s\n  cmd: wtb memory set %s <value>%s",
			r.KeySuggestion, r.KeySuggestion, r.Command)
	case DestTopicFile:
		hint := ""
		if r.TopicHint != "" {
			hint = "\n  hint: " + r.TopicHint
		}
		return fmt.Sprintf("TOPIC_FILE\n  file: %s%s", r.TopicFile, hint)
	}
	return "UNKNOWN"
}

// numericValueRe matches an explicit numeric value in a description.
// Required for contextPatterns routing: "limit 3200Mi" matches, "limit is high" does not.
var numericValueRe = regexp.MustCompile(`\d+\s*(?:Mi|MB|GB|ms|m|%|mCPU)?`)

// keyword-based patterns for routing decisions
var (
	keychainPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(password|passwd|secret|api.?key|token|credential)\b`),
		regexp.MustCompile(`(?i)\b(channel.?id|connection.?id|client.?id|webhook.?id|workspace.?id)\b`),
		// UUID-like value pattern
		regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
		// Slack channel ID (Cxxxx)
		regexp.MustCompile(`\bC[0-9A-Z]{9,}\b`),
	}

	contextPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(limit|threshold|warn|crit|max|min|timeout|interval|duration|count|size|replica|parallelism)\b`),
		regexp.MustCompile(`(?i)\b(Mi|MB|GB|ms|mCPU|percent|%)\b`),
		regexp.MustCompile(`(?i)\b(dashboard|monitor)\s+id\b`),
		regexp.MustCompile(`(?i)\b(endpoint|url|host|port)\b`),
	}

	// topicKeywords maps keyword patterns to (topic file relative path, section hint)
	topicKeywords = []struct {
		pattern *regexp.Regexp
		file    string
		hint    string
	}{
		{regexp.MustCompile(`(?i)\b(flink|checkpoint|taskmanager|jobmanager|rocksdb|savepoint)\b`), "memory/heuristics-ops.md", "## Flink section"},
		{regexp.MustCompile(`(?i)\b(webhook.builder|webhook.sender|alex)\b`), "memory/webhook-ss2269.md", "## Webhook section"},
		{regexp.MustCompile(`(?i)\b(ci.sentinel|ci sentinel|guardrail)\b`), "memory/heuristics-ops.md", "## ci-sentinel section"},
		{regexp.MustCompile(`(?i)\b(hikari|slick|severino|connection.?pool|keepalive)\b`), "docs/workflow/runbooks/severino.md", "## HikariCP section"},
		{regexp.MustCompile(`(?i)\b(sherlock|ibutton|identification.?token)\b`), "memory/heuristics-ops.md", "## Sherlock section"},
		{regexp.MustCompile(`(?i)\b(airbyte|cdc|sync|replication.?slot)\b`), "memory/airbyte-ops.md", "## Connections section"},
		{regexp.MustCompile(`(?i)\b(datadog|monitor|dashboard|metric|alert)\b`), "memory/datadog-ops.md", "## Monitores section"},
		{regexp.MustCompile(`(?i)\b(kotlin|gradle|spring|jvm|oom|heap|gclog)\b`), "memory/kotlin-gradle.md", "## relevant section"},
		{regexp.MustCompile(`(?i)\b(kubectl|namespace|pod|k8s|kubernetes|psql|scylla)\b`), "memory/k8s-prod-access.md", "## relevant section"},
		{regexp.MustCompile(`(?i)\b(coderabbit|code.?review|pr.review|pull.request)\b`), "memory/feedback_code_review.md", "## relevant section"},
		{regexp.MustCompile(`(?i)\b(herbie|esn|device.?id|painel)\b`), "memory/herbie-ops.md", "## relevant section"},
		{regexp.MustCompile(`(?i)\b(snowflake|montecarlo|data.quality|drift)\b`), "memory/montecarlo-ops.md", "## relevant section"},
	}
)

// Route applies the routing heuristic to determine where to store a piece of knowledge.
func Route(description string) RouteResult {
	// 1. Keychain — credentials and operational IDs
	for _, p := range keychainPatterns {
		if p.MatchString(description) {
			key := toSnakeKey(description)
			return RouteResult{
				Dest:          DestKeychain,
				KeySuggestion: key,
			}
		}
	}

	// 2. context.json — structured facts with numeric/measurable values
	// Requires an explicit number in the description to avoid routing narrative heuristics here.
	for _, p := range contextPatterns {
		if p.MatchString(description) && numericValueRe.MatchString(description) {
			key := toSnakeKey(description)
			extra := ""
			if t := inferType(description); t != "" {
				extra = fmt.Sprintf(" --type %s", t)
			}
			if topic := inferTopic(description); topic != "" {
				extra += fmt.Sprintf(" --topic %s", topic)
			}
			return RouteResult{
				Dest:          DestContextJSON,
				KeySuggestion: key,
				Command:       extra,
			}
		}
	}

	// 3. Topic file — narrative, heuristic, architectural note
	for _, kw := range topicKeywords {
		if kw.pattern.MatchString(description) {
			return RouteResult{
				Dest:      DestTopicFile,
				TopicFile: kw.file,
				TopicHint: kw.hint,
			}
		}
	}

	// Default: unknown — suggest MEMORY.md one-liner
	return RouteResult{
		Dest:      DestTopicFile,
		TopicFile: "MEMORY.md",
		TopicHint: "Add as a one-liner under ## Regras de processo",
	}
}

// toSnakeKey derives a snake_case key from a free-text description.
// Strips stop words and joins significant words with underscores.
func toSnakeKey(description string) string {
	stop := map[string]bool{
		"o": true, "a": true, "de": true, "do": true, "da": true, "em": true,
		"é": true, "e": true, "the": true, "is": true, "an": true,
		"for": true, "to": true, "in": true, "of": true, "at": true, "with": true,
		"and": true, "or": true, "that": true, "has": true, "was": true, "are": true,
	}
	// Normalize: lowercase, keep only alphanum and spaces
	re := regexp.MustCompile(`[^a-zA-Z0-9\s_-]`)
	clean := strings.ToLower(re.ReplaceAllString(description, " "))

	words := strings.Fields(clean)
	var parts []string
	for _, w := range words {
		if !stop[w] && len(w) > 1 {
			parts = append(parts, w)
		}
		if len(parts) >= 5 {
			break
		}
	}
	if len(parts) == 0 {
		return "unknown_key"
	}
	return strings.Join(parts, "_")
}

// inferType returns a type tag for context.json based on keywords.
func inferType(description string) string {
	d := strings.ToLower(description)
	switch {
	case strings.ContainsAny(d, "threshold warn crit alert"):
		return "threshold"
	case strings.Contains(d, "limit") || strings.Contains(d, "max") || strings.Contains(d, "min"):
		return "limit"
	case strings.Contains(d, "timeout") || strings.Contains(d, "interval") || strings.Contains(d, "duration") || strings.Contains(d, "ms"):
		return "timeout"
	case strings.Contains(d, "endpoint") || strings.Contains(d, "url") || strings.Contains(d, "host"):
		return "endpoint"
	case strings.Contains(d, "replica") || strings.Contains(d, "parallelism") || strings.Contains(d, "count"):
		return "config"
	}
	return "fact"
}

// inferTopic returns a topic tag for context.json based on keywords.
func inferTopic(description string) string {
	d := strings.ToLower(description)
	switch {
	case strings.Contains(d, "flink") || strings.Contains(d, "webhook") || strings.Contains(d, "taskmanager"):
		return "webhook"
	case strings.Contains(d, "airbyte") || strings.Contains(d, "cdc") || strings.Contains(d, "snowflake"):
		return "airbyte"
	case strings.Contains(d, "datadog") || strings.Contains(d, "monitor") || strings.Contains(d, "metric"):
		return "datadog"
	case strings.Contains(d, "fusca") || strings.Contains(d, "cerberus") || strings.Contains(d, "icarus"):
		return "fusca"
	case strings.Contains(d, "severino") || strings.Contains(d, "hikari"):
		return "severino"
	case strings.Contains(d, "kubernetes") || strings.Contains(d, "kubectl") || strings.Contains(d, "pod"):
		return "k8s"
	}
	return ""
}
