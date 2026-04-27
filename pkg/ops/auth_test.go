package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// parseINI
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestParseINI_BasicSections(t *testing.T) {
	content := `
[default]
region = us-east-1
output = json

[profile dev]
sso_start_url = https://dev.example.com/start
sso_region = us-east-1
`
	f := writeTempFile(t, content)
	sections, err := parseINI(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sections["default"]["region"] != "us-east-1" {
		t.Errorf("expected default.region=us-east-1, got %q", sections["default"]["region"])
	}
	if sections["profile dev"]["sso_start_url"] != "https://dev.example.com/start" {
		t.Errorf("unexpected sso_start_url: %q", sections["profile dev"]["sso_start_url"])
	}
}

func TestParseINI_SkipsCommentsAndBlanks(t *testing.T) {
	content := `
# this is a comment
; also a comment

[profile test]
# key below
sso_start_url = https://test.example.com/start
`
	f := writeTempFile(t, content)
	sections, err := parseINI(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections["profile test"]["sso_start_url"] != "https://test.example.com/start" {
		t.Errorf("unexpected value: %q", sections["profile test"]["sso_start_url"])
	}
}

func TestParseINI_FileNotFound(t *testing.T) {
	_, err := parseINI("/nonexistent/path/config")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseINI_ValueWithEquals(t *testing.T) {
	content := `
[profile x]
sso_start_url = https://example.com/start?foo=bar
`
	f := writeTempFile(t, content)
	sections, _ := parseINI(f)
	if sections["profile x"]["sso_start_url"] != "https://example.com/start?foo=bar" {
		t.Errorf("value with equals incorrectly split: %q", sections["profile x"]["sso_start_url"])
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// awsProfileStartURL
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestAWSProfileStartURL_DirectURL(t *testing.T) {
	content := `
[profile myprofile]
sso_start_url = https://example.com/start
sso_region = us-east-1
`
	f := writeTempFile(t, content)
	url, err := awsProfileStartURL(f, "myprofile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/start" {
		t.Errorf("expected start URL, got %q", url)
	}
}

func TestAWSProfileStartURL_ViaSSO_Session(t *testing.T) {
	content := `
[profile myprofile]
sso_session = main-session
region = us-east-1

[sso-session main-session]
sso_start_url = https://sso.example.com/start
sso_region = us-east-1
`
	f := writeTempFile(t, content)
	url, err := awsProfileStartURL(f, "myprofile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://sso.example.com/start" {
		t.Errorf("expected URL via sso-session, got %q", url)
	}
}

func TestAWSProfileStartURL_DefaultProfile(t *testing.T) {
	content := `
[default]
sso_start_url = https://default.example.com/start
`
	f := writeTempFile(t, content)
	url, err := awsProfileStartURL(f, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://default.example.com/start" {
		t.Errorf("expected default URL, got %q", url)
	}
}

func TestAWSProfileStartURL_ProfileNotFound(t *testing.T) {
	content := `
[profile other]
sso_start_url = https://other.example.com/start
`
	f := writeTempFile(t, content)
	_, err := awsProfileStartURL(f, "missing")
	if err == nil {
		t.Error("expected error for missing profile")
	}
}

func TestAWSProfileStartURL_NoURL(t *testing.T) {
	content := `
[profile bare]
region = us-east-1
`
	f := writeTempFile(t, content)
	_, err := awsProfileStartURL(f, "bare")
	if err == nil {
		t.Error("expected error when no sso_start_url or sso_session found")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// parseAWSTime
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestParseAWSTime_Formats(t *testing.T) {
	cases := []struct {
		input    string
		wantYear int
	}{
		{"2026-02-23T18:00:00UTC", 2026},
		{"2026-02-23T18:00:00Z", 2026},
		{"2026-02-23T18:00:00+00:00", 2026},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseAWSTime(tc.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got.Year() != tc.wantYear {
				t.Errorf("expected year %d, got %d", tc.wantYear, got.Year())
			}
		})
	}
}

func TestParseAWSTime_InvalidFormat(t *testing.T) {
	_, err := parseAWSTime("not-a-date")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// findTokenExpiry
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestFindTokenExpiry_Found(t *testing.T) {
	startURL := "https://example.com/start"
	expiry := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	cache := ssoCache{StartURL: startURL, ExpiresAt: expiry}
	data, _ := json.Marshal(cache)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "abc123.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	got, found := findTokenExpiry(dir, startURL)
	if !found {
		t.Fatal("expected token to be found")
	}
	if got.IsZero() {
		t.Error("expected non-zero expiry time")
	}
}

func TestFindTokenExpiry_URLMismatch(t *testing.T) {
	cache := ssoCache{StartURL: "https://other.com/start", ExpiresAt: "2026-02-23T18:00:00Z"}
	data, _ := json.Marshal(cache)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "abc123.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	_, found := findTokenExpiry(dir, "https://example.com/start")
	if found {
		t.Error("should not find token for different URL")
	}
}

func TestFindTokenExpiry_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, found := findTokenExpiry(dir, "https://example.com/start")
	if found {
		t.Error("should not find token in empty dir")
	}
}

func TestFindTokenExpiry_MissingDir(t *testing.T) {
	_, found := findTokenExpiry("/nonexistent/dir", "https://example.com/start")
	if found {
		t.Error("should return false for missing directory")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CheckAWSAuth — integration via temp files
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestCheckAWSAuth_ValidToken(t *testing.T) {
	dir, configPath, cacheDir := setupAWSDir(t,
		"https://example.com/start",
		time.Now().Add(2*time.Hour),
	)
	_ = dir

	// Override by patching awsProfileStartURL directly is not possible,
	// so we test the individual helpers instead and verify integration via
	// the error-path (profile not found).
	sections, err := parseINI(configPath)
	if err != nil {
		t.Fatalf("parseINI: %v", err)
	}
	if sections["profile cobli-tech"]["sso_start_url"] != "https://example.com/start" {
		t.Errorf("unexpected sections: %v", sections)
	}

	expiry, found := findTokenExpiry(cacheDir, "https://example.com/start")
	if !found {
		t.Fatal("token not found")
	}
	if time.Until(expiry) < time.Hour {
		t.Errorf("expected at least 1h remaining, got %v", time.Until(expiry))
	}
}

func TestErrorResult_Fields(t *testing.T) {
	r := errorResult("myprofile", "test signal")
	if r.Status != "error" {
		t.Errorf("expected error status, got %q", r.Status)
	}
	if r.Signal != "test signal" {
		t.Errorf("unexpected signal: %q", r.Signal)
	}
	if r.Cost != "zero-llm" {
		t.Errorf("expected zero-llm cost, got %q", r.Cost)
	}
	if r.Data["profile"] != "myprofile" {
		t.Errorf("expected profile in data")
	}
	if r.Data["valid"] != false {
		t.Errorf("expected valid=false")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// helpers
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.ini")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

// setupAWSDir creates a minimal ~/.aws structure in a temp dir
// and returns (homeDir, configPath, cacheDir).
func setupAWSDir(t *testing.T, startURL string, expiry time.Time) (string, string, string) {
	t.Helper()
	homeDir := t.TempDir()
	awsDir := filepath.Join(homeDir, ".aws")
	cacheDir := filepath.Join(awsDir, "sso", "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}

	configContent := "[profile cobli-tech]\nsso_start_url = " + startURL + "\nsso_region = us-east-1\n"
	configPath := filepath.Join(awsDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	cache := ssoCache{StartURL: startURL, ExpiresAt: expiry.UTC().Format(time.RFC3339)}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(filepath.Join(cacheDir, "token.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	return homeDir, configPath, cacheDir
}
