package ops

import (
	"net/http"
	"testing"
)

func TestCheckAirbyte_MissingCredentials(t *testing.T) {
	r := CheckAirbyte(AirbyteConfig{})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckAirbyte_CriticalFailures(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{
		"connections": [
			{"connectionId": "1", "name": "conn1", "latestSyncJobStatus": "failed"},
			{"connectionId": "2", "name": "conn2", "latestSyncJobStatus": "failed"},
			{"connectionId": "3", "name": "conn3", "latestSyncJobStatus": "failed"},
			{"connectionId": "4", "name": "conn4", "latestSyncJobStatus": "failed"},
			{"connectionId": "5", "name": "conn5", "latestSyncJobStatus": "succeeded"}
		]
	}`)

	r := CheckAirbyte(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "critical" {
		t.Errorf("expected critical (>3 failures), got %q", r.Status)
	}
}

func TestCheckAirbyte_AllHealthy(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{
		"connections": [
			{"connectionId": "1", "name": "conn1", "latestSyncJobStatus": "succeeded"},
			{"connectionId": "2", "name": "conn2", "latestSyncJobStatus": "succeeded"}
		]
	}`)

	r := CheckAirbyte(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

func TestCheckAirbyte_API500(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(500, `error`)
	r := CheckAirbyte(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

// --- CheckAirbyteScheduleMap tests (008-A) ---

func TestCheckAirbyteScheduleMap_MissingConfig(t *testing.T) {
	r := CheckAirbyteScheduleMap(AirbyteConfig{})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckAirbyteScheduleMap_AllCron_OK(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	// First call: connections; second call: destinations list (returns 200 with names)
	calls := 0
	httpDo = func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return mockHTTPResponse(200, `{
				"connections": [
					{"connectionId":"c1","name":"fusca-cdc","status":"active","scheduleType":"cron",
					 "destinationId":"d1","scheduleData":{"cron":{"cronExpression":"0 */15 * * * ?","cronTimeZone":"UTC"}}},
					{"connectionId":"c2","name":"sherlock-sync","status":"active","scheduleType":"cron",
					 "destinationId":"d1","scheduleData":{"cron":{"cronExpression":"0 0 * * * ?","cronTimeZone":"UTC"}}}
				]
			}`)(req)
		}
		return mockHTTPResponse(200, `{"destinations":[{"destinationId":"d1","name":"Snowflake DWH"}]}`)(req)
	}

	r := CheckAirbyteScheduleMap(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Signal)
	}
	if r.Data["cron_count"] != 2 {
		t.Errorf("expected cron_count=2, got %v", r.Data["cron_count"])
	}
	if r.Data["manual_count"] != 0 {
		t.Errorf("expected manual_count=0, got %v", r.Data["manual_count"])
	}
}

func TestCheckAirbyteScheduleMap_ManualActive_Warn(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	calls := 0
	httpDo = func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return mockHTTPResponse(200, `{
				"connections": [
					{"connectionId":"c1","name":"manual-conn","status":"active","scheduleType":"manual","destinationId":"d1"},
					{"connectionId":"c2","name":"cron-conn","status":"active","scheduleType":"cron","destinationId":"d1",
					 "scheduleData":{"cron":{"cronExpression":"0 */30 * * * ?","cronTimeZone":"UTC"}}}
				]
			}`)(req)
		}
		return mockHTTPResponse(200, `{"destinations":[]}`)(req)
	}

	r := CheckAirbyteScheduleMap(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "warn" {
		t.Errorf("expected warn (active manual), got %q", r.Status)
	}
	if r.Data["manual_count"] != 1 {
		t.Errorf("expected manual_count=1, got %v", r.Data["manual_count"])
	}
}

func TestCheckAirbyteScheduleMap_Inactive_Warn(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	calls := 0
	httpDo = func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return mockHTTPResponse(200, `{
				"connections": [
					{"connectionId":"c1","name":"active-conn","status":"active","scheduleType":"cron","destinationId":"d1",
					 "scheduleData":{"cron":{"cronExpression":"0 */15 * * * ?","cronTimeZone":"UTC"}}},
					{"connectionId":"c2","name":"paused-conn","status":"inactive","scheduleType":"cron","destinationId":"d1",
					 "scheduleData":{"cron":{"cronExpression":"0 0 * * * ?","cronTimeZone":"UTC"}}}
				]
			}`)(req)
		}
		return mockHTTPResponse(200, `{"destinations":[]}`)(req)
	}

	r := CheckAirbyteScheduleMap(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "warn" {
		t.Errorf("expected warn (inactive), got %q", r.Status)
	}
	if r.Data["inactive_count"] != 1 {
		t.Errorf("expected inactive_count=1, got %v", r.Data["inactive_count"])
	}
}

