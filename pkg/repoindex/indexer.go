package repoindex

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/llm"
)

// IndexOptions controls indexing behavior.
type IndexOptions struct {
	RepoName string
	RepoPath string
	Owner    string // squad/team owner — populated from --owner flag or architecture summary.md
	LLM      llm.Completer
	Force    bool      // skip hash check, re-index everything
	Verbose  bool
	Output   io.Writer // verbose output destination; nil → os.Stdout
}

// IndexResult reports what happened during indexing.
type IndexResult struct {
	RepoName      string
	LayersIndexed []string
	LayersSkipped []string
	TotalCost     float64
	Error         error
}

// IndexRepo indexes a repository into repos.db.
// It detects the parser, checks file hashes, bundles changed layers,
// calls the LLM in parallel, and persists all extracted entities.
func IndexRepo(db *DB, opts IndexOptions) IndexResult {
	result := IndexResult{RepoName: opts.RepoName}

	parser := DetectParser(opts.RepoPath)
	if parser == nil {
		result.Error = fmt.Errorf("no parser found for repo at %s", opts.RepoPath)
		return result
	}

	// Upsert repo record.
	repoID := slugID(opts.RepoName)
	owner := opts.Owner
	if owner == "" {
		owner = readOwnerFromArchitecture(opts.RepoPath, opts.RepoName)
	}
	ciPlatform := detectCIPlatform(opts.RepoPath)
	if err := upsertRepo(db.sql, repoID, opts.RepoName, opts.RepoPath, parser.Lang(), parser.Framework(), owner, ciPlatform); err != nil {
		result.Error = fmt.Errorf("upsert repo: %w", err)
		return result
	}

	layers, err := parser.Layers(opts.RepoPath)
	if err != nil {
		result.Error = fmt.Errorf("get layers: %w", err)
		return result
	}

	// Determine which layers need re-indexing based on file hashes.
	type layerWork struct {
		layer   Layer
		content string
	}
	var toIndex []layerWork

	for _, layer := range layers {
		changed, content := layerNeedsReindex(db.sql, repoID, layer, opts.Force)
		if changed {
			toIndex = append(toIndex, layerWork{layer, content})
			result.LayersIndexed = append(result.LayersIndexed, layer.Name)
		} else {
			result.LayersSkipped = append(result.LayersSkipped, layer.Name)
		}
	}

	if len(toIndex) == 0 {
		return result
	}

	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	if opts.Verbose {
		fmt.Fprintf(out, "indexing %d layer(s): %s\n", len(toIndex), strings.Join(result.LayersIndexed, ", "))
	}

	// Fan out LLM calls in parallel.
	type layerResult struct {
		name      string
		extracted ExtractedLayer
		files     []string
		err       error
	}
	results := make([]layerResult, len(toIndex))
	var wg sync.WaitGroup

	// Set system prompt once if the LLM supports it.
	if sp, ok := opts.LLM.(interface{ SetSystemPrompt(string) }); ok {
		sp.SetSystemPrompt(parser.SystemPrompt())
	}

	for i, work := range toIndex {
		wg.Add(1)
		go func(i int, work layerWork) {
			defer wg.Done()
			prompt := parser.LayerPrompt(work.layer.Name, work.content)
			raw, err := opts.LLM.Complete(prompt, 8000)
			if err != nil {
				results[i] = layerResult{name: work.layer.Name, files: work.layer.Files, err: err}
				return
			}
			var extracted ExtractedLayer
			if err := parseJSON(raw, &extracted); err != nil {
				results[i] = layerResult{name: work.layer.Name, files: work.layer.Files, err: fmt.Errorf("parse layer %s: %w\nraw: %.200s", work.layer.Name, err, raw)}
				return
			}
			extracted = sanitize(extracted)
			results[i] = layerResult{name: work.layer.Name, extracted: extracted, files: work.layer.Files}
		}(i, work)
	}
	wg.Wait()

	// Persist each layer result.
	for _, lr := range results {
		if lr.err != nil {
			if opts.Verbose {
				fmt.Fprintf(out, "layer %s error: %v\n", lr.name, lr.err)
			}
			continue
		}
		if err := persistLayer(db.sql, repoID, lr.extracted); err != nil {
			if opts.Verbose {
				fmt.Fprintf(out, "persist layer %s: %v\n", lr.name, err)
			}
			continue
		}
		// Update hashes for successfully indexed files.
		for _, f := range lr.files {
			h := fileHash(f)
			if h != "" {
				saveHash(db.sql, repoID, f, h)
			}
		}
	}

	// Stamp last_indexed_at.
	db.sql.Exec(`UPDATE repos SET last_indexed_at=? WHERE id=?`, time.Now().Format(time.RFC3339), repoID)

	// Detect secondary stack (Python/LangChain, etc.) deterministically.
	if sl, sf := detectSecondaryStack(opts.RepoPath); sl != "" {
		db.sql.Exec(`UPDATE repos SET secondary_lang=?, secondary_framework=? WHERE id=?`, sl, sf, repoID)
	}

	// Deterministic Kafka topic detection for Kotlin repos — supplements LLM extraction.
	if parser.Lang() == "kotlin" {
		detected := DetectKafkaTopics(opts.RepoPath)
		mergeDetectedTopics(db, repoID, detected)
	}

	return result
}

