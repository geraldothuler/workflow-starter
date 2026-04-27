package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SchemaDep records how many times a repo references a schema-registry proto model.
type SchemaDep struct {
	RepoName   string
	ModelName  string
	MatchCount int
}

// IndexSchemaDeps scans every indexed repo's source files for occurrences of
// schema-registry proto model names and upserts results into schema_deps.
// Returns the number of (repo, model) pairs recorded.
func IndexSchemaDeps(db *DB) (int, error) {
	models, err := queryProtoModelNames(db)
	if err != nil {
		return 0, err
	}
	if len(models) == 0 {
		return 0, fmt.Errorf("no proto models indexed — run: wtb repo index schema-registry first")
	}

	// Drop names too short to be meaningful (avoid noise from e.g. "Id", "Key").
	var candidates []string
	for _, m := range models {
		if len(m) >= 5 {
			candidates = append(candidates, m)
		}
	}

	repos, err := ListRepos(db)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, repo := range repos {
		if repo.Framework == "schema-registry" {
			continue
		}
		counts, err := grepModelUsages(repo.Path, candidates)
		if err != nil {
			continue
		}
		for modelName, count := range counts {
			if count == 0 {
				continue
			}
			if err := upsertSchemaDep(db, repo.Name, modelName, count); err != nil {
				return total, err
			}
			total++
		}
	}
	return total, nil
}

// QuerySchemaDeps returns schema_deps filtered by optional repo/model substrings.
func QuerySchemaDeps(db *DB, repoFilter, modelFilter string) ([]SchemaDep, error) {
	q := `SELECT repo_name, model_name, match_count FROM schema_deps WHERE 1=1`
	var args []interface{}
	if repoFilter != "" {
		q += ` AND repo_name LIKE ?`
		args = append(args, "%"+repoFilter+"%")
	}
	if modelFilter != "" {
		q += ` AND model_name LIKE ?`
		args = append(args, "%"+modelFilter+"%")
	}
	q += ` ORDER BY repo_name, match_count DESC`

	rows, err := db.Raw().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []SchemaDep
	for rows.Next() {
		var d SchemaDep
		if err := rows.Scan(&d.RepoName, &d.ModelName, &d.MatchCount); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

// QuerySchemaDepsByRepo returns proto contracts used by a single repo, sorted by match count.
func QuerySchemaDepsByRepo(db *DB, repoName string) ([]SchemaDep, error) {
	rows, err := db.Raw().Query(
		`SELECT repo_name, model_name, match_count FROM schema_deps WHERE repo_name=? ORDER BY match_count DESC`,
		repoName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []SchemaDep
	for rows.Next() {
		var d SchemaDep
		if err := rows.Scan(&d.RepoName, &d.ModelName, &d.MatchCount); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

// RenderSchemaDepsByRepo groups deps by repo — shows which contracts each repo uses.
func RenderSchemaDepsByRepo(deps []SchemaDep) string {
	byRepo := map[string][]SchemaDep{}
	var repoOrder []string
	for _, d := range deps {
		if _, ok := byRepo[d.RepoName]; !ok {
			repoOrder = append(repoOrder, d.RepoName)
		}
		byRepo[d.RepoName] = append(byRepo[d.RepoName], d)
	}

	var sb strings.Builder
	for _, repo := range repoOrder {
		sb.WriteString(fmt.Sprintf("\n%s\n", repo))
		for _, d := range byRepo[repo] {
			sb.WriteString(fmt.Sprintf("  %-40s %d matches\n", d.ModelName, d.MatchCount))
		}
	}
	return sb.String()
}

// RenderSchemaDepsByModel groups deps by model — shows which repos use each contract.
func RenderSchemaDepsByModel(deps []SchemaDep) string {
	byModel := map[string][]SchemaDep{}
	var modelOrder []string
	for _, d := range deps {
		if _, ok := byModel[d.ModelName]; !ok {
			modelOrder = append(modelOrder, d.ModelName)
		}
		byModel[d.ModelName] = append(byModel[d.ModelName], d)
	}
	sort.Strings(modelOrder)

	var sb strings.Builder
	for _, model := range modelOrder {
		rs := byModel[model]
		sort.Slice(rs, func(i, j int) bool { return rs[i].MatchCount > rs[j].MatchCount })
		sb.WriteString(fmt.Sprintf("\n%s\n", model))
		for _, d := range rs {
			sb.WriteString(fmt.Sprintf("  %-50s %d\n", d.RepoName, d.MatchCount))
		}
	}
	return sb.String()
}

// ── internals ─────────────────────────────────────────────────────────────────

// queryProtoModelNames returns all distinct model names with dialect="protobuf".
func queryProtoModelNames(db *DB) ([]string, error) {
	rows, err := db.Raw().Query(`
		SELECT DISTINCT m.name
		FROM models m
		JOIN repos r ON r.id = m.repo_id
		WHERE m.dialect = 'protobuf'
		ORDER BY m.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// grepModelUsages reads every source file under repoPath once and counts
// string occurrences of each candidate model name.
func grepModelUsages(repoPath string, candidates []string) (map[string]int, error) {
	counts := map[string]int{}
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
		if !isSourceExt(filepath.Ext(path)) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		for _, name := range candidates {
			if strings.Contains(content, name) {
				counts[name] += strings.Count(content, name)
			}
		}
		return nil
	})
	return counts, err
}

func upsertSchemaDep(db *DB, repoName, modelName string, count int) error {
	_, err := db.Raw().Exec(`
		INSERT INTO schema_deps (repo_name, model_name, match_count)
		VALUES (?, ?, ?)
		ON CONFLICT (repo_name, model_name) DO UPDATE SET match_count = excluded.match_count
	`, repoName, modelName, count)
	return err
}
