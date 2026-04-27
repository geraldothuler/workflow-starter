package docstore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Document is a stored workflow artefact.
type Document struct {
	ID        string
	Type      string
	Title     string
	DocDate   string // YYYY-MM-DD
	Repo      string
	Tags      []string
	Content   string
	DeletedAt string // empty = not deleted
	CreatedAt string
	UpdatedAt string
}

// DocInput is the data needed to add a document.
type DocInput struct {
	ID      string   // optional — if set, used as document ID instead of slug(Title)
	Type    string
	Title   string
	DocDate string
	Repo    string
	Tags    []string
	Content string
}

// DocFilter controls which documents List() returns.
type DocFilter struct {
	Type  string // exact match; "" = all non-deleted
	Since string // YYYY-MM-DD lower bound on doc_date
	Repo  string // substring match
	Tag   string // substring match in tags field
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9\s]+`)
var spaces = regexp.MustCompile(`\s+`)

func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlnum.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	s = spaces.ReplaceAllString(s, "-")
	if len(s) > 60 {
		s = strings.TrimRight(s[:60], "-")
	}
	return s
}

func (db *DB) uniqueID(base string) (string, error) {
	slug := base
	for i := 2; ; i++ {
		var count int
		if err := db.sql.QueryRow("SELECT COUNT(*) FROM documents WHERE id=?", slug).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func now() string { return time.Now().Format("2006-01-02") }

// Add inserts a new document.
func (db *DB) Add(in DocInput) (Document, error) {
	if strings.TrimSpace(in.Title) == "" {
		return Document{}, fmt.Errorf("title is required")
	}
	base := in.ID
	if base == "" {
		base = slugify(in.Title)
	}
	id, err := db.uniqueID(base)
	if err != nil {
		return Document{}, err
	}
	ts := now()
	tags := strings.Join(in.Tags, ",")

	_, err = db.sql.Exec(
		`INSERT INTO documents (id, type, title, doc_date, repo, tags, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.Type, in.Title, in.DocDate, in.Repo, tags, in.Content, ts, ts,
	)
	if err != nil {
		return Document{}, err
	}
	return db.Get(id)
}

// Get returns a single document by ID (including soft-deleted).
func (db *DB) Get(id string) (Document, error) {
	docs, err := db.query(
		`SELECT id, type, title, doc_date, repo, tags, content, deleted_at, created_at, updated_at
		 FROM documents WHERE id=?`, id)
	if err != nil {
		return Document{}, err
	}
	if len(docs) == 0 {
		return Document{}, fmt.Errorf("document %q not found", id)
	}
	return docs[0], nil
}

// List returns non-deleted documents matching the filter, ordered by doc_date DESC.
func (db *DB) List(f DocFilter) ([]Document, error) {
	q := `SELECT id, type, title, doc_date, repo, tags, content, deleted_at, created_at, updated_at
	      FROM documents WHERE deleted_at = ''`
	var args []any

	if f.Type != "" {
		q += " AND type = ?"
		args = append(args, f.Type)
	}
	if f.Since != "" {
		q += " AND doc_date >= ?"
		args = append(args, f.Since)
	}
	if f.Repo != "" {
		q += " AND repo LIKE ?"
		args = append(args, "%"+f.Repo+"%")
	}
	if f.Tag != "" {
		q += " AND tags LIKE ?"
		args = append(args, "%"+f.Tag+"%")
	}
	q += " ORDER BY doc_date DESC"
	return db.query(q, args...)
}

// Search performs FTS5 keyword search on title, content, and tags.
func (db *DB) Search(keyword string) ([]Document, error) {
	return db.query(`
		SELECT d.id, d.type, d.title, d.doc_date, d.repo, d.tags, d.content, d.deleted_at, d.created_at, d.updated_at
		FROM documents d
		JOIN documents_fts f ON d.id = f.id
		WHERE documents_fts MATCH ? AND d.deleted_at = ''
		ORDER BY d.doc_date DESC`, keyword)
}

