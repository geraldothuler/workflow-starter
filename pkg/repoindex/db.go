package repoindex

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
)

const dbFile = "repos.duckdb"

// DB wraps the DuckDB connection for the repo index.
type DB struct {
	sql      *sql.DB
	repoRoot string
}

// Open opens (or creates) repos.duckdb in repoRoot, applies the schema, and returns a DB.
func Open(repoRoot string) (*DB, error) {
	path := filepath.Join(repoRoot, dbFile)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	sqldb, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, err
	}
	// DuckDB enforces single-writer locking at the OS file level — no need to
	// artificially limit the sql pool. Limiting to 1 caused nested-query deadlocks
	// (GetSnapshot iterates handlers and concurrently queries model_fields).
	if err := execSchema(sqldb); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{sql: sqldb, repoRoot: repoRoot}, nil
}

// execSchema executes each DDL statement individually (DuckDB requires single-statement Exec).
func execSchema(db *sql.DB) error {
	for _, stmt := range splitStatements(schema) {
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("stmt %q: %w", truncate(stmt, 60), err)
		}
	}
	return nil
}

func splitStatements(s string) []string {
	var stmts []string
	for _, part := range strings.Split(s, ";") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			stmts = append(stmts, trimmed)
		}
	}
	return stmts
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// OpenReadOnly opens repos.duckdb in read-only mode.
// Multiple processes may hold read-only connections simultaneously.
// Returns an error if the file does not exist.
func OpenReadOnly(repoRoot string) (*DB, error) {
	path := filepath.Join(repoRoot, dbFile)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("repos.duckdb not found in %s: run 'wtb repo index' first", repoRoot)
	}
	sqldb, err := sql.Open("duckdb", path+"?access_mode=read_only")
	if err != nil {
		return nil, err
	}
	return &DB{sql: sqldb, repoRoot: repoRoot}, nil
}

// Close closes the underlying connection.
func (db *DB) Close() error { return db.sql.Close() }

// Raw returns the underlying *sql.DB for direct queries (used by wtb repo query).
func (db *DB) Raw() *sql.DB { return db.sql }

