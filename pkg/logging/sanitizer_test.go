package logging

import (
	"strings"
	"testing"
)

func TestNewSanitizer(t *testing.T) {
	s := NewSanitizer(true)
	if s == nil {
		t.Fatal("expected non-nil sanitizer")
	}
	if !s.enabled {
		t.Error("expected enabled=true")
	}
}

func TestSanitize_Disabled(t *testing.T) {
	s := NewSanitizer(false)
	input := "My email is user@example.com"
	result := s.Sanitize(input)
	if result != input {
		t.Error("disabled sanitizer should return input unchanged")
	}
}

func TestSanitize_Email(t *testing.T) {
	s := NewSanitizer(true)
	input := "Contact: user@example.com for details"
	result := s.Sanitize(input)
	if strings.Contains(result, "user@example.com") {
		t.Error("email should be redacted")
	}
	if !strings.Contains(result, "[EMAIL-REDACTED]") {
		t.Error("should contain [EMAIL-REDACTED]")
	}
}

func TestSanitize_CPF(t *testing.T) {
	s := NewSanitizer(true)

	tests := []struct {
		name  string
		input string
	}{
		{"formatted", "CPF: 123.456.789-00"},
		{"unformatted", "CPF: 12345678900"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if !strings.Contains(result, "[CPF-REDACTED]") {
				t.Errorf("CPF should be redacted in: %s -> %s", tt.input, result)
			}
		})
	}
}

func TestSanitize_CreditCard(t *testing.T) {
	s := NewSanitizer(true)
	// The phone regex runs before credit card regex, so credit card
	// groups like "1111 1111" get matched as phone numbers first.
	// What matters is that the digits are redacted (by either pattern).
	input := "Card: 4111 1111 1111 1111"
	result := s.Sanitize(input)
	if strings.Contains(result, "4111") {
		t.Error("credit card digits should be redacted from output")
	}
	if !strings.Contains(result, "REDACTED") {
		t.Errorf("should contain some REDACTED marker: %s", result)
	}
}

func TestSanitize_JWT(t *testing.T) {
	s := NewSanitizer(true)
	input := "Token: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123"
	result := s.Sanitize(input)
	if !strings.Contains(result, "[JWT-REDACTED]") {
		t.Errorf("JWT should be redacted: %s", result)
	}
}

func TestSanitize_LocalIP_Preserved(t *testing.T) {
	s := NewSanitizer(true)
	input := "Server at 127.0.0.1 and 192.168.1.1"
	result := s.Sanitize(input)
	if !strings.Contains(result, "127.0.0.1") {
		t.Error("local IPs should be preserved")
	}
	if !strings.Contains(result, "192.168.1.1") {
		t.Error("private IPs should be preserved")
	}
}

func TestSanitize_PublicIP_Redacted(t *testing.T) {
	s := NewSanitizer(true)
	input := "Connected to 8.8.8.8"
	result := s.Sanitize(input)
	if strings.Contains(result, "8.8.8.8") {
		t.Error("public IP should be redacted")
	}
}

func TestSanitize_Password(t *testing.T) {
	s := NewSanitizer(true)
	input := "password=mysecret123"
	result := s.Sanitize(input)
	if strings.Contains(result, "mysecret123") {
		t.Error("password should be redacted")
	}
}

func TestSanitize_BearerToken(t *testing.T) {
	s := NewSanitizer(true)
	input := "Authorization: Bearer abc123def456"
	result := s.Sanitize(input)
	if strings.Contains(result, "abc123def456") {
		t.Error("bearer token should be redacted")
	}
}

