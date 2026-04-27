package ops

import (
	"encoding/json"
	"fmt"
	"math"
)

// AirbyteConfig holds Airbyte API connection settings.
type AirbyteConfig struct {
	URL         string // e.g. https://airbyte.internal
	APIKey      string
	WorkspaceID string
}

// CheckAirbyte queries Airbyte for connection sync health.
func CheckAirbyte(cfg AirbyteConfig) OpsResult {
	if cfg.URL == "" || cfg.WorkspaceID == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "Airbyte: missing URL or workspace ID",
			Actions: []string{"set --input airbyte-url=..., airbyte-workspace-id=..."},
			Cost:    "zero-llm",
		}
	}

	headers := map[string]string{
		"Accept": "application/json",
	}
	if cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + cfg.APIKey
	}

	// Fetch connections
	connURL := fmt.Sprintf("%s/api/v1/connections?workspaceId=%s", cfg.URL, cfg.WorkspaceID)
	body, statusCode, err := httpGet(connURL, headers)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Airbyte API error: %v", err),
			Cost:   "zero-llm",
		}
	}
	if statusCode != 200 {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Airbyte API returned HTTP %d", statusCode),
			Cost:   "zero-llm",
		}
	}

	return evaluateAirbyte(body)
}

func evaluateAirbyte(body []byte) OpsResult {
	var resp struct {
		Connections []struct {
			ConnectionID        string `json:"connectionId"`
			Name                string `json:"name"`
			Status              string `json:"status"`
			LatestSyncJobStatus string `json:"latestSyncJobStatus"`
			ScheduleType        string `json:"scheduleType"`
		} `json:"connections"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		// Try alternative structure
		var altResp struct {
			Data []struct {
				ConnectionID        string `json:"connectionId"`
				Name                string `json:"name"`
				Status              string `json:"status"`
				LatestSyncJobStatus string `json:"latestSyncJobStatus"`
			} `json:"data"`
		}
		if err2 := json.Unmarshal(body, &altResp); err2 != nil {
			return OpsResult{
				Status: "error",
				Signal: fmt.Sprintf("Airbyte: failed to parse response: %v", err),
				Cost:   "zero-llm",
			}
		}
		// Use altResp data
		failedCount := 0
		for _, conn := range altResp.Data {
			if conn.LatestSyncJobStatus == "failed" {
				failedCount++
			}
		}
		return buildAirbyteResult(len(altResp.Data), failedCount)
	}

	failedCount := 0
	for _, conn := range resp.Connections {
		if conn.LatestSyncJobStatus == "failed" {
			failedCount++
		}
	}

	return buildAirbyteResult(len(resp.Connections), failedCount)
}

// ScheduleEntry describes one Airbyte connection in the schedule map (Discovery 008-A).
type ScheduleEntry struct {
	ConnectionID  string  `json:"connection_id"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`        // active | inactive | deprecated
	ScheduleType  string  `json:"schedule_type"` // cron | basic | manual
	CronExpr      string  `json:"cron_expr,omitempty"`
	IntervalMin   float64 `json:"interval_min,omitempty"`
	DestinationID string  `json:"destination_id"`
	Destination   string  `json:"destination,omitempty"`
}

// CheckAirbyteScheduleMap fetches all connections in a workspace and returns a
// structured schedule map: type breakdown + per-connection cron/interval/destination.
// Heuristic: manual connections (no automation) and inactive connections → warn.
func CheckAirbyteScheduleMap(cfg AirbyteConfig) OpsResult {
	if cfg.URL == "" || cfg.WorkspaceID == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "Airbyte schedule-map: missing URL or workspace ID",
			Actions: []string{"set --input airbyte-url=..., airbyte-workspace-id=..."},
			Cost:    "zero-llm",
		}
	}

	headers := map[string]string{"Accept": "application/json"}
	if cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + cfg.APIKey
	}

	// 1. Fetch all connections
	connBody, status, err := httpGet(
		fmt.Sprintf("%s/api/v1/connections?workspaceId=%s", cfg.URL, cfg.WorkspaceID),
		headers,
	)
	if err != nil {
		return OpsResult{Status: "error", Signal: fmt.Sprintf("Airbyte schedule-map: API error: %v", err), Cost: "zero-llm"}
	}
	if status != 200 {
		return OpsResult{Status: "error", Signal: fmt.Sprintf("Airbyte schedule-map: HTTP %d", status), Cost: "zero-llm"}
	}

	// 2. Fetch destination names (best-effort — non-critical)
	destNames := map[string]string{}
	postHeaders := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	if cfg.APIKey != "" {
		postHeaders["Authorization"] = "Bearer " + cfg.APIKey
	}
	destReqBody, _ := json.Marshal(map[string]string{"workspaceId": cfg.WorkspaceID})
	if destBody, dStatus, dErr := httpPost(cfg.URL+"/api/v1/destinations/list", postHeaders, destReqBody); dErr == nil && dStatus == 200 {
		var destResp struct {
			Destinations []struct {
				DestinationID string `json:"destinationId"`
				Name          string `json:"name"`
			} `json:"destinations"`
		}
		if json.Unmarshal(destBody, &destResp) == nil {
			for _, d := range destResp.Destinations {
				destNames[d.DestinationID] = d.Name
			}
		}
	}

	return evaluateAirbyteScheduleMap(connBody, destNames)
}

func evaluateAirbyteScheduleMap(body []byte, destNames map[string]string) OpsResult {
	var resp struct {
		Connections []struct {
			ConnectionID  string `json:"connectionId"`
			Name          string `json:"name"`
			Status        string `json:"status"`
			ScheduleType  string `json:"scheduleType"`
			DestinationID string `json:"destinationId"`
			ScheduleData  struct {
				Cron *struct {
					CronExpression string `json:"cronExpression"`
					CronTimeZone   string `json:"cronTimeZone"`
				} `json:"cron"`
				BasicSchedule *struct {
					Units    float64 `json:"units"`
					TimeUnit string  `json:"timeUnit"` // minutes | hours | days | weeks | months
				} `json:"basicSchedule"`
			} `json:"scheduleData"`
		} `json:"connections"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return OpsResult{Status: "error", Signal: fmt.Sprintf("Airbyte schedule-map: parse error: %v", err), Cost: "zero-llm"}
	}
	if len(resp.Connections) == 0 {
		return OpsResult{
			Status:  "warn",
			Signal:  "Airbyte schedule-map: no connections found in workspace",
			Actions: []string{"verify workspace ID"},
			Cost:    "zero-llm",
		}
	}

	entries := make([]ScheduleEntry, 0, len(resp.Connections))
	counts := map[string]int{"cron": 0, "basic": 0, "manual": 0}
	inactiveCount := 0
	manualActive := 0

	for _, c := range resp.Connections {
		e := ScheduleEntry{
			ConnectionID:  c.ConnectionID,
			Name:          c.Name,
			Status:        c.Status,
			ScheduleType:  c.ScheduleType,
			DestinationID: c.DestinationID,
			Destination:   destNames[c.DestinationID],
		}
		switch c.ScheduleType {
		case "cron":
			counts["cron"]++
			if c.ScheduleData.Cron != nil {
				e.CronExpr = c.ScheduleData.Cron.CronExpression
			}
		case "basic":
			counts["basic"]++
			if c.ScheduleData.BasicSchedule != nil {
				e.IntervalMin = toMinutes(c.ScheduleData.BasicSchedule.Units, c.ScheduleData.BasicSchedule.TimeUnit)
			}
		default:
			counts["manual"]++
			if c.Status == "active" {
				manualActive++
			}
		}
		if c.Status != "active" {
			inactiveCount++
		}
		entries = append(entries, e)
	}

	total := len(entries)
	signal := fmt.Sprintf("Airbyte schedule-map: %d connections — %d cron, %d basic, %d manual",
		total, counts["cron"], counts["basic"], counts["manual"])
	if inactiveCount > 0 {
		signal += fmt.Sprintf(" | %d inactive", inactiveCount)
	}

	data := map[string]any{
		"total":           total,
		"cron_count":      counts["cron"],
		"basic_count":     counts["basic"],
		"manual_count":    counts["manual"],
		"inactive_count":  inactiveCount,
		"manual_active":   manualActive,
		"schedule_issues": manualActive + inactiveCount,
		"connections":     entries,
	}

	hStatus, _, _ := EvalHeuristics(data, loadHeuristics("airbyte-schedule-map"))
	var actions []string
	if manualActive > 0 {
		actions = append(actions, fmt.Sprintf("%d active connections with manual schedule — consider automating", manualActive))
	}
	if inactiveCount > 0 {
		actions = append(actions, fmt.Sprintf("%d inactive connections — review if deprecated or intentionally paused", inactiveCount))
	}
	return OpsResult{Status: hStatus, Signal: signal, Data: data, Actions: actions, Cost: "zero-llm"}
}

