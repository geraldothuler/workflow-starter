---
name: webhook-lag-resend
description: >
  Diagnóstico de lag do webhook-builder (Flink) e resend de eventos perdidos para frotas afetadas.
  Fluxo 1 — lag-check: verificar consumer lag atual via Flink REST API.
  Fluxo 2 — resend: cross-reference Snowflake × ScyllaDB por frota/período e reenviar ausentes (frotas legado/HMAC).
  Fluxo 2b — retransmit 401s via Kafka command topic: publicar JSON → webhook-sender job reprocessa com auth fresco (Stone OAuth e qualquer frota Bearer).
  Fluxo 3 — análise de latência source→send.
  Ativar quando: lag crescendo no builder, suspeita de eventos não entregues, pós-deploy com stateless restart, solicitação de resend para frota específica, 401s acumulados após incidente de auth.
user-invocable: true
---

# Skill: webhook-lag-resend

Dois fluxos independentes — executar conforme contexto.

---

## Fluxo 1 — Lag Check (Flink REST API)

### Contexto
- Namespace: `cobli-flink-jobs` | Context: `cobli-prod-devices`
- JM label: `component=jobmanager,app=webhook-builder`
- Vertices fixos (estáveis enquanto código não mudar):
  - device-path source: `bc764cd8ddf7a0cff126f51c16239658`
  - status-event source: `feca28aff5a3958840bee985ee7de4d3`
  - UEP (Unified Event Processor): `f72f7c2e79a4b446422535df9efb607d`

### Procedimento

```bash
# 1. Port-forward para JM (usar porta livre, ex: 8086)
JM_POD=$(kubectl get pods -n cobli-flink-jobs --context cobli-prod-devices \
  -l component=jobmanager,app=webhook-builder -o jsonpath='{.items[0].metadata.name}')
kubectl port-forward -n cobli-flink-jobs --context cobli-prod-devices $JM_POD 8086:8081 &>/dev/null &
sleep 2

# 2. Job ID atual
JOB_ID=$(curl -sf http://localhost:8086/jobs | python3 -c \
  "import sys,json; jobs=json.load(sys.stdin)['jobs']; print(next(j['id'] for j in jobs if j['status']=='RUNNING'))")

# 3. Lag por subtask (p=4 → subtasks 0-3)
curl -sf "http://localhost:8086/jobs/$JOB_ID/vertices/bc764cd8ddf7a0cff126f51c16239658/metrics?\
get=0.KafkaSourceReader.KafkaConsumer.records-lag-max,\
1.KafkaSourceReader.KafkaConsumer.records-lag-max,\
2.KafkaSourceReader.KafkaConsumer.records-lag-max,\
3.KafkaSourceReader.KafkaConsumer.records-lag-max" \
| python3 -c "
import sys,json
data=json.load(sys.stdin)
total=sum(float(m.get('value',0)) for m in data)
[print(f'  subtask {m[\"id\"].split(\".\")[0]}: {float(m.get(\"value\",0)):.0f}') for m in data]
print(f'  TOTAL: {total:.0f}')
"

# 4. Checkpoints
curl -sf "http://localhost:8086/jobs/$JOB_ID/checkpoints" | python3 -c "
import sys,json
d=json.load(sys.stdin); c=d.get('counts',{}); lat=d.get('latest',{}).get('completed',{})
print(f'completed={c.get(\"completed\")} failed={c.get(\"failed\")} duration={lat.get(\"end_to_end_duration\")}ms')
"
```

### Alertas
| Sinal | Diagnóstico |
|-------|-------------|
| Lag crescendo + UEP busy=100% | Paralelismo insuficiente — verificar `FLINK_PARALLELISM` env var |
| Lag crescendo + Scylla sink busy=100% | Scylla sobrecarregado — aguardar estabilização |
| Checkpoints falhando + lag crescendo | Backpressure — checar `cp_fail` rate |
| Lag zero mas checkpoints falhando | Estado corrompido — considerar restart stateless |

### Fix de paralelismo (causa raiz frequente)
`application.conf` hardcoda `parallelism=1`; `env.setParallelism()` sobrescreve helm. Fix obrigatório: env var.

```bash
# Escalar para p=4
kubectl patch flinkdeployment webhook-builder \
  -n cobli-flink-jobs --context cobli-prod-devices --type=json -p '[
    {"op":"replace","path":"/spec/job/parallelism","value":4},
    {"op":"add","path":"/spec/podTemplate/spec/containers/0/env/-","value":{"name":"FLINK_PARALLELISM","value":"4"}}
  ]'
```

---

## Fluxo 2 — Cross-reference SF × Scylla e Resend

