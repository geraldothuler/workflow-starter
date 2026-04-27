---
name: pod-cleanup
description: >
  Limpeza de pods efêmeros esquecidos nos clusters Cobli (cobli-prod e cobli-prod-devices).
  Remove automaticamente pods com label workflow.cobli/ephemeral=true ou padrões de nome conhecidos.
  Sinaliza pods Running suspeitos e pods de outros times.
  Ativar em: encerramento de sessão (Session Exit Rule), após investigações prod, ou quando usuário pedir "limpa pods", "pods esquecidos", "cleanup".
user-invocable: true
---

# Pod Cleanup — Clusters Cobli

Executar automaticamente no **Session Exit Rule** (antes do savepoint) e sempre que o usuário pedir limpeza de pods.

## Contextos e namespaces autorizados

| Context | Namespaces com permissão | Namespaces ignorados |
|---------|-------------------------|---------------------|
| `cobli-prod` | `organization`, `ecosystem` | `default` e todos os demais |
| `cobli-prod-devices` | `cobli-flink-jobs` | `default` e todos os demais |

> **Fonte de verdade:** `deployments.json`. Qualquer namespace não listado acima é tratado como "não nosso" — ignorar, não varrer.

---

## Convenção de pods efêmeros

Todo pod criado para investigação deve usar `~/workflow/scripts/kube-run.sh` em vez de `kubectl run` direto.
O wrapper injeta automaticamente `--labels=workflow.cobli/ephemeral=true`.

```bash
# Correto — usa wrapper
~/workflow/scripts/kube-run.sh meu-pod --image=python:3.11-slim -n cobli-flink-jobs \
  --context cobli-prod-devices --restart=Never --rm -i -- python3 - < script.py

# Incorreto — kubectl run direto (sem label)
kubectl run meu-pod --image=python:3.11-slim ...
```

---

## Procedimento

### 1. Coletar pods candidatos — apenas namespaces autorizados

```bash
# cobli-prod-devices
for ns in cobli-flink-jobs; do
  kubectl get pods -n "$ns" --context cobli-prod-devices 2>/dev/null \
    | grep -E "Completed|Evicted|Error"
done

# cobli-prod
for ns in organization ecosystem; do
  kubectl get pods -n "$ns" --context cobli-prod 2>/dev/null \
    | grep -E "Completed|Evicted|Error"
done
```

### 2. Classificação e ação

**Filtro primário — label (determinístico):**

| Label | Estado | Ação |
|-------|--------|------|
| `workflow.cobli/ephemeral=true` | Completed / Evicted / Error | **Auto-deletar** |

**Filtro secundário — padrão de nome (fallback para pods sem label):**

| Padrão | Estado | Ação |
|--------|--------|------|
| `cqlsh-*`, `*-probe`, `kubectl-run-*`, `*-shell-*`, `tmp-*` | Completed | **Auto-deletar** |
| `psql-*`, `scylla-*`, `fleet-health-*`, `wb-latency`, `stone-health` | Completed | **Auto-deletar** |
| `*-XXXXXXXXXX` (sufixo numérico 10 dígitos = Unix timestamp) | Completed | **Auto-deletar** |
| `*-debug-*`, `*-test-*` | Completed | **Auto-deletar** |
| Qualquer | Evicted | **Auto-deletar** |
| `*-debug-*`, `*-test-*` | Running >7d | **Sinalizar** — não deletar sem confirmar |
| Qualquer | Terminating >10min | **Sinalizar** — force-delete apenas se confirmado |

**Padrões a ignorar — mesmo dentro de namespaces autorizados:**

| Padrão | Motivo |
|--------|--------|
| `node-debugger-*` | Time de infra/SRE |
| `ecr-cred-refresher-*` | CronJob periódico de infra — não é efêmero de investigação |

### 3. Deletar pods classificados como auto-deletar

```bash
kubectl delete pod <nome1> <nome2> ... -n <namespace> --context <ctx>
```

Se o delete retornar `Forbidden`: registrar no report como "sem permissão" e prosseguir — não retentar.

```bash
# Evicted em lote (se encontrar muitos):
kubectl get pods -n <ns> --context <ctx> --field-selector=status.phase=Failed \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' \
  | xargs -r kubectl delete pod -n <ns> --context <ctx>
```

### 4. Formato de report

```
=== Pod Cleanup — <data> ===

AUTO-DELETADOS (<N> pods):
  cobli-prod-devices / cobli-flink-jobs:
    cqlsh-gap-xxx                Completed  2d    [label: ephemeral]
    fleet-health-1773762000      Completed  45h   [padrão: timestamp]
  cobli-prod / organization:
    rfid-sample-1773762723       Completed  7d    [padrão: timestamp]
  ...

SEM PERMISSÃO (pulados):
  cobli-prod / organization:
    algum-pod   Forbidden — registrado, não deletado

SINALIZADOS (não deletados — requerem atenção):
  cobli-prod / organization:
    debug-consumer-xxx   Running 84d   [confirmar com time antes de deletar]

Nenhum pod efêmero encontrado. ✅  ← (se limpo)
```

---

## Regras de segurança

- **Nunca varrer `default` ou namespaces fora da allowlist** — `node-debugger-*` e similares pertencem à infra.
- **Nunca deletar pods Running sem confirmação explícita**, mesmo com nome sugestivo.
- **Erro `Forbidden` no delete** → registrar no report, não retentar, prosseguir.
- `--force --grace-period=0` apenas se Terminating >10min E usuário confirmou.
- `ecr-cred-refresher-*` nunca deletar — é CronJob de infra, não pod de investigação.

---

## Integração — Session Exit Rule

Este cleanup deve rodar **como passo 0 do Session Exit**, antes da revisão de memory files:

```
0. /pod-cleanup — deletar efêmeros, sinalizar suspeitos
1. Novas heurísticas → memory files
2. Novos IDs / endpoints → topic file relevante
3. Regras novas → MEMORY.md
4. Skills com novo comportamento → SKILL.md correspondente
5. cost-report.sh
6. wtb cycle-check --save
```
