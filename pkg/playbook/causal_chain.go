package playbook

// CausalRule maps a cause analyzer to an effect analyzer with reasoning.
type CausalRule struct {
	CauseAnalyzer  string
	EffectAnalyzer string
	Reasoning      string
}

// DefaultCausalRules returns the built-in causal rules for CDC investigations.
func DefaultCausalRules() []CausalRule {
	return []CausalRule{
		{
			CauseAnalyzer:  "analyze_inactive_slots",
			EffectAnalyzer: "analyze_failed_connectors",
			Reasoning:      "Inactive replication slot causes CDC connector to lose position and fail",
		},
		{
			CauseAnalyzer:  "analyze_inactive_slots",
			EffectAnalyzer: "analyze_empty_consumer_groups",
			Reasoning:      "Inactive replication slot stops data flow, consumer groups become empty",
		},
		{
			CauseAnalyzer:  "analyze_failed_connectors",
			EffectAnalyzer: "analyze_unhealthy_pods",
			Reasoning:      "Failed Kafka connector causes pipeline pods to crash or restart",
		},
		{
			CauseAnalyzer:  "analyze_wal_lag",
			EffectAnalyzer: "analyze_inactive_slots",
			Reasoning:      "Excessive WAL lag indicates consumer fell behind, triggering slot deactivation",
		},
		{
			CauseAnalyzer:  "analyze_connection_saturation",
			EffectAnalyzer: "analyze_inactive_slots",
			Reasoning:      "Connection saturation prevents replication clients from connecting",
		},
	}
}

// BuildCausalChain links findings based on causal rules.
// For each rule, if findings from both cause and effect analyzers exist,
// creates a CausalLink from cause to effect.
func BuildCausalChain(findings []Finding, rules []CausalRule) []CausalLink {
	if len(findings) == 0 || len(rules) == 0 {
		return nil
	}

	// Index findings by analyzer name
	byAnalyzer := make(map[string][]Finding)
	for _, f := range findings {
		byAnalyzer[f.AnalyzerName] = append(byAnalyzer[f.AnalyzerName], f)
	}

	var links []CausalLink

	// Severity priority for ordering: critical > warning > info
	severityOrder := map[string]int{
		SeverityCritical: 0,
		SeverityWarning:  1,
		SeverityInfo:     2,
	}

	for _, rule := range rules {
		causes, hasCause := byAnalyzer[rule.CauseAnalyzer]
		effects, hasEffect := byAnalyzer[rule.EffectAnalyzer]
		if !hasCause || !hasEffect {
			continue
		}

		// Link each cause to each effect
		for _, cause := range causes {
			for _, effect := range effects {
				links = append(links, CausalLink{
					From:      cause.ID,
					To:        effect.ID,
					Reasoning: rule.Reasoning,
				})
			}
		}
		_ = severityOrder // ordering done by rule order (critical rules first)
	}

	// Sort: links with critical causes first
	findingByID := make(map[string]Finding)
	for _, f := range findings {
		findingByID[f.ID] = f
	}

	sortCausalLinks(links, findingByID, severityOrder)

	return links
}

func sortCausalLinks(links []CausalLink, findingByID map[string]Finding, severityOrder map[string]int) {
	// Simple insertion sort (N is small)
	for i := 1; i < len(links); i++ {
		for j := i; j > 0; j-- {
			a := findingByID[links[j-1].From]
			b := findingByID[links[j].From]
			orderA := severityOrder[a.Severity]
			orderB := severityOrder[b.Severity]
			if orderA > orderB {
				links[j-1], links[j] = links[j], links[j-1]
			} else {
				break
			}
		}
	}
}