### Contexto
- Tabela SF: `SILVER.GEOFENCE_EVENTS` (warehouse: `DATA_PLATFORM_ANALYSIS`)
- Tabela Scylla: `herbie.webhook_events` (PK: `fleet_id, event_time, device_id, event_type, id`)
- Credencial Scylla: k8s secret `webhook-builder-secrets` em `cobli-flink-jobs/cobli-prod-devices`, key `cassandra-password`
- SF: `~/.workflow/snowflake_query.py --json "SELECT ..."` (SSO cacheado — output inclui SSO header antes do JSON, usar `content[content.find('['):]`)
- Script principal: `~/workflow/scripts/probes/webhook_gap_resend.py` (reutilizável)

### Conexão Scylla — parâmetros obrigatórios

```python
from cassandra.cluster import Cluster
from cassandra.auth import PlainTextAuthProvider
from cassandra.policies import DCAwareRoundRobinPolicy
import uuid

# Endpoint: confirmar via kubectl get flinkdeployment webhook-builder
#   -n cobli-flink-jobs --context cobli-prod-devices
#   -o jsonpath='{.spec.podTemplate.spec.containers[0].env[10].value}'
SCYLLA_HOST = 'herbie-database.prod.aws.cobli.co'
SCYLLA_PASS = # kubectl get secret webhook-builder-secrets -n cobli-flink-jobs
              # --context cobli-prod-devices -o jsonpath='{.data.cassandra-password}' | base64 -d

auth = PlainTextAuthProvider('webhook_builder', SCYLLA_PASS)
cluster = Cluster(
    [SCYLLA_HOST], port=9042,
    auth_provider=auth,
    load_balancing_policy=DCAwareRoundRobinPolicy(local_dc='AWS_US_EAST_1')  # NÃO 'us-east-1'
)
session = cluster.connect('herbie')

# UUID sempre como tipo nativo — string crua gera InvalidRequest
FLEET_UUID = uuid.UUID(fleet_id_str)

# event_time retorna datetime — converter para ms:
# int(r.event_time.replace(tzinfo=timezone.utc).timestamp() * 1000)
```

**Pod probe:** `cobli-flink-jobs / cobli-prod-devices` — NetworkPolicy bloqueia `ecosystem/cobli-prod`.

```bash
SCYLLA_PASS=$(kubectl get secret webhook-builder-secrets \
  -n cobli-flink-jobs --context cobli-prod-devices \
  -o jsonpath='{.data.cassandra-password}' | base64 -d)
FLEET_ID=$(bash ~/workflow/scripts/secret-get.sh <keychain-key>)

kubectl run scylla-probe --image=python:3.11-slim \
  -n cobli-flink-jobs --context cobli-prod-devices \
  --restart=Never --rm -i \
  --env="SCYLLA_PASS=$SCYLLA_PASS" --env="FLEET_ID=$FLEET_ID" \
  -- python3 - << 'PYEOF'
# script aqui
PYEOF
```

### Frotas conhecidas
| Cliente | Keychain key (fleet UUID) | SF baseline | Evento types | Auth especial |
|---------|--------------------------|-------------|--------------|---------------|
| ABC Cargas | `workflow-abc-cargas-fleet-uuid` | `SILVER.GEOFENCE_EVENTS` ✅ | geofence only | — (HMAC) |
| Stone | `workflow-stone-fleet-uuid` | ❌ não usar SF | todos os tipos (position, ignition, geofence, speed, battery…) | Bearer token OAuth |

**ATENÇÃO — Keychain:** as chaves `workflow-webhook-client-*` retornam o client_id curto (ex: `b9ebc418`), NÃO o fleet UUID.
Sempre usar as chaves `workflow-*-fleet-uuid` para queries SF e Scylla.

```bash
# Resolver fleet_id (UUID completo) antes de usar
FLEET_ABC=$(bash ~/workflow/scripts/secret-get.sh workflow-abc-cargas-fleet-uuid)
FLEET_STONE=$(bash ~/workflow/scripts/secret-get.sh workflow-stone-fleet-uuid)
```

**Stone — event types observados (herbie.webhook_events):**
| Event type | % do volume |
|------------|-------------|
| position | ~72% |
| ignition_on / ignition_off | ~12% |
| position_sleep | ~6% |
| alert_driven_over_speed / end_alert_driven_over_speed | ~5% |
| geofence_in / geofence_out | ~4% |
| battery_external_low | ~1% |

**Stone — baseline de volume (dias úteis):** ~8k-25k eventos/dia. Fins de semana: ~3k-12k.
Queda brusca abaixo de 2k em dia útil → sinal de alerta. Verificar status codes no Scylla.

### Autenticação Stone (única frota com auth especial)

