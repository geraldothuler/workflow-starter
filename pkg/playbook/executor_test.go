package playbook

import (
	"context"
	"fmt"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/infracontext"
)

// MockProvider implements infracontext.Provider for testing.
type MockProvider struct {
	id        string
	name      string
	available bool
	ic        *infracontext.InfraContext
	err       error
}

func (m *MockProvider) ID() string                                                           { return m.id }
func (m *MockProvider) Name() string                                                         { return m.name }
func (m *MockProvider) Available() bool                                                      { return m.available }
func (m *MockProvider) Fetch(_ context.Context, _ infracontext.FetchOptions) (*infracontext.InfraContext, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ic, nil
}

func newTestRegistry(providers ...*MockProvider) *infracontext.Registry {
	reg := infracontext.NewRegistry()
	for _, p := range providers {
		reg.Register(p)
	}
	return reg
}

func TestExecutor_RequiredProviderMissing(t *testing.T) {
	reg := newTestRegistry()
	exec := NewExecutor(reg)

	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test",
		RequiredProviders: []ProviderRef{{ID: "postgresql"}},
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
		},
	}

	_, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err == nil {
		t.Fatal("expected error for missing required provider")
	}
}

func TestExecutor_RequiredProviderUnavailable(t *testing.T) {
	reg := newTestRegistry(&MockProvider{id: "postgresql", name: "PG", available: false})
	exec := NewExecutor(reg)

	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test",
		RequiredProviders: []ProviderRef{{ID: "postgresql"}},
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
		},
	}

	_, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err == nil {
		t.Fatal("expected error for unavailable required provider")
	}
}

func TestExecutor_OptionalStepSkipped(t *testing.T) {
	pgProvider := &MockProvider{
		id: "postgresql", name: "PG", available: true,
		ic: &infracontext.InfraContext{Provider: "postgresql"},
	}
	reg := newTestRegistry(pgProvider)
	exec := NewExecutor(reg)

	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test",
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
			{ID: "s2", Provider: "kafka", Optional: true, Analyzers: []AnalyzerRef{{Name: "analyze_failed_connectors"}}},
		},
	}

	report, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.StepsExecuted != 1 {
		t.Errorf("steps executed = %d, want 1", report.StepsExecuted)
	}
	if report.StepsSkipped != 1 {
		t.Errorf("steps skipped = %d, want 1", report.StepsSkipped)
	}
	if len(report.StepResults) != 2 {
		t.Errorf("step results = %d, want 2", len(report.StepResults))
	}
	if report.StepResults[1].Status != StepStatusSkipped {
		t.Errorf("step 2 status = %q, want skipped", report.StepResults[1].Status)
	}
}

func TestExecutor_SingleStep(t *testing.T) {
	pgProvider := &MockProvider{
		id: "postgresql", name: "PG", available: true,
		ic: &infracontext.InfraContext{
			Provider: "postgresql",
			Health: []infracontext.HealthCheck{
				{Component: "test_slot", Kind: "ReplicationSlot", Status: infracontext.HealthStatusUnhealthy},
			},
		},
	}
	reg := newTestRegistry(pgProvider)
	exec := NewExecutor(reg)

	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test",
		RequiredProviders: []ProviderRef{{ID: "postgresql"}},
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
		},
	}

	report, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.StepsExecuted != 1 {
		t.Errorf("steps executed = %d, want 1", report.StepsExecuted)
	}
	if len(report.Findings) != 1 {
		t.Errorf("findings = %d, want 1", len(report.Findings))
	}
	if report.Findings[0].Severity != SeverityCritical {
		t.Errorf("finding severity = %q, want critical", report.Findings[0].Severity)
	}
	if report.Summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestExecutor_MultiStep(t *testing.T) {
	pgProvider := &MockProvider{
		id: "postgresql", name: "PG", available: true,
		ic: &infracontext.InfraContext{
			Provider: "postgresql",
			Health: []infracontext.HealthCheck{
				{Component: "cdc_slot", Kind: "ReplicationSlot", Status: infracontext.HealthStatusUnhealthy},
			},
		},
	}
	kafkaProvider := &MockProvider{
		id: "kafka", name: "Kafka", available: true,
		ic: &infracontext.InfraContext{
			Provider: "kafka",
			Health: []infracontext.HealthCheck{
				{Component: "cdc-source", Kind: "KafkaConnector", Status: infracontext.HealthStatusUnhealthy, Message: "task failed"},
			},
		},
	}
	reg := newTestRegistry(pgProvider, kafkaProvider)
	exec := NewExecutor(reg)

	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test Multi-Step",
		RequiredProviders: []ProviderRef{{ID: "postgresql"}},
		OptionalProviders: []ProviderRef{{ID: "kafka"}},
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
			{ID: "s2", Provider: "kafka", Analyzers: []AnalyzerRef{{Name: "analyze_failed_connectors"}}},
		},
	}

	report, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.StepsExecuted != 2 {
		t.Errorf("steps executed = %d, want 2", report.StepsExecuted)
	}
	if len(report.Findings) != 2 {
		t.Errorf("findings = %d, want 2", len(report.Findings))
	}
}

