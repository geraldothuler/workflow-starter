package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PythonFlaskParser handles Python web service repos:
//   - Flask REST APIs (cerberus-api style, multi-module with wsgi.py)
//   - FastAPI services
//   - Django apps
//
// Detection heuristics (in order of preference):
//  1. Root-level requirements.txt or pyproject.toml with a web framework
//  2. Any top-level directory containing wsgi.py (multi-module Flask pattern)
type PythonFlaskParser struct {
	repoPath  string // stored at construction time for eager framework detection
	framework string // flask | fastapi | django | python
}

func (p *PythonFlaskParser) Lang() string { return "python" }

func (p *PythonFlaskParser) Framework() string {
	if p.framework == "" && p.repoPath != "" {
		p.detectFramework(p.repoPath)
	}
	if p.framework != "" {
		return p.framework
	}
	return "python"
}

// Layers returns file groups for LLM indexing.
// Handles both flat (root requirements.txt) and multi-module (subdir/wsgi.py) layouts.
func (p *PythonFlaskParser) Layers(repoPath string) ([]Layer, error) {
	p.detectFramework(repoPath)

	var layers []Layer

	// Layer 1: infra — deploy configs, requirements, wsgi entry points, catalog
	infra := Layer{Name: "infra"}
	infra.Files = p.findInfraFiles(repoPath)
	if len(infra.Files) > 0 {
		layers = append(layers, infra)
	}

	// Layer 2: api — route and view files (Flask blueprints, FastAPI routers)
	api := Layer{Name: "api"}
	api.Files = p.findAPIFiles(repoPath)
	if len(api.Files) > 0 {
		layers = append(layers, api)
	}

	// Layer 3: models — SQLAlchemy models, Alembic migrations
	models := Layer{Name: "models"}
	models.Files = p.findModelFiles(repoPath)
	if len(models.Files) > 0 {
		layers = append(layers, models)
	}

	// Layer 4: config — config.py, .env.example, helm values
	cfg := Layer{Name: "config"}
	cfg.Files = p.findConfigFiles(repoPath)
	if len(cfg.Files) > 0 {
		layers = append(layers, cfg)
	}

	return layers, nil
}

func (p *PythonFlaskParser) detectFramework(repoPath string) {
	var content string
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if fi.Name() == "target" || fi.Name() == ".git" || fi.Name() == "node_modules" {
			return filepath.SkipDir
		}
		name := fi.Name()
		if name == "requirements.txt" || name == "requirements-prod.txt" || name == "requirements.in" {
			data, err := os.ReadFile(path)
			if err == nil {
				content += strings.ToLower(string(data))
			}
		}
		return nil
	})

	switch {
	case strings.Contains(content, "fastapi"):
		p.framework = "fastapi"
	case strings.Contains(content, "django"):
		p.framework = "django"
	case strings.Contains(content, "flask"):
		p.framework = "flask"
	default:
		p.framework = "python"
	}
}

func (p *PythonFlaskParser) findInfraFiles(repoPath string) []string {
	var files []string

	// catalog-info.yaml (service metadata)
	if fileExists(repoPath, "catalog-info.yaml") {
		files = append(files, filepath.Join(repoPath, "catalog-info.yaml"))
	}

	// Root-level requirements files
	for _, rel := range []string{"requirements.txt", "requirements-prod.txt", "pyproject.toml", "setup.py"} {
		if fileExists(repoPath, rel) {
			files = append(files, filepath.Join(repoPath, rel))
		}
	}

	// Multi-module: find wsgi.py and requirements.txt in top-level subdirs
	entries, _ := os.ReadDir(repoPath)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "deploy" || name == "docs" || name == "tests" {
			continue
		}
		modDir := filepath.Join(repoPath, name)
		// wsgi.py at module root
		if _, err := os.Stat(filepath.Join(modDir, "wsgi.py")); err == nil {
			files = append(files, filepath.Join(modDir, "wsgi.py"))
		}
		// requirements.txt in src/ or module root
		for _, rel := range []string{"src/requirements.txt", "src/requirements.in", "requirements.txt"} {
			p := filepath.Join(modDir, filepath.FromSlash(rel))
			if _, err := os.Stat(p); err == nil {
				files = append(files, p)
				break
			}
		}
	}

	// Helm/K8s deploy configs
	for _, helmDir := range []string{"deploy/helm", "deploy/kubernetes", "k8s"} {
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

	return files
}

func (p *PythonFlaskParser) findAPIFiles(repoPath string) []string {
	var files []string
	seen := map[string]bool{}

	addFile := func(path string) {
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}

	// Walk looking for routes/ and views/ directories, and files named app.py, main.py
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			name := fi.Name()
			if name == ".git" || name == "target" || name == "node_modules" ||
				name == "__pycache__" || name == ".venv" || name == "venv" ||
				name == "migrations" || name == "tests" || name == "docs" ||
				name == "static" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(fi.Name(), ".py") {
			return nil
		}

		dir := filepath.Base(filepath.Dir(path))
		name := strings.TrimSuffix(fi.Name(), ".py")

		// Route/view/handler files
		isAPI := dir == "routes" || dir == "views" || dir == "handlers" ||
			dir == "controllers" || dir == "api" || dir == "endpoints" ||
			name == "app" || name == "main" || name == "server"

		// Also include files with route decorators (lightweight check via name)
		if isAPI {
			addFile(path)
		}
		return nil
	})

	if len(files) > 30 {
		files = files[:30]
	}
	return files
}

