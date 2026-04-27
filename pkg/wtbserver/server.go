// Package wtbserver implements the wtb daemon: a long-lived process that holds
// DuckDB/SQLite connections and serves two transports:
//
//  1. HTTP over Unix domain socket (~/.wtb/wtb.sock) — used by the wtb CLI.
//     Plain text output for display commands; JSON for data; chunked streaming
//     for long-running operations (repo index).
//
//  2. MCP Streamable HTTP on TCP localhost:7654 — used by Claude Code.
//     Configured via `url: http://localhost:7654/mcp` in .claude/settings.json.
//     Same DB connections, zero per-session overhead.
package wtbserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/Cobliteam/workflow-toolkit/pkg/docstore"
	"github.com/Cobliteam/workflow-toolkit/pkg/mcp"
	"github.com/Cobliteam/workflow-toolkit/pkg/repoindex"
	"github.com/Cobliteam/workflow-toolkit/pkg/taskstore"
	"github.com/Cobliteam/workflow-toolkit/pkg/webhook"
)

// DefaultMCPPort is the TCP port for the MCP streamable HTTP transport.
const DefaultMCPPort = "7654"

const sockDir = ".wtb"
const sockFile = "wtb.sock"

// SockPath returns the Unix socket path (~/.wtb/wtb.sock).
func SockPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, sockDir)
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, sockFile)
}

// Server holds shared DB connections and serves two transports.
// repoDB is intentionally NOT held as a persistent connection — it is opened
// read-only per request so CLI commands can open it freely in parallel.
type Server struct {
	repoRoot string
	docDB    *docstore.DB
	taskDB   *taskstore.DB
	jobs     *jobRegistry
	// Unix socket transport (CLI)
	srv      *http.Server
	listener net.Listener
	// MCP streamable HTTP transport (Claude Code)
	mcpSrv *mcpserver.StreamableHTTPServer
	// Webhook ingestion (optional, only if --webhook-port is set)
	webhookSrv *http.Server
}

// New creates a server bound to repoRoot — opens all DB connections.
// repos.duckdb is NOT opened here; it is opened per-request (read-only)
// so that CLI commands can acquire read/write access concurrently.
func New(repoRoot string) (*Server, error) {
	docDB, err := docstore.Open(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open docs.db: %w", err)
	}
	taskDB, err := taskstore.Open(repoRoot)
	if err != nil {
		docDB.Close()
		return nil, fmt.Errorf("open backlog.db: %w", err)
	}
	return &Server{
		repoRoot: repoRoot,
		docDB:    docDB,
		taskDB:   taskDB,
		jobs:     newJobRegistry(),
	}, nil
}

// Start begins listening on sockPath (Unix socket for CLI) and on
// localhost:mcpPort (TCP for MCP). Both run in background goroutines.
// Pass mcpPort="" to use DefaultMCPPort.
func (s *Server) Start(sockPath, mcpPort string) error {
	if mcpPort == "" {
		mcpPort = DefaultMCPPort
	}

	// — Unix socket transport (CLI) —
	os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", sockPath, err)
	}
	s.listener = ln

	mux := http.NewServeMux()
	s.registerRoutes(mux)
	s.srv = &http.Server{Handler: mux}
	go s.srv.Serve(ln)

	// — MCP streamable HTTP transport (Claude Code) —
	mcpSrv := mcp.NewServer(s.repoRoot, s.repoRoot)
	s.mcpSrv = mcpserver.NewStreamableHTTPServer(mcpSrv,
		mcpserver.WithEndpointPath("/mcp"),
	)
	go func() {
		// Start blocks until shutdown — ignore "server closed" on graceful stop.
		if err := s.mcpSrv.Start(":" + mcpPort); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
		}
	}()

	return nil
}

// StartWebhook starts the webhook ingestion server on the given TCP port.
// It loads use-cases/webhook-routing.yml and registers /webhooks/github and
// /webhooks/datadog. Secrets are read from Keychain at dispatch time.
// Pass port="" to disable (no-op).
func (s *Server) StartWebhook(port string) error {
	if port == "" {
		return nil
	}
	routing, err := webhook.LoadRouting(s.repoRoot)
	if err != nil {
		return fmt.Errorf("webhook routing: %w", err)
	}
	h := webhook.New(routing, s.repoRoot, s.repoRoot)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	s.webhookSrv = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	go func() {
		if err := s.webhookSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "webhook server error: %v\n", err)
		}
	}()
	return nil
}

