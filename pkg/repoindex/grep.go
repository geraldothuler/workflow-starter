package repoindex

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// GrepMatch is a single line match in a repo's source file.
type GrepMatch struct {
	Repo    string
	File    string
	LineNum int
	Line    string
}

// GrepOptions configures a GrepRepos call.
type GrepOptions struct {
	Pattern    string   // regexp pattern
	RepoFilter string   // limit to one repo name (empty = all)
	ExtFilter  []string // e.g. [".kt", ".scala"] — empty = all source files
	ContextN   int      // lines of context before/after (0 = match line only)
	MaxMatches int      // 0 = unlimited
}

// GrepRepos searches source files of all indexed repos for the given regex.
// Skips .git, target, build, node_modules, .gradle dirs.
func GrepRepos(db *DB, opts GrepOptions) ([]GrepMatch, error) {
	repos, err := ListRepos(db)
	if err != nil {
		return nil, err
	}

	re, err := regexp.Compile(opts.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", opts.Pattern, err)
	}

	var matches []GrepMatch
	for _, repo := range repos {
		if opts.RepoFilter != "" && repo.Name != opts.RepoFilter {
			continue
		}
		if _, err := os.Stat(repo.Path); err != nil {
			continue
		}

		err := filepath.Walk(repo.Path, func(path string, fi os.FileInfo, err error) error {
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
			if len(opts.ExtFilter) > 0 {
				if !strSliceContains(opts.ExtFilter, ext) {
					return nil
				}
			} else if !isSourceExt(ext) {
				return nil
			}

			lineMatches, err := grepFile(path, re)
			if err != nil {
				return nil
			}
			for _, m := range lineMatches {
				matches = append(matches, GrepMatch{
					Repo:    repo.Name,
					File:    path,
					LineNum: m.lineNum,
					Line:    m.line,
				})
				if opts.MaxMatches > 0 && len(matches) >= opts.MaxMatches {
					return fmt.Errorf("maxmatches")
				}
			}
			return nil
		})
		if err != nil && err.Error() != "maxmatches" {
			// non-fatal: skip broken repos
		}
		if opts.MaxMatches > 0 && len(matches) >= opts.MaxMatches {
			break
		}
	}
	return matches, nil
}

type lineMatch struct {
	lineNum int
	line    string
}

func grepFile(path string, re *regexp.Regexp) ([]lineMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []lineMatch
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			out = append(out, lineMatch{lineNum: lineNum, line: strings.TrimSpace(line)})
		}
	}
	return out, scanner.Err()
}

func skipDir(name string) bool {
	switch name {
	case ".git", "target", "build", "node_modules", ".gradle", ".idea", "dist", ".next", "out":
		return true
	}
	return false
}

func isSourceExt(ext string) bool {
	switch ext {
	case ".kt", ".scala", ".java", ".ts", ".tsx", ".js", ".jsx",
		".go", ".py", ".rb", ".rs",
		".yml", ".yaml", ".properties", ".conf", ".toml",
		".json", ".sbt", ".gradle", ".kts":
		return true
	}
	return false
}

// RenderGrepTable renders grep results grouped by repo as a table.
func RenderGrepTable(matches []GrepMatch) string {
	if len(matches) == 0 {
		return "(no matches)\n"
	}
	cols := []string{"repo", "file", "line", "match"}
	var rows [][]string
	for _, m := range matches {
		line := m.Line
		if len(line) > 120 {
			line = line[:120] + "…"
		}
		rows = append(rows, []string{
			m.Repo,
			relativePath(m.File),
			fmt.Sprint(m.LineNum),
			line,
		})
	}
	return RenderTable(cols, rows)
}

// RenderGrepGrouped renders grep results grouped by repo, grep-style.
func RenderGrepGrouped(matches []GrepMatch) string {
	if len(matches) == 0 {
		return "(no matches)\n"
	}

	// Group by repo
	type group struct {
		repo    string
		matches []GrepMatch
	}
	order := []string{}
	byRepo := map[string][]GrepMatch{}
	for _, m := range matches {
		if _, ok := byRepo[m.Repo]; !ok {
			order = append(order, m.Repo)
		}
		byRepo[m.Repo] = append(byRepo[m.Repo], m)
	}
	sort.Strings(order)

	var sb strings.Builder
	for _, repo := range order {
		ms := byRepo[repo]
		sb.WriteString(fmt.Sprintf("\n── %s (%d match(es)) ──\n", repo, len(ms)))
		for _, m := range ms {
			line := m.Line
			if len(line) > 140 {
				line = line[:140] + "…"
			}
			sb.WriteString(fmt.Sprintf("  %s:%d: %s\n", relativePath(m.File), m.LineNum, line))
		}
	}
	return sb.String()
}

func relativePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	rel := strings.TrimPrefix(path, home+"/Cobliteam/")
	return rel
}