func TestExecutor_CausalChain(t *testing.T) {
	pgProvider := &MockProvider{
		id: "postgresql", name: "PG", available: true,
		ic: &infracontext.InfraContext{
			Provider: "postgresql",
			Health: []infracontext.HealthCheck{
				{Component: "slot1", Kind: "ReplicationSlot", Status: infracontext.HealthStatusUnhealthy},
			},
		},
	}
	kafkaProvider := &MockProvider{
		id: "kafka", name: "Kafka", available: true,
		ic: &infracontext.InfraContext{
			Provider: "kafka",
			Health: []infracontext.HealthCheck{
				{Component: "connector1", Kind: "KafkaConnector", Status: infracontext.HealthStatusUnhealthy},
			},
		},
	}
	reg := newTestRegistry(pgProvider, kafkaProvider)
	exec := NewExecutor(reg)

	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test Causal",
		RequiredProviders: []ProviderRef{{ID: "postgresql"}},
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
			{ID: "s2", Provider: "kafka", Analyzers: []AnalyzerRef{{Name: "analyze_failed_connectors"}}},
		},
	}

	report, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.CausalChain) == 0 {
		t.Error("expected causal chain links (inactive_slots -> failed_connectors)")
	}
}

func TestExecutor_ReportComplete(t *testing.T) {
	pgProvider := &MockProvider{
		id: "postgresql", name: "PG", available: true,
		ic: &infracontext.InfraContext{Provider: "postgresql"},
	}
	reg := newTestRegistry(pgProvider)
	exec := NewExecutor(reg)

	spec := &PlaybookSpec{
		ID:    "test-complete",
		Title: "Test Complete Report",
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
		},
	}

	report, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.PlaybookID != "test-complete" {
		t.Errorf("playbook id = %q", report.PlaybookID)
	}
	if report.PlaybookTitle != "Test Complete Report" {
		t.Errorf("playbook title = %q", report.PlaybookTitle)
	}
	if report.StartedAt.IsZero() {
		t.Error("started_at should not be zero")
	}
	if report.CompletedAt.IsZero() {
		t.Error("completed_at should not be zero")
	}
	if report.Duration <= 0 {
		t.Error("duration should be positive")
	}
	if report.Markdown == "" {
		t.Error("markdown should not be empty")
	}
}

func TestExecutor_FetchError(t *testing.T) {
	pgProvider := &MockProvider{
		id: "postgresql", name: "PG", available: true,
		err: fmt.Errorf("connection refused"),
	}
	reg := newTestRegistry(pgProvider)
	exec := NewExecutor(reg)

	// Non-optional step with fetch error
	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test",
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
		},
	}

	report, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err != nil {
		t.Fatalf("non-optional fetch error should not fail entire execution: %v", err)
	}
	if report.StepResults[0].Status != StepStatusError {
		t.Errorf("step status = %q, want error", report.StepResults[0].Status)
	}
}

func TestExecutor_OptionalFetchError(t *testing.T) {
	pgProvider := &MockProvider{
		id: "postgresql", name: "PG", available: true,
		err: fmt.Errorf("timeout"),
	}
	reg := newTestRegistry(pgProvider)
	exec := NewExecutor(reg)

	spec := &PlaybookSpec{
		ID:    "test",
		Title: "Test",
		Steps: []PlaybookStep{
			{ID: "s1", Provider: "postgresql", Optional: true, Analyzers: []AnalyzerRef{{Name: "analyze_inactive_slots"}}},
		},
	}

	report, err := exec.Execute(context.Background(), spec, ExecuteOptions{})
	if err != nil {
		t.Fatalf("optional fetch error should be skipped: %v", err)
	}
	if report.StepsSkipped != 1 {
		t.Errorf("steps skipped = %d, want 1", report.StepsSkipped)
	}
}
