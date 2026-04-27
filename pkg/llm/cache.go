package llm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// CacheConfig configura cache de respostas LLM
type CacheConfig struct {
	Enabled  bool          // Se o cache está habilitado (default: true)
	TTL      time.Duration // Tempo de vida das entradas (default: 24h)
	CacheDir string        // Diretório de cache (default: .workflow/cache/llm)
	Verbose  bool          // Log cache hits/misses
}

// DefaultCacheConfig retorna configuração padrão de cache
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:  true,
		TTL:      24 * time.Hour,
		CacheDir: filepath.Join(".workflow", "cache", "llm"),
		Verbose:  false,
	}
}

// cacheEntry representa uma entrada no cache
type cacheEntry struct {
	Response  string    `json:"response"`
	Usage     *Usage    `json:"usage,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
}

// CacheProvider wraps um LLMProvider com cache em disco (decorator pattern)
type CacheProvider struct {
	inner  LLMProvider
	config CacheConfig

	// Funções injetáveis para testes
	nowFunc   func() time.Time
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
	stat      func(string) (os.FileInfo, error)

	// System prompt atual (necessário para cache key)
	systemPrompt string
}

// WithCache wraps um LLMProvider com cache em disco
func WithCache(provider LLMProvider, config CacheConfig) *CacheProvider {
	return &CacheProvider{
		inner:     provider,
		config:    config,
		nowFunc:   time.Now,
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		mkdirAll:  os.MkdirAll,
		stat:      os.Stat,
	}
}

// ProviderName retorna nome do provider inner
func (c *CacheProvider) ProviderName() string {
	return c.inner.ProviderName()
}

// ModelID retorna modelo do provider inner
func (c *CacheProvider) ModelID() string {
	return c.inner.ModelID()
}

// SetSystemPrompt armazena e delega para o provider inner
func (c *CacheProvider) SetSystemPrompt(system string) {
	c.systemPrompt = system
	c.inner.SetSystemPrompt(system)
}

// Complete implementa Completer com cache
func (c *CacheProvider) Complete(prompt string, maxTokens int) (string, error) {
	response, _, err := c.CompleteWithUsage(prompt, maxTokens)
	return response, err
}

// CompleteWithUsage implementa Completer com cache em disco.
// Cache key = SHA256(system_prompt + prompt + maxTokens + provider + model).
// Retorna resposta cached se disponível e dentro do TTL.
// Em cache miss, chama o provider inner e salva o resultado.
func (c *CacheProvider) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	if !c.config.Enabled {
		return c.inner.CompleteWithUsage(prompt, maxTokens)
	}

	key := c.cacheKey(prompt, maxTokens)
	cachePath := c.cachePath(key)

	// Check cache hit
	if entry, ok := c.loadFromCache(cachePath); ok {
		if c.config.Verbose {
			log.Printf("[cache] HIT for %s/%s (key=%s)", c.inner.ProviderName(), c.inner.ModelID(), key[:12])
		}
		// Retorna usage com Cost=0 para indicar cache hit (sem custo real)
		cachedUsage := &Usage{}
		if entry.Usage != nil {
			cachedUsage.InputTokens = entry.Usage.InputTokens
			cachedUsage.OutputTokens = entry.Usage.OutputTokens
			cachedUsage.TotalTokens = entry.Usage.TotalTokens
			cachedUsage.Cost = 0 // Cache hit não tem custo
		}
		return entry.Response, cachedUsage, nil
	}

	if c.config.Verbose {
		log.Printf("[cache] MISS for %s/%s (key=%s)", c.inner.ProviderName(), c.inner.ModelID(), key[:12])
	}

	// Cache miss: chamar provider inner
	response, usage, err := c.inner.CompleteWithUsage(prompt, maxTokens)
	if err != nil {
		return response, usage, err
	}

	// Salvar no cache (fire-and-forget, erros de IO não afetam o fluxo)
	c.saveToCache(cachePath, response, usage)

	return response, usage, nil
}

// cacheKey gera uma chave única baseada nos inputs da chamada LLM
func (c *CacheProvider) cacheKey(prompt string, maxTokens int) string {
	h := sha256.New()
	h.Write([]byte(c.systemPrompt))
	h.Write([]byte("\x00")) // separator
	h.Write([]byte(prompt))
	h.Write([]byte("\x00"))
	h.Write([]byte(fmt.Sprintf("%d", maxTokens)))
	h.Write([]byte("\x00"))
	h.Write([]byte(c.inner.ProviderName()))
	h.Write([]byte("\x00"))
	h.Write([]byte(c.inner.ModelID()))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// cachePath retorna o caminho do arquivo de cache para uma chave
func (c *CacheProvider) cachePath(key string) string {
	// Usar 2 primeiros chars como subdiretório para evitar diretórios enormes
	return filepath.Join(c.config.CacheDir, key[:2], key+".json")
}

// loadFromCache tenta carregar uma entrada do cache
func (c *CacheProvider) loadFromCache(path string) (*cacheEntry, bool) {
	data, err := c.readFile(path)
	if err != nil {
		return nil, false // File doesn't exist or can't be read
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false // Invalid JSON, treat as miss
	}

	// Check TTL
	if c.nowFunc().Sub(entry.CreatedAt) > c.config.TTL {
		return nil, false // Expired
	}

	return &entry, true
}

// saveToCache salva uma entrada no cache
func (c *CacheProvider) saveToCache(path string, response string, usage *Usage) {
	entry := cacheEntry{
		Response:  response,
		Usage:     usage,
		CreatedAt: c.nowFunc(),
		Provider:  c.inner.ProviderName(),
		Model:     c.inner.ModelID(),
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return // Silently skip
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	// SECURITY: Sensitive directories use 0700 (owner-only access)
	if err := c.mkdirAll(dir, 0700); err != nil {
		return
	}

	// SECURITY: Cache files use 0600 (owner-only read/write) — they may contain LLM responses
	c.writeFile(path, data, 0600)
}

// CacheStats retorna estatísticas do cache (total de entradas, tamanho)
func (c *CacheProvider) CacheStats() (entries int, totalBytes int64) {
	filepath.Walk(c.config.CacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".json" {
			entries++
			totalBytes += info.Size()
		}
		return nil
	})
	return
}

// ClearCache remove todas as entradas do cache
func (c *CacheProvider) ClearCache() error {
	return os.RemoveAll(c.config.CacheDir)
}
