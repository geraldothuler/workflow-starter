package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScalaFlinkParser handles Scala + Apache Flink streaming jobs (alexstrasza-* family).
type ScalaFlinkParser struct{}

func (p *ScalaFlinkParser) Lang() string      { return "scala" }
func (p *ScalaFlinkParser) Framework() string { return "flink" }

// Layers returns 5 bundles for a Scala/Flink job:
//  1. infra   — build.sbt + application.conf → deps, Kafka topics, DB config, deploy
//  2. job     — top-level *.scala (App + main trait) → stream topology, sources, sinks
//  3. processors — processors/*.scala → Flink operators, FSM, state descriptors
//  4. providers  — providers/*.scala + requests/*.scala → external data access
//  5. states     — states/**/*.scala → Flink state classes and serializers
func (p *ScalaFlinkParser) Layers(repoPath string) ([]Layer, error) {
	layers := []Layer{}

	// Layer 1: infra
	infra := Layer{Name: "infra"}
	for _, rel := range []string{"build.sbt", "project/plugins.sbt", "project/build.properties"} {
		if fileExists(repoPath, rel) {
			infra.Files = append(infra.Files, filepath.Join(repoPath, rel))
		}
	}
	// application.conf — walk all modules (multi-module repos like sherlock have
	// conf under stream/src/main/resources/, not root src/main/resources/).
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		name := fi.Name()
		if (name == "application.conf" || name == "reference.conf") &&
			!isTestPath(path) {
			infra.Files = append(infra.Files, path)
		}
		return nil
	})
	if len(infra.Files) > 0 {
		layers = append(layers, infra)
	}

	// Layer 2: job (top-level Scala files — App entrypoint + main trait)
	job := Layer{Name: "job"}
	job.Files = globFiles(repoPath, scalaMainRoot(repoPath), "*.scala")
	if len(job.Files) > 0 {
		layers = append(layers, job)
	}

	// Layer 3: processors
	processors := Layer{Name: "processors"}
	processors.Files = append(processors.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/processors", "*.scala")...)
	processors.Files = append(processors.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/handlers", "*.scala")...)
	processors.Files = append(processors.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/functions", "*.scala")...)
	if len(processors.Files) > 0 {
		layers = append(layers, processors)
	}

	// Layer 4: providers + requests
	providers := Layer{Name: "providers"}
	providers.Files = append(providers.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/providers", "*.scala")...)
	providers.Files = append(providers.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/requests", "*.scala")...)
	providers.Files = append(providers.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/clients", "*.scala")...)
	providers.Files = append(providers.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/repositories", "*.scala")...)
	if len(providers.Files) > 0 {
		layers = append(layers, providers)
	}

	// Layer 5: states + serialization
	states := Layer{Name: "states"}
	states.Files = append(states.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/states", "*.scala")...)
	states.Files = append(states.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/states/serialization", "*.scala")...)
	states.Files = append(states.Files, globFiles(repoPath, scalaMainRoot(repoPath)+"/serialization", "*.scala")...)
	if len(states.Files) > 0 {
		layers = append(layers, states)
	}

	return layers, nil
}

func (p *ScalaFlinkParser) SystemPrompt() string {
	return `You are a senior Scala + Apache Flink streaming architect with deep knowledge of:
- Flink DataStream API (KeyedStream, ProcessFunction, RichFlatMapFunction, ValueState, ListState)
- Kafka consumers/producers in Flink (FlinkKafkaConsumer/Producer)
- Slick 3.x for PostgreSQL access
- Typesafe Config (application.conf)
- ScalaPB / Protobuf for schema
- Kamon metrics

You extract structured metadata from Scala/Flink source files to populate a code intelligence database.
Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *ScalaFlinkParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [{"name":"","handler_file":"","trigger_type":"","trigger_detail":"","timeout":0,"max_retry":0,"concurrency":0,"vpc":false,"description":""}],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[{"assoc_type":"","target_model":"","foreign_key":""}]}],
  "external_apis": [{"name":"","url":"","method":"","auth_type":"","description":""}],
  "db_connections": [{"dialect":"","host_var":"","pool_min":0,"pool_max":0,"pool_idle":0}],
  "config_vars": [{"key":"","source":"","description":""}]
}`

	instructions := map[string]string{
		"infra": fmt.Sprintf(`Extract from build.sbt and application.conf:
- handlers: the Flink job itself as a single handler (name=job name from build.sbt, trigger_type="kafka-consumer", trigger_detail=Kafka consumer topic from application.conf, description=brief job purpose).
- events: Kafka topics consumed (event_type="kafka-consumer", bus_name=topic name) and produced (event_type="kafka-producer", bus_name=topic name). Read topic names from application.conf (app.source.queue.name, app.sink.queue.*).
- db_connections: from application.conf — dialect=postgres, host_var=env var name for server (e.g. OSM_CUSTOM_DB_SERVER_NAME), pool_min/max from numThreads.
- config_vars: all env vars referenced with ${?VAR_NAME} pattern in application.conf — key=VAR_NAME, source="env", description=what it configures.
- external_apis: none typically in infra layer.
- models: none.

Output schema:
%s

Files:
`, schema),

		"job": fmt.Sprintf(`Extract from the main Scala trait/object that extends AlexstraszaStreamingApp or similar:
- handlers: extract the full stream topology as a description (source → keyBy → flatMap/process → sink). name=class name, trigger_type="flink-job", trigger_detail=stream operators chain.
- events: Kafka topics as sink events (event_type="kafka-producer", name=producer variable name, bus_name=topic from config key, description=output schema type).
- external_apis: any sink besides Kafka (Cassandra sink class name as name, url="cassandra", auth_type="none").
- Omit models, db_connections, config_vars.

Output schema:
%s

Files:
`, schema),

		"processors": fmt.Sprintf(`Extract from Flink processor/function Scala files:
- handlers: each Flink operator class (name=class name, trigger_type=Flink operator type e.g. "KeyedProcessFunction"/"RichFlatMapFunction"/"ProcessWindowFunction", trigger_detail=key type and input/output types, description=what this operator does in the pipeline).
- models: FSM state classes used (name=state class name, table_name="flink-state", dialect="flink-valuestate", fields=state fields with types).
- config_vars: config keys read via config.getString/getLong/getBoolean (key=config path, source="typesafe-config").
- Omit events, external_apis, db_connections.

Output schema:
%s

Files:
`, schema),

		"providers": fmt.Sprintf(`Extract from provider and repository Scala files:
- external_apis: each external data source (name=class/object name, url=database host or HTTP URL, method=SQL/GET/POST, auth_type=infer from credentials pattern, description=what data is retrieved and for what purpose).
- db_connections: Slick Database.forConfig connections (dialect=postgres, host_var=config path used).
- models: SQL queries as implicit models — extract table/function names accessed (name=function/table name, table_name=actual table or function, dialect=postgres).
- Omit handlers, events, config_vars.

Output schema:
%s

Files:
`, schema),

		"states": fmt.Sprintf(`Extract from Flink state and serializer Scala files:
- models: each state class (name=class name, table_name="flink-state", dialect="flink-kryo"/"flink-valuestate", fields=all case class fields with Scala types).
- handlers: serializer classes (name=class name, trigger_type="flink-serializer", description=what state it serializes and serialization strategy).
- Omit events, external_apis, db_connections, config_vars.

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

// scalaMainRoot finds the deepest directory under src/main/scala that contains
// .scala files directly (i.e. the actual package root, not intermediate package dirs).
// Descends through single-directory chains (pure package namespace dirs).
func scalaMainRoot(repoPath string) string {
	importPath := filepath.Join("src", "main", "scala")
	base := filepath.Join(repoPath, importPath)

	current := base
	for i := 0; i < 10; i++ {
		// List direct children.
		children, err := filepath.Glob(filepath.Join(current, "*"))
		if err != nil || len(children) == 0 {
			break
		}

		// Count directories vs .scala files at this level.
		var dirs []string
		var scalaCount int
		for _, c := range children {
			if isDir(c) {
				dirs = append(dirs, c)
			} else if filepath.Ext(c) == ".scala" {
				scalaCount++
			}
		}

		// If there are .scala files here, this IS the root.
		if scalaCount > 0 {
			break
		}

		// If there's exactly one sub-directory and no scala files, descend.
		if len(dirs) == 1 {
			current = dirs[0]
			continue
		}

		// Multiple dirs (e.g. co/ has cobli/ but also other things) — stop.
		break
	}

	rel, err := filepath.Rel(repoPath, current)
	if err != nil {
		return importPath
	}
	return rel
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func isTestPath(path string) bool {
	return strings.Contains(path, "/test/") ||
		strings.Contains(path, "/intTest/") ||
		strings.Contains(path, "/testFixtures/") ||
		strings.Contains(path, "/it/")
}
