package runner

import "strings"

// ResolveMappingTemplate expands ${key} and ${key:-default} placeholders in
// tmpl using values from inputs. Nested defaults are supported:
//
//	${namespace:-${environment}}   → inputs["namespace"] or inputs["environment"]
//	${profile:-cobli-tech}        → inputs["profile"] or literal "cobli-tech"
//
// A custom scanner is used instead of regexp to correctly track nested ${}
// brace depth (e.g. ${key:-${other}} has two closing braces).
func ResolveMappingTemplate(tmpl string, inputs RunInputs) string {
	var out strings.Builder
	i := 0
	for i < len(tmpl) {
		if i+1 < len(tmpl) && tmpl[i] == '$' && tmpl[i+1] == '{' {
			// Find the matching closing brace by tracking depth.
			// Every '${' increments depth; every '}' decrements it.
			depth := 1
			j := i + 2
			for j < len(tmpl) && depth > 0 {
				switch {
				case tmpl[j] == '{' && j > 0 && tmpl[j-1] == '$':
					depth++
				case tmpl[j] == '}':
					depth--
				}
				j++
			}
			if depth != 0 {
				// Unmatched brace — emit literally and advance one char.
				out.WriteByte(tmpl[i])
				i++
				continue
			}
			inner := tmpl[i+2 : j-1]
			if idx := strings.Index(inner, ":-"); idx >= 0 {
				key := inner[:idx]
				defVal := inner[idx+2:]
				if v, ok := inputs[key]; ok && v != "" {
					out.WriteString(v)
				} else {
					out.WriteString(ResolveMappingTemplate(defVal, inputs))
				}
			} else {
				out.WriteString(inputs[inner])
			}
			i = j
		} else {
			out.WriteByte(tmpl[i])
			i++
		}
	}
	return out.String()
}

// ResolveInputs applies step.InputMapping to produce a merged RunInputs.
// Mapping entries are only applied when the target key is NOT already set by
// the caller — caller-provided values always win.
//
// Template syntax: ${key}, ${key:-literal}, ${key:-${other}}
func ResolveInputs(mapping map[string]string, inputs RunInputs) RunInputs {
	if len(mapping) == 0 {
		return inputs
	}
	resolved := make(RunInputs, len(inputs)+len(mapping))
	for k, v := range inputs {
		resolved[k] = v
	}
	for k, tmpl := range mapping {
		if _, ok := resolved[k]; !ok {
			if val := ResolveMappingTemplate(tmpl, resolved); val != "" {
				resolved[k] = val
			}
		}
	}
	return resolved
}
