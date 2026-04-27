package ops

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// LogsConfig holds parameters for log analysis.
type LogsConfig struct {
	FilePath string   // path to log file; "-" or "" reads from stdin
	Patterns []string // pattern IDs: kafka, lock, oom, slow-query, connection, panic, all
	Since    string   // skip lines whose timestamp < Since (lexicographic, ISO-8601 prefix)
	Limit    int      // max anomaly summaries returned (default: 50)
}

// logPattern defines a named pattern to detect in log lines.
type logPattern struct {
	ID       string
	Re       *regexp.Regexp
	Severity string // critical | warn
	Actions  []string
}

// LogAnomalySummary is one entry in the anomalies list — one per matched pattern.
type LogAnomalySummary struct {
	PatternID string   `json:"pattern"`
	Count     int64    `json:"count"`
	FirstSeen string   `json:"first_seen,omitempty"`
	LastSeen  string   `json:"last_seen,omitempty"`
	Severity  string   `json:"severity"`
	Examples  []string `json:"examples,omitempty"` // up to 3 representative lines
}

// builtinLogPatterns is the default set used when no custom patterns are provided.
var builtinLogPatterns = []logPattern{
	{
		ID:       "kafka-epoch",
		Re:       regexp.MustCompile(`InvalidProducerEpochException|ProducerFencedException`),
		Severity: "critical",
		Actions: []string{
			"investigar epoch conflict: verificar se múltiplos produtores compartilham o mesmo transactional.id",
			"contexto Fusca: PR #1077 (revert) resolve por reset do transactional state",
		},
	},
	{
		ID:       "lock-wait",
		Re:       regexp.MustCompile(`(?i)lock wait timeout|deadlock detected|LockNotAvailable`),
		Severity: "critical",
		Actions: []string{
			"identificar root blocker: SELECT pid,query FROM pg_stat_activity WHERE wait_event_type='Lock'",
		},
	},
	{
		ID:       "oom",
		Re:       regexp.MustCompile(`OutOfMemoryError|java\.lang\.OutOfMemory|OOM killer|Killed process`),
		Severity: "critical",
		Actions: []string{
			"verificar memória dos pods: kubectl top pods",
			"revisar memory limits no deployment",
		},
	},
	{
		ID:       "slow-query",
		Re:       regexp.MustCompile(`(?i)slow query|duration: [5-9]\d{3,}ms|took \d{4,}ms`),
		Severity: "warn",
		Actions: []string{
			"investigar queries lentas: SELECT pid,now()-query_start,left(query,80) FROM pg_stat_activity WHERE state='active'",
		},
	},
	{
		ID:       "connection",
		Re:       regexp.MustCompile(`(?i)connection refused|connection reset by peer|broken pipe|EOF reading`),
		Severity: "warn",
		Actions:  []string{"verificar conectividade entre serviços e status dos endpoints"},
	},
	{
		ID:       "panic",
		Re:       regexp.MustCompile(`panic:|runtime error:|goroutine \d+ \[running\]`),
		Severity: "critical",
		Actions: []string{
			"verificar stack trace completo",
			"reiniciar serviço se necessário: kubectl rollout restart deployment/<name>",
		},
	},
}

// patternAliases maps short names used in --patterns to pattern IDs.
var patternAliases = map[string]string{
	"kafka":      "kafka-epoch",
	"lock":       "lock-wait",
	"oom":        "oom",
	"slow-query": "slow-query",
	"connection": "connection",
	"panic":      "panic",
	// canonical IDs also accepted
	"kafka-epoch": "kafka-epoch",
	"lock-wait":   "lock-wait",
}

