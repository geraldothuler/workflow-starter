package ops

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SlackConfig holds Slack API connection settings.
type SlackConfig struct {
	Token   string
	Channel string
	Query   string // optional keyword filter
	Window  string // time window (e.g. "1h")
}

// CheckSlack queries a Slack channel for recent messages and scans for incident keywords.
func CheckSlack(cfg SlackConfig) OpsResult {
	if cfg.Token == "" || cfg.Channel == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "Slack: missing token or channel",
			Actions: []string{"set --input slack-token=... slack-channel=..."},
			Cost:    "zero-llm",
		}
	}

	apiURL := fmt.Sprintf("https://slack.com/api/conversations.history?channel=%s&limit=100", cfg.Channel)
	headers := map[string]string{
		"Authorization": "Bearer " + cfg.Token,
		"Content-Type":  "application/json",
	}

	body, statusCode, err := httpGet(apiURL, headers)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Slack API error: %v", err),
			Cost:   "zero-llm",
		}
	}
	if statusCode != 200 {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Slack API returned HTTP %d", statusCode),
			Cost:   "zero-llm",
		}
	}

	return evaluateSlack(body)
}

func evaluateSlack(body []byte) OpsResult {
	var resp struct {
		OK       bool `json:"ok"`
		Messages []struct {
			Text string `json:"text"`
			TS   string `json:"ts"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Slack: failed to parse response: %v", err),
			Cost:   "zero-llm",
		}
	}

	if !resp.OK {
		return OpsResult{
			Status: "error",
			Signal: "Slack API returned ok=false",
			Cost:   "zero-llm",
		}
	}

	keywords := []string{"error", "critical", "incident", "outage", "down", "alert", "failure"}
	criticalCount, warnCount := 0, 0

	for _, msg := range resp.Messages {
		lower := strings.ToLower(msg.Text)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				if kw == "critical" || kw == "outage" || kw == "down" {
					criticalCount++
				} else {
					warnCount++
				}
				break
			}
		}
	}

	data := map[string]any{
		"total_messages":   len(resp.Messages),
		"critical_matches": criticalCount,
		"warn_matches":     warnCount,
	}

	hStatus, hSignal, hActions := EvalHeuristics(data, loadHeuristics("slack"))
	signal := fmt.Sprintf("Slack: %d messages, no significant anomalies", len(resp.Messages))
	if hSignal != "" {
		signal = hSignal
	}
	return OpsResult{
		Status:  hStatus,
		Signal:  signal,
		Data:    data,
		Actions: hActions,
		Cost:    "zero-llm",
	}
}
