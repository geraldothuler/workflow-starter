---
name: health-check
description: >
  Avaliação de saúde dos apps Cobli: fusca, icarus, iris, cerberus, severino e webhooks.
  Avaliar pods, memória, CPU, restarts e erros críticos (OOM, CrashLoop, exceções).
  Ativar quando o usuário pedir "saúde", "health", "status dos apps", "como estão os serviços".
user-invocable: true
---

# Health Check — Apps Cobli

## Mapa de serviços

| Serviço | Namespace | Context | Deployment(s) |
|---------|-----------|---------|---------------|
| fusca-api | organization | cobli-prod | fusca-api-app-deployment |
| fusca-driver-past-retry | organization | cobli-prod | fusca-driver-association-past-retry-job-deployment |
| fusca-identification-token | organization | cobli-prod | fusca-identification-token-job-deployment |
| fusca-cassandra-sync | organization | cobli-prod | fusca-cassandra-sync-app-deployment |
| icarus | ecosystem | cobli-prod | icarus-v1-app-deployment |
| iris | ecosystem | cobli-prod | iris-app-deployment |
| cerberus-api | ecosystem | cobli-prod | cerberus-api-app-deployment |
| cerberus-keys-api | ecosystem | cobli-prod | cerberus-keys-api-app-deployment |
| severino | organization | cobli-prod | severino-app-deployment |
| webhook-builder | cobli-flink-jobs | cobli-prod-devices | webhook-builder + webhook-builder-taskmanager |
| webhook-sender | cobli-flink-jobs | cobli-prod-devices | webhook-sender + webhook-sender-taskmanager |

## ⛔ Guardrail obrigatório

**NUNCA reportar um serviço como saudável sem completar todos os 6 checks:**
1. Pods: status, age, restarts
2. HPA: replicas atuais vs min/max + métrica de scaling
3. CPU/memória: `kubectl top` vs limites
4. Latência p99 + error rate (Datadog APM)
5. Rollout recente: `kubectl rollout status` + eventos do namespace
6. Logs de erro crítico (OOM, CrashLoop, exceções nos últimos 15min)

Análise rasa ("pods Running = saudável") é proibida — causa incidentes não detectados.

---

## Procedimento de avaliação

### 1–3. Apps k8s — pods, recursos e logs

```bash
# Todos os apps de uma vez (pods + kubectl top + logs):
bash ~/workflow/scripts/probes/k8s_app_health.sh --all

# App individual:
bash ~/workflow/scripts/probes/k8s_app_health.sh --app fusca-api
```

| Exit | Significado | Ação |
|------|-------------|------|
| 0 | ✅ HEALTHY | nada |
| 2 | 🔴 POD_DOWN | verificar deployment, eventos do namespace |
| 3 | ⚠️ OOM_RISK | verificar threshold via `wtb memory list --topic <serviço>` |
| 4 | 🔴 CRASH_LOOP / RESTARTS | `kubectl logs deployment/<deploy> --previous -n <ns> --context cobli-prod` |

Apps monitorados (config em `deployments.json["apps"]`):
`fusca-api`, `fusca-cassandra-sync`, `fusca-driver-past-retry`, `fusca-identification-token`,
`icarus-v1`, `iris`, `cerberus-api`, `cerberus-keys-api`, `severino`, `webhook-sender`

### 4. HPA — estado de scaling

```bash
# Todos os apps monitorados:
for ns in organization ecosystem; do
  echo "=== $ns ==="
  kubectl get hpa -n $ns --context cobli-prod 2>/dev/null
done
kubectl get hpa -n cobli-flink-jobs --context cobli-prod-devices 2>/dev/null
```

Sinais de alerta: `TARGETS` muito próximo de 100%, replicas em `MAXPODS`, eventos de scale recentes.

### 5. Latência p99 + error rate — Datadog APM

Usar MCP Datadog (via `/datadog-rca`) ou REST:

```bash
DD_API_KEY=$(bash ~/workflow/scripts/secret-get.sh workflow-dd-api-key)
DD_APP_KEY=$(bash ~/workflow/scripts/secret-get.sh workflow-dd-app-key)
NOW=$(date +%s); FROM=$((NOW - 900))  # últimos 15min

# Latência p99 — cerberus-api (exemplo)
curl -sf "https://api.datadoghq.com/api/v1/query?from=${FROM}&to=${NOW}&query=p99:trace.web.request{service:cerberus-api}" \
  -H "DD-API-KEY: $DD_API_KEY" -H "DD-APPLICATION-KEY: $DD_APP_KEY" \
  | python3 -c "import json,sys; d=json.load(sys.stdin); pts=d['series'][0]['pointlist'] if d.get('series') else []; print('p99 atual:', pts[-1][1] if pts else 'sem dados', 'ms')" 2>/dev/null
```

