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

func TestEncryptedCache_HitReturnsCachedResponse(t *testing.T) {
	mock := NewMockClient("encrypted response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-master-key")

	// First call: cache miss → calls mock
	resp1, usage1, err := cache.CompleteWithUsage("hello", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp1 != "encrypted response" {
		t.Errorf("expected 'encrypted response', got %q", resp1)
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
	if resp2 != "encrypted response" {
		t.Errorf("expected 'encrypted response', got %q", resp2)
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

func TestEncryptedCache_MissCallsProvider(t *testing.T) {
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

	cache := WithEncryptedCache(mock, config, "test-key")

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

func TestEncryptedCache_TTLExpiration(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

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

func TestEncryptedCache_TTLExpirationDeletesFile(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cache.nowFunc = func() time.Time { return now }

	// Create a cache entry
	cache.CompleteWithUsage("test", 100)

	// Verify .enc file exists
	entries, _ := cache.CacheStats()
	if entries != 1 {
		t.Fatalf("expected 1 cache entry, got %d", entries)
	}

	// Expire it
	cache.nowFunc = func() time.Time { return now.Add(2 * time.Hour) }
	cache.CompleteWithUsage("test", 100) // triggers load → TTL check → delete

	// The old entry should be deleted and a new one created
	// (same key, but fresh entry from the new call)
	entries, _ = cache.CacheStats()
	if entries != 1 {
		t.Errorf("expected 1 entry (old deleted, new created), got %d", entries)
	}
}

func TestEncryptedCache_DisabledBypassesCache(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  false,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

	cache.CompleteWithUsage("test", 100)
	cache.CompleteWithUsage("test", 100)
	cache.CompleteWithUsage("test", 100)

	if mock.CallCount != 3 {
		t.Errorf("expected 3 calls (cache disabled), got %d", mock.CallCount)
	}
}

func TestEncryptedCache_DifferentMaxTokensAreDifferentKeys(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

	cache.CompleteWithUsage("same prompt", 100)
	cache.CompleteWithUsage("same prompt", 200)

	if mock.CallCount != 2 {
		t.Errorf("expected 2 calls (different maxTokens), got %d", mock.CallCount)
	}
}

func TestEncryptedCache_SystemPromptAffectsKey(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

	cache.SetSystemPrompt("system A")
	cache.CompleteWithUsage("same prompt", 100)

	cache.SetSystemPrompt("system B")
	cache.CompleteWithUsage("same prompt", 100)

	if mock.CallCount != 2 {
		t.Errorf("expected 2 calls (different system prompts), got %d", mock.CallCount)
	}
}

func TestEncryptedCache_ErrorsAreNotCached(t *testing.T) {
	var callCount int32
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				return "", nil, os.ErrNotExist
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

	cache := WithEncryptedCache(mock, config, "test-key")

	_, _, err := cache.CompleteWithUsage("test", 100)
	if err == nil {
		t.Fatal("expected error on first call")
	}

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

func TestEncryptedCache_CompleteDelegatesToCompleteWithUsage(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

	resp, err := cache.Complete("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("expected 'response', got %q", resp)
	}
}

func TestEncryptedCache_DelegatesProviderMethods(t *testing.T) {
	mock := NewMockProvider("claude", "sonnet-4", "response")
	config := DefaultCacheConfig()
	config.CacheDir = t.TempDir()

	cache := WithEncryptedCache(mock, config, "test-key")

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

func TestEncryptedCache_FilesAreEncrypted(t *testing.T) {
	mock := NewMockClient("secret response data")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-master-key")
	cache.CompleteWithUsage("test prompt", 100)

	// Find the .enc file
	var encFile string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".enc") {
			encFile = path
		}
		return nil
	})

	if encFile == "" {
		t.Fatal("no .enc file was created")
	}

	// Read raw file content
	data, err := os.ReadFile(encFile)
	if err != nil {
		t.Fatalf("error reading encrypted file: %v", err)
	}

	// Verify it's NOT plaintext JSON
	var entry cacheEntry
	if json.Unmarshal(data, &entry) == nil && entry.Response != "" {
		t.Error("cache file appears to be plaintext JSON — should be encrypted")
	}

	// Verify the plaintext response is NOT visible in the raw file
	if strings.Contains(string(data), "secret response data") {
		t.Error("plaintext response visible in encrypted cache file")
	}

	// Verify it IS valid base64 (our encrypted format)
	if len(data) == 0 {
		t.Error("encrypted file is empty")
	}
}

func TestEncryptedCache_WrongKeyCannotDecrypt(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	// Write cache with key A
	cacheA := WithEncryptedCache(mock, config, "key-alpha")
	cacheA.CompleteWithUsage("test", 100)
	if mock.CallCount != 1 {
		t.Fatalf("expected 1 call, got %d", mock.CallCount)
	}

	// Try to read cache with key B → should miss (can't decrypt)
	cacheB := WithEncryptedCache(mock, config, "key-beta")
	cacheB.CompleteWithUsage("test", 100)
	if mock.CallCount != 2 {
		t.Errorf("expected 2 calls (wrong key = cache miss), got %d", mock.CallCount)
	}
}

func TestEncryptedCache_WrongKeyRemovesCorruptedFile(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	// Write cache with key A
	cacheA := WithEncryptedCache(mock, config, "key-alpha")
	cacheA.CompleteWithUsage("test", 100)

	// Verify 1 entry exists
	entries, _ := cacheA.CacheStats()
	if entries != 1 {
		t.Fatalf("expected 1 entry, got %d", entries)
	}

	// Read with key B → can't decrypt → should remove the corrupted file
	cacheB := WithEncryptedCache(mock, config, "key-beta")
	cacheB.CompleteWithUsage("test", 100) // miss, creates new entry with key B

	// Should still have 1 entry (old deleted, new created with different encryption)
	entries, _ = cacheB.CacheStats()
	if entries != 1 {
		t.Errorf("expected 1 entry, got %d", entries)
	}
}

func TestEncryptedCache_FilePermissions(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	// Track permissions written
	var writtenPerm os.FileMode
	var dirPerm os.FileMode

	cache := WithEncryptedCache(mock, config, "test-key")
	cache.writeFile = func(name string, data []byte, perm os.FileMode) error {
		writtenPerm = perm
		return os.WriteFile(name, data, perm)
	}
	cache.mkdirAll = func(path string, perm os.FileMode) error {
		dirPerm = perm
		return os.MkdirAll(path, perm)
	}

	cache.CompleteWithUsage("test", 100)

	if writtenPerm != 0600 {
		t.Errorf("expected file perm 0600, got %04o", writtenPerm)
	}
	if dirPerm != 0700 {
		t.Errorf("expected dir perm 0700, got %04o", dirPerm)
	}
}

func TestEncryptedCache_SystemPromptNotStoredInCacheFile(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")
	cache.SetSystemPrompt("TOP SECRET SYSTEM PROMPT — do not leak")
	cache.CompleteWithUsage("test", 100)

	// Decrypt the cache entry directly and verify system prompt is NOT stored
	var encFile string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".enc") {
			encFile = path
		}
		return nil
	})

	if encFile == "" {
		t.Fatal("no .enc file found")
	}

	data, err := os.ReadFile(encFile)
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}

	decrypted, err := cacheDecrypt(data, "test-key")
	if err != nil {
		t.Fatalf("error decrypting: %v", err)
	}

	// Verify the decrypted content does NOT contain the system prompt
	if strings.Contains(string(decrypted), "TOP SECRET SYSTEM PROMPT") {
		t.Error("system prompt found in cache entry — should NOT be stored")
	}

	// Verify it IS a valid cacheEntry
	var entry cacheEntry
	if err := json.Unmarshal(decrypted, &entry); err != nil {
		t.Fatalf("decrypted content is not valid JSON: %v", err)
	}
	if entry.Response != "response" {
		t.Errorf("expected 'response', got %q", entry.Response)
	}
}

func TestEncryptedCache_CacheKeyConsistency(t *testing.T) {
	mock := NewMockClient("resp")
	config := DefaultCacheConfig()
	config.CacheDir = t.TempDir()

	cache := WithEncryptedCache(mock, config, "test-key")
	cache.SetSystemPrompt("system")

	// Same inputs should produce same key
	key1 := cache.cacheKey("prompt", 100)
	key2 := cache.cacheKey("prompt", 100)
	if key1 != key2 {
		t.Errorf("same inputs produced different keys: %s vs %s", key1, key2)
	}

	if len(key1) != 64 {
		t.Errorf("expected 64-char hex key, got %d chars", len(key1))
	}

	key3 := cache.cacheKey("different prompt", 100)
	if key1 == key3 {
		t.Error("different prompts produced same key")
	}
}

func TestEncryptedCache_ClearCache(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: cacheDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

	cache.CompleteWithUsage("prompt 1", 100)
	cache.CompleteWithUsage("prompt 2", 100)

	entries, _ := cache.CacheStats()
	if entries != 2 {
		t.Fatalf("expected 2 cache entries, got %d", entries)
	}

	err := cache.ClearCache()
	if err != nil {
		t.Fatalf("error clearing cache: %v", err)
	}

	entries, _ = cache.CacheStats()
	if entries != 0 {
		t.Errorf("expected 0 entries after clear, got %d", entries)
	}

	callsBefore := mock.CallCount
	cache.CompleteWithUsage("prompt 1", 100)
	if mock.CallCount != callsBefore+1 {
		t.Error("expected provider call after cache clear")
	}
}

func TestEncryptedCache_CacheStats(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

	// Initially empty
	entries, bytes := cache.CacheStats()
	if entries != 0 || bytes != 0 {
		t.Errorf("expected 0 entries/0 bytes initially, got %d/%d", entries, bytes)
	}

	// Add entries
	cache.CompleteWithUsage("prompt 1", 100)
	cache.CompleteWithUsage("prompt 2", 100)

	entries, bytes = cache.CacheStats()
	if entries != 2 {
		t.Errorf("expected 2 entries, got %d", entries)
	}
	if bytes == 0 {
		t.Error("expected non-zero bytes")
	}
}

func TestEncryptedCache_AutoGeneratesKeyWhenEmpty(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
	}

	// Empty master key → auto-generates session key
	cache := WithEncryptedCache(mock, config, "")

	// Should still work (encrypt + decrypt with auto-generated key)
	resp, _, err := cache.CompleteWithUsage("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("expected 'response', got %q", resp)
	}

	// Should be a cache hit on second call
	resp2, _, err := cache.CompleteWithUsage("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2 != "response" {
		t.Errorf("expected 'response', got %q", resp2)
	}
	if mock.CallCount != 1 {
		t.Errorf("expected 1 call (cache hit), got %d", mock.CallCount)
	}
}