func TestSafeLog(t *testing.T) {
	s := NewSanitizer(true)
	result := s.SafeLog("Email: %s", "user@test.com")
	if strings.Contains(result, "user@test.com") {
		t.Error("SafeLog should sanitize emails")
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger(LevelNormal, true)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.level != LevelNormal {
		t.Errorf("expected level Normal, got %d", logger.level)
	}
}

func TestLogLevels(t *testing.T) {
	if LevelQuiet != 0 {
		t.Error("LevelQuiet should be 0")
	}
	if LevelNormal != 1 {
		t.Error("LevelNormal should be 1")
	}
	if LevelVerbose != 2 {
		t.Error("LevelVerbose should be 2")
	}
	if LevelDebug != 3 {
		t.Error("LevelDebug should be 3")
	}
}

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ... [truncated]"},
		{"exact", 5, "exact"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := TruncateForLog(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		showFirst int
		showLast  int
		expected  string
	}{
		{"normal", "1234567890", 2, 2, "12******90"},
		{"short", "abc", 2, 2, "***"},
		{"email-like", "user@test.com", 3, 3, "use*******com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitive(tt.value, tt.showFirst, tt.showLast)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeForExport(t *testing.T) {
	s := NewSanitizer(true)

	data := map[string]interface{}{
		"name":  "John",
		"email": "john@example.com",
		"nested": map[string]interface{}{
			"cpf": "123.456.789-00",
		},
	}

	result := s.SanitizeForExport(data)

	if result["email"] == "john@example.com" {
		t.Error("email should be redacted in export")
	}
	nested := result["nested"].(map[string]interface{})
	if nested["cpf"] == "123.456.789-00" {
		t.Error("CPF should be redacted in nested export")
	}
}

func TestSanitizeForExport_Disabled(t *testing.T) {
	s := NewSanitizer(false)
	data := map[string]interface{}{
		"email": "user@test.com",
	}
	result := s.SanitizeForExport(data)
	if result["email"] != "user@test.com" {
		t.Error("disabled sanitizer should return data unchanged")
	}
}

func TestSanitizeForExport_SliceOfStrings(t *testing.T) {
	s := NewSanitizer(true)
	data := map[string]interface{}{
		"items": []interface{}{
			"normal text",
			"email: user@test.com",
		},
	}

	result := s.SanitizeForExport(data)
	items := result["items"].([]interface{})
	if items[0] != "normal text" {
		t.Error("non-sensitive string should be unchanged")
	}
	if items[1] == "email: user@test.com" {
		t.Error("email in slice should be redacted")
	}
}

func TestSanitizeForExport_SliceOfMaps(t *testing.T) {
	s := NewSanitizer(true)
	data := map[string]interface{}{
		"records": []interface{}{
			map[string]interface{}{
				"email": "user@test.com",
			},
		},
	}

	result := s.SanitizeForExport(data)
	records := result["records"].([]interface{})
	rec := records[0].(map[string]interface{})
	if rec["email"] == "user@test.com" {
		t.Error("email in nested map in slice should be redacted")
	}
}

func TestSanitizeForExport_NonStringValues(t *testing.T) {
	s := NewSanitizer(true)
	data := map[string]interface{}{
		"count":  42,
		"active": true,
		"items":  []interface{}{123, false},
	}

	result := s.SanitizeForExport(data)
	if result["count"] != 42 {
		t.Error("int should be preserved")
	}
	if result["active"] != true {
		t.Error("bool should be preserved")
	}
	items := result["items"].([]interface{})
	if items[0] != 123 {
		t.Error("int in slice should be preserved")
	}
}

func TestSanitize_CNPJ(t *testing.T) {
	s := NewSanitizer(true)
	input := "CNPJ: 12.345.678/0001-90"
	result := s.Sanitize(input)
	if !strings.Contains(result, "[CNPJ-REDACTED]") {
		t.Errorf("CNPJ should be redacted: %s", result)
	}
}

func TestSanitize_AnthropicKey(t *testing.T) {
	s := NewSanitizer(true)
	// Generate a fake key that matches the pattern (sk-ant- followed by 95+ chars)
	fakeKey := "sk-ant-" + strings.Repeat("abcdef1234567890", 6) // 96 chars
	input := "Key: " + fakeKey
	result := s.Sanitize(input)
	if !strings.Contains(result, "[ANTHROPIC-KEY-REDACTED]") {
		t.Error("Anthropic key should be redacted")
	}
}

func TestSanitize_10IP_Preserved(t *testing.T) {
	s := NewSanitizer(true)
	input := "Server at 10.0.0.1"
	result := s.Sanitize(input)
	if !strings.Contains(result, "10.0.0.1") {
		t.Error("10.x.x.x IPs should be preserved")
	}
}

func TestLogger_QuietLevel(t *testing.T) {
	// Logger at Quiet level should only output via Quiet and Error/Warning/Success
	logger := NewLogger(LevelQuiet, true)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.level != LevelQuiet {
		t.Error("expected LevelQuiet")
	}
}

func TestLogger_DebugLevel(t *testing.T) {
	logger := NewLogger(LevelDebug, true)
	if logger.level != LevelDebug {
		t.Error("expected LevelDebug")
	}
}