// detectSecondaryStack scans for requirements.txt or pyproject.toml anywhere in
// the repo (excluding target/ and .git/) and infers a secondary language/framework.
// Returns ("", "") when no secondary stack is detected.
func detectSecondaryStack(repoPath string) (lang, framework string) {
	var reqContent string
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip generated/vendor directories
		if fi.IsDir() {
			name := fi.Name()
			if name == "target" || name == ".git" || name == "node_modules" || name == ".gradle" {
				return filepath.SkipDir
			}
			return nil
		}
		name := fi.Name()
		if name == "requirements.txt" || name == "requirements-prod.txt" || name == "pyproject.toml" {
			data, err := os.ReadFile(path)
			if err == nil {
				reqContent += strings.ToLower(string(data))
			}
		}
		return nil
	})
	if reqContent == "" {
		return "", ""
	}
	// Skip deploy-only Python tooling (boto3/awscli without app frameworks)
	hasAppFramework := strings.Contains(reqContent, "langchain") ||
		strings.Contains(reqContent, "pyspark") ||
		strings.Contains(reqContent, "django") ||
		strings.Contains(reqContent, "fastapi") ||
		strings.Contains(reqContent, "flask") ||
		strings.Contains(reqContent, "openai") ||
		strings.Contains(reqContent, "faiss")
	if !hasAppFramework {
		return "", ""
	}

	lang = "python"
	switch {
	case strings.Contains(reqContent, "langchain"):
		framework = "langchain"
	case strings.Contains(reqContent, "pyspark"):
		framework = "spark"
	case strings.Contains(reqContent, "django"):
		framework = "django"
	case strings.Contains(reqContent, "fastapi"):
		framework = "fastapi"
	case strings.Contains(reqContent, "flask"):
		framework = "flask"
	default:
		framework = "python"
	}
	return lang, framework
}

// layerNeedsReindex returns true if any file in the layer changed since last index.
// Also returns the bundled file content ready for the LLM prompt.
func layerNeedsReindex(sqldb *sql.DB, repoID string, layer Layer, force bool) (bool, string) {
	content := bundleFiles(layer.Files)
	if force {
		return true, content
	}
	for _, f := range layer.Files {
		current := fileHash(f)
		if current == "" {
			continue
		}
		var saved string
		sqldb.QueryRow(`SELECT sha256 FROM file_hashes WHERE repo_id=? AND path=?`, repoID, f).Scan(&saved)
		if saved != current {
			return true, content
		}
	}
	return false, ""
}

