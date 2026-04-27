package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// test helpers — shell injection
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func withShellExec(t *testing.T, fn func(ctx context.Context, name string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := shellExec
	t.Cleanup(func() { shellExec = orig })
	shellExec = fn
}

func withShellOutput(t *testing.T, fn func(name string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := shellOutput
	t.Cleanup(func() { shellOutput = orig })
	shellOutput = fn
}

// dbMetricsJSON builds a psql-like JSON output line for DB health integration tests.
func dbMetricsJSON(m dbMetrics) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CheckK8sStatus integration (mocked kubectl)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckK8sStatus_Integration_Healthy(t *testing.T) {
	pods := mustMarshalPodList(t, []k8sPodItem{
		makePod("api-abc-1", "abc", true, 0, "", ""),
		makePod("api-abc-2", "abc", true, 0, "", ""),
		makePod("api-abc-3", "abc", true, 0, "", ""),
	})
	withShellExec(t, func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "kubectl" {
			return pods, nil
		}
		return nil, fmt.Errorf("unexpected command: %s", name)
	})

	r := CheckK8sStatus(K8sConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		Deployment:     "api",
	})

	if r.Status != "ok" {
		t.Errorf("want status ok, got %q (signal: %s)", r.Status, r.Signal)
	}
	if r.Data["ready"] != 3 {
		t.Errorf("want ready=3, got %v", r.Data["ready"])
	}
	if r.Data["total"] != 3 {
		t.Errorf("want total=3, got %v", r.Data["total"])
	}
}

func TestCheckK8sStatus_Integration_PodNotReady(t *testing.T) {
	pods := mustMarshalPodList(t, []k8sPodItem{
		makePod("api-abc-1", "abc", true, 0, "", ""),
		makePod("api-abc-2", "abc", false, 5, "2026-02-23T10:00:00Z", ""),
		makePod("api-abc-3", "abc", true, 0, "", ""),
	})
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return pods, nil
	})

	r := CheckK8sStatus(K8sConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		Deployment:     "api",
	})

	if r.Status != "critical" {
		t.Errorf("want critical, got %q (signal: %s)", r.Status, r.Signal)
	}
	if r.Data["ready"] != 2 {
		t.Errorf("want ready=2, got %v", r.Data["ready"])
	}
}

func TestCheckK8sStatus_Integration_KubectlFailure(t *testing.T) {
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("error: context deadline exceeded"), fmt.Errorf("exit status 1")
	})

	r := CheckK8sStatus(K8sConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		Deployment:     "api",
	})

	if r.Status != "error" {
		t.Errorf("want error, got %q (signal: %s)", r.Status, r.Signal)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CheckDBHealth integration (mocked kubectl + aws)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckDBHealth_Integration_Healthy(t *testing.T) {
	metrics := dbMetricsJSON(dbMetrics{
		Locks:       0,
		LongTX:      0,
		IdleInTX:    2,
		TotalConn:   20,
		ActiveConn:  5,
		DBSizeBytes: 512 * 1024 * 1024,
		WALLagBytes: 0,
	})
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		// kubectl run ... -- psql → returns JSON metrics preceded by pod lifecycle messages
		return []byte("pod/wtb-db-health-1234 created\n" + metrics + "\npod deleted\n"), nil
	})

	r := CheckDBHealth(DBHealthConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		DBHost:         "pg.internal",
		DBUser:         "app",
		DBPassword:     "secret",
		DBName:         "postgres",
		DBPort:         "5432",
	})

	if r.Status != "ok" {
		t.Errorf("want ok, got %q (signal: %s)", r.Status, r.Signal)
	}
	if !strings.Contains(r.Signal, "saudável") {
		t.Errorf("want 'saudável' in signal, got %q", r.Signal)
	}
}

func TestCheckDBHealth_Integration_CriticalLocks(t *testing.T) {
	metrics := dbMetricsJSON(dbMetrics{
		Locks:       10,
		LongTX:      0,
		IdleInTX:    0,
		TotalConn:   50,
		ActiveConn:  15,
		DBSizeBytes: 1024 * 1024 * 1024,
		WALLagBytes: 0,
	})
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(metrics + "\n"), nil
	})

	r := CheckDBHealth(DBHealthConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		DBHost:         "pg.internal",
		DBUser:         "app",
		DBPassword:     "secret",
		DBName:         "postgres",
		DBPort:         "5432",
	})

	if r.Status != "critical" {
		t.Errorf("want critical, got %q (signal: %s)", r.Status, r.Signal)
	}
	if !strings.Contains(r.Signal, "lock waits") {
		t.Errorf("want 'lock waits' in signal, got %q", r.Signal)
	}
}

