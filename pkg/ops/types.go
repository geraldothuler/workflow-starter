package ops

// OpsResult is the standard output contract for all wtb ops commands.
//
// Signal is the primary field consumed by LLMs — a single human-readable sentence
// summarising state and trend. Data contains structured values for heuristic decisions.
// Actions holds zero-LLM suggestions computed entirely in Go.
type OpsResult struct {
	Status  string         `json:"status"`  // ok | warn | critical | error
	Signal  string         `json:"signal"`  // intent summary for LLM consumption
	Data    map[string]any `json:"data"`    // structured data for decision making
	Actions []string       `json:"actions"` // heuristic next actions (zero-LLM)
	Cost    string         `json:"cost"`    // zero-llm | low | medium
}
