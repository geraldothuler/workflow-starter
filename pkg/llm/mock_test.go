package llm

import (
	"errors"
	"testing"
)

func TestMockClient_Complete(t *testing.T) {
	mock := NewMockClient("response1", "response2", "response3")

	r1, err := mock.Complete("prompt1", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1 != "response1" {
		t.Errorf("expected 'response1', got %q", r1)
	}

	r2, err := mock.Complete("prompt2", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2 != "response2" {
		t.Errorf("expected 'response2', got %q", r2)
	}

	r3, err := mock.Complete("prompt3", 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r3 != "response3" {
		t.Errorf("expected 'response3', got %q", r3)
	}
}

func TestMockClient_CompleteWithUsage(t *testing.T) {
	mock := NewMockClient("response")

	response, usage, err := mock.CompleteWithUsage("test prompt", 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "response" {
		t.Errorf("expected 'response', got %q", response)
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	if usage.InputTokens != 100 {
		t.Errorf("expected InputTokens=100, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 50 {
		t.Errorf("expected OutputTokens=50, got %d", usage.OutputTokens)
	}
}

func TestMockClient_Error(t *testing.T) {
	expectedErr := errors.New("mock LLM error")
	mock := NewMockClientWithError(expectedErr)

	_, err := mock.Complete("prompt", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestMockClient_CallTracking(t *testing.T) {
	mock := NewMockClient("r1", "r2")

	mock.Complete("first prompt", 100)
	mock.Complete("second prompt", 200)

	if mock.CallCount != 2 {
		t.Errorf("expected CallCount=2, got %d", mock.CallCount)
	}
	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(mock.Calls))
	}
	if mock.Calls[0].Prompt != "first prompt" {
		t.Errorf("expected first prompt, got %q", mock.Calls[0].Prompt)
	}
	if mock.Calls[0].MaxTokens != 100 {
		t.Errorf("expected MaxTokens=100, got %d", mock.Calls[0].MaxTokens)
	}
	if mock.Calls[1].Prompt != "second prompt" {
		t.Errorf("expected second prompt, got %q", mock.Calls[1].Prompt)
	}
}

func TestMockClient_CompleteFunc(t *testing.T) {
	mock := &MockClient{
		CompleteFunc: func(prompt string, maxTokens int) (string, *Usage, error) {
			return "custom:" + prompt, &Usage{InputTokens: 42}, nil
		},
	}

	response, usage, err := mock.CompleteWithUsage("hello", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "custom:hello" {
		t.Errorf("expected 'custom:hello', got %q", response)
	}
	if usage.InputTokens != 42 {
		t.Errorf("expected InputTokens=42, got %d", usage.InputTokens)
	}
}

func TestMockClient_ExceedsResponses(t *testing.T) {
	mock := NewMockClient("only-response")

	r1, _ := mock.Complete("p1", 100)
	r2, _ := mock.Complete("p2", 100)

	if r1 != "only-response" {
		t.Errorf("expected 'only-response', got %q", r1)
	}
	// Should repeat last response when exhausted
	if r2 != "only-response" {
		t.Errorf("expected 'only-response' (repeated), got %q", r2)
	}
}

func TestMockClient_ImplementsCompleter(t *testing.T) {
	// Compile-time check that MockClient implements Completer
	var _ Completer = &MockClient{}
}

func TestClient_ImplementsCompleter(t *testing.T) {
	// Compile-time check that Client implements Completer
	var _ Completer = &Client{}
}
