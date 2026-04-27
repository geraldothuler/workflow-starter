package sync

// Conflict representa conflito
type Conflict struct {
	Type     string `json:"type"`
	Message  string `json:"message"`
	Resolved bool   `json:"resolved"`
}
