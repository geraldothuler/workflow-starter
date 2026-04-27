package llm

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_SuccessOnFirstTry(t *testing.T) {
	mock := NewMockClient("hello world")
	config := DefaultRetryConfig()

	retry := WithRetry(mock, config)

	response, usage, err := retry.CompleteWithUsage("test prompt", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response != "hello world" {
		t.Errorf("expected 'hello world', got %q", response)
	}

	if usage == nil {
		t.Fatal("expected usage to be non-nil")
	}

	if mock.CallCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	// Fails 2 times with retryable error, succeeds on 3rd attempt
	var callCount int32
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n <= 2 {
				return "", &Usage{InputTokens: 10, Cost: 0.001}, fmt.Errorf("status 429: rate limit exceeded")
			}
			return "success", &Usage{InputTokens: 100, OutputTokens: 50, Cost: 0.01}, nil
		},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond // Fast for tests
	config.MaxDelay = 10 * time.Millisecond

	retry := WithRetry(mock, config)

	response, usage, err := retry.CompleteWithUsage("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response != "success" {
		t.Errorf("expected 'success', got %q", response)
	}

	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}

	// Usage should be accumulated from all 3 attempts
	if usage.InputTokens != 120 { // 10 + 10 + 100
		t.Errorf("expected accumulated InputTokens=120, got %d", usage.InputTokens)
	}
	if usage.Cost < 0.012 { // 0.001 + 0.001 + 0.01
		t.Errorf("expected accumulated Cost >= 0.012, got %f", usage.Cost)
	}
}

func TestRetry_ExhaustedRetries(t *testing.T) {
	// All attempts fail with retryable error
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			return "", &Usage{InputTokens: 10}, fmt.Errorf("status 503: service unavailable")
		},
	}

	config := DefaultRetryConfig()
	config.MaxRetries = 2
	config.InitialDelay = 1 * time.Millisecond

	retry := WithRetry(mock, config)

	_, usage, err := retry.CompleteWithUsage("test", 100)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}

	// Error should indicate all attempts failed
	if !strings.Contains(err.Error(), "all 3 attempts failed") {
		t.Errorf("expected 'all 3 attempts failed' in error, got: %v", err)
	}

	// Usage accumulated from all 3 attempts (initial + 2 retries)
	if usage.InputTokens != 30 { // 10 * 3
		t.Errorf("expected accumulated InputTokens=30, got %d", usage.InputTokens)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	// 401 Unauthorized should NOT be retried
	var callCount int32
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			atomic.AddInt32(&callCount, 1)
			return "", nil, fmt.Errorf("status 401: unauthorized - invalid API key")
		},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond

	retry := WithRetry(mock, config)

	_, _, err := retry.CompleteWithUsage("test", 100)
	if err == nil {
		t.Fatal("expected error for non-retryable 401")
	}

	// Should only have called once (no retry for 401)
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected exactly 1 call (no retry for 401), got %d", callCount)
	}
}

func TestRetry_BackoffTiming(t *testing.T) {
	// Verify that delays increase with exponential backoff
	var delays []time.Duration
	var callCount int32

	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n <= 3 {
				return "", nil, fmt.Errorf("status 429: rate limit")
			}
			return "ok", &Usage{}, nil
		},
	}

	config := RetryConfig{
		MaxRetries:     3,
		InitialDelay:   100 * time.Millisecond,
		MaxDelay:       5 * time.Second,
		BackoffFactor:  2.0,
		RetryableCodes: []int{429},
	}

	retry := WithRetry(mock, config)
	// Inject custom sleep to capture delays
	retry.sleepFunc = func(d time.Duration) {
		delays = append(delays, d)
	}

	_, _, err := retry.CompleteWithUsage("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 delays (before attempts 2, 3, 4)
	if len(delays) != 3 {
		t.Fatalf("expected 3 delays, got %d", len(delays))
	}

	// Verify exponential backoff: 100ms, 200ms, 400ms
	expectedDelays := []time.Duration{
		100 * time.Millisecond, // 100ms * 2^0
		200 * time.Millisecond, // 100ms * 2^1
		400 * time.Millisecond, // 100ms * 2^2
	}

	for i, expected := range expectedDelays {
		if delays[i] != expected {
			t.Errorf("delay[%d]: expected %v, got %v", i, expected, delays[i])
		}
	}
}