Stone usa OAuth Bearer token obtido antes do envio — **não usa HMAC `X-Cobli-Signature`**.

**⚠️ Stone retorna 401 para token inválido/expirado** — o webhook-sender não retenta em 401 por design (client error = permanente). Se token expirar silenciosamente, todos os eventos ficam com `status_code=401, number_retries=0` no Scylla sem reenvio automático. Exige resend manual após renovação do token/fix.
*(Observado 21-23/03/2026: 4.148 eventos com 401, todos retries=0. Recuperou automaticamente no lado Stone às 23/03 ~11h BRT.)*

```python
STONE_FLEET_ID = "6ef59d87-52f5-4dc9-9d4e-4091ecd1fcd7"  # confirmar via Keychain workflow-stone-fleet-uuid
STONE_AUTH_URL = "https://stonelog-homolog.stone.com.br/authentication/auth/token"
STONE_AUTH_SSM = "/cobli/k8s/prod/webhook-sender/AUTH_CREDENTIALS_JSON"  # {client_id, client_secret}

def fetch_stone_token() -> str:
    creds_json = subprocess.check_output(
        ["aws", "ssm", "get-parameter", "--name", STONE_AUTH_SSM,
         "--with-decryption", "--query", "Parameter.Value", "--output", "text"], text=True).strip()
    creds = json.loads(creds_json)
    body = json.dumps({"client_id": creds["client_id"], "client_secret": creds["client_secret"]})
    req = urllib.request.Request(STONE_AUTH_URL, data=body.encode(),
                                 headers={"Content-Type": "application/json"}, method="POST")
    with urllib.request.urlopen(req, timeout=15) as resp:
        data = json.loads(resp.read())
    return data.get("accessToken") or data.get("access_token")

# Adicionar no header antes do envio:
if FLEET_ID == STONE_FLEET_ID:
    stone_token = fetch_stone_token()
    extra_headers["Authorization"] = f"Bearer {stone_token}"

# Em caso de 400 ou 401: refresh e retry uma vez (Stone pode retornar 400 para auth inválido)
if status_code in (400, 401) and FLEET_ID == STONE_FLEET_ID:
    stone_token = fetch_stone_token()
    headers["Authorization"] = f"Bearer {stone_token}"
    # retry...
```

**Diagnóstico de token issue via Scylla:**
```bash
# Verificar % de 400s nos últimos dias — burst de 400 = token inválido
kubectl run scylla-probe ... -- python3 - << 'PYEOF'
# Buscar eventos e contar status codes em Python (GROUP BY não suportado em Scylla para colunas não-PK)
from collections import Counter
rows = list(session.execute(
    "SELECT status_code FROM herbie.webhook_events WHERE fleet_id=%s AND event_time>=%s AND event_time<%s",
    [FLEET_UUID, start_ms, end_ms]
))
counts = Counter(r.status_code for r in rows)
for code, n in sorted(counts.items(), key=lambda x: -x[1]):
    print(f"  {code}: {n} ({n/len(rows)*100:.1f}%)")
PYEOF
# Se status_code=400 > 5%: investigar token Stone
```

---

## Fluxo 2b — Retransmit 401s via Kafka command topic

**Substitui o resend manual (Python + SSM + kubectl pod) para cenários de 401s acumulados após incidente de auth.**
**Apenas frotas Bearer/OAuth** — Stone e similares. **Não usar para ABC Cargas (HMAC)**: o transformer (`AlexstraszaV1Transformer`) vive no sender e seria aplicado de novo sobre o body já transformado → `event_data: {}` + HMAC ausente → 400. ABC Cargas continua no Fluxo 2.

### Pré-requisitos
- webhook-sender PR #144 deployado com `KAFKA_RETRANSMIT_COMMANDS_TOPIC=webhook-retransmit-commands`
- Tópico `webhook-retransmit-commands` criado em prod (RF=3, 10 partições) — ✅ criado em 23/03/2026

### Quando usar
Eventos com `status_code=401` acumulados no Scylla após incidente de auth (token expirado, circuit-open, etc).
Publica comando JSON → job queries Scylla, reconstrói proto, emite no pipeline completo com auth fresco.
Não requer script Python, token SSM manual, nem kubectl pod probe.

### Formato do comando

```json
{
  "fleet_id": "<UUID>",
  "from_ms": <epoch_ms>,
  "to_ms": <epoch_ms>,
  "status_codes": [401]
}
```

`from_ms`/`to_ms` são baseados em `event_time` (timestamp GPS do device) — não wall-clock de envio.
Usar janela mais ampla que o período do incidente (ex: `±2h` além do window detectado).

### Publicar comando

