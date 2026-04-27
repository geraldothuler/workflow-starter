package repoindex

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ── Types ──────────────────────────────────────────────────────────────────────

// InternalHTTPCall represents a synchronous HTTP call from one indexed repo to another.
type InternalHTTPCall struct {
	FromRepo string
	ToRepo   string // matched indexed repo name
	Via      string // "FeignClient" | "RestTemplate" | "WebClient"
	Detail   string // matched name or URL fragment
}

// SharedDBTable represents a DB table_name found in 2+ repos of the merged set.
type SharedDBTable struct {
	TableName     string
	MergedRepos   []string // repos within the merge set that have this table
	ExternalRepos []string // other indexed repos that also have this table
}

// SharedProtoContract represents a proto model used by 2+ repos in the merged set.
type SharedProtoContract struct {
	ModelName string
	Repos     []string // repos within the merge set that reference this model
}

// SharedConfigKey represents a config key used by this repo and at least one other.
type SharedConfigKey struct {
	Key   string
	Repos []string
}

// ── Gap 3: Internal HTTP calls ─────────────────────────────────────────────────

var (
	reFeignBlock  = regexp.MustCompile(`(?s)@FeignClient\s*\(([^)]+)\)`)
	reFeignName   = regexp.MustCompile(`name\s*=\s*"([^"]+)"`)
	reFeignURL    = regexp.MustCompile(`url\s*=\s*"([^"$\{][^"]*)"`)
	reRestLiteral = regexp.MustCompile(`restTemplate\.\w+\s*\(\s*"(https?://[^"]+)"`)
	reWebCreate   = regexp.MustCompile(`WebClient\.create\s*\(\s*"(https?://[^"]+)"`)
	reWebURI      = regexp.MustCompile(`\.uri\s*\(\s*"(https?://[^"]+)"`)
)

type httpFinding struct {
	via    string
	target string
}

// detectInternalHTTPCalls scans source files of the merged repos for HTTP calls
// that target other indexed repos, using annotation-aware regex patterns.
func detectInternalHTTPCalls(db *DB, repoNames []string) ([]InternalHTTPCall, error) {
	allRepos, err := ListRepos(db)
	if err != nil {
		return nil, err
	}

	// Build path map for repos being analyzed
	mergedSet := map[string]bool{}
	for _, n := range repoNames {
		mergedSet[n] = true
	}

	var calls []InternalHTTPCall
	seen := map[string]bool{}

	for _, repo := range allRepos {
		if !mergedSet[repo.Name] {
			continue
		}
		if _, err := os.Stat(repo.Path); err != nil {
			continue
		}

		findings, err := scanRepoForHTTPCalls(repo.Path)
		if err != nil {
			continue
		}

		for _, f := range findings {
			for _, target := range allRepos {
				if target.Name == repo.Name {
					continue
				}
				if !urlMatchesRepo(f.target, target) {
					continue
				}
				key := repo.Name + "→" + target.Name + ":" + f.via
				if seen[key] {
					continue
				}
				seen[key] = true
				calls = append(calls, InternalHTTPCall{
					FromRepo: repo.Name,
					ToRepo:   target.Name,
					Via:      f.via,
					Detail:   f.target,
				})
			}
		}
	}

	sort.Slice(calls, func(i, j int) bool {
		if calls[i].FromRepo != calls[j].FromRepo {
			return calls[i].FromRepo < calls[j].FromRepo
		}
		return calls[i].ToRepo < calls[j].ToRepo
	})
	return calls, nil
}

// scanRepoForHTTPCalls walks .kt source files and extracts HTTP call targets.
func scanRepoForHTTPCalls(repoPath string) ([]httpFinding, error) {
	var out []httpFinding
	err := filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			if skipDir(fi.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".kt" && ext != ".java" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		out = append(out, extractHTTPFindings(string(data))...)
		return nil
	})
	return out, err
}

// extractHTTPFindings applies all HTTP-detection regexes to file content.
func extractHTTPFindings(content string) []httpFinding {
	var out []httpFinding

	// @FeignClient blocks (may span multiple lines)
	for _, block := range reFeignBlock.FindAllStringSubmatch(content, -1) {
		inner := block[1]
		if m := reFeignName.FindStringSubmatch(inner); m != nil {
			out = append(out, httpFinding{via: "FeignClient", target: m[1]})
		} else if m := reFeignURL.FindStringSubmatch(inner); m != nil {
			out = append(out, httpFinding{via: "FeignClient", target: m[1]})
		}
	}

	// restTemplate.*(url_literal)
	for _, m := range reRestLiteral.FindAllStringSubmatch(content, -1) {
		out = append(out, httpFinding{via: "RestTemplate", target: m[1]})
	}

	// WebClient.create(url_literal)
	for _, m := range reWebCreate.FindAllStringSubmatch(content, -1) {
		out = append(out, httpFinding{via: "WebClient", target: m[1]})
	}

	// .uri(url_literal) — only when URL is absolute (http/https)
	for _, m := range reWebURI.FindAllStringSubmatch(content, -1) {
		out = append(out, httpFinding{via: "WebClient", target: m[1]})
	}

	return out
}

