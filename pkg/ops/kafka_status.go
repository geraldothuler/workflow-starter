package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Error thresholds for Kafka heuristics.
const (
	criticalEpochErrors  = 1  // any epoch/fencing error is critical
	warnRebalanceCount   = 5  // rebalances in the window before warn
	warnLagGrowthPct     = 10 // % growth in lag (first→last third) to trigger warn
	criticalLagGrowthPct = 30 // % growth to trigger critical
)

// Compiled patterns searched in kubectl logs.
var (
	reEpochError     = regexp.MustCompile(`InvalidProducerEpochException|ProducerFencedException`)
	reCoordError     = regexp.MustCompile(`COORDINATOR_NOT_AVAILABLE|NOT_COORDINATOR|OFFSET_METADATA_TOO_LARGE|GroupLoadInProgressException`)
	reRebalance      = regexp.MustCompile(`Rebalance in progress|Group is in the middle of rebalancing|assigned partitions|Revoke partitions`)
	reLagJSON        = regexp.MustCompile(`"(?:records[_-]lag|consumer[_-]lag|lag)":\s*(\d+)`)
	reLagText        = regexp.MustCompile(`(?i)(?:consumer\s+lag|records-lag-max|lag)[=:\s]+(\d+)`)
	reProducingRate  = regexp.MustCompile(`(?i)(?:producing|sent)[=:\s]+(\d+)(?:\s*/\s*(?:min|sec|s))?`)
	reConsumingRate  = regexp.MustCompile(`(?i)(?:consuming|received|processed)[=:\s]+(\d+)(?:\s*/\s*(?:min|sec|s))?`)
	reLogTimestamp   = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)
)

// KafkaConfig holds parameters for the Kafka status check.
type KafkaConfig struct {
	// kubectl parameters — required for logs source
	KubectlContext string
	Namespace      string
	Deployment     string // derives selector "app=<name>"
	LabelSelector  string // explicit selector (alternative to Deployment)

	// Kafka context — optional, used in signal and Datadog query
	Topic         string
	ConsumerGroup string

	// Time window for logs/metrics (e.g. "10m", "2h"). Default: "10m"
	Window string

	// Source: logs | datadog. Default: logs
	Source string

	// Datadog parameters — required for datadog source
	DatadogAPIKey string
	DatadogAppKey string
	DatadogSite   string // default: datadoghq.com
	DatadogQuery  string // optional custom metric query
}

// CheckKafkaStatus checks Kafka consumer health from kubectl logs or Datadog.
func CheckKafkaStatus(cfg KafkaConfig) OpsResult {
	if cfg.Window == "" {
		cfg.Window = "10m"
	}
	if cfg.Source == "" {
		cfg.Source = "logs"
	}
	if cfg.DatadogSite == "" {
		cfg.DatadogSite = "datadoghq.com"
	}

	switch cfg.Source {
	case "datadog":
		return checkKafkaFromDatadog(cfg)
	default:
		return checkKafkaFromLogs(cfg)
	}
}

// ── logs source ───────────────────────────────────────────────────────────────

func checkKafkaFromLogs(cfg KafkaConfig) OpsResult {
	selector := cfg.LabelSelector
	if selector == "" && cfg.Deployment != "" {
		selector = "app=" + cfg.Deployment
	}
	if selector == "" {
		return OpsResult{
			Status: "error",
			Signal: "informe --deployment ou --label para identificar os pods do consumidor Kafka",
			Data:   map[string]any{},
			Actions: []string{
				"wtb ops kafka status --context <ctx> --namespace <ns> --deployment <nome> --topic <topic>",
			},
			Cost: "zero-llm",
		}
	}

	raw, err := fetchKafkaLogs(cfg, selector)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("falha ao obter logs Kafka ('%s'): %v", selector, err),
			Data:   map[string]any{"selector": selector, "namespace": cfg.Namespace, "context": cfg.KubectlContext},
			Actions: []string{
				"verificar conectividade kubectl e contexto: " + cfg.KubectlContext,
			},
			Cost: "zero-llm",
		}
	}

	return analyzeKafkaLogs(cfg, raw, selector)
}

