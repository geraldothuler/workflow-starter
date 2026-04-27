package sources

import (
	"fmt"
	"strings"
)

// ExtractPath retrieves a value from nested JSON using dot-notation.
// "." returns the root. "a.b.c" traverses nested maps.
// Returns nil if the path is not found.
func ExtractPath(data any, path string) any {
	if data == nil {
		return nil
	}
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return data
	}

	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

// ExtractString is a convenience that extracts a value and returns it as string.
// Returns "" if the path is not found or the value is nil.
func ExtractString(data any, path string) string {
	v := ExtractPath(data, path)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// ExtractMap extracts a value and asserts it as map[string]any.
// Returns nil if the path is not found or the value is not a map.
func ExtractMap(data any, path string) map[string]any {
	v := ExtractPath(data, path)
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// ExtractSlice extracts a value and asserts it as []any.
// Returns nil if the path is not found or the value is not a slice.
func ExtractSlice(data any, path string) []any {
	v := ExtractPath(data, path)
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}
