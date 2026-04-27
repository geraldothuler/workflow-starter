package infracontext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache provides TTL-based caching for InfraContext results.
// Data is stored both in memory and on disk (in .workflow/cache/infra/).
type Cache struct {
	mu       sync.RWMutex
	entries  map[string]*cacheEntry
	cacheDir string
}

type cacheEntry struct {
	Data      *InfraContext `json:"data"`
	ExpiresAt time.Time    `json:"expires_at"`
}

// NewCache creates a new cache with the given directory.
// If cacheDir is empty, only in-memory caching is used.
func NewCache(cacheDir string) *Cache {
	c := &Cache{
		entries:  make(map[string]*cacheEntry),
		cacheDir: cacheDir,
	}

	// Load existing disk cache entries
	if cacheDir != "" {
		c.loadFromDisk()
	}

	return c
}

// Get retrieves a cached InfraContext if it exists and hasn't expired.
func (c *Cache) Get(key string) (*InfraContext, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Data, true
}

// Put stores an InfraContext in the cache with the TTL from the context.
func (c *Cache) Put(key string, ic *InfraContext) {
	ttl := ic.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	entry := &cacheEntry{
		Data:      ic,
		ExpiresAt: time.Now().Add(ttl),
	}

	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()

	// Persist to disk
	if c.cacheDir != "" {
		c.saveToDisk(key, entry)
	}
}

// CacheKey generates a cache key from provider ID, namespace, and context.
func CacheKey(providerID, namespace, kubeContext string) string {
	key := providerID + ":" + namespace
	if kubeContext != "" {
		key += ":" + kubeContext
	}
	return key
}

// Expire removes expired entries from the cache.
func (c *Cache) Expire() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			c.removeDiskEntry(key)
		}
	}
}

func (c *Cache) loadFromDisk() {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(c.cacheDir, entry.Name()))
		if err != nil {
			continue
		}
		var ce cacheEntry
		if err := json.Unmarshal(data, &ce); err != nil {
			continue
		}
		if time.Now().Before(ce.ExpiresAt) {
			key := entry.Name()[:len(entry.Name())-5] // remove .json
			c.entries[key] = &ce
		}
	}
}

func (c *Cache) saveToDisk(key string, entry *cacheEntry) {
	if err := os.MkdirAll(c.cacheDir, 0700); err != nil {
		return
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	filename := sanitizeKey(key) + ".json"
	_ = os.WriteFile(filepath.Join(c.cacheDir, filename), data, 0600)
}

func (c *Cache) removeDiskEntry(key string) {
	if c.cacheDir == "" {
		return
	}
	filename := sanitizeKey(key) + ".json"
	_ = os.Remove(filepath.Join(c.cacheDir, filename))
}

// sanitizeKey replaces characters unsafe for filenames.
func sanitizeKey(key string) string {
	result := make([]byte, len(key))
	for i := range key {
		if key[i] == '/' || key[i] == ':' || key[i] == '\\' {
			result[i] = '_'
		} else {
			result[i] = key[i]
		}
	}
	return string(result)
}
