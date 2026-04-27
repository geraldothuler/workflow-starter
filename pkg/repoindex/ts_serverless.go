package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TypeScriptServerlessParser handles repos using TypeScript + Serverless Framework (AWS Lambda).
type TypeScriptServerlessParser struct{}

func (p *TypeScriptServerlessParser) Lang() string      { return "typescript" }
func (p *TypeScriptServerlessParser) Framework() string { return "serverless" }

// Layers returns 4 bundled file groups for hermes-style repos:
//  1. infra  — serverless.yml + config.ts (triggers, schedules, env vars, pool)
//  2. models — src/models/*.ts (Sequelize models, fields, associations)
//  3. apis   — src/modules/requests/*.ts (external HTTP APIs, SFTP, auth flows)
//  4. db     — src/database/*.ts (DB connection config)
func (p *TypeScriptServerlessParser) Layers(repoPath string) ([]Layer, error) {
	layers := []Layer{}

	// Layer 1: infra
	infra := Layer{Name: "infra"}
	for _, rel := range []string{"serverless.yml", "serverless.ts"} {
		if fileExists(repoPath, rel) {
			infra.Files = append(infra.Files, filepath.Join(repoPath, rel))
		}
	}
	if fileExists(repoPath, "src/config.ts") {
		infra.Files = append(infra.Files, filepath.Join(repoPath, "src/config.ts"))
	}
	if len(infra.Files) > 0 {
		layers = append(layers, infra)
	}

	// Layer 2: models
	models := Layer{Name: "models"}
	models.Files = globFiles(repoPath, "src/models", "*.ts")
	if len(models.Files) > 0 {
		layers = append(layers, models)
	}

	// Layer 3: apis (requests modules)
	apis := Layer{Name: "apis"}
	apis.Files = globFiles(repoPath, "src/modules/requests", "*.ts")
	apis.Files = append(apis.Files, globFiles(repoPath, "src/modules/requests/database", "*.ts")...)
	if len(apis.Files) > 0 {
		layers = append(layers, apis)
	}

	// Layer 4: db
	db := Layer{Name: "db"}
	db.Files = globFiles(repoPath, "src/database", "*.ts")
	if len(db.Files) > 0 {
		layers = append(layers, db)
	}

	return layers, nil
}

func (p *TypeScriptServerlessParser) SystemPrompt() string {
	return `You are a senior TypeScript + AWS Serverless Framework architect.
You extract structured metadata from source files to populate a code intelligence database.
Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *TypeScriptServerlessParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [{"name":"","handler_file":"","trigger_type":"","trigger_detail":"","timeout":0,"max_retry":0,"concurrency":0,"vpc":false,"description":""}],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[{"assoc_type":"","target_model":"","foreign_key":""}]}],
  "external_apis": [{"name":"","url":"","method":"","auth_type":"","description":""}],
  "db_connections": [{"dialect":"","host_var":"","pool_min":0,"pool_max":0,"pool_idle":0}],
  "config_vars": [{"key":"","source":"","description":""}]
}`

	instructions := map[string]string{
		"infra": fmt.Sprintf(`Extract from serverless.yml and config.ts:
- handlers: all Lambda functions with their trigger (eventbridge/s3/schedule/http), trigger_detail (cron expression, event pattern JSON, S3 prefix), timeout (seconds), max_retry, concurrency (reservedConcurrency; 0=unlimited), vpc (true if vpc block present).
- events: all EventBridge event types produced (detail-type values) and the EventBus name.
- config_vars: all SSM parameters (/cobli/...) and environment variables with their source (ssm/env/hardcoded).
- external_apis: any hardcoded URLs in config.ts (name=constant name, url=value, method=GET unless obvious, auth_type=infer from context).
- db_connections: infer from config vars (HOST, USERNAME, PASSWORD, MAX_CONNECTIONS_POOL, LAMBDA_FUNCTION_TIMEOUT).

Output schema:
%s

Files:
`, schema),
		"models": fmt.Sprintf(`Extract from Sequelize model files:
- models: each model with its tableName, dialect=postgres, all fields (name, DataTypes type as string, nullable, primaryKey, unique), and associations (belongsTo/hasMany/belongsToMany with target model name and foreignKey).
- Omit handlers, events, external_apis, db_connections, config_vars.

Output schema:
%s

Files:
`, schema),
		"apis": fmt.Sprintf(`Extract from request module files:
- external_apis: every external HTTP/SFTP/API call (name=function name or constant, url, method GET/POST/PUT/DELETE, auth_type: oauth2/basic/bearer/api-key/sftp/none, description=what data it fetches).
- Do NOT extract internal database calls — only external services.
- Omit handlers, events, models, db_connections, config_vars.

Output schema:
%s

Files:
`, schema),
		"db": fmt.Sprintf(`Extract from database configuration files:
- db_connections: dialect (postgres/mysql/sqlite), host_var (env var name for host), pool_min, pool_max, pool_idle (ms).
- Omit everything else.

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

// globFiles returns absolute paths matching pattern under baseDir/subDir.
func globFiles(repoPath, subDir, pattern string) []string {
	dir := filepath.Join(repoPath, filepath.FromSlash(subDir))
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil
	}
	return matches
}

// fileExists checks if a file exists relative to repoPath.
func fileExists(repoPath, rel string) bool {
	_, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(rel)))
	return err == nil
}

// bundleFiles concatenates file contents with path headers for LLM context.
func bundleFiles(files []string) string {
	var sb strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n\n// === FILE: %s ===\n", filepath.Base(f)))
		sb.Write(data)
	}
	return sb.String()
}
