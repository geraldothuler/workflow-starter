package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	warnRestartWindowMin = 10 // minutes
	warnRestartPodCount  = 2  // pods with recent restart before warn
)

// K8sConfig holds parameters for the k8s status check.
type K8sConfig struct {
	KubectlContext string
	Namespace      string
	Deployment     string // deployment name → derives selector "app=<name>"
	LabelSelector  string // explicit label selector (alternative to Deployment)
	ExpectedHash   string // optional — activates deploy hash validation
}

type podInfo struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Ready       bool   `json:"ready"`
	Restarts    int64  `json:"restarts"`
	Age         string `json:"age"`
	LastRestart string `json:"last_restart,omitempty"`
}

// kubectl JSON types — minimal structs for what we need.

type k8sPodList struct {
	Items []k8sPodItem `json:"items"`
}

type k8sPodItem struct {
	Metadata k8sPodMeta   `json:"metadata"`
	Status   k8sPodStatus `json:"status"`
}

type k8sPodMeta struct {
	Name              string            `json:"name"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
}

type k8sPodStatus struct {
	Phase             string               `json:"phase"`
	Conditions        []k8sCondition       `json:"conditions"`
	ContainerStatuses []k8sContainerStatus `json:"containerStatuses"`
}

type k8sCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type k8sContainerStatus struct {
	RestartCount int64        `json:"restartCount"`
	Ready        bool         `json:"ready"`
	LastState    k8sLastState `json:"lastState"`
}

type k8sLastState struct {
	Terminated *k8sTerminated `json:"terminated"`
}

type k8sTerminated struct {
	FinishedAt string `json:"finishedAt"`
}

// CheckK8sStatus checks pod health for a deployment or label selector.
func CheckK8sStatus(cfg K8sConfig) OpsResult {
	selector := cfg.LabelSelector
	if selector == "" && cfg.Deployment != "" {
		selector = "app=" + cfg.Deployment
	}
	if selector == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "informe --deployment ou --label para identificar os pods",
			Data:    map[string]any{},
			Actions: []string{"wtb ops k8s status --deployment <nome> --context <ctx> --namespace <ns>"},
			Cost:    "zero-llm",
		}
	}

	raw, err := fetchK8sPods(cfg, selector)
	if err != nil {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("falha ao consultar pods (selector '%s'): %v", selector, err),
			Data:    map[string]any{"selector": selector, "namespace": cfg.Namespace, "context": cfg.KubectlContext},
			Actions: []string{"verificar conectividade kubectl e contexto: " + cfg.KubectlContext},
			Cost:    "zero-llm",
		}
	}

	pods, hash, err := parseK8sPods(raw)
	if err != nil {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("falha ao parsear resposta kubectl: %v", err),
			Data:    map[string]any{"selector": selector},
			Actions: []string{},
			Cost:    "zero-llm",
		}
	}

	if len(pods) == 0 {
		label := cfg.Deployment
		if label == "" {
			label = selector
		}
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("nenhum pod encontrado para '%s' em %s/%s",
				label, cfg.KubectlContext, cfg.Namespace),
			Data: map[string]any{
				"selector":  selector,
				"namespace": cfg.Namespace,
				"context":   cfg.KubectlContext,
			},
			Actions: []string{
				"verificar namespace e selector — use --label para selector customizado",
				fmt.Sprintf("listar todos: kubectl get pods -n %s --context %s", cfg.Namespace, cfg.KubectlContext),
			},
			Cost: "zero-llm",
		}
	}

	return evaluateK8sHealth(cfg, pods, hash, selector)
}

// fetchK8sPods runs kubectl get pods -o json for the given selector.
func fetchK8sPods(cfg K8sConfig, selector string) ([]byte, error) {
	args := []string{
		"get", "pods",
		"-n", cfg.Namespace,
		"--context", cfg.KubectlContext,
		"-l", selector,
		"-o", "json",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := shellExec(ctx, "kubectl", args...)
	if err != nil {
		return nil, fmt.Errorf("%v — %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// parseK8sPods parses kubectl JSON into []podInfo and extracts the pod-template-hash.
func parseK8sPods(raw []byte) ([]podInfo, string, error) {
	var list k8sPodList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, "", fmt.Errorf("JSON inválido: %v", err)
	}

	now := time.Now()
	var pods []podInfo
	var hash string

	for _, item := range list.Items {
		// Extract pod-template-hash from the first pod that has it.
		if hash == "" {
			if h, ok := item.Metadata.Labels["pod-template-hash"]; ok {
				hash = h
			}
		}

		// Ready condition.
		ready := false
		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				ready = true
				break
			}
		}

		// Sum restarts across all containers; track latest restart time.
		var totalRestarts int64
		var lastRestartTime time.Time
		for _, cs := range item.Status.ContainerStatuses {
			totalRestarts += cs.RestartCount
			if cs.LastState.Terminated != nil && cs.LastState.Terminated.FinishedAt != "" {
				t, err := time.Parse(time.RFC3339, cs.LastState.Terminated.FinishedAt)
				if err == nil && t.After(lastRestartTime) {
					lastRestartTime = t
				}
			}
		}

		// Compute age.
		age := ""
		if item.Metadata.CreationTimestamp != "" {
			created, err := time.Parse(time.RFC3339, item.Metadata.CreationTimestamp)
			if err == nil {
				age = humanDuration(now.Sub(created))
			}
		}

		p := podInfo{
			Name:     item.Metadata.Name,
			Status:   item.Status.Phase,
			Ready:    ready,
			Restarts: totalRestarts,
			Age:      age,
		}
		if !lastRestartTime.IsZero() {
			p.LastRestart = lastRestartTime.UTC().Format(time.RFC3339)
		}
		pods = append(pods, p)
	}

	return pods, hash, nil
}

// evaluateK8sHealth applies heuristics and builds the OpsResult.
func evaluateK8sHealth(cfg K8sConfig, pods []podInfo, hash, selector string) OpsResult {
	var criticals, warnings, actions []string

	total := len(pods)
	readyCount := 0
	now := time.Now()
	recentRestartPods := 0 // pods that had a restart within warnRestartWindowMin

	for _, p := range pods {
		if p.Ready {
			readyCount++
		}
		if p.LastRestart != "" {
			t, err := time.Parse(time.RFC3339, p.LastRestart)
			if err == nil && now.Sub(t) < time.Duration(warnRestartWindowMin)*time.Minute {
				recentRestartPods++
			}
		}
	}

	// Heuristic: pods not ready.
	if readyCount < total {
		notReady := total - readyCount
		criticals = append(criticals,
			fmt.Sprintf("%d/%d pods not ready", notReady, total))
		actions = append(actions,
			fmt.Sprintf("descrever pods: kubectl describe pods -n %s --context %s -l %s",
				cfg.Namespace, cfg.KubectlContext, selector),
			fmt.Sprintf("ver eventos: kubectl get events -n %s --context %s --sort-by='.lastTimestamp' | tail -20",
				cfg.Namespace, cfg.KubectlContext),
		)
	}

	// Heuristic: recent restarts.
	if recentRestartPods > warnRestartPodCount {
		warnings = append(warnings,
			fmt.Sprintf("%d pods com restart nos últimos %d min", recentRestartPods, warnRestartWindowMin))
		actions = append(actions,
			fmt.Sprintf("ver logs anteriores: kubectl logs -n %s --context %s <pod> --previous",
				cfg.Namespace, cfg.KubectlContext),
		)
	}

	// Heuristic: deploy hash mismatch.
	if cfg.ExpectedHash != "" && hash != "" && cfg.ExpectedHash != hash {
		warnings = append(warnings,
			fmt.Sprintf("hash difere: esperado '%s', atual '%s' — possível deploy não-intencional",
				cfg.ExpectedHash, hash))
		name := cfg.Deployment
		if name == "" {
			name = "<deployment>"
		}
		actions = append(actions,
			fmt.Sprintf("histórico: kubectl rollout history deployment/%s -n %s --context %s",
				name, cfg.Namespace, cfg.KubectlContext),
		)
	}

	// Determine status.
	status := "ok"
	if len(criticals) > 0 {
		status = "critical"
	} else if len(warnings) > 0 {
		status = "warn"
	}

	signal := buildK8sSignal(cfg, readyCount, total, hash, recentRestartPods, status, criticals, warnings)

	data := map[string]any{
		"context":   cfg.KubectlContext,
		"namespace": cfg.Namespace,
		"selector":  selector,
		"ready":     readyCount,
		"total":     total,
		"hash":      hash,
		"pods":      pods,
	}
	if cfg.ExpectedHash != "" {
		data["expected_hash"] = cfg.ExpectedHash
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

func buildK8sSignal(cfg K8sConfig, ready, total int, hash string, recentRestarts int, status string, criticals, warnings []string) string {
	parts := []string{
		fmt.Sprintf("%d/%d pods Ready", ready, total),
	}
	if hash != "" {
		parts = append(parts, "hash "+hash)
	}
	if recentRestarts > 0 {
		parts = append(parts, fmt.Sprintf("%d restarts recentes (<%dmin)", recentRestarts, warnRestartWindowMin))
	} else {
		parts = append(parts, "0 restarts recentes")
	}

	label := cfg.Deployment
	if label == "" {
		label = cfg.LabelSelector
	}
	base := fmt.Sprintf("%s/%s — %s", cfg.Namespace, label, strings.Join(parts, ", "))

	switch status {
	case "critical":
		return base + " — CRÍTICO: " + strings.Join(criticals, "; ")
	case "warn":
		return base + " — atenção: " + strings.Join(warnings, "; ")
	default:
		return base + " — saudável"
	}
}

// K8sEventsConfig holds parameters for the k8s events check.
type K8sEventsConfig struct {
	KubectlContext string
	Namespace      string
	WindowMin      int // look-back window; default 30
}

// K8sEventEntry describes one K8s event matching infra instability patterns.
type K8sEventEntry struct {
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	Count         int    `json:"count"`
	LastTimestamp string `json:"last_timestamp"`
	Type          string `json:"type"`
}

// evictionReasons is the set of event reasons that indicate infra instability.
var evictionReasons = map[string]bool{
	"FailedScheduling":     true,
	"TaintManagerEviction": true,
	"Evicted":              true,
}

// CheckK8sEvents fetches namespace events and returns infra instability signals.
func CheckK8sEvents(cfg K8sEventsConfig) OpsResult {
	if cfg.Namespace == "" || cfg.KubectlContext == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "k8s-events: missing namespace or kubectl context",
			Actions: []string{"set --namespace and --context"},
			Cost:    "zero-llm",
		}
	}
	if cfg.WindowMin <= 0 {
		cfg.WindowMin = 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	raw, err := shellExec(ctx, "kubectl", "get", "events",
		"-n", cfg.Namespace,
		"--context", cfg.KubectlContext,
		"--sort-by=.lastTimestamp",
		"-o", "json",
	)
	if err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("k8s-events: kubectl error: %v — %s", err, strings.TrimSpace(string(raw))),
			Cost:   "zero-llm",
		}
	}

	return parseAndEvalK8sEvents(raw, cfg)
}

func parseAndEvalK8sEvents(raw []byte, cfg K8sEventsConfig) OpsResult {
	var eventList struct {
		Items []struct {
			Reason        string `json:"reason"`
			Message       string `json:"message"`
			Count         int    `json:"count"`
			LastTimestamp string `json:"lastTimestamp"`
			Type          string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &eventList); err != nil {
		return OpsResult{
			Status: "error",
			Signal: fmt.Sprintf("k8s-events: failed to parse events: %v", err),
			Cost:   "zero-llm",
		}
	}

	cutoff := time.Now().Add(-time.Duration(cfg.WindowMin) * time.Minute)
	var matchingEvents []K8sEventEntry

	for _, item := range eventList.Items {
		if !evictionReasons[item.Reason] {
			continue
		}
		if item.LastTimestamp != "" {
			t, err := time.Parse(time.RFC3339, item.LastTimestamp)
			if err == nil && t.Before(cutoff) {
				continue // outside window
			}
		}
		matchingEvents = append(matchingEvents, K8sEventEntry{
			Reason:        item.Reason,
			Message:       item.Message,
			Count:         item.Count,
			LastTimestamp: item.LastTimestamp,
			Type:          item.Type,
		})
	}

	data := map[string]any{
		"eviction_event_count": len(matchingEvents),
		"window_min":           cfg.WindowMin,
		"namespace":            cfg.Namespace,
		"context":              cfg.KubectlContext,
		"events":               matchingEvents,
	}

	status, hSignal, actions := EvalHeuristics(data, loadHeuristics("k8s-events"))
	var signal string
	if status == "ok" {
		signal = fmt.Sprintf("k8s-events: no eviction/scheduling events in %s (last %dmin)", cfg.Namespace, cfg.WindowMin)
	} else {
		signal = fmt.Sprintf("k8s-events %s/%s: %s", cfg.KubectlContext, cfg.Namespace, hSignal)
	}

	return OpsResult{
		Status:  status,
		Signal:  signal,
		Data:    data,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// humanDuration formats a time.Duration as a human-readable age string (e.g. "23h", "4d").
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
