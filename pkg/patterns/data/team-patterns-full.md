👥 TEAM PATTERNS - DOCUMENTAÇÃO COMPLETA

## TP-001: Backend em Kotlin + Spring Boot
Stack: Spring Boot 3.2+ com Kotlin 1.9+
Detalhes:
- Ecossistema maduro (Spring Data, Spring Kafka)
- Time backend tem expertise (3+ devs seniores)
- Performance excelente (JVM otimizado)
- Observabilidade pronta (Actuator + Micrometer)
- Já rodamos 8+ serviços em produção
- WebFlux para reactive
- Arrow-kt para functional programming
- ❌ Java proibido (muito verboso)

Quando NÃO usar:
- Projeto Python/Django existente
- Time é 100% JavaScript (considerar NestJS)
- Caso de uso é script/worker simples (considerar Python)

## TP-002: Data Processing
Tech: Flink 1.17+ (streaming) + Kafka 3.5+ (messaging)
Detalhes:
- Flink com RocksDB state backend
- Kafka com particionamento por device_id
- ❌ Spark Streaming (latência alta)

Alternativas permitidas:
- Filas simples/pequenas → SQS ou RabbitMQ
- Pub/sub sem durabilidade → Redis Pub/Sub

## TP-003: Databases
Tech: PostgreSQL 15 + TimescaleDB, ScyllaDB 5.2+
Detalhes:
- PostgreSQL: Transações ACID, JSON/JSONB, PostGIS
- ScyllaDB: write-heavy workloads
- Flyway para migrations
- HikariCP para connection pooling
- ❌ MongoDB, DynamoDB

Alternativas aceitas por caso:
- Key-value → Redis
- Time-series → InfluxDB ou TimescaleDB
- Grafo → Neo4j
- Search → Elasticsearch

## TP-004: Build & Quality
Tech: Gradle Wrapper + Kotlin DSL + Ktlint
Detalhes:
- Ktlint obrigatório (CI quebra se falhar)
- Gradle Wrapper para build reproducível
- ❌ Maven proibido

## TP-005: CI/CD
Tech: CircleCI (único aprovado)
Detalhes:
- Path filtering obrigatório (economia de custo)
- Pipeline: build → ktlintCheck → test → deploy
- Testes automatizados
- Lint + Security scan
- Build de containers
- ❌ GitHub Actions, Jenkins, GitLab CI

CD: ArgoCD (Kubernetes)
- GitOps
- Rollback fácil
- Canary deployments quando possível

## TP-006: Schema Management
Tech: Git submodule (schemas versionados)
Detalhes:
- alert-schemas/ como submodule
- Versionamento junto com código
- ❌ Confluent Schema Registry

## TP-007: Observabilidade
Tech: DataDog APM (plataforma única)
Detalhes:
- Usar /actuator/metrics (DataDog coleta auto)
- ❌ Prometheus endpoints (/actuator/prometheus)
- RED metrics obrigatórias
- Alertas configurados

## TP-008: Anti-Patterns Infra
Detalhes:
- ❌ Service Mesh (Istio, Linkerd) → complexidade desnecessária
- ✅ mTLS direto + Kubernetes policies
- ✅ TLS/HTTPS obrigatório
- Dependency scanning (Snyk ou Dependabot)
- OWASP Top 10 verificado
