---
name: webhook-local-validate
description: >
  Ciclo completo de validação local dos jobs Flink webhook-builder e webhook-sender antes de commit.
  Ordem obrigatória: ktlint → test → intTest → run Docker (simula prod).
  Garante que o job sobe, connecta no Kafka/ScyllaDB, operadores vão a RUNNING e Log4jReconfigurer funciona.
  Ativar antes de qualquer commit que altere código Kotlin nos repos webhook-builder ou webhook-sender.
user-invocable: true
---

# Skill: webhook-local-validate

Ciclo de validação local para os jobs Flink webhook. Executar sempre antes de commit.

---

## Pré-requisitos

```bash
# Verificar que Kafka e ScyllaDB estão up
docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "kafka|scylla"

# Se não estiverem:
cd ~/Cobliteam/webhook && make dev-up
# Aguardar saída: "healthy" para os dois containers
```

Containers esperados:
- `webhook-kafka-1` — Status `healthy`
- `webhook-scylla-1` — Status `healthy`

---

## Ciclo completo

### 1. ktlint

```bash
cd ~/Cobliteam/webhook

# Format (corrige automaticamente)
./gradlew :webhook-builder:ktlintFormat :webhook-sender:ktlintFormat

# Check (deve passar sem erros)
./gradlew :webhook-builder:ktlintCheck :webhook-sender:ktlintCheck
```

### 2. Testes unitários

```bash
./gradlew :webhook-builder:test :webhook-sender:test
```

### 3. IntTests (requer Kafka + ScyllaDB up)

```bash
# Builder: usa testcontainers para Kafka, ScyllaDB real (webhook-scylla-1)
./gradlew :webhook-builder:integrationTest

# Sender: usa ScyllaDB real (webhook-scylla-1)
./gradlew :webhook-sender:intTest
```

**Tabela alvo:** `herbie.webhook_events` no container `webhook-scylla-1` — deve existir.

### 4. Run local via Docker (simula prod)

**Por que Docker, não `./gradlew :module:run`:**
- `./gradlew :module:run` não tem `/opt/flink/log4j-console.properties` → `Log4jReconfigurer` não ativa → comportamento diferente de prod
- Docker com `flink:1.18.1-java17` + `-Dlog4j.configurationFile` apontando para path inexistente reproduz exatamente o startup do Flink em prod
- Confirmado que sem o fix, `StatusLogger: Reconfiguration failed` aparece; com o fix, logs JSON aparecem corretamente

#### 4a. Builder

```bash
cd ~/Cobliteam/webhook

# Build fat JAR
./gradlew :webhook-builder:shadowJar

# Remover container anterior se existir
docker rm -f webhook-builder-local 2>/dev/null || true

# Rodar em Docker simulando startup Flink prod
# --network host: conecta no Kafka e Scylla locais (127.0.0.1)
# -Dlog4j.configurationFile: aponta para path inexistente (igual ao Flink startup script)
# Log4jReconfigurer lê de /opt/flink/log4j-console.properties (nosso arquivo montado)
timeout 45 docker run --name webhook-builder-local --rm \
  --network host \
  -v "$(pwd)/webhook-builder/build/libs/webhook-builder-all.jar":/opt/flink/usrlib/app.jar \
  -v "$(pwd)/webhook-builder/src/main/resources/log4j-console.properties":/opt/flink/log4j-console.properties \
  flink:1.18.1-java17 \
  java \
    -Dlog4j.configurationFile=/opt/flink/conf/log4j-console.properties \
    --add-opens=java.base/java.util=ALL-UNNAMED \
    -cp "/opt/flink/lib/*:/opt/flink/usrlib/app.jar" \
    co.cobli.webhook.ApplicationKt 2>&1 | \
  grep -E "co\.cobli|StatusLogger|RUNNING|checkpoint|ERROR|Kafka|Cassandra|R:localhost" | head -40
```

#### 4b. Sender

```bash
cd ~/Cobliteam/webhook

# Build fat JAR
./gradlew :webhook-sender:shadowJar

# Remover container anterior se existir
docker rm -f webhook-sender-local 2>/dev/null || true

timeout 45 docker run --name webhook-sender-local --rm \
  --network host \
  -v "$(pwd)/webhook-sender/build/libs/webhook-sender-all.jar":/opt/flink/usrlib/app.jar \
  -v "$(pwd)/webhook-sender/src/main/resources/log4j-console.properties":/opt/flink/log4j-console.properties \
  flink:1.18.1-java17 \
  java \
    -Dlog4j.configurationFile=/opt/flink/conf/log4j-console.properties \
    --add-opens=java.base/java.util=ALL-UNNAMED \
    -cp "/opt/flink/lib/*:/opt/flink/usrlib/app.jar" \
    co.cobli.webhook.ApplicationKt 2>&1 | \
  grep -E "co\.cobli|StatusLogger|RUNNING|checkpoint|ERROR|Kafka|Cassandra|R:localhost" | head -40
```

---

## O que validar no output do run Docker