func TestCheckAirbyteScheduleMap_BasicSchedule(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	calls := 0
	httpDo = func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return mockHTTPResponse(200, `{
				"connections": [
					{"connectionId":"c1","name":"basic-conn","status":"active","scheduleType":"basic","destinationId":"d1",
					 "scheduleData":{"basicSchedule":{"units":2,"timeUnit":"hours"}}}
				]
			}`)(req)
		}
		return mockHTTPResponse(200, `{"destinations":[]}`)(req)
	}

	r := CheckAirbyteScheduleMap(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
	if r.Data["basic_count"] != 1 {
		t.Errorf("expected basic_count=1, got %v", r.Data["basic_count"])
	}
	entries := r.Data["connections"].([]ScheduleEntry)
	if entries[0].IntervalMin != 120 {
		t.Errorf("expected interval_min=120 (2h), got %v", entries[0].IntervalMin)
	}
}

func TestCheckAirbyteScheduleMap_NoConnections(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	calls := 0
	httpDo = func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return mockHTTPResponse(200, `{"connections":[]}`)(req)
		}
		return mockHTTPResponse(200, `{"destinations":[]}`)(req)
	}

	r := CheckAirbyteScheduleMap(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "warn" {
		t.Errorf("expected warn (no connections), got %q", r.Status)
	}
}

func TestCheckAirbyteScheduleMap_API500(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(500, `error`)

	r := CheckAirbyteScheduleMap(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckAirbyteScheduleMap_DestinationResolved(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	calls := 0
	httpDo = func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return mockHTTPResponse(200, `{
				"connections": [
					{"connectionId":"c1","name":"fusca-cdc","status":"active","scheduleType":"cron",
					 "destinationId":"dest-snowflake","scheduleData":{"cron":{"cronExpression":"0 */15 * * * ?"}}}
				]
			}`)(req)
		}
		return mockHTTPResponse(200, `{"destinations":[{"destinationId":"dest-snowflake","name":"Snowflake DWH"}]}`)(req)
	}

	r := CheckAirbyteScheduleMap(AirbyteConfig{URL: "https://airbyte.test", WorkspaceID: "ws1"})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
	entries := r.Data["connections"].([]ScheduleEntry)
	if entries[0].Destination != "Snowflake DWH" {
		t.Errorf("expected destination=Snowflake DWH, got %q", entries[0].Destination)
	}
}

// --- CheckAirbyteJobProfile tests ---

