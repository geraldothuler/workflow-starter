package security

import (
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(llm.ProviderClaude)
	if rl == nil {
		t.Fatal("expected non-nil rate limiter")
	}
	if rl.maxTokens != 50 {
		t.Errorf("expected maxTokens=50 for Claude, got %d", rl.maxTokens)
	}
}

func TestNewRateLimiter_Providers(t *testing.T) {
	tests := []struct {
		provider     llm.Provider
		expectedMax  int
	}{
		{llm.ProviderClaude, 50},
		{llm.ProviderChatGPT, 100},
		{llm.ProviderGemini, 60},
		{"unknown", 30},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			rl := NewRateLimiter(tt.provider)
			if rl.maxTokens != tt.expectedMax {
				t.Errorf("expected maxTokens=%d, got %d", tt.expectedMax, rl.maxTokens)
			}
		})
	}
}

func TestRateLimiter_GetStats(t *testing.T) {
	rl := NewRateLimiter(llm.ProviderClaude)

	stats := rl.GetStats()
	if stats.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", stats.Provider)
	}
	if stats.TokensMax != 50 {
		t.Errorf("expected TokensMax=50, got %d", stats.TokensMax)
	}
	if stats.RequestCount != 0 {
		t.Errorf("expected RequestCount=0, got %d", stats.RequestCount)
	}
}

func TestRateLimiter_TrackCost(t *testing.T) {
	rl := NewRateLimiter(llm.ProviderClaude)

	rl.TrackCost(0.01)
	rl.TrackCost(0.02)

	stats := rl.GetStats()
	if stats.CostAccumulated != 0.03 {
		t.Errorf("expected cost 0.03, got %f", stats.CostAccumulated)
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(llm.ProviderClaude)
	rl.TrackCost(1.0)
	rl.Wait()

	rl.Reset()
	stats := rl.GetStats()

	if stats.RequestCount != 0 {
		t.Errorf("expected RequestCount=0 after reset, got %d", stats.RequestCount)
	}
	if stats.CostAccumulated != 0 {
		t.Errorf("expected cost=0 after reset, got %f", stats.CostAccumulated)
	}
}

func TestNewCostGuard(t *testing.T) {
	cg := NewCostGuard(1.0)
	if cg == nil {
		t.Fatal("expected non-nil cost guard")
	}
	if cg.maxCostPerRun != 1.0 {
		t.Errorf("expected maxCost=1.0, got %f", cg.maxCostPerRun)
	}
}

func TestCostGuard_CheckCost(t *testing.T) {
	cg := NewCostGuard(1.0)

	// Under limit
	err := cg.CheckCost(0.5)
	if err != nil {
		t.Errorf("expected no error for cost under limit: %v", err)
	}

	// Over limit
	err = cg.CheckCost(1.0) // Total would be 1.5
	if err == nil {
		t.Error("expected error for cost over limit")
	}
}

func TestCostGuard_GetCurrentCost(t *testing.T) {
	cg := NewCostGuard(10.0)
	cg.CheckCost(0.5)
	cg.CheckCost(0.3)

	cost := cg.GetCurrentCost()
	if cost != 0.8 {
		t.Errorf("expected current cost 0.8, got %f", cost)
	}
}

func TestCostGuard_Reset(t *testing.T) {
	cg := NewCostGuard(1.0)
	cg.CheckCost(0.5)
	cg.Reset()

	if cg.GetCurrentCost() != 0 {
		t.Error("expected cost=0 after reset")
	}
}

func TestNewRequestThrottler(t *testing.T) {
	rt := NewRequestThrottler(100)
	if rt == nil {
		t.Fatal("expected non-nil throttler")
	}
}
