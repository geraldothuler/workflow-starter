---
name: estimate
description: >
  Estimativa calibrada de feature/epic usando dados históricos do backlog.db e docs.db.
  Produz breakdown por fase, riscos identificados e racional apresentável para stakeholders.
  Usar antes de qualquer planejamento de sprint ou quando produto pedir sizing.
argument-hint: "<descrição da feature> [--repos repo1,repo2]"
user-invocable: true
---

# estimate — Estimativa Calibrada por Dados Históricos

Produz estimativa técnica de uma feature usando contexto real do histórico do time,
não feeling. Segue o mesmo processo que o Matheus Reyes fez manualmente para a V1
de alertas: dados → estimate → gordura → apresentação para stakeholders.

## Quando usar

- `/estimate "implementar X"` — estimate completo com contexto histórico
- `/estimate "implementar X" --repos trigger-action-api,herbie-dashboard` — com repos específicos para análise de scope

---

## Passo 1 — Coletar contexto histórico

Execute em paralelo:

```bash
# Histórico completo de tasks (incluindo done/blocked)
wtb backlog list --status all

# Postmortems e incidentes recentes (últimos 6 meses)
wtb doc list --type postmortem --since $(date -v-6m +%Y-%m-%d) 2>/dev/null || \
wtb doc list --type postmortem --since $(date -d '6 months ago' +%Y-%m-%d 2>/dev/null || echo "2025-10-01")

wtb doc list --type incident --since $(date -v-6m +%Y-%m-%d) 2>/dev/null || \
wtb doc list --type incident --since $(date -d '6 months ago' +%Y-%m-%d 2>/dev/null || echo "2025-10-01")

# Valores calibrados do time
wtb memory list
```

Registre mentalmente:
- Quantas tasks viraram bloqueio ou incidente
- Quais repos/áreas geraram mais surpresas
- Cadência real de entrega (tasks done por semana)

---

## Passo 2 — Analisar scope da feature

Se `--repos` for passado, leia os repos indicados para:
- Identificar dependências e acoplamentos relevantes para a feature
- Estimar número de arquivos que precisarão mudar
- Detectar dívidas técnicas na área que podem explodir durante o desenvolvimento

Se não houver repos, derive o scope da descrição.

---

## Passo 3 — Produzir estimativa

Com os dados coletados, gere o output no formato abaixo. Use dias úteis.
Seja pessimista: o histórico do time (postmortems, blockers) é evidência real.

---

## Formato de saída

```markdown
# Estimativa — <nome da feature>

## Resumo executivo
**Estimativa base:** X dias úteis  
**Com gordura (bugs, blockers, incidentes inesperados):** Y dias úteis  
**Alocação recomendada:** N devs  
**Confiança:** alta / média / baixa

---

## Breakdown por fase

| Fase | Escopo | Dias úteis |
|------|--------|-----------|
| Backend | <o que muda> | X |
| Frontend | <o que muda> | X |
| Testes & review | unit, int, CodeRabbit, QA manual | X |
| Integração & deploy | PRs, CI, smoke test em Polentas | X |
| **Base total** | | **X** |
| **Gordura (Y%)** | bugs surpresa, blockers, aprendizado | +X |
| **Total recomendado** | | **X** |

---

## Riscos identificados

Liste riscos concretos derivados do histórico, não genéricos.
Exemplo: "Esta área gerou 3 bloqueios nos últimos 6 meses (ver backlog). Probabilidade alta de surpresa."

| Risco | Probabilidade | Impacto | Mitigação |
|-------|--------------|---------|-----------|
| <risco específico> | alta/média/baixa | +X dias | <ação concreta> |

---

## Calibração histórica

Explique brevemente como chegou nos números:
- Cadência observada no backlog: X tasks/semana
- Incidentes na área nos últimos 6 meses: N
- Dívidas técnicas identificadas que podem impactar: lista
- Fator de gordura aplicado: Y% (justificar com base no histórico)

---

## Próximos passos recomendados

1. Validar scope com tech lead antes de commitar a estimativa
2. Mapear tasks no backlog: `wtb backlog add --title "..." --repo <repo>`
3. Identificar dependencies que precisam de alinhamento externo
```

---

## Regras de calibração

- **Gordura mínima:** 20% para features em área sem incidentes recentes
- **Gordura recomendada:** 30–40% para áreas com bloqueios ou incidentes no histórico
- **Gordura alta:** 50%+ se a feature toca área com dívida técnica conhecida ou novo repo
- **Nunca arredondar para baixo** — o histórico do backlog é evidência, não pessimismo
- Se o histórico mostrar que tarefas similares atrasaram, diga explicitamente

---

## Ao final

Pergunte: "Quer que eu já crie as tasks no backlog.db com `wtb backlog add`?"
Se sim, crie uma task por fase do breakdown, com `--repo` e `--tags` apropriados.