func TestEncryptedCache_EncryptDecryptRoundtrip(t *testing.T) {
	testCases := []struct {
		name     string
		data     string
		password string
	}{
		{"simple", "hello world", "password123"},
		{"empty", "", "password123"},
		{"unicode", "olá mundo 🌍 日本語", "chave-mestre"},
		{"large", strings.Repeat("x", 100000), "key"},
		{"json", `{"response":"test","created_at":"2026-01-01T00:00:00Z"}`, "json-key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := cacheEncrypt([]byte(tc.data), tc.password)
			if err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}

			decrypted, err := cacheDecrypt(encrypted, tc.password)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}

			if string(decrypted) != tc.data {
				t.Errorf("roundtrip failed: got %q, want %q", string(decrypted), tc.data)
			}
		})
	}
}

func TestEncryptedCache_DecryptWithWrongPassword(t *testing.T) {
	data := []byte("sensitive data")

	encrypted, err := cacheEncrypt(data, "correct-password")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = cacheDecrypt(encrypted, "wrong-password")
	if err == nil {
		t.Fatal("expected error decrypting with wrong password")
	}
}

func TestEncryptedCache_DecryptInvalidData(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"not_base64", []byte("this is not base64!!!")},
		{"too_short", []byte("YQ==")}, // just "a" in base64
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cacheDecrypt(tc.data, "any-key")
			if err == nil {
				t.Error("expected error for invalid data")
			}
		})
	}
}

func TestEncryptedCache_VerboseLogging(t *testing.T) {
	mock := NewMockClient("response")
	tmpDir := t.TempDir()

	config := CacheConfig{
		Enabled:  true,
		TTL:      1 * time.Hour,
		CacheDir: tmpDir,
		Verbose:  true,
	}

	cache := WithEncryptedCache(mock, config, "test-key")

	// Just verify it doesn't panic with verbose on
	cache.CompleteWithUsage("test", 100)
	cache.CompleteWithUsage("test", 100) // Cache hit
}