func (p *PythonFlaskParser) findModelFiles(repoPath string) []string {
	var files []string
	seen := map[string]bool{}

	addFile := func(path string) {
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}

	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			name := fi.Name()
			if name == ".git" || name == "target" || name == "__pycache__" ||
				name == ".venv" || name == "venv" || name == "tests" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(fi.Name(), ".py") {
			return nil
		}

		dir := filepath.Base(filepath.Dir(path))
		if dir == "models" || dir == "schemas" || dir == "entities" {
			addFile(path)
		}
		return nil
	})

	// Also include Alembic migration files (first 3 only for schema overview)
	migrationsDir := filepath.Join(repoPath, "core", "migrations")
	if fi, err := os.Stat(migrationsDir); err == nil && fi.IsDir() {
		count := 0
		filepath.Walk(migrationsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".py") || count >= 3 {
				return nil
			}
			if strings.Contains(info.Name(), "initial") || strings.Contains(info.Name(), "create") {
				addFile(path)
				count++
			}
			return nil
		})
	}

	return files
}

func (p *PythonFlaskParser) findConfigFiles(repoPath string) []string {
	var files []string

	// Top-level config file
	if fileExists(repoPath, "config.py") {
		files = append(files, filepath.Join(repoPath, "config.py"))
	}

	// Module-level config files
	entries, _ := os.ReadDir(repoPath)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		modDir := filepath.Join(repoPath, name)
		for _, rel := range []string{"config.py", "src/config.py", "settings.py"} {
			p := filepath.Join(modDir, filepath.FromSlash(rel))
			if _, err := os.Stat(p); err == nil {
				files = append(files, p)
				break
			}
		}
		// .env.example
		if envEx := filepath.Join(modDir, ".env.example"); fileExists(modDir, ".env.example") {
			_ = envEx
			files = append(files, filepath.Join(modDir, ".env.example"))
		}
	}

	// Root .env.example
	if fileExists(repoPath, ".env.example") {
		files = append(files, filepath.Join(repoPath, ".env.example"))
	}

	return files
}

func (p *PythonFlaskParser) SystemPrompt() string {
	return `You are a senior Python web services architect.
You extract structured metadata from Python source files (Flask, FastAPI, Django) to populate a code intelligence database.
Repos may use Flask Blueprints, FastAPI routers, SQLAlchemy models, Alembic migrations, or gunicorn/wsgi deployments.
Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *PythonFlaskParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [{"name":"","handler_file":"","trigger_type":"","trigger_detail":"","timeout":0,"max_retry":0,"concurrency":0,"vpc":false,"description":""}],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[{"assoc_type":"","target_model":"","foreign_key":""}]}],
  "external_apis": [{"name":"","url":"","method":"","auth_type":"","description":""}],
  "db_connections": [{"dialect":"","host_var":"","pool_min":0,"pool_max":0,"pool_idle":0}],
  "config_vars": [{"key":"","source":"","description":""}]
}`

	instructions := map[string]string{
		"infra": fmt.Sprintf(`Extract from requirements.txt, wsgi.py, catalog-info.yaml, and Helm/k8s configs:
- handlers: Flask/FastAPI routes registered in wsgi.py or app factory (trigger_type="http", trigger_detail="METHOD /path").
- config_vars: environment variable names from Helm values YAML (source=env) or SSM parameters (source=ssm).
- db_connections: infer from requirements (psycopg2 → dialect=postgres, pymysql → mysql) and env var names like DATABASE_URL, DB_HOST.
- external_apis: any hardcoded base URLs in helm values or wsgi.py (name=env var or service name, url=value, auth_type=infer).
- events: Kafka consumers/producers only if present (rare in Flask repos).
- models: leave empty (covered in models layer).

Output schema:
%s

Files:
`, schema),
		"api": fmt.Sprintf(`Extract from Flask Blueprint route files and FastAPI router files:
- handlers: every HTTP endpoint decorated with @blueprint.route, @router.get/post/put/delete, @app.route:
  name=function name, handler_file=file path, trigger_type="http", trigger_detail="METHOD /path", description=docstring if present.
- external_apis: any requests.get/post/Session calls or httpx calls to external URLs:
  name=function or variable name, url=URL or base URL env var, method=HTTP method, auth_type=Bearer/API-Key/None.
- events: leave empty unless Kafka/Celery tasks are present.
- models, db_connections, config_vars: leave empty.

Output schema:
%s

Files:
`, schema),
		"models": fmt.Sprintf(`Extract from SQLAlchemy model files and Alembic migrations:
- models: every SQLAlchemy Model class (db.Model):
  name=class name, table_name=__tablename__, dialect=postgres,
  fields=[column name + SQLAlchemy type → mapped to simple type string (String→string, Integer→int, Boolean→bool, DateTime→datetime)],
  associations=[ForeignKey/relationship → assoc_type=ForeignKey or BelongsTo/HasMany, target_model=referenced class].
- db_connections: if SQLAlchemy engine or DB URL is configured here (dialect, host_var).
- handlers, events, external_apis, config_vars: leave empty.

Output schema:
%s

Files:
`, schema),
		"config": fmt.Sprintf(`Extract from config.py, settings.py, and .env.example:
- config_vars: all environment variable names (os.environ.get, os.getenv):
  key=var name, source=env, description=comment or inline context if available.
- db_connections: SQLAlchemy SQLALCHEMY_DATABASE_URI pattern → dialect=postgres, host_var=DATABASE_URL or similar.
- external_apis: any base URL config values (name=config key, url=value or placeholder).
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
