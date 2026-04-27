package cycles

// Signal represents a single heuristic check in the cycle detector.
type Signal struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Weight int    `json:"weight"`
	Detail string `json:"detail"` // e.g. "39/39 packages"
}

// CycleResult is the output contract for cycle-end detection (zero-LLM).
type CycleResult struct {
	ShouldSavepoint bool     `json:"should_savepoint"`
	Score           int      `json:"score"`
	MaxScore        int      `json:"max_score"`
	Threshold       int      `json:"threshold"`
	Signals         []Signal `json:"signals"`
	Summary         string   `json:"summary"`
	FilesChanged    []string `json:"files_changed"`
	PackagesTouched []string `json:"packages_touched"`
	Cost            string   `json:"cost"` // always "zero-llm"
}
