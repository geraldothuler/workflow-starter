package credentials

import (
	"sync"
	"time"
)

// SessionCache is a thread-safe in-memory TTL cache for session credentials.
// Each entry has an independent expiration time.
type SessionCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry

	// nowFunc is injectable for testing (defaults to time.Now).
	nowFunc func() time.Time
}

type cacheEntry struct {
	credential *Credential
	expiresAt  time.Time
}

// NewSessionCache creates a new empty session cache.
func NewSessionCache() *SessionCache {
	return &SessionCache{
		entries: make(map[string]*cacheEntry),
		nowFunc: time.Now,
	}
}

// Get retrieves a cached credential by key.
// Returns nil if the entry doesn't exist or has expired.
// Expired entries are lazily cleaned up on access.
func (c *SessionCache) Get(key string) *Credential {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	if c.nowFunc().After(entry.expiresAt) {
		// Expired — clean up lazily
		c.mu.Lock()
		// Double-check after acquiring write lock
		if e, ok := c.entries[key]; ok && c.nowFunc().After(e.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return nil
	}

	return entry.credential
}

// Set stores a credential with a TTL.
func (c *SessionCache) Set(key string, cred *Credential, ttl time.Duration) {
	expiresAt := c.nowFunc().Add(ttl)

	// Also set ExpiresAt on the credential itself
	cred.ExpiresAt = &expiresAt

	c.mu.Lock()
	c.entries[key] = &cacheEntry{
		credential: cred,
		expiresAt:  expiresAt,
	}
	c.mu.Unlock()
}

// Invalidate removes a specific entry from the cache.
func (c *SessionCache) Invalidate(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// InvalidateAll clears all entries from the cache.
func (c *SessionCache) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

// Cleanup removes all expired entries from the cache.
// This can be called periodically for proactive cleanup.
func (c *SessionCache) Cleanup() int {
	now := c.nowFunc()
	removed := 0

	c.mu.Lock()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
			removed++
		}
	}
	c.mu.Unlock()

	return removed
}

// Size returns the number of entries in the cache (including expired ones not yet cleaned).
func (c *SessionCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// ActiveSize returns the number of non-expired entries.
func (c *SessionCache) ActiveSize() int {
	now := c.nowFunc()
	count := 0

	c.mu.RLock()
	for _, entry := range c.entries {
		if !now.After(entry.expiresAt) {
			count++
		}
	}
	c.mu.RUnlock()

	return count
}
