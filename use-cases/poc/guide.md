# Guia — use-case `poc`

Um PoC valida **uma hipótese específica** com critério de sucesso definido antecipadamente.
Vive entre `discovery` (exploratório) e `review`/`implementação` (comprometimento).

---

## Quando usar

| Situação | Use-case correto |
|----------|-----------------|
| Pergunta técnica aberta, sem hipótese | `discovery` |
| Hipótese clara, experimento < 8h | **`poc`** |
| Anomalia em produção, pressão de tempo | `incident` |
| Hipótese validada, pronto para construir | `review` → implementação |

---

## Estrutura do artefato

Frontmatter obrigatório:

```markdown
---
type: poc
id: NNN
hypothesis: "X resolve Y se Z"
origin: discovery/NNN
success_criteria: "critério mensurável"
time_box: "Xh"
decision: pending | success | failure | partial
---
```

Seções:

```markdown
## Hipótese
## Método
## Critério de sucesso
## Resultados
## Decisão
```

---

## Decisões e handoffs

| Decisão | Significado | Handoff |
|---------|------------|---------|
| `success` | Critério de sucesso atingido | → `review` → implementação |
| `failure` | Hipótese refutada, aprendizado registrado | → `postmortem` (se houver impacto) ou terminal |
| `partial` | Parcialmente validado, novos unknowns emergem | → `discovery/NNN+1` (bidirecional) |

---

## Convenção de nomes

```
docs/workflow/poc/
├── INDEX.md
└── NNN-<context>-YYYY-MM-DD.md
```

- `NNN` zero-padded (001, 002, ...)
- `<context>` é o tema, não o tipo (ex: `audience-tags`, `schedule-map`)
- Registrar em `docs/workflow/poc/INDEX.md` ao criar

---

## Checklist de qualidade

Antes de registrar a decisão:

- [ ] A hipótese está formulada como "X resolve Y se Z"?
- [ ] O critério de sucesso era mensurável antes de executar?
- [ ] O time-box foi respeitado (ou há registro de por que foi extrapolado)?
- [ ] A decisão inclui handoff explícito?
- [ ] Se `partial`: o novo discovery está criado ou na fila?

---

## Casos de uso imediatos

| ID | Contexto | Origem | Time-box | Critério |
|----|---------|--------|---------|---------|
| 001 | audience-tags | discovery/009 | 2h | filtrar "estratégico+longo prazo" retorna 3 docs corretos |
| 002 | schedule-map | discovery/008 | 3h | `wtb ops airbyte --mode schedule-map` lista 21 consumers com cron |
| 003 | warehouse-cost | discovery/008+007 | 2h | `wtb ops snowflake` retorna AUTO_SUSPEND + créditos/dia |