// toMinutes converts a basicSchedule unit+timeUnit to minutes.
func toMinutes(units float64, timeUnit string) float64 {
	switch timeUnit {
	case "hours":
		return units * 60
	case "days":
		return units * 1440
	case "weeks":
		return units * 10080
	case "months":
		return units * 43200
	default: // minutes
		return units
	}
}

// AirbyteJobProfileConfig holds settings for job-profile analysis (Discovery 008-B).
type AirbyteJobProfileConfig struct {
	AirbyteConfig
	ConnectionID        string
	ScheduleIntervalMin float64 // schedule cadence; used to compute gap_real_min
	AutoSuspendMin      float64 // optional: Snowflake warehouse AUTO_SUSPEND for gap heuristic
	PageSize            int     // jobs to fetch; default 50
}

// CheckAirbyteJobProfile queries job history for a connection and computes
// avg_duration_min, avg_records, success_rate, and gap_real_min = schedule - avg_duration.
// If AutoSuspendMin > 0, applies evalGapHeuristic for warehouse cost signal.
func CheckAirbyteJobProfile(cfg AirbyteJobProfileConfig) OpsResult {
	if cfg.URL == "" || cfg.ConnectionID == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "Airbyte job-profile: missing URL or connection ID",
			Actions: []string{"set --input airbyte-url=..., airbyte-connection-id=..."},
			Cost:    "zero-llm",
		}
	}

	pageSize := cfg.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	if cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + cfg.APIKey
	}

	reqBody, _ := json.Marshal(map[string]any{
		"configId":    cfg.ConnectionID,
		"configTypes": []string{"sync"},
		"pagination":  map[string]any{"pageSize": pageSize},
	})

	body, statusCode, err := httpPost(cfg.URL+"/api/v1/jobs/list", headers, reqBody)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Airbyte job-profile API error: %v", err),
			Cost:   "zero-llm",
		}
	}
	if statusCode != 200 {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Airbyte job-profile API returned HTTP %d", statusCode),
			Cost:   "zero-llm",
		}
	}

	return evaluateAirbyteJobProfile(body, cfg)
}