// Shutdown closes all servers and DB connections gracefully.
func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.srv.Shutdown(ctx)
	if s.mcpSrv != nil {
		s.mcpSrv.Shutdown(ctx)
	}
	if s.webhookSrv != nil {
		s.webhookSrv.Shutdown(ctx)
	}
	s.docDB.Close()
	s.taskDB.Close()
}

// registerRoutes wires all HTTP endpoints.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)

	// Repo — reads
	mux.HandleFunc("/repo/show", s.handleRepoShow)
	mux.HandleFunc("/repo/list", s.handleRepoList)
	mux.HandleFunc("/repo/query", s.handleRepoQuery)
	mux.HandleFunc("/repo/export-summary", s.handleRepoExportSummary)

	// Repo — async write (indexing)
	mux.HandleFunc("/repo/index", s.handleRepoIndex)

	// Job streaming (Fase 3)
	mux.HandleFunc("/jobs/stream", s.handleJobStream)
	mux.HandleFunc("/jobs/status", s.handleJobStatus)

	// Doc — reads
	mux.HandleFunc("/doc/list", s.handleDocList)
	mux.HandleFunc("/doc/search", s.handleDocSearch)
	mux.HandleFunc("/doc/get", s.handleDocGet)

	// Doc — writes
	mux.HandleFunc("/doc/add", s.handleDocAdd)
	mux.HandleFunc("/doc/append", s.handleDocAppend)
	mux.HandleFunc("/doc/update", s.handleDocUpdate)
	mux.HandleFunc("/doc/delete", s.handleDocDelete)

	// Backlog — reads
	mux.HandleFunc("/backlog/list", s.handleBacklogList)
	mux.HandleFunc("/backlog/search", s.handleBacklogSearch)

	// Backlog — writes
	mux.HandleFunc("/backlog/add", s.handleBacklogAdd)
	mux.HandleFunc("/backlog/update", s.handleBacklogUpdate)
	mux.HandleFunc("/backlog/done", s.handleBacklogTransition)
	mux.HandleFunc("/backlog/block", s.handleBacklogTransition)
	mux.HandleFunc("/backlog/start", s.handleBacklogTransition)
}

// ── Health ─────────────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "ok",
		"repo_root": s.repoRoot,
	})
}

// ── Repo reads ─────────────────────────────────────────────────────────────────

func (s *Server) handleRepoShow(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	section := r.URL.Query().Get("section")
	table := r.URL.Query().Get("table") == "true"

	db, err := repoindex.OpenReadOnly(s.repoRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer db.Close()

	snap, err := repoindex.GetSnapshot(db, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if section != "" {
		cols, rows := repoindex.SnapshotSection(snap, section)
		if cols == nil {
			http.Error(w, fmt.Sprintf("unknown section %q — use: handlers, models, apis, events, config", section), http.StatusBadRequest)
			return
		}
		if table {
			fmt.Fprint(w, repoindex.RenderTable(cols, rows))
			return
		}
		type row = map[string]string
		var out []row
		for _, r := range rows {
			m := make(row)
			for i, c := range cols {
				m[c] = r[i]
			}
			out = append(out, m)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
		return
	}

	if table {
		w.Header().Set("Content-Type", "text/plain")
		writeSnapTable(w, snap)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

func (s *Server) handleRepoList(w http.ResponseWriter, r *http.Request) {
	db, err := repoindex.OpenReadOnly(s.repoRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer db.Close()

	repos, err := repoindex.ListRepos(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repos)
}

func (s *Server) handleRepoQuery(w http.ResponseWriter, r *http.Request) {
	sqlQuery := r.URL.Query().Get("sql")
	table := r.URL.Query().Get("table") == "true"

	if sqlQuery == "" {
		http.Error(w, "sql is required", http.StatusBadRequest)
		return
	}
	db, err := repoindex.OpenReadOnly(s.repoRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer db.Close()

	cols, rows, err := repoindex.QueryRows(db, sqlQuery)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if table {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, repoindex.RenderTable(cols, rows))
		return
	}
	type rowMap = map[string]string
	var out []rowMap
	for _, row := range rows {
		m := make(rowMap)
		for i, c := range cols {
			m[c] = row[i]
		}
		out = append(out, m)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) handleRepoExportSummary(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "markdown"
	}
	db, err := repoindex.OpenReadOnly(s.repoRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer db.Close()

	output, err := repoindex.ExportSummary(db, name, format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, output)
}

// ── Repo async index (Fase 3) ─────────────────────────────────────────────────

func (s *Server) handleRepoIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("name")
	path := q.Get("path")
	owner := q.Get("owner")
	force := q.Get("force") == "true"
	provider := q.Get("provider")
	if provider == "" {
		provider = "claudecli"
	}
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, "Cobliteam", name)
	}

	jobID := s.jobs.create(name)
	go s.runIndexJob(jobID, name, path, owner, provider, force)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"job_id": jobID, "status": "started"})
}

// ── Job streaming (Fase 3) ────────────────────────────────────────────────────

func (s *Server) handleJobStream(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	// SSE headers for real-time streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, canFlush := w.(http.Flusher)

	ctx := r.Context()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var offset int
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			chunk, done := s.jobs.readFrom(jobID, offset)
			if len(chunk) > 0 {
				fmt.Fprint(w, chunk)
				offset += len(chunk)
				if canFlush {
					flusher.Flush()
				}
			}
			if done {
				// Send final status line
				job := s.jobs.get(jobID)
				if job != nil && job.err != nil {
					fmt.Fprintf(w, "\n[error] %v\n", job.err)
				}
				return
			}
		}
	}
}

