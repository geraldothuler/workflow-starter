package ops

import (
	"strings"
	"testing"
	"time"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// extractLogTimestamp
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestExtractLogTimestamp_ISO8601(t *testing.T) {
	line := "2026-02-22T09:15:03 ERROR InvalidProducerEpochException"
	got := extractLogTimestamp(line)
	if got != "2026-02-22T09:15:03" {
		t.Errorf("unexpected timestamp: %q", got)
	}
}

func TestExtractLogTimestamp_SpaceSeparator(t *testing.T) {
	line := "2026-02-22 09:15:03 WARN slow query detected"
	got := extractLogTimestamp(line)
	if got != "2026-02-22 09:15:03" {
		t.Errorf("unexpected timestamp: %q", got)
	}
}

func TestExtractLogTimestamp_NoTimestamp(t *testing.T) {
	got := extractLogTimestamp("ERROR no timestamp here")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// parseKafkaLogLines
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestParseKafkaLogLines_EpochErrors(t *testing.T) {
	lines := []string{
		"2026-02-22T09:10:00 ERROR InvalidProducerEpochException on topic orders",
		"2026-02-22T09:10:05 ERROR ProducerFencedException: producer was fenced",
		"2026-02-22T09:10:10 INFO normal log line",
	}
	m := parseKafkaLogLines(lines)
	if m.EpochErrors != 2 {
		t.Errorf("expected 2 epoch errors, got %d", m.EpochErrors)
	}
	if m.FirstErrorSeen != "2026-02-22T09:10:00" {
		t.Errorf("unexpected FirstErrorSeen: %q", m.FirstErrorSeen)
	}
	if m.LastErrorSeen != "2026-02-22T09:10:05" {
		t.Errorf("unexpected LastErrorSeen: %q", m.LastErrorSeen)
	}
}

func TestParseKafkaLogLines_CoordErrors(t *testing.T) {
	lines := []string{
		"2026-02-22T09:00:00 ERROR COORDINATOR_NOT_AVAILABLE for group my-group",
		"2026-02-22T09:00:01 ERROR NOT_COORDINATOR response received",
	}
	m := parseKafkaLogLines(lines)
	if m.CoordErrors != 2 {
		t.Errorf("expected 2 coord errors, got %d", m.CoordErrors)
	}
	if m.EpochErrors != 0 {
		t.Errorf("expected 0 epoch errors, got %d", m.EpochErrors)
	}
}

func TestParseKafkaLogLines_Rebalances(t *testing.T) {
	lines := []string{
		"INFO Rebalance in progress for group my-group",
		"INFO assigned partitions: [orders-0, orders-1]",
		"INFO Group is in the middle of rebalancing",
	}
	m := parseKafkaLogLines(lines)
	if m.Rebalances != 3 {
		t.Errorf("expected 3 rebalances, got %d", m.Rebalances)
	}
}

func TestParseKafkaLogLines_LagJSON(t *testing.T) {
	lines := []string{
		`{"records_lag": 1500, "topic": "orders"}`,
		`{"consumer_lag": 2000, "group": "my-group"}`,
	}
	m := parseKafkaLogLines(lines)
	if m.MaxLag != 2000 {
		t.Errorf("expected max lag 2000, got %d", m.MaxLag)
	}
}

func TestParseKafkaLogLines_LagText(t *testing.T) {
	lines := []string{
		"INFO consumer lag=500 for partition orders-0",
		"INFO records-lag-max=1200",
	}
	m := parseKafkaLogLines(lines)
	if m.MaxLag != 1200 {
		t.Errorf("expected max lag 1200, got %d", m.MaxLag)
	}
}

func TestParseKafkaLogLines_ProductionConsumingRates(t *testing.T) {
	lines := []string{
		"INFO producing=350/min",
		"INFO consuming=300/min",
	}
	m := parseKafkaLogLines(lines)
	if m.ProducingRate != 350 {
		t.Errorf("expected producing rate 350, got %d", m.ProducingRate)
	}
	if m.ConsumingRate != 300 {
		t.Errorf("expected consuming rate 300, got %d", m.ConsumingRate)
	}
}

func TestParseKafkaLogLines_EmptyInput(t *testing.T) {
	m := parseKafkaLogLines([]string{})
	if m.EpochErrors != 0 || m.CoordErrors != 0 || m.Rebalances != 0 {
		t.Errorf("expected all zeros for empty input, got %+v", m)
	}
	if m.MaxLag != -1 {
		t.Errorf("expected MaxLag=-1 for no lag data, got %d", m.MaxLag)
	}
}

func TestParseKafkaLogLines_TotalLinesCount(t *testing.T) {
	lines := []string{"line1", "line2", "line3"}
	m := parseKafkaLogLines(lines)
	if m.TotalLines != 3 {
		t.Errorf("expected 3 total lines, got %d", m.TotalLines)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// analyzeKafkaLogs
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAnalyzeKafkaLogs_CriticalOnEpochError(t *testing.T) {
	cfg := KafkaConfig{
		Namespace:  "fusca",
		Deployment: "fusca-cdc",
		Topic:      "orders",
		Window:     "10m",
	}
	logs := "2026-02-22T09:10:00 ERROR InvalidProducerEpochException on topic orders\n" +
		"2026-02-22T09:10:01 INFO normal log\n"

	r := analyzeKafkaLogs(cfg, logs, "app=fusca-cdc")

	if r.Status != "critical" {
		t.Errorf("expected critical for epoch error, got %q", r.Status)
	}
	if !strings.Contains(r.Signal, "CRÍTICO") {
		t.Errorf("expected CRÍTICO in signal: %q", r.Signal)
	}
	if r.Data["source"] != "logs" {
		t.Errorf("expected source=logs, got %v", r.Data["source"])
	}
}

func TestAnalyzeKafkaLogs_WarnRebalances(t *testing.T) {
	cfg := KafkaConfig{Namespace: "fusca", Deployment: "fusca-cdc", Window: "10m"}
	lines := strings.Repeat("INFO Rebalance in progress\n", warnRebalanceCount+2)

	r := analyzeKafkaLogs(cfg, lines, "app=fusca-cdc")

	if r.Status != "warn" {
		t.Errorf("expected warn for excessive rebalances, got %q", r.Status)
	}
}

func TestAnalyzeKafkaLogs_Healthy(t *testing.T) {
	cfg := KafkaConfig{Namespace: "fusca", Deployment: "fusca-cdc", Topic: "orders", Window: "10m"}
	logs := "2026-02-22T09:00:00 INFO consuming=500/min\n" +
		"2026-02-22T09:00:01 INFO producing=550/min\n"

	r := analyzeKafkaLogs(cfg, logs, "app=fusca-cdc")

	if r.Status != "ok" {
		t.Errorf("expected ok for healthy logs, got %q (signal: %s)", r.Status, r.Signal)
	}
	if !strings.Contains(r.Signal, "saudável") {
		t.Errorf("expected 'saudável' in signal: %q", r.Signal)
	}
}

func TestAnalyzeKafkaLogs_DataFields(t *testing.T) {
	cfg := KafkaConfig{
		Namespace:     "fusca",
		Deployment:    "fusca-cdc",
		Topic:         "orders",
		ConsumerGroup: "cdc-group",
		Window:        "30m",
	}
	logs := "2026-02-22T09:00:00 INFO nothing interesting\n"

	r := analyzeKafkaLogs(cfg, logs, "app=fusca-cdc")

	if r.Data["topic"] != "orders" {
		t.Errorf("expected topic in data, got %v", r.Data["topic"])
	}
	if r.Data["consumer_group"] != "cdc-group" {
		t.Errorf("expected consumer_group in data, got %v", r.Data["consumer_group"])
	}
	if r.Data["window"] != "30m" {
		t.Errorf("expected window=30m in data, got %v", r.Data["window"])
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CheckKafkaStatus — no-selector error path
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckKafkaStatus_NoSelectorError(t *testing.T) {
	cfg := KafkaConfig{KubectlContext: "cobli-prod", Namespace: "fusca"}
	r := CheckKafkaStatus(cfg)
	if r.Status != "error" {
		t.Errorf("expected error when no selector given, got %q", r.Status)
	}
	if r.Cost != "zero-llm" {
		t.Errorf("expected zero-llm cost, got %q", r.Cost)
	}
}

func TestCheckKafkaStatus_DatadogMissingCreds(t *testing.T) {
	cfg := KafkaConfig{
		Source:    "datadog",
		Namespace: "fusca",
		// No API/App keys
	}
	r := CheckKafkaStatus(cfg)
	if r.Status != "error" {
		t.Errorf("expected error for missing Datadog creds, got %q", r.Status)
	}
	if !strings.Contains(r.Signal, "Datadog") {
		t.Errorf("expected Datadog mention in signal: %q", r.Signal)
	}
}

func TestCheckKafkaStatus_DatadogInvalidWindow(t *testing.T) {
	cfg := KafkaConfig{
		Source:        "datadog",
		Window:        "not-a-duration",
		DatadogAPIKey: "key",
		DatadogAppKey: "appkey",
	}
	r := CheckKafkaStatus(cfg)
	if r.Status != "error" {
		t.Errorf("expected error for invalid window, got %q", r.Status)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// computeMetricTrend
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestComputeMetricTrend_Stable(t *testing.T) {
	points := makePoints([]float64{100, 100, 100, 100, 100, 100, 100, 100, 100})
	got := computeMetricTrend(points)
	if got != "stable" {
		t.Errorf("expected stable, got %q", got)
	}
}

func TestComputeMetricTrend_Growing(t *testing.T) {
	// Grows from 100 to 200 — 100% growth
	points := makePoints([]float64{100, 100, 100, 150, 150, 150, 200, 200, 200})
	got := computeMetricTrend(points)
	if got != "growing" {
		t.Errorf("expected growing, got %q", got)
	}
}

func TestComputeMetricTrend_Decreasing(t *testing.T) {
	// Drops from 200 to 100 — 50% decrease
	points := makePoints([]float64{200, 200, 200, 150, 150, 150, 100, 100, 100})
	got := computeMetricTrend(points)
	if got != "decreasing" {
		t.Errorf("expected decreasing, got %q", got)
	}
}

func TestComputeMetricTrend_TooFewPoints(t *testing.T) {
	got := computeMetricTrend(makePoints([]float64{100, 200}))
	if got != "stable" {
		t.Errorf("expected stable for <3 points, got %q", got)
	}
}

func TestComputeMetricTrend_AllZero(t *testing.T) {
	points := makePoints([]float64{0, 0, 0, 0, 0, 0})
	got := computeMetricTrend(points)
	if got != "stable" {
		t.Errorf("expected stable for all-zero, got %q", got)
	}
}

func TestComputeMetricTrend_FromZeroToNonZero(t *testing.T) {
	// Was zero, now non-zero → growing
	points := makePoints([]float64{0, 0, 0, 50, 100, 150, 200, 200, 200})
	got := computeMetricTrend(points)
	if got != "growing" {
		t.Errorf("expected growing when starting from zero, got %q", got)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// avgSlice
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAvgSlice_Basic(t *testing.T) {
	points := makePoints([]float64{10, 20, 30})
	got := avgSlice(points)
	if got != 20 {
		t.Errorf("expected avg=20, got %f", got)
	}
}

func TestAvgSlice_Empty(t *testing.T) {
	got := avgSlice(nil)
	if got != 0 {
		t.Errorf("expected 0 for empty slice, got %f", got)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// buildDatadogQuery
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBuildDatadogQuery_WithTopicAndGroup(t *testing.T) {
	cfg := KafkaConfig{Topic: "orders", ConsumerGroup: "cdc-group"}
	q := buildDatadogQuery(cfg)
	if !strings.Contains(q, "topic:orders") {
		t.Errorf("expected topic filter: %q", q)
	}
	if !strings.Contains(q, "consumer_group:cdc-group") {
		t.Errorf("expected consumer_group filter: %q", q)
	}
	if !strings.Contains(q, "kafka.consumer_lag") {
		t.Errorf("expected lag metric: %q", q)
	}
}

func TestBuildDatadogQuery_NoFilters(t *testing.T) {
	cfg := KafkaConfig{}
	q := buildDatadogQuery(cfg)
	if !strings.Contains(q, "avg:kafka.consumer_lag{*}") {
		t.Errorf("expected wildcard query: %q", q)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// evaluateKafkaDatadog
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestEvaluateKafkaDatadog_NoData(t *testing.T) {
	cfg := KafkaConfig{Topic: "orders", Window: "10m"}
	r := evaluateKafkaDatadog(cfg, []ddSeries{}, "avg:kafka.consumer_lag{*}", testTime(), testTime())
	if r.Status != "warn" {
		t.Errorf("expected warn for empty series, got %q", r.Status)
	}
}

func TestEvaluateKafkaDatadog_StableLag(t *testing.T) {
	cfg := KafkaConfig{Topic: "orders", Window: "10m"}
	// 9 equal points → stable
	points := makePoints([]float64{500, 500, 500, 500, 500, 500, 500, 500, 500})
	series := []ddSeries{{Metric: "kafka.consumer_lag", Pointlist: points, Scope: "topic:orders"}}
	r := evaluateKafkaDatadog(cfg, series, "avg:kafka.consumer_lag{*}", testTime(), testTime())
	if r.Status != "ok" {
		t.Errorf("expected ok for stable lag, got %q (signal: %s)", r.Status, r.Signal)
	}
	if r.Data["lag_current"] != int64(500) {
		t.Errorf("expected lag_current=500, got %v", r.Data["lag_current"])
	}
	if r.Data["trend"] != "stable" {
		t.Errorf("expected trend=stable, got %v", r.Data["trend"])
	}
}

func TestEvaluateKafkaDatadog_CriticalGrowingLag(t *testing.T) {
	cfg := KafkaConfig{Topic: "orders", Window: "10m"}
	// Grows from 100 to 500 — 400% → critical
	points := makePoints([]float64{100, 100, 100, 200, 300, 400, 500, 500, 500})
	series := []ddSeries{{Metric: "kafka.consumer_lag", Pointlist: points, Scope: "topic:orders"}}
	r := evaluateKafkaDatadog(cfg, series, "avg:kafka.consumer_lag{*}", testTime(), testTime())
	if r.Status != "critical" {
		t.Errorf("expected critical for >%d%% growth, got %q", criticalLagGrowthPct, r.Status)
	}
}

func TestEvaluateKafkaDatadog_WarnGrowingLag(t *testing.T) {
	cfg := KafkaConfig{Topic: "orders", Window: "10m"}
	// ~20% growth — warn
	points := makePoints([]float64{100, 100, 100, 110, 115, 115, 120, 120, 120})
	series := []ddSeries{{Metric: "kafka.consumer_lag", Pointlist: points, Scope: "topic:orders"}}
	r := evaluateKafkaDatadog(cfg, series, "avg:kafka.consumer_lag{*}", testTime(), testTime())
	if r.Status != "warn" {
		t.Errorf("expected warn for moderate growth, got %q (signal: %s)", r.Status, r.Signal)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// buildKafkaLogsSignal / buildKafkaDatadogSignal
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBuildKafkaLogsSignal_ContainsTopic(t *testing.T) {
	cfg := KafkaConfig{Topic: "orders", Window: "10m"}
	m := kafkaLogMetrics{MaxLag: -1}
	sig := buildKafkaLogsSignal(cfg, m, "ok", nil, nil, "app=fusca-cdc")
	if !strings.Contains(sig, "orders") {
		t.Errorf("signal missing topic: %q", sig)
	}
	if !strings.Contains(sig, "saudável") {
		t.Errorf("expected 'saudável': %q", sig)
	}
}

func TestBuildKafkaDatadogSignal_ContainsLag(t *testing.T) {
	cfg := KafkaConfig{Topic: "orders", Window: "10m"}
	sig := buildKafkaDatadogSignal(cfg, 1500, "stable", "ok", nil, nil)
	if !strings.Contains(sig, "1500") {
		t.Errorf("signal missing lag value: %q", sig)
	}
	if !strings.Contains(sig, "estável") {
		t.Errorf("expected 'estável': %q", sig)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// helpers
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// makePoints converts a []float64 of values into [][]float64 {timestamp, value}.
func makePoints(values []float64) [][]float64 {
	pts := make([][]float64, len(values))
	for i, v := range values {
		pts[i] = []float64{float64(1000 + i), v}
	}
	return pts
}

func testTime() time.Time {
	return time.Date(2026, 2, 22, 9, 0, 0, 0, time.UTC)
}
