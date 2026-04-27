package llm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptedCacheProvider wraps an LLMProvider with AES-256-GCM encrypted disk cache.
// Drop-in replacement for CacheProvider with encrypted storage.
//
// Security properties:
// - All cache entries encrypted with AES-256-GCM
// - File permissions 0600 (owner-only read/write)
// - Directory permissions 0700 (owner-only)
// - System prompt is NOT stored in cache entries
// - TTL enforced on encrypted entries (decrypt, check timestamp, delete if expired)
// - Master key derived via PBKDF2 (100K iterations)
type EncryptedCacheProvider struct {
	inner  LLMProvider
	config CacheConfig

	// Master key for encryption (from WTB_MASTER_KEY or auto-generated)
	masterKey string

	// Funções injetáveis para testes
	nowFunc   func() time.Time
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
	stat      func(string) (os.FileInfo, error)
	removeF   func(string) error

	// System prompt atual (para cache key, but NOT stored)
	systemPrompt string
}

const cacheKeyIterations = 100000

// WithEncryptedCache wraps an LLMProvider with encrypted disk cache.
// If masterKey is empty, tries WTB_MASTER_KEY env var, then auto-generates.
func WithEncryptedCache(provider LLMProvider, config CacheConfig, masterKey string) *EncryptedCacheProvider {
	if masterKey == "" {
		masterKey = os.Getenv("WTB_MASTER_KEY")
	}
	if masterKey == "" {
		// Auto-generate a session-only key (cache is ephemeral if no master key)
		masterKey = generateSessionKey()
	}

	return &EncryptedCacheProvider{
		inner:     provider,
		config:    config,
		masterKey: masterKey,
		nowFunc:   time.Now,
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		mkdirAll:  os.MkdirAll,
		stat:      os.Stat,
		removeF:   os.Remove,
	}
}

// ProviderName delegates to inner provider
func (c *EncryptedCacheProvider) ProviderName() string {
	return c.inner.ProviderName()
}

// ModelID delegates to inner provider
func (c *EncryptedCacheProvider) ModelID() string {
	return c.inner.ModelID()
}

// SetSystemPrompt stores and delegates to inner provider
func (c *EncryptedCacheProvider) SetSystemPrompt(system string) {
	c.systemPrompt = system
	c.inner.SetSystemPrompt(system)
}

// Complete implements Completer with encrypted cache
func (c *EncryptedCacheProvider) Complete(prompt string, maxTokens int) (string, error) {
	response, _, err := c.CompleteWithUsage(prompt, maxTokens)
	return response, err
}

// CompleteWithUsage implements Completer with encrypted disk cache.
// Cache key = SHA256(system_prompt + prompt + maxTokens + provider + model).
// Returns cached response if available, within TTL, and decryptable.
func (c *EncryptedCacheProvider) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	if !c.config.Enabled {
		return c.inner.CompleteWithUsage(prompt, maxTokens)
	}

	key := c.cacheKey(prompt, maxTokens)
	cachePath := c.cachePath(key)

	// Check cache hit
	if entry, ok := c.loadFromCache(cachePath); ok {
		if c.config.Verbose {
			log.Printf("[encrypted-cache] HIT for %s/%s (key=%s)", c.inner.ProviderName(), c.inner.ModelID(), key[:12])
		}
		cachedUsage := &Usage{}
		if entry.Usage != nil {
			cachedUsage.InputTokens = entry.Usage.InputTokens
			cachedUsage.OutputTokens = entry.Usage.OutputTokens
			cachedUsage.TotalTokens = entry.Usage.TotalTokens
			cachedUsage.Cost = 0 // Cache hit has no cost
		}
		return entry.Response, cachedUsage, nil
	}

	if c.config.Verbose {
		log.Printf("[encrypted-cache] MISS for %s/%s (key=%s)", c.inner.ProviderName(), c.inner.ModelID(), key[:12])
	}

	// Cache miss: call inner provider
	response, usage, err := c.inner.CompleteWithUsage(prompt, maxTokens)
	if err != nil {
		return response, usage, err
	}

	// Save encrypted cache entry (fire-and-forget)
	c.saveToCache(cachePath, response, usage)

	return response, usage, nil
}

