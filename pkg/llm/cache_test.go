package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCache_HitReturnsCachedResponse(t *testing.T) {
	mock := NewMockClient("fresh response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)

	// First call: cache miss → calls mock
	resp1, usage1, err := cache.CompleteWithUsage("hello", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp1 != "fresh response" {
		t.Errorf("expected 'fresh response', got %q", resp1)
	}
	if mock.CallCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount)
	}
	if usage1.Cost == 0 {
		t.Error("expected non-zero cost on first call")
	}

	// Second call: cache hit → should NOT call mock again
	resp2, usage2, err := cache.CompleteWithUsage("hello", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2 != "fresh response" {
		t.Errorf("expected 'fresh response', got %q", resp2)
	}
	if mock.CallCount != 1 {
		t.Errorf("expected still 1 call (cache hit), got %d", mock.CallCount)
	}

	// Cache hit should have Cost=0
	if usage2.Cost != 0 {
		t.Errorf("expected Cost=0 for cache hit, got %f", usage2.Cost)
	}
	// But tokens should be preserved
	if usage2.InputTokens == 0 {
		t.Error("expected InputTokens from cached usage")
	}
}

func TestCache_MissCallsProvider(t *testing.T) {
	var callCount int32
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			atomic.AddInt32(&callCount, 1)
			return "response for: " + prompt, &Usage{InputTokens: 50, Cost: 0.01}, nil
		},
	}

	tmpDir := t.TempDir()
	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)

	// Different prompts → different cache keys → both miss
	resp1, _, _ := cache.CompleteWithUsage("prompt A", 100)
	resp2, _, _ := cache.CompleteWithUsage("prompt B", 100)

	if resp1 != "response for: prompt A" {
		t.Errorf("unexpected resp1: %q", resp1)
	}
	if resp2 != "response for: prompt B" {
		t.Errorf("unexpected resp2: %q", resp2)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 calls (different prompts), got %d", callCount)
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)

	// Freeze time
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cache.nowFunc = func() time.Time { return now }

	// First call at noon
	cache.CompleteWithUsage("test", 100)
	if mock.CallCount != 1 {
		t.Fatalf("expected 1 call, got %d", mock.CallCount)
	}

	// Second call 30min later → still valid
	cache.nowFunc = func() time.Time { return now.Add(30 * time.Minute) }
	cache.CompleteWithUsage("test", 100)
	if mock.CallCount != 1 {
		t.Errorf("expected still 1 call (within TTL), got %d", mock.CallCount)
	}

	// Third call 2 hours later → expired
	cache.nowFunc = func() time.Time { return now.Add(2 * time.Hour) }
	cache.CompleteWithUsage("test", 100)
	if mock.CallCount != 2 {
		t.Errorf("expected 2 calls (TTL expired), got %d", mock.CallCount)
	}
}

func TestCache_DisabledBypassesCache(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  false, // Cache disabled
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)

	cache.CompleteWithUsage("test", 100)
	cache.CompleteWithUsage("test", 100)
	cache.CompleteWithUsage("test", 100)

	// All 3 calls should go to provider (cache disabled)
	if mock.CallCount != 3 {
		t.Errorf("expected 3 calls (cache disabled), got %d", mock.CallCount)
	}
}

func TestCache_DifferentMaxTokensAreDifferentKeys(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)

	cache.CompleteWithUsage("same prompt", 100)
	cache.CompleteWithUsage("same prompt", 200) // Different maxTokens

	if mock.CallCount != 2 {
		t.Errorf("expected 2 calls (different maxTokens), got %d", mock.CallCount)
	}
}

func TestCache_SystemPromptAffectsKey(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)

	// Call with system prompt A
	cache.SetSystemPrompt("system A")
	cache.CompleteWithUsage("same prompt", 100)

	// Call with system prompt B → different key
	cache.SetSystemPrompt("system B")
	cache.CompleteWithUsage("same prompt", 100)

	if mock.CallCount != 2 {
		t.Errorf("expected 2 calls (different system prompts), got %d", mock.CallCount)
	}
}