```bash
# Resolver fleet UUID (Stone)
FLEET_ID=$(bash ~/workflow/scripts/secret-get.sh workflow-stone-fleet-uuid)

# Converter datas para epoch ms — ajustar conforme janela do incidente
FROM_MS=$(python3 -c "from datetime import datetime,timezone; print(int(datetime(2026,3,21,0,0,0,tzinfo=timezone.utc).timestamp()*1000))")
TO_MS=$(python3 -c "from datetime import datetime,timezone; print(int(datetime(2026,3,23,23,59,59,tzinfo=timezone.utc).timestamp()*1000))")

# Montar e publicar o JSON
COMMAND=$(python3 -c "import json; print(json.dumps({'fleet_id': '$FLEET_ID', 'from_ms': $FROM_MS, 'to_ms': $TO_MS, 'status_codes': [401]}))")

echo "$COMMAND" | docker run --rm -i \
  confluentinc/cp-kafka:7.4.0 \
  kafka-console-producer \
  --broker-list kafka-msk-1.prod.aws.cobli.co:9092,kafka-msk-2.prod.aws.cobli.co:9092,kafka-msk-3.prod.aws.cobli.co:9092 \
  --topic webhook-retransmit-commands
```

### O que acontece após publicação

1. `RetransmitTriggerFunction` lê o JSON do tópico Kafka
2. Queries Scylla: `fleet_id + event_time BETWEEN [from_ms, to_ms]`
3. Filtra `status_code in status_codes` (ex: `[401]`)
4. Reconstrói `WebhookEventPB` para cada evento
5. Emite no pipeline principal → `BearerAuthRoutingFunction` → `BearerAuthEnricherFunction` → `SendWebhookAsyncFunction` → `WebhookSendResultSink`
6. Auth fresco: token OAuth obtido no momento do envio (circuit breaker incluso — sem repeat-storm)
7. Atualiza `status_code` no Scylla (ex: `401 → 202`)
8. Slack ao concluir: `Events with 401 — Found: N | Attempted: N | Succeeded: N | Failed: N`

### Verificar resultado

```bash
# Status codes após retransmissão (via pod probe Scylla)
# Eventos que antes tinham 401 devem aparecer com 202 (Stone) ou 200 (ABC Cargas)
SCYLLA_PASS=$(kubectl get secret webhook-builder-secrets \
  -n cobli-flink-jobs --context cobli-prod-devices \
  -o jsonpath='{.data.cassandra-password}' | base64 -d)
FLEET_UUID=$(bash ~/workflow/scripts/secret-get.sh workflow-stone-fleet-uuid)

kubectl run scylla-probe --image=python:3.11-slim \
  -n cobli-flink-jobs --context cobli-prod-devices \
  --restart=Never --rm -i \
  --env="SCYLLA_PASS=$SCYLLA_PASS" --env="FLEET_UUID=$FLEET_UUID" \
  -- python3 - << 'PYEOF'
from cassandra.cluster import Cluster
from cassandra.auth import PlainTextAuthProvider
from cassandra.policies import DCAwareRoundRobinPolicy
from collections import Counter
from datetime import datetime, timezone
import os, uuid

auth = PlainTextAuthProvider('webhook_builder', os.environ['SCYLLA_PASS'])
cluster = Cluster(['herbie-database.prod.aws.cobli.co'], port=9042, auth_provider=auth,
                  load_balancing_policy=DCAwareRoundRobinPolicy(local_dc='AWS_US_EAST_1'))
session = cluster.connect('herbie')

fleet_uuid = uuid.UUID(os.environ['FLEET_UUID'])
# Ajustar janela conforme o incidente
t1 = int(datetime(2026,3,21,0,0,0,tzinfo=timezone.utc).timestamp()*1000)
t2 = int(datetime(2026,3,23,23,59,59,tzinfo=timezone.utc).timestamp()*1000)

rows = list(session.execute(
    "SELECT status_code FROM webhook_events WHERE fleet_id=%s AND event_time>=%s AND event_time<%s",
    [fleet_uuid, t1, t2]))
counts = Counter(r.status_code for r in rows)
print(f"Total: {len(rows)}")
for code, n in sorted(counts.items(), key=lambda x: -x[1]):
    print(f"  {code}: {n} ({n/len(rows)*100:.1f}%)")
PYEOF
```

### Gotchas

