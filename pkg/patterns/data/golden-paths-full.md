🎯 GOLDEN PATHS - DOCUMENTAÇÃO COMPLETA

## GP-001: Event Sourcing com Kafka
Quando: Eventos de telemetria IoT
Tech: Kafka com particionamento por device_id
Detalhes:
- Alto throughput (10k+ eventos/segundo)
- Durabilidade (retenção de 7 dias padrão)
- At-least-once delivery garantido
- Já rodamos 3 clusters em produção

## GP-002: Stream Processing com Flink
Quando: Detecção de padrões em tempo real
Tech: Flink + RocksDB state backend
Detalhes:
- Flink 1.17+ (streaming)
- Kafka 3.5+ (messaging)
- ❌ Spark Streaming (latência alta)

## GP-003: Camada de Dados Híbrida
Quando: Alta volumetria writes + queries analytics
Tech: ScyllaDB (writes) + PostgreSQL (reads)
Detalhes:
- PostgreSQL 15+ com TimescaleDB para time-series
- ScyllaDB 5.2+ para write-heavy workloads
- Transações ACID (PostgreSQL) para dados financeiros
- JSON/JSONB para dados semi-estruturados

## GP-004: Cache com Redis
Quando: >1000 req/s, dados que mudam pouco
Tech: Redis Cluster, write-through pattern
Detalhes:
- Redis 7+ com cluster gerenciado
- Latência sub-milisegundo
- Casos: cache, sessions, rate limiting
- TTL automático

## GP-005: Notificações Assíncronas
Quando: Push/email/SMS com retry
Tech: RabbitMQ + DLQ, circuit breaker
Detalhes:
- Dead letter queues para retry
- Circuit breaker para resiliência
- Múltiplos canais (push, email, SMS)

## GP-006: Observabilidade ÚNICA com DataDog
Quando: SEMPRE (obrigatório)
Tech: DataDog APM (metrics + logs + traces + dashboards)
Detalhes:
- RED metrics (Rate, Errors, Duration)
- Business metrics personalizados
- Structured JSON logging
- Distributed tracing
- ❌ SEM Prometheus, Grafana, ELK, Jaeger

## GP-007: Deploy Blue-Green
Quando: Deployments em produção
Tech: Kubernetes, canary 95/5 por 15min
Detalhes:
- ArgoCD (GitOps)
- Rollback fácil
- Container: Docker
- Orquestração: Kubernetes (GKE)
- IaC: Terraform

## GP-008: Segurança mTLS + JWT
Quando: APIs service-to-service
Tech: mTLS com rotação 30d, JWT 15min
Detalhes:
- mTLS direto (sem service mesh)
- JWT tokens com 15min de vida
- Rotação de certificados a cada 30 dias
- Secrets no Google Secret Manager