Thresholds calibrados: `wtb memory list --topic cerberus` / `--topic webhook` / `--topic fusca`.

### 6. Webhooks Flink — diagnóstico completo via REST

```bash
bash ~/workflow/scripts/probes/flink_cold_start_diagnosis.sh --job webhook-builder
bash ~/workflow/scripts/probes/flink_cold_start_diagnosis.sh --job webhook-sender
```

| Exit | Significado | Ação |
|------|-------------|------|
| 0 | ✅ HEALTHY | nada |
| 2 | 🔴 COLD_START / sem job | verificar source topics e consumer group |
| 3 | 🔴 HEAL_FAILED | carregar skill `ops-probes` → protocolo exit 3 |

Recursos TM (não cobertos pelos probes — checar separadamente):
```bash
kubectl top pods -n cobli-flink-jobs --context cobli-prod-devices | grep webhook
wtb memory list --topic webhook   # limites e thresholds calibrados
```

- Monitor A6 ID: `security find-generic-password -s workflow-dd-monitor-webhook-builder-a6 -w`
- Clientes principais: ABC Cargas (`b9ebc418`) e Stone (`6ef59d87`) — checar `webhook_events` se suspeito

### 5. Contexto específico por serviço

**Fusca:** fonte de verdade de vehicle/device. OOM histórico em `DriverAssociationPastRetryCron` — checar PENDING_EVENTS se cron com restarts.

**Severino:** Play 2.6 + HikariCP. `connectionTimeout=5000ms` (safe); `keepaliveTime` deve estar configurado. Checar pool exhaustion se `TimeoutException`.

**Icarus:** branch padrão `icarus-v1`. Verificar `icarus-v1-app-deployment` (não `master`).

**Iris:** parte do pipeline webhook. Erros de validação de assinatura refletem no builder.

**Cerberus:** API de autenticação. Latência alta → HPA metric. Limite CPU 500m após PR #116.

**webhook-sender — bearer auth degraded mode (Stone):**

O sender tem circuit breaker para o endpoint de auth da Stone. Indisponibilidade do lado da Stone é esperada (especialmente em homolog) e **não é anomalia**. Avaliar com a seguinte lógica:

```bash
# Extrair eventos de auth dos últimos 30min
kubectl logs -n cobli-flink-jobs --context cobli-prod-devices \
  deployment/webhook-sender-taskmanager --since=30m 2>/dev/null \
  | grep -iE "bearer_auth_error|bearer_auth_circuit|bearer_auth_token_refresh_failed|X-Auth-Unavailable"
```

**Classificação dos eventos de auth:**

| Evento | Nível | Interpretação |
|--------|-------|---------------|
| `bearer_auth_error` (intervalo crescente) | ℹ️ info | Circuit breaker abrindo com backoff exponencial — normal |
| `bearer_auth_error` a cada ~300s | ℹ️ info | Estabilizou no cap (5min) — probe mínimo, sem overhead |
| `bearer_auth_circuit_open` / `bearer_auth_token_refresh_failed` | ℹ️ info | Probes intermediários — esperado |
| `bearer_auth_circuit_closed` | ℹ️ info | Stone voltou, circuit fechado — retransmissão automática |

**Reportar como ℹ️ (informativo, sem ⚠️ ou 🔴) quando:**
- `bearer_auth_error` presente mas com intervalos crescentes (backoff ok)
- Pod `Running`, 0 restarts, memória estável
- Erros restritos ao fleet Stone (`6ef59d87`) — outros fleets não afetados

**Escalar para ⚠️ ou 🔴 apenas se houver anomalia do nosso lado:**

| Anomalia | Sinal |
|----------|-------|
| Memory leak | TM mem crescendo continuamente entre health checks, sem plateau |
| Thundering herd | `bearer_auth_error` em rajada sem backoff (intervalos < 5s) |
| Contaminação de outros fleets | Erros com fleet_id ≠ Stone (`6ef59d87`) |
| Circuit nunca fecha | `bearer_auth_circuit_closed` ausente após Stone voltar (verificar via Slack #team-heimdall-alarms) |
| Retransmissão travada | `bearer_auth_retransmit_complete` ausente após recovery + `bearer_auth_retransmit_skip` repetido |
| Pod restart | Qualquer restart no TM — pode indicar OOM do executor de retransmissão |
| Job não RUNNING | FlinkDeployment `FAILED` ou `ERROR RECONCILING` |
