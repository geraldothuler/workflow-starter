package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// MonteCarloConfig holds Monte Carlo API settings and probe selection.
type MonteCarloConfig struct {
	APIKey  string
	APIToken string
	ProbeID  string            // which probe to run (default: first in YAML)
	Vars     map[string]string // template vars: RuleUUID, StartTime, EndTime
	TableID  string            // deprecated: kept for backward compat
}

// mcProbeConfig declares a GraphQL probe and field extraction rules.
type mcProbeConfig struct {
	ID      string            `yaml:"id,omitempty"`
	GQL     string            `yaml:"gql,omitempty"`
	Extract map[string]string `yaml:"extract,omitempty"` // result_field → dot-notation path
}

// mcYAMLConfig is the full structure of montecarlo.yml.
type mcYAMLConfig struct {
	Endpoint   string          `yaml:"endpoint,omitempty"`
	Probes     []mcProbeConfig `yaml:"probes,omitempty"`
	Heuristics []HeuristicRule `yaml:"heuristics,omitempty"`
}

func loadMCConfig() (mcYAMLConfig, error) {
	data, err := heuristicsFS.ReadFile("config/heuristics/montecarlo.yml")
	if err != nil {
		return mcYAMLConfig{}, fmt.Errorf("montecarlo: failed to read montecarlo.yml: %w", err)
	}
	var cfg mcYAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return mcYAMLConfig{}, fmt.Errorf("montecarlo: invalid montecarlo.yml: %w", err)
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://api.getmontecarlo.com/graphql"
	}
	return cfg, nil
}

// CheckMonteCarlo runs a YAML-declared probe against the Monte Carlo GraphQL API.
func CheckMonteCarlo(cfg MonteCarloConfig) OpsResult {
	if cfg.APIKey == "" || cfg.APIToken == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "Monte Carlo: missing API key or token",
			Actions: []string{"set --api-key <key-id> --api-token <secret>"},
			Cost:    "zero-llm",
		}
	}

	mcCfg, err := loadMCConfig()
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Monte Carlo: config error: %v", err),
			Cost:   "zero-llm",
		}
	}

	probe, ok := selectProbe(mcCfg.Probes, cfg.ProbeID)
	if !ok {
		return OpsResult{
			Status:  "error",
			Signal:  "Monte Carlo: no probe configured in montecarlo.yml",
			Cost:    "zero-llm",
		}
	}

	gql, err := templateGQL(probe.GQL, cfg.Vars)
	if err != nil {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("Monte Carlo: GQL template error: %v", err),
			Cost:    "zero-llm",
		}
	}

	body, statusCode, err := httpPost(mcCfg.Endpoint, map[string]string{
		"x-mcd-id":     cfg.APIKey,
		"x-mcd-token":  cfg.APIToken,
		"Content-Type": "application/json",
	}, []byte(`{"query": `+jsonStr(gql)+`}`))
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Monte Carlo API error: %v", err),
			Cost:   "zero-llm",
		}
	}
	if statusCode != 200 {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Monte Carlo API returned HTTP %d", statusCode),
			Cost:   "zero-llm",
		}
	}

	data, err := extractMCFields(body, probe.Extract)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("Monte Carlo: failed to parse response: %v", err),
			Cost:   "zero-llm",
		}
	}

	hStatus, hSignal, hActions := EvalHeuristics(data, mcCfg.Heuristics)
	signal := "Monte Carlo: no active breaches"
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

// selectProbe returns the probe matching probeID, or the first probe if probeID is empty.
func selectProbe(probes []mcProbeConfig, probeID string) (mcProbeConfig, bool) {
	if len(probes) == 0 {
		return mcProbeConfig{}, false
	}
	if probeID == "" {
		return probes[0], true
	}
	for _, p := range probes {
		if p.ID == probeID {
			return p, true
		}
	}
	return mcProbeConfig{}, false
}

