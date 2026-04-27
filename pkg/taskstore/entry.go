package taskstore

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Status values.
const (
	StatusPending    = "pending"
	StatusInProgress = "in-progress"
	StatusBlocked    = "blocked"
	StatusDone       = "done"
)

// Entry is a fully loaded task with all relations.
type Entry struct {
	ID          string
	Title       string
	Status      string
	Description string
	DateTarget  string // YYYY-MM-DD, may be empty
	DateCreated string
	DateUpdated string
	DateDone    string // YYYY-MM-DD, may be empty
	Repos       []string
	Tags        []string
	Jira        []string
	BlockedBy   []string
}

// EntryInput is the data needed to create a new entry.
type EntryInput struct {
	Title       string
	Description string
	DateTarget  string
	DateDone    string // if set, entry is created as done with this date (YYYY-MM-DD)
	Repos       []string
	Tags        []string
	Jira        []string
}

// Filter controls which entries List() returns.
type Filter struct {
	Status string // "" = non-done; "all" = every status; specific value otherwise
	Repo   string // substring match on repo name
	Tag    string // exact tag match
	Jira   string // exact ticket match
	Since  string // YYYY-MM-DD lower bound on date_created
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9\s]+`)
var spaces = regexp.MustCompile(`\s+`)

func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlnum.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	s = spaces.ReplaceAllString(s, "-")
	if len(s) > 50 {
		s = strings.TrimRight(s[:50], "-")
	}
	return s
}

func (db *DB) uniqueID(base string) (string, error) {
	slug := base
	for i := 2; ; i++ {
		var count int
		if err := db.sql.QueryRow("SELECT COUNT(*) FROM entries WHERE id=?", slug).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func today() string { return time.Now().Format("2006-01-02") }

// Add inserts a new entry and re-renders backlog.md.
func (db *DB) Add(in EntryInput) (Entry, error) {
	if strings.TrimSpace(in.Title) == "" {
		return Entry{}, fmt.Errorf("title is required")
	}
	id, err := db.uniqueID(slugify(in.Title))
	if err != nil {
		return Entry{}, err
	}
	now := today()

	status := StatusPending
	dateDone := ""
	if in.DateDone != "" {
		status = StatusDone
		dateDone = in.DateDone
	}

	tx, err := db.sql.Begin()
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`INSERT INTO entries (id, title, status, description, date_target, date_created, date_updated, date_done)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.Title, status, in.Description, in.DateTarget, now, now, dateDone,
	); err != nil {
		return Entry{}, err
	}
	if err := txInsertRelations(tx, id, in.Repos, in.Tags, in.Jira); err != nil {
		return Entry{}, err
	}
	if err := tx.Commit(); err != nil {
		return Entry{}, err
	}

	return db.Get(id)
}

// transition changes status and re-renders.
func (db *DB) transition(id, status string) error {
	now := today()
	done := ""
	if status == StatusDone {
		done = now
	}
	res, err := db.sql.Exec(
		`UPDATE entries SET status=?, date_updated=?, date_done=? WHERE id=?`,
		status, now, done, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("entry %q not found", id)
	}
	return nil
}

func (db *DB) Done(id string) error  { return db.transition(id, StatusDone) }
func (db *DB) Block(id string) error { return db.transition(id, StatusBlocked) }
func (db *DB) Start(id string) error { return db.transition(id, StatusInProgress) }

// EntryUpdateInput holds the fields that Update() may change (zero value = no change).
type EntryUpdateInput struct {
	Title       string
	Description string
	DateTarget  string
	Repos       []string // nil = no change
	Tags        []string // nil = no change
	Jira        []string // nil = no change
}

