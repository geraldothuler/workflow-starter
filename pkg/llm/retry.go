package llm

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// RetryConfig configura retry com exponential backoff
type RetryConfig struct {
	MaxRetries    int           // Número máximo de retries (default: 3)
	InitialDelay  time.Duration // Delay inicial entre tentativas (default: 1s)
	MaxDelay      time.Duration // Delay máximo entre tentativas (default: 30s)
	BackoffFactor float64       // Fator multiplicador do delay (default: 2.0)
	RetryableCodes []int        // HTTP status codes retryable (default: [429, 500, 502, 503])
	Verbose       bool          // Log retry attempts
}

// DefaultRetryConfig retorna configuração padrão de retry
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialDelay:   1 * time.Second,
		MaxDelay:       30 * time.Second,
		BackoffFactor:  2.0,
		RetryableCodes: []int{429, 500, 502, 503},
		Verbose:        false,
	}
}

// RetryProvider wraps um LLMProvider com retry automático (decorator pattern)
type RetryProvider struct {
	inner  LLMProvider
	config RetryConfig

	// sleepFunc permite injetar sleep para testes (default: time.Sleep)
	sleepFunc func(time.Duration)
}

// WithRetry wraps um LLMProvider com retry automático
func WithRetry(provider LLMProvider, config RetryConfig) *RetryProvider {
	return &RetryProvider{
		inner:     provider,
		config:    config,
		sleepFunc: time.Sleep,
	}
}

// ProviderName retorna nome do provider inner
func (r *RetryProvider) ProviderName() string {
	return r.inner.ProviderName()
}

// ModelID retorna modelo do provider inner
func (r *RetryProvider) ModelID() string {
	return r.inner.ModelID()
}

// SetSystemPrompt delega para o provider inner
func (r *RetryProvider) SetSystemPrompt(system string) {
	r.inner.SetSystemPrompt(system)
}

// Complete implementa Completer com retry automático
func (r *RetryProvider) Complete(prompt string, maxTokens int) (string, error) {
	response, _, err := r.CompleteWithUsage(prompt, maxTokens)
	return response, err
}

// CompleteWithUsage implementa Completer com retry automático.
// Acumula usage de todas as tentativas (incluindo tentativas falhas que retornaram usage parcial).
func (r *RetryProvider) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	totalUsage := &Usage{}
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.calculateDelay(attempt)
			if r.config.Verbose {
				log.Printf("[retry] %s: attempt %d/%d after %v (last error: %v)",
					r.inner.ProviderName(), attempt+1, r.config.MaxRetries+1, delay, lastErr)
			}
			r.sleepFunc(delay)
		}

		response, usage, err := r.inner.CompleteWithUsage(prompt, maxTokens)

		// Acumular usage mesmo em falhas (API pode cobrar por tentativas parciais)
		if usage != nil {
			totalUsage.InputTokens += usage.InputTokens
			totalUsage.OutputTokens += usage.OutputTokens
			totalUsage.TotalTokens += usage.TotalTokens
			totalUsage.Cost += usage.Cost
		}

		if err == nil {
			return response, totalUsage, nil
		}

		lastErr = err

		// Se o erro não é retryable, retorna imediatamente
		if !r.isRetryable(err) {
			if r.config.Verbose {
				log.Printf("[retry] %s: non-retryable error, giving up: %v",
					r.inner.ProviderName(), err)
			}
			return "", totalUsage, err
		}
	}

	// Todas as tentativas falharam
	return "", totalUsage, fmt.Errorf("all %d attempts failed for %s: %w",
		r.config.MaxRetries+1, r.inner.ProviderName(), lastErr)
}

// calculateDelay calcula o delay com exponential backoff
func (r *RetryProvider) calculateDelay(attempt int) time.Duration {
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.BackoffFactor, float64(attempt-1))
	if time.Duration(delay) > r.config.MaxDelay {
		return r.config.MaxDelay
	}
	return time.Duration(delay)
}

// isRetryable verifica se um erro é retryable.
// Analisa a mensagem de erro por HTTP status codes e patterns conhecidos.
func (r *RetryProvider) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	// Check HTTP status codes na mensagem de erro
	for _, code := range r.config.RetryableCodes {
		codeStr := fmt.Sprintf("%d", code)
		if strings.Contains(msg, codeStr) {
			return true
		}
	}

	// Patterns de erro transiente conhecidos
	retryablePatterns := []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"overloaded",
		"temporarily unavailable",
		"service unavailable",
		"internal server error",
		"bad gateway",
		"gateway timeout",
		"timeout",
		"connection reset",
		"connection refused",
		"eof",
		"broken pipe",
		"no such host",        // DNS transiente
		"i/o timeout",
		"tls handshake timeout",
	}

	msgLower := strings.ToLower(msg)
	for _, pattern := range retryablePatterns {
		if strings.Contains(msgLower, pattern) {
			return true
		}
	}

	// Erros que NÃO devem retry (client-side)
	nonRetryablePatterns := []string{
		"400",           // Bad Request
		"401",           // Unauthorized
		"403",           // Forbidden
		"404",           // Not Found
		"invalid",       // Validation errors
		"unauthorized",
		"forbidden",
		"authentication",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(msgLower, pattern) {
			return false
		}
	}

	// Default: retry para erros desconhecidos (fail-safe)
	return false
}
