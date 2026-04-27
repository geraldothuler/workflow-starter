package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

// --- IsValidType ---

func TestIsValidType_Valid(t *testing.T) {
	for _, v := range WorkflowTypes {
		if !IsValidType(v) {
			t.Errorf("IsValidType(%q) = false, want true", v)
		}
	}
}

func TestIsValidType_Invalid(t *testing.T) {
	cases := []string{"unknown", "", "INCIDENT", "review2"}
	for _, c := range cases {
		if IsValidType(c) {
			t.Errorf("IsValidType(%q) = true, want false", c)
		}
	}
}

// --- NextNNN ---

func TestNextNNN_NonExistentDir(t *testing.T) {
	nnn, err := NextNNN("/tmp/nonexistent-scaffold-dir-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nnn != "001" {
		t.Errorf("got %q, want %q", nnn, "001")
	}
}

func TestNextNNN_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	nnn, err := NextNNN(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nnn != "001" {
		t.Errorf("got %q, want %q", nnn, "001")
	}
}

func TestNextNNN_WithArtefacts(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "001-incident-2026-01-01.md")
	touch(t, dir, "002-incident-2026-01-02.md")
	touch(t, dir, "INDEX.md") // should not count

	nnn, err := NextNNN(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nnn != "003" {
		t.Errorf("got %q, want %q", nnn, "003")
	}
}

func TestNextNNN_IgnoresNonNNNFiles(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "001-artefact.md")
	touch(t, dir, "README.md")   // no NNN prefix
	touch(t, dir, "notes.txt")   // no NNN prefix

	nnn, err := NextNNN(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nnn != "002" {
		t.Errorf("got %q, want %q", nnn, "002")
	}
}

// --- ListArtefacts ---

func TestListArtefacts_Empty(t *testing.T) {
	dir := t.TempDir()
	entries, err := ListArtefacts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestListArtefacts_ExcludesIndexMd(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "INDEX.md")
	touch(t, dir, "001-foo.md")

	entries, err := ListArtefacts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0] != "001-foo.md" {
		t.Errorf("got %v, want [001-foo.md]", entries)
	}
}

func TestListArtefacts_Sorted(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "003-c.md")
	touch(t, dir, "001-a.md")
	touch(t, dir, "002-b.md")

	entries, err := ListArtefacts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"001-a.md", "002-b.md", "003-c.md"}
	for i, w := range want {
		if entries[i] != w {
			t.Errorf("entries[%d] = %q, want %q", i, entries[i], w)
		}
	}
}

// --- ExtractNNN ---

func TestExtractNNN(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"001-incident-2026-01-01.md", "001"},
		{"002-postmortem.md", "002"},
		{"123", "123"},
		{"ab", "ab"},  // shorter than 3 — return as-is
		{"", ""},
	}
	for _, c := range cases {
		got := ExtractNNN(c.name)
		if got != c.want {
			t.Errorf("ExtractNNN(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// --- helpers ---

func touch(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
		t.Fatalf("touch %s: %v", name, err)
	}
}
