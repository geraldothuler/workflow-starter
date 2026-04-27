package repoindex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SetDDMonitor upserts a Datadog monitor reference for a repo.
func SetDDMonitor(db *DB, repoName, monitorID, name, mtype, status string) error {
	repoID := slugID(repoName)
	id := slugID(fmt.Sprintf("%s-%s", repoID, monitorID))
	now := time.Now().Format(time.RFC3339)
	_, err := db.sql.Exec(`
		INSERT INTO dd_monitors(id, repo_id, monitor_id, name, type, status, fetched_at)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, type=excluded.type, status=excluded.status, fetched_at=excluded.fetched_at`,
		id, repoID, monitorID, name, mtype, status, now)
	return err
}

// GetDDMonitors returns cached Datadog monitors for a repo.
func GetDDMonitors(db *DB, repoName string) ([]DDMonitor, error) {
	repoID := slugID(repoName)
	rows, err := db.sql.Query(`SELECT id,repo_id,monitor_id,name,type,status,url,fetched_at FROM dd_monitors WHERE repo_id=? ORDER BY name`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var monitors []DDMonitor
	for rows.Next() {
		var m DDMonitor
		rows.Scan(&m.ID, &m.RepoID, &m.MonitorID, &m.Name, &m.Type, &m.Status, &m.URL, &m.FetchedAt)
		monitors = append(monitors, m)
	}
	return monitors, nil
}

// FetchDDMonitors queries the Datadog API for monitors tagged with service:<svcName>.
func FetchDDMonitors(apiKey, appKey, svcName string) ([]DDMonitor, error) {
	query := url.QueryEscape(fmt.Sprintf("service:%s", svcName))
	reqURL := fmt.Sprintf("https://api.datadoghq.com/api/v1/monitor/search?query=%s&per_page=50", query)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dd api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dd api status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Monitors []struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			Type   string `json:"type"`
			Status string `json:"overall_state"`
		} `json:"monitors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode dd response: %w", err)
	}

	var monitors []DDMonitor
	for _, m := range result.Monitors {
		monitors = append(monitors, DDMonitor{
			MonitorID: fmt.Sprintf("%d", m.ID),
			Name:      m.Name,
			Type:      m.Type,
			Status:    m.Status,
			URL:       fmt.Sprintf("https://app.datadoghq.com/monitors/%d", m.ID),
		})
	}
	return monitors, nil
}
