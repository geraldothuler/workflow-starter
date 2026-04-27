---
name: health-check
description: >
  Avaliação de saúde de serviços em produção: pods, HPA, CPU/mem, latência p99, rollout recente e logs críticos.
  Ativar quando o usuário pedir "saúde", "health", "status dos serviços", "como estão os apps".
user-invocable: true
---

# Health Check — Serviços em Produção

## Configuração (adaptar ao projeto)

Antes de usar este skill, configure o mapa de serviços via memória operacional:

```bash
# Registrar serviços monitorados
wtb memory set health_services "api-principal,worker,scheduler" \
  --type config --topic health-check \
  --desc "Serviços monitorados pelo health-check"

wtb memory set k8s_context "seu-cluster-prod" \
  --type config --topic health-check \
  --desc "Contexto kubectl de produção"

wtb memory set k8s_namespace "sua-namespace" \
  --type config --topic health-check \
  --desc "Namespace k8s principal"
```

Ou consulte: `wtb memory list --topic health-check`

---

## ⛔ Guardrail obrigatório — 6 checks

**NUNCA reportar um serviço como saudável sem completar todos os 6 checks:**

1. **Pods:** status, age, restart count
2. **HPA:** replicas atuais vs min/max + métrica de scaling
3. **CPU/memória:** `kubectl top` vs limites do deployment
4. **Latência p99 + error rate:** APM (Datadog ou equivalente)
5. **Rollout recente:** `kubectl rollout status` + eventos do namespace
6. **Logs críticos:** OOM, CrashLoop, exceções nos últimos 15min

Análise rasa ("pods Running = saudável") é proibida — causa incidentes não detectados.

---

## Procedimento de avaliação

### 1. Pods — status e restarts

```bash
CONTEXT=$(wtb memory get k8s_context 2>/dev/null | awk '/value:/{print $2}')
NS=$(wtb memory get k8s_namespace 2>/dev/null | awk '/value:/{print $2}')

kubectl get pods -n ${NS} --context ${CONTEXT}
kubectl describe pods -n ${NS} --context ${CONTEXT} | grep -E "Restart|OOMKilled|Error"
```

Sinais de alerta: `CrashLoopBackOff`, restarts > 5 na última hora, pods em `Pending` ou `Error`.

### 2. HPA — estado de scaling

```bash
kubectl get hpa -n ${NS} --context ${CONTEXT}
```

Sinais de alerta: `TARGETS` próximo de 100%, replicas em `MAXPODS`, scale events recentes.

### 3. CPU/memória

```bash
kubectl top pods -n ${NS} --context ${CONTEXT}
```

Comparar com limites: `kubectl get deployment <nome> -n ${NS} -o yaml | grep -A4 resources:`.

Thresholds calibrados: `wtb memory list --topic <serviço>`.

### 4. Latência p99 + error rate

Use o MCP Datadog (preferido) ou REST API:

```bash
# Via MCP Datadog
# search_datadog_metrics + get_datadog_metric com query p99:trace.web.request{service:<nome>}

# Via REST (fallback)
DD_API_KEY=$(security find-generic-password -s "workflow-dd-api-key" -a $(whoami) -w)
DD_APP_KEY=$(security find-generic-password -s "workflow-dd-app-key" -a $(whoami) -w)
NOW=$(date +%s); FROM=$((NOW - 900))

curl -sf "https://api.datadoghq.com/api/v1/query?from=${FROM}&to=${NOW}&query=p99:trace.web.request{service:<nome-do-serviço>}" \
  -H "DD-API-KEY: $DD_API_KEY" -H "DD-APPLICATION-KEY: $DD_APP_KEY"
```

### 5. Rollout recente

```bash
kubectl rollout status deployment/<nome> -n ${NS} --context ${CONTEXT}
kubectl get events -n ${NS} --context ${CONTEXT} --sort-by='.lastTimestamp' | tail -20
```

### 6. Logs críticos (últimos 15min)

```bash
kubectl logs deployment/<nome> -n ${NS} --context ${CONTEXT} --since=15m \
  | grep -iE "OOM|CrashLoop|Exception|ERROR|FATAL|panic"
```

---

## Classificação de saúde

| Score | Significado | Ação |
|-------|-------------|------|
| ✅ HEALTHY | Todos os 6 checks ok | Nenhuma |
| ⚠️ DEGRADED | 1-2 checks com anomalia | Monitorar, abrir discovery |
| 🔴 CRITICAL | Pod down / OOM / CrashLoop | `wtb run ops-response` imediatamente |

---

## Escalada

```bash
# Anomalia confirmada → iniciar ops-response
wtb run ops-response --input symptom="<descrição>"

# Ou via MCP
# workflow_run: use_case=ops-response, inputs={symptom: "..."}
```
