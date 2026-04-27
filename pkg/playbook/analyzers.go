package playbook

import (
	"fmt"

	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
)

// AnalyzerFunc inspects an InfraContext and returns findings.
// It receives the accumulated findings from previous analyzers in the same execution,
// allowing cross-step awareness.
type AnalyzerFunc func(
	ic *infracontext.InfraContext,
	args map[string]any,
	accumulated []Finding,
) ([]Finding, error)

var analyzerRegistry = map[string]AnalyzerFunc{}

// RegisterAnalyzer adds an analyzer to the global registry.
func RegisterAnalyzer(name string, fn AnalyzerFunc) {
	analyzerRegistry[name] = fn
}

// CallAnalyzer invokes a registered analyzer by name.
func CallAnalyzer(name string, ic *infracontext.InfraContext, args map[string]any, findings []Finding) ([]Finding, error) {
	fn, ok := analyzerRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unknown analyzer: %q", name)
	}
	return fn(ic, args, findings)
}

// ListAnalyzers returns all registered analyzer names.
func ListAnalyzers() []string {
	names := make([]string, 0, len(analyzerRegistry))
	for name := range analyzerRegistry {
		names = append(names, name)
	}
	return names
}