| Problema | Causa | Fix |
|----------|-------|-----|
| `found=0` após publicar comando | `event_time` dos 401s fora da janela `from_ms/to_ms` — event_time é GPS, não wall-clock | Ampliar range para ±3h além do incidente; verificar `event_time` real de alguns 401s via pod probe |
| Evento emitido mas `status_code` não atualiza | Job retransmite mas novo status_code = 401 novamente | Investigar se circuit ainda está aberto ou token inválido — verificar logs `bearer_auth_retransmit_start` |
| Stone: double-transform risk | Stone não tem transformer em `legacy-contracts.yml` | Seguro: `PayloadTransformerRegistry.resolve()` retorna null → passthrough direto |
| Paralelismo 1: retransmissão lenta | Rate limit 50ms/evento, 200ms entre batches de 50 | Esperado por design (anti-thundering-herd). 3k eventos ≈ ~5-10min |
| `event_type` desconhecido — skipped | Scylla tem event_type que não existe em `WebhookEventTypePB` | Ver log `retransmit_unknown_event_type` — conta em `skipped` no log final |

---

### Payload por frota

| Frota | Formato | event_type | Timestamp |
|-------|---------|------------|-----------|
| ABC Cargas (legado) | snake_case (`event_id`, `event_type`, `event_data`) | `geofence_in` / `geofence_out` | ISO 8601 |
| Stone (passthrough) | camelCase (`eventId`, `eventType`, `eventData`) | `GEOFENCE_IN` / `GEOFENCE_OUT` | ISO 8601 |

**Stone — metodologia de gap analysis diferente de ABC Cargas:**
- SF (`SILVER.GEOFENCE_EVENTS`) **não serve como baseline** — Stone assina todos os event_types, SF só tem geofence
- Usar **Scylla × Scylla histórico** (week-over-week) como referência de volume
- Principal sinal de gap: `status_code=400` em >5% dos eventos → token OAuth inválido/expirado
- Resend para Stone requer adaptação do payload (camelCase + todos os event_types) — estratégia ainda não definida formalmente; incidentes anteriores foram resolvidos com fix no webhook-sender + resend manual pelo time

### Procedimento resend (frotas legado)

**1. Exportar SF events**

> **Airbyte sync lag — regra obrigatória antes de usar SF como baseline:**
> SF (SILVER.GEOFENCE_EVENTS) é alimentado via Airbyte CDC horário (`0 0 * * * ?`, avg ~8.6min).
> Lag total: até **~70 minutos** entre evento no Fusca e disponibilidade no SF.
>
> | Período end_utc | Usar SF? | Ação |
> |-----------------|----------|------|
> | > 2h atrás | ✅ Sim | SF completo |
> | 1-2h atrás | ⚠️ Cautela | Adicionar buffer: `end_utc += 2h` na query Scylla |
> | < 1h atrás | ❌ Não | SF incompleto — usar só Scylla para verificação |

```bash
python3 ~/.workflow/snowflake_query.py --json "
SELECT device_id::TEXT, event_time::TEXT, type AS event_type,
       geofence_id::TEXT, latitude, longitude
FROM SILVER.GEOFENCE_EVENTS
WHERE FLEET_ID = '<fleet_id>'
  AND event_time >= '<start_utc>'
  AND event_time <  '<end_utc>'
ORDER BY event_time
" > /tmp/sf_events.json
# ATENÇÃO: arquivo pode ter header SSO antes do JSON — usar content[content.find('['):]
```

**2. Executar script principal (dry-run primeiro)**
```bash
cd /tmp && python3 webhook_gap_resend.py \
  --fleet-id <fleet_id> \
  --start "YYYY-MM-DD HH:MM:SS" \
  --end   "YYYY-MM-DD HH:MM:SS" \
  --dry-run

# Sem --dry-run para executar de fato
```

**3. Verificação pós-resend**
```bash
# Rerun com --dry-run — deve retornar 0 missing
# Se ainda restam com window=300s, ampliar: --window-secs 600 ou 1800
# Se ainda restam após janela ampla: gap verdadeiro — investigar por device_id específico
```

### Padrão de INSERT Scylla pós-resend
```sql
INSERT INTO herbie.webhook_events
  (id, fleet_id, event_time, event_type, device_id, body, url, status_code, number_retries)
VALUES (uuid, fleet_uuid, event_time_ms, 'geofence_in', 'device_id',
        'body_json', 'https://...', 200, 0);
```

---

## Bugs conhecidos no script / gotchas