// fileHash returns the SHA-256 hex of a file, or "" on error.
func fileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func saveHash(sqldb *sql.DB, repoID, path, sha string) {
	sqldb.Exec(`
		INSERT INTO file_hashes(repo_id,path,sha256,indexed_at)
		VALUES(?,?,?,?)
		ON CONFLICT(repo_id,path) DO UPDATE SET sha256=excluded.sha256, indexed_at=excluded.indexed_at`,
		repoID, path, sha, time.Now().Format(time.RFC3339))
}

// persistLayer writes all extracted entities for a repo, replacing previous data for changed layers.
func persistLayer(sqldb *sql.DB, repoID string, extracted ExtractedLayer) error {
	tx, err := sqldb.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)

	for _, h := range extracted.Handlers {
		id := slugID(repoID + "-handler-" + h.Name)
		vpc := 0
		if h.VPC {
			vpc = 1
		}
		tx.Exec(`INSERT INTO handlers(id,repo_id,name,handler_file,trigger_type,trigger_detail,timeout,max_retry,concurrency,vpc,description)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET
			  handler_file=excluded.handler_file, trigger_type=excluded.trigger_type,
			  trigger_detail=excluded.trigger_detail, timeout=excluded.timeout,
			  max_retry=excluded.max_retry, concurrency=excluded.concurrency,
			  vpc=excluded.vpc, description=excluded.description`,
			id, repoID, h.Name, h.HandlerFile, h.TriggerType, h.TriggerDetail,
			h.Timeout, h.MaxRetry, h.Concurrency, vpc, h.Description)
	}

	for _, e := range extracted.Events {
		id := slugID(repoID + "-event-" + e.Name)
		tx.Exec(`INSERT INTO events(id,repo_id,name,event_type,detail_type,bus_name,description)
			VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET
			  event_type=excluded.event_type, detail_type=excluded.detail_type,
			  bus_name=excluded.bus_name, description=excluded.description`,
			id, repoID, e.Name, e.EventType, e.DetailType, e.BusName, e.Description)
	}

	for _, m := range extracted.Models {
		modelID := slugID(repoID + "-model-" + m.Name)
		tx.Exec(`INSERT INTO models(id,repo_id,name,table_name,dialect)
			VALUES(?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET table_name=excluded.table_name, dialect=excluded.dialect`,
			modelID, repoID, m.Name, m.TableName, m.Dialect)

		for _, f := range m.Fields {
			fid := slugID(modelID + "-field-" + f.Name)
			nullable, pk, uniq := boolInt(f.Nullable), boolInt(f.PrimaryKey), boolInt(f.Unique)
			tx.Exec(`INSERT INTO model_fields(id,model_id,name,type,nullable,primary_key,unique_field)
				VALUES(?,?,?,?,?,?,?)
				ON CONFLICT(id) DO UPDATE SET type=excluded.type,nullable=excluded.nullable,primary_key=excluded.primary_key,unique_field=excluded.unique_field`,
				fid, modelID, f.Name, f.Type, nullable, pk, uniq)
		}

		for _, a := range m.Associations {
			aid := slugID(modelID + "-assoc-" + a.AssocType + "-" + a.TargetModel)
			tx.Exec(`INSERT INTO model_associations(id,model_id,assoc_type,target_model,foreign_key)
				VALUES(?,?,?,?,?)
				ON CONFLICT(id) DO UPDATE SET assoc_type=excluded.assoc_type,target_model=excluded.target_model,foreign_key=excluded.foreign_key`,
				aid, modelID, a.AssocType, a.TargetModel, a.ForeignKey)
		}
	}

	for _, a := range extracted.ExternalAPIs {
		id := slugID(repoID + "-api-" + a.Name)
		tx.Exec(`INSERT INTO external_apis(id,repo_id,name,url,method,auth_type,description)
			VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET url=excluded.url,method=excluded.method,auth_type=excluded.auth_type,description=excluded.description`,
			id, repoID, a.Name, a.URL, a.Method, a.AuthType, a.Description)
	}

	for _, d := range extracted.DBConnections {
		id := slugID(repoID + "-db-" + d.Dialect + "-" + d.HostVar)
		tx.Exec(`INSERT INTO db_connections(id,repo_id,dialect,host_var,pool_min,pool_max,pool_idle)
			VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET dialect=excluded.dialect,host_var=excluded.host_var,pool_min=excluded.pool_min,pool_max=excluded.pool_max,pool_idle=excluded.pool_idle`,
			id, repoID, d.Dialect, d.HostVar, d.PoolMin, d.PoolMax, d.PoolIdle)
	}

	for _, c := range extracted.ConfigVars {
		id := slugID(repoID + "-cfg-" + c.Key)
		tx.Exec(`INSERT INTO config_vars(id,repo_id,key,source,description)
			VALUES(?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET source=excluded.source,description=excluded.description`,
			id, repoID, c.Key, c.Source, c.Description)
	}

	_ = now
	return tx.Commit()
}

