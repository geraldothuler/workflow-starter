package examples

import (
	"strings"
	"testing"
)

func TestGetEventSourcingExample_ByDomain(t *testing.T) {
	ex := GetEventSourcingExample("Telemetria")
	if ex == nil {
		t.Fatal("expected non-nil example")
	}
	if ex.Domain != "Telemetria" {
		t.Errorf("expected domain 'Telemetria', got %q", ex.Domain)
	}
	if ex.Details["Volume"] == "" {
		t.Error("expected Volume in details")
	}
}

func TestGetEventSourcingExample_Default(t *testing.T) {
	ex := GetEventSourcingExample("NonExistent")
	if ex == nil {
		t.Fatal("expected non-nil default example")
	}
	// Should return first example
	if ex.Domain != "Telemetria" {
		t.Errorf("expected default domain 'Telemetria', got %q", ex.Domain)
	}
}

func TestGetStackExample_Backend(t *testing.T) {
	ex := GetStackExample("backend")
	if ex == nil {
		t.Fatal("expected non-nil example")
	}
	if !strings.Contains(ex.Description, "Go") {
		t.Error("backend should mention Go")
	}
}

func TestGetStackExample_AllTypes(t *testing.T) {
	types := []string{"backend", "frontend", "mobile", "iot"}
	for _, typ := range types {
		ex := GetStackExample(typ)
		if ex == nil {
			t.Errorf("expected example for type %q", typ)
		}
	}
}

func TestGetStackExample_NonExistent(t *testing.T) {
	ex := GetStackExample("nonexistent")
	if ex != nil {
		t.Error("expected nil for non-existent stack")
	}
}

func TestGetNFRExample_ByDomain(t *testing.T) {
	ex := GetNFRExample("Performance")
	if ex == nil {
		t.Fatal("expected non-nil example")
	}
	if ex.Domain != "Performance" {
		t.Errorf("expected domain 'Performance', got %q", ex.Domain)
	}
}

func TestGetNFRExample_Default(t *testing.T) {
	ex := GetNFRExample("NonExistent")
	if ex == nil {
		t.Fatal("expected non-nil default example")
	}
}

func TestFormatExample(t *testing.T) {
	ex := &CobliExample{
		Domain:      "Test",
		Description: "Test description",
		Why:         "Test reason",
		Details: map[string]string{
			"Key1": "Value1",
		},
	}

	result := FormatExample(ex)
	if !strings.Contains(result, "Test") {
		t.Error("should contain domain")
	}
	if !strings.Contains(result, "Test description") {
		t.Error("should contain description")
	}
	if !strings.Contains(result, "Test reason") {
		t.Error("should contain reason")
	}
	if !strings.Contains(result, "Value1") {
		t.Error("should contain detail value")
	}
}

func TestFormatExample_EmptyDetails(t *testing.T) {
	ex := &CobliExample{
		Domain:      "Empty",
		Description: "No details",
		Why:         "Testing",
	}

	result := FormatExample(ex)
	if !strings.Contains(result, "Empty") {
		t.Error("should contain domain")
	}
}

func TestPatternExamples(t *testing.T) {
	if len(PatternExamples) == 0 {
		t.Error("expected pattern examples")
	}
	if _, ok := PatternExamples["event_sourcing"]; !ok {
		t.Error("expected event_sourcing pattern")
	}
	if _, ok := PatternExamples["api_gateway"]; !ok {
		t.Error("expected api_gateway pattern")
	}
}

func TestEventSourcingExamples(t *testing.T) {
	if len(EventSourcingExamples) < 3 {
		t.Errorf("expected at least 3 event sourcing examples, got %d", len(EventSourcingExamples))
	}
}

func TestNFRExamples(t *testing.T) {
	if len(NFRExamples) < 3 {
		t.Errorf("expected at least 3 NFR examples, got %d", len(NFRExamples))
	}
}
