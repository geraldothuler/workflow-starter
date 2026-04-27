package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KotlinFlinkParser handles multi-module Kotlin + Apache Flink streaming jobs.
// Typical structure:
//
//	<module>/src/main/kotlin/<pkg>/
//	  Application.kt or <Name>Job.kt      → job topology entry point
//	  application/                         → Flink operators and functions
//	  domain/                              → state classes and domain models
//	  infrastructure/flink/               → StreamExecutionEnvironment config
//	  infrastructure/kafka/               → Kafka sources and sinks
//	  infrastructure/scylla/              → Cassandra/ScyllaDB access
//	  infrastructure/<other>/             → HTTP clients and external services
type KotlinFlinkParser struct{}

func (p *KotlinFlinkParser) Lang() string      { return "kotlin" }
func (p *KotlinFlinkParser) Framework() string { return "flink" }

// Layers returns 5 bundles for a Kotlin/Flink monorepo:
//  1. infra      — root + module build.gradle.kts + application.conf → deps, Kafka, DB config
//  2. job        — *Job.kt + Application.kt + infrastructure/flink/*.kt → stream topology
//  3. processors — application/*.kt → Flink operators, KeyedProcessFunction, async functions
//  4. providers  — infrastructure/kafka/*.kt, scylla/*.kt, etc. → external data access
//  5. states     — domain/*.kt + infrastructure/serialization/*.kt → state classes, serializers
func (p *KotlinFlinkParser) Layers(repoPath string) ([]Layer, error) {
	infra := Layer{Name: "infra"}
	job := Layer{Name: "job"}
	processors := Layer{Name: "processors"}
	providers := Layer{Name: "providers"}
	states := Layer{Name: "states"}

	// Layer 1: infra — root build files
	for _, rel := range []string{
		"build.gradle.kts", "build.gradle",
		"settings.gradle.kts", "settings.gradle",
		"gradle.properties",
	} {
		if fileExists(repoPath, rel) {
			infra.Files = append(infra.Files, filepath.Join(repoPath, rel))
		}
	}
	// Per-module build.gradle.kts (sub-modules in monorepo)
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if fi.Name() != "build.gradle.kts" {
			return nil
		}
		if path == filepath.Join(repoPath, "build.gradle.kts") {
			return nil // root already added above
		}
		infra.Files = append(infra.Files, path)
		return nil
	})
	// application.conf and reference.conf across all modules
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		name := fi.Name()
		if (name == "application.conf" || name == "reference.conf") &&
			!isTestPath(path) && !strings.Contains(path, "/build/") {
			infra.Files = append(infra.Files, path)
		}
		return nil
	})

	// Layers 2-5: classify Kotlin source files by directory convention
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".kt" {
			return nil
		}
		if isTestPath(path) || strings.Contains(path, "/build/") {
			return nil
		}

		name := fi.Name()
		dir := filepath.ToSlash(filepath.Dir(path))

		switch {
		// job: entry points (*Job.kt, Application.kt) and Flink env config
		case strings.HasSuffix(name, "Job.kt"),
			name == "Application.kt",
			strings.Contains(dir, "/infrastructure/flink"):
			job.Files = append(job.Files, path)

		// processors: Flink operators and functions in application/ packages
		case strings.Contains(dir, "/application"):
			processors.Files = append(processors.Files, path)

		// states: domain models + serialization classes (Kryo, state descriptors)
		case strings.Contains(dir, "/domain"),
			strings.Contains(dir, "/infrastructure/serialization"),
			strings.Contains(dir, "/infrastructure/state"):
			states.Files = append(states.Files, path)

		// providers: remaining infrastructure (kafka/, scylla/, odometer/, auth/, json/)
		case strings.Contains(dir, "/infrastructure"):
			providers.Files = append(providers.Files, path)
		}

		return nil
	})

	layers := []Layer{}
	for _, l := range []Layer{infra, job, processors, providers, states} {
		if len(l.Files) > 0 {
			layers = append(layers, l)
		}
	}
	return layers, nil
}

