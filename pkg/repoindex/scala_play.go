package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScalaPlayParser handles Scala + Play Framework web services (e.g. herbie-api-maintenance,
// herbie-api-performance). Detected when build.sbt contains "ProdServerStart" or
// "alexstrasza-core-play".
type ScalaPlayParser struct{}

func (p *ScalaPlayParser) Lang() string      { return "scala" }
func (p *ScalaPlayParser) Framework() string { return "play" }

// Layers returns 4 bundles for a Scala/Play service:
//  1. infra       — conf/routes, conf/application.conf, build.sbt → deps, DB config, Kafka
//  2. controllers — app/**/controllers/*.scala → HTTP handlers, routes, request/response types
//  3. models      — app/**/models/*.scala → DB models, case classes, DAOs
//  4. services    — app/**/services/*.scala → business logic, external API calls
func (p *ScalaPlayParser) Layers(repoPath string) ([]Layer, error) {
	layers := []Layer{}

	// Find the Play module root: may be at repoPath directly or under a submodule (e.g. api/).
	// Heuristic: look for conf/routes — its grandparent is the module root.
	moduleRoot := playModuleRoot(repoPath)

	// Layer 1: infra
	infra := Layer{Name: "infra"}
	// Always include root build.sbt
	if fileExists(repoPath, "build.sbt") {
		infra.Files = append(infra.Files, filepath.Join(repoPath, "build.sbt"))
	}
	for _, rel := range []string{
		"conf/routes",
		"conf/application.conf",
		"conf/reference.conf",
	} {
		if fileExists(moduleRoot, rel) {
			p := filepath.Join(moduleRoot, rel)
			// Avoid duplicate if moduleRoot == repoPath and build.sbt already added
			infra.Files = append(infra.Files, p)
		}
	}
	if len(infra.Files) > 0 {
		layers = append(layers, infra)
	}

	// Layer 2: controllers (walk app/ recursively — Play packages are deeply nested)
	controllers := Layer{Name: "controllers"}
	controllers.Files = walkScalaByDir(moduleRoot, "app", "controller")
	if len(controllers.Files) > 0 {
		layers = append(layers, controllers)
	}

	// Layer 3: models
	models := Layer{Name: "models"}
	models.Files = walkScalaByDir(moduleRoot, "app", "model", "domain")
	if len(models.Files) > 0 {
		layers = append(layers, models)
	}

	// Layer 4: services, providers, repositories, clients
	services := Layer{Name: "services"}
	services.Files = walkScalaByDir(moduleRoot, "app", "service", "provider", "repositor", "client")
	if len(services.Files) > 0 {
		layers = append(layers, services)
	}

	return layers, nil
}

func (p *ScalaPlayParser) SystemPrompt() string {
	return `You are a senior Scala + Play Framework architect with deep knowledge of:
- Play Framework 2.x (Routes DSL, Action, Controller, Result)
- Slick 3.x for PostgreSQL / MySQL access
- Typesafe Config (application.conf, reference.conf)
- Akka (actors, streams) as used within Play
- Kafka clients (alpakka-kafka or direct KafkaConsumer/KafkaProducer)
- Play JSON (Json.reads/writes macros)

You extract structured metadata from Scala/Play source files to populate a code intelligence database.
Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *ScalaPlayParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [{"name":"","handler_file":"","trigger_type":"","trigger_detail":"","timeout":0,"max_retry":0,"concurrency":0,"vpc":false,"description":""}],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[{"assoc_type":"","target_model":"","foreign_key":""}]}],
  "external_apis": [{"name":"","url":"","method":"","auth_type":"","description":""}],
  "db_connections": [{"dialect":"","host_var":"","pool_min":0,"pool_max":0,"pool_idle":0}],
  "config_vars": [{"key":"","source":"","description":""}]
}`

	instructions := map[string]string{
		"infra": fmt.Sprintf(`Extract from build.sbt, conf/routes, and conf/application.conf:
- handlers: list each HTTP route from conf/routes as a handler (name="METHOD /path", trigger_type="http", trigger_detail="Controller#action", description=brief purpose).
- events: Kafka topics consumed (event_type="kafka-consumer", bus_name=topic name) and produced (event_type="kafka-producer", bus_name=topic name). Read topic names from application.conf.
- db_connections: from application.conf slick configuration — dialect=postgres or mysql, host_var=env var name for the DB host, pool_min/max from connectionPool settings.
- config_vars: all env vars referenced with ${?VAR_NAME} or $${VAR_NAME} in application.conf — key=VAR_NAME, source="env".
- external_apis: none typically in infra layer.
- models: none.

Output schema:
%s

Files:
`, schema),

		"controllers": fmt.Sprintf(`Extract from Play controller Scala files:
- handlers: each controller Action method (name="ControllerClass#methodName", handler_file=file path, trigger_type="http", trigger_detail=HTTP method + path from route if visible, description=what this action does).
- external_apis: any HTTP clients called from within controller actions (name=client class/object name, url=base URL or env var pattern, method=HTTP method, description=what data is fetched).
- Omit models, events, db_connections, config_vars.

Output schema:
%s

Files:
`, schema),

		"models": fmt.Sprintf(`Extract from Play model and domain Scala files:
- models: each case class or Slick Table class mapped to DB (name=class name, table_name=actual DB table name, dialect=postgres or mysql, fields=column fields with types, associations=foreign key relationships).
- db_connections: any Slick Database.forConfig found here (dialect=postgres, host_var=config path).
- Omit handlers, events, external_apis, config_vars.

Output schema:
%s

Files:
`, schema),

		"services": fmt.Sprintf(`Extract from service, repository, and client Scala files:
- external_apis: each external HTTP call (name=client/service class, url=base URL or env var reference, method=HTTP method used, auth_type=infer from auth headers/oauth, description=what this call does).
- db_connections: Slick Database.forConfig connections (dialect=postgres or mysql, host_var=config path).
- models: any query result case classes (name=class name, table_name=DB table if known, dialect=postgres).
- config_vars: config keys read via config.getString (key=config path, source="typesafe-config").
- Omit handlers, events.

Output schema:
%s

Files:
`, schema),
	}

	instruction, ok := instructions[layerName]
	if !ok {
		instruction = fmt.Sprintf("Extract all relevant entities. Output schema:\n%s\n\nFiles:\n", schema)
	}
	return instruction + "\n" + content
}

// playModuleRoot finds the Play module root directory. In single-module repos it
// equals repoPath; in multi-module repos (e.g. repo/api/) it returns the subdir
// containing conf/routes.
func playModuleRoot(repoPath string) string {
	// Check root first
	if fileExists(repoPath, "conf/routes") {
		return repoPath
	}
	// Search one level deep for a submodule
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return repoPath
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(repoPath, e.Name())
		if fileExists(candidate, "conf/routes") || fileExists(candidate, "conf/application.conf") {
			return candidate
		}
	}
	return repoPath
}

// walkScalaByDir recursively collects *.scala files under repoPath/baseDir
// whose path (relative to baseDir) contains any of the given keywords in any
// directory segment. This handles deeply nested Play package structures
// (e.g. app/co/cobli/maintenances/controllers/pastMaintenance/*.scala).
func walkScalaByDir(repoPath, baseDir string, keywords ...string) []string {
	root := filepath.Join(repoPath, baseDir)
	var files []string
	filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || filepath.Ext(path) != ".scala" {
			return nil
		}
		if isTestPath(path) {
			return nil
		}
		// Check each directory segment of the path for a keyword match.
		rel, _ := filepath.Rel(root, path)
		parts := strings.Split(strings.ToLower(filepath.ToSlash(rel)), "/")
		// parts[-1] is the filename — check only the directory segments.
		matched := false
		for _, seg := range parts[:len(parts)-1] {
			for _, kw := range keywords {
				if strings.Contains(seg, kw) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			files = append(files, path)
			return nil
		}
		return nil
	})
	return files
}