| Problema | Causa | Fix |
|----------|-------|-----|
| `webhook_signature` não encontrada apesar de existir | `custom_headers` com valor `{'solid-api-sid': ...}` — "solid" contém substring "id" que o filtro descarta a linha | filtrar header por `stripped.startswith("id ")`, não `"id" in line` |
| Snowflake multi-statement erro | `cur.execute("USE DB; USE SCHEMA")` não suportado | separar em dois `cur.execute()` |
| SF output vazio / JSON parse fail | SSO header precede o JSON no arquivo | `content[content.find('['):]` |
| TM ID com `:` na URL Flink metrics | URL encoding necessário | `urllib.parse.quote(tm_id)` antes de usar em URL |
| `webhook_builder` user precisa de SELECT em `webhook_signature` | permissão confirmada | usar sempre `webhook_builder`, não outro user |
| `UnresolvableContactPoints` no pod probe | namespace sem acesso de rede ao Scylla | usar `cobli-flink-jobs/cobli-prod-devices` |
| `NoHostAvailable` com IPs 10.42.x.x | DC errado no `DCAwareRoundRobinPolicy` | usar `AWS_US_EAST_1`, nunca `us-east-1` |
| `InvalidRequest` na query Scylla | fleet_id passado como string | `uuid.UUID(fleet_id_str)` |
| `TypeError: datetime not JSON serializable` | `event_time` retorna `datetime`, não `int` | `.replace(tzinfo=utc).timestamp()*1000` |
| Colunas SF uppercase | SF retorna `DEVICE_ID`, `EVENT_TIME`, `EVENT_TYPE` | usar chaves uppercase ao acessar o dict |
| Count SF > Scylla mas missing_sf = 0 | SF tem múltiplos eventos para mesmo device+type dentro de ±30s | não é gap — é granularidade diferente |
| Scylla retorna 0 para janela que deveria ter eventos | Janela de timestamp calculada incorretamente (off-by-timezone ou erro de cálculo) | Sempre derivar timestamps via Python `datetime(y,m,d,tzinfo=utc).timestamp()*1000` — nunca calcular manualmente. Validar: imprimir `datetime.utcfromtimestamp(start_ms/1000)` antes de usar |
| `GROUP BY` em Scylla falha para colunas não-PK | Scylla não suporta GROUP BY em colunas fora da PK | Buscar todos os registros e agrupar com `collections.Counter` em Python |
| Stone 401 sem retry | `status_code=401` = client error permanente para o webhook-sender → sem retry automático (retries=0) | Se 401 > 5%: investigar token OAuth. Exige resend manual após fix. Observado 21-23/03/2026: 4.148 eventos 401, recuperação automática no lado Stone às ~11h BRT do dia 23 |
| `event_time` como `datetime` quebra comparação com `int` | Cassandra driver retorna `datetime` para colunas timestamp | Usar helper: `def get_ts(r): et=r.event_time; return int(et.replace(tzinfo=utc).timestamp()*1000) if isinstance(et, datetime) else et` |
| ABC Cargas retorna 400 "Dados em Formato Incorreto" | Campo `address` ausente no `event_data` — ABC Cargas valida presença do campo mesmo que vazio | Sempre incluir `"address": ""` no resend (ou valor real do `geo_map`). Sem address = 400 universal |
| HMAC resend computado sobre dado errado | Resend computava HMAC sobre `event_data` interno; ABC Cargas valida HMAC sobre o **body completo** (AlexstraszaV1 JSON inteiro) | Computar `hmac.new(key, full_body_str, sha256).hexdigest()` — não sobre `event_data` isolado |
| UPDATE Scylla falha silenciosamente para 429s | `event_time` no Scylla tem precisão de milissegundos (ex: `1773839145817`). Conversão via `norm_et` trunca para segundos → WHERE clause não bate na PK | Usar o objeto `datetime` original da row diretamente: `int(r.event_time.replace(tzinfo=utc).timestamp()*1000)` — não recomputar a partir da string |
| Java `UUID.nameUUIDFromBytes` ≠ `uuid.UUID(bytes=md5)` | Java seta bits de versão 3 e variante 2 no MD5; Python não seta automaticamente | `d=bytearray(hashlib.md5(name).digest()); d[6]=(d[6]&0x0F)|0x30; d[8]=(d[8]&0x3F)|0x80; uuid.UUID(bytes=bytes(d))` |

---

## Lições aprendidas

