---
name: ops-probes
description: Probe scripts para diagnóstico operacional de Flink, Kafka, ScyllaDB e PostgreSQL. Carrega o contrato de exit codes, protocolo de self-healing (exit 3 → investigar), e ciclo de auto-evolução via probe-evolve.sh.
user-invocable: false
---

# ops-probes — Contrato e Protocolo

Base: `~/workflow/scripts/probes/`

## Mapa de scripts

| Script | Quando usar | Exit codes |
|--------|------------|------------|
| `flink_cold_start_diagnosis.sh` | Diagnóstico completo de saúde do Flink job — HEALTHY/COLD_START/CATCHING_UP/BACKPRESSURE | 0=healthy 1=erro 2=cold_start **3=heal_failed** |
| `flink_job_metrics.sh` | Overview rápido: estado, uptime, contadores, checkpoints | 0=ok 1=erro 2=sem job |
| `kafka_consumer_lag.sh` | Consumer lag via Flink REST (mais confiável que kafka-consumer-groups) | 0=ok 1=erro 2=sem RUNNING |
| `k8s_app_health.sh` | Saúde de apps k8s: pods, memória, logs — fusca, icarus, iris, cerberus, severino, webhook-sender | 0=healthy 1=erro 2=pod_down 3=oom_risk 4=crash_loop |
| `postgres_webhook_subscriptions.sh` | Subscrições webhook ativas no PostgreSQL fusca | 0=ok 1=erro 2=sem subscrição |
| `scylla_fleet_health.py` | Saúde de entregas por frota (Scylla herbie.webhook_events) | 0=ok |
| `probe-evolve.sh` | Discovery de novos jobs + calibração de thresholds do learn.log | 0=ok 1=erro |

## Config central: `deployments.json`

Lido por todos os scripts. Prioridade: env var \| CLI arg > JSON > fallback hardcoded.

```bash
# Rodar com job alternativo — lê config automaticamente
bash scripts/probes/flink_cold_start_diagnosis.sh --job sherlock-driver-stream
```

## Contrato de exit codes — ação obrigatória

| Exit code | Significado | Claude faz |
|-----------|------------|------------|
| 0 | healthy / ok | nada |
| 1 | erro genérico (kubectl, curl, etc.) | reportar ao usuário |
| 2 | cold_start ou sem job | reportar diagnóstico |
| **3** | **heal_failed — ambiguidade ou nenhum pod** | **investigar + corrigir** (ver protocolo abaixo) |
| 4 | odometer_loop detectado | reportar diagnóstico |

## Protocolo exit 3 — investigação obrigatória

Quando um probe retorna exit 3:

1. Ler o JSON diagnóstico em `/tmp/probe-heal-<job>-<ts>.json` (path impresso no stderr)
2. O JSON contém: `tried_label`, `found_pods`, `all_vertices` (conforme o tipo de falha)
3. Investigar:
   - **JM label**: comparar `tried_label` com `found_pods[].app` → identificar app correto
   - **Vertex ambiguidade**: ler `all_vertices` → determinar qual é UEP e qual é Sink
4. Corrigir `deployments.json` com `_update_config` ou editar direto
5. Re-rodar o probe

```bash
# Leitura do diagnóstico (path vem no stderr do probe)
cat /tmp/probe-heal-<job>-<ts>.json | jq .

# Correção manual se necessário
jq '.flink["webhook-builder"].jm_label = "component=jobmanager,app=<novo-app>"' \
  ~/workflow/scripts/probes/deployments.json > /tmp/d.json && mv /tmp/d.json \
  ~/workflow/scripts/probes/deployments.json
```

## Self-healing automático — o que já acontece sem intervenção

| Situação | Comportamento automático |
|----------|------------------------|
| JM label errado, mas pod existe | `_heal_jm_label` descobre, atualiza config, re-executa |
| Vertex name mudou, match fuzzy claro | Python faz fuzzy match, shell atualiza config, diagnóstico continua |
| Vertex ambíguo (UEP = Sink) | exit 3 + JSON → Claude investiga |
| Nenhum pod jobmanager | exit 3 + JSON → Claude investiga |

## Ciclo de auto-evolução

### Novo job Flink deployado
```bash
bash scripts/probes/probe-evolve.sh --discover           # dry-run: mostra o que seria adicionado
bash scripts/probes/probe-evolve.sh --discover --apply   # aplica em deployments.json
# Par novo (cluster/namespace desconhecido):
bash scripts/probes/probe-evolve.sh --discover --namespace <ns> --context <ctx> --apply
```

### Session Exit — consolidar lições do dia
```bash
bash scripts/probes/probe-evolve.sh --calibrate          # dry-run: propõe ajustes
bash scripts/probes/probe-evolve.sh --calibrate --apply  # aplica threshold calibration
```

**Calibração aplica quando:**
- `jm_label` ou `vertex` foi healed ≥2x com o mesmo valor → promove para config definitiva
- `fail_loop_idle_per_min` p90 observado (HEALTHY ≥10min) < 50% do threshold atual → propõe p90×3

### learn.log — observações acumuladas

```
scripts/probes/learn.log  ← JSONL commitado, append-only
```

Tipos: `jm_label_healed`, `vertex_healed`, `threshold_observation`.

Inspecionar:
```bash
cat ~/workflow/scripts/probes/learn.log | jq -s 'group_by(.type) | .[] | {type: .[0].type, count: length}'
```

## Scylla fleet health — invocação

Não tem wrapper bash — roda dentro de pod kubectl:

```bash
SCYLLA_PASS=$(kubectl get secret webhook-builder-secrets -n cobli-flink-jobs \
  --context cobli-prod-devices -o jsonpath='{.data.cassandra-password}' | base64 -d)
FLEET_UUID=$(bash ~/workflow/scripts/secret-get.sh workflow-<frota>-fleet-uuid)

kubectl run fleet-health-$(date +%s) --image=python:3.11-slim \
  -n cobli-flink-jobs --context cobli-prod-devices \
  --restart=Never --rm -i \
  --env="SCYLLA_PASS=$SCYLLA_PASS" \
  --env="FLEET_UUID=$FLEET_UUID" \
  --env="FLEET_NAME=<nome>" \
  -- python3 - < ~/workflow/scripts/probes/scylla_fleet_health.py
```

## Quando NÃO usar estes scripts

- **Deploy recovery (CrashLoopBackOff, state incompatibility)** → skill `flink-deploy-recovery`
- **Lag crescendo + resend de eventos** → skill `webhook-lag-resend`
- **Saúde geral de todos os apps de uma vez** → `k8s_app_health.sh --all` (já parte deste conjunto) ou skill `health-check` para contexto e bearer auth
