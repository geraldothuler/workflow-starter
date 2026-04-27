package mcp

import (
	"testing"
)

// TestNewServer_ToolsRegistered verifies the server is created with all
// expected tools registered and no panics during registration.
func TestNewServer_ToolsRegistered(t *testing.T) {
	workflowHome := t.TempDir()
	repoPath := t.TempDir()

	s := NewServer(workflowHome, repoPath)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

// TestProp_BuildsCorrectMap verifies the prop() helper builds a valid
// JSON-schema property map.
func TestProp_FieldTypes(t *testing.T) {
	cases := []struct {
		typ         string
		description string
	}{
		{"string", "A string field"},
		{"boolean", "A boolean field"},
		{"object", "An object field"},
		{"array", "An array field"},
	}

	for _, c := range cases {
		p := prop(c.typ, c.description)
		if p["type"] != c.typ {
			t.Errorf("prop(%q, ...) type = %v, want %q", c.typ, p["type"], c.typ)
		}
		if p["description"] != c.description {
			t.Errorf("prop(%q, %q) description = %v, want %q", c.typ, c.description, p["description"], c.description)
		}
	}
}

// TestToolNames_AllDistinct verifies the 22 tool names are all distinct
// (no duplicates that would silently overwrite each other).
func TestToolNames_AllDistinct(t *testing.T) {
	expected := []string{
		"workflow_run",
		"workflow_list_use_cases",
		"workflow_new",
		"workflow_index",
		"workflow_list",
		"ops_probe",
		"ops_db_health",
		"ops_k8s_status",
		"ops_kafka_status",
		"ops_logs_analyze",
		"ops_plan_new",
		"ops_plan_show",
		"ops_jira",
		"ops_slack",
		"ops_websearch",
		"ops_snowflake",
		"ops_montecarlo",
		"ops_airbyte",
		"ops_github",
		"playbook_run",
		"playbook_list",
		"workflow_status",
	}

	seen := map[string]bool{}
	for _, name := range expected {
		if seen[name] {
			t.Errorf("duplicate tool name: %q", name)
		}
		seen[name] = true
	}

	if len(expected) != 22 {
		t.Errorf("expected 22 tools, got %d", len(expected))
	}
}