// cacheKey generates a unique key based on LLM call inputs (same as CacheProvider)
func (c *EncryptedCacheProvider) cacheKey(prompt string, maxTokens int) string {
	h := sha256.New()
	h.Write([]byte(c.systemPrompt))
	h.Write([]byte("\x00"))
	h.Write([]byte(prompt))
	h.Write([]byte("\x00"))
	h.Write([]byte(fmt.Sprintf("%d", maxTokens)))
	h.Write([]byte("\x00"))
	h.Write([]byte(c.inner.ProviderName()))
	h.Write([]byte("\x00"))
	h.Write([]byte(c.inner.ModelID()))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// cachePath returns the encrypted cache file path
func (c *EncryptedCacheProvider) cachePath(key string) string {
	return filepath.Join(c.config.CacheDir, key[:2], key+".enc")
}

// loadFromCache decrypts and loads a cache entry
func (c *EncryptedCacheProvider) loadFromCache(path string) (*cacheEntry, bool) {
	data, err := c.readFile(path)
	if err != nil {
		return nil, false // File doesn't exist or can't be read
	}

	// Decrypt
	decrypted, err := cacheDecrypt(data, c.masterKey)
	if err != nil {
		// Corrupted or wrong key — remove file silently
		c.removeF(path)
		return nil, false
	}

	var entry cacheEntry
	if err := json.Unmarshal(decrypted, &entry); err != nil {
		return nil, false
	}

	// Check TTL
	if c.nowFunc().Sub(entry.CreatedAt) > c.config.TTL {
		// Expired — remove file
		c.removeF(path)
		return nil, false
	}

	return &entry, true
}

// saveToCache encrypts and saves a cache entry
func (c *EncryptedCacheProvider) saveToCache(path string, response string, usage *Usage) {
	// NOTE: system prompt is NOT stored in the cache entry — only in the cache key hash.
	// This prevents leaking system prompt content if the cache is compromised.
	entry := cacheEntry{
		Response:  response,
		Usage:     usage,
		CreatedAt: c.nowFunc(),
		Provider:  c.inner.ProviderName(),
		Model:     c.inner.ModelID(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	// Encrypt
	encrypted, err := cacheEncrypt(data, c.masterKey)
	if err != nil {
		return
	}

	// Ensure directory exists with secure permissions
	dir := filepath.Dir(path)
	if err := c.mkdirAll(dir, 0700); err != nil {
		return
	}

	// Write encrypted file with secure permissions
	c.writeFile(path, encrypted, 0600)
}

// CacheStats returns cache statistics
func (c *EncryptedCacheProvider) CacheStats() (entries int, totalBytes int64) {
	filepath.Walk(c.config.CacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".enc" {
			entries++
			totalBytes += info.Size()
		}
		return nil
	})
	return
}

// ClearCache removes all cache entries
func (c *EncryptedCacheProvider) ClearCache() error {
	return os.RemoveAll(c.config.CacheDir)
}

// --- Self-contained AES-256-GCM encryption for cache ---
// Matches the format used by pkg/credentials/encrypted_file_provider.go

// cacheEncrypt encrypts data with AES-256-GCM using PBKDF2 key derivation.
// Format: base64( salt(32) + nonce + ciphertext + tag )
func cacheEncrypt(data []byte, password string) ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	key := pbkdf2.Key([]byte(password), salt, cacheKeyIterations, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	result := append(salt, ciphertext...)

	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(result)))
	base64.StdEncoding.Encode(encoded, result)

	return encoded, nil
}

// cacheDecrypt decrypts data encrypted by cacheEncrypt.
func cacheDecrypt(data []byte, password string) ([]byte, error) {
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(decoded, data)
	if err != nil {
		return nil, err
	}
	decoded = decoded[:n]

	if len(decoded) < 32 {
		return nil, fmt.Errorf("invalid encrypted data")
	}
	salt := decoded[:32]
	ciphertext := decoded[32:]

	key := pbkdf2.Key([]byte(password), salt, cacheKeyIterations, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("invalid ciphertext")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// generateSessionKey creates a random session key for ephemeral cache
func generateSessionKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}