// SearchAll performs FTS5 search on docs.db and also scans savepoints/ and
// artifacts/ directories for matching .md files. Results are merged and
// deduplicated (docs.db wins on ID collision), ordered by doc_date DESC.
func (db *DB) SearchAll(keyword string) ([]Document, error) {
	dbDocs, err := db.Search(keyword)
	if err != nil {
		// FTS5 can reject keywords with special chars; fall back to empty
		dbDocs = nil
	}

	var fsDocs []Document
	for _, subdir := range []string{"savepoints", "artifacts"} {
		fsDocs = append(fsDocs, db.searchFiles(keyword, subdir)...)
	}

	seen := make(map[string]bool, len(dbDocs))
	result := make([]Document, 0, len(dbDocs)+len(fsDocs))
	for _, d := range dbDocs {
		seen[d.ID] = true
		result = append(result, d)
	}
	for _, d := range fsDocs {
		if !seen[d.ID] {
			seen[d.ID] = true
			result = append(result, d)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].DocDate != result[j].DocDate {
			return result[i].DocDate > result[j].DocDate
		}
		return result[i].ID > result[j].ID
	})
	return result, nil
}

// searchFiles walks subdir under repoRoot, returning Documents from .md files
// that contain all words in keyword (case-insensitive substring match).
func (db *DB) searchFiles(keyword, subdir string) []Document {
	dir := filepath.Join(db.repoRoot, subdir)
	words := strings.Fields(strings.ToLower(keyword))
	if len(words) == 0 {
		return nil
	}

	// Map subdir name to a clean document type
	docType := strings.TrimRight(subdir, "s") // "savepoints"→"savepoint", "artifacts"→"artifact"

	var docs []Document
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lower := strings.ToLower(string(raw))
		for _, w := range words {
			if !strings.Contains(lower, w) {
				return nil
			}
		}
		in := parseDoc(entry.Name(), string(raw), docType)
		rel, _ := filepath.Rel(db.repoRoot, path)
		id := strings.TrimSuffix(rel, ".md")
		id = strings.ReplaceAll(id, string(filepath.Separator), "/")
		docs = append(docs, Document{
			ID:      id,
			Type:    in.Type,
			Title:   in.Title,
			DocDate: in.DocDate,
			Repo:    in.Repo,
			Tags:    in.Tags,
			Content: in.Content,
		})
		return nil
	})
	return docs
}

// Append adds content to the end of an existing document's content field.
func (db *DB) Append(id, content string) (Document, error) {
	d, err := db.Get(id)
	if err != nil {
		return Document{}, err
	}
	newContent := strings.TrimRight(d.Content, "\n") + "\n\n" + content
	_, err = db.sql.Exec(
		`UPDATE documents SET content=?, updated_at=? WHERE id=?`,
		newContent, now(), id,
	)
	if err != nil {
		return Document{}, err
	}
	return db.Get(id)
}

// DocUpdateInput holds the fields that Update() may change (zero value = no change).
type DocUpdateInput struct {
	Title   string
	Tags    []string // nil = no change; empty slice = clear tags
	Repo    string
	DocDate string
	Content string // non-empty replaces content entirely
}

// Update changes metadata fields of an existing document.
func (db *DB) Update(id string, in DocUpdateInput) (Document, error) {
	d, err := db.Get(id)
	if err != nil {
		return Document{}, err
	}
	title := d.Title
	if in.Title != "" {
		title = in.Title
	}
	repo := d.Repo
	if in.Repo != "" {
		repo = in.Repo
	}
	docDate := d.DocDate
	if in.DocDate != "" {
		docDate = in.DocDate
	}
	content := d.Content
	if in.Content != "" {
		content = in.Content
	}
	tags := strings.Join(d.Tags, ",")
	if in.Tags != nil {
		tags = strings.Join(in.Tags, ",")
	}
	_, err = db.sql.Exec(
		`UPDATE documents SET title=?, repo=?, doc_date=?, tags=?, content=?, updated_at=? WHERE id=?`,
		title, repo, docDate, tags, content, now(), id,
	)
	if err != nil {
		return Document{}, err
	}
	return db.Get(id)
}

