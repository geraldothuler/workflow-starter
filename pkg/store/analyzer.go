package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

// AnalysisResult holds the LLM analysis of ops history.
type AnalysisResult struct {
	Patterns       []Pattern       `json:"patterns"`
	SuggestedRules []SuggestedRule `json:"suggested_rules"`
	Confidence     float64         `json:"confidence"`
	DataPoints     int             `json:"data_points"`
	TimeRange      string          `json:"time_range"`
}

// Pattern is an observed anomaly or trend in the ops history.
type Pattern struct {
	Probe       string  `json:"probe"`
	Observation string  `json:"observation"`
	Confidence  float64 `json:"confidence"`
}

// SuggestedRule is a TrendRule proposed by LLM, with supporting rationale.
// Compatible with store_rules.yml — Rationale is omitempty for YAML serialization.
type SuggestedRule struct {
	Probe             string `json:"probe"              yaml:"probe"`
	Window            int    `json:"window"             yaml:"window"`
	ConsecutiveStatus string `json:"consecutive_status" yaml:"consecutive_status"`
	EscalateTo        string `json:"escalate_to"        yaml:"escalate_to"`
	Signal            string `json:"signal"             yaml:"signal"`
	Rationale         string `json:"rationale"          yaml:"rationale,omitempty"`
}

// ToTrendRule converts to a TrendRule for direct use in evalRules (drops Rationale).
func (s SuggestedRule) ToTrendRule() TrendRule {
	return TrendRule{
		Probe:             s.Probe,
		Window:            s.Window,
		ConsecutiveStatus: s.ConsecutiveStatus,
		EscalateTo:        s.EscalateTo,
		Signal:            s.Signal,
	}
}

const analysisMaxRecords = 200
const analysisMaxTokens = 2000

// AnalyzeHistory sends the last analysisMaxRecords entries from the SQLite log
// to the LLM and returns pattern observations + suggested trend rules.
// Returns an empty AnalysisResult (no LLM call) when the database is empty.
func AnalyzeHistory(path string, provider llm.LLMProvider) (*AnalysisResult, error) {
	records := QueryTrend(path, "", analysisMaxRecords)
	if len(records) == 0 {
		return &AnalysisResult{}, nil
	}

	prompt := buildAnalysisPrompt(records)
	raw, _, err := provider.CompleteWithUsage(prompt, analysisMaxTokens)
	if err != nil {
		return nil, fmt.Errorf("llm analysis failed: %w", err)
	}

	raw = strings.TrimSpace(raw)
	var result AnalysisResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse analysis response: %w", err)
	}
	return &result, nil
}

// buildAnalysisPrompt formats ops records for the LLM.
// Contains the "sugerir_regras" keyword so the mock provider routes correctly.
func buildAnalysisPrompt(records []Record) string {
	var sb strings.Builder
	sb.WriteString("Você é um SRE analisando histórico de execuções de probes operacionais.\n\n")
	sb.WriteString("HISTÓRICO (ts | probe | status | signal):\n")
	for _, r := range records {
		ts := r.Ts
		if len(ts) > 16 {
			ts = ts[:16]
		}
		sb.WriteString(fmt.Sprintf("  %s | %-20s | %-8s | %s\n", ts, r.Probe, r.Status, r.Signal))
	}
	sb.WriteString("\nTAREFA: sugerir_regras YAML compatíveis com store_rules.yml.\n")
	sb.WriteString("Identifique padrões de degradação recorrentes e proponha regras de heurística\n")
	sb.WriteString("de segunda ordem (janelas de execuções consecutivas com mesmo status).\n\n")
	sb.WriteString("FORMATO DE SAÍDA (JSON estrito, sem markdown):\n")
	sb.WriteString(`{
  "patterns": [{"probe": "...", "observation": "...", "confidence": 0.0}],
  "suggested_rules": [{"probe": "...", "window": 3, "consecutive_status": "warn", "escalate_to": "critical", "signal": "...", "rationale": "..."}],
  "confidence": 0.0,
  "data_points": 0,
  "time_range": "24h"
}`)
	return sb.String()
}