func fetchKafkaLogs(cfg KafkaConfig, selector string) (string, error) {
	args := []string{
		"logs",
		"-n", cfg.Namespace,
		"--context", cfg.KubectlContext,
		"-l", selector,
		"--since=" + cfg.Window,
		"--tail=10000",
		"--prefix",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := shellExec(ctx, "kubectl", args...)
	if err != nil {
		return "", fmt.Errorf("%v — %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

type kafkaLogMetrics struct {
	EpochErrors    int64
	CoordErrors    int64
	Rebalances     int64
	MaxLag         int64
	ProducingRate  int64
	ConsumingRate  int64
	FirstErrorSeen string
	LastErrorSeen  string
	TotalLines     int
}

func analyzeKafkaLogs(cfg KafkaConfig, logs, selector string) OpsResult {
	lines := strings.Split(logs, "\n")
	m := parseKafkaLogLines(lines)

	var criticals, warnings, actions []string

	// Heuristic: epoch/fencing errors (root cause of Fusca CDC incident)
	if m.EpochErrors >= criticalEpochErrors {
		detail := fmt.Sprintf("%d InvalidProducerEpochException/ProducerFencedException", m.EpochErrors)
		if m.FirstErrorSeen != "" && m.LastErrorSeen != "" {
			detail += fmt.Sprintf(" (primeiro: %s, último: %s)", m.FirstErrorSeen, m.LastErrorSeen)
		}
		criticals = append(criticals, detail)
		actions = append(actions,
			"verificar se múltiplos produtores compartilham o mesmo transactional.id",
			"investigar deploys/restarts recentes do produtor Kafka",
		)
	}

	// Heuristic: coordinator errors
	if m.CoordErrors > 0 {
		warnings = append(warnings, fmt.Sprintf("%d erros de coordenador Kafka", m.CoordErrors))
		actions = append(actions, "verificar conectividade com o broker Kafka e zookeeper")
	}

	// Heuristic: excessive rebalances
	if m.Rebalances > warnRebalanceCount {
		warnings = append(warnings,
			fmt.Sprintf("%d rebalances detectados — possível instabilidade do consumer group", m.Rebalances))
		actions = append(actions,
			"investigar max.poll.interval.ms e lentidão no processamento",
		)
	}

	status := "ok"
	if len(criticals) > 0 {
		status = "critical"
	} else if len(warnings) > 0 {
		status = "warn"
	}

	signal := buildKafkaLogsSignal(cfg, m, status, criticals, warnings, selector)

	data := map[string]any{
		"source":      "logs",
		"selector":    selector,
		"namespace":   cfg.Namespace,
		"window":      cfg.Window,
		"total_lines": m.TotalLines,
		"errors": map[string]any{
			"epoch_errors":  m.EpochErrors,
			"coord_errors":  m.CoordErrors,
			"rebalances":    m.Rebalances,
		},
	}
	if cfg.Topic != "" {
		data["topic"] = cfg.Topic
	}
	if cfg.ConsumerGroup != "" {
		data["consumer_group"] = cfg.ConsumerGroup
	}
	if m.MaxLag >= 0 {
		data["lag_max"] = m.MaxLag
	}
	if m.ProducingRate > 0 {
		data["producing_rate"] = m.ProducingRate
	}
	if m.ConsumingRate > 0 {
		data["consuming_rate"] = m.ConsumingRate
	}
	if m.FirstErrorSeen != "" {
		data["error_first_seen"] = m.FirstErrorSeen
	}
	if m.LastErrorSeen != "" {
		data["error_last_seen"] = m.LastErrorSeen
	}
	if len(criticals) > 0 {
		data["criticals"] = criticals
	}
	if len(warnings) > 0 {
		data["warnings"] = warnings
	}

	return OpsResult{
		Status:  status,
		Signal:  signal,
		Data:    data,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

func parseKafkaLogLines(lines []string) kafkaLogMetrics {
	m := kafkaLogMetrics{MaxLag: -1}
	m.TotalLines = len(lines)

	for _, line := range lines {
		if reEpochError.MatchString(line) {
			m.EpochErrors++
			ts := extractLogTimestamp(line)
			if m.FirstErrorSeen == "" && ts != "" {
				m.FirstErrorSeen = ts
			}
			if ts != "" {
				m.LastErrorSeen = ts
			}
		}
		if reCoordError.MatchString(line) {
			m.CoordErrors++
		}
		if reRebalance.MatchString(line) {
			m.Rebalances++
		}
		if sub := reLagJSON.FindStringSubmatch(line); len(sub) > 1 {
			if v, err := strconv.ParseInt(sub[1], 10, 64); err == nil && v > m.MaxLag {
				m.MaxLag = v
			}
		} else if sub := reLagText.FindStringSubmatch(line); len(sub) > 1 {
			if v, err := strconv.ParseInt(sub[1], 10, 64); err == nil && v > m.MaxLag {
				m.MaxLag = v
			}
		}
		if sub := reProducingRate.FindStringSubmatch(line); len(sub) > 1 {
			if v, err := strconv.ParseInt(sub[1], 10, 64); err == nil && v > m.ProducingRate {
				m.ProducingRate = v
			}
		}
		if sub := reConsumingRate.FindStringSubmatch(line); len(sub) > 1 {
			if v, err := strconv.ParseInt(sub[1], 10, 64); err == nil && v > m.ConsumingRate {
				m.ConsumingRate = v
			}
		}
	}
	return m
}

func extractLogTimestamp(line string) string {
	if m := reLogTimestamp.FindString(line); m != "" {
		return m
	}
	return ""
}

func buildKafkaLogsSignal(cfg KafkaConfig, m kafkaLogMetrics, status string, criticals, warnings []string, selector string) string {
	parts := []string{}

	if m.MaxLag >= 0 {
		parts = append(parts, fmt.Sprintf("lag %d", m.MaxLag))
	}
	if m.ProducingRate > 0 {
		parts = append(parts, fmt.Sprintf("producing %d/min", m.ProducingRate))
	}
	if m.ConsumingRate > 0 {
		parts = append(parts, fmt.Sprintf("consuming %d/min", m.ConsumingRate))
	}

	errParts := []string{}
	if m.EpochErrors == 0 {
		errParts = append(errParts, "zero epoch errors")
	} else {
		errParts = append(errParts, fmt.Sprintf("%d epoch errors", m.EpochErrors))
	}
	if m.Rebalances > 0 {
		errParts = append(errParts, fmt.Sprintf("%d rebalances", m.Rebalances))
	}
	parts = append(parts, strings.Join(errParts, ", "))

	label := cfg.Topic
	if label == "" {
		label = selector
	}
	base := fmt.Sprintf("%s — %s (últim%s %s)", label,
		strings.Join(parts, ", "),
		func() string {
			if strings.HasSuffix(cfg.Window, "m") || cfg.Window == "" {
				return "os"
			}
			return "as"
		}(),
		cfg.Window)

	switch status {
	case "critical":
		return base + " — CRÍTICO: " + strings.Join(criticals, "; ")
	case "warn":
		return base + " — atenção: " + strings.Join(warnings, "; ")
	default:
		return base + " — saudável"
	}
}

// ── datadog source ────────────────────────────────────────────────────────────

type ddMetricsResponse struct {
	Series []ddSeries `json:"series"`
}

type ddSeries struct {
	Metric    string      `json:"metric"`
	Pointlist [][]float64 `json:"pointlist"`
	Scope     string      `json:"scope"`
}

func checkKafkaFromDatadog(cfg KafkaConfig) OpsResult {
	if cfg.DatadogAPIKey == "" || cfg.DatadogAppKey == "" {
		return OpsResult{
			Status: "error",
			Signal: "credenciais Datadog não configuradas — informe --dd-api-key e --dd-app-key ou defina DD_API_KEY e DD_APP_KEY",
			Data:   map[string]any{"source": "datadog"},
			Actions: []string{
				"exportar: export DD_API_KEY=<key> DD_APP_KEY=<key>",
				"ou usar --dd-api-key e --dd-app-key",
			},
			Cost: "zero-llm",
		}
	}

	query := cfg.DatadogQuery
	if query == "" {
		query = buildDatadogQuery(cfg)
	}

	dur, err := time.ParseDuration(cfg.Window)
	if err != nil {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("janela de tempo inválida '%s': %v", cfg.Window, err),
			Data:    map[string]any{"window": cfg.Window},
			Actions: []string{"exemplos válidos: 10m, 1h, 2h30m"},
			Cost:    "zero-llm",
		}
	}

	now := time.Now()
	from := now.Add(-dur)

	series, err := fetchDatadogMetrics(cfg, query, from, now)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("falha ao consultar Datadog ('%s'): %v", query, err),
			Data:   map[string]any{"query": query, "source": "datadog"},
			Actions: []string{
				"verificar DD_API_KEY e DD_APP_KEY",
				"verificar DD site: " + cfg.DatadogSite,
			},
			Cost: "zero-llm",
		}
	}

	return evaluateKafkaDatadog(cfg, series, query, from, now)
}

func buildDatadogQuery(cfg KafkaConfig) string {
	filters := []string{}
	if cfg.Topic != "" {
		filters = append(filters, "topic:"+cfg.Topic)
	}
	if cfg.ConsumerGroup != "" {
		filters = append(filters, "consumer_group:"+cfg.ConsumerGroup)
	}
	scope := "*"
	if len(filters) > 0 {
		scope = strings.Join(filters, ",")
	}
	return fmt.Sprintf("avg:kafka.consumer_lag{%s}", scope)
}

func fetchDatadogMetrics(cfg KafkaConfig, query string, from, to time.Time) ([]ddSeries, error) {
	endpoint := fmt.Sprintf("https://api.%s/api/v1/query", cfg.DatadogSite)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("from", strconv.FormatInt(from.Unix(), 10))
	q.Set("to", strconv.FormatInt(to.Unix(), 10))
	q.Set("query", query)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("DD-API-KEY", cfg.DatadogAPIKey)
	req.Header.Set("DD-APPLICATION-KEY", cfg.DatadogAppKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result ddMetricsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("resposta inválida: %v", err)
	}
	return result.Series, nil
}

func evaluateKafkaDatadog(cfg KafkaConfig, series []ddSeries, query string, from, to time.Time) OpsResult {
	if len(series) == 0 || len(series[0].Pointlist) == 0 {
		return OpsResult{
			Status: "warn",
			Signal: fmt.Sprintf("nenhuma métrica encontrada para '%s' — topic/consumer-group corretos?", query),
			Data: map[string]any{
				"source": "datadog",
				"query":  query,
				"window": cfg.Window,
			},
			Actions: []string{
				"verificar --topic e --consumer-group",
				"usar --dd-query para query customizada",
			},
			Cost: "zero-llm",
		}
	}

	points := series[0].Pointlist
	currentLag := int64(points[len(points)-1][1])
	trend := computeMetricTrend(points)

	var criticals, warnings, actions []string

	// Heuristic: growing lag
	if trend == "growing" {
		growthPct := computeGrowthPct(points)
		if growthPct >= criticalLagGrowthPct {
			criticals = append(criticals,
				fmt.Sprintf("lag crescendo %.0f%% na janela de %s (atual: %d)", growthPct, cfg.Window, currentLag))
			actions = append(actions,
				"investigar lentidão no consumer: verificar CPU, GC, I/O do pod consumidor",
				"verificar se há pause/stop no consumer group",
			)
		} else {
			warnings = append(warnings,
				fmt.Sprintf("lag crescendo %.0f%% na janela de %s (atual: %d)", growthPct, cfg.Window, currentLag))
			actions = append(actions,
				"monitorar por mais %s — se mantiver tendência, investigar consumer", cfg.Window,
			)
		}
	}

	status := "ok"
	if len(criticals) > 0 {
		status = "critical"
	} else if len(warnings) > 0 {
		status = "warn"
	}

	signal := buildKafkaDatadogSignal(cfg, currentLag, trend, status, criticals, warnings)

	scope := series[0].Scope
	data := map[string]any{
		"source":      "datadog",
		"query":       query,
		"scope":       scope,
		"window":      cfg.Window,
		"lag_current": currentLag,
		"trend":       trend,
		"data_points": len(points),
	}
	if cfg.Topic != "" {
		data["topic"] = cfg.Topic
	}
	if cfg.ConsumerGroup != "" {
		data["consumer_group"] = cfg.ConsumerGroup
	}
	if len(criticals) > 0 {
		data["criticals"] = criticals
	}
	if len(warnings) > 0 {
		data["warnings"] = warnings
	}

	return OpsResult{
		Status:  status,
		Signal:  signal,
		Data:    data,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// computeMetricTrend compares the average of the first third vs the last third
// of a time series to determine trend direction.
func computeMetricTrend(points [][]float64) string {
	n := len(points)
	if n < 3 {
		return "stable"
	}

	third := n / 3
	firstAvg := avgSlice(points[:third])
	lastAvg := avgSlice(points[n-third:])

	if firstAvg == 0 && lastAvg == 0 {
		return "stable"
	}

	var growthPct float64
	if firstAvg > 0 {
		growthPct = (lastAvg - firstAvg) / firstAvg * 100
	} else {
		growthPct = 100 // was zero, now non-zero → growing
	}

	switch {
	case growthPct >= float64(warnLagGrowthPct):
		return "growing"
	case growthPct <= -float64(warnLagGrowthPct):
		return "decreasing"
	default:
		return "stable"
	}
}

func computeGrowthPct(points [][]float64) float64 {
	n := len(points)
	if n < 3 {
		return 0
	}
	third := n / 3
	firstAvg := avgSlice(points[:third])
	lastAvg := avgSlice(points[n-third:])
	if firstAvg == 0 {
		return 100
	}
	return (lastAvg - firstAvg) / firstAvg * 100
}

func avgSlice(points [][]float64) float64 {
	if len(points) == 0 {
		return 0
	}
	var sum float64
	for _, p := range points {
		if len(p) >= 2 {
			sum += p[1]
		}
	}
	return sum / float64(len(points))
}

func buildKafkaDatadogSignal(cfg KafkaConfig, lag int64, trend, status string, criticals, warnings []string) string {
	trendLabel := map[string]string{
		"growing":    "crescendo",
		"decreasing": "decrescendo",
		"stable":     "estável",
	}[trend]

	label := cfg.Topic
	if label == "" && cfg.ConsumerGroup != "" {
		label = cfg.ConsumerGroup
	}
	if label == "" {
		label = "kafka"
	}

	base := fmt.Sprintf("%s — lag %d (%s, últimas %s)", label, lag, trendLabel, cfg.Window)

	switch status {
	case "critical":
		return base + " — CRÍTICO: " + strings.Join(criticals, "; ")
	case "warn":
		return base + " — atenção: " + strings.Join(warnings, "; ")
	default:
		return base + " — saudável"
	}
}
