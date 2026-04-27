// Package store implements an append-only SQLite log of OpsResult executions
// at ~/.workflow/ops-log.db.
//
// Design: lightweight local store — disposable and reconstructible.
// Schema is applied on first open; reads return nil gracefully on missing file.
package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Record is one entry in the ops log.
type Record struct {
	Ts     string `json:"ts"`     // RFC3339 UTC
	Probe  string `json:"probe"`  // "db-health", "airbyte", etc.
	Status string `json:"status"` // ok | warn | critical | error
	Signal string `json:"signal"` // human-readable summary
	Repo   string `json:"repo"`   // repo/context path
}

// DefaultPath returns ~/.workflow/ops-log.db.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".workflow", "ops-log.db")
}

const schema = `
CREATE TABLE IF NOT EXISTS ops_log (
	id     INTEGER PRIMARY KEY AUTOINCREMENT,
	ts     TEXT NOT NULL,
	probe  TEXT NOT NULL,
	status TEXT NOT NULL,
	signal TEXT NOT NULL,
	repo   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_probe_ts ON ops_log(probe, ts);
`

// openDB opens (or creates) the SQLite database at path, applying the schema.
func openDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Append inserts one Record (creates DB and schema if missing).
func Append(path, probe, status, signal, repo string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	ts := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(
		`INSERT INTO ops_log (ts, probe, status, signal, repo) VALUES (?, ?, ?, ?, ?)`,
		ts, probe, status, signal, repo,
	)
	return err
}

// ReadAll returns all records ordered by ts ascending.
// Returns nil slice (not error) if the database file does not exist.
func ReadAll(path string) ([]Record, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT ts, probe, status, signal, repo FROM ops_log ORDER BY ts ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.Ts, &r.Probe, &r.Status, &r.Signal, &r.Repo); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// QueryTrend returns the last n records for a probe, newest first.
// If probe is empty, returns the last n records across all probes.
func QueryTrend(path, probe string, n int) []Record {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	db, err := openDB(path)
	if err != nil {
		return nil
	}
	defer db.Close()

	var rows *sql.Rows
	if probe == "" {
		rows, err = db.Query(
			`SELECT ts, probe, status, signal, repo FROM ops_log ORDER BY ts DESC LIMIT ?`, n,
		)
	} else {
		rows, err = db.Query(
			`SELECT ts, probe, status, signal, repo FROM ops_log WHERE probe=? ORDER BY ts DESC LIMIT ?`,
			probe, n,
		)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.Ts, &r.Probe, &r.Status, &r.Signal, &r.Repo); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}
