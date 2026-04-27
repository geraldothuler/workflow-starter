package dbops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/secret"
	"gopkg.in/yaml.v3"
)

// Ensure yaml is used (referenced in readSessionYML)
var _ = yaml.Unmarshal

// Run executes a named query for a repo and returns the result envelope.
// Credentials are resolved via VPN-first strategy: cache → probe → pod bootstrap.
func Run(queriesDir, repo, queryName string, params map[string]string) (*QueryResult, error) {
	rq, err := LoadQueries(queriesDir, repo)
	if err != nil {
		return nil, err
	}

	nq := findQuery(rq, queryName)
	if nq == nil {
		// Fallback: look in _schema.yml using repo's credentials
		if sq, err2 := LoadQueries(queriesDir, "_schema"); err2 == nil {
			if snq := findQuery(sq, queryName); snq != nil {
				nq = snq
				// Use repo's namespace/context for credential resolution
			}
		}
	}
	if nq == nil {
		return nil, fmt.Errorf("query %q not found in %s or _schema (available: %s)",
			queryName, repo, listQueryNames(rq))
	}

	sql := applyParams(nq.SQL, nq.Params, params)
	sql = injectPagination(sql, nq.Driver, params)
	return execQuery(rq, repo, nq, sql)
}

// RunSQL executes an ad-hoc SQL string against a repo's database.
// The driver must be specified explicitly ("postgres", "cassandra", "snowflake").
// Pagination is automatically injected if the query has no LIMIT clause.
func RunSQL(queriesDir, repo, driver, sql string, params map[string]string) (*QueryResult, error) {
	rq, err := LoadQueries(queriesDir, repo)
	if err != nil {
		rq = &RepoQueries{Repo: repo}
	}
	sql = injectPagination(sql, driver, params)
	nq := &NamedQuery{Name: "ad-hoc", Desc: "ad-hoc SQL", Driver: driver, SQL: sql}
	return execQuery(rq, repo, nq, sql)
}

// LoadQueries loads the YAML query catalog for a repo.
func LoadQueries(queriesDir, repo string) (*RepoQueries, error) {
	path := filepath.Join(queriesDir, repo+".yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no query file for %s (looked at %s)", repo, path)
	}
	var rq RepoQueries
	if err := yaml.Unmarshal(data, &rq); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &rq, nil
}

// ListQueryNames returns all query names defined for a repo.
func ListQueryNames(queriesDir, repo string) ([]NamedQuery, error) {
	rq, err := LoadQueries(queriesDir, repo)
	if err != nil {
		return nil, err
	}
	return rq.Queries, nil
}

// ── internal ──────────────────────────────────────────────────────────────────

func execQuery(rq *RepoQueries, repo string, nq *NamedQuery, query string) (*QueryResult, error) {
	start := time.Now()

	kubectlCtx := rq.Context
	if kubectlCtx == "" {
		kubectlCtx = defaultKubectlContext
	}

	var rows []map[string]any
	var err error

	switch nq.Driver {
	case "postgres":
		creds, cerr := resolvePostgresCreds(rq, repo, kubectlCtx)
		if cerr != nil {
			return nil, cerr
		}
		rows, err = QueryPostgres(creds, query)

	case "cassandra", "scylla":
		creds, cerr := resolveCassandraCreds(rq, repo, kubectlCtx)
		if cerr != nil {
			return nil, cerr
		}
		rows, err = QueryScylla(creds, query)

	case "snowflake":
		account := snowflakeConfig("snowflake-account", "SNOWFLAKE_ACCOUNT", "gaa90132.us-east-1")
		user := snowflakeConfig("snowflake-user", "SNOWFLAKE_USER", "")
		role := snowflakeConfig("snowflake-role", "SNOWFLAKE_ROLE", "PLATFORM_ANALYSIS_ROLE_DPV2")
		wh := snowflakeConfig("snowflake-warehouse", "SNOWFLAKE_WAREHOUSE", "DATA_PLATFORM_ANALYSIS")
		rows, err = QuerySnowflake(account, user, role, wh, "", "", query)

	default:
		return nil, fmt.Errorf("unknown driver %q (supported: postgres, cassandra, snowflake)", nq.Driver)
	}

	if err != nil {
		return nil, err
	}

	cols := nq.Columns
	if len(cols) == 0 && len(rows) > 0 {
		for k := range rows[0] {
			cols = append(cols, ColumnMeta{Name: k})
		}
	}

	return &QueryResult{
		Repo:      repo,
		Query:     nq.Name,
		Driver:    nq.Driver,
		Columns:   cols,
		Rows:      rows,
		Count:     len(rows),
		ElapsedMs: time.Since(start).Milliseconds(),
	}, nil
}

func resolvePostgresCreds(rq *RepoQueries, repo, kubectlCtx string) (*DBCredentials, error) {
	if cached := LoadCached(repo, "postgres"); cached != nil && ProbeVPN(cached.Host, cached.Port) {
		return cached, nil
	}
	creds, err := BootstrapCredentials(repo, rq.Namespace, "postgres", kubectlCtx)
	if err != nil {
		return nil, fmt.Errorf("could not resolve postgres credentials for %s: %w\n"+
			"  → make sure VPN is connected and a pod is running in namespace %s",
			repo, err, rq.Namespace)
	}
	if !ProbeVPN(creds.Host, creds.Port) {
		return nil, fmt.Errorf("postgres host %s:%s not reachable — connect VPN first",
			creds.Host, creds.Port)
	}
	SaveCredentials(repo, "postgres", creds) //nolint:errcheck
	return creds, nil
}