func TestCache_ErrorsAreNotCached(t *testing.T) {
	var callCount int32
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				return "", nil, os.ErrNotExist // Simulate error
			}
			return "success on retry", &Usage{Cost: 0.01}, nil
		},
	}

	tmpDir := t.TempDir()
	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)

	// First call: error
	_, _, err := cache.CompleteWithUsage("test", 100)
	if err == nil {
		t.Fatal("expected error on first call")
	}

	// Second call: should hit provider again (errors not cached)
	resp, _, err := cache.CompleteWithUsage("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "success on retry" {
		t.Errorf("expected 'success on retry', got %q", resp)
	}

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 calls (error not cached), got %d", callCount)
	}
}

func TestCache_CompleteDelegatesToCompleteWithUsage(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)

	resp, err := cache.Complete("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("expected 'response', got %q", resp)
	}
}

func TestCache_DelegatesProviderMethods(t *testing.T) {
	mock := NewMockProvider("claude", "sonnet-4", "response")
	config := DefaultCacheConfig()
	config.CacheDir = t.TempDir()

	cache := WithCache(mock, config)

	if cache.ProviderName() != "claude" {
		t.Errorf("ProviderName: expected 'claude', got %q", cache.ProviderName())
	}
	if cache.ModelID() != "sonnet-4" {
		t.Errorf("ModelID: expected 'sonnet-4', got %q", cache.ModelID())
	}

	cache.SetSystemPrompt("test system")
	if mock.GetSystemPrompt() != "test system" {
		t.Errorf("SetSystemPrompt not delegated correctly")
	}
}

func TestCache_CacheKeyConsistency(t *testing.T) {
	mock := NewMockClient("resp")
	config := DefaultCacheConfig()
	config.CacheDir = t.TempDir()

	cache := WithCache(mock, config)
	cache.SetSystemPrompt("system")

	// Same inputs should produce same key
	key1 := cache.cacheKey("prompt", 100)
	key2 := cache.cacheKey("prompt", 100)
	if key1 != key2 {
		t.Errorf("same inputs produced different keys: %s vs %s", key1, key2)
	}

	// Key should be 64 hex chars (SHA256)
	if len(key1) != 64 {
		t.Errorf("expected 64-char hex key, got %d chars", len(key1))
	}

	// Different inputs → different keys
	key3 := cache.cacheKey("different prompt", 100)
	if key1 == key3 {
		t.Error("different prompts produced same key")
	}
}

func TestCache_FileStructure(t *testing.T) {
	mock := NewMockClient("cached response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithCache(mock, config)
	cache.CompleteWithUsage("test prompt", 100)

	// Verify cache file was created with correct structure
	var found bool
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			found = true

			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("error reading cache file: %v", readErr)
			}

			var entry cacheEntry
			if jsonErr := json.Unmarshal(data, &entry); jsonErr != nil {
				t.Fatalf("invalid JSON in cache file: %v", jsonErr)
			}

			if entry.Response != "cached response" {
				t.Errorf("cached response: expected 'cached response', got %q", entry.Response)
			}
			if entry.Provider != "mock" {
				t.Errorf("cached provider: expected 'mock', got %q", entry.Provider)
			}
			if entry.CreatedAt.IsZero() {
				t.Error("created_at should not be zero")
			}
		}
		return nil
	})

	if !found {
		t.Error("no cache file was created")
	}
}

func TestCache_ClearCache(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: cacheDir,
	}

	cache := WithCache(mock, config)

	// Create some cache entries
	cache.CompleteWithUsage("prompt 1", 100)
	cache.CompleteWithUsage("prompt 2", 100)

	// Verify entries exist
	entries, _ := cache.CacheStats()
	if entries != 2 {
		t.Fatalf("expected 2 cache entries, got %d", entries)
	}

	// Clear cache
	err := cache.ClearCache()
	if err != nil {
		t.Fatalf("error clearing cache: %v", err)
	}

	// Verify empty
	entries, _ = cache.CacheStats()
	if entries != 0 {
		t.Errorf("expected 0 entries after clear, got %d", entries)
	}

	// Should call provider again (cache cleared)
	callsBefore := mock.CallCount
	cache.CompleteWithUsage("prompt 1", 100)
	if mock.CallCount != callsBefore+1 {
		t.Error("expected provider call after cache clear")
	}
}

func TestCache_VerboseLogging(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
		Verbose:  true, // Enable verbose → should not panic
	}

	cache := WithCache(mock, config)

	// Just verify it doesn't panic with verbose on
	cache.CompleteWithUsage("test", 100)
	cache.CompleteWithUsage("test", 100) // Cache hit
}