func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	job := s.jobs.get(jobID)
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]string{
		"id":     job.id,
		"name":   job.name,
		"status": job.status,
	}
	if job.err != nil {
		resp["error"] = job.err.Error()
	}
	json.NewEncoder(w).Encode(resp)
}

// ── Doc reads ──────────────────────────────────────────────────────────────────

func (s *Server) handleDocList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	docs, err := s.docDB.List(docstore.DocFilter{
		Type:  q.Get("type"),
		Since: q.Get("since"),
		Repo:  q.Get("repo"),
		Tag:   q.Get("tag"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(docs) == 0 {
		fmt.Fprintln(w, "— sem artefatos para os filtros fornecidos.")
		return
	}
	writeDocs(w, docs)
}

func (s *Server) handleDocSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "q is required", http.StatusBadRequest)
		return
	}
	docs, err := s.docDB.SearchAll(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(docs) == 0 {
		fmt.Fprintf(w, "— sem resultados para %q.\n", q)
		return
	}
	writeDocs(w, docs)
}

func (s *Server) handleDocGet(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	d, err := s.docDB.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeDocFull(w, d)
}

// ── Doc writes ─────────────────────────────────────────────────────────────────

func (s *Server) handleDocAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var in docstore.DocInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	d, err := s.docDB.Add(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}{ID: d.ID, Title: d.Title})
}

func (s *Server) handleDocAppend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		Section string `json:"section"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Section != "" {
		d, err := s.docDB.Get(req.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newContent := insertAfterSection(d.Content, req.Section, req.Content)
		_, err = s.docDB.Update(req.ID, docstore.DocUpdateInput{Content: newContent})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "✓ atualizado: %s\n", req.ID)
		return
	}
	d, err := s.docDB.Append(req.ID, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "✓ conteúdo adicionado: %s — %s\n", d.ID, d.Title)
}

func (s *Server) handleDocUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID    string                 `json:"id"`
		Input docstore.DocUpdateInput `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	d, err := s.docDB.Update(req.ID, req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "✓ atualizado: %s — %s\n", d.ID, d.Title)
}

func (s *Server) handleDocDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if err := s.docDB.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "✓ deletado (soft): %s\n", id)
}

// ── Backlog reads ──────────────────────────────────────────────────────────────

func (s *Server) handleBacklogList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	entries, err := s.taskDB.List(taskstore.Filter{
		Status: q.Get("status"),
		Repo:   q.Get("repo"),
		Tag:    q.Get("tag"),
		Jira:   q.Get("jira"),
		Since:  q.Get("since"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(entries) == 0 {
		fmt.Fprintln(w, "— sem entradas para os filtros fornecidos.")
		return
	}
	writeEntries(w, entries)
}

func (s *Server) handleBacklogSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "q is required", http.StatusBadRequest)
		return
	}
	entries, err := s.taskDB.Search(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(entries) == 0 {
		fmt.Fprintf(w, "— sem resultados para %q.\n", q)
		return
	}
	writeEntries(w, entries)
}

// ── Backlog writes ─────────────────────────────────────────────────────────────

func (s *Server) handleBacklogAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var in taskstore.EntryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	e, err := s.taskDB.Add(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "✓ adicionado: %s — %s\n", e.ID, e.Title)
}