| Sinal | Esperado | Ação se ausente |
|-------|----------|----------------|
| **Ausência de `StatusLogger: Reconfiguration failed`** | ✅ obrigatório | Log4jReconfigurer com bug — verificar `setConfigLocation` |
| Linhas `co.cobli.webhook` em JSON / com nível INFO | ✅ | Log4j2 não ativou — verificar reconfigureIfNeeded() |
| `Job ... switched from state CREATED to RUNNING` | ✅ obrigatório | verificar logs de erro |
| `Unified Event Processor ... INITIALIZING to RUNNING` | ✅ obrigatório | erro no open() |
| `Scylla Async Write ... INITIALIZING to RUNNING` | ✅ obrigatório | ScyllaDB não conectou |
| `R:localhost/127.0.0.1:9042` | ✅ | ScyllaDB conectado |
| `Checkpoint storage is set to 'jobmanager'` | ✅ | normal: sem S3 local |

### Sinal de sucesso do Log4jReconfigurer

Com o fix correto, os logs de aplicação aparecem **antes** do job ir a RUNNING, no formato configurado pelo `log4j-console.properties`:

```
{"instant":{"epochSecond":...},"thread":"...","level":"INFO","loggerName":"co.cobli.webhook...","message":"..."}
```

Se aparecer apenas saída do Flink sem nenhuma linha `co.cobli`, o reconfigurer falhou silenciosamente.

---

## Avisos esperados — ignorar

| Aviso | Origem | Ignorar? |
|-------|--------|----------|
| `ClassNotFoundException: com.esri.core.geometry.ogc.OGCGeometry` | Cassandra driver verifica ESRI (opcional) | ✅ sim |
| `Class ... cannot be used as a POJO type` | Flink serializando Protobuf | ✅ sim |
| `class ... does not contain a setter/getter for field bitField0_` | Protobuf reflection | ✅ sim |
| `The configuration option taskmanager.cpu.cores ... is not set` | Modo mini-cluster | ✅ sim |
| `No metrics reporter configured` | Sem DataDog local | ✅ sim |
| `Failed to load web based job submission extension` | flink-runtime-web não no classpath | ✅ sim |
| `Log file environment variable 'log.file' is not set` | Sem arquivo de log local | ✅ sim |
| `Hadoop FS is not available` | Sem plugin S3 local | ✅ sim |

---

## Checkpoints locais

Localmente, `state.checkpoints.dir` não está configurado → Flink usa `jobmanager` como storage (in-memory).
Isso é intencional e esperado. Checkpoints são criados a cada ~30s (mesmo intervalo de prod).

---

## Notas importantes

- **Log4jReconfigurer ativa no Docker**: `/opt/flink/log4j-console.properties` é montado e existe. `-Dlog4j.configurationFile` aponta para path inexistente → simula exatamente o comportamento prod.
- **`log4j-console.properties` em `src/main/resources/`** é o arquivo que o `Log4jReconfigurer` lê explicitamente. Não é auto-detectado pelo Log4j2 (que procura `log4j2.properties` / `log4j2.xml`).
- **Kafka topics**: auto-criação está habilitada no container local (`KAFKA_AUTO_CREATE_TOPICS_ENABLE=true`). Não é necessário criar os tópicos manualmente.
- **ScyllaDB**: schema do `webhook-builder` usa `herbie.webhook_events`, sender usa `webhook_events` (sem keyspace prefix — resolve via `keyspace=herbie` na config).
- **`--network host`**: necessário para que o container Docker conecte no Kafka e ScyllaDB rodando em `127.0.0.1` no host.

---

## Ciclo rápido (apenas o que mudou)

```bash
cd ~/Cobliteam/webhook

# Se mudou só código sender (sem ScyllaDB schema change):
./gradlew :webhook-sender:ktlintFormat :webhook-sender:ktlintCheck :webhook-sender:test :webhook-sender:intTest :webhook-sender:shadowJar
docker rm -f webhook-sender-local 2>/dev/null || true
timeout 45 docker run --name webhook-sender-local --rm --network host \
  -v "$(pwd)/webhook-sender/build/libs/webhook-sender-all.jar":/opt/flink/usrlib/app.jar \
  -v "$(pwd)/webhook-sender/src/main/resources/log4j-console.properties":/opt/flink/log4j-console.properties \
  flink:1.18.1-java17 \
  java -Dlog4j.configurationFile=/opt/flink/conf/log4j-console.properties \
    --add-opens=java.base/java.util=ALL-UNNAMED \
    -cp "/opt/flink/lib/*:/opt/flink/usrlib/app.jar" \
    co.cobli.webhook.ApplicationKt 2>&1 | \
  grep -E "co\.cobli|StatusLogger|RUNNING|ERROR" | head -20

# Se mudou só código builder:
./gradlew :webhook-builder:ktlintFormat :webhook-builder:ktlintCheck :webhook-builder:test :webhook-builder:integrationTest :webhook-builder:shadowJar
docker rm -f webhook-builder-local 2>/dev/null || true
timeout 45 docker run --name webhook-builder-local --rm --network host \
  -v "$(pwd)/webhook-builder/build/libs/webhook-builder-all.jar":/opt/flink/usrlib/app.jar \
  -v "$(pwd)/webhook-builder/src/main/resources/log4j-console.properties":/opt/flink/log4j-console.properties \
  flink:1.18.1-java17 \
  java -Dlog4j.configurationFile=/opt/flink/conf/log4j-console.properties \
    --add-opens=java.base/java.util=ALL-UNNAMED \
    -cp "/opt/flink/lib/*:/opt/flink/usrlib/app.jar" \
    co.cobli.webhook.ApplicationKt 2>&1 | \
  grep -E "co\.cobli|StatusLogger|RUNNING|ERROR" | head -20
```
