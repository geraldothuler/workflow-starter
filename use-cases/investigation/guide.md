# Use Case: Investigation — Playbook Investigation

**Tipo:** pipeline | **Engine principal:** pkg/playbook, pkg/infracontext

---

## Quando usar

- Incidente ativo e a causa raiz não está clara
- Incidente encerrado e você precisa de análise estruturada para o postmortem
- Anomalia detectada que requer investigação systematic

## Pipeline

```
incidente/anomalia
    ↓
run_playbook (pkg/playbook)
    → executar analyzers YAML-driven (ex: CDC lag, lock contention)
    → construir causal chain com evidências
    ↓
enrich_with_infra (pkg/infracontext)       [opcional]
    → correlacionar com estado atual de infra
    ↓
render_report (pkg/playbook)
    → relatório Markdown com findings e recomendações
```

## Playbooks disponíveis

Os playbooks vivem em `pkg/playbook/config/`. Adicionar novo playbook = criar novo YAML.

Exemplos existentes:
- `fusca-cdc-audit` — CDC audit com 6 analyzers (epoch contention, lock wait, etc.)

## Primitivas envolvidas

| Primitiva | Onde aplica |
|-----------|-------------|
| **Environment Probe** | coleta estado atual: kubectl, kafka, psql |
| **Heuristic-First** | analyzers zero-LLM executam antes de qualquer LLM call |
| **ADR Capture** | findings registrados na causal chain |

## Execução (wtb run — Phase 4)

```bash
# Quando wtb run estiver implementado:
wtb run investigation --playbook fusca-cdc-audit --env production
wtb run investigation --playbook fusca-cdc-audit --window 4h
```

## Handoff

- `ops-response` → `investigation` — ops probe identificou necessidade de análise profunda
- `incident` → `investigation` — durante ou após o incidente
- `investigation` → `postmortem` — findings viram input do postmortem

## Artefatos gerados

- `docs/workflow/investigation/NNN-<context>-YYYY-MM-DD.md` — relatório de investigação
- `.workflow/investigation/causal-chain.json` — dados estruturados da causal chain
