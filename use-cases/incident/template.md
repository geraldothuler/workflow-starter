# Savepoint — [Titulo do Incidente] (YYYY-MM-DD)

**Salvo em:** YYYY-MM-DD HH:MM TZ
**Status:** Em andamento / ENCERRADO
**Severidade:** Critical / High / Medium
**Servicos afetados:** [lista]

---

## Contexto

<!-- O que foi observado, quando, por quem, como chegou ate voce -->

---

## Estado atual do ambiente

<!-- Environment Probe: output dos comandos relevantes -->

```
# k8s
kubectl get pods -n <ns>

# Postgres
psql -c "SELECT ..."

# Kafka
kafka-consumer-groups.sh ...
```

**Findings:**
- [item 1]
- [item 2]

---

## Hipoteses

<!-- Heuristic-First: patterns conhecidos identificados -->

| # | Hipotese | Evidencia | Probabilidade |
|---|---------|-----------|---------------|
| 1 | | | |

---

## Acoes executadas

<!-- ADR Capture: cada decisao com raciocinio -->

| HH:MM | Acao | Raciocinio | Resultado |
|-------|------|-----------|-----------|
| | | | |

---

## Decisoes registradas

<!-- ADR-NNN para decisoes significativas -->

### ADR-001: [Titulo]
- **Decisao:**
- **Contexto:**
- **Alternativas descartadas:**
- **Consequencias:**

---

## Para retomar

<!-- O que precisa saber ao voltar para este incidente -->

---

## Handoff

**Status:** Em andamento / ENCERRADO

Se encerrado:
- **Postmortem:** [link] / nao gerado
- **Review:** [link] / nao gerado
- **Proximos passos:** [lista]

---

*Convencao: `savepoint-YYYY-MM-DD.md` | Pasta: `NNN-<context>-YYYY-MM-DD/`*