func TestCheckDBHealth_Integration_KubectlFailure(t *testing.T) {
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("error: no auth"), fmt.Errorf("exit status 1")
	})

	r := CheckDBHealth(DBHealthConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		DBHost:         "pg.internal",
		DBUser:         "app",
		DBPassword:     "secret",
		DBName:         "postgres",
		DBPort:         "5432",
	})

	if r.Status != "error" {
		t.Errorf("want error, got %q (signal: %s)", r.Status, r.Signal)
	}
}

func TestCheckDBHealth_Integration_SSMPassword(t *testing.T) {
	metrics := dbMetricsJSON(dbMetrics{
		Locks:       0,
		LongTX:      0,
		IdleInTX:    1,
		TotalConn:   10,
		ActiveConn:  2,
		DBSizeBytes: 256 * 1024 * 1024,
		WALLagBytes: 0,
	})
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(metrics + "\n"), nil
	})
	withShellOutput(t, func(name string, args ...string) ([]byte, error) {
		if name == "aws" {
			return []byte("ssm-secret-password\n"), nil
		}
		return nil, fmt.Errorf("unexpected command: %s", name)
	})

	r := CheckDBHealth(DBHealthConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		DBHost:         "pg.internal",
		DBUser:         "app",
		DBPasswordSSM:  "/prod/db/password",
		AWSProfile:     "cobli-tech",
		DBName:         "postgres",
		DBPort:         "5432",
	})

	if r.Status != "ok" {
		t.Errorf("want ok via SSM password, got %q (signal: %s)", r.Status, r.Signal)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CheckKafkaStatus integration — logs source (mocked kubectl)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckKafkaStatus_Integration_HealthyLogs(t *testing.T) {
	logs := strings.Join([]string{
		"2026-02-23T10:00:01 INFO Processing message batch completed",
		"2026-02-23T10:00:02 INFO Consumed records: lag=0",
		"2026-02-23T10:00:03 INFO Heartbeat sent successfully",
	}, "\n")
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(logs), nil
	})

	r := CheckKafkaStatus(KafkaConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		Deployment:     "cdc",
		Window:         "10m",
		Source:         "logs",
	})

	if r.Status != "ok" {
		t.Errorf("want ok, got %q (signal: %s)", r.Status, r.Signal)
	}
}

func TestCheckKafkaStatus_Integration_EpochErrors(t *testing.T) {
	logs := strings.Join([]string{
		"2026-02-23T10:00:01 ERROR InvalidProducerEpochException: Producer attempted to produce with an old epoch",
		"2026-02-23T10:00:02 ERROR ProducerFencedException: Producer was fenced out",
		"2026-02-23T10:00:03 ERROR InvalidProducerEpochException: epoch conflict detected",
	}, "\n")
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(logs), nil
	})

	r := CheckKafkaStatus(KafkaConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		Deployment:     "cdc",
		Window:         "10m",
		Source:         "logs",
	})

	if r.Status != "critical" {
		t.Errorf("want critical for epoch errors, got %q (signal: %s)", r.Status, r.Signal)
	}
	errors := r.Data["errors"].(map[string]any)
	if errors["epoch_errors"] != int64(3) {
		t.Errorf("want 3 epoch errors, got %v", errors["epoch_errors"])
	}
}

func TestCheckKafkaStatus_Integration_KubectlFailure(t *testing.T) {
	withShellExec(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("unable to retrieve container logs"), fmt.Errorf("exit status 1")
	})

	r := CheckKafkaStatus(KafkaConfig{
		KubectlContext: "test-ctx",
		Namespace:      "test-ns",
		Deployment:     "cdc",
		Source:         "logs",
	})

	if r.Status != "error" {
		t.Errorf("want error, got %q (signal: %s)", r.Status, r.Signal)
	}
}
