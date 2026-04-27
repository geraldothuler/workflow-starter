---
name: flink-deploy-recovery
description: Flink job deploy recovery — diagnose and recover from CrashLoopBackOff, state incompatibility, and operator ERROR RECONCILING. Apply when webhook-builder or any alex-job-chart Flink job fails to start after deploy.
user-invocable: false
---

# Flink Deploy Recovery — Cobli (alex-job-chart)

## Diagnóstico rápido

```bash
# 1. Status geral
kubectl get pods -n cobli-flink-jobs --context cobli-prod-devices | grep <job>
kubectl get flinkdeployment <job> -n cobli-flink-jobs --context cobli-prod-devices \
  -o jsonpath='{.status.jobManagerDeploymentStatus} {.status.jobStatus.state}'

# 2. Erro detalhado do operador
kubectl get flinkdeployment <job> -n cobli-flink-jobs --context cobli-prod-devices \
  -o yaml | grep -A5 '"error"'

# 3. Logs do JM (após filtrar ruído)
kubectl logs -n cobli-flink-jobs --context cobli-prod-devices deployment/<job> --tail=100 \
  | grep -v StatusLogger
```

## Árvore de decisão

```
JM em CrashLoopBackOff?
  └─ Sim → checar erro do operador
       ├─ "RecoveryFailureException: JobManager deployment is missing and HA data not available"
       │    └─ SOLUÇÃO: helm uninstall + install (ver abaixo)
       │
       ├─ "Cannot restore job. State incompatibility"
       │    ├─ Schema change em classe serializada (Kryo)? → helm uninstall + install
       │    └─ Operador removido/renomeado? → allowNonRestoredState pode ajudar (ver abaixo)
       │
       └─ Outro erro → checar logs JM completos
```

## Regra crítica: quando usar o quê

| Situação | Solução |
|----------|---------|
| JM crashando + `ERROR RECONCILING` no operador | **helm uninstall + install** |
| Mudança de tipo de state (BroadcastState → ValueState) | **helm uninstall + install** |
| Kryo schema incompatibility (novo campo em classe serializada) | **helm uninstall + install** |
| Operador *removido* da topologia (state não reclamado) | `allowNonRestoredState: true` pode funcionar |
| Upgrade normal (código sem mudança de state) | `helm upgrade` — operador tira savepoint automaticamente |

**Por que `allowNonRestoredState` NÃO resolve mudança de tipo:**
- Só bypassa state de operadores *removidos* da topologia (unclaimed state)
- Não resolve incompatibilidade de schema em operadores que *existem* mas mudaram de tipo
- Com JM em CrashLoopBackOff, o operador não consegue tirar savepoint → `ERROR RECONCILING`

## Procedimento: helm uninstall + install

```bash
# 1. Uninstall
helm uninstall <job> -n cobli-flink-jobs --kube-context cobli-prod-devices

# 2. Aguardar pods sumirem (10–15s)
sleep 15 && kubectl get pods -n cobli-flink-jobs --context cobli-prod-devices | grep <job>

# 3. Install — IMPORTANTE: passar jobImageVersion explicitamente (CI injeta; prod.yaml não tem)
helm install <job> ~/Cobliteam/webhook/<job>/deploy/helm/chart \
  -n cobli-flink-jobs \
  --kube-context cobli-prod-devices \
  -f ~/Cobliteam/webhook/<job>/deploy/helm/prod.yaml \
  --set "alex-job-chart.jobImageVersion=<SHA-do-commit>"

# 4. Verificar (aguardar ~30s)
sleep 30 && kubectl get pods -n cobli-flink-jobs --context cobli-prod-devices | grep <job>
kubectl get flinkdeployment <job> -n cobli-flink-jobs --context cobli-prod-devices \
  -o jsonpath='{.status.jobManagerDeploymentStatus} {.status.jobStatus.state}'
```

**Onde encontrar o SHA correto:**
```bash
# Último commit mergeado na main do repo webhook
git -C ~/Cobliteam/webhook log --oneline -1
# Ou: imagem anterior que estava funcionando (antes do crash)
kubectl get flinkdeployment <job> -n cobli-flink-jobs --context cobli-prod-devices \
  -o jsonpath='{.status.lastStableSpec}' | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['spec']['flinkConfiguration'].get('kubernetes.container.image','?'))"
```

## Implicações do cold start (sem savepoint)

| State | Comportamento | Impacto |
|-------|--------------|---------|
| `subscriptionState` (novo) | Começa null, populado pelo stream 2 em < 5min | Eventos descartados durante warm-up |
| `lastUnifiedState` | Começa null, reconstruído por eventos incoming | Perda de cooldown de position (3min) — 1 webhook extra possível por device |
| `odometerState` | Começa null, re-fetched do odometer API | Odômetro inicia do valor da API (correto) |
| BroadcastState antigo | Descartado (incompatível) | Esperado — este era o problema |

## Validação pós-deploy

```bash
# 1. Job rodando
kubectl get flinkdeployment <job> -n cobli-flink-jobs --context cobli-prod-devices \
  -o jsonpath='{.status.jobManagerDeploymentStatus} {.status.jobStatus.state}'
# Esperado: READY RUNNING

# 2. Throughput via Flink REST
kubectl port-forward -n cobli-flink-jobs --context cobli-prod-devices svc/<job>-rest 8081:8081 &
curl -s http://localhost:8081/jobs | python3 -c "import sys,json; print(json.load(sys.stdin))"
JOB_ID=<id>
curl -s "http://localhost:8081/jobs/$JOB_ID" | python3 -c "
import sys,json; d=json.load(sys.stdin)
for v in d['vertices']: print(v['name'][:50], v['status'], 'in:', v['metrics'].get('read-records','-'), 'out:', v['metrics'].get('write-records','-'))
"
kill $(lsof -ti:8081) 2>/dev/null

# 3. Verificar eventos em Scylla (clientes principais)
# ABC Cargas: b9ebc418-d5b4-41b0-a53f-10857345ed7b
# Stone:      6ef59d87-52f5-4dc9-9d4e-4091ecd1fcd7
```
