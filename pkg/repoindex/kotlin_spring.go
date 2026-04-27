package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KotlinSpringParser handles multi-module Kotlin + Spring Boot projects (fusca family).
type KotlinSpringParser struct{}

func (p *KotlinSpringParser) Lang() string      { return "kotlin" }
func (p *KotlinSpringParser) Framework() string { return "spring-boot" }

// Layers returns bundles per module + cross-cutting layers:
//  1. infra      — root build.gradle.kts + application.properties/yml → deps, Kafka, DB config
//  2. api        — *Controller.kt files → REST endpoints, request/response types
//  3. models     — *Entity.kt, *Repository.kt → JPA entities, tables, queries
//  4. services   — *Service.kt, *Consumer.kt, *Producer.kt → business logic, Kafka
//  5. config     — *Config.kt, *Configuration.kt, *Properties.kt → Spring config beans
func (p *KotlinSpringParser) Layers(repoPath string) ([]Layer, error) {
	layers := []Layer{}

	// Layer 1: infra — root build files + all application.properties/yml
	infra := Layer{Name: "infra"}
	for _, rel := range []string{"build.gradle.kts", "build.gradle", "settings.gradle.kts", "settings.gradle"} {
		if fileExists(repoPath, rel) {
			infra.Files = append(infra.Files, filepath.Join(repoPath, rel))
		}
	}
	// Collect application.properties and application.yml from all modules
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		name := fi.Name()
		if strings.HasPrefix(name, "application") && (strings.HasSuffix(name, ".properties") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
			if !strings.Contains(path, "/test/") && !strings.Contains(path, "/intTest/") {
				infra.Files = append(infra.Files, path)
			}
		}
		return nil
	})
	// Per-module build.gradle.kts
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || fi.Name() != "build.gradle.kts" {
			return nil
		}
		if path != filepath.Join(repoPath, "build.gradle.kts") {
			infra.Files = append(infra.Files, path)
		}
		return nil
	})
	if len(infra.Files) > 0 {
		layers = append(layers, infra)
	}

	// Layers 2-6: scan all Kotlin source files by suffix pattern
	var controllers, models, services, configs, topics []string
	filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".kt") {
			return nil
		}
		// Skip test dirs
		if strings.Contains(path, "/test/") || strings.Contains(path, "/intTest/") || strings.Contains(path, "/testFixtures/") {
			return nil
		}
		name := fi.Name()
		switch {
		case strings.HasSuffix(name, "Controller.kt") || strings.Contains(path, "/controller/"):
			controllers = append(controllers, path)
		case strings.HasSuffix(name, "Entity.kt") || strings.HasSuffix(name, "Repository.kt") ||
			strings.Contains(path, "/entity/") || strings.Contains(path, "/repository/") ||
			strings.Contains(path, "/model/") || strings.HasSuffix(name, "Table.kt"):
			models = append(models, path)
		case strings.HasSuffix(name, "Service.kt") || strings.HasSuffix(name, "Consumer.kt") ||
			strings.HasSuffix(name, "Producer.kt") || strings.HasSuffix(name, "Processor.kt") ||
			strings.HasSuffix(name, "Handler.kt") || strings.Contains(path, "/service/") ||
			strings.Contains(path, "/consumer/") || strings.Contains(path, "/producer/"):
			services = append(services, path)
		case strings.HasSuffix(name, "Config.kt") || strings.HasSuffix(name, "Configuration.kt") ||
			strings.HasSuffix(name, "Properties.kt") || strings.Contains(path, "/config/"):
			configs = append(configs, path)
		// Layer topics: enum classes defining Kafka topic names + KafkaEvent implementations
		case strings.HasSuffix(name, "Topic.kt") || strings.HasSuffix(name, "Topics.kt") ||
			strings.HasSuffix(name, "KafkaEvent.kt") || strings.HasSuffix(name, "KafkaEvents.kt") ||
			strings.Contains(path, "/event/") || strings.Contains(path, "/kafka/") ||
			strings.Contains(path, "/topic/"):
			topics = append(topics, path)
		}
		return nil
	})

	if len(controllers) > 0 {
		layers = append(layers, Layer{Name: "api", Files: controllers})
	}
	if len(models) > 0 {
		layers = append(layers, Layer{Name: "models", Files: models})
	}
	if len(services) > 0 {
		layers = append(layers, Layer{Name: "services", Files: services})
	}
	if len(configs) > 0 {
		layers = append(layers, Layer{Name: "config", Files: configs})
	}
	if len(topics) > 0 {
		layers = append(layers, Layer{Name: "topics", Files: topics})
	}

	return layers, nil
}