func (s *Server) handleBacklogUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID    string                    `json:"id"`
		Input taskstore.EntryUpdateInput `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	e, err := s.taskDB.Update(req.ID, req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "✓ atualizado: %s — %s\n", e.ID, e.Title)
}

func (s *Server) handleBacklogTransition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	// Determine action from URL path
	action := filepath.Base(r.URL.Path)
	switch action {
	case "done":
		if err := s.taskDB.Done(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "✓ done: %s\n", id)
	case "block":
		if err := s.taskDB.Block(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "✓ blocked: %s\n", id)
	case "start":
		if err := s.taskDB.Start(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "✓ in-progress: %s\n", id)
	default:
		http.Error(w, "unknown transition: "+action, http.StatusBadRequest)
	}
}

// ── Formatters (mirrors cmd/wtb output exactly) ────────────────────────────────

func writeDocs(w io.Writer, docs []docstore.Document) {
	for _, d := range docs {
		fmt.Fprintf(w, "[%s] %s — %s", d.Type, d.ID, d.Title)
		if d.DocDate != "" {
			fmt.Fprintf(w, " (%s)", d.DocDate)
		}
		fmt.Fprintln(w)
		if d.Repo != "" {
			fmt.Fprintf(w, "  repo: %s\n", d.Repo)
		}
		if len(d.Tags) > 0 {
			fmt.Fprintf(w, "  tags: %s\n", strings.Join(d.Tags, ", "))
		}
		fmt.Fprintln(w)
	}
}

func writeDocFull(w io.Writer, d docstore.Document) {
	fmt.Fprintf(w, "# %s\n", d.Title)
	fmt.Fprintf(w, "id: %s | type: %s | date: %s", d.ID, d.Type, d.DocDate)
	if d.Repo != "" {
		fmt.Fprintf(w, " | repo: %s", d.Repo)
	}
	if len(d.Tags) > 0 {
		fmt.Fprintf(w, " | tags: %s", strings.Join(d.Tags, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w, d.Content)
}

func writeEntries(w io.Writer, entries []taskstore.Entry) {
	statusLabel := map[string]string{
		taskstore.StatusPending:    "pending",
		taskstore.StatusInProgress: "in-progress",
		taskstore.StatusBlocked:    "blocked ⚠",
		taskstore.StatusDone:       "done ✓",
	}
	for _, e := range entries {
		label := statusLabel[e.Status]
		if label == "" {
			label = e.Status
		}
		fmt.Fprintf(w, "[%s] %s — %s\n", label, e.ID, e.Title)
		if len(e.Repos) > 0 {
			fmt.Fprintf(w, "  repos: %s\n", strings.Join(e.Repos, ", "))
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(w, "  tags:  %s\n", strings.Join(e.Tags, ", "))
		}
		if len(e.Jira) > 0 {
			fmt.Fprintf(w, "  jira:  %s\n", strings.Join(e.Jira, ", "))
		}
		if e.DateTarget != "" {
			fmt.Fprintf(w, "  alvo:  %s\n", e.DateTarget)
		}
		if e.Description != "" {
			lines := strings.SplitN(strings.TrimSpace(e.Description), "\n", 3)
			fmt.Fprintf(w, "  desc:  %s\n", lines[0])
		}
		fmt.Fprintln(w)
	}
}

func writeSnapTable(w io.Writer, snap *repoindex.RepoSnapshot) {
	fmt.Fprintf(w, "REPO: %s (%s + %s) — %s\n\n", snap.Repo.Name, snap.Repo.Lang, snap.Repo.Framework, snap.Repo.Path)
	for _, sec := range []string{"handlers", "models", "apis", "events", "config"} {
		cols, rows := repoindex.SnapshotSection(snap, sec)
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(w, "── %s ──\n", strings.ToUpper(sec))
		fmt.Fprint(w, repoindex.RenderTable(cols, rows))
		fmt.Fprintln(w)
	}
}

// insertAfterSection mirrors the same helper in cmd/wtb/doc_cmd.go.
func insertAfterSection(content, section, addition string) string {
	lines := strings.Split(content, "\n")
	insertAt := -1
	inSection := false
	for i, line := range lines {
		if strings.TrimSpace(line) == strings.TrimSpace(section) {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "#") {
			insertAt = i
			break
		}
	}
	addition = "\n" + strings.TrimLeft(addition, "\n")
	if insertAt < 0 {
		return content + addition
	}
	before := strings.Join(lines[:insertAt], "\n")
	after := strings.Join(lines[insertAt:], "\n")
	return before + addition + "\n" + after
}