// templateGQL renders a GQL string using text/template with the provided vars.
// Default vars (StartTime, EndTime) are injected if not already set.
func templateGQL(gql string, vars map[string]string) (string, error) {
	if !strings.Contains(gql, "{{") {
		return gql, nil
	}
	all := map[string]string{
		"StartTime": time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339),
		"EndTime":   time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range vars {
		all[k] = v
	}
	tmpl, err := template.New("gql").Parse(gql)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, all); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// extractMCFields extracts named numeric fields from a Monte Carlo GraphQL response.
// Each entry in extract maps a result field name to a dot-notation path into data.*.
// Returns an error if the response contains GraphQL errors (even on HTTP 200).
func extractMCFields(body []byte, extract map[string]string) (map[string]any, error) {
	var resp struct {
		Data   map[string]any `json:"data"`
		Errors []any          `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		errBytes, _ := json.Marshal(resp.Errors)
		return nil, fmt.Errorf("GraphQL errors: %s", errBytes)
	}
	if resp.Data == nil {
		return map[string]any{}, nil
	}
	result := make(map[string]any, len(extract))
	for field, path := range extract {
		if v, ok := mcExtractNumeric(resp.Data, path); ok {
			result[field] = v
		}
	}
	return result, nil
}

// ── Path extractor ─────────────────────────────────────────────────────────────
//
// Supports dot-notation paths with optional array access and sum aggregation:
//   "key"              → map key lookup
//   "a.b"             → nested key lookup
//   "arr[-1].field"   → last element of array, then field
//   "arr[*].field|sum" → sum of field across all array elements

type mcSeg struct {
	typ string // "key" | "idx" | "star"
	key string
	idx int
}

func parseMCPath(path string) []mcSeg {
	var segs []mcSeg
	for _, part := range strings.Split(path, ".") {
		if i := strings.Index(part, "["); i >= 0 {
			if key := part[:i]; key != "" {
				segs = append(segs, mcSeg{typ: "key", key: key})
			}
			inner := part[i+1 : len(part)-1]
			if inner == "*" {
				segs = append(segs, mcSeg{typ: "star"})
			} else {
				idx, err := strconv.Atoi(inner)
				if err != nil {
					// invalid index (e.g. "arr[last]") — skip segment, navigate will return nil
					segs = append(segs, mcSeg{typ: "invalid"})
				} else {
					segs = append(segs, mcSeg{typ: "idx", idx: idx})
				}
			}
		} else if part != "" {
			segs = append(segs, mcSeg{typ: "key", key: part})
		}
	}
	return segs
}

func mcNavigate(v any, segs []mcSeg) any {
	if len(segs) == 0 {
		return v
	}
	seg := segs[0]
	rest := segs[1:]
	switch seg.typ {
	case "key":
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		return mcNavigate(m[seg.key], rest)
	case "idx":
		arr, ok := v.([]any)
		if !ok {
			return nil
		}
		idx := seg.idx
		if idx < 0 {
			idx = len(arr) + idx
		}
		if idx < 0 || idx >= len(arr) {
			return nil
		}
		return mcNavigate(arr[idx], rest)
	case "star":
		arr, ok := v.([]any)
		if !ok {
			return nil
		}
		var results []any
		for _, item := range arr {
			if r := mcNavigate(item, rest); r != nil {
				results = append(results, r)
			}
		}
		return results
	}
	return nil
}

func mcExtractNumeric(data map[string]any, path string) (float64, bool) {
	agg := ""
	if i := strings.LastIndex(path, "|"); i >= 0 {
		agg = path[i+1:]
		path = path[:i]
	}
	raw := mcNavigate(data, parseMCPath(path))
	if raw == nil {
		return 0, false
	}
	if agg == "sum" {
		arr, ok := raw.([]any)
		if !ok {
			return 0, false
		}
		var sum float64
		for _, item := range arr {
			if n, ok := toFloat64(item); ok {
				sum += n
			}
		}
		return sum, true
	}
	return toFloat64(raw)
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	case string:
		n, err := strconv.ParseFloat(x, 64)
		return n, err == nil
	}
	return 0, false
}

// jsonStr encodes s as a JSON string literal (with quotes).
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
