package context

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDetectStack_GoRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/foo\n\ngo 1.24\n")

	stack, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack error: %v", err)
	}
	assertContains(t, stack.Backend, "Go", "Backend")
}

func TestDetectStack_NodeRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"app","dependencies":{"express":"^4.0"}}`)

	stack, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack error: %v", err)
	}
	assertContains(t, stack.Backend, "Node.js", "Backend")
}

func TestDetectStack_DockerRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM alpine:3.19\nRUN echo hello\n")

	stack, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack error: %v", err)
	}
	assertContains(t, stack.Infrastructure, "Docker", "Infrastructure")
}

func TestDetectStack_EmptyRepo(t *testing.T) {
	dir := t.TempDir()

	stack, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack error: %v", err)
	}
	if len(stack.Backend) != 0 {
		t.Errorf("expected empty Backend, got %v", stack.Backend)
	}
	if len(stack.Database) != 0 {
		t.Errorf("expected empty Database, got %v", stack.Database)
	}
	if len(stack.Queue) != 0 {
		t.Errorf("expected empty Queue, got %v", stack.Queue)
	}
	if len(stack.Infrastructure) != 0 {
		t.Errorf("expected empty Infrastructure, got %v", stack.Infrastructure)
	}
	if len(stack.CICD) != 0 {
		t.Errorf("expected empty CICD, got %v", stack.CICD)
	}
	if len(stack.Frontend) != 0 {
		t.Errorf("expected empty Frontend, got %v", stack.Frontend)
	}
}

func TestDetectStack_MixedRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/svc\n\ngo 1.24\n")
	writeFile(t, dir, "Dockerfile", "FROM golang:1.24\nCOPY . .\n")
	writeFile(t, dir, "docker-compose.yml", "services:\n  db:\n    image: postgres:16\n  app:\n    build: .\n")

	stack, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack error: %v", err)
	}
	assertContains(t, stack.Backend, "Go", "Backend")
	assertContains(t, stack.Infrastructure, "Docker", "Infrastructure")
	assertContains(t, stack.Database, "PostgreSQL", "Database")
}

func TestDetectStack_GitHubActions(t *testing.T) {
	dir := t.TempDir()
	workflowDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("failed to create .github/workflows: %v", err)
	}
	writeFile(t, workflowDir, "ci.yml", "name: CI\non: push\n")

	stack, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack error: %v", err)
	}
	assertContains(t, stack.CICD, "GitHub Actions", "CICD")
}

func TestDetectStack_FrontendReact(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"app","dependencies":{"react":"^18.0","react-dom":"^18.0"}}`)

	stack, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack error: %v", err)
	}
	assertContains(t, stack.Frontend, "React", "Frontend")
}

func TestDetectStack_EnvPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "DATABASE_URL=postgres://localhost:5432/mydb\nREDIS_URL=redis://localhost:6379\n")

	stack, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack error: %v", err)
	}
	assertContains(t, stack.Database, "PostgreSQL", "Database")
	assertContains(t, stack.Database, "Redis", "Database")
}

// --- helpers ---

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func assertContains(t *testing.T, slice []string, want, label string) {
	t.Helper()
	sort.Strings(slice)
	for _, v := range slice {
		if v == want {
			return
		}
	}
	t.Errorf("expected %s to contain %q, got %v", label, want, slice)
}
