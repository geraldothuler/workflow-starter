package dbops

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultKubectlContext = "cobli-prod"

// ProbeVPN returns true if host:port is reachable within 3 seconds.
// Used to decide whether to go direct (VPN) or fall back to pod.
func ProbeVPN(host, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// credsFile returns the path to the local DB credentials cache.
// Lives in ~/.workflow/db-creds.yml (gitignored alongside session.yml).
func credsFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".workflow", "db-creds.yml")
}

// LoadCached reads cached credentials for repo+driver from ~/.workflow/db-creds.yml.
// Returns nil if not found.
func LoadCached(repo, driver string) *DBCredentials {
	data, err := os.ReadFile(credsFile())
	if err != nil {
		return nil
	}
	var root map[string]map[string]map[string]string
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil
	}
	drivers, ok := root[repo]
	if !ok {
		return nil
	}
	fields, ok := drivers[driver]
	if !ok {
		return nil
	}
	return fieldsToCredentials(fields)
}

// SaveCredentials writes credentials for repo+driver to ~/.workflow/db-creds.yml.
func SaveCredentials(repo, driver string, creds *DBCredentials) error {
	// Load existing content
	var root map[string]map[string]map[string]string
	if data, err := os.ReadFile(credsFile()); err == nil {
		yaml.Unmarshal(data, &root) //nolint:errcheck
	}
	if root == nil {
		root = map[string]map[string]map[string]string{}
	}
	if root[repo] == nil {
		root[repo] = map[string]map[string]string{}
	}
	root[repo][driver] = credentialsToFields(creds)

	data, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(credsFile()), 0700); err != nil {
		return err
	}
	return os.WriteFile(credsFile(), data, 0600)
}

