package wtbserver

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
	"github.com/Cobliteam/workflow-toolkit/pkg/repoindex"
)

// job represents a long-running async operation (e.g. repo index).
type job struct {
	id     string
	name   string
	status string // "running" | "done" | "failed"
	buf    bytes.Buffer
	mu     sync.Mutex
	err    error
}

// jobRegistry manages async jobs in memory.
type jobRegistry struct {
	mu   sync.Mutex
	jobs map[string]*job
}

func newJobRegistry() *jobRegistry {
	return &jobRegistry{jobs: make(map[string]*job)}
}

func (r *jobRegistry) create(name string) string {
	id := fmt.Sprintf("%d-%s", time.Now().UnixMilli(), randHex(4))
	j := &job{id: id, name: name, status: "running"}
	r.mu.Lock()
	r.jobs[id] = j
	r.mu.Unlock()
	return id
}

func (r *jobRegistry) get(id string) *job {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.jobs[id]
}

// readFrom returns unread output starting at offset, and whether the job is done.
func (r *jobRegistry) readFrom(id string, offset int) (string, bool) {
	j := r.get(id)
	if j == nil {
		return "", true
	}
	j.mu.Lock()
	data := j.buf.String()
	status := j.status
	j.mu.Unlock()

	if offset >= len(data) {
		return "", status != "running"
	}
	return data[offset:], status != "running"
}

// write appends text to a job's output buffer.
func (j *job) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.buf.Write(p)
}

func (j *job) logf(format string, args ...interface{}) {
	j.Write([]byte(fmt.Sprintf(format, args...)))
}

// runIndexJob executes repo indexing in a goroutine, streaming output to the job buffer.
func (s *Server) runIndexJob(jobID, repoName, repoPath, owner, providerName string, force bool) {
	j := s.jobs.get(jobID)
	if j == nil {
		return
	}

	j.logf("indexing %s (path: %s, provider: %s, force: %v)\n", repoName, repoPath, providerName, force)

	if _, err := os.Stat(repoPath); err != nil {
		j.logf("error: repo path not found: %s\n", repoPath)
		j.mu.Lock()
		j.status = "failed"
		j.err = fmt.Errorf("repo path not found: %s", repoPath)
		j.mu.Unlock()
		return
	}

	resolver, _ := credentials.NewFullResolver(s.repoRoot, os.Getenv("WTB_MASTER_KEY"))
	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:     providerName,
		CredResolver: resolver,
	})
	if err != nil {
		j.logf("error: llm provider %q: %v\n", providerName, err)
		j.mu.Lock()
		j.status = "failed"
		j.err = err
		j.mu.Unlock()
		return
	}

	if sp, ok := provider.(interface{ SetSystemPrompt(string) }); ok {
		_ = sp
	}

	repoDB, err := repoindex.Open(s.repoRoot)
	if err != nil {
		j.logf("error: open repos.duckdb: %v\n", err)
		j.mu.Lock()
		j.status = "failed"
		j.err = err
		j.mu.Unlock()
		return
	}
	defer repoDB.Close()

	result := repoindex.IndexRepo(repoDB, repoindex.IndexOptions{
		RepoName: repoName,
		RepoPath: repoPath,
		Owner:    resolveOwner(owner, repoPath, repoName),
		LLM:      provider,
		Force:    force,
		Verbose:  true,
		Output:   j,
	})

	j.mu.Lock()
	if result.Error != nil {
		j.logf("error: %v\n", result.Error)
		j.status = "failed"
		j.err = result.Error
	} else {
		if len(result.LayersIndexed) == 0 {
			j.logf("repo %q is up-to-date (skipped: %s)\n", repoName, strings.Join(result.LayersSkipped, ", "))
		} else {
			j.logf("indexed: %s\n", strings.Join(result.LayersIndexed, ", "))
			if len(result.LayersSkipped) > 0 {
				j.logf("skipped (unchanged): %s\n", strings.Join(result.LayersSkipped, ", "))
			}
		}
		j.status = "done"
	}
	j.mu.Unlock()
}

// resolveOwner reads owner from architecture/summary.md if not provided.
func resolveOwner(owner, repoPath, repoName string) string {
	if owner != "" {
		return owner
	}
	// Try architecture summary
	for _, rel := range []string{
		filepath.Join("..", "architecture", "services", repoName+".md"),
		filepath.Join("..", "architecture", repoName+".md"),
	} {
		p := filepath.Join(repoPath, rel)
		if data, err := os.ReadFile(p); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(strings.ToLower(line), "owner:") ||
					strings.HasPrefix(strings.ToLower(line), "squad:") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}
	return ""
}

func randHex(n int) string {
	const chars = "abcdef0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
