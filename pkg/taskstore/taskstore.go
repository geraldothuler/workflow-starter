// Package taskstore manages the operational task backlog in a local SQLite database.
//
// Single file in the repo root:
//
//	backlog.db   — SQLite source of truth (gitignored)
//
// Query via `wtb backlog list/search`. No markdown rendering.
package taskstore

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const dbFile = "backlog.db"

// DB wraps the SQLite connection and the repo root path.
type DB struct {
	sql      *sql.DB
	repoRoot string
}

// Open opens (or creates) backlog.db in repoRoot, applies the schema, and returns a DB.
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
	return &DB{sql: sqldb, repoRoot: repoRoot}, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.sql.Close()
}

const schema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS entries (
	id           TEXT PRIMARY KEY,
	title        TEXT NOT NULL,
	status       TEXT NOT NULL DEFAULT 'pending',
	description  TEXT NOT NULL DEFAULT '',
	date_target  TEXT NOT NULL DEFAULT '',
	date_created TEXT NOT NULL,
	date_updated TEXT NOT NULL,
	date_done    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS entry_repos (
	entry_id TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
	repo     TEXT NOT NULL,
	PRIMARY KEY (entry_id, repo)
);

CREATE TABLE IF NOT EXISTS entry_tags (
	entry_id TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
	tag      TEXT NOT NULL,
	PRIMARY KEY (entry_id, tag)
);

CREATE TABLE IF NOT EXISTS entry_jira (
	entry_id TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
	ticket   TEXT NOT NULL,
	PRIMARY KEY (entry_id, ticket)
);

CREATE TABLE IF NOT EXISTS entry_blocked (
	entry_id   TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
	blocked_by TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
	PRIMARY KEY (entry_id, blocked_by)
);

CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
	id, title, description,
	content=entries,
	content_rowid=rowid
);

CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
	INSERT INTO entries_fts(rowid, id, title, description)
	VALUES (new.rowid, new.id, new.title, new.description);
END;

CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
	INSERT INTO entries_fts(entries_fts, rowid, id, title, description)
	VALUES ('delete', old.rowid, old.id, old.title, old.description);
END;

CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
	INSERT INTO entries_fts(entries_fts, rowid, id, title, description)
	VALUES ('delete', old.rowid, old.id, old.title, old.description);
	INSERT INTO entries_fts(rowid, id, title, description)
	VALUES (new.rowid, new.id, new.title, new.description);
END;
`
