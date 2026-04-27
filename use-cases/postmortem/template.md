# Postmortem: [Titulo do Incidente]

**Data do incidente:** YYYY-MM-DD
**Data do postmortem:** YYYY-MM-DD
**Severidade:** Critical / High / Medium
**Servicos afetados:** [lista]
**Duracao total:** Xh Ymin (YYYY-MM-DD HH:MM → YYYY-MM-DD HH:MM TZ)

---

## Resumo executivo

<!-- 2-3 frases: o que aconteceu, impacto, como foi resolvido -->

---

## Timeline

<!-- Heuristic-First: construida a partir dos savepoints — nao inferida -->

| Horario | Evento | Quem/Sistema |
|---------|--------|-------------|
| HH:MM | | |

---

## Root Cause

<!-- Causa raiz com evidencias — nao hipoteses -->

**Causa imediata:**

**Causa sistêmica:**

**Evidencias:**
- [link para savepoint ou output especifico]

---

## Impacto

| Dimensao | Valor |
|----------|-------|
| Usuarios afetados | |
| Duracao da degradacao | |
| Dados comprometidos | nenhum / [descricao] |
| SLA impactado | sim / nao |

---

## Acoes corretivas

| # | Acao | Tipo | Status | Owner |
|---|------|------|--------|-------|
| 1 | | Preventiva / Detectiva / Corretiva | Pendente / Em andamento / Concluida | |

---

## Decisoes chave (ADR)

<!-- ADR Capture para decisoes significativas durante o incidente -->

### ADR-001: [Titulo]
- **Status:** Accepted
- **Contexto:** [o que motivou]
- **Decisao:** [o que foi decidido]
- **Consequencias:** [positivas e negativas]
- **Alternativas consideradas:** [opcoes descartadas + motivo]

---

## Licoes aprendidas

<!-- Socratic Probe: o que cada decisao revela sobre o sistema e o processo -->

### O que funcionou bem
- [item]

### O que pode melhorar
- [item]

### Perguntas que ficam abertas
- [pergunta que merece acompanhamento]

---

## Chain

**Origem:** [incident/NNN-<context>-YYYY-MM-DD/](../incident/NNN-context-YYYY-MM-DD/)
**Review:** [review/NNN-<context>-YYYY-MM-DD.md](../review/NNN-context-YYYY-MM-DD.md) / nao gerado
**1on1:** nao aplicavel a este postmortem

---

*Convencao: `NNN-<context>-YYYY-MM-DD.md` | Pasta: `postmortem/`*