func (p *KotlinFlinkParser) SystemPrompt() string {
	return `You are a senior Kotlin + Apache Flink streaming architect with deep knowledge of:
- Flink DataStream API (KeyedProcessFunction, RichAsyncFunction, KeyedStream, OutputTag, side outputs)
- Flink Kafka connector (KafkaSource.builder(), KafkaSink.builder(), flink-connector-kafka 3.x)
- Kotlin idioms (sealed classes, data classes, object declarations, extension functions, coroutines)
- Cassandra Driver 4 / ScyllaDB (CqlSession, PreparedStatement, async execution via executeAsync)
- OkHttp 4 for async HTTP delivery with retry and circuit breaker patterns
- Typesafe Config (application.conf / config.getString) for all runtime configuration
- Protobuf 3 / protobuf-kotlin for Kafka message serialization
- Multi-module Gradle (settings.gradle.kts) Flink monorepos

You extract structured metadata from Kotlin/Flink source files to populate a code intelligence database.
Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *KotlinFlinkParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [{"name":"","handler_file":"","trigger_type":"","trigger_detail":"","timeout":0,"max_retry":0,"concurrency":0,"vpc":false,"description":""}],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[{"assoc_type":"","target_model":"","foreign_key":""}]}],
  "external_apis": [{"name":"","url":"","method":"","auth_type":"","description":""}],
  "db_connections": [{"dialect":"","host_var":"","pool_min":0,"pool_max":0,"pool_idle":0}],
  "config_vars": [{"key":"","source":"","description":""}]
}`

	instructions := map[string]string{
		"infra": fmt.Sprintf(`Extract from build.gradle.kts and application.conf:
- handlers: each Flink job module as a handler (name=module name from settings.gradle.kts, trigger_type="flink-job", trigger_detail=Kafka source topic from application.conf, description=brief job purpose).
- events: Kafka topics consumed (event_type="kafka-consumer", bus_name=topic name) and produced (event_type="kafka-producer", bus_name=topic name). Read topic names from application.conf — look for app.source.*, app.sink.*, kafka.topics.*, or any string value matching a Kafka topic naming pattern (kebab-case, 3+ chars).
- db_connections: ScyllaDB/Cassandra contact points from application.conf (dialect="cassandra", host_var=env var or config key for contact-points, pool_min/max if configured).
- config_vars: all ${?ENV_VAR} or ${ENV_VAR} substitutions in application.conf — key=VAR_NAME, source="env", description=what it configures.
- external_apis: none typically in infra layer.
- models: none.

Output schema:
%s

Files:
`, schema),

		"job": fmt.Sprintf(`Extract from Flink job entry point (*Job.kt, Application.kt) and StreamExecutionEnvironment config files:
- handlers: each Flink job class (name=class name, trigger_type="flink-job", trigger_detail=stream topology summary — sources → keyBy → operators → sinks, description=what events are processed and what the output produces).
- events: Kafka source topics (event_type="kafka-consumer", bus_name=topic name, description=Protobuf/data type consumed) and sink topics (event_type="kafka-producer", bus_name=topic name, description=event type produced).
- external_apis: any non-Kafka sinks used directly in the job (Cassandra → url="cassandra://herbie", method="CQL"; HTTP async sink → url from config key).
- config_vars: environment variables loaded in ApplicationConfig or config loader class.
- Omit models, db_connections.

Output schema:
%s

Files:
`, schema),

		"processors": fmt.Sprintf(`Extract from Flink operator and function Kotlin files (application/ package):
- handlers: each Flink operator class (name=class name, trigger_type=Flink base class e.g. "KeyedProcessFunction"/"RichAsyncFunction"/"ProcessFunction"/"RichFlatMapFunction", trigger_detail=key type + ValueState/ListState descriptors managed, description=what this operator does — input event type, transformation, output event type).
- models: Flink state used — for each ValueStateDescriptor/ListStateDescriptor/MapStateDescriptor found, emit a model (name=state descriptor name, table_name="flink-state", dialect="flink-valuestate"/"flink-liststate"/"flink-mapstate", fields=state class fields with Kotlin types).
- config_vars: config keys read from ApplicationConfig (key=config path, source="typesafe-config", description=what it controls).
- Omit events, external_apis, db_connections.

Output schema:
%s

Files:
`, schema),

		"providers": fmt.Sprintf(`Extract from infrastructure provider Kotlin files (kafka/, scylla/, odometer/, auth/, http/ packages):
- external_apis: each external system accessed (name=class or object name, url=contact point / base URL from config or literal, method=CQL/GET/POST, auth_type=none/bearer/hmac/api-key, description=what data is read or written and for what purpose).
- db_connections: ScyllaDB CqlSession configuration (dialect="cassandra", host_var=config key for contact-points, pool_min/max from session profile or DataStax driver config).
- models: CQL table access — for each prepared statement, extract table name and columns (name=table name, table_name=actual table, dialect="cassandra", fields=columns bound or selected).
- config_vars: config keys from ApplicationConfig used in this layer (key=config path, source="typesafe-config").
- Omit handlers, events.

Output schema:
%s

Files:
`, schema),

		"states": fmt.Sprintf(`Extract from domain model and serialization Kotlin files:
- models: each domain data class or sealed class hierarchy (name=class name, table_name="flink-state" if used as Flink Kryo-serialized state or "domain" if pure domain DTO, dialect="flink-kryo" if the class is registered with Flink serialization, fields=all data class constructor fields with Kotlin types and nullable flag).
- handlers: custom Kryo serializer classes (name=class name, trigger_type="flink-serializer", description=which state class it serializes and why custom serialization was needed).
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
