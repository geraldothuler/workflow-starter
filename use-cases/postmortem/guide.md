# Workflow: Postmortem

**Tipo:** postmortem
**Primitivas principais:** ADR Capture, Heuristic-First (timeline), Socratic Probe (licoes)

---

## Trigger

- Incidente encerrado com impacto relevante (producao afetada, SLA impactado, dados em risco)
- Incidente sem impacto mas com licao tecnica valiosa
- Qualquer incidente que se repete — obrigatorio

---

## Input Necessario

- Pasta de incidente: `docs/workflow/incident/NNN-<context>-YYYY-MM-DD/` com todos os savepoints
- Escopo de impacto final (quantificado: usuarios, tempo, dados)
- Pessoas envolvidas na resolucao (por papel, nao por nome se PII)

---

## Primitivas Usadas

| Primitiva | Como se aplica |
|-----------|---------------|
| **ADR Capture** | Registrar decisoes chave do incidente no formato ADR (o que foi decidido, alternativas, contexto) |
| **Heuristic-First** | Construir timeline precisa a partir dos savepoints; identificar root cause sem LLM primeiro |
| **Socratic Probe** | Extrair licoes com perguntas: "o que essa decisao revela sobre o sistema?" — 1 pergunta/vez |

---

## Artefatos Produzidos

| Artefato | Formato | Onde vai |
|----------|---------|----------|
| Postmortem doc | `NNN-<context>-YYYY-MM-DD.md` | `docs/workflow/postmortem/` |

**Secoes obrigatorias:** Timeline, Root Cause, Impact, Acoes Corretivas, Licoes, Chain.

---

## Handoff Possivel

**Para review:** quando ha dimensao de colaboracao (como time e ferramentas se saíram, o que melhorar).
- Input para review: este postmortem + savepoints de incidente
- Campo obrigatorio: link para review no final do documento (ou "review: nao gerado")

---

## Regra de Atualizacao do INDEX.md

Ao criar um novo postmortem:

```markdown
| NNN | YYYY-MM-DD | [Titulo](./NNN-context-YYYY-MM-DD.md) | Concluido | link |
```

---

## Chain Links

```
postmortem NNN
  ← incident/NNN-<context>-YYYY-MM-DD/ (origem)
  → review/NNN-<context>-YYYY-MM-DD.md (se gerado)
```
