🎯 GOLDEN PATHS (Resumo):

GP-001: Event Sourcing com Kafka
  - Quando: Eventos de telemetria IoT
  - Tech: Kafka com particionamento por device_id

GP-002: Stream Processing com Flink
  - Quando: Detecção de padrões em tempo real
  - Tech: Flink + RocksDB state backend

GP-003: Camada de Dados Híbrida
  - Quando: Alta volumetria writes + queries analytics
  - Tech: ScyllaDB (writes) + PostgreSQL (reads)

GP-004: Cache com Redis
  - Quando: >1000 req/s, dados que mudam pouco
  - Tech: Redis Cluster, write-through pattern

GP-005: Notificações Assíncronas
  - Quando: Push/email/SMS com retry
  - Tech: RabbitMQ + DLQ, circuit breaker

GP-006: Observabilidade ÚNICA com DataDog
  - Quando: SEMPRE (obrigatório)
  - Tech: DataDog APM (metrics + logs + traces + dashboards)
  - ❌ SEM Prometheus, Grafana, ELK, Jaeger

GP-007: Deploy Blue-Green
  - Quando: Deployments em produção
  - Tech: Kubernetes, canary 95/5 por 15min

GP-008: Segurança mTLS + JWT
  - Quando: APIs service-to-service
  - Tech: mTLS com rotação 30d, JWT 15min
