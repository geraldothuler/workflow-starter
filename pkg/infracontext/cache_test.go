package infracontext

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCache_PutGet(t *testing.T) {
	cache := NewCache("")

	ic := &InfraContext{
		Provider:  "kubectl",
		Cluster:   "test",
		Namespace: "default",
		FetchedAt: time.Now(),
		TTL:       5 * time.Minute,
	}

	cache.Put("test-key", ic)

	got, ok := cache.Get("test-key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Provider != "kubectl" {
		t.Errorf("provider = %q", got.Provider)
	}
}

func TestCache_Miss(t *testing.T) {
	cache := NewCache("")

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCache_Expiration(t *testing.T) {
	cache := NewCache("")

	ic := &InfraContext{
		Provider:  "kubectl",
		FetchedAt: time.Now(),
		TTL:       1 * time.Millisecond,
	}

	cache.Put("expire-key", ic)
	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get("expire-key")
	if ok {
		t.Error("expected cache miss after TTL")
	}
}

func TestCache_DefaultTTL(t *testing.T) {
	cache := NewCache("")

	ic := &InfraContext{
		Provider:  "kubectl",
		FetchedAt: time.Now(),
		TTL:       0, // Should get default 5m
	}

	cache.Put("default-ttl", ic)

	got, ok := cache.Get("default-ttl")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Provider != "kubectl" {
		t.Errorf("provider = %q", got.Provider)
	}
}

func TestCache_Expire(t *testing.T) {
	cache := NewCache("")

	// Add two entries: one expired, one not
	cache.Put("expired", &InfraContext{
		Provider:  "test",
		FetchedAt: time.Now(),
		TTL:       1 * time.Millisecond,
	})
	cache.Put("fresh", &InfraContext{
		Provider:  "test",
		FetchedAt: time.Now(),
		TTL:       5 * time.Minute,
	})

	time.Sleep(5 * time.Millisecond)
	cache.Expire()

	_, ok := cache.Get("expired")
	if ok {
		t.Error("expected expired entry to be removed")
	}

	_, ok = cache.Get("fresh")
	if !ok {
		t.Error("expected fresh entry to still exist")
	}
}

func TestCache_DiskPersistence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache", "infra")

	cache1 := NewCache(dir)
	cache1.Put("disk-key", &InfraContext{
		Provider:  "kubectl",
		Cluster:   "prod",
		FetchedAt: time.Now(),
		TTL:       5 * time.Minute,
	})

	// Create new cache instance from same directory
	cache2 := NewCache(dir)

	got, ok := cache2.Get("disk-key")
	if !ok {
		t.Fatal("expected cache hit from disk")
	}
	if got.Cluster != "prod" {
		t.Errorf("cluster = %q, want prod", got.Cluster)
	}
}

func TestCacheKey(t *testing.T) {
	tests := []struct {
		provider  string
		namespace string
		context   string
		want      string
	}{
		{"kubectl", "default", "", "kubectl:default"},
		{"kubectl", "prod", "prod-cluster", "kubectl:prod:prod-cluster"},
		{"datadog", "monitoring", "", "datadog:monitoring"},
	}

	for _, tt := range tests {
		got := CacheKey(tt.provider, tt.namespace, tt.context)
		if got != tt.want {
			t.Errorf("CacheKey(%q, %q, %q) = %q, want %q",
				tt.provider, tt.namespace, tt.context, got, tt.want)
		}
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"kubectl:default", "kubectl_default"},
		{"kubectl:prod:cluster", "kubectl_prod_cluster"},
		{"simple", "simple"},
		{"path/with/slashes", "path_with_slashes"},
	}

	for _, tt := range tests {
		got := sanitizeKey(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
