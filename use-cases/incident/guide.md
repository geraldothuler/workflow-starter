# Workflow: Incident

**Tipo:** incident
**Primitivas principais:** Environment Probe, Heuristic-First, ADR Capture

---

## Trigger

Uma ou mais das seguintes:
- Alerta de producao ativo (Datadog, PagerDuty, Slack)
- Erro sistematico reportado por usuario/colega
- Degradacao observavel (latencia, error rate, lag crescente)
- Anomalia detectada manualmente

---

## Input Necessario

- Descricao inicial do sintoma (o que foi observado, quando, por quem)
- Ambiente afetado (producao, staging, regiao)
- Sistemas suspeitos (servicos, bancos, filas)
- Escopo de impacto estimado (usuarios, volume, SLA)

---

## Primitivas Usadas

| Primitiva | Como se aplica |
|-----------|---------------|
| **Environment Probe** | Coletar estado real: k8s pods, logs, metricas, lag Kafka, locks PG |
| **Heuristic-First** | Analisar patterns conhecidos antes de LLM (lock contention, OOM, epoch error) |
| **ADR Capture** | Registrar cada decisao relevante: quando escalar, quando reverter, quando encerrar |

---

## Artefatos Produzidos

| Artefato | Formato | Onde vai |
|----------|---------|----------|
| Savepoint inicial | `savepoint-YYYY-MM-DD.md` | `docs/workflow/incident/NNN-<context>-YYYY-MM-DD/` |
| Savepoint de retomada | `savepoint-YYYY-MM-DD-followup.md` | mesma pasta |
| Timeline de acoes | incluido no savepoint | — |

**Convencao de pasta:** `NNN-<context>-YYYY-MM-DD/` onde NNN e zero-padded e data e a data de inicio.

---

## Handoff Possivel

**Para postmortem:** quando incidente encerrado e ha licoes a extrair.
- Input para postmortem: todos os savepoints da pasta de incidente
- Campo obrigatorio no ultimo savepoint: `Status: ENCERRADO` + link para postmortem (se criado)

---

## Regra de Atualizacao do INDEX.md

Ao criar um novo incidente:

```markdown
| NNN | YYYY-MM-DD | [Titulo](./NNN-context-YYYY-MM-DD/) | Em andamento / Encerrado | link |
```

Atualizar `status` quando encerrar.

---

## Chain Links

```
incident NNN
  → postmortem/NNN-<context>-YYYY-MM-DD.md (se gerado)
  → review/NNN-<context>-YYYY-MM-DD.md (se gerado)
```

Incluir esses links na secao **Chain** do `incident/INDEX.md`.
