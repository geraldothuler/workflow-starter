package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/docstore"
)

const contextFileName = "context.json"

// Entry is a single structured fact in the context store.
type Entry struct {
	Key          string `json:"key"`
	Value        string `json:"value"`
	Type         string `json:"type,omitempty"`
	Topic        string `json:"topic,omitempty"`
	Description  string `json:"description,omitempty"`
	LastVerified string `json:"last_verified,omitempty"` // YYYY-MM-DD
}

// Store manages structured facts backed by docs.db (type=config).
// Topic is stored in the Repo field; Type/Description/LastVerified are
// encoded in the Tags field as "type:<v>", "desc:<v>", "verified:<v>".
type Store struct {
	db *docstore.DB
}

// LoadStore opens docs.db for the given repoRoot and returns a Store.
func LoadStore(repoRoot string) (*Store, error) {
	db, err := docstore.Open(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("context store: %w", err)
	}
	return &Store{db: db}, nil
}

// Save is a no-op — docs.db is written synchronously in Set.
func (s *Store) Save(_ string) error { return nil }

// Set adds or updates a config entry. Uses today's date as last_verified.
func (s *Store) Set(key, value, entryType, topic, description string) {
	today := time.Now().Format("2006-01-02")
	tags := buildTags(entryType, description, today)

	existing, err := s.db.Get(key)
	if err == nil && existing.Type == "config" {
		// Merge — preserve existing values if args are empty
		if entryType == "" {
			entryType = tagVal(existing.Tags, "type")
		}
		if topic == "" {
			topic = existing.Repo
		}
		if description == "" {
			description = tagVal(existing.Tags, "desc")
		}
		tags = buildTags(entryType, description, today)
		_, _ = s.db.Update(key, docstore.DocUpdateInput{
			Content: value,
			Repo:    topic,
			Tags:    tags,
		})
		return
	}
	_, _ = s.db.Add(docstore.DocInput{
		ID:      key,
		Type:    "config",
		Title:   key,
		DocDate: today,
		Repo:    topic,
		Tags:    tags,
		Content: value,
	})
}

// Get returns the entry for a key, or false if not found.
func (s *Store) Get(key string) (Entry, bool) {
	doc, err := s.db.Get(key)
	if err != nil || doc.Type != "config" || doc.DeletedAt != "" {
		return Entry{}, false
	}
	return docToEntry(doc), true
}

// FilterByTopic returns entries matching a topic (empty = all config entries).
func (s *Store) FilterByTopic(topic string) []Entry {
	docs, err := s.db.List(docstore.DocFilter{Type: "config", Repo: topic})
	if err != nil {
		return nil
	}
	out := make([]Entry, 0, len(docs))
	for _, d := range docs {
		out = append(out, docToEntry(d))
	}
	return out
}

// Stale returns entries whose LastVerified is older than maxDays.
func (s *Store) Stale(maxDays int) []Entry {
	docs, err := s.db.List(docstore.DocFilter{Type: "config"})
	if err != nil {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -maxDays)
	var out []Entry
	for _, d := range docs {
		verified := tagVal(d.Tags, "verified")
		if verified == "" {
			out = append(out, docToEntry(d))
			continue
		}
		t, err := time.Parse("2006-01-02", verified)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			out = append(out, docToEntry(d))
		}
	}
	return out
}

// MigrateFromJSON imports all entries from context.json into docs.db.
// Returns the count of entries imported and any error.
// Entries already present (by key) are skipped.
func MigrateFromJSON(repoRoot string, s *Store) (int, error) {
	path := filepath.Join(repoRoot, contextFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading context.json: %w", err)
	}
	var raw struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, fmt.Errorf("parsing context.json: %w", err)
	}
	count := 0
	for _, e := range raw.Entries {
		if _, exists := s.Get(e.Key); exists {
			continue
		}
		s.Set(e.Key, e.Value, e.Type, e.Topic, e.Description)
		count++
	}
	return count, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// buildTags encodes type, description, and last_verified as tag strings.
func buildTags(entryType, description, verified string) []string {
	var tags []string
	if entryType != "" {
		tags = append(tags, "type:"+entryType)
	}
	if description != "" {
		tags = append(tags, "desc:"+description)
	}
	if verified != "" {
		tags = append(tags, "verified:"+verified)
	}
	return tags
}

// tagVal extracts the value of a "prefix:value" tag from a tag slice.
func tagVal(tags []string, prefix string) string {
	p := prefix + ":"
	for _, t := range tags {
		if strings.HasPrefix(t, p) {
			return strings.TrimPrefix(t, p)
		}
	}
	return ""
}

// docToEntry converts a docstore.Document (type=config) to an Entry.
func docToEntry(d docstore.Document) Entry {
	return Entry{
		Key:          d.ID,
		Value:        d.Content,
		Type:         tagVal(d.Tags, "type"),
		Topic:        d.Repo,
		Description:  tagVal(d.Tags, "desc"),
		LastVerified: tagVal(d.Tags, "verified"),
	}
}
