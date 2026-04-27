package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	criticalLockCount   = 5
	warnDeadTuplePct    = 10.0
	criticalWALLagBytes = int64(10 * 1024 * 1024 * 1024) // 10 GB
	warnWALLagBytes     = int64(1 * 1024 * 1024 * 1024)  // 1 GB

	dbHealthQuery = `SELECT row_to_json(h) FROM (SELECT ` +
		`(SELECT count(*)::int FROM pg_stat_activity WHERE wait_event_type='Lock') AS locks,` +
		`(SELECT count(*)::int FROM pg_stat_activity WHERE state='active' AND xact_start<now()-interval'5 minutes') AS long_tx,` +
		`(SELECT count(*)::int FROM pg_stat_activity WHERE state='idle in transaction') AS idle_in_tx,` +
		`(SELECT count(*)::int FROM pg_stat_activity) AS total_conn,` +
		`(SELECT count(*)::int FROM pg_stat_activity WHERE state='active') AS active_conn,` +
		`pg_database_size(current_database()) AS db_size_bytes,` +
		`(SELECT coalesce(max(pg_wal_lsn_diff(pg_current_wal_lsn(),restart_lsn)),0)::bigint FROM pg_replication_slots) AS wal_lag_bytes,` +
		`(SELECT json_agg(t) FROM (` +
		`SELECT relname AS name,n_live_tup AS live_tuples,n_dead_tup AS dead_tuples,` +
		`pg_size_pretty(pg_total_relation_size(relid)) AS size,` +
		`to_char(last_autovacuum,'YYYY-MM-DD HH24:MI') AS last_autovacuum ` +
		`FROM pg_stat_user_tables WHERE n_dead_tup>1000 ORDER BY n_dead_tup DESC LIMIT 10` +
		`) t) AS tables` +
		`) h`
)

// DBHealthConfig holds connection parameters for the health check.
type DBHealthConfig struct {
	KubectlContext string
	Namespace      string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string // direct password (takes priority)
	DBPasswordSSM  string // SSM parameter path (fallback)
	AWSProfile     string // AWS profile for SSM fetch
	DBName         string
}

type dbMetrics struct {
	Locks       int64          `json:"locks"`
	LongTX      int64          `json:"long_tx"`
	IdleInTX    int64          `json:"idle_in_tx"`
	TotalConn   int64          `json:"total_conn"`
	ActiveConn  int64          `json:"active_conn"`
	DBSizeBytes int64          `json:"db_size_bytes"`
	WALLagBytes int64          `json:"wal_lag_bytes"`
	Tables      []tableMetrics `json:"tables"`
}

type tableMetrics struct {
	Name           string `json:"name"`
	LiveTuples     int64  `json:"live_tuples"`
	DeadTuples     int64  `json:"dead_tuples"`
	Size           string `json:"size"`
	LastAutovacuum string `json:"last_autovacuum"`
}

// CheckDBHealth connects to PostgreSQL via kubectl and returns a structured health signal.
func CheckDBHealth(cfg DBHealthConfig) OpsResult {
	password, err := resolvePassword(cfg)
	if err != nil {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("não foi possível obter senha do banco: %v", err),
			Data:    map[string]any{"host": cfg.DBHost},
			Actions: []string{authScriptNote},
			Cost:    "zero-llm",
		}
	}

	m, err := fetchDBMetrics(cfg, password)
	if err != nil {
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("falha ao conectar ao banco '%s': %v", cfg.DBHost, err),
			Data:    map[string]any{"host": cfg.DBHost},
			Actions: []string{"verificar conectividade kubectl e credenciais"},
			Cost:    "zero-llm",
		}
	}

	return evaluateDBHealth(cfg.DBHost, m)
}

// resolvePassword returns the DB password, trying direct flag then SSM.
func resolvePassword(cfg DBHealthConfig) (string, error) {
	if cfg.DBPassword != "" {
		return cfg.DBPassword, nil
	}
	if cfg.DBPasswordSSM == "" {
		return "", fmt.Errorf("informe --db-password ou --db-password-ssm")
	}
	return fetchSSMPassword(cfg.DBPasswordSSM, cfg.AWSProfile)
}

