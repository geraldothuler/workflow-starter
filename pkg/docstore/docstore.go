// Package docstore manages workflow artefacts (discovery, savepoint, runbook,
// 1on1, postmortem, incident) in a local SQLite database.
//
//	docs.db  — source of truth (gitignored)
//
// Query via `wtb doc list/search/get`. Soft-delete preserves history.
package docstore

import (
	"database/sql"
	_ "embed"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
	"gopkg.in/yaml.v3"
)

//go:embed doc-types.yml
var defaultDocTypesYML []byte

const dbFile = "docs.db"

// DB wraps the SQLite connection and the repo root path.
type DB struct {
	sql      *sql.DB
	repoRoot string
	// Types holds the valid document types, loaded from doc-types.yml.
	Types []string
}

// loadDocTypes loads valid types from {repoRoot}/doc-types.yml if present,
// otherwise uses the embedded default.
func loadDocTypes(repoRoot string) []string {
	data, err := os.ReadFile(filepath.Join(repoRoot, "doc-types.yml"))
	if err != nil {
		data = defaultDocTypesYML
	}
	var cfg struct {
		Types []string `yaml:"types"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil || len(cfg.Types) == 0 {
		_ = yaml.Unmarshal(defaultDocTypesYML, &cfg)
	}
	return cfg.Types
}

// IsValidType reports whether t is one of the loaded valid types.
func (db *DB) IsValidType(t string) bool {
	for _, v := range db.Types {
		if v == t {
			return true
		}
	}
	return false
}

// Open opens (or creates) docs.db in repoRoot, applies the schema, and returns a DB.
func Open(repoRoot string) (*DB, error) {
	path := filepath.Join(repoRoot, dbFile)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := sqldb.Exec(schema); err != nil {
		sqldb.Close()
		return nil, err
	}
	return &DB{sql: sqldb, repoRoot: repoRoot, Types: loadDocTypes(repoRoot)}, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.sql.Close()
}

const schema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS documents (
	id         TEXT PRIMARY KEY,
	type       TEXT NOT NULL,
	title      TEXT NOT NULL,
	doc_date   TEXT NOT NULL DEFAULT '',
	repo       TEXT NOT NULL DEFAULT '',
	tags       TEXT NOT NULL DEFAULT '',
	content    TEXT NOT NULL DEFAULT '',
	deleted_at TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_documents_type     ON documents(type);
CREATE INDEX IF NOT EXISTS idx_documents_doc_date ON documents(doc_date);
CREATE INDEX IF NOT EXISTS idx_documents_repo     ON documents(repo);

CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
	id, title, content, tags,
	content=documents,
	content_rowid=rowid
);

CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
	INSERT INTO documents_fts(rowid, id, title, content, tags)
	VALUES (new.rowid, new.id, new.title, new.content, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
	INSERT INTO documents_fts(documents_fts, rowid, id, title, content, tags)
	VALUES ('delete', old.rowid, old.id, old.title, old.content, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
	INSERT INTO documents_fts(documents_fts, rowid, id, title, content, tags)
	VALUES ('delete', old.rowid, old.id, old.title, old.content, old.tags);
	INSERT INTO documents_fts(rowid, id, title, content, tags)
	VALUES (new.rowid, new.id, new.title, new.content, new.tags);
END;
`
