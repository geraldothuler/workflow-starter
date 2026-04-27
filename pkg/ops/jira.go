package ops

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
)

// JiraConfig holds Jira API connection settings.
type JiraConfig struct {
	URL      string // e.g. https://company.atlassian.net
	Email    string
	APIToken string
	Project  string
	JQL      string // optional custom JQL
}

// CheckJira queries Jira for open P0/P1 issues in the project.
func CheckJira(cfg JiraConfig) OpsResult {
	if cfg.URL == "" || cfg.Email == "" || cfg.APIToken == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "Jira: missing credentials (URL, email, or API token)",
			Actions: []string{"set --input jira-url=..., jira-email=..., jira-token=..."},
			Cost:    "zero-llm",
		}
	}

	jql := cfg.JQL
	if jql == "" {
		jql = fmt.Sprintf("project = %s AND status != Done AND priority in (Highest, High) ORDER BY priority ASC, created DESC", cfg.Project)
	}

	apiURL := fmt.Sprintf("%s/rest/api/3/search?jql=%s&maxResults=50", cfg.URL, url.QueryEscape(jql))
	auth := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.APIToken))
	headers := map[string]string{
		"Authorization": "Basic " + auth,
		"Accept":        "application/json",
	}

	body, statusCode, err := httpGet(apiURL, headers)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Jira API error: %v", err),
			Cost:   "zero-llm",
		}
	}
	if statusCode != 200 {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Jira API returned HTTP %d", statusCode),
			Cost:   "zero-llm",
		}
	}

	return evaluateJira(body)
}

func evaluateJira(body []byte) OpsResult {
	var resp struct {
		Total  int `json:"total"`
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary  string `json:"summary"`
				Priority struct {
					Name string `json:"name"`
				} `json:"priority"`
				Status struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		} `json:"issues"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Jira: failed to parse response: %v", err),
			Cost:   "zero-llm",
		}
	}

	p0Count, p1Count := 0, 0
	for _, issue := range resp.Issues {
		switch issue.Fields.Priority.Name {
		case "Highest":
			p0Count++
		case "High":
			p1Count++
		}
	}

	data := map[string]any{
		"total":    resp.Total,
		"p0_count": p0Count,
		"p1_count": p1Count,
	}

	base := fmt.Sprintf("Jira: %d open (%d P0, %d P1)", resp.Total, p0Count, p1Count)
	hStatus, hSignal, hActions := EvalHeuristics(data, loadHeuristics("jira"))
	signal := base
	if hSignal != "" {
		signal += " | " + hSignal
	}

	return OpsResult{
		Status:  hStatus,
		Signal:  signal,
		Data:    data,
		Actions: hActions,
		Cost:    "zero-llm",
	}
}
