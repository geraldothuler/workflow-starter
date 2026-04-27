package ops

import (
	"os"
	"strings"
	"testing"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// humanLines
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestHumanLines(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{10000, "10.0K"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := humanLines(tc.n)
			if got != tc.want {
				t.Errorf("humanLines(%d) = %q, want %q", tc.n, got, tc.want)
			}
		})
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// shortTime
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestShortTime(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"2026-02-22T09:10:03", "09:10:03"},
		{"2026-02-22 09:10:03", "09:10:03"},
		{"short", "short"}, // < 19 chars → returned as-is
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := shortTime(tc.input)
			if got != tc.want {
				t.Errorf("shortTime(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// severityRank
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestSeverityRank(t *testing.T) {
	if severityRank("critical") <= severityRank("warn") {
		t.Error("critical should rank higher than warn")
	}
	if severityRank("warn") <= severityRank("info") {
		t.Error("warn should rank higher than unknown")
	}
	if severityRank("critical") != 2 {
		t.Errorf("expected critical=2, got %d", severityRank("critical"))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// trimPodPrefix
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestTrimPodPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"[pod/fusca-cdc-abc-1] ERROR something bad", "ERROR something bad"},
		{"pod/fusca-cdc-abc-1 ERROR something bad", "ERROR something bad"},
		{"ERROR no prefix here", "ERROR no prefix here"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := trimPodPrefix(tc.input)
			if got != tc.want {
				t.Errorf("trimPodPrefix(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// selectPatterns
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestSelectPatterns_EmptyReturnsAll(t *testing.T) {
	got := selectPatterns(nil)
	if len(got) != len(builtinLogPatterns) {
		t.Errorf("expected %d patterns, got %d", len(builtinLogPatterns), len(got))
	}
}

func TestSelectPatterns_AllKeyword(t *testing.T) {
	got := selectPatterns([]string{"all"})
	if len(got) != len(builtinLogPatterns) {
		t.Errorf("expected all %d patterns, got %d", len(builtinLogPatterns), len(got))
	}
}

func TestSelectPatterns_ShortAlias(t *testing.T) {
	got := selectPatterns([]string{"kafka"})
	if len(got) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(got))
	}
	if got[0].ID != "kafka-epoch" {
		t.Errorf("expected kafka-epoch, got %q", got[0].ID)
	}
}

func TestSelectPatterns_MultipleAliases(t *testing.T) {
	got := selectPatterns([]string{"kafka", "panic", "oom"})
	if len(got) != 3 {
		t.Errorf("expected 3 patterns, got %d", len(got))
	}
}

func TestSelectPatterns_Dedup(t *testing.T) {
	// "kafka" and "kafka-epoch" both resolve to kafka-epoch
	got := selectPatterns([]string{"kafka", "kafka-epoch"})
	if len(got) != 1 {
		t.Errorf("expected dedup to 1 pattern, got %d", len(got))
	}
}

func TestSelectPatterns_UnknownReturnsNil(t *testing.T) {
	got := selectPatterns([]string{"unknown-pattern"})
	if got != nil {
		t.Errorf("expected nil for unknown pattern, got %v", got)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// scanLog
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestScanLog_MatchesEpochError(t *testing.T) {
	log := "2026-02-22T09:10:00 ERROR InvalidProducerEpochException\n" +
		"2026-02-22T09:10:01 INFO normal line\n" +
		"2026-02-22T09:10:02 ERROR ProducerFencedException\n"

	r := strings.NewReader(log)
	patterns := selectPatterns([]string{"kafka"})
	total, anomalies := scanLog(r, patterns, "")

	if total != 3 {
		t.Errorf("expected 3 total lines, got %d", total)
	}
	if len(anomalies) != 1 {
		t.Fatalf("expected 1 anomaly, got %d", len(anomalies))
	}
	if anomalies[0].Count != 2 {
		t.Errorf("expected count=2, got %d", anomalies[0].Count)
	}
	if anomalies[0].PatternID != "kafka-epoch" {
		t.Errorf("expected kafka-epoch, got %q", anomalies[0].PatternID)
	}
	if anomalies[0].Severity != "critical" {
		t.Errorf("expected critical severity, got %q", anomalies[0].Severity)
	}
}

func TestScanLog_SinceFilter(t *testing.T) {
	// Only line at 09:20 should be counted; 09:10 and 09:15 are before "since"
	log := "2026-02-22T09:10:00 ERROR InvalidProducerEpochException early\n" +
		"2026-02-22T09:15:00 ERROR InvalidProducerEpochException before-since\n" +
		"2026-02-22T09:20:00 ERROR InvalidProducerEpochException after-since\n"

	r := strings.NewReader(log)
	patterns := selectPatterns([]string{"kafka"})
	_, anomalies := scanLog(r, patterns, "2026-02-22T09:18:00")

	if len(anomalies) != 1 {
		t.Fatalf("expected 1 anomaly after since filter, got %d", len(anomalies))
	}
	if anomalies[0].Count != 1 {
		t.Errorf("expected count=1 (only lines after since), got %d", anomalies[0].Count)
	}
}

func TestScanLog_MaxExamplesThree(t *testing.T) {
	// 5 epoch errors — examples should be capped at 3
	log := "2026-02-22T09:10:00 ERROR InvalidProducerEpochException 1\n" +
		"2026-02-22T09:10:01 ERROR InvalidProducerEpochException 2\n" +
		"2026-02-22T09:10:02 ERROR InvalidProducerEpochException 3\n" +
		"2026-02-22T09:10:03 ERROR InvalidProducerEpochException 4\n" +
		"2026-02-22T09:10:04 ERROR InvalidProducerEpochException 5\n"

	r := strings.NewReader(log)
	patterns := selectPatterns([]string{"kafka"})
	_, anomalies := scanLog(r, patterns, "")

	if len(anomalies[0].Examples) > 3 {
		t.Errorf("examples should be capped at 3, got %d", len(anomalies[0].Examples))
	}
}

func TestScanLog_NoMatches(t *testing.T) {
	log := "2026-02-22T09:00:00 INFO everything is fine\n" +
		"2026-02-22T09:00:01 INFO another normal line\n"

	r := strings.NewReader(log)
	patterns := builtinLogPatterns
	total, anomalies := scanLog(r, patterns, "")

	if total != 2 {
		t.Errorf("expected 2 total lines, got %d", total)
	}
	if len(anomalies) != 0 {
		t.Errorf("expected no anomalies, got %d", len(anomalies))
	}
}

func TestScanLog_FirstAndLastSeen(t *testing.T) {
	log := "2026-02-22T09:10:00 ERROR InvalidProducerEpochException first\n" +
		"2026-02-22T09:15:00 ERROR InvalidProducerEpochException middle\n" +
		"2026-02-22T09:20:00 ERROR InvalidProducerEpochException last\n"

	r := strings.NewReader(log)
	patterns := selectPatterns([]string{"kafka"})
	_, anomalies := scanLog(r, patterns, "")

	if anomalies[0].FirstSeen != "2026-02-22T09:10:00" {
		t.Errorf("unexpected FirstSeen: %q", anomalies[0].FirstSeen)
	}
	if anomalies[0].LastSeen != "2026-02-22T09:20:00" {
		t.Errorf("unexpected LastSeen: %q", anomalies[0].LastSeen)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// buildLogsSignal
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBuildLogsSignal_NoAnomalies(t *testing.T) {
	sig := buildLogsSignal("app.log", 5000, nil, "", "ok")
	if !strings.Contains(sig, "zero anomalias") {
		t.Errorf("expected 'zero anomalias' in signal: %q", sig)
	}
	if !strings.Contains(sig, "5.0K") {
		t.Errorf("expected humanized line count: %q", sig)
	}
}

func TestBuildLogsSignal_WithSince(t *testing.T) {
	sig := buildLogsSignal("app.log", 100, nil, "2026-02-22T09:00:00", "ok")
	if !strings.Contains(sig, "2026-02-22T09:00:00") {
		t.Errorf("expected since in signal: %q", sig)
	}
}

func TestBuildLogsSignal_WithAnomalies(t *testing.T) {
	anomalies := []LogAnomalySummary{
		{PatternID: "kafka-epoch", Count: 3, Severity: "critical",
			FirstSeen: "2026-02-22T09:10:00", LastSeen: "2026-02-22T09:15:00"},
	}
	sig := buildLogsSignal("app.log", 10000, anomalies, "", "critical")
	if !strings.Contains(sig, "kafka-epoch") {
		t.Errorf("expected anomaly ID in signal: %q", sig)
	}
	if !strings.Contains(sig, "CRÍTICO") {
		t.Errorf("expected CRÍTICO in critical signal: %q", sig)
	}
}

func TestBuildLogsSignal_Stdin(t *testing.T) {
	sig := buildLogsSignal("", 100, nil, "", "ok")
	if !strings.Contains(sig, "stdin") {
		t.Errorf("expected 'stdin' in signal: %q", sig)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CheckLogsAnalyze — file-based integration
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckLogsAnalyze_FromFile(t *testing.T) {
	content := "2026-02-22T09:10:00 ERROR InvalidProducerEpochException\n" +
		"2026-02-22T09:10:01 INFO normal line\n" +
		"2026-02-22T09:10:02 ERROR ProducerFencedException\n"

	f, err := os.CreateTemp(t.TempDir(), "test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()

	cfg := LogsConfig{FilePath: f.Name(), Patterns: []string{"kafka"}}
	r := CheckLogsAnalyze(cfg)

	if r.Status != "critical" {
		t.Errorf("expected critical, got %q (signal: %s)", r.Status, r.Signal)
	}
	if r.Cost != "zero-llm" {
		t.Errorf("expected zero-llm, got %q", r.Cost)
	}
	anomalies, ok := r.Data["anomalies"].([]LogAnomalySummary)
	if !ok || len(anomalies) == 0 {
		t.Errorf("expected anomalies in data")
	}
}

func TestCheckLogsAnalyze_UnknownPattern(t *testing.T) {
	cfg := LogsConfig{Patterns: []string{"nonexistent-pattern"}}
	r := CheckLogsAnalyze(cfg)
	if r.Status != "error" {
		t.Errorf("expected error for unknown pattern, got %q", r.Status)
	}
}

func TestCheckLogsAnalyze_FileNotFound(t *testing.T) {
	cfg := LogsConfig{FilePath: "/nonexistent/path/app.log"}
	r := CheckLogsAnalyze(cfg)
	if r.Status != "error" {
		t.Errorf("expected error for missing file, got %q", r.Status)
	}
}

func TestCheckLogsAnalyze_SortCriticalFirst(t *testing.T) {
	content := "2026-02-22T09:10:00 INFO slow query duration: 5000ms\n" +
		"2026-02-22T09:10:01 ERROR OutOfMemoryError: Java heap space\n"

	f, err := os.CreateTemp(t.TempDir(), "test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()

	cfg := LogsConfig{FilePath: f.Name(), Patterns: []string{"oom", "slow-query"}}
	r := CheckLogsAnalyze(cfg)

	anomalies, ok := r.Data["anomalies"].([]LogAnomalySummary)
	if !ok || len(anomalies) < 2 {
		t.Fatalf("expected 2 anomalies, got %d", len(anomalies))
	}
	if anomalies[0].Severity != "critical" {
		t.Errorf("critical anomaly should sort first, got %q", anomalies[0].Severity)
	}
}

func TestCheckLogsAnalyze_AllPatterns(t *testing.T) {
	content := "2026-02-22T09:10:00 INFO everything is fine here\n"

	f, err := os.CreateTemp(t.TempDir(), "test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()

	cfg := LogsConfig{FilePath: f.Name(), Patterns: []string{"all"}}
	r := CheckLogsAnalyze(cfg)

	if r.Status != "ok" {
		t.Errorf("expected ok for clean logs, got %q", r.Status)
	}
}
