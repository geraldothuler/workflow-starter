package playbook

import (
	"sort"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
)

func TestRegisterAndCallAnalyzer(t *testing.T) {
	name := "test_analyzer_" + t.Name()
	called := false

	RegisterAnalyzer(name, func(ic *infracontext.InfraContext, args map[string]any, acc []Finding) ([]Finding, error) {
		called = true
		return []Finding{{ID: "f1", Title: "test finding"}}, nil
	})

	findings, err := CallAnalyzer(name, &infracontext.InfraContext{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("analyzer was not called")
	}
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].ID != "f1" {
		t.Errorf("finding id = %q", findings[0].ID)
	}
}

func TestCallAnalyzer_Unknown(t *testing.T) {
	_, err := CallAnalyzer("nonexistent_analyzer_xyz", nil, nil, nil)
	if err == nil {
		t.Error("expected error for unknown analyzer")
	}
}

func TestListAnalyzers(t *testing.T) {
	name := "test_list_analyzer_" + t.Name()
	RegisterAnalyzer(name, func(ic *infracontext.InfraContext, args map[string]any, acc []Finding) ([]Finding, error) {
		return nil, nil
	})

	list := ListAnalyzers()
	sort.Strings(list)

	found := false
	for _, n := range list {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("registered analyzer %q not found in list", name)
	}
}
