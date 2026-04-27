package repoindex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CategorizeMetric classifies a DD metric name for incident triage.
// Returns "" for generic infra noise (container.*, kubernetes.*, etc.) — caller should skip.
func CategorizeMetric(name, svcName string) string {
	lower := strings.ToLower(name)

	// Infra noise — skip entirely
	for _, skip := range []string{
		"container.", "kubernetes.", "kubernetes_state.", "containerd.", "cri.",
		"datadog.estimated_usage.", "dd.scorecard.", "datadog.agent.",
	} {
		if strings.HasPrefix(lower, skip) {
			return ""
		}
	}

	// Business: service-specific prefix (fusca.*, vehicle.*, cobli.*, flink.operator.<svc>.*)
	slug := strings.ToLower(strings.ReplaceAll(svcName, "-", "_"))
	if strings.HasPrefix(lower, slug+".") ||
		strings.HasPrefix(lower, "flink.operator."+slug) {
		return "business"
	}
	// Other well-known business prefixes
	if strings.HasPrefix(lower, "cobli.") || strings.HasPrefix(lower, "vehicle.") {
		return "business"
	}

	switch {
	case strings.HasPrefix(lower, "trace."):
		return "apm"
	case strings.HasPrefix(lower, "jvm."):
		return "jvm"
	case strings.HasPrefix(lower, "hikaricp."):
		return "middleware"
	case strings.HasPrefix(lower, "resilience4j."):
		return "middleware"
	case strings.HasPrefix(lower, "kafka.consumer.") || strings.HasPrefix(lower, "kafka.producer."):
		return "kafka"
	case strings.HasPrefix(lower, "flink."):
		return "flink"
	}
	return "" // skip unclassified
}

// StoreServiceMetrics replaces all metrics for a repo with the provided list.
func StoreServiceMetrics(db *DB, repoName string, metrics []ServiceMetric) error {
	repoID := slugID(repoName)
	now := time.Now().Format(time.RFC3339)

	db.sql.Exec(`DELETE FROM service_metrics WHERE repo_id=?`, repoID)

	for _, m := range metrics {
		id := slugID(fmt.Sprintf("%s-%s", repoID, m.MetricName))
		_, err := db.sql.Exec(
			`INSERT INTO service_metrics(id,repo_id,metric_name,category,fetched_at) VALUES(?,?,?,?,?)`,
			id, repoID, m.MetricName, m.Category, now)
		if err != nil {
			return fmt.Errorf("insert metric %q: %w", m.MetricName, err)
		}
	}
	return nil
}

// GetServiceMetrics returns the categorized metrics for a repo, ordered by category then name.
func GetServiceMetrics(db *DB, repoName string) ([]ServiceMetric, error) {
	repoID := slugID(repoName)
	rows, err := db.sql.Query(
		`SELECT id,repo_id,metric_name,category,fetched_at FROM service_metrics WHERE repo_id=? ORDER BY category,metric_name`,
		repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var metrics []ServiceMetric
	for rows.Next() {
		var m ServiceMetric
		rows.Scan(&m.ID, &m.RepoID, &m.MetricName, &m.Category, &m.FetchedAt)
		metrics = append(metrics, m)
	}
	return metrics, nil
}

// FetchServiceMetrics queries the DD v2 metrics API for all metrics active in the last 24h
// tagged with service:<svcName>, then categorizes and returns them.
func FetchServiceMetrics(apiKey, appKey, svcName string) ([]ServiceMetric, error) {
	// DD v2 metrics list with tag filter — paginate via cursor
	baseURL := "https://api.datadoghq.com/api/v2/metrics"
	params := url.Values{}
	params.Set("filter[tags]", fmt.Sprintf("service:%s", svcName))
	params.Set("window[seconds]", "86400") // active in last 24h
	params.Set("page[size]", "10000")

	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dd metrics api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dd metrics api status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"` // metric name is the ID in v2
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode metrics response: %w", err)
	}

	var metrics []ServiceMetric
	for _, d := range result.Data {
		cat := CategorizeMetric(d.ID, svcName)
		if cat == "" {
			continue
		}
		metrics = append(metrics, ServiceMetric{
			MetricName: d.ID,
			Category:   cat,
		})
	}
	return metrics, nil
}
