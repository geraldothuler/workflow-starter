package sources

import (
	"fmt"
	"sort"
	"strings"
)

// JSONWalker converts arbitrary JSON data to structured markdown.
// It traverses the JSON tree depth-first, applying rules from WalkerConfig
// to determine how each key/value is rendered.
type JSONWalker struct {
	config WalkerConfig
}

// WalkerConfig holds pre-computed sets for fast lookup.
type WalkerConfig struct {
	MaxDepth    int
	HeadingKeys map[string]bool
	ListKeys    map[string]bool
	SkipKeys    map[string]bool
	ValueKeys   map[string]bool
	CodeKeys    map[string]bool
}

// NewJSONWalker creates a walker from a WalkerSpec.
// If spec is nil, sensible defaults are used.
func NewJSONWalker(spec *WalkerSpec) *JSONWalker {
	var merged *WalkerSpec
	if spec == nil {
		merged = DefaultWalkerConfig()
	} else {
		merged = spec.Merged()
	}

	return &JSONWalker{
		config: WalkerConfig{
			MaxDepth:    merged.MaxDepth,
			HeadingKeys: toSet(merged.HeadingKeys),
			ListKeys:    toSet(merged.ListKeys),
			SkipKeys:    toSet(merged.SkipKeys),
			ValueKeys:   toSet(merged.ValueKeys),
			CodeKeys:    toSet(merged.CodeKeys),
		},
	}
}

// Walk converts arbitrary JSON data (as any) to markdown.
func (w *JSONWalker) Walk(data any) string {
	var sb strings.Builder
	w.walkNode(&sb, data, 0, "")
	return sb.String()
}

// walkNode recursively processes a JSON node.
func (w *JSONWalker) walkNode(sb *strings.Builder, node any, depth int, context string) {
	if depth > w.config.MaxDepth {
		return
	}

	switch v := node.(type) {
	case map[string]any:
		w.walkObject(sb, v, depth, context)
	case []any:
		w.walkArray(sb, v, depth, context)
	case string:
		if v != "" {
			sb.WriteString(v)
			sb.WriteString("\n")
		}
	case float64:
		sb.WriteString(fmt.Sprintf("%v", v))
		sb.WriteString("\n")
	case bool:
		sb.WriteString(fmt.Sprintf("%v", v))
		sb.WriteString("\n")
	case nil:
		// skip nil values
	}
}

// walkObject processes a JSON object (map).
func (w *JSONWalker) walkObject(sb *strings.Builder, obj map[string]any, depth int, context string) {
	// First, check for heading key to create a section heading
	heading := w.findHeading(obj)
	if heading != "" {
		w.writeHeading(sb, heading, depth)
	}

	// Get sorted keys for deterministic output
	keys := sortedKeys(obj)

	for _, key := range keys {
		value := obj[key]

		// Skip configured keys
		if w.config.SkipKeys[key] {
			continue
		}

		// Skip heading key (already rendered as heading)
		if w.config.HeadingKeys[key] && heading != "" {
			continue
		}

		// Handle by key role
		switch {
		case w.config.CodeKeys[key]:
			w.writeCodeBlock(sb, value)

		case w.config.ValueKeys[key]:
			w.writeKeyValue(sb, key, value)

		case w.config.ListKeys[key]:
			if arr, ok := value.([]any); ok {
				for _, item := range arr {
					w.walkNode(sb, item, depth+1, key)
				}
			}

		default:
			// Recurse into nested structures
			switch val := value.(type) {
			case map[string]any:
				w.walkNode(sb, val, depth+1, key)

			case []any:
				if len(val) > 0 {
					if w.isArrayOfObjects(val) {
						// Array of objects → recurse each
						for _, item := range val {
							w.walkNode(sb, item, depth+1, key)
						}
					} else {
						// Array of primitives → bullet list
						w.writeBulletList(sb, key, val)
					}
				}

			case string:
				if val != "" && !w.config.HeadingKeys[key] {
					w.writeKeyValue(sb, key, val)
				}

			case float64, bool:
				w.writeKeyValue(sb, key, val)

			case nil:
				// skip nil
			}
		}
	}

	// Add spacing after objects at low depth
	if depth <= 2 && heading != "" {
		sb.WriteString("\n")
	}
}

// walkArray processes a JSON array.
func (w *JSONWalker) walkArray(sb *strings.Builder, arr []any, depth int, context string) {
	if len(arr) == 0 {
		return
	}

	if w.isArrayOfObjects(arr) {
		for _, item := range arr {
			w.walkNode(sb, item, depth, context)
		}
	} else {
		for _, item := range arr {
			sb.WriteString("- ")
			sb.WriteString(fmt.Sprintf("%v", item))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
}

// findHeading looks for a heading key in an object.
func (w *JSONWalker) findHeading(obj map[string]any) string {
	// Check heading keys in order of priority
	orderedKeys := []string{"name", "title", "label"}
	for _, key := range orderedKeys {
		if w.config.HeadingKeys[key] {
			if val, ok := obj[key]; ok {
				if s, ok := val.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	// Check any other heading keys
	for key := range w.config.HeadingKeys {
		if val, ok := obj[key]; ok {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// writeHeading writes a markdown heading at the appropriate depth.
// Depth 0 → ##, Depth 1 → ##, Depth 2 → ###, etc. (H1 is reserved for page title)
func (w *JSONWalker) writeHeading(sb *strings.Builder, text string, depth int) {
	level := depth + 1 // depth 0 → H1, depth 1 → H2
	if level < 2 {
		level = 2 // Minimum H2 (H1 reserved for page title added by ConfigSource)
	}
	if level > 5 {
		level = 5
	}
	sb.WriteString(strings.Repeat("#", level))
	sb.WriteString(" ")
	sb.WriteString(text)
	sb.WriteString("\n\n")
}

// writeKeyValue writes a bold key-value pair.
func (w *JSONWalker) writeKeyValue(sb *strings.Builder, key string, value any) {
	sb.WriteString("**")
	sb.WriteString(key)
	sb.WriteString(":** ")
	sb.WriteString(fmt.Sprintf("%v", value))
	sb.WriteString("\n")
}

// writeCodeBlock writes a fenced code block.
func (w *JSONWalker) writeCodeBlock(sb *strings.Builder, value any) {
	text := fmt.Sprintf("%v", value)
	if text == "" {
		return
	}
	sb.WriteString("```\n")
	sb.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```\n\n")
}

// writeBulletList writes an array of primitives as a bullet list.
func (w *JSONWalker) writeBulletList(sb *strings.Builder, key string, items []any) {
	sb.WriteString("**")
	sb.WriteString(key)
	sb.WriteString(":**\n")
	for _, item := range items {
		sb.WriteString("- ")
		sb.WriteString(fmt.Sprintf("%v", item))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
}

// isArrayOfObjects checks if all items in the array are maps.
func (w *JSONWalker) isArrayOfObjects(arr []any) bool {
	if len(arr) == 0 {
		return false
	}
	for _, item := range arr {
		if _, ok := item.(map[string]any); !ok {
			return false
		}
	}
	return true
}

// --- Helpers ---

func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return map[string]bool{}
	}
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