// fetchSSMPassword retrieves a parameter from AWS SSM.
func fetchSSMPassword(ssmPath, awsProfile string) (string, error) {
	args := []string{
		"ssm", "get-parameter",
		"--profile", awsProfile,
		"--region", "us-east-1",
		"--name", ssmPath,
		"--with-decryption",
		"--query", "Parameter.Value",
		"--output", "text",
	}
	out, err := shellOutput("aws", args...)
	if err != nil {
		return "", fmt.Errorf("SSM fetch falhou (token expirado? execute: ~/Cobliteam/dev-setup/aws-login.sh): %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// fetchDBMetrics runs a psql query via kubectl ephemeral pod and parses the JSON result.
// It attempts to use an ephemeral K8s Secret so PGPASSWORD is not exposed in the pod spec
// (which would make it visible in kubectl describe / K8s audit logs).
// Falls back to the direct --env approach if Secret creation fails (e.g. insufficient RBAC).
func fetchDBMetrics(cfg DBHealthConfig, password string) (dbMetrics, error) {
	podName := fmt.Sprintf("wtb-db-health-%d", time.Now().Unix()%10000)

	// Try ephemeral Secret approach first (F002).
	useSecret := false
	secretName := podName
	secretCtx, secretCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := createEphemeralSecret(secretCtx, cfg, secretName, password); err == nil {
		useSecret = true
		defer deleteEphemeralSecret(cfg, secretName)
	}
	secretCancel()

	baseArgs := []string{
		"run", podName,
		"--rm", "-i", "--tty=false",
		"-n", cfg.Namespace,
		"--context", cfg.KubectlContext,
		"--image=postgres:16-alpine",
		"--restart=Never",
		"--env=PGHOST=" + cfg.DBHost,
		"--env=PGPORT=" + cfg.DBPort,
		"--env=PGUSER=" + cfg.DBUser,
		"--env=PGDATABASE=" + cfg.DBName,
	}

	var args []string
	if useSecret {
		overrides, err := podSecretOverrides(podName, secretName)
		if err != nil {
			return dbMetrics{}, fmt.Errorf("failed to build pod overrides: %w", err)
		}
		// PGPASSWORD comes from the Secret via envFrom — not in pod spec.
		args = append(baseArgs, "--overrides", overrides, "--", "psql", "-tA", "-c", dbHealthQuery)
	} else {
		// Fallback: password in env var (visible in pod spec).
		args = append(baseArgs, "--env=PGPASSWORD="+password, "--", "psql", "-tA", "-c", dbHealthQuery)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	out, err := shellExec(ctx, "kubectl", args...)
	if err != nil {
		// Surface kubectl/psql error without leaking password.
		safeOut := strings.ReplaceAll(string(out), password, "***")
		return dbMetrics{}, fmt.Errorf("%v — %s", err, strings.TrimSpace(safeOut))
	}

	jsonStr := extractFirstJSON(string(out))
	if jsonStr == "" {
		return dbMetrics{}, fmt.Errorf("nenhum JSON encontrado no output do psql")
	}

	var m dbMetrics
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return dbMetrics{}, fmt.Errorf("falha ao parsear métricas: %v", err)
	}

	return m, nil
}

// createEphemeralSecret creates a K8s Secret containing PGPASSWORD for the probe pod.
// Avoids passing the password as a pod env var (exposed in kubectl describe / audit logs).
func createEphemeralSecret(ctx context.Context, cfg DBHealthConfig, secretName, password string) error {
	args := []string{
		"create", "secret", "generic", secretName,
		"-n", cfg.Namespace,
		"--context", cfg.KubectlContext,
		"--from-literal=PGPASSWORD=" + password,
	}
	_, err := shellExec(ctx, "kubectl", args...)
	return err
}

// deleteEphemeralSecret removes the probe Secret. Best-effort: errors are silently ignored
// to avoid masking the probe result.
func deleteEphemeralSecret(cfg DBHealthConfig, secretName string) {
	args := []string{
		"delete", "secret", secretName,
		"-n", cfg.Namespace,
		"--context", cfg.KubectlContext,
		"--ignore-not-found",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shellExec(ctx, "kubectl", args...) //nolint:errcheck — best-effort cleanup
}

// podSecretOverrides returns the --overrides JSON for kubectl run that adds envFrom
// referencing the ephemeral Secret. PGPASSWORD is read from the Secret, not --env.
func podSecretOverrides(podName, secretName string) (string, error) {
	type nameRef struct {
		Name string `json:"name"`
	}
	type secretRef struct {
		SecretRef nameRef `json:"secretRef"`
	}
	type containerOverride struct {
		Name    string      `json:"name"`
		EnvFrom []secretRef `json:"envFrom"`
	}
	type specOverride struct {
		Containers []containerOverride `json:"containers"`
	}
	type podOverride struct {
		Spec specOverride `json:"spec"`
	}
	ov := podOverride{
		Spec: specOverride{
			Containers: []containerOverride{{
				Name:    podName,
				EnvFrom: []secretRef{{SecretRef: nameRef{Name: secretName}}},
			}},
		},
	}
	b, err := json.Marshal(ov)
	return string(b), err
}

// extractFirstJSON finds the first line in output that is a valid JSON object.
func extractFirstJSON(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			var check map[string]any
			if json.Unmarshal([]byte(line), &check) == nil {
				return line
			}
		}
	}
	return ""
}

// evaluateDBHealth applies heuristics and returns the OpsResult.
func evaluateDBHealth(host string, m dbMetrics) OpsResult {
	var criticals, warnings, actions []string

	// Heuristic: locks
	if m.Locks > int64(criticalLockCount) {
		criticals = append(criticals, fmt.Sprintf("%d lock waits (threshold: %d)", m.Locks, criticalLockCount))
		actions = append(actions,
			"identificar root blocker: SELECT pid,query,wait_event FROM pg_stat_activity WHERE wait_event_type='Lock' ORDER BY query_start",
			"considerar: SELECT pg_terminate_backend(<root_pid>)",
		)
	}

	// Heuristic: long transactions
	if m.LongTX > 0 {
		warnings = append(warnings, fmt.Sprintf("%d transação(ões) ativa(s) > 5 min", m.LongTX))
		actions = append(actions,
			"investigar: SELECT pid,now()-xact_start AS duration,left(query,80) FROM pg_stat_activity WHERE state='active' AND xact_start<now()-interval'5 minutes'",
		)
	}

	// Heuristic: dead tuples
	for _, t := range m.Tables {
		if t.LiveTuples > 0 {
			pct := float64(t.DeadTuples) / float64(t.LiveTuples) * 100
			if pct > warnDeadTuplePct {
				warnings = append(warnings, fmt.Sprintf("'%s': %.1f%% dead tuples (%d)", t.Name, pct, t.DeadTuples))
				actions = append(actions, fmt.Sprintf("executar: VACUUM ANALYZE %s", t.Name))
			}
		}
	}

	// Heuristic: WAL lag
	walLag := humanBytes(m.WALLagBytes)
	if m.WALLagBytes > criticalWALLagBytes {
		criticals = append(criticals, fmt.Sprintf("WAL lag %s (threshold: 10 GB)", walLag))
	} else if m.WALLagBytes > warnWALLagBytes {
		warnings = append(warnings, fmt.Sprintf("WAL lag %s (threshold: 1 GB)", walLag))
	}

	// Determine status
	status := "ok"
	if len(criticals) > 0 {
		status = "critical"
	} else if len(warnings) > 0 {
		status = "warn"
	}

	// Build signal
	signal := buildDBSignal(host, status, m, criticals, warnings)

	// Build data
	data := map[string]any{
		"host":          host,
		"locks":         m.Locks,
		"long_tx":       m.LongTX,
		"idle_in_tx":    m.IdleInTX,
		"total_conn":    m.TotalConn,
		"active_conn":   m.ActiveConn,
		"db_size":       humanBytes(m.DBSizeBytes),
		"db_size_bytes": m.DBSizeBytes,
		"wal_lag":       walLag,
		"wal_lag_bytes": m.WALLagBytes,
	}
	if len(m.Tables) > 0 {
		data["tables"] = m.Tables
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

func buildDBSignal(host, status string, m dbMetrics, criticals, warnings []string) string {
	parts := []string{
		fmt.Sprintf("%d locks", m.Locks),
		fmt.Sprintf("%d tx longas", m.LongTX),
		fmt.Sprintf("%d/%d conn", m.ActiveConn, m.TotalConn),
	}
	if m.WALLagBytes > 0 {
		parts = append(parts, fmt.Sprintf("WAL lag %s", humanBytes(m.WALLagBytes)))
	}
	base := fmt.Sprintf("%s — %s", host, strings.Join(parts, ", "))

	switch status {
	case "critical":
		return base + " — CRÍTICO: " + strings.Join(criticals, "; ")
	case "warn":
		return base + " — atenção: " + strings.Join(warnings, "; ")
	default:
		return base + " — saudável"
	}
}

func humanBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/gb)
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/mb)
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/kb)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
