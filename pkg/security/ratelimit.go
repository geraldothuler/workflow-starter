package security

import (
	"fmt"
	"sync"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

// RateLimiter implementa token bucket algorithm
type RateLimiter struct {
	provider        llm.Provider
	maxTokens       int           // Tokens máximos no bucket
	refillRate      int           // Tokens por minuto
	currentTokens   int           // Tokens disponíveis
	lastRefill      time.Time     // Última recarga
	mu              sync.Mutex    // Thread safety
	warningAt       int           // Avisar quando restar X%
	requestCount    int           // Contador de requests
	costAccumulated float64       // Custo acumulado
}

// NewRateLimiter cria novo rate limiter
func NewRateLimiter(provider llm.Provider) *RateLimiter {
	maxTokens, refillRate := getProviderLimits(provider)
	
	return &RateLimiter{
		provider:      provider,
		maxTokens:     maxTokens,
		refillRate:    refillRate,
		currentTokens: maxTokens, // Começa cheio
		lastRefill:    time.Now(),
		warningAt:     20, // Avisar quando 20% restante
	}
}

// getProviderLimits retorna limites por provider
func getProviderLimits(provider llm.Provider) (maxTokens, refillRate int) {
	switch provider {
	case llm.ProviderClaude:
		// Anthropic: ~50 req/min
		return 50, 50
	case llm.ProviderChatGPT:
		// OpenAI (tier 1): ~500 req/min, mas vamos ser conservadores
		return 100, 100
	case llm.ProviderGemini:
		// Google: ~60 req/min
		return 60, 60
	default:
		// Default conservador
		return 30, 30
	}
}

// Wait aguarda até ter tokens disponíveis
func (rl *RateLimiter) Wait() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Recarregar tokens
	rl.refill()

	// Se não tem tokens, calcular tempo de espera
	if rl.currentTokens <= 0 {
		// Tempo para próximo token
		tokensNeeded := 1
		secondsPerToken := 60.0 / float64(rl.refillRate)
		waitTime := time.Duration(float64(tokensNeeded) * secondsPerToken * float64(time.Second))

		fmt.Printf("\n⏳ Rate limit: aguardando %v para próximo request...\n", waitTime.Round(time.Second))
		
		rl.mu.Unlock()
		time.Sleep(waitTime)
		rl.mu.Lock()
		
		// Recarregar após espera
		rl.refill()
	}

	// Consumir token
	rl.currentTokens--
	rl.requestCount++

	// Warning se poucos tokens restantes
	percentRemaining := float64(rl.currentTokens) / float64(rl.maxTokens) * 100
	if percentRemaining <= float64(rl.warningAt) && percentRemaining > 0 {
		fmt.Printf("⚠️  Rate limit: %d%% dos tokens restantes (%d/%d)\n", 
			int(percentRemaining), rl.currentTokens, rl.maxTokens)
	}

	return nil
}

// refill recarrega tokens baseado no tempo decorrido
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	// Calcular quantos tokens adicionar
	minutesElapsed := elapsed.Minutes()
	tokensToAdd := int(minutesElapsed * float64(rl.refillRate))

	if tokensToAdd > 0 {
		rl.currentTokens += tokensToAdd
		if rl.currentTokens > rl.maxTokens {
			rl.currentTokens = rl.maxTokens
		}
		rl.lastRefill = now
	}
}

// TrackCost registra custo de operação
func (rl *RateLimiter) TrackCost(cost float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.costAccumulated += cost
}

// GetStats retorna estatísticas
func (rl *RateLimiter) GetStats() Stats {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill() // Atualizar tokens antes de reportar

	return Stats{
		Provider:         string(rl.provider),
		RequestCount:     rl.requestCount,
		TokensRemaining:  rl.currentTokens,
		TokensMax:        rl.maxTokens,
		CostAccumulated:  rl.costAccumulated,
		RefillRate:       rl.refillRate,
	}
}

// Stats estatísticas do rate limiter
type Stats struct {
	Provider        string
	RequestCount    int
	TokensRemaining int
	TokensMax       int
	CostAccumulated float64
	RefillRate      int
}

// Reset reseta contador (útil para testes)
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.currentTokens = rl.maxTokens
	rl.lastRefill = time.Now()
	rl.requestCount = 0
	rl.costAccumulated = 0
}

// CostGuard protege contra custos excessivos
type CostGuard struct {
	maxCostPerRun   float64
	currentCost     float64
	warningThreshold float64
	mu              sync.Mutex
}

// NewCostGuard cria novo cost guard
func NewCostGuard(maxCostPerRun float64) *CostGuard {
	return &CostGuard{
		maxCostPerRun:    maxCostPerRun,
		warningThreshold: maxCostPerRun * 0.8, // Avisar em 80%
	}
}

// CheckCost verifica se pode gastar mais
func (cg *CostGuard) CheckCost(additionalCost float64) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	projectedCost := cg.currentCost + additionalCost

	// Hard limit
	if projectedCost > cg.maxCostPerRun {
		return fmt.Errorf("custo excederia limite: $%.4f (limite: $%.4f)\n\n"+
			"Opções:\n"+
			"1. Aumente o limite: --max-cost X\n"+
			"2. Use provider mais barato (Gemini)\n"+
			"3. Reduza escopo do projeto",
			projectedCost, cg.maxCostPerRun)
	}

	// Warning
	if projectedCost > cg.warningThreshold && cg.currentCost <= cg.warningThreshold {
		fmt.Printf("⚠️  Custo se aproximando do limite: $%.4f / $%.4f (%.0f%%)\n",
			projectedCost, cg.maxCostPerRun, 
			projectedCost/cg.maxCostPerRun*100)
	}

	cg.currentCost = projectedCost
	return nil
}

// GetCurrentCost retorna custo atual
func (cg *CostGuard) GetCurrentCost() float64 {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	return cg.currentCost
}

// Reset reseta custo
func (cg *CostGuard) Reset() {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.currentCost = 0
}

// RequestThrottler throttle mais agressivo para burst protection
type RequestThrottler struct {
	minDelay time.Duration
	lastCall time.Time
	mu       sync.Mutex
}

// NewRequestThrottler cria throttler
func NewRequestThrottler(minDelay time.Duration) *RequestThrottler {
	return &RequestThrottler{
		minDelay: minDelay,
		lastCall: time.Time{}, // Zero time
	}
}

// Throttle aguarda se necessário
func (rt *RequestThrottler) Throttle() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if !rt.lastCall.IsZero() {
		elapsed := time.Since(rt.lastCall)
		if elapsed < rt.minDelay {
			time.Sleep(rt.minDelay - elapsed)
		}
	}

	rt.lastCall = time.Now()
}