// Delete soft-deletes a document.
func (db *DB) Delete(id string) error {
	res, err := db.sql.Exec(
		`UPDATE documents SET deleted_at=?, updated_at=? WHERE id=? AND deleted_at=''`,
		now(), now(), id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("document %q not found or already deleted", id)
	}
	return nil
}

// ImportDir imports all .md files from dir as the given docType.
// It parses YAML frontmatter (if present) to extract title, date, repo, tags.
// Returns count of imported files and any errors encountered.
func (db *DB) ImportDir(dir, docType string) (int, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, []error{err}
	}

	count := 0
	var errs []error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}

		in := parseDoc(e.Name(), string(content), docType)
		if _, err := db.Add(in); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		count++
	}
	return count, errs
}

// ── frontmatter parser ────────────────────────────────────────────────────────

var reFMDate = regexp.MustCompile(`^date:\s*["']?(\d{4}-\d{2}-\d{2})["']?`)
var reFMTitle = regexp.MustCompile(`^title:\s*["'](.+)["']`)
var reFMTags = regexp.MustCompile(`^tags:\s*\[(.+)\]`)
var reFMRepo = regexp.MustCompile(`^(?:repo|repos):\s*["']?([^"'\s]+)["']?`)
var reFilenameDate = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)

// parseDoc extracts DocInput from a markdown file, trying frontmatter first,
// then falling back to filename patterns and h1 heading.
func parseDoc(filename, content, docType string) DocInput {
	in := DocInput{Type: docType}

	// Infer date from filename
	if m := reFilenameDate.FindStringSubmatch(filename); m != nil {
		in.DocDate = m[1]
	}

	// Try YAML frontmatter
	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end >= 0 {
			fm := content[3 : 3+end]
			scanner := bufio.NewScanner(strings.NewReader(fm))
			for scanner.Scan() {
				line := scanner.Text()
				if m := reFMDate.FindStringSubmatch(line); m != nil {
					in.DocDate = m[1]
				} else if m := reFMTitle.FindStringSubmatch(line); m != nil {
					in.Title = m[1]
				} else if m := reFMTags.FindStringSubmatch(line); m != nil {
					for _, t := range strings.Split(m[1], ",") {
						t = strings.Trim(strings.TrimSpace(t), `"'`)
						if t != "" {
							in.Tags = append(in.Tags, t)
						}
					}
				} else if m := reFMRepo.FindStringSubmatch(line); m != nil {
					in.Repo = m[1]
				}
			}
			in.Content = strings.TrimSpace(content[3+end+3:])
		}
	} else {
		in.Content = content
	}

	// Fallback title: first h1 heading or filename stem
	if in.Title == "" {
		for _, line := range strings.SplitN(in.Content, "\n", 20) {
			if strings.HasPrefix(line, "# ") {
				in.Title = strings.TrimPrefix(line, "# ")
				break
			}
		}
	}
	if in.Title == "" {
		stem := strings.TrimSuffix(filename, ".md")
		in.Title = stem
	}

	return in
}

// ── internal query helper ─────────────────────────────────────────────────────

func (db *DB) query(q string, args ...any) ([]Document, error) {
	rows, err := db.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		var tagStr string
		if err := rows.Scan(&d.ID, &d.Type, &d.Title, &d.DocDate, &d.Repo,
			&tagStr, &d.Content, &d.DeletedAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		if tagStr != "" {
			d.Tags = strings.Split(tagStr, ",")
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// GetTemplate returns the content of the template for a given doc type.
// Templates are stored as type=template with ID "template-{docType}".
func (db *DB) GetTemplate(docType string) (string, bool) {
	doc, err := db.Get("template-" + docType)
	if err != nil || doc.Type != "template" || doc.DeletedAt != "" {
		return "", false
	}
	return doc.Content, true
}

// SetTemplate creates or updates the template for a given doc type.
func (db *DB) SetTemplate(docType, title, content string) error {
	id := "template-" + docType
	_, err := db.Get(id)
	if err == nil {
		_, err = db.Update(id, DocUpdateInput{Title: title, Content: content})
		return err
	}
	_, err = db.Add(DocInput{
		ID:      id,
		Type:    "template",
		Title:   title,
		Content: content,
	})
	return err
}