func evaluateAirbyteJobProfile(body []byte, cfg AirbyteJobProfileConfig) OpsResult {
	var resp struct {
		Jobs []struct {
			Job struct {
				ID        int64  `json:"id"`
				Status    string `json:"status"`
				CreatedAt int64  `json:"createdAt"`
				UpdatedAt int64  `json:"updatedAt"`
			} `json:"job"`
			Attempts []struct {
				Attempt struct {
					RecordsSynced int64 `json:"recordsSynced"`
				} `json:"attempt"`
				Failures []struct {
					FailureOrigin string  `json:"failureOrigin"`
					FailureType   *string `json:"failureType"` // nil when JSON null — pod kill indicator
				} `json:"failures"`
			} `json:"attempts"`
		} `json:"jobs"`
		TotalJobCount int `json:"totalJobCount"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Airbyte job-profile: failed to parse response: %v", err),
			Cost:   "zero-llm",
		}
	}
	if len(resp.Jobs) == 0 {
		return OpsResult{
			Status:  "warn",
			Signal:  fmt.Sprintf("Airbyte job-profile %s: no jobs found", cfg.ConnectionID),
			Actions: []string{"verify connection ID and that syncs have run"},
			Cost:    "zero-llm",
		}
	}

	var totalDurationSec float64
	var totalRecords int64
	succeededCount := 0
	nullFailureJobs := 0
	maxAttemptsPerJob := 0
	fastFailAttempts := 0
	for _, j := range resp.Jobs {
		dur := float64(j.Job.UpdatedAt - j.Job.CreatedAt)
		if dur > 0 {
			totalDurationSec += dur
		}

		attemptCount := len(j.Attempts)
		if attemptCount > maxAttemptsPerJob {
			maxAttemptsPerJob = attemptCount
		}

		jobHasNullFailure := false
		for _, a := range j.Attempts {
			totalRecords += a.Attempt.RecordsSynced
			attemptHasNullFailure := false
			for _, f := range a.Failures {
				if f.FailureType == nil && f.FailureOrigin == "source" {
					attemptHasNullFailure = true
					break
				}
			}
			if attemptHasNullFailure {
				fastFailAttempts++
				jobHasNullFailure = true
			}
		}
		if jobHasNullFailure {
			nullFailureJobs++
		}

		if j.Job.Status == "succeeded" {
			succeededCount++
		}
	}

	n := len(resp.Jobs)
	avgDurationMin := (totalDurationSec / float64(n)) / 60.0
	avgRecords := totalRecords / int64(n)
	successRate := float64(succeededCount) / float64(n)

	gapRealMin := 0.0
	if cfg.ScheduleIntervalMin > 0 {
		gapRealMin = math.Max(0, cfg.ScheduleIntervalMin-avgDurationMin)
	}

	data := map[string]any{
		"connection_id":      cfg.ConnectionID,
		"jobs_sampled":       n,
		"total_job_count":    resp.TotalJobCount,
		"avg_duration_min":   math.Round(avgDurationMin*10) / 10,
		"avg_records":        avgRecords,
		"success_rate":       math.Round(successRate*1000) / 1000,
		"succeeded_count":    succeededCount,
		"gap_real_min":       math.Round(gapRealMin*10) / 10,
		"null_failure_jobs":  nullFailureJobs,
		"max_attempts_per_job": maxAttemptsPerJob,
		"fast_fail_attempts": fastFailAttempts,
	}

	avgRecordsStr := formatCount(avgRecords)
	signal := fmt.Sprintf("Airbyte job-profile %s: avg %.1fmin, %s records/sync, %.1f%% success (%d/%d)",
		cfg.ConnectionID, avgDurationMin, avgRecordsStr, successRate*100, succeededCount, n)

	if cfg.AutoSuspendMin > 0 && cfg.ScheduleIntervalMin > 0 {
		autoSuspendSec := int(cfg.AutoSuspendMin * 60)
		status, heuristicSignal, actions := evalGapHeuristic(gapRealMin, cfg.AutoSuspendMin, autoSuspendSec, "warehouse")
		signal += " | " + heuristicSignal
		return OpsResult{
			Status:  status,
			Signal:  signal,
			Data:    data,
			Actions: actions,
			Cost:    "zero-llm",
		}
	}

	hStatus, hSignal, hActions := EvalHeuristics(data, loadHeuristics("airbyte-job-profile"))
	if hSignal != "" {
		signal += " | " + hSignal
	}
	if cfg.ScheduleIntervalMin > 0 {
		hActions = append(hActions, fmt.Sprintf("gap_real_min=%.1f — pass --auto-suspend-min for warehouse cost analysis", gapRealMin))
	}

	return OpsResult{
		Status:  hStatus,
		Signal:  signal,
		Data:    data,
		Actions: hActions,
		Cost:    "zero-llm",
	}
}

// formatCount formats large numbers with K/M suffixes for readability.
func formatCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func buildAirbyteResult(total, failedCount int) OpsResult {
	data := map[string]any{
		"total_connections":  total,
		"failed_connections": failedCount,
	}

	hStatus, hSignal, hActions := EvalHeuristics(data, loadHeuristics("airbyte-status"))
	signal := fmt.Sprintf("Airbyte: %d connections, all syncs healthy", total)
	if hSignal != "" {
		signal = "Airbyte: " + hSignal
	}
	return OpsResult{
		Status:  hStatus,
		Signal:  signal,
		Data:    data,
		Actions: hActions,
		Cost:    "zero-llm",
	}
}