// BootstrapCredentials discovers credentials by exec-ing into a running pod
// for the given repo. It parses the pod's env vars and extracts DB connection info.
func BootstrapCredentials(repo, namespace, driver, kubectlContext string) (*DBCredentials, error) {
	if kubectlContext == "" {
		kubectlContext = defaultKubectlContext
	}

	pod, err := findRunningPod(repo, namespace, kubectlContext)
	if err != nil {
		return nil, fmt.Errorf("no running pod for %s in %s: %w", repo, namespace, err)
	}

	envVars, err := podEnv(pod, namespace, kubectlContext)
	if err != nil {
		return nil, fmt.Errorf("kubectl exec failed: %w", err)
	}

	switch driver {
	case "postgres":
		return parsePostgresCreds(envVars)
	case "cassandra", "scylla":
		return parseCassandraCreds(envVars)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

// ResolveCredentials returns credentials using VPN-first strategy:
//  1. Check cache (session-local, in-memory via LoadCached)
//  2. If not cached, bootstrap from pod and cache
//
// Callers should check VPN connectivity separately before calling drivers.
func ResolveCredentials(repo, namespace, driver, kubectlContext string) (*DBCredentials, error) {
	if cached := LoadCached(repo, driver); cached != nil {
		return cached, nil
	}
	creds, err := BootstrapCredentials(repo, namespace, driver, kubectlContext)
	if err != nil {
		return nil, err
	}
	if err := SaveCredentials(repo, driver, creds); err != nil {
		// Non-fatal — we still have the creds for this call
		fmt.Fprintf(os.Stderr, "warn: could not cache credentials: %v\n", err)
	}
	return creds, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

var reJDBCURL = regexp.MustCompile(`jdbc:postgresql://([^:/]+):(\d+)/(\S+)`)

func findRunningPod(repo, namespace, kubectlContext string) (string, error) {
	out, err := exec.Command("kubectl", "get", "pods",
		"-n", namespace,
		"--context", kubectlContext,
		"--no-headers").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(strings.ToLower(line), strings.ToLower(repo)) {
			continue
		}
		if !strings.Contains(line, "Running") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no Running pod matching %q", repo)
}

func podEnv(pod, namespace, kubectlContext string) (map[string]string, error) {
	out, err := exec.Command("kubectl", "exec",
		"-n", namespace,
		"--context", kubectlContext,
		pod, "--", "env").Output()
	if err != nil {
		return nil, err
	}
	envVars := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		envVars[line[:idx]] = line[idx+1:]
	}
	return envVars, nil
}

func parsePostgresCreds(env map[string]string) (*DBCredentials, error) {
	// Try known env var patterns in priority order
	urlVars := []string{
		"SPRING_DATASOURCE_URL",
		"ANALYTICS_REPORT_DATASOURCE_URL",
		"FLEET-POLICY_DATASOURCE_URL",
		"DATABASE_URL",
	}
	var jdbcURL string
	for _, v := range urlVars {
		if val, ok := env[v]; ok && strings.HasPrefix(val, "jdbc:postgresql://") {
			jdbcURL = val
			break
		}
	}
	if jdbcURL == "" {
		return nil, fmt.Errorf("no jdbc:postgresql:// URL found in pod env")
	}

	m := reJDBCURL.FindStringSubmatch(jdbcURL)
	if m == nil {
		return nil, fmt.Errorf("could not parse JDBC URL: %s", jdbcURL)
	}

	creds := &DBCredentials{
		Host:     m[1],
		Port:     m[2],
		Database: m[3],
	}

	// User
	for _, k := range []string{"SPRING_DATASOURCE_USERNAME", "DB_USER", "POSTGRES_USER"} {
		if v, ok := env[k]; ok {
			creds.User = v
			break
		}
	}
	// Password
	for _, k := range []string{"SPRING_DATASOURCE_PASSWORD", "DB_PASSWORD", "POSTGRES_PASSWORD"} {
		if v, ok := env[k]; ok {
			creds.Password = v
			break
		}
	}

	return creds, nil
}

func parseCassandraCreds(env map[string]string) (*DBCredentials, error) {
	creds := &DBCredentials{}

	for _, k := range []string{"SPRING_DATA_CASSANDRA_CONTACTPOINTS", "SPRING_CASSANDRA_CONTACTPOINTS", "CASSANDRA_CLUSTER_CONTACT_POINTS", "HERBIE_DATABASE_CONTACT_POINTS"} {
		if v, ok := env[k]; ok {
			creds.ContactPoints = v
			break
		}
	}
	if creds.ContactPoints == "" {
		return nil, fmt.Errorf("no Cassandra contact points found in pod env")
	}

	for _, k := range []string{"SPRING_DATA_CASSANDRA_KEYSPACENAME", "CASSANDRA_KEYSPACE"} {
		if v, ok := env[k]; ok {
			creds.Keyspace = v
			break
		}
	}
	for _, k := range []string{"SPRING_DATA_CASSANDRA_USERNAME", "CASSANDRA_USERNAME"} {
		if v, ok := env[k]; ok {
			creds.User = v
			break
		}
	}
	for _, k := range []string{"SPRING_DATA_CASSANDRA_PASSWORD", "CASSANDRA_PASSWORD"} {
		if v, ok := env[k]; ok {
			creds.Password = v
			break
		}
	}
	for _, k := range []string{"SPRING_DATA_CASSANDRA_LOCALDATACENTER"} {
		if v, ok := env[k]; ok {
			creds.Datacenter = v
			break
		}
	}

	return creds, nil
}

func fieldsToCredentials(f map[string]string) *DBCredentials {
	return &DBCredentials{
		Host:          f["host"],
		Port:          f["port"],
		Database:      f["database"],
		User:          f["user"],
		Password:      f["password"],
		ContactPoints: f["contact_points"],
		Keyspace:      f["keyspace"],
		Datacenter:    f["datacenter"],
	}
}

func credentialsToFields(c *DBCredentials) map[string]string {
	return map[string]string{
		"host":           c.Host,
		"port":           c.Port,
		"database":       c.Database,
		"user":           c.User,
		"password":       c.Password,
		"contact_points": c.ContactPoints,
		"keyspace":       c.Keyspace,
		"datacenter":     c.Datacenter,
	}
}
