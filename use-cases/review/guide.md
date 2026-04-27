# Workflow: Review

**Tipo:** review
**Primitivas principais:** Options Scoring (ROI), Compliance Scoring (score colaboracao), ADR Capture

---

## Trigger

- Postmortem de incidente concluido com dimensao colaborativa relevante
- Ciclo de sprint/projeto com entrega significativa
- Mudanca arquitetural ou de processo que merece retrospectiva

---

## Input Necessario

- Postmortem (se originado de incidente): `docs/workflow/postmortem/NNN-<context>-YYYY-MM-DD.md`
- Contexto da colaboracao: quem participou, como as ferramentas se saíram, onde houve friccao
- Metricas disponiveis (tempo de resolucao, score de colaboracao anterior)

---

## Primitivas Usadas

| Primitiva | Como se aplica |
|-----------|---------------|
| **Options Scoring** | Avaliar alternativas para melhorar processo (ROI, risco, viabilidade) com scores |
| **Compliance Scoring** | Score de colaboracao 0-100: o que o time/ferramenta entregou vs. o que era esperado |
| **ADR Capture** | Registrar decisoes de processo: o que mudar, o que manter, por que |

---

## Artefatos Produzidos

| Artefato | Formato | Onde vai |
|----------|---------|----------|
| Review doc | `NNN-<context>-YYYY-MM-DD.md` | `docs/workflow/review/` |

**Secoes obrigatorias:** Score de colaboracao, O que funcionou, O que pode melhorar, Decisoes de processo, Chain.

---

## Handoff Possivel

**Para 1on1:** quando ha dimensao de calibracao de parceria (especialmente com Claude Code ou ferramentas AI).
- Input para 1on1: secao de score de colaboracao e observacoes sobre friccao
- Campo obrigatorio: link para 1on1 no final (ou "1on1: nao agendado")

---

## Regra de Atualizacao do INDEX.md

```markdown
| NNN | YYYY-MM-DD | [Titulo](./NNN-context-YYYY-MM-DD.md) | Concluido | score | link |
```

---

## Chain Links

```
review NNN
  ← postmortem/NNN-<context>-YYYY-MM-DD.md (origem)
  → ~/.workflow/1on1/sessions/NNN-YYYY-MM-DD.md (se gerado)
```
