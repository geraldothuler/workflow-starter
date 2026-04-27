package privacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector()
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if len(d.patterns) == 0 {
		t.Error("expected patterns to be loaded")
	}
}

func TestScan_CPF(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name  string
		input string
	}{
		{"formatted", "CPF do cliente: 529.982.247-25"},
		{"unformatted", "CPF: 52998224725"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detections := d.Scan(tt.input)
			found := false
			for _, det := range detections {
				if det.Type == PIITypeCPF {
					found = true
				}
			}
			if !found {
				t.Errorf("should detect CPF in: %s", tt.input)
			}
		})
	}
}

func TestScan_CPF_Invalid(t *testing.T) {
	d := NewDetector()
	// All same digits should not validate
	detections := d.Scan("CPF: 111.111.111-11")
	for _, det := range detections {
		if det.Type == PIITypeCPF {
			t.Error("should not detect invalid CPF (all same digits)")
		}
	}
}

func TestScan_CNPJ(t *testing.T) {
	d := NewDetector()
	detections := d.Scan("CNPJ: 12.345.678/0001-00")
	found := false
	for _, det := range detections {
		if det.Type == PIITypeCNPJ {
			found = true
		}
	}
	if !found {
		t.Error("should detect CNPJ")
	}
}

func TestScan_Email(t *testing.T) {
	d := NewDetector()
	detections := d.Scan("Contact: user@example.com for info")
	found := false
	for _, det := range detections {
		if det.Type == PIITypeEmail {
			found = true
		}
	}
	if !found {
		t.Error("should detect email")
	}
}

func TestScan_IP_Public(t *testing.T) {
	d := NewDetector()
	detections := d.Scan("Connected to 8.8.8.8")
	found := false
	for _, det := range detections {
		if det.Type == PIITypeIP {
			found = true
		}
	}
	if !found {
		t.Error("should detect public IP")
	}
}

func TestScan_IP_Local(t *testing.T) {
	d := NewDetector()
	detections := d.Scan("Server at 127.0.0.1 and 192.168.1.1 and 10.0.0.1")
	for _, det := range detections {
		if det.Type == PIITypeIP {
			t.Error("should not detect local IPs")
		}
	}
}

func TestScan_NoDetections(t *testing.T) {
	d := NewDetector()
	detections := d.Scan("This is a normal text without any PII")
	if len(detections) != 0 {
		t.Errorf("expected 0 detections, got %d", len(detections))
	}
}

func TestValidateCPF(t *testing.T) {
	d := NewDetector()

	// Valid CPF (11 digits, not all same)
	if !d.validateCPF("529.982.247-25") {
		t.Error("should validate formatted CPF")
	}

	// Invalid: all same
	if d.validateCPF("000.000.000-00") {
		t.Error("should reject all-zeros CPF")
	}

	// Invalid: wrong length
	if d.validateCPF("123") {
		t.Error("should reject short CPF")
	}
}

func TestValidateCNPJ(t *testing.T) {
	d := NewDetector()

	if !d.validateCNPJ("12.345.678/0001-00") {
		t.Error("should validate formatted CNPJ")
	}
	if d.validateCNPJ("123") {
		t.Error("should reject short CNPJ")
	}
}

func TestValidateLuhn(t *testing.T) {
	d := NewDetector()

	// Valid Luhn number (Visa test)
	if !d.validateLuhn("4111111111111111") {
		t.Error("should validate valid Luhn number")
	}

	// Too short
	if d.validateLuhn("123") {
		t.Error("should reject short number")
	}

	// Invalid checksum
	if d.validateLuhn("4111111111111112") {
		t.Error("should reject invalid Luhn")
	}
}

func TestMask(t *testing.T) {
	d := NewDetector()

	result := d.mask("1234567890")
	if result != "***7890" {
		t.Errorf("expected '***7890', got %q", result)
	}

	result = d.mask("ab")
	if result != "***" {
		t.Errorf("expected '***' for short value, got %q", result)
	}
}

func TestReport(t *testing.T) {
	d := NewDetector()
	detections := []PIIDetection{
		{Type: PIITypeCPF, Value: "***7-25", Line: 1},
		{Type: PIITypeEmail, Value: "***com", Line: 2},
		{Type: PIITypeCPF, Value: "***8-00", Line: 5},
	}

	report := d.Report(detections)
	if !strings.Contains(report, "3 dado(s)") {
		t.Error("report should mention count")
	}
	if !strings.Contains(report, "CPF") {
		t.Error("report should mention CPF type")
	}
}

func TestReport_Empty(t *testing.T) {
	d := NewDetector()
	report := d.Report(nil)
	if !strings.Contains(report, "Nenhum dado sensível") {
		t.Error("should show no PII message")
	}
}

func TestScanFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(file, []byte("Email: user@test.com, IP: 8.8.8.8"), 0644)

	d := NewDetector()
	detections, err := d.ScanFile(file)
	if err != nil {
		t.Fatalf("ScanFile failed: %v", err)
	}
	if len(detections) == 0 {
		t.Error("expected detections from file")
	}
}

func TestScanFile_NonExistent(t *testing.T) {
	d := NewDetector()
	_, err := d.ScanFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestAnonymize(t *testing.T) {
	d := NewDetector()
	text := "Email: user@test.com, CPF: 529.982.247-25, IP: 8.8.8.8"
	result := d.Anonymize(text)

	if strings.Contains(result, "user@test.com") {
		t.Error("email should be anonymized")
	}
	if !strings.Contains(result, "[EMAIL-REMOVIDO]") {
		t.Error("should contain email placeholder")
	}
	if !strings.Contains(result, "[IP-REMOVIDO]") {
		t.Error("should contain IP placeholder")
	}
}

func TestAnonymize_PreservesLocalIP(t *testing.T) {
	d := NewDetector()
	text := "Server at 127.0.0.1 and 192.168.1.1"
	result := d.Anonymize(text)

	if !strings.Contains(result, "127.0.0.1") {
		t.Error("local IP should be preserved")
	}
	if !strings.Contains(result, "192.168.1.1") {
		t.Error("private IP should be preserved")
	}
}

func TestPIITypeConstants(t *testing.T) {
	if PIITypeCPF != "CPF" {
		t.Error("wrong CPF constant")
	}
	if PIITypeEmail != "Email" {
		t.Error("wrong Email constant")
	}
}