const schema = `
CREATE TABLE IF NOT EXISTS repos (
	id             TEXT PRIMARY KEY,
	name           TEXT NOT NULL UNIQUE,
	path           TEXT NOT NULL,
	lang           TEXT NOT NULL DEFAULT '',
	framework      TEXT NOT NULL DEFAULT '',
	owner          TEXT NOT NULL DEFAULT '',
	last_indexed_at TEXT NOT NULL DEFAULT ''
);

ALTER TABLE repos ADD COLUMN IF NOT EXISTS owner TEXT;
ALTER TABLE repos ADD COLUMN IF NOT EXISTS secondary_lang TEXT;
ALTER TABLE repos ADD COLUMN IF NOT EXISTS secondary_framework TEXT;
ALTER TABLE repos ADD COLUMN IF NOT EXISTS dd_service_name TEXT;
ALTER TABLE repos ADD COLUMN IF NOT EXISTS primary_hostname TEXT;
ALTER TABLE repos ADD COLUMN IF NOT EXISTS ci_platform TEXT;

CREATE TABLE IF NOT EXISTS file_hashes (
	repo_id    TEXT NOT NULL,
	path       TEXT NOT NULL,
	sha256     TEXT NOT NULL,
	indexed_at TEXT NOT NULL,
	PRIMARY KEY (repo_id, path)
);

CREATE TABLE IF NOT EXISTS handlers (
	id             TEXT PRIMARY KEY,
	repo_id        TEXT NOT NULL,
	name           TEXT NOT NULL,
	handler_file   TEXT NOT NULL DEFAULT '',
	trigger_type   TEXT NOT NULL DEFAULT '',
	trigger_detail TEXT NOT NULL DEFAULT '',
	timeout        INTEGER NOT NULL DEFAULT 0,
	max_retry      INTEGER NOT NULL DEFAULT 0,
	concurrency    INTEGER NOT NULL DEFAULT 0,
	vpc            INTEGER NOT NULL DEFAULT 0,
	description    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS events (
	id          TEXT PRIMARY KEY,
	repo_id     TEXT NOT NULL,
	name        TEXT NOT NULL,
	event_type  TEXT NOT NULL DEFAULT '',
	detail_type TEXT NOT NULL DEFAULT '',
	bus_name    TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS models (
	id         TEXT PRIMARY KEY,
	repo_id    TEXT NOT NULL,
	name       TEXT NOT NULL,
	table_name TEXT NOT NULL DEFAULT '',
	dialect    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS model_fields (
	id          TEXT PRIMARY KEY,
	model_id    TEXT NOT NULL,
	name        TEXT NOT NULL,
	type        TEXT NOT NULL DEFAULT '',
	nullable    INTEGER NOT NULL DEFAULT 1,
	primary_key INTEGER NOT NULL DEFAULT 0,
	unique_field INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS model_associations (
	id           TEXT PRIMARY KEY,
	model_id     TEXT NOT NULL,
	assoc_type   TEXT NOT NULL DEFAULT '',
	target_model TEXT NOT NULL DEFAULT '',
	foreign_key  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS external_apis (
	id          TEXT PRIMARY KEY,
	repo_id     TEXT NOT NULL,
	name        TEXT NOT NULL,
	url         TEXT NOT NULL DEFAULT '',
	method      TEXT NOT NULL DEFAULT '',
	auth_type   TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS db_connections (
	id        TEXT PRIMARY KEY,
	repo_id   TEXT NOT NULL,
	dialect   TEXT NOT NULL DEFAULT '',
	host_var  TEXT NOT NULL DEFAULT '',
	pool_min  INTEGER NOT NULL DEFAULT 0,
	pool_max  INTEGER NOT NULL DEFAULT 0,
	pool_idle INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS config_vars (
	id          TEXT PRIMARY KEY,
	repo_id     TEXT NOT NULL,
	key         TEXT NOT NULL,
	source      TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS notes (
	id          TEXT PRIMARY KEY,
	entity_type TEXT NOT NULL,
	entity_id   TEXT NOT NULL,
	repo_id     TEXT NOT NULL,
	content     TEXT NOT NULL,
	created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_handlers_repo    ON handlers(repo_id);
CREATE INDEX IF NOT EXISTS idx_events_repo      ON events(repo_id);
CREATE INDEX IF NOT EXISTS idx_models_repo      ON models(repo_id);
CREATE INDEX IF NOT EXISTS idx_ext_apis_repo    ON external_apis(repo_id);
CREATE INDEX IF NOT EXISTS idx_db_conn_repo     ON db_connections(repo_id);
CREATE INDEX IF NOT EXISTS idx_config_vars_repo ON config_vars(repo_id);
CREATE INDEX IF NOT EXISTS idx_notes_entity     ON notes(entity_type, entity_id);

CREATE TABLE IF NOT EXISTS embeddings (
	id          TEXT PRIMARY KEY,
	entity_type TEXT NOT NULL,
	entity_id   TEXT NOT NULL,
	repo_id     TEXT NOT NULL,
	model       TEXT NOT NULL DEFAULT '',
	vector      BLOB NOT NULL,
	created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_embeddings_repo   ON embeddings(repo_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_entity ON embeddings(entity_type, entity_id);

CREATE TABLE IF NOT EXISTS schema_deps (
	repo_name   TEXT NOT NULL,
	model_name  TEXT NOT NULL,
	match_count INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (repo_name, model_name)
);

CREATE INDEX IF NOT EXISTS idx_schema_deps_repo  ON schema_deps(repo_name);
CREATE INDEX IF NOT EXISTS idx_schema_deps_model ON schema_deps(model_name);

ALTER TABLE repos ADD COLUMN IF NOT EXISTS dd_service_name TEXT;
ALTER TABLE repos ADD COLUMN IF NOT EXISTS primary_hostname TEXT;

CREATE TABLE IF NOT EXISTS deployment_units (
    id              TEXT PRIMARY KEY,
    repo_id         TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    namespace       TEXT NOT NULL DEFAULT '',
    replicas_min    INTEGER NOT NULL DEFAULT 0,
    replicas_max    INTEGER NOT NULL DEFAULT 0,
    consumer_group  TEXT NOT NULL DEFAULT '',
    deprecated      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS topic_enrichments (
    id              TEXT PRIMARY KEY,
    repo_id         TEXT NOT NULL,
    deployment_unit TEXT NOT NULL DEFAULT '',
    topic           TEXT NOT NULL,
    direction       TEXT NOT NULL DEFAULT '',
    serialization   TEXT NOT NULL DEFAULT '',
    consumer_group  TEXT NOT NULL DEFAULT '',
    key_description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS dd_monitors (
    id          TEXT PRIMARY KEY,
    repo_id     TEXT NOT NULL,
    monitor_id  TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT '',
    url         TEXT NOT NULL DEFAULT '',
    fetched_at  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_deployment_units_repo  ON deployment_units(repo_id);
CREATE INDEX IF NOT EXISTS idx_topic_enrich_repo      ON topic_enrichments(repo_id);
CREATE INDEX IF NOT EXISTS idx_dd_monitors_repo       ON dd_monitors(repo_id);

CREATE TABLE IF NOT EXISTS chart_snapshots (
    id           TEXT PRIMARY KEY,
    repo_id      TEXT NOT NULL,
    env          TEXT NOT NULL DEFAULT 'prod',
    image_tag    TEXT NOT NULL DEFAULT '',
    app_version  TEXT NOT NULL DEFAULT '',
    captured_at  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS chart_sidecars (
    id           TEXT PRIMARY KEY,
    snapshot_id  TEXT NOT NULL,
    repo_id      TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    image        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS chart_resources (
    id            TEXT PRIMARY KEY,
    snapshot_id   TEXT NOT NULL,
    repo_id       TEXT NOT NULL,
    container     TEXT NOT NULL DEFAULT '',
    cpu_request   TEXT NOT NULL DEFAULT '',
    cpu_limit     TEXT NOT NULL DEFAULT '',
    mem_request   TEXT NOT NULL DEFAULT '',
    mem_limit     TEXT NOT NULL DEFAULT '',
    heap_size     TEXT NOT NULL DEFAULT '',
    replicas_min  INTEGER NOT NULL DEFAULT 0,
    replicas_max  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chart_env_vars (
    id           TEXT PRIMARY KEY,
    snapshot_id  TEXT NOT NULL,
    repo_id      TEXT NOT NULL,
    key          TEXT NOT NULL DEFAULT '',
    value        TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_chart_snapshots_repo ON chart_snapshots(repo_id);

ALTER TABLE chart_snapshots ADD COLUMN IF NOT EXISTS kube_context TEXT;
ALTER TABLE chart_snapshots ADD COLUMN IF NOT EXISTS namespace TEXT;
CREATE INDEX IF NOT EXISTS idx_chart_sidecars_snap  ON chart_sidecars(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_chart_resources_snap ON chart_resources(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_chart_env_vars_snap  ON chart_env_vars(snapshot_id);

CREATE TABLE IF NOT EXISTS service_metrics (
    id          TEXT PRIMARY KEY,
    repo_id     TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT '',
    fetched_at  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_service_metrics_repo      ON service_metrics(repo_id);
CREATE INDEX IF NOT EXISTS idx_service_metrics_category  ON service_metrics(repo_id, category);

CREATE TABLE IF NOT EXISTS service_routes (
    id          TEXT PRIMARY KEY,
    repo_id     TEXT NOT NULL,
    prefix      TEXT NOT NULL,
    method      TEXT NOT NULL DEFAULT '',
    captured_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS service_route_aliases (
    id       TEXT PRIMARY KEY,
    route_id TEXT NOT NULL,
    alias    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS public_endpoints (
    id           TEXT PRIMARY KEY,
    path         TEXT NOT NULL,
    method       TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    summary      TEXT NOT NULL DEFAULT '',
    auth_type    TEXT NOT NULL DEFAULT '',
    captured_at  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_service_routes_repo        ON service_routes(repo_id);
CREATE INDEX IF NOT EXISTS idx_service_route_aliases_rid  ON service_route_aliases(route_id);
CREATE INDEX IF NOT EXISTS idx_service_route_aliases_ali  ON service_route_aliases(alias);
CREATE INDEX IF NOT EXISTS idx_public_endpoints_path      ON public_endpoints(path);
`