// Update changes metadata of an existing entry.
func (db *DB) Update(id string, in EntryUpdateInput) (Entry, error) {
	e, err := db.Get(id)
	if err != nil {
		return Entry{}, err
	}
	title := e.Title
	if in.Title != "" {
		title = in.Title
	}
	desc := e.Description
	if in.Description != "" {
		desc = in.Description
	}
	dateTarget := e.DateTarget
	if in.DateTarget != "" {
		dateTarget = in.DateTarget
	}

	tx, err := db.sql.Begin()
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`UPDATE entries SET title=?, description=?, date_target=?, date_updated=? WHERE id=?`,
		title, desc, dateTarget, today(), id,
	); err != nil {
		return Entry{}, err
	}
	if in.Repos != nil {
		if _, err := tx.Exec("DELETE FROM entry_repos WHERE entry_id=?", id); err != nil {
			return Entry{}, err
		}
		for _, r := range in.Repos {
			if _, err := tx.Exec("INSERT OR IGNORE INTO entry_repos (entry_id, repo) VALUES (?,?)", id, r); err != nil {
				return Entry{}, err
			}
		}
	}
	if in.Tags != nil {
		if _, err := tx.Exec("DELETE FROM entry_tags WHERE entry_id=?", id); err != nil {
			return Entry{}, err
		}
		for _, t := range in.Tags {
			if _, err := tx.Exec("INSERT OR IGNORE INTO entry_tags (entry_id, tag) VALUES (?,?)", id, t); err != nil {
				return Entry{}, err
			}
		}
	}
	if in.Jira != nil {
		if _, err := tx.Exec("DELETE FROM entry_jira WHERE entry_id=?", id); err != nil {
			return Entry{}, err
		}
		for _, j := range in.Jira {
			if _, err := tx.Exec("INSERT OR IGNORE INTO entry_jira (entry_id, ticket) VALUES (?,?)", id, j); err != nil {
				return Entry{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return Entry{}, err
	}
	return db.Get(id)
}

// Get returns a single entry by ID.
func (db *DB) Get(id string) (Entry, error) {
	entries, err := db.runQuery(
		`SELECT id, title, status, description, date_target, date_created, date_updated, date_done
		 FROM entries WHERE id=?`, id)
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, fmt.Errorf("entry %q not found", id)
	}
	return entries[0], nil
}

// List returns entries matching the filter, ordered by date_created DESC.
func (db *DB) List(f Filter) ([]Entry, error) {
	q := `SELECT DISTINCT e.id, e.title, e.status, e.description,
		e.date_target, e.date_created, e.date_updated, e.date_done
		FROM entries e`

	var joins, where []string
	var args []any

	if f.Repo != "" {
		joins = append(joins, "JOIN entry_repos er ON e.id=er.entry_id")
		where = append(where, "er.repo LIKE ?")
		args = append(args, "%"+f.Repo+"%")
	}
	if f.Tag != "" {
		joins = append(joins, "JOIN entry_tags et ON e.id=et.entry_id")
		where = append(where, "et.tag = ?")
		args = append(args, f.Tag)
	}
	if f.Jira != "" {
		joins = append(joins, "JOIN entry_jira ej ON e.id=ej.entry_id")
		where = append(where, "ej.ticket = ?")
		args = append(args, f.Jira)
	}
	switch f.Status {
	case "all":
		// no status filter
	case "":
		where = append(where, "e.status != 'done'")
	default:
		where = append(where, "e.status = ?")
		args = append(args, f.Status)
	}
	if f.Since != "" {
		where = append(where, "e.date_created >= ?")
		args = append(args, f.Since)
	}

	if len(joins) > 0 {
		q += " " + strings.Join(joins, " ")
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY e.date_created DESC"

	return db.runQuery(q, args...)
}

// Search performs FTS5 keyword search on title + description.
func (db *DB) Search(keyword string) ([]Entry, error) {
	return db.runQuery(`
		SELECT e.id, e.title, e.status, e.description,
		       e.date_target, e.date_created, e.date_updated, e.date_done
		FROM entries e
		JOIN entries_fts f ON e.id = f.id
		WHERE entries_fts MATCH ?
		ORDER BY e.date_created DESC`, keyword)
}

// runQuery executes a SELECT and returns fully loaded entries.
func (db *DB) runQuery(q string, args ...any) ([]Entry, error) {
	rows, err := db.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Title, &e.Status, &e.Description,
			&e.DateTarget, &e.DateCreated, &e.DateUpdated, &e.DateDone); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range entries {
		if err := db.loadRelations(&entries[i]); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func (db *DB) loadRelations(e *Entry) error {
	e.Repos, _ = db.stringList("SELECT repo    FROM entry_repos   WHERE entry_id=? ORDER BY repo", e.ID)
	e.Tags, _ = db.stringList("SELECT tag     FROM entry_tags    WHERE entry_id=? ORDER BY tag", e.ID)
	e.Jira, _ = db.stringList("SELECT ticket  FROM entry_jira    WHERE entry_id=? ORDER BY ticket", e.ID)
	e.BlockedBy, _ = db.stringList("SELECT blocked_by FROM entry_blocked WHERE entry_id=? ORDER BY blocked_by", e.ID)
	return nil
}

func (db *DB) stringList(q, id string) ([]string, error) {
	rows, err := db.sql.Query(q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// txInsertRelations inserts all relation rows within an active transaction.
func txInsertRelations(tx *sql.Tx, id string, repos, tags, jira []string) error {
	for _, r := range repos {
		if _, err := tx.Exec("INSERT OR IGNORE INTO entry_repos (entry_id, repo) VALUES (?,?)", id, r); err != nil {
			return err
		}
	}
	for _, t := range tags {
		if _, err := tx.Exec("INSERT OR IGNORE INTO entry_tags (entry_id, tag) VALUES (?,?)", id, t); err != nil {
			return err
		}
	}
	for _, j := range jira {
		if _, err := tx.Exec("INSERT OR IGNORE INTO entry_jira (entry_id, ticket) VALUES (?,?)", id, j); err != nil {
			return err
		}
	}
	return nil
}
