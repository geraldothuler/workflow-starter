package playbook

import "testing"

func TestBuildCausalChain_Empty(t *testing.T) {
	links := BuildCausalChain(nil, DefaultCausalRules())
	if len(links) != 0 {
		t.Errorf("expected 0 links for nil findings, got %d", len(links))
	}

	links2 := BuildCausalChain([]Finding{{ID: "f1"}}, nil)
	if len(links2) != 0 {
		t.Errorf("expected 0 links for nil rules, got %d", len(links2))
	}
}

func TestBuildCausalChain_SingleLink(t *testing.T) {
	findings := []Finding{
		{ID: "cause-1", AnalyzerName: "analyze_inactive_slots", Severity: SeverityCritical},
		{ID: "effect-1", AnalyzerName: "analyze_failed_connectors", Severity: SeverityCritical},
	}

	links := BuildCausalChain(findings, DefaultCausalRules())
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].From != "cause-1" || links[0].To != "effect-1" {
		t.Errorf("link = %s -> %s, want cause-1 -> effect-1", links[0].From, links[0].To)
	}
	if links[0].Reasoning == "" {
		t.Error("reasoning should not be empty")
	}
}

func TestBuildCausalChain_MultipleLinks(t *testing.T) {
	findings := []Finding{
		{ID: "slot-1", AnalyzerName: "analyze_inactive_slots", Severity: SeverityCritical},
		{ID: "connector-1", AnalyzerName: "analyze_failed_connectors", Severity: SeverityCritical},
		{ID: "cg-1", AnalyzerName: "analyze_empty_consumer_groups", Severity: SeverityWarning},
	}

	links := BuildCausalChain(findings, DefaultCausalRules())
	// inactive_slots -> failed_connectors + inactive_slots -> empty_consumer_groups
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
}

func TestBuildCausalChain_NoMatches(t *testing.T) {
	findings := []Finding{
		{ID: "f1", AnalyzerName: "analyze_connection_saturation", Severity: SeverityWarning},
		// No inactive_slots findings, so no link connection_saturation -> inactive_slots
	}

	links := BuildCausalChain(findings, DefaultCausalRules())
	if len(links) != 0 {
		t.Errorf("expected 0 links (no matching pairs), got %d", len(links))
	}
}

func TestBuildCausalChain_SeverityOrdering(t *testing.T) {
	findings := []Finding{
		{ID: "warn-cause", AnalyzerName: "analyze_connection_saturation", Severity: SeverityWarning},
		{ID: "crit-cause", AnalyzerName: "analyze_inactive_slots", Severity: SeverityCritical},
		{ID: "slot-effect", AnalyzerName: "analyze_inactive_slots", Severity: SeverityCritical},
		{ID: "connector-effect", AnalyzerName: "analyze_failed_connectors", Severity: SeverityCritical},
	}

	// Rules: connection_saturation -> inactive_slots, inactive_slots -> failed_connectors
	links := BuildCausalChain(findings, DefaultCausalRules())
	if len(links) < 2 {
		t.Fatalf("expected at least 2 links, got %d", len(links))
	}

	// Critical cause links should come first
	firstCause := ""
	for _, f := range findings {
		if f.ID == links[0].From {
			firstCause = f.Severity
			break
		}
	}
	if firstCause != SeverityCritical {
		t.Errorf("first link cause severity = %q, want critical", firstCause)
	}
}
