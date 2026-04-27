package repoindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NodeJSParser handles Node.js / TypeScript repos:
//   - GraphQL BFF (Apollo, janus-style)
//   - REST APIs (Express, herbie-api-style)
//   - AI Agent frameworks (Mastra, ai-style)
//   - Frontend SPAs (React, herbie-dashboard-style)
type NodeJSParser struct {
	framework string // detected sub-framework
}

func (p *NodeJSParser) Lang() string { return "typescript" }

func (p *NodeJSParser) Framework() string {
	if p.framework != "" {
		return p.framework
	}
	return "node"
}

// Layers returns file groups for LLM indexing.
// Adapts to the repo structure via heuristics on package.json and directory layout.
func (p *NodeJSParser) Layers(repoPath string) ([]Layer, error) {
	p.detectFramework(repoPath)

	var layers []Layer

	// Layer 1: infra — deployment/k8s configs + package.json + entry point
	infra := Layer{Name: "infra"}
	infra.Files = append(infra.Files, p.findInfraFiles(repoPath)...)
	if len(infra.Files) > 0 {
		layers = append(layers, infra)
	}

	// Layer 2: api — datasources, controllers, resolvers, routes, tools
	api := Layer{Name: "api"}
	api.Files = p.findAPIFiles(repoPath)
	if len(api.Files) > 0 {
		layers = append(layers, api)
	}

	// Layer 3: config — env config, application config files
	cfg := Layer{Name: "config"}
	cfg.Files = p.findConfigFiles(repoPath)
	if len(cfg.Files) > 0 {
		layers = append(layers, cfg)
	}

	return layers, nil
}

func (p *NodeJSParser) detectFramework(repoPath string) {
	pkgPath := filepath.Join(repoPath, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}
	all := make(map[string]bool)
	for k := range pkg.Dependencies {
		all[k] = true
	}
	for k := range pkg.DevDependencies {
		all[k] = true
	}

	switch {
	case all["@mastra/core"]:
		p.framework = "mastra"
	case all["@apollo/server"] || all["apollo-server"] || all["@apollo/datasource-rest"]:
		p.framework = "graphql-apollo"
	case all["react"] && !all["express"] && !all["fastify"]:
		p.framework = "react"
	case all["fastify"]:
		p.framework = "fastify"
	case all["express"]:
		p.framework = "express"
	default:
		p.framework = "node"
	}
}

func (p *NodeJSParser) findInfraFiles(repoPath string) []string {
	var files []string

	// package.json (root only)
	if fileExists(repoPath, "package.json") {
		files = append(files, filepath.Join(repoPath, "package.json"))
	}

	// Helm/K8s deployment configs
	for _, helmDir := range []string{"deploy/helm", "deploy/kubernetes", "config_k8s", "k8s"} {
		helmPath := filepath.Join(repoPath, filepath.FromSlash(helmDir))
		fi, err := os.Stat(helmPath)
		if err != nil || !fi.IsDir() {
			continue
		}
		filepath.Walk(helmPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				files = append(files, path)
			}
			return nil
		})
	}

	// Amplify (frontend)
	if fileExists(repoPath, "amplify.yml") {
		files = append(files, filepath.Join(repoPath, "amplify.yml"))
	}

	// Server entry point
	for _, entry := range []string{
		"src/server.ts", "src/index.ts", "src/main.ts", "src/app.ts",
		"src/mastra/index.ts",
		"index.ts", "index.js", "server.js",
	} {
		if fileExists(repoPath, entry) {
			files = append(files, filepath.Join(repoPath, filepath.FromSlash(entry)))
			break
		}
	}

	return files
}

func (p *NodeJSParser) findAPIFiles(repoPath string) []string {
	var files []string
	seen := map[string]bool{}

	addFile := func(path string) {
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}

	// Directories that contain API interaction code
	apiDirs := []string{
		"src/datasource", "src/datasources",                        // Apollo DataSources
		"src/base",                                                  // Apollo base datasource
		"src/controllers", "src/routes",                            // Express controllers/routes
		"src/mastra/api-routes",                                     // Mastra routes
		"src/mastra/tools",                                          // Mastra tools (external API calls)
		"src/modules/requests",                                      // TS Serverless request modules
	}

	for _, dir := range apiDirs {
		dirPath := filepath.Join(repoPath, filepath.FromSlash(dir))
		fi, err := os.Stat(dirPath)
		if err != nil || !fi.IsDir() {
			continue
		}
		// For mastra/tools, limit to 1 level deep index files only (too many files)
		if strings.HasSuffix(dir, "/tools") {
			// Add one file per tool subdirectory (the main index/execute file)
			entries, _ := os.ReadDir(dirPath)
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				toolDir := filepath.Join(dirPath, e.Name())
				for _, candidate := range []string{"index.ts", "execute.ts", e.Name() + ".ts"} {
					candidate := filepath.Join(toolDir, candidate)
					if _, err := os.Stat(candidate); err == nil {
						addFile(candidate)
						break
					}
				}
			}
			continue
		}
		filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if isSourceFile(path) {
				addFile(path)
			}
			return nil
		})
	}

	// Also look for resolver files in domain subdirs (janus pattern)
	srcPath := filepath.Join(repoPath, "src")
	if fi, err := os.Stat(srcPath); err == nil && fi.IsDir() {
		filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			name := strings.ToLower(info.Name())
			if (strings.Contains(name, "datasource") || strings.Contains(name, "resolver")) &&
				isSourceFile(path) {
				addFile(path)
			}
			return nil
		})
	}

	// Limit to avoid huge prompts
	if len(files) > 30 {
		files = files[:30]
	}
	return files
}

