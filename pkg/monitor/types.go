package monitor

// MonitorSignal represents a single message match from a Slack channel.
type MonitorSignal struct {
	Channel   string   `json:"channel"`
	Timestamp string   `json:"ts"`
	Text      string   `json:"text"`
	Score     int      `json:"score"`    // 0–100
	Level     string   `json:"level"`    // "p0" | "p1"
	Keywords  []string `json:"keywords"` // matched keywords
	ThreadURL string   `json:"thread_url"`
}

// MonitorResult is the output of a single polling cycle.
type MonitorResult struct {
	Signals        []MonitorSignal `json:"signals"`
	ChannelsPolled []string        `json:"channels_polled"`
	NewMessages    int             `json:"new_messages"`
	Cost           string          `json:"cost"` // always "zero-llm"
}
