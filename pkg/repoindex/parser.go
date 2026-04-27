package repoindex

import (
	"os"
	"path/filepath"
	"strings"
)

// isScalaPlay returns true when a build.sbt contains markers specific to
// Play Framework (ProdServerStart entrypoint or the alexstrasza-core-play lib).
func isScalaPlay(repoPath string) bool {
	data, err := os.ReadFile(filepath.Join(repoPath, "build.sbt"))
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, "ProdServerStart") ||
		strings.Contains(content, "alexstrasza-core-play")
}


// RepoParser is the pluggable interface for per-language/framework indexing.
// Implement this to add support for a new repo type (Scala, Kotlin, etc.).
//
// Layers() returns the file groups to be fed to the LLM, one call per layer.
// The core indexer handles hashing, LLM dispatch, and DB insertion.
type RepoParser interface {
	// Lang returns the primary language identifier (e.g. "typescript", "scala").
	Lang() string

	// Framework returns the framework identifier (e.g. "serverless", "play", "spring").
	Framework() string

	// Layers returns the ordered list of file groups to index.
	// Each layer is fed to the LLM as a single bundled context.
	Layers(repoPath string) ([]Layer, error)

	// SystemPrompt returns the LLM system prompt for this parser.
	// It describes the language/framework conventions to the LLM.
	SystemPrompt() string

	// LayerPrompt returns the user prompt for a specific layer given its bundled content.
	LayerPrompt(layerName, content string) string
}

// DetectParser returns the appropriate RepoParser for a given repo path.
// It uses simple heuristics (file presence) to identify the stack.
// Returns nil for library repos (no deployable job entrypoint detected).
func DetectParser(repoPath string) RepoParser {
	if fileExists(repoPath, "serverless.yml") || fileExists(repoPath, "serverless.ts") {
		return &TypeScriptServerlessParser{}
	}
	if fileExists(repoPath, "build.sbt") {
		// Exclude shared library repos: no src/main/resources (no app config) and
		// no top-level main Scala files — only sub-module builds with shared code.
		if isScalaLib(repoPath) {
			return nil
		}
		// Detect Play Framework before defaulting to Flink.
		if isScalaPlay(repoPath) {
			return &ScalaPlayParser{}
		}
		return &ScalaFlinkParser{}
	}
	if fileExists(repoPath, "build.gradle.kts") || fileExists(repoPath, "build.gradle") {
		if isKotlinFlink(repoPath) {
			return &KotlinFlinkParser{}
		}
		return &KotlinSpringParser{}
	}
	// Schema registry: repo with a protos/ directory containing .proto files.
	if isDir(filepath.Join(repoPath, "protos")) {
		return &ProtoSchemaParser{}
	}
	// Node.js / TypeScript repos (package.json without serverless.yml).
	if fileExists(repoPath, "package.json") {
		return &NodeJSParser{}
	}
	// Flutter / Dart apps (pubspec.yaml).
	if fileExists(repoPath, "pubspec.yaml") {
		return &FlutterDartParser{}
	}
	// Python web services: root requirements.txt / pyproject.toml / setup.py,
	// OR multi-module layout where a top-level subdir contains wsgi.py.
	if isPythonRepo(repoPath) {
		return &PythonFlaskParser{repoPath: repoPath}
	}
	return nil
}

// isPythonRepo returns true when the repo appears to be a Python web service.
// Heuristics:
//  1. Root-level requirements.txt, pyproject.toml, or setup.py
//  2. Any immediate subdirectory contains wsgi.py (multi-module Flask pattern)
func isPythonRepo(repoPath string) bool {
	for _, f := range []string{"requirements.txt", "pyproject.toml", "setup.py"} {
		if fileExists(repoPath, f) {
			return true
		}
	}
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wsgi := filepath.Join(repoPath, e.Name(), "wsgi.py")
		if _, err := os.Stat(wsgi); err == nil {
			return true
		}
	}
	return false
}

// isKotlinFlink returns true when a Kotlin repo uses Apache Flink as its runtime
// rather than Spring Boot. Heuristic: any build.gradle.kts in the repo (root or
// module) declares a dependency on org.apache.flink.
func isKotlinFlink(repoPath string) bool {
	found := false
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || found {
			return nil
		}
		if fi.Name() != "build.gradle.kts" && fi.Name() != "build.gradle" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), "org.apache.flink") ||
			strings.Contains(string(data), "flink-streaming") ||
			strings.Contains(string(data), "flink-clients") {
			found = true
		}
		return nil
	})
	return found
}

// isScalaLib returns true when a Scala repo looks like a shared library rather
// than a deployable Flink job — heuristic: no application.conf anywhere under
// src/ (only target/ generated files) and no App/main object at root source level.
func isScalaLib(repoPath string) bool {
	hasAppConf := false
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if fi.Name() == "application.conf" && !strings.Contains(path, "/target/") {
			hasAppConf = true
			return filepath.SkipAll
		}
		return nil
	})
	return !hasAppConf
}