// CheckLogsAnalyze scans a log file (or stdin) for known anomaly patterns.
func CheckLogsAnalyze(cfg LogsConfig) OpsResult {
	if cfg.Limit <= 0 {
		cfg.Limit = 50
	}

	patterns := selectPatterns(cfg.Patterns)
	if len(patterns) == 0 {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("padrões desconhecidos: %v — use: kafka, lock, oom, slow-query, connection, panic, all", cfg.Patterns),
			Data:    map[string]any{},
			Actions: []string{"wtb ops logs analyze --patterns all"},
			Cost:    "zero-llm",
		}
	}

	r, closer, err := openLogReader(cfg.FilePath)
	if err != nil {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("não foi possível abrir '%s': %v", cfg.FilePath, err),
			Data:    map[string]any{"file": cfg.FilePath},
			Actions: []string{"verificar caminho do arquivo ou usar stdin: cat arquivo.log | wtb ops logs analyze"},
			Cost:    "zero-llm",
		}
	}
	defer closer()

	totalLines, anomalies := scanLog(r, patterns, cfg.Since)

	// Sort: critical first, then by count descending.
	sort.Slice(anomalies, func(i, j int) bool {
		si, sj := severityRank(anomalies[i].Severity), severityRank(anomalies[j].Severity)
		if si != sj {
			return si > sj
		}
		return anomalies[i].Count > anomalies[j].Count
	})
	if len(anomalies) > cfg.Limit {
		anomalies = anomalies[:cfg.Limit]
	}

	// Collect actions from matched patterns.
	var actions []string
	seen := map[string]bool{}
	for _, a := range anomalies {
		for _, pat := range patterns {
			if pat.ID == a.PatternID {
				for _, act := range pat.Actions {
					if !seen[act] {
						actions = append(actions, act)
						seen[act] = true
					}
				}
			}
		}
	}

	// Determine status.
	status := "ok"
	for _, a := range anomalies {
		if a.Severity == "critical" {
			status = "critical"
			break
		}
		if a.Severity == "warn" && status == "ok" {
			status = "warn"
		}
	}

	signal := buildLogsSignal(cfg.FilePath, totalLines, anomalies, cfg.Since, status)

	return OpsResult{
		Status: status,
		Signal: signal,
		Data: map[string]any{
			"file":        cfg.FilePath,
			"total_lines": totalLines,
			"since":       cfg.Since,
			"anomalies":   anomalies,
		},
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// openLogReader returns a reader for the file path (or stdin if "-"/empty).
func openLogReader(path string) (io.Reader, func(), error) {
	if path == "" || path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { f.Close() }, nil
}

// selectPatterns returns the patterns to run based on the names list.
func selectPatterns(names []string) []logPattern {
	if len(names) == 0 {
		return builtinLogPatterns
	}
	for _, n := range names {
		if n == "all" {
			return builtinLogPatterns
		}
	}

	byID := map[string]logPattern{}
	for _, p := range builtinLogPatterns {
		byID[p.ID] = p
	}

	var selected []logPattern
	seenID := map[string]bool{}
	for _, name := range names {
		id, ok := patternAliases[name]
		if !ok {
			// Unknown name — signal error via empty return.
			return nil
		}
		if !seenID[id] {
			seenID[id] = true
			if p, ok := byID[id]; ok {
				selected = append(selected, p)
			}
		}
	}
	return selected
}

// scanLog reads r line by line, matches patterns, and returns total line count + anomaly summaries.
func scanLog(r io.Reader, patterns []logPattern, since string) (int64, []LogAnomalySummary) {
	type state struct {
		count     int64
		firstSeen string
		lastSeen  string
		examples  []string
	}
	states := make([]state, len(patterns))

	scanner := bufio.NewScanner(r)
	// Allow lines up to 1 MB to handle verbose JSON/stack-trace logs.
	buf := make([]byte, 1*1024*1024)
	scanner.Buffer(buf, cap(buf))

	var totalLines int64

	for scanner.Scan() {
		line := scanner.Text()
		totalLines++

		// Apply --since filter: skip lines with an earlier timestamp.
		if since != "" {
			if ts := extractLogTimestamp(line); ts != "" && ts < since {
				continue
			}
		}

		ts := extractLogTimestamp(line)

		for i, pat := range patterns {
			if pat.Re.MatchString(line) {
				states[i].count++
				if states[i].firstSeen == "" && ts != "" {
					states[i].firstSeen = ts
				}
				if ts != "" {
					states[i].lastSeen = ts
				}
				if len(states[i].examples) < 3 {
					// Trim pod prefix from kubectl --prefix output.
					example := trimPodPrefix(line)
					if len(example) > 200 {
						example = example[:200] + "…"
					}
					states[i].examples = append(states[i].examples, example)
				}
			}
		}
	}

	var anomalies []LogAnomalySummary
	for i, pat := range patterns {
		if states[i].count == 0 {
			continue
		}
		anomalies = append(anomalies, LogAnomalySummary{
			PatternID: pat.ID,
			Count:     states[i].count,
			FirstSeen: states[i].firstSeen,
			LastSeen:  states[i].lastSeen,
			Severity:  pat.Severity,
			Examples:  states[i].examples,
		})
	}
	return totalLines, anomalies
}

// trimPodPrefix removes the "pod/name " prefix added by kubectl logs --prefix.
func trimPodPrefix(line string) string {
	if strings.HasPrefix(line, "[pod/") || strings.HasPrefix(line, "pod/") {
		if idx := strings.Index(line, "] "); idx != -1 {
			return line[idx+2:]
		}
		if idx := strings.Index(line, " "); idx != -1 {
			return line[idx+1:]
		}
	}
	return line
}

func severityRank(s string) int {
	if s == "critical" {
		return 2
	}
	if s == "warn" {
		return 1
	}
	return 0
}

func buildLogsSignal(filePath string, totalLines int64, anomalies []LogAnomalySummary, since, status string) string {
	source := filePath
	if source == "" || source == "-" {
		source = "stdin"
	}

	linesFmt := humanLines(totalLines)

	if len(anomalies) == 0 {
		sinceNote := ""
		if since != "" {
			sinceNote = " (desde " + since + ")"
		}
		return fmt.Sprintf("%s — %s linhas%s — zero anomalias detectadas", source, linesFmt, sinceNote)
	}

	// Build compact summary of critical anomalies first.
	var parts []string
	for _, a := range anomalies {
		part := fmt.Sprintf("%d %s", a.Count, a.PatternID)
		if a.FirstSeen != "" && a.LastSeen != "" && a.FirstSeen != a.LastSeen {
			part += fmt.Sprintf(" (%s→%s)", shortTime(a.FirstSeen), shortTime(a.LastSeen))
		} else if a.FirstSeen != "" {
			part += fmt.Sprintf(" (%s)", shortTime(a.FirstSeen))
		}
		parts = append(parts, part)
		if len(parts) >= 3 {
			break
		}
	}

	base := fmt.Sprintf("%s — %s linhas — %s", source, linesFmt, strings.Join(parts, "; "))

	switch status {
	case "critical":
		return base + " — CRÍTICO"
	case "warn":
		return base + " — atenção"
	default:
		return base + " — ok"
	}
}

// shortTime returns just the time portion of a timestamp string.
func shortTime(ts string) string {
	if len(ts) >= 19 {
		// "2026-02-22T09:10:03" or "2026-02-22 09:10:03"
		return ts[11:19]
	}
	return ts
}

// humanLines formats a line count with K/M suffix.
func humanLines(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
