👥 TEAM PATTERNS (Resumo):

TP-001: Backend em Kotlin + Spring Boot
  - ✅ Kotlin obrigatório (null-safety, coroutines)
  - ❌ Java proibido (verboso)
  - Stack: Spring Boot 3.2+, WebFlux, Arrow-kt

TP-002: Data Processing
  - ✅ Flink 1.17+ (streaming)
  - ✅ Kafka 3.5+ (messaging)
  - ❌ Spark Streaming (latência alta)

TP-003: Databases
  - ✅ PostgreSQL 15 + TimescaleDB
  - ✅ ScyllaDB 5.2+
  - ❌ MongoDB, DynamoDB

TP-004: Build & Quality
  - ✅ Gradle Wrapper + Kotlin DSL
  - ✅ Ktlint obrigatório (CI quebra se falhar)
  - ❌ Maven proibido

TP-005: CI/CD
  - ✅ CircleCI (único aprovado)
  - ❌ GitHub Actions, Jenkins, GitLab CI
  - Pipeline: build → ktlintCheck → test → deploy

TP-006: Schema Management
  - ✅ Git submodule (schemas versionados)
  - ❌ Confluent Schema Registry
  - Estrutura: alert-schemas/ como submodule

TP-007: Observabilidade
  - ✅ DataDog APM (plataforma única)
  - ❌ Prometheus endpoints (/actuator/prometheus)
  - Usar: /actuator/metrics (DataDog coleta auto)

TP-008: Anti-Patterns Infra
  - ❌ Service Mesh (complexidade desnecessária)
  - ✅ mTLS direto + Kubernetes policies