- **Janela Scylla deve ser mais ampla que SF**: eventos processados durante lag são gravados com event_time original, mas o dreno pode ocorrer horas depois → usar `±3-7h` além do período SF
- **300s de window é suficiente**: cobre casos normais; lag severo ainda assim grava event_time original
- **Tipo SF é CamelCase**: `GeofenceIn`/`GeofenceOut` → normalizar para `geofence_in`/`geofence_out` (frotas legado)
- **Nunca passar senha via `sed`**: caracteres especiais (`*`, `$`) corrompem — usar `--env` no kubectl
- **Ratio esperado SF/Scylla para frota com webhook total**: ~1.0; ~0.50 é baseline geral (nem todas as frotas têm webhook)
- **Airbyte lag invalida SF para dados recentes**: SF via Airbyte CDC horário (`0 0 * * * ?`, ~8.6min avg). Lag total até 70min. Para análises com end_time nas últimas 2h: estender janela Scylla em +2h OU aguardar o próximo sync completar antes de concluir gap real
- **Stone token**: buscar SSM uma vez antes do loop, refresh somente em 401; token tem `expiresIn` (segundos)
- **Stone payload**: o webhook-sender usa passthrough (`input.eventData`) — sem transformação. `PayloadTransformerRegistry.resolve()` retorna `null` para Stone → `passthroughPayloads` counter
- **Stone retorna 401 para auth inválido**: webhook-sender não retenta em 401 → todos os eventos ficam gravados no Scylla com `status_code=401, retries=0`, sem reenvio automático. Requer fix + resend manual. (Observado 21-23/03/2026: 4.148 eventos, recuperação automática no lado Stone às ~11h BRT do dia 23)
- **HDOP Stone**: campo não está presente no body dos eventos Stone — não é enviado no payload. Sem eventos com HDOP inválido porque o campo simplesmente não existe no schema Stone
- **Stone não usa SF como baseline**: assina todos os event_types (position, ignition, geofence, speed, battery…) — SF só tem geofence. Usar Scylla histórico week-over-week. Queda >80% em dia útil = alerta crítico
- **Stone volume baseline**: dias úteis ~8k-25k/dia; fim de semana ~3k-12k. Variação alta é normal (frota operacional)
- **Diagnóstico Stone**: verificar % de `status_code=401` primeiro — é a causa mais frequente de gap. Coletar todos os rows + `Counter` em Python (Scylla não suporta GROUP BY em colunas não-PK)
- **WRITETIME(body) é o único proxy de sent_at**: não existe coluna `sent_at` — usar `WRITETIME(body) // 1000` para ms de envio. UUID v3/v4 não têm timestamp
- **Status code por frota**: ABC Cargas = `200`, Stone = `202` — filtrar corretamente ao calcular latência de sucesso
- **Latência por event_type**: geofence é o mais rápido (~4-5s p50) em ambas frotas; ignition/sleep são mais lentos (>60s p50) por acumulação em janelas de processamento batch
- **ABC Cargas: `address` é obrigatório**: campo `address` no `event_data` é validado pelo endpoint `/api/macro/recordPositions`. String vazia é aceita. Ausência = 400 "Dados em Formato Incorreto" universal. Sempre incluir `"address": ""` no resend quando não disponível
- **ABC Cargas: HMAC sobre body completo**: `X-Cobli-Signature` = `HmacSHA256(secret_key, full_body_json_string)`. A chave é `"cobli"`. ABC Cargas valida o HMAC sobre o body AlexstraszaV1 recebido — não sobre o `event_data` interno
- **ABC Cargas: replay de body exato retorna 200**: ABC Cargas aceita reenvio do mesmo evento (não deduplication por event_id); útil para teste de conectividade/formato
- **Resend ABC Cargas: geo_map com name+address**: ao carregar geo_map dos 200s existentes, capturar `(name, address)` do `event_data` → usar no resend de missing events. Geofences sem histórico ficam com `name=""`, `address=""`

---

## Fluxo 3 — Análise de Latência (source → send)

### Metodologia: WRITETIME(body)

`WRITETIME(column)` no Scylla retorna **microssegundos** desde Unix epoch quando aquela célula foi gravada no SSTable.
Para `herbie.webhook_events`, `WRITETIME(body)` ≈ momento do envio concluído.

```python
latency_ms = WRITETIME(body) // 1000 - event_time_ms
latency_s  = latency_ms / 1000.0
```

**Filtro obrigatório — janela limpa:** usar `WRITETIME` (não `event_time`) como janela primária.
Eventos com `event_time` no range mas `WRITETIME` fora são de resend/catch-up e distorcem os percentis.

```python
# WRITETIME em microsegundos — t1_us/t2_us = t1_ms*1000, t2_ms*1000
rows_clean = [r for r in rows if r.wt and t1_us <= r.wt < t2_us
              and r.status_code and 200 <= r.status_code < 300]
```

**Por que não usar UUID?**
- `id` = UUID v3 (name-based MD5, determinístico) → sem timestamp
- `eventId` no body = UUID v4 (random) → sem timestamp
- Nunca tentar extrair timestamp de UUIDs desta tabela

### Categorias de latência — separar antes de interpretar

