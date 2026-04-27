package repoindex

import (
	"regexp"
	"strings"
)

// SanitizeRule is a function that filters or corrects an ExtractedLayer.
type SanitizeRule func(ExtractedLayer) ExtractedLayer

// reBootstrapAddr matches host:port patterns typical of Kafka bootstrap servers
// (e.g. "kafka:9092", "broker1.example.com:9093,broker2:9093").
var reBootstrapAddr = regexp.MustCompile(`[\w.\-]+(:\d{2,5})(,[\w.\-]+:\d{2,5})*`)

// rejectBootstrapBusName removes events whose bus_name looks like a bootstrap
// server address rather than an actual topic name.
func rejectBootstrapBusName(layer ExtractedLayer) ExtractedLayer {
	filtered := layer.Events[:0]
	for _, e := range layer.Events {
		if e.BusName != "" && reBootstrapAddr.MatchString(e.BusName) {
			continue // skip — looks like host:port, not a topic
		}
		filtered = append(filtered, e)
	}
	layer.Events = filtered
	return layer
}

// rejectDynamicBusName removes events whose bus_name is a placeholder,
// runtime-dynamic value, or LLM-generated description rather than a concrete
// Kafka topic name. Kafka topic names: lowercase, digits, hyphens, underscores,
// dots — no spaces, no parentheses, no code expressions.
func rejectDynamicBusName(layer ExtractedLayer) ExtractedLayer {
	filtered := layer.Events[:0]
	for _, e := range layer.Events {
		if !isConcreteTopic(e.BusName) {
			continue
		}
		filtered = append(filtered, e)
	}
	layer.Events = filtered
	return layer
}

// isConcreteTopic returns true only when the string looks like an actual Kafka
// topic name. Topic names are kebab-case / snake_case / dot-separated identifiers
// — no spaces, no parentheses, not a config key path, not a code expression.
func isConcreteTopic(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)

	// Structural disqualifiers — anything the LLM would add as a description
	for _, bad := range []string{
		" ",             // spaces → it's a phrase, not a topic
		"(",             // parentheses → description or code expression
		"${", "$(",      // template placeholders
		"dynamic",       // catch-all for "dynamic topic" descriptions
		"configured",    // "configured at runtime", "configured via..."
		"call site",     // "caller-supplied", "call site"
		"passed at",     // "passed at runtime"
		"not explicit",  // "topic name not explicit in properties"
		"resolved at",   // "resolved at runtime"
		"not specified", // "topic name not specified"
		"runtime",       // general runtime-dynamic marker
		"<",             // <dynamic>, <topic>
	} {
		if strings.Contains(lower, bad) {
			return false
		}
	}

	// Config key path disqualifiers — e.g. "app.sink.geofence-topic.name",
	// "spring.kafka.consumer.group-id"
	if strings.HasPrefix(lower, "app.") ||
		strings.HasPrefix(lower, "spring.") ||
		strings.HasPrefix(lower, "kafka.") {
		return false
	}

	// Consumer group ID disqualifiers — names ending in "-consumer" or "-group"
	// are group IDs, not topic names (e.g. "total-costs-of-ownership-consumer")
	if strings.HasSuffix(lower, "-consumer") ||
		strings.HasSuffix(lower, "-group") ||
		strings.HasSuffix(lower, "-consumer-group") {
		return false
	}

	// Code expression disqualifiers — contains method call syntax
	if strings.Contains(lower, ".topicname") ||
		strings.Contains(lower, ".kafkatopic()") ||
		strings.Contains(lower, ".topic()") {
		return false
	}

	return true
}

// rejectLocalhostURLs removes external_apis whose URL refers to localhost or
// loopback — these are dev/test stubs, not real dependencies.
func rejectLocalhostURLs(layer ExtractedLayer) ExtractedLayer {
	filtered := layer.ExternalAPIs[:0]
	for _, a := range layer.ExternalAPIs {
		lower := strings.ToLower(a.URL)
		if strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1") {
			continue
		}
		filtered = append(filtered, a)
	}
	layer.ExternalAPIs = filtered
	return layer
}

// defaultRules is the ordered slice of sanitization rules applied after each
// LLM extraction, before the layer is persisted to the database.
var defaultRules = []SanitizeRule{
	rejectBootstrapBusName,
	rejectDynamicBusName,
	rejectLocalhostURLs,
}

// sanitize applies all defaultRules to an ExtractedLayer and returns the result.
func sanitize(layer ExtractedLayer) ExtractedLayer {
	for _, rule := range defaultRules {
		layer = rule(layer)
	}
	return layer
}