func TestRetry_UsageAccumulation(t *testing.T) {
	// Verify that usage from failed attempts is accumulated
	var callCount int32
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			n := atomic.AddInt32(&callCount, 1)
			usage := &Usage{
				InputTokens:  int(n) * 100,
				OutputTokens: int(n) * 50,
				TotalTokens:  int(n) * 150,
				Cost:         float64(n) * 0.01,
			}
			if n <= 2 {
				return "", usage, fmt.Errorf("status 500: internal error")
			}
			return "done", usage, nil
		},
	}

	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond

	retry := WithRetry(mock, config)

	_, usage, err := retry.CompleteWithUsage("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Usage accumulated: call1(100,50,150,0.01) + call2(200,100,300,0.02) + call3(300,150,450,0.03)
	expectedInput := 100 + 200 + 300
	expectedOutput := 50 + 100 + 150
	expectedTotal := 150 + 300 + 450
	expectedCost := 0.01 + 0.02 + 0.03

	if usage.InputTokens != expectedInput {
		t.Errorf("InputTokens: expected %d, got %d", expectedInput, usage.InputTokens)
	}
	if usage.OutputTokens != expectedOutput {
		t.Errorf("OutputTokens: expected %d, got %d", expectedOutput, usage.OutputTokens)
	}
	if usage.TotalTokens != expectedTotal {
		t.Errorf("TotalTokens: expected %d, got %d", expectedTotal, usage.TotalTokens)
	}
	if usage.Cost != expectedCost {
		t.Errorf("Cost: expected %f, got %f", expectedCost, usage.Cost)
	}
}

func TestRetry_Complete_DelegatesToCompleteWithUsage(t *testing.T) {
	mock := NewMockClient("response")
	config := DefaultRetryConfig()

	retry := WithRetry(mock, config)

	response, err := retry.Complete("test", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response != "response" {
		t.Errorf("expected 'response', got %q", response)
	}
}

func TestRetry_DelegatesProviderMethods(t *testing.T) {
	mock := NewMockProvider("claude", "sonnet-4", "response")
	config := DefaultRetryConfig()

	retry := WithRetry(mock, config)

	if retry.ProviderName() != "claude" {
		t.Errorf("ProviderName: expected 'claude', got %q", retry.ProviderName())
	}
	if retry.ModelID() != "sonnet-4" {
		t.Errorf("ModelID: expected 'sonnet-4', got %q", retry.ModelID())
	}

	retry.SetSystemPrompt("test system prompt")
	if mock.GetSystemPrompt() != "test system prompt" {
		t.Errorf("SetSystemPrompt not delegated correctly")
	}
}

func TestRetry_MaxDelayRespected(t *testing.T) {
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			return "", nil, fmt.Errorf("status 429: rate limit")
		},
	}

	config := RetryConfig{
		MaxRetries:     5,
		InitialDelay:   1 * time.Second,
		MaxDelay:       3 * time.Second,
		BackoffFactor:  10.0, // Aggressive backoff
		RetryableCodes: []int{429},
	}

	var delays []time.Duration
	retry := WithRetry(mock, config)
	retry.sleepFunc = func(d time.Duration) {
		delays = append(delays, d)
	}

	retry.CompleteWithUsage("test", 100)

	// All delays should be <= MaxDelay
	for i, d := range delays {
		if d > config.MaxDelay {
			t.Errorf("delay[%d]=%v exceeds MaxDelay=%v", i, d, config.MaxDelay)
		}
	}
}

func TestRetry_IsRetryable(t *testing.T) {
	retry := WithRetry(nil, DefaultRetryConfig())

	tests := []struct {
		err       error
		retryable bool
		desc      string
	}{
		{nil, false, "nil error"},
		{errors.New("status 429: rate limit exceeded"), true, "429 rate limit"},
		{errors.New("status 500: internal server error"), true, "500 internal"},
		{errors.New("status 502: bad gateway"), true, "502 bad gateway"},
		{errors.New("status 503: service unavailable"), true, "503 service unavailable"},
		{errors.New("connection timeout"), true, "timeout"},
		{errors.New("connection reset by peer"), true, "connection reset"},
		{errors.New("EOF"), true, "EOF"},
		{errors.New("overloaded_error: API is overloaded"), true, "overloaded"},
		{errors.New("status 401: unauthorized"), false, "401 unauthorized"},
		{errors.New("status 400: bad request"), false, "400 bad request"},
		{errors.New("status 403: forbidden"), false, "403 forbidden"},
		{errors.New("invalid API key"), false, "invalid key"},
		{errors.New("some unknown error"), false, "unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := retry.isRetryable(tt.err)
			if result != tt.retryable {
				t.Errorf("isRetryable(%q) = %v, want %v", tt.err, result, tt.retryable)
			}
		})
	}
}