func upsertRepo(sqldb *sql.DB, id, name, path, lang, framework, owner, ciPlatform string) error {
	_, err := sqldb.Exec(`
		INSERT INTO repos(id,name,path,lang,framework,owner,last_indexed_at,ci_platform)
		VALUES(?,?,?,?,?,?,'',?)
		ON CONFLICT(id) DO UPDATE SET path=excluded.path, lang=excluded.lang, framework=excluded.framework,
		  owner=CASE WHEN excluded.owner != '' THEN excluded.owner ELSE repos.owner END,
		  ci_platform=CASE WHEN excluded.ci_platform != '' THEN excluded.ci_platform ELSE repos.ci_platform END`,
		id, name, path, lang, framework, owner, ciPlatform)
	return err
}

// detectCIPlatform returns "circleci", "github-actions", or "circleci" (default for Cobli repos).
func detectCIPlatform(repoPath string) string {
	if _, err := os.Stat(filepath.Join(repoPath, ".github", "workflows")); err == nil {
		return "github-actions"
	}
	return "circleci"
}

// readOwnerFromArchitecture reads the owner from Cobliteam/architecture summary.md if present.
// Looks for lines like: **Owner:** value, | Owner Team | value |, - **Owner Team:** value
func readOwnerFromArchitecture(repoPath, repoName string) string {
	archBase := filepath.Join(filepath.Dir(repoPath), "architecture", "docs")
	for _, category := range []string{"apis", "jobs", "gateways", "frontend", "apps"} {
		summaryPath := filepath.Join(archBase, category, repoName, "summary.md")
		data, err := os.ReadFile(summaryPath)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			var key, value string
			if strings.Contains(line, "|") {
				// Table format: | Owner Team | value |
				parts := strings.Split(line, "|")
				if len(parts) >= 3 {
					key = strings.ToLower(strings.Trim(parts[1], "*_ "))
					value = strings.TrimSpace(parts[2])
					value = strings.Trim(value, "*_ ")
				}
			} else if idx := strings.Index(line, ":"); idx >= 0 {
				// Colon format: **Owner:** value, Owner Team: value
				rawKey := line[:idx]
				key = strings.ToLower(strings.Trim(rawKey, "-*_ "))
				value = strings.TrimSpace(line[idx+1:])
				value = strings.Trim(value, "*_ ")
			}
			// Key must start with "owner" (not "ownership" in URLs)
			if !strings.HasPrefix(key, "owner") && !strings.HasPrefix(key, "team owner") {
				continue
			}
			// Strip parenthetical suffixes like "fieldOps (Monitoring)"
			if idx := strings.Index(value, "("); idx > 0 {
				value = strings.TrimSpace(value[:idx])
			}
			if value != "" && strings.ToLower(value) != "unknown" && !strings.HasPrefix(value, "http") {
				return value
			}
		}
	}
	return ""
}

// slugID creates a deterministic short ID from a string (hex prefix of SHA-256).
func slugID(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseJSON strips markdown fences from LLM output and unmarshals JSON.
func parseJSON(raw string, v interface{}) error {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.SplitN(raw, "\n", 2)
		if len(lines) > 1 {
			raw = lines[1]
		}
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}
	return json.Unmarshal([]byte(raw), v)
}