// urlMatchesRepo returns true if the given URL or service name references the repo.
func urlMatchesRepo(urlOrName string, repo Repo) bool {
	t := strings.ToLower(urlOrName)
	name := strings.ToLower(repo.Name)

	if strings.Contains(t, name) {
		return true
	}
	// Without -api suffix (e.g. "fusca" matches "fusca-api")
	base := strings.TrimSuffix(name, "-api")
	if len(base) >= 4 && strings.Contains(t, base) {
		return true
	}
	// dd_service_name if set
	if repo.DDServiceName != "" {
		svc := strings.ToLower(repo.DDServiceName)
		if len(svc) >= 4 && strings.Contains(t, svc) {
			return true
		}
	}
	return false
}

// ── Gap 2: Cross-repo DB models ────────────────────────────────────────────────

// detectSharedDBTables finds table_names that appear in 2+ repos of the merged set,
// and also flags tables shared with repos outside the merged set.
func detectSharedDBTables(db *DB, repoNames []string) ([]SharedDBTable, error) {
	if len(repoNames) < 2 {
		return nil, nil
	}

	// Fetch all postgres/JPA models across all repos
	rows, err := db.Raw().Query(`
		SELECT m.table_name, r.name
		FROM models m
		JOIN repos r ON r.id = m.repo_id
		WHERE m.dialect IN ('postgres', 'postgresql', 'jpa', 'cassandra', 'scylla')
		  AND m.table_name != ''
		ORDER BY m.table_name, r.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type entry struct {
		merged   []string
		external []string
	}
	tables := map[string]*entry{}

	mergedSet := map[string]bool{}
	for _, n := range repoNames {
		mergedSet[n] = true
	}

	for rows.Next() {
		var tableName, repoName string
		if err := rows.Scan(&tableName, &repoName); err != nil {
			continue
		}
		if _, ok := tables[tableName]; !ok {
			tables[tableName] = &entry{}
		}
		if mergedSet[repoName] {
			tables[tableName].merged = append(tables[tableName].merged, repoName)
		} else {
			tables[tableName].external = append(tables[tableName].external, repoName)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []SharedDBTable
	for tableName, e := range tables {
		if len(e.merged) < 2 {
			continue // only report tables found in 2+ repos of the merged set
		}
		sort.Strings(e.merged)
		sort.Strings(e.external)
		result = append(result, SharedDBTable{
			TableName:     tableName,
			MergedRepos:   e.merged,
			ExternalRepos: e.external,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].TableName < result[j].TableName })
	return result, nil
}

// ── Gap 1: Proto contracts in ImpactResult ─────────────────────────────────────

// detectSharedProtoContracts returns proto models referenced by 2+ repos in the merged set.
func detectSharedProtoContracts(db *DB, repoNames []string) ([]SharedProtoContract, error) {
	if len(repoNames) < 2 {
		return nil, nil
	}

	rows, err := db.Raw().Query(`
		SELECT model_name, repo_name
		FROM schema_deps
		WHERE repo_name IN (` + placeholders(len(repoNames)) + `)
		  AND match_count > 0
		ORDER BY model_name, repo_name
	`, stringsToArgs(repoNames)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byModel := map[string][]string{}
	for rows.Next() {
		var model, repo string
		if err := rows.Scan(&model, &repo); err != nil {
			continue
		}
		byModel[model] = append(byModel[model], repo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []SharedProtoContract
	for model, repos := range byModel {
		if len(repos) < 2 {
			continue
		}
		sort.Strings(repos)
		result = append(result, SharedProtoContract{
			ModelName: model,
			Repos:     repos,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ModelName < result[j].ModelName })
	return result, nil
}

// ── Gap 4: Shared config keys (Canvas) ────────────────────────────────────────

// detectSharedConfigKeys returns config keys used by repoName and at least one other repo.
func detectSharedConfigKeys(db *DB, repoName string) ([]SharedConfigKey, error) {
	rows, err := db.Raw().Query(`
		SELECT cv.key, r.name
		FROM config_vars cv
		JOIN repos r ON r.id = cv.repo_id
		WHERE cv.source IN ('ssm', 'env')
		  AND cv.key IN (
		      SELECT cv2.key
		      FROM config_vars cv2
		      JOIN repos r2 ON r2.id = cv2.repo_id
		      WHERE r2.name = ?
		        AND cv2.source IN ('ssm', 'env')
		  )
		ORDER BY cv.key, r.name
	`, repoName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byKey := map[string][]string{}
	for rows.Next() {
		var key, repo string
		if err := rows.Scan(&key, &repo); err != nil {
			continue
		}
		byKey[key] = append(byKey[key], repo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []SharedConfigKey
	for key, repos := range byKey {
		if len(repos) < 2 {
			continue
		}
		sort.Strings(repos)
		result = append(result, SharedConfigKey{Key: key, Repos: repos})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

// ── helpers ────────────────────────────────────────────────────────────────────

// placeholders returns n comma-separated "?" for SQL IN clauses.
func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	sb := strings.Builder{}
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('?')
	}
	return sb.String()
}

// stringsToArgs converts []string to []interface{} for variadic sql args.
func stringsToArgs(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