func (p *KotlinSpringParser) SystemPrompt() string {
	return `You are a senior Kotlin + Spring Boot architect with deep knowledge of:
- Spring Boot 3.x (Spring Data JPA, Spring Web MVC, Spring Kafka, Spring Security)
- Kotlin idioms (data classes, sealed classes, extension functions, coroutines)
- Flyway for DB migrations, HikariCP connection pool
- Kafka consumers/producers (@KafkaListener, KafkaTemplate)
- LaunchDarkly feature flags
- Multi-module Gradle projects

You extract structured metadata from Kotlin/Spring Boot source files to populate a code intelligence database.
Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *KotlinSpringParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [{"name":"","handler_file":"","trigger_type":"","trigger_detail":"","timeout":0,"max_retry":0,"concurrency":0,"vpc":false,"description":""}],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[{"assoc_type":"","target_model":"","foreign_key":""}]}],
  "external_apis": [{"name":"","url":"","method":"","auth_type":"","description":""}],
  "db_connections": [{"dialect":"","host_var":"","pool_min":0,"pool_max":0,"pool_idle":0}],
  "config_vars": [{"key":"","source":"","description":""}]
}`

	instructions := map[string]string{
		"infra": fmt.Sprintf(`Extract from build.gradle.kts and application.properties/yml:
- handlers: each Spring Boot module as a handler (name=module name from build.gradle.kts, trigger_type="spring-boot-api" or "kafka-consumer" or "scheduler", description=module purpose).
- events: Kafka topics (event_type="kafka-consumer"/"kafka-producer", bus_name=topic name from properties, description=what events flow).
- db_connections: from application.properties — dialect=postgres/cassandra/etc, host_var=env var for datasource url, pool_min/max from hikari config.
- config_vars: all environment-specific properties that would be overridden in prod (key=property key, source="env"/"spring-property", description=what it controls).
- external_apis: none typically here.
- models: none.

Output schema:
%s

Files:
`, schema),

		"api": fmt.Sprintf(`Extract from Spring MVC Controller Kotlin files:
- handlers: each REST endpoint method (name=method name, handler_file=controller class, trigger_type="http", trigger_detail="METHOD /path — request type → response type", description=endpoint purpose).
- external_apis: none (these ARE the API, not callers).
- Omit models, events, db_connections, config_vars.

Output schema:
%s

Files:
`, schema),

		"models": fmt.Sprintf(`Extract from JPA Entity and Repository Kotlin files:
- models: each @Entity class (name=class name, table_name=@Table name or snake_case of class, dialect=postgres, fields=all @Column fields with Kotlin types, associations=@OneToMany/@ManyToOne/@ManyToMany with target entity and join column).
- external_apis: none.
- Omit handlers, events, db_connections, config_vars.

Output schema:
%s

Files:
`, schema),

		"services": fmt.Sprintf(`Extract from Service, Consumer, Producer Kotlin files:
- handlers: Kafka @KafkaListener methods (name=method name, trigger_type="kafka-consumer", trigger_detail=topics consumed, description=what the listener does).
- events: Kafka topics produced via KafkaTemplate (event_type="kafka-producer", bus_name=topic name).
- external_apis: any HTTP client calls (RestTemplate, WebClient, Ktor) — name=target service/method, url=base URL or config property, method=HTTP verb, auth_type=bearer/none/api-key.
- Omit models, db_connections, config_vars.

Output schema:
%s

Files:
`, schema),

		"topics": fmt.Sprintf(`Extract Kafka topic definitions from enum classes and KafkaEvent implementations.

STEP 1 — Find all enum classes with a topic name string property (e.g. KafkaTopic, FooTopic):
  For each enum entry, the bus_name is the string literal value (e.g. "driver-events", "status-event").
  Output one event per enum entry:
    - name: enum entry name (e.g. "DRIVER_EVENTS_TOPIC")
    - event_type: "kafka-producer" if the topic is written to, "kafka-consumer" if read, "kafka-topic" if unknown direction
    - bus_name: the string literal (e.g. "driver-events") — NEVER use the enum entry name, always the string value
    - description: what entity/event flows through this topic

STEP 2 — For each class implementing a KafkaEvent interface (has kafkaTopic() method):
  Determine which enum entry it returns from kafkaTopic().
  Update the event_type for that bus_name based on whether it is produced (kafka-producer) or consumed.

STEP 3 — For @KafkaListener annotations in /event/ or /kafka/ directories:
  Output event_type="kafka-consumer", bus_name=topic string from the annotation.

Output only the events array. Do not emit handlers, models, db_connections, config_vars, or external_apis.

Output schema:
%s

Files:
`, schema),

		"config": fmt.Sprintf(`Extract from Spring @Configuration and @ConfigurationProperties Kotlin files:
- db_connections: DataSource beans (dialect=postgres/cassandra, host_var=property key for datasource URL, pool_min/max from HikariConfig).
- config_vars: @ConfigurationProperties fields (key=property prefix + field, source="spring-property", description=what it configures).
- external_apis: any WebClient/RestTemplate beans with base URLs.
- Omit handlers, events, models.

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