func (p *NodeJSParser) findConfigFiles(repoPath string) []string {
	var files []string

	// Env validation files
	for _, rel := range []string{
		"src/infra/config/env.ts",
		"src/config/env.ts",
		"src/config.ts",
		"src/env.ts",
		"config.ts",
		".env.example",
	} {
		if fileExists(repoPath, rel) {
			files = append(files, filepath.Join(repoPath, filepath.FromSlash(rel)))
		}
	}

	// YAML config files (herbie-api pattern)
	for _, rel := range []string{
		"config_k8s/default.yaml",
		"config_k8s/custom-environment-variables.yaml",
	} {
		if fileExists(repoPath, rel) {
			files = append(files, filepath.Join(repoPath, filepath.FromSlash(rel)))
		}
	}

	return files
}

func (p *NodeJSParser) SystemPrompt() string {
	return `You are a senior Node.js / TypeScript architect.
You extract structured metadata from source files to populate a code intelligence database.
Repos may use Apollo GraphQL, Express, Mastra AI, React, or other Node frameworks.
Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *NodeJSParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [{"name":"","handler_file":"","trigger_type":"","trigger_detail":"","timeout":0,"max_retry":0,"concurrency":0,"vpc":false,"description":""}],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[{"assoc_type":"","target_model":"","foreign_key":""}]}],
  "external_apis": [{"name":"","url":"","method":"","auth_type":"","description":""}],
  "db_connections": [{"dialect":"","host_var":"","pool_min":0,"pool_max":0,"pool_idle":0}],
  "config_vars": [{"key":"","source":"","description":""}]
}`

	instructions := map[string]string{
		"infra": fmt.Sprintf(`Extract from package.json, deployment configs, and server entry point:
- handlers: all HTTP routes/endpoints defined (name=route path or handler function, trigger_type="http", trigger_detail="METHOD /path", description=purpose)
  For Mastra: registerApiRoute calls. For Express: app.get/post/put/delete or router definitions.
  For React frontend: leave handlers empty.
- events: all Kafka topics consumed or produced (event_type: kafka-consumer or kafka-producer, bus_name=topic name).
  If no Kafka, leave empty.
- config_vars: all SSM parameters from deployment YAML (/cobli/k8s/.../KEY → source=ssm) and
  environment variable names from package.json scripts or helm values (source=env).
- db_connections: infer from SSM secrets named *POSTGRES* or *DATABASE* or *DB* (dialect=postgres).
- external_apis: any hardcoded base URLs in config files (name=env var name, url=value, auth_type=infer).

Output schema:
%s

Files:
`, schema),
		"api": fmt.Sprintf(`Extract from datasource, controller, resolver, route, and tool files:
- external_apis: every external HTTP or REST call:
  - Apollo DataSource (RESTDataSource): name=class name, url=this.baseURL or resolved URL, method=GET unless POST is explicit.
  - Express controller axios calls: name=function/method name, url=full URL or base URL env var, method from axios.get/post/etc.
  - Mastra tool fetch() calls: name=tool name, url=URL pattern, method=GET unless explicit.
  - Omit internal database calls. Omit calls to localhost.
- handlers: any route registrations visible in route files (trigger_type="http", trigger_detail="METHOD /path").
  For Apollo resolvers: name=resolver name, trigger_type="graphql", trigger_detail="Query/Mutation name".
- events: Kafka producers/consumers only if present.
- models, db_connections, config_vars: leave empty (covered in other layers).

Output schema:
%s

Files:
`, schema),
		"config": fmt.Sprintf(`Extract from environment config, .env.example, and YAML config files:
- config_vars: all env var names used (source=ssm for /cobli/k8s paths, source=env for plain env vars).
  Include descriptions from comments or context if available.
- db_connections: any DB connection details (dialect, host_var for the host env var name, pool sizes).
- external_apis: any base URLs defined as config values (name=config key, url=value or placeholder).
- handlers, events, models: leave empty.

Output schema:
%s

Files:
`, schema),
	}

	instruction, ok := instructions[layerName]
	if !ok {
		instruction = fmt.Sprintf("Extract all relevant entities.\n\nOutput schema:\n%s\n\nFiles:\n", schema)
	}

	return instruction + "\n" + content
}

// isSourceFile returns true for TypeScript, JavaScript, and YAML source files.
func isSourceFile(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".yaml", ".yml"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