func resolveCassandraCreds(rq *RepoQueries, repo, kubectlCtx string) (*DBCredentials, error) {
	// Prefer jupyter-read from Keychain (universal read access to all keyspaces)
	if creds := jupyterReadCreds(); creds != nil && ProbeVPN(creds.ContactPoints, "9042") {
		return creds, nil
	}
	// Fallback: cached service credentials
	if cached := LoadCached(repo, "cassandra"); cached != nil {
		return cached, nil
	}
	// Fallback: bootstrap from pod
	creds, err := BootstrapCredentials(repo, rq.Namespace, "cassandra", kubectlCtx)
	if err != nil {
		return nil, fmt.Errorf("could not resolve cassandra credentials for %s: %w\n"+
			"  → hint: bash ~/workflow/scripts/secret-set.sh workflow-scylla-jupyter-read '<pass>'",
			repo, err)
	}
	if !ProbeVPN(creds.ContactPoints, "9042") {
		return nil, fmt.Errorf("cassandra host %s:9042 not reachable — connect VPN first",
			creds.ContactPoints)
	}
	SaveCredentials(repo, "cassandra", creds) //nolint:errcheck
	return creds, nil
}

// jupyterReadCreds reads the jupyter-read Scylla credentials from Keychain.
func jupyterReadCreds() *DBCredentials {
	pass, err := keychainLookup("workflow-scylla-jupyter-read")
	if err != nil || pass == "" {
		return nil
	}
	return &DBCredentials{
		ContactPoints: "herbie-database.prod.aws.cobli.co",
		User:          "jupyter-reader",
		Password:      pass,
		Datacenter:    "AWS_US_EAST_1",
	}
}

func keychainLookup(service string) (string, error) {
	return secret.Get(service)
}

func applyParams(sql string, defs []ParamDef, provided map[string]string) string {
	for _, p := range defs {
		val, ok := provided[p.Name]
		if !ok {
			val = p.Default
		}
		sql = strings.ReplaceAll(sql, ":"+p.Name, val)
	}
	return sql
}

func findQuery(rq *RepoQueries, name string) *NamedQuery {
	for i := range rq.Queries {
		if rq.Queries[i].Name == name {
			return &rq.Queries[i]
		}
	}
	return nil
}

func listQueryNames(rq *RepoQueries) string {
	names := make([]string, len(rq.Queries))
	for i, q := range rq.Queries {
		names[i] = q.Name
	}
	return strings.Join(names, ", ")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// snowflakeConfig resolves a Snowflake config value: env var → session.yml → fallback.
func snowflakeConfig(sessionKey, envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if v := readSessionYML(sessionKey); v != "" {
		return v
	}
	return fallback
}

// readSessionYML reads a flat key from ~/.workflow/session.yml.
func readSessionYML(key string) string {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".workflow", "session.yml"))
	if err != nil {
		// Also try ~/workflow/session.yml
		data, err = os.ReadFile(filepath.Join(home, "workflow", "session.yml"))
		if err != nil {
			return ""
		}
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return ""
	}
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// injectPagination adds LIMIT (and OFFSET for postgres) to queries without one.
// Default page size: 200 rows. Override with --param page_size=N --param offset=N.
// CQL (Cassandra/Scylla) does not support OFFSET — only LIMIT is injected.
func injectPagination(sql, driver string, params map[string]string) string {
	upper := strings.ToUpper(sql)
	if strings.Contains(upper, "LIMIT") {
		return sql
	}
	// Write statements never get pagination injected.
	firstWord := strings.Fields(upper)
	if len(firstWord) > 0 {
		switch firstWord[0] {
		case "INSERT", "UPDATE", "DELETE", "TRUNCATE", "CREATE", "DROP", "ALTER":
			return sql
		}
	}
	if isScalarQuery(upper) {
		return sql
	}

	pageSize := "200"
	if v, ok := params["page_size"]; ok && v != "" {
		pageSize = v
	}

	trimmed := strings.TrimRight(strings.TrimSpace(sql), ";")

	switch driver {
	case "cassandra", "scylla":
		// CQL: LIMIT only, no OFFSET
		return trimmed + "\nLIMIT " + pageSize
	default:
		// PostgreSQL, Snowflake: LIMIT + OFFSET
		offset := "0"
		if v, ok := params["offset"]; ok && v != "" {
			offset = v
		}
		return trimmed + "\nLIMIT " + pageSize + " OFFSET " + offset
	}
}

// isScalarQuery returns true for queries that return a single value (COUNT, MAX, etc.)
// where pagination would break semantics.
func isScalarQuery(upperSQL string) bool {
	// Queries with GROUP BY are not scalar even if they use aggregates
	if strings.Contains(upperSQL, "GROUP BY") {
		return false
	}
	// System catalog queries (information_schema, pg_stat, system_schema) — safe to paginate
	if strings.Contains(upperSQL, "INFORMATION_SCHEMA") ||
		strings.Contains(upperSQL, "PG_STAT") ||
		strings.Contains(upperSQL, "SYSTEM_SCHEMA") {
		return false
	}
	// Pure scalar: SELECT COUNT(*), SELECT MAX(...), SELECT SUM(...) with no subquery
	scalarFns := []string{"COUNT(*)", "COUNT(1)", "MAX(", "MIN(", "SUM(", "AVG("}
	selectIdx := strings.Index(upperSQL, "SELECT")
	fromIdx := strings.Index(upperSQL, "FROM")
	if selectIdx < 0 || fromIdx < 0 {
		return false
	}
	selectClause := upperSQL[selectIdx+6 : fromIdx]
	// If select clause contains only one expression and it's a scalar aggregate
	parts := strings.Split(strings.TrimSpace(selectClause), ",")
	if len(parts) != 1 {
		return false
	}
	for _, fn := range scalarFns {
		if strings.Contains(parts[0], fn) {
			return true
		}
	}
	return false
}