func TestCheckAirbyteJobProfile_MissingConfig(t *testing.T) {
	r := CheckAirbyteJobProfile(AirbyteJobProfileConfig{})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckAirbyteJobProfile_MissingConnectionID(t *testing.T) {
	r := CheckAirbyteJobProfile(AirbyteJobProfileConfig{
		AirbyteConfig: AirbyteConfig{URL: "https://airbyte.test"},
	})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

func TestCheckAirbyteJobProfile_HappyPath(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	// 3 jobs: 2 succeeded, 1 failed; durations 480s, 510s, 495s (~8min avg)
	httpDo = mockHTTPResponse(200, `{
		"jobs": [
			{"job": {"id": 1, "status": "succeeded", "createdAt": 1708000000, "updatedAt": 1708000480},
			 "attempts": [{"attempt": {"recordsSynced": 473000}}]},
			{"job": {"id": 2, "status": "succeeded", "createdAt": 1707913600, "updatedAt": 1707914110},
			 "attempts": [{"attempt": {"recordsSynced": 465000}}]},
			{"job": {"id": 3, "status": "failed",    "createdAt": 1707827200, "updatedAt": 1707827695},
			 "attempts": [{"attempt": {"recordsSynced": 0}}]}
		],
		"totalJobCount": 158
	}`)

	r := CheckAirbyteJobProfile(AirbyteJobProfileConfig{
		AirbyteConfig: AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:  "conn-abc",
		PageSize:      50,
	})
	if r.Status != "warn" {
		t.Errorf("expected warn (success_rate <0.9), got %q", r.Status)
	}
	if r.Data["connection_id"] != "conn-abc" {
		t.Errorf("expected connection_id conn-abc, got %v", r.Data["connection_id"])
	}
	if r.Data["jobs_sampled"] != 3 {
		t.Errorf("expected jobs_sampled=3, got %v", r.Data["jobs_sampled"])
	}
}

func TestCheckAirbyteJobProfile_AllSucceeded(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{
		"jobs": [
			{"job": {"id": 1, "status": "succeeded", "createdAt": 1708000000, "updatedAt": 1708000504},
			 "attempts": [{"attempt": {"recordsSynced": 500000}}]},
			{"job": {"id": 2, "status": "succeeded", "createdAt": 1707913600, "updatedAt": 1707914104},
			 "attempts": [{"attempt": {"recordsSynced": 490000}}]}
		],
		"totalJobCount": 50
	}`)

	r := CheckAirbyteJobProfile(AirbyteJobProfileConfig{
		AirbyteConfig: AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:  "conn-xyz",
	})
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q", r.Status)
	}
}

func TestCheckAirbyteJobProfile_WithGapHeuristic_AlwaysOn(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	// avg duration ~8.4min, schedule=15min → gap_real=6.6min ≤ auto_suspend=10min → warn
	httpDo = mockHTTPResponse(200, `{
		"jobs": [
			{"job": {"id": 1, "status": "succeeded", "createdAt": 1708000000, "updatedAt": 1708000504},
			 "attempts": [{"attempt": {"recordsSynced": 473000}}]}
		],
		"totalJobCount": 100
	}`)

	r := CheckAirbyteJobProfile(AirbyteJobProfileConfig{
		AirbyteConfig:       AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:        "conn-fusca-cdc",
		ScheduleIntervalMin: 15,
		AutoSuspendMin:      10,
	})
	if r.Status != "warn" {
		t.Errorf("expected warn (always-on), got %q", r.Status)
	}
}

func TestCheckAirbyteJobProfile_WithGapHeuristic_Suspends(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	// avg duration ~8.4min, schedule=30min → gap_real=21.6min > auto_suspend=10min → ok
	httpDo = mockHTTPResponse(200, `{
		"jobs": [
			{"job": {"id": 1, "status": "succeeded", "createdAt": 1708000000, "updatedAt": 1708000504},
			 "attempts": [{"attempt": {"recordsSynced": 473000}}]}
		],
		"totalJobCount": 100
	}`)

	r := CheckAirbyteJobProfile(AirbyteJobProfileConfig{
		AirbyteConfig:       AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:        "conn-fusca-cdc",
		ScheduleIntervalMin: 30,
		AutoSuspendMin:      10,
	})
	if r.Status != "ok" {
		t.Errorf("expected ok (gap > auto_suspend), got %q", r.Status)
	}
}

func TestCheckAirbyteJobProfile_NoJobs(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(200, `{"jobs": [], "totalJobCount": 0}`)

	r := CheckAirbyteJobProfile(AirbyteJobProfileConfig{
		AirbyteConfig: AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:  "conn-empty",
	})
	if r.Status != "warn" {
		t.Errorf("expected warn (no jobs), got %q", r.Status)
	}
}

func TestCheckAirbyteJobProfile_API500(t *testing.T) {
	origHTTP := httpDo
	defer func() { httpDo = origHTTP }()

	httpDo = mockHTTPResponse(500, `server error`)

	r := CheckAirbyteJobProfile(AirbyteJobProfileConfig{
		AirbyteConfig: AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:  "conn-abc",
	})
	if r.Status != "error" {
		t.Errorf("expected error, got %q", r.Status)
	}
}

// --- Infra instability heuristic tests ---

func TestEvaluateAirbyteJobProfile_InfraInstabilityWarn(t *testing.T) {
	// 1 job, 3 attempts, todos com failureType=null + failureOrigin=source → infra instability
	body := []byte(`{
		"jobs": [
			{"job": {"id": 1, "status": "failed", "createdAt": 1708000000, "updatedAt": 1708000060},
			 "attempts": [
				{"attempt": {"recordsSynced": 0}, "failures": [{"failureOrigin": "source", "failureType": null}]},
				{"attempt": {"recordsSynced": 0}, "failures": [{"failureOrigin": "source", "failureType": null}]},
				{"attempt": {"recordsSynced": 0}, "failures": [{"failureOrigin": "source", "failureType": null}]}
			 ]}
		],
		"totalJobCount": 1
	}`)
	cfg := AirbyteJobProfileConfig{
		AirbyteConfig: AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:  "conn-fusca-cdc",
	}
	r := evaluateAirbyteJobProfile(body, cfg)
	if r.Status != "warn" {
		t.Errorf("expected warn (infra instability), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["null_failure_jobs"] != 1 {
		t.Errorf("expected null_failure_jobs=1, got %v", r.Data["null_failure_jobs"])
	}
	if r.Data["max_attempts_per_job"] != 3 {
		t.Errorf("expected max_attempts_per_job=3, got %v", r.Data["max_attempts_per_job"])
	}
}

func TestEvaluateAirbyteJobProfile_ConnectorFailure_NoInfraHint(t *testing.T) {
	// 1 job, 1 attempt com failureType=system_error → low_success_rate (não infra)
	body := []byte(`{
		"jobs": [
			{"job": {"id": 1, "status": "failed", "createdAt": 1708000000, "updatedAt": 1708000300},
			 "attempts": [
				{"attempt": {"recordsSynced": 0}, "failures": [{"failureOrigin": "source", "failureType": "system_error"}]}
			 ]}
		],
		"totalJobCount": 1
	}`)
	cfg := AirbyteJobProfileConfig{
		AirbyteConfig: AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:  "conn-sherlock",
	}
	r := evaluateAirbyteJobProfile(body, cfg)
	if r.Status != "warn" {
		t.Errorf("expected warn (low success_rate), got %q", r.Status)
	}
	if r.Data["null_failure_jobs"] != 0 {
		t.Errorf("expected null_failure_jobs=0 (system_error != null), got %v", r.Data["null_failure_jobs"])
	}
}

func TestEvaluateAirbyteJobProfile_NullFailureDataFields(t *testing.T) {
	// 2 jobs: job1 com 3 attempts (2 null+source, 1 system_error), job2 com 1 attempt normal
	body := []byte(`{
		"jobs": [
			{"job": {"id": 1, "status": "failed", "createdAt": 1708000000, "updatedAt": 1708000060},
			 "attempts": [
				{"attempt": {"recordsSynced": 0}, "failures": [{"failureOrigin": "source", "failureType": null}]},
				{"attempt": {"recordsSynced": 0}, "failures": [{"failureOrigin": "source", "failureType": null}]},
				{"attempt": {"recordsSynced": 0}, "failures": [{"failureOrigin": "source", "failureType": "system_error"}]}
			 ]},
			{"job": {"id": 2, "status": "succeeded", "createdAt": 1707913600, "updatedAt": 1707914000},
			 "attempts": [
				{"attempt": {"recordsSynced": 50000}}
			 ]}
		],
		"totalJobCount": 2
	}`)
	cfg := AirbyteJobProfileConfig{
		AirbyteConfig: AirbyteConfig{URL: "https://airbyte.test"},
		ConnectionID:  "conn-test",
	}
	r := evaluateAirbyteJobProfile(body, cfg)
	if r.Data["null_failure_jobs"] != 1 {
		t.Errorf("expected null_failure_jobs=1, got %v", r.Data["null_failure_jobs"])
	}
	if r.Data["max_attempts_per_job"] != 3 {
		t.Errorf("expected max_attempts_per_job=3, got %v", r.Data["max_attempts_per_job"])
	}
	if r.Data["fast_fail_attempts"] != 2 {
		t.Errorf("expected fast_fail_attempts=2, got %v", r.Data["fast_fail_attempts"])
	}
}
