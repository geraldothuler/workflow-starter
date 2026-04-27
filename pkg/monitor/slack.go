package monitor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpGet performs an HTTP GET with custom headers (package-local, mirrors pkg/ops pattern).
var httpGet = func(url string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// slackMessage represents a single Slack message from conversations.history.
type slackMessage struct {
	Text    string `json:"text"`
	TS      string `json:"ts"`
	Channel string `json:"-"` // filled in by the caller
}

// PollChannels polls the configured Slack channels for new messages since lastTS.
// lastTS maps channel name → last seen timestamp (empty string = fetch latest 10).
// Returns a MonitorResult with all matched signals across channels.
func PollChannels(cfg *SlackMonitorConfig, token string, lastTS map[string]string) MonitorResult {
	result := MonitorResult{
		ChannelsPolled: cfg.Channels,
		Cost:           "zero-llm",
	}

	authHeader := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	for _, ch := range cfg.Channels {
		oldest := ""
		if ts, ok := lastTS[ch]; ok {
			oldest = ts
		}

		apiURL := fmt.Sprintf(
			"https://slack.com/api/conversations.history?channel=%s&limit=50",
			ch,
		)
		if oldest != "" {
			apiURL += "&oldest=" + oldest
		}

		body, statusCode, err := httpGet(apiURL, authHeader)
		if err != nil || statusCode != 200 {
			continue
		}

		msgs, err := parseSlackHistory(body, ch)
		if err != nil {
			continue
		}

		result.NewMessages += len(msgs)

		for _, msg := range msgs {
			score, level, matched := scoreMessage(msg.Text, cfg.Signals)
			if score == 0 {
				continue
			}
			result.Signals = append(result.Signals, MonitorSignal{
				Channel:   ch,
				Timestamp: msg.TS,
				Text:      truncate(msg.Text, 120),
				Score:     score,
				Level:     level,
				Keywords:  matched,
				ThreadURL: buildThreadURL(ch, msg.TS),
			})
		}
	}

	return result
}

// scoreMessage scores a message text against the configured signal levels.
// Returns (score, level, matchedKeywords). Score 0 means no match.
func scoreMessage(text string, signals map[string]SignalConfig) (score int, level string, matched []string) {
	lower := strings.ToLower(text)

	// Check p0 first (higher priority), then p1, then remaining levels.
	priority := []string{"p0", "p1"}
	seen := map[string]bool{}
	for _, l := range priority {
		seen[l] = true
	}
	// Append any extra levels from config in stable order.
	for l := range signals {
		if !seen[l] {
			priority = append(priority, l)
		}
	}

	for _, lvl := range priority {
		sig, ok := signals[lvl]
		if !ok {
			continue
		}
		var hits []string
		for _, kw := range sig.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				hits = append(hits, kw)
			}
		}
		if len(hits) > 0 {
			return sig.Score, lvl, hits
		}
	}
	return 0, "", nil
}

// buildThreadURL constructs a Slack thread URL from channel name and timestamp.
// The timestamp format from the API is "1234567890.123456" — we strip the dot for the URL.
func buildThreadURL(channel, ts string) string {
	// ts from Slack API: "1234567890.123456" → URL uses "1234567890123456"
	slug := strings.ReplaceAll(ts, ".", "")
	return fmt.Sprintf("https://slack.com/app_redirect?channel=%s&message_ts=%s", channel, slug)
}

// parseSlackHistory unmarshals the conversations.history response and returns messages.
func parseSlackHistory(body []byte, channel string) ([]slackMessage, error) {
	var resp struct {
		OK       bool           `json:"ok"`
		Messages []slackMessage `json:"messages"`
		Error    string         `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("slack API error: %s", resp.Error)
	}
	for i := range resp.Messages {
		resp.Messages[i].Channel = channel
	}
	return resp.Messages, nil
}

// truncate shortens text to maxLen characters, appending "..." if cut.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
