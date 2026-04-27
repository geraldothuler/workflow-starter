package credentials

import (
	"sync"
	"testing"
	"time"
)

func TestSessionCache_GetMiss(t *testing.T) {
	c := NewSessionCache()
	if got := c.Get("nonexistent"); got != nil {
		t.Errorf("expected nil for cache miss, got %v", got)
	}
}

func TestSessionCache_SetAndGet(t *testing.T) {
	c := NewSessionCache()
	cred := &Credential{Name: "TOKEN", Value: "secret", Source: "test"}
	c.Set("key1", cred, 1*time.Hour)

	got := c.Get("key1")
	if got == nil {
		t.Fatal("expected cache hit")
	}
	if got.Value != "secret" {
		t.Errorf("expected 'secret', got %q", got.Value)
	}
	if got.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestSessionCache_TTLExpiration(t *testing.T) {
	c := NewSessionCache()
	now := time.Now()
	c.nowFunc = func() time.Time { return now }

	cred := &Credential{Name: "TOKEN", Value: "secret", Source: "test"}
	c.Set("key1", cred, 1*time.Hour)

	// Still within TTL
	c.nowFunc = func() time.Time { return now.Add(30 * time.Minute) }
	if got := c.Get("key1"); got == nil {
		t.Error("expected cache hit within TTL")
	}

	// Past TTL
	c.nowFunc = func() time.Time { return now.Add(2 * time.Hour) }
	if got := c.Get("key1"); got != nil {
		t.Error("expected nil after TTL expiration")
	}

	// Entry should have been cleaned up
	if c.Size() != 0 {
		t.Errorf("expected size 0 after expiration cleanup, got %d", c.Size())
	}
}

func TestSessionCache_Invalidate(t *testing.T) {
	c := NewSessionCache()
	c.Set("key1", &Credential{Value: "v1"}, 1*time.Hour)
	c.Set("key2", &Credential{Value: "v2"}, 1*time.Hour)

	c.Invalidate("key1")

	if c.Get("key1") != nil {
		t.Error("expected key1 to be invalidated")
	}
	if c.Get("key2") == nil {
		t.Error("expected key2 to still exist")
	}
}

func TestSessionCache_InvalidateAll(t *testing.T) {
	c := NewSessionCache()
	c.Set("key1", &Credential{Value: "v1"}, 1*time.Hour)
	c.Set("key2", &Credential{Value: "v2"}, 1*time.Hour)

	c.InvalidateAll()

	if c.Size() != 0 {
		t.Errorf("expected size 0 after InvalidateAll, got %d", c.Size())
	}
}

func TestSessionCache_Cleanup(t *testing.T) {
	c := NewSessionCache()
	now := time.Now()
	c.nowFunc = func() time.Time { return now }

	c.Set("live", &Credential{Value: "alive"}, 2*time.Hour)
	c.Set("dead1", &Credential{Value: "expired1"}, 30*time.Minute)
	c.Set("dead2", &Credential{Value: "expired2"}, 45*time.Minute)

	// Advance time past the short-TTL entries
	c.nowFunc = func() time.Time { return now.Add(1 * time.Hour) }

	removed := c.Cleanup()
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
	if c.Size() != 1 {
		t.Errorf("expected 1 remaining, got %d", c.Size())
	}
	if c.Get("live") == nil {
		t.Error("expected 'live' entry to still exist")
	}
}

func TestSessionCache_Size(t *testing.T) {
	c := NewSessionCache()
	if c.Size() != 0 {
		t.Errorf("expected size 0, got %d", c.Size())
	}

	c.Set("k1", &Credential{Value: "v1"}, 1*time.Hour)
	c.Set("k2", &Credential{Value: "v2"}, 1*time.Hour)

	if c.Size() != 2 {
		t.Errorf("expected size 2, got %d", c.Size())
	}
}

func TestSessionCache_ActiveSize(t *testing.T) {
	c := NewSessionCache()
	now := time.Now()
	c.nowFunc = func() time.Time { return now }

	c.Set("active", &Credential{Value: "v1"}, 2*time.Hour)
	c.Set("expired", &Credential{Value: "v2"}, 30*time.Minute)

	c.nowFunc = func() time.Time { return now.Add(1 * time.Hour) }

	if got := c.ActiveSize(); got != 1 {
		t.Errorf("expected active size 1, got %d", got)
	}
}

func TestSessionCache_ConcurrentAccess(t *testing.T) {
	c := NewSessionCache()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Set("key", &Credential{Value: "v"}, 1*time.Hour)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Get("key")
		}()
	}

	wg.Wait()
	// No race condition — test passes if no panic/deadlock
}

func TestSessionCache_OverwriteKey(t *testing.T) {
	c := NewSessionCache()
	c.Set("key", &Credential{Value: "old"}, 1*time.Hour)
	c.Set("key", &Credential{Value: "new"}, 1*time.Hour)

	got := c.Get("key")
	if got == nil || got.Value != "new" {
		t.Errorf("expected 'new', got %v", got)
	}
}

func TestCredential_IsExpired(t *testing.T) {
	t.Run("no expiration", func(t *testing.T) {
		c := &Credential{Value: "v"}
		if c.IsExpired() {
			t.Error("credential without ExpiresAt should not be expired")
		}
	})

	t.Run("not expired", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour)
		c := &Credential{Value: "v", ExpiresAt: &future}
		if c.IsExpired() {
			t.Error("credential with future ExpiresAt should not be expired")
		}
	})

	t.Run("expired", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		c := &Credential{Value: "v", ExpiresAt: &past}
		if !c.IsExpired() {
			t.Error("credential with past ExpiresAt should be expired")
		}
	})
}
