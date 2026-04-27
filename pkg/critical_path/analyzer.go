// Package critical_path provides heuristic epic ordering and dependency
// inference for generated backlogs. Zero LLM calls — aggregates existing
// pipeline data (epic metadata, deep dives, patterns) into a structured
// execution plan with phases and inferred dependencies.
package critical_path

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FOUNDATION KEYWORDS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

var foundationKeywords = []string{
	"infrastructure", "infra", "auth", "authentication", "authorization",
	"database", "db", "api", "core", "platform", "setup", "config",
	"configuration", "base", "foundation", "scaffold", "bootstrap",
	"security", "identity", "schema", "migration",
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// MAIN ENTRY POINT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// AnalyzeCriticalPath performs heuristic critical path analysis on a backlog.
// It infers epic dependencies, detects foundation epics, scores priority,
// and groups epics into execution phases. Zero LLM calls.
func AnalyzeCriticalPath(backlog *types.Backlog) *types.CriticalPathReport {
	if backlog == nil || len(backlog.Epics) == 0 {
		return &types.CriticalPathReport{
			Items:        []types.CriticalPathItem{},
			Phases:       []types.ExecutionPhase{},
			Dependencies: []types.DependencyEdge{},
			Summary:      "No epics to analyze",
		}
	}

	// Step 1: Detect foundation epics
	foundations := detectFoundations(backlog)

	// Step 2: Infer technology dependencies from deep dives
	techEdges := inferTechDependencies(backlog)

	// Step 3: Infer pattern-based dependencies
	patternEdges := analyzePatternDependencies(backlog, foundations)

	// Step 4: Merge and deduplicate edges
	allEdges := append(techEdges, patternEdges...)
	allEdges = deduplicateEdges(allEdges)

	// Step 5: Score epic priority
	scores := scoreEpicPriority(backlog, foundations, allEdges)

	// Step 6: Group into execution phases (topological sort)
	phases, items := groupExecutionPhases(backlog, scores, allEdges, foundations)

	// Step 7: Assign sequential IDs
	for i := range items {
		items[i].ID = fmt.Sprintf("CPT-%03d", i+1)
	}

	return &types.CriticalPathReport{
		Items:        items,
		Phases:       phases,
		Dependencies: allEdges,
		Summary:      buildSummary(phases, allEdges, foundations),
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 1: FOUNDATION DETECTION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// detectFoundations identifies epics that are foundational (infrastructure,
// auth, database, etc.) based on keyword matching in title, description, and tags.
// An epic is considered a foundation if it matches ≥2 keywords.
func detectFoundations(backlog *types.Backlog) map[string]bool {
	foundations := make(map[string]bool)

	for _, epic := range backlog.Epics {
		matches := 0
		searchText := strings.ToLower(epic.Title + " " + epic.Description)

		for _, kw := range foundationKeywords {
			if strings.Contains(searchText, kw) {
				matches++
			}
		}

		// Also check tags
		for _, tag := range epic.Tags {
			tagLower := strings.ToLower(fmt.Sprint(tag))
			for _, kw := range foundationKeywords {
				if strings.Contains(tagLower, kw) {
					matches++
				}
			}
		}

		if matches >= 2 {
			foundations[epic.Code] = true
		}
	}

	return foundations
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 2: TECHNOLOGY DEPENDENCY INFERENCE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// epicDeepDiveInfo holds aggregated deep dive information for an epic.
type epicDeepDiveInfo struct {
	terms           map[string]string // term -> classification (highest)
	criticalCount   int
	specificCount   int
}

// inferTechDependencies infers dependencies between epics based on
// technology overlap in deep dives. If Epic A has a critical/specific
// deep dive for a technology that Epic B has as standard, A likely
// sets up the technology and B uses it → A should come before B.
func inferTechDependencies(backlog *types.Backlog) []types.DependencyEdge {
	// Build epic -> deep dive map via Story.EpicID
	epicDDs := buildEpicDeepDiveMap(backlog)
	if len(epicDDs) < 2 {
		return nil
	}

	var edges []types.DependencyEdge
	epicCodes := make([]string, 0, len(epicDDs))
	for code := range epicDDs {
		epicCodes = append(epicCodes, code)
	}
	sort.Strings(epicCodes) // deterministic

	// Compare each pair of epics
	for i := 0; i < len(epicCodes); i++ {
		for j := 0; j < len(epicCodes); j++ {
			if i == j {
				continue
			}
			codeA := epicCodes[i]
			codeB := epicCodes[j]
			infoA := epicDDs[codeA]
			infoB := epicDDs[codeB]

			// Check shared technologies
			for term, classA := range infoA.terms {
				classB, exists := infoB.terms[term]
				if !exists {
					continue
				}

				// A has critical/specific, B has standard → A before B
				if (classA == "critical" || classA == "specific") && classB == "standard" {
					edges = append(edges, types.DependencyEdge{
						From:       codeA,
						To:         codeB,
						Type:       "technology",
						Confidence: 0.8,
						Reasoning:  fmt.Sprintf("%s configures %s (%s) while %s uses it (%s)", codeA, term, classA, codeB, classB),
					})
				}
			}
		}
	}

	return edges
}

// buildEpicDeepDiveMap links epic codes to their deep dives via Story.EpicID.
func buildEpicDeepDiveMap(backlog *types.Backlog) map[string]*epicDeepDiveInfo {
	// Build story -> epic code map
	storyToEpic := make(map[string]string)
	for _, epic := range backlog.Epics {
		for _, story := range epic.Stories {
			storyToEpic[story.ID] = epic.Code
		}
	}

	// Group deep dives by epic
	epicDDs := make(map[string]*epicDeepDiveInfo)
	for _, dd := range backlog.DeepDives {
		epicCode := storyToEpic[dd.StoryID]
		if epicCode == "" {
			// Try matching by scope if no StoryID
			if dd.Scope == "global" {
				continue // Global deep dives don't belong to specific epics
			}
			continue
		}

		if epicDDs[epicCode] == nil {
			epicDDs[epicCode] = &epicDeepDiveInfo{
				terms: make(map[string]string),
			}
		}

		info := epicDDs[epicCode]
		termLower := strings.ToLower(dd.Term)

		// Keep highest classification for each term
		existing := info.terms[termLower]
		if classificationRank(dd.Classification) > classificationRank(existing) {
			info.terms[termLower] = dd.Classification
		}

		switch dd.Classification {
		case "critical":
			info.criticalCount++
		case "specific":
			info.specificCount++
		}
	}

	return epicDDs
}

// classificationRank returns numeric rank for deep dive classification.
func classificationRank(classification string) int {
	switch classification {
	case "critical":
		return 4
	case "specific":
		return 3
	case "standard":
		return 2
	case "trivial":
		return 1
	default:
		return 0
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 3: PATTERN DEPENDENCY ANALYSIS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// analyzePatternDependencies infers dependencies between epics based on
// shared pattern suggestions. If a pattern affects multiple epics,
// the foundation epic is likely the source.
func analyzePatternDependencies(backlog *types.Backlog, foundations map[string]bool) []types.DependencyEdge {
	if len(backlog.PatternSuggestions) == 0 {
		return nil
	}

	var edges []types.DependencyEdge

	for _, ps := range backlog.PatternSuggestions {
		if len(ps.AffectedEpics) < 2 || ps.Type == "anti-pattern" {
			continue
		}

		// Find the "source" epic: prefer foundation, else highest complexity
		sourceEpic := ""
		for _, epicCode := range ps.AffectedEpics {
			if foundations[epicCode] {
				sourceEpic = epicCode
				break
			}
		}

		if sourceEpic == "" {
			// Use highest complexity epic as source
			maxComplexity := -1
			for _, epicCode := range ps.AffectedEpics {
				for _, epic := range backlog.Epics {
					if epic.Code == epicCode && epic.Complexity > maxComplexity {
						maxComplexity = epic.Complexity
						sourceEpic = epicCode
					}
				}
			}
		}

		if sourceEpic == "" {
			continue
		}

		// Create edges from source to other affected epics
		for _, epicCode := range ps.AffectedEpics {
			if epicCode == sourceEpic {
				continue
			}
			edges = append(edges, types.DependencyEdge{
				From:       sourceEpic,
				To:         epicCode,
				Type:       "pattern",
				Confidence: 0.6,
				Reasoning:  fmt.Sprintf("Pattern %q affects both %s and %s; %s likely establishes it", ps.PatternName, sourceEpic, epicCode, sourceEpic),
			})
		}
	}

	return edges
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 4: EPIC PRIORITY SCORING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// scoreEpicPriority calculates a priority score (1-100) for each epic.
// Higher score = should start earlier.
// Formula: foundation_bonus(30) + complexity(×3, max 30) + critical_tech(×5, max 20) + dependents(×5, max 20)
func scoreEpicPriority(backlog *types.Backlog, foundations map[string]bool, edges []types.DependencyEdge) map[string]int {
	scores := make(map[string]int)

	// Count dependents (how many epics depend on this one)
	dependentCount := make(map[string]int)
	for _, edge := range edges {
		dependentCount[edge.From]++
	}

	// Build epic deep dive map for tech counts
	epicDDs := buildEpicDeepDiveMap(backlog)

	for _, epic := range backlog.Epics {
		score := 0

		// Foundation bonus: +30
		if foundations[epic.Code] {
			score += 30
		}

		// Complexity weight: complexity × 3, max 30
		complexityScore := epic.Complexity * 3
		if complexityScore > 30 {
			complexityScore = 30
		}
		score += complexityScore

		// Critical tech count: ×5, max 20
		if info, ok := epicDDs[epic.Code]; ok {
			techScore := (info.criticalCount + info.specificCount) * 5
			if techScore > 20 {
				techScore = 20
			}
			score += techScore
		}

		// Dependents count: ×5, max 20
		depScore := dependentCount[epic.Code] * 5
		if depScore > 20 {
			depScore = 20
		}
		score += depScore

		// Cap at 100
		if score > 100 {
			score = 100
		}

		scores[epic.Code] = score
	}

	return scores
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ANALYZER 5: EXECUTION PHASE GROUPING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// groupExecutionPhases performs topological sort (Kahn's algorithm)
// on the dependency graph and groups epics into execution phases.
// Phase 1 = no dependencies, Phase 2 = depends on Phase 1, etc.
func groupExecutionPhases(
	backlog *types.Backlog,
	scores map[string]int,
	edges []types.DependencyEdge,
	foundations map[string]bool,
) ([]types.ExecutionPhase, []types.CriticalPathItem) {
	// Build adjacency list and in-degree
	inDegree := make(map[string]int)
	adjList := make(map[string][]string)
	epicCodes := make([]string, 0)

	for _, epic := range backlog.Epics {
		inDegree[epic.Code] = 0
		epicCodes = append(epicCodes, epic.Code)
	}

	for _, edge := range edges {
		// Only count edges for epics that exist in backlog
		if _, ok := inDegree[edge.From]; !ok {
			continue
		}
		if _, ok := inDegree[edge.To]; !ok {
			continue
		}
		adjList[edge.From] = append(adjList[edge.From], edge.To)
		inDegree[edge.To]++
	}

	// Kahn's algorithm: BFS topological sort by levels
	var phases []types.ExecutionPhase
	var items []types.CriticalPathItem
	visited := make(map[string]bool)

	// Build epic lookup
	epicMap := make(map[string]*types.Epic)
	for i := range backlog.Epics {
		epicMap[backlog.Epics[i].Code] = &backlog.Epics[i]
	}

	phaseNum := 0
	for len(visited) < len(epicCodes) {
		phaseNum++

		// Find all nodes with in-degree 0 (not yet visited)
		var ready []string
		for _, code := range epicCodes {
			if !visited[code] && inDegree[code] == 0 {
				ready = append(ready, code)
			}
		}

		// If no ready nodes but unvisited remain → cycle detected, break
		if len(ready) == 0 {
			// Add remaining nodes in a single catch-all phase
			for _, code := range epicCodes {
				if !visited[code] {
					ready = append(ready, code)
				}
			}
			if len(ready) == 0 {
				break
			}
		}

		// Sort ready nodes by priority (highest first)
		sort.Slice(ready, func(i, j int) bool {
			return scores[ready[i]] > scores[ready[j]]
		})

		// Calculate total effort for this phase
		totalEffort := 0
		for _, code := range ready {
			if epic, ok := epicMap[code]; ok {
				for _, story := range epic.Stories {
					totalEffort += story.Effort
				}
			}
		}

		// Build reasoning
		reasoning := buildPhaseReasoning(phaseNum, ready, foundations, edges)

		phase := types.ExecutionPhase{
			Phase:       phaseNum,
			EpicCodes:   ready,
			Parallel:    len(ready) > 1,
			TotalEffort: totalEffort,
			Reasoning:   reasoning,
		}
		phases = append(phases, phase)

		// Build items for this phase
		for _, code := range ready {
			epic := epicMap[code]
			if epic == nil {
				continue
			}

			// Find dependencies for this epic
			var dependsOn []string
			for _, edge := range edges {
				if edge.To == code {
					dependsOn = append(dependsOn, edge.From)
				}
			}

			// Build tags
			var tags []string
			if foundations[code] {
				tags = append(tags, "foundation")
			}
			if epic.Complexity >= 7 {
				tags = append(tags, "high-complexity")
			}

			item := types.CriticalPathItem{
				EpicCode:     code,
				EpicTitle:    epic.Title,
				Phase:        phaseNum,
				Priority:     scores[code],
				Reasoning:    buildItemReasoning(code, foundations, dependsOn, scores[code]),
				DependsOn:    dependsOn,
				IsFoundation: foundations[code],
				Tags:         tags,
			}
			items = append(items, item)

			visited[code] = true
		}

		// Decrease in-degree for neighbors
		for _, code := range ready {
			for _, neighbor := range adjList[code] {
				inDegree[neighbor]--
			}
		}
	}

	return phases, items
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// deduplicateEdges removes duplicate From/To pairs, keeping the highest confidence.
func deduplicateEdges(edges []types.DependencyEdge) []types.DependencyEdge {
	type edgeKey struct{ from, to string }
	best := make(map[edgeKey]types.DependencyEdge)

	for _, edge := range edges {
		key := edgeKey{edge.From, edge.To}
		if existing, ok := best[key]; !ok || edge.Confidence > existing.Confidence {
			best[key] = edge
		}
	}

	result := make([]types.DependencyEdge, 0, len(best))
	for _, edge := range best {
		result = append(result, edge)
	}

	// Sort for deterministic output
	sort.Slice(result, func(i, j int) bool {
		if result[i].From != result[j].From {
			return result[i].From < result[j].From
		}
		return result[i].To < result[j].To
	})

	return result
}

// buildSummary generates a human-readable summary of the critical path analysis.
func buildSummary(phases []types.ExecutionPhase, edges []types.DependencyEdge, foundations map[string]bool) string {
	foundationCount := 0
	for range foundations {
		foundationCount++
	}

	parts := []string{}
	parts = append(parts, fmt.Sprintf("%d phase(s)", len(phases)))
	if foundationCount > 0 {
		parts = append(parts, fmt.Sprintf("%d foundation epic(s)", foundationCount))
	}
	parts = append(parts, fmt.Sprintf("%d inferred dependenc(ies)", len(edges)))

	return strings.Join(parts, ", ")
}

// buildPhaseReasoning generates reasoning for a phase grouping.
func buildPhaseReasoning(phaseNum int, epicCodes []string, foundations map[string]bool, edges []types.DependencyEdge) string {
	if phaseNum == 1 {
		hasFoundation := false
		for _, code := range epicCodes {
			if foundations[code] {
				hasFoundation = true
				break
			}
		}
		if hasFoundation {
			return "Foundation epics with no dependencies — start here"
		}
		return "Epics with no dependencies — can start immediately"
	}

	// Find which phases this depends on
	depPhases := make(map[string]bool)
	for _, edge := range edges {
		for _, code := range epicCodes {
			if edge.To == code {
				depPhases[edge.From] = true
			}
		}
	}

	if len(depPhases) > 0 {
		deps := make([]string, 0, len(depPhases))
		for dep := range depPhases {
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		return fmt.Sprintf("Depends on: %s", strings.Join(deps, ", "))
	}

	return fmt.Sprintf("Phase %d execution group", phaseNum)
}

// buildItemReasoning generates reasoning for a single epic's position.
func buildItemReasoning(code string, foundations map[string]bool, dependsOn []string, priority int) string {
	parts := []string{}

	if foundations[code] {
		parts = append(parts, "foundation epic")
	}

	if len(dependsOn) > 0 {
		parts = append(parts, fmt.Sprintf("depends on %s", strings.Join(dependsOn, ", ")))
	} else {
		parts = append(parts, "no dependencies")
	}

	parts = append(parts, fmt.Sprintf("priority %d", priority))

	return strings.Join(parts, "; ")
}
