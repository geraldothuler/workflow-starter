package ops

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// humanDuration
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestHumanDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{90 * time.Minute, "1h"},
		{23 * time.Hour, "23h"},
		{48 * time.Hour, "2d"},
		{7 * 24 * time.Hour, "7d"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := humanDuration(tc.d)
			if got != tc.want {
				t.Errorf("humanDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// parseK8sPods
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestParseK8sPods_AllReady(t *testing.T) {
	raw := mustMarshalPodList(t, []k8sPodItem{
		makePod("fusca-api-app-abc-1", "abc123", true, 0, "", ""),
		makePod("fusca-api-app-abc-2", "abc123", true, 0, "", ""),
	})

	pods, hash, err := parseK8sPods(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	if hash != "abc123" {
		t.Errorf("expected hash abc123, got %q", hash)
	}
	for _, p := range pods {
		if !p.Ready {
			t.Errorf("pod %q should be ready", p.Name)
		}
		if p.Restarts != 0 {
			t.Errorf("expected 0 restarts for %q", p.Name)
		}
	}
}

func TestParseK8sPods_NotReady(t *testing.T) {
	raw := mustMarshalPodList(t, []k8sPodItem{
		makePod("fusca-api-app-1", "def456", false, 3, "2026-02-23T10:00:00Z", ""),
	})

	pods, _, err := parseK8sPods(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pods[0].Ready {
		t.Error("pod should not be ready")
	}
	if pods[0].Restarts != 3 {
		t.Errorf("expected 3 restarts, got %d", pods[0].Restarts)
	}
	if pods[0].LastRestart == "" {
		t.Error("expected LastRestart to be set")
	}
}

func TestParseK8sPods_EmptyList(t *testing.T) {
	raw := mustMarshalPodList(t, []k8sPodItem{})
	pods, hash, err := parseK8sPods(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pods) != 0 {
		t.Errorf("expected empty pods, got %d", len(pods))
	}
	if hash != "" {
		t.Errorf("expected empty hash, got %q", hash)
	}
}

func TestParseK8sPods_InvalidJSON(t *testing.T) {
	_, _, err := parseK8sPods([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseK8sPods_AgeComputed(t *testing.T) {
	// Pod created 2 hours ago
	created := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	item := k8sPodItem{
		Metadata: k8sPodMeta{
			Name:              "test-pod",
			CreationTimestamp: created,
			Labels:            map[string]string{"pod-template-hash": "xyz"},
		},
		Status: k8sPodStatus{
			Phase:      "Running",
			Conditions: []k8sCondition{{Type: "Ready", Status: "True"}},
		},
	}
	raw := mustMarshalPodList(t, []k8sPodItem{item})
	pods, _, err := parseK8sPods(raw)
	if err != nil {
		t.Fatal(err)
	}
	if pods[0].Age == "" {
		t.Error("expected non-empty age")
	}
	if !strings.Contains(pods[0].Age, "h") {
		t.Errorf("expected hours in age for 2h-old pod, got %q", pods[0].Age)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// evaluateK8sHealth
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestEvaluateK8sHealth_AllHealthy(t *testing.T) {
	pods := []podInfo{
		{Name: "pod-1", Ready: true, Restarts: 0, Status: "Running"},
		{Name: "pod-2", Ready: true, Restarts: 0, Status: "Running"},
	}
	cfg := K8sConfig{KubectlContext: "cobli-prod", Namespace: "fusca", Deployment: "fusca-api-app"}
	r := evaluateK8sHealth(cfg, pods, "abc123", "app=fusca-api-app")

	if r.Status != "ok" {
		t.Errorf("expected ok, got %q (signal: %s)", r.Status, r.Signal)
	}
	if !strings.Contains(r.Signal, "saudável") {
		t.Errorf("expected 'saudável' in signal: %q", r.Signal)
	}
}

func TestEvaluateK8sHealth_PodNotReady(t *testing.T) {
	pods := []podInfo{
		{Name: "pod-1", Ready: true, Restarts: 0, Status: "Running"},
		{Name: "pod-2", Ready: false, Restarts: 5, Status: "CrashLoopBackOff"},
	}
	cfg := K8sConfig{KubectlContext: "cobli-prod", Namespace: "fusca", Deployment: "fusca-api-app"}
	r := evaluateK8sHealth(cfg, pods, "abc123", "app=fusca-api-app")

	if r.Status != "critical" {
		t.Errorf("expected critical when pod not ready, got %q", r.Status)
	}
	if !strings.Contains(r.Signal, "CRÍTICO") {
		t.Errorf("expected CRÍTICO in signal: %q", r.Signal)
	}
	if r.Data["ready"] != 1 {
		t.Errorf("expected ready=1, got %v", r.Data["ready"])
	}
	if r.Data["total"] != 2 {
		t.Errorf("expected total=2, got %v", r.Data["total"])
	}
}

func TestEvaluateK8sHealth_RecentRestartWarn(t *testing.T) {
	// 3 pods with restart in the last 5 minutes → warn (threshold = 2)
	recentRestart := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
	pods := []podInfo{
		{Name: "pod-1", Ready: true, Restarts: 1, LastRestart: recentRestart},
		{Name: "pod-2", Ready: true, Restarts: 1, LastRestart: recentRestart},
		{Name: "pod-3", Ready: true, Restarts: 1, LastRestart: recentRestart},
	}
	cfg := K8sConfig{KubectlContext: "cobli-prod", Namespace: "fusca", Deployment: "fusca-api-app"}
	r := evaluateK8sHealth(cfg, pods, "abc123", "app=fusca-api-app")

	if r.Status != "warn" {
		t.Errorf("expected warn for >%d pods with recent restarts, got %q", warnRestartPodCount, r.Status)
	}
}

func TestEvaluateK8sHealth_OldRestartNoWarn(t *testing.T) {
	// Restarts are old — should not warn
	oldRestart := time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339)
	pods := []podInfo{
		{Name: "pod-1", Ready: true, Restarts: 5, LastRestart: oldRestart},
		{Name: "pod-2", Ready: true, Restarts: 5, LastRestart: oldRestart},
		{Name: "pod-3", Ready: true, Restarts: 5, LastRestart: oldRestart},
	}
	cfg := K8sConfig{KubectlContext: "cobli-prod", Namespace: "fusca", Deployment: "fusca-api-app"}
	r := evaluateK8sHealth(cfg, pods, "abc123", "app=fusca-api-app")

	if r.Status != "ok" {
		t.Errorf("old restarts should not trigger warn, got %q (signal: %s)", r.Status, r.Signal)
	}
}

func TestEvaluateK8sHealth_HashMismatchWarn(t *testing.T) {
	pods := []podInfo{
		{Name: "pod-1", Ready: true, Restarts: 0},
	}
	cfg := K8sConfig{
		KubectlContext: "cobli-prod",
		Namespace:      "fusca",
		Deployment:     "fusca-api-app",
		ExpectedHash:   "expectedhash",
	}
	r := evaluateK8sHealth(cfg, pods, "actualhash", "app=fusca-api-app")

	if r.Status != "warn" {
		t.Errorf("expected warn for hash mismatch, got %q", r.Status)
	}
	if !strings.Contains(r.Signal, "atenção") {
		t.Errorf("expected 'atenção' in signal: %q", r.Signal)
	}
}

func TestEvaluateK8sHealth_HashMatchNoWarn(t *testing.T) {
	pods := []podInfo{
		{Name: "pod-1", Ready: true, Restarts: 0},
	}
	cfg := K8sConfig{
		KubectlContext: "cobli-prod",
		Namespace:      "fusca",
		Deployment:     "fusca-api-app",
		ExpectedHash:   "samehash",
	}
	r := evaluateK8sHealth(cfg, pods, "samehash", "app=fusca-api-app")

	if r.Status != "ok" {
		t.Errorf("matching hash should not warn, got %q", r.Status)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CheckK8sStatus — no-selector error path (no kubectl needed)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckK8sStatus_NoSelectorError(t *testing.T) {
	cfg := K8sConfig{KubectlContext: "cobli-prod", Namespace: "fusca"}
	r := CheckK8sStatus(cfg)
	if r.Status != "error" {
		t.Errorf("expected error when no selector given, got %q", r.Status)
	}
	if r.Cost != "zero-llm" {
		t.Errorf("expected zero-llm cost")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// buildK8sSignal
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBuildK8sSignal_ContainsNamespace(t *testing.T) {
	cfg := K8sConfig{KubectlContext: "cobli-prod", Namespace: "fusca", Deployment: "fusca-api-app"}
	sig := buildK8sSignal(cfg, 2, 2, "abc123", 0, "ok", nil, nil)
	if !strings.Contains(sig, "fusca") {
		t.Errorf("signal missing namespace: %q", sig)
	}
	if !strings.Contains(sig, "2/2") {
		t.Errorf("signal missing ready/total: %q", sig)
	}
	if !strings.Contains(sig, "saudável") {
		t.Errorf("expected 'saudável' in ok signal: %q", sig)
	}
}

func TestBuildK8sSignal_Critical(t *testing.T) {
	cfg := K8sConfig{KubectlContext: "cobli-prod", Namespace: "fusca", Deployment: "fusca-api-app"}
	sig := buildK8sSignal(cfg, 1, 2, "", 0, "critical", []string{"1/2 pods not ready"}, nil)
	if !strings.Contains(sig, "CRÍTICO") {
		t.Errorf("expected CRÍTICO in critical signal: %q", sig)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// helpers
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func makePod(name, hash string, ready bool, restarts int64, lastRestart, createdAt string) k8sPodItem {
	conditions := []k8sCondition{}
	if ready {
		conditions = append(conditions, k8sCondition{Type: "Ready", Status: "True"})
	}

	var containerStatuses []k8sContainerStatus
	cs := k8sContainerStatus{RestartCount: restarts, Ready: ready}
	if lastRestart != "" {
		cs.LastState = k8sLastState{
			Terminated: &k8sTerminated{FinishedAt: lastRestart},
		}
	}
	containerStatuses = append(containerStatuses, cs)

	if createdAt == "" {
		createdAt = time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	}

	return k8sPodItem{
		Metadata: k8sPodMeta{
			Name:              name,
			CreationTimestamp: createdAt,
			Labels:            map[string]string{"pod-template-hash": hash},
		},
		Status: k8sPodStatus{
			Phase:             "Running",
			Conditions:        conditions,
			ContainerStatuses: containerStatuses,
		},
	}
}

func mustMarshalPodList(t *testing.T, items []k8sPodItem) []byte {
	t.Helper()
	data, err := json.Marshal(k8sPodList{Items: items})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	return data
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CheckK8sEvents
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckK8sEvents_MissingConfig(t *testing.T) {
	r := CheckK8sEvents(K8sEventsConfig{})
	if r.Status != "error" {
		t.Errorf("expected error for missing config, got %q", r.Status)
	}
	if r.Cost != "zero-llm" {
		t.Errorf("expected zero-llm cost")
	}
}

func TestCheckK8sEvents_EvictionWithinWindow_Warn(t *testing.T) {
	recent := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"reason": "TaintManagerEviction", "message": "pod evicted", "count": 1, "lastTimestamp": recent, "type": "Warning"},
		},
	})
	cfg := K8sEventsConfig{KubectlContext: "cobli-prod", Namespace: "data-platform", WindowMin: 30}
	r := parseAndEvalK8sEvents(raw, cfg)
	if r.Status != "warn" {
		t.Errorf("expected warn, got %q: %s", r.Status, r.Signal)
	}
	if r.Data["eviction_event_count"] != 1 {
		t.Errorf("expected eviction_event_count=1, got %v", r.Data["eviction_event_count"])
	}
}

func TestCheckK8sEvents_NormalEventsOnly_OK(t *testing.T) {
	recent := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"reason": "Pulled", "message": "image pulled", "count": 1, "lastTimestamp": recent, "type": "Normal"},
			{"reason": "Started", "message": "container started", "count": 1, "lastTimestamp": recent, "type": "Normal"},
		},
	})
	cfg := K8sEventsConfig{KubectlContext: "cobli-prod", Namespace: "data-platform", WindowMin: 30}
	r := parseAndEvalK8sEvents(raw, cfg)
	if r.Status != "ok" {
		t.Errorf("expected ok (only normal events), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["eviction_event_count"] != 0 {
		t.Errorf("expected eviction_event_count=0, got %v", r.Data["eviction_event_count"])
	}
}

func TestCheckK8sEvents_EventsOutsideWindow_OK(t *testing.T) {
	old := time.Now().Add(-120 * time.Minute).UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"reason": "FailedScheduling", "message": "no nodes available", "count": 3, "lastTimestamp": old, "type": "Warning"},
		},
	})
	cfg := K8sEventsConfig{KubectlContext: "cobli-prod", Namespace: "data-platform", WindowMin: 30}
	r := parseAndEvalK8sEvents(raw, cfg)
	if r.Status != "ok" {
		t.Errorf("expected ok (events outside 30min window), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["eviction_event_count"] != 0 {
		t.Errorf("expected eviction_event_count=0 (filtered), got %v", r.Data["eviction_event_count"])
	}
}
