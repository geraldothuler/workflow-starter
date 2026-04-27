package main

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/scaffold"
)

// TestCredentialPatterns_DetectsCredentials verifies security guardrail:
// credential patterns are detected in output
func TestCredentialPatterns_DetectsCredentials(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantHit bool
	}{
		{"password_equals", "password=mysecret123", true},
		{"PASSWORD_upper", "PASSWORD=mysecret123", true},
		{"secret_equals", "secret=abc123def", true},
		{"aws_access_key", "AKIAIOSFODNN7EXAMPLE", true},
		{"token_equals", "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9abc123", true},
		{"safe_text", "This is a normal log message", false},
		{"safe_reference", "password=$DB_PASSWORD", false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ContainsCredential(tc.input)
			if got != tc.wantHit {
				t.Errorf("ContainsCredential(%q) = %v, want %v", tc.input, got, tc.wantHit)
			}
		})
	}
}

// TestExportCmd_PrintsWarning verifies LGPD guardrail:
// wtb export must print sensitive data warning before creating zip
func TestExportCmd_PrintsWarning(t *testing.T) {
	// Build the binary first (skip if go build not available in test env)
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "wtb-test")

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = filepath.Join(os.Getenv("HOME"), "workflow", "cmd", "wtb")
	if err := buildCmd.Run(); err != nil {
		t.Skipf("skipping: could not build wtb binary: %v", err)
	}

	// Create a fake ~/.workflow dir
	fakeHome := t.TempDir()
	workflowDir := filepath.Join(fakeHome, ".workflow", "1on1")
	if err := os.MkdirAll(workflowDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write a dummy file
	if err := os.WriteFile(filepath.Join(workflowDir, "README.md"), []byte("# test"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "export")
	cmd.Env = append(os.Environ(), "HOME="+fakeHome)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // may fail on zip creation, we just need the warning

	output := out.String()
	if !strings.Contains(output, "AVISO") && !strings.Contains(output, "sensiv") {
		t.Errorf("export command did not print sensitive data warning.\nOutput:\n%s", output)
	}
}

// TestZipDir_CreatesValidZip verifies zipDir creates a readable zip
func TestZipDir_CreatesValidZip(t *testing.T) {
	src := t.TempDir()
	// Create some test files
	if err := os.WriteFile(filepath.Join(src, "test.md"), []byte("# test content"), 0600); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(src, "subdir")
	if err := os.MkdirAll(subDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.md"), []byte("nested"), 0600); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "output.zip")
	if err := zipDir(src, dest); err != nil {
		t.Fatalf("zipDir failed: %v", err)
	}

	// Verify zip is readable
	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("zip not readable: %v", err)
	}
	defer r.Close()

	if len(r.File) == 0 {
		t.Error("zip is empty")
	}
}

// TestCountActiveLines_CountsCorrectly verifies premise counting
func TestCountActiveLines_CountsCorrectly(t *testing.T) {
	f := t.TempDir()
	content := `# Premises

### P001 — First
Content here.

### P002 — Second
Content here.

### P003 — Third
Content here.
`
	path := filepath.Join(f, "premises.md")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	got := countActiveLines(path, "### P")
	if got != 3 {
		t.Errorf("countActiveLines = %d, want 3", got)
	}
}

// TestDeletePersonalData_RequiresConfirmation verifies LGPD delete requires explicit confirmation
func TestDeletePersonalData_RequiresConfirmation(t *testing.T) {
	binPath := buildBinary(t)
	if binPath == "" {
		t.Skip("skipping: could not build wtb binary")
	}

	fakeHome := t.TempDir()
	workflowDir := filepath.Join(fakeHome, ".workflow")
	if err := os.MkdirAll(workflowDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "test.md"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	// Send wrong confirmation — should cancel
	cmd := exec.Command(binPath, "delete-personal-data")
	cmd.Env = append(os.Environ(), "HOME="+fakeHome)
	cmd.Stdin = strings.NewReader("NO\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()

	output := out.String()
	if !strings.Contains(output, "cancelada") {
		t.Errorf("expected cancellation message, got: %s", output)
	}

	// Verify data still exists
	if _, err := os.Stat(filepath.Join(workflowDir, "test.md")); os.IsNotExist(err) {
		t.Error("data was deleted without confirmation")
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "wtb-test")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = filepath.Join(os.Getenv("HOME"), "workflow", "cmd", "wtb")
	if err := buildCmd.Run(); err != nil {
		return ""
	}
	return binPath
}

// TestNextNNN_EmptyDir returns 001 for empty directory
func TestNextNNN_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := scaffold.NextNNN(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "001" {
		t.Errorf("scaffold.NextNNN(empty) = %q, want %q", got, "001")
	}
}

// TestNextNNN_ExistingArtefacts increments correctly
func TestNextNNN_ExistingArtefacts(t *testing.T) {
	dir := t.TempDir()
	// Create two existing artefacts
	for _, name := range []string{"001-incident-2026-01-01.md", "002-incident-2026-01-02.md", "INDEX.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scaffold.NextNNN(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "003" {
		t.Errorf("scaffold.NextNNN(2 artefacts) = %q, want %q", got, "003")
	}
}

// TestRenderTemplate_SubstitutesVars verifies template rendering
func TestRenderTemplate_SubstitutesVars(t *testing.T) {
	content := "# Savepoint YYYY-MM-DD\n\nID: NNN\n"
	vars := map[string]string{
		"Date":    "2026-02-23",
		"NNN":     "001",
		"Context": "test",
		"Type":    "incident",
	}
	got, err := scaffold.RenderTemplate(content, vars)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "2026-02-23") {
		t.Errorf("expected date in output, got: %s", got)
	}
	if !strings.Contains(got, "001") {
		t.Errorf("expected NNN in output, got: %s", got)
	}
}

// TestIsValidType_AcceptsValidTypes checks workflow type validation
func TestIsValidType_AcceptsValidTypes(t *testing.T) {
	cases := []struct {
		input string
		valid bool
	}{
		{"incident", true},
		{"postmortem", true},
		{"review", true},
		{"1on1", true},
		{"sprint", false},
		{"planning", false},
		{"", false},
	}
	for _, tc := range cases {
		got := scaffold.IsValidType(tc.input)
		if got != tc.valid {
			t.Errorf("scaffold.IsValidType(%q) = %v, want %v", tc.input, got, tc.valid)
		}
	}
}

// TestExpandHome_ExpandsTilde verifies ~ expansion
func TestExpandHome_ExpandsTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := expandHome("~/workflow")
	want := filepath.Join(home, "workflow")
	if got != want {
		t.Errorf("expandHome(~/workflow) = %q, want %q", got, want)
	}
}

// TestListArtefacts_IgnoresIndex verifies INDEX.md is excluded from listing
func TestListArtefacts_IgnoresIndex(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"001-test-2026-01-01.md", "002-test-2026-01-02.md", "INDEX.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scaffold.ListArtefacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("scaffold.ListArtefacts = %d entries, want 2; got: %v", len(got), got)
	}
	for _, e := range got {
		if e == "INDEX.md" {
			t.Error("INDEX.md should not be in artefact list")
		}
	}
}