| Categoria | Causa | Observável | Ação |
|-----------|-------|------------|------|
| **Pipeline puro** | Flink UEP → sender → Scylla | Geofence p50=5s — device online durante evento | Medir e monitorar |
| **Device delivery delay** | Device offline → buffer → reconnect | Ignition/position_sleep p50=75s, max=16h | Esperado — não é problema de pipeline |
| **Design: position suppression** | `PositionWebhookStrategy`: emite só se ≥3min desde último por device (event_time-based) | Reduz volume ~3-5×; pipeline latency dos eventos que passam é normal | Documentar, não otimizar |
| **Design: battery debounce** | `BatteryExternalLowWebhookStrategy`: 240min (4h) por device (env: `BATTERY_LOW_EVENT_DEBOUNCE_MINUTES`) | 1 alerta/4h/device por design | Não medir como latência de entrega |
| **Catch-up / incidente** | Resend manual pós-incidente, ou lag acumulado | WRITETIME >> event_time por horas/dias | Excluir com filtro WRITETIME na janela |

**Conclusão:** `WRITETIME - event_time` ≠ latência de pipeline pura para todos os tipos.
O p50 de geofence (~5s) é o melhor indicador do pipeline. Ignition/position p50 alto = device buffer, não atraso do pipeline.

### Resultados de referência — janela limpa (14/03 14:00 – 15/03 14:00 UTC)

| Frota | Types | Status | p50 | p75 | p90 | >600s |
|-------|-------|--------|-----|-----|-----|-------|
| ABC Cargas | geofence only | 200 | 5.1s | 20.8s | 377s | 8.8% |
| Stone | todos | 202 | 6.2s | 75.1s | 401s | 7.7% |

**Stone por event_type (janela limpa):**
```
geofence_in/out  : p50=4-5s    p90=9-13s     ← pipeline puro
position         : p50=5.1s    p90=250s      ← device buffer nos outliers; supressão 3min/device
ignition_off     : p50=75.2s   p90=1591s     ← device delivery delay (sleep → reconnect)
position_sleep   : p50=75.1s   p90=1591s     ← idem (correlacionado com ignition_off)
ignition_on      : p50=202s    p90=4293s     ← device wakes up com buffer acumulado
battery          : p50=5s      —             ← pipeline rápido; debounce 4h é design, não pipeline
```

**Heurísticas SLA — pipeline:**
- **Geofence**: p50 < 10s = saudável; p50 > 30s = lag ou problema de pipeline
- **Position**: p50 < 10s = saudável; tail alto é normal (device offline)
- **Ignition/sleep**: p50 alto por design (device buffer) — não usar como SLA de pipeline
- **Alerta geral**: proporção de >600s acima de 15% → investigar lag/parallelism no Flink

### Procedimento

```bash
SCYLLA_PASS=$(kubectl get secret webhook-builder-secrets \
  -n cobli-flink-jobs --context cobli-prod-devices \
  -o jsonpath='{.data.cassandra-password}' | base64 -d)
FLEET_UUID=$(bash ~/workflow/scripts/secret-get.sh workflow-<frota>-fleet-uuid)

kubectl run wb-latency --image=python:3.11-slim \
  -n cobli-flink-jobs --context cobli-prod-devices \
  --restart=Never --rm -i \
  --env="SCYLLA_PASS=$SCYLLA_PASS" --env="FLEET_UUID=$FLEET_UUID" \
  -- python3 - < ~/workflow/scripts/probes/webhook_latency_analysis.py
```

Script usa janela default: últimas 42h (com buffer de 2h p/ Airbyte). Ajustar `T1`/`T2` inline para períodos específicos.

---

## Scripts reutilizáveis

| Script | Uso |
|--------|-----|
| `~/workflow/scripts/probes/webhook_gap_resend.py` | Resend completo: SF × Scylla, geofence names, envio, INSERT Scylla |
| `~/workflow/scripts/probes/stone_health_check.py` | Health check Stone: histórico 7d + status codes + event types + alertas automáticos |
| `~/workflow/scripts/probes/webhook_latency_analysis.py` | Análise de latência source→send: WRITETIME, percentis, por event_type |
| `~/workflow/monitors/wb-drain-monitor.sh` | Monitor de lag em loop (2min interval, para ao zerar) |
| `~/workflow/scripts/probes/webhook_gap_lost_events.csv` (gitignored) | Cache dos eventos ausentes da última execução |

**Padrão de execução de scripts via pod probe:**
```bash
SCYLLA_PASS=$(kubectl get secret webhook-builder-secrets \
  -n cobli-flink-jobs --context cobli-prod-devices \
  -o jsonpath='{.data.cassandra-password}' | base64 -d)
STONE_UUID=$(bash ~/workflow/scripts/secret-get.sh workflow-stone-fleet-uuid)

kubectl run stone-health --image=python:3.11-slim \
  -n cobli-flink-jobs --context cobli-prod-devices \
  --restart=Never --rm -i \
  --env="SCYLLA_PASS=$SCYLLA_PASS" --env="STONE_UUID=$STONE_UUID" \
  -- python3 - < ~/workflow/scripts/probes/stone_health_check.py
```
