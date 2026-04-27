# Use Case: Discovery — Análise Técnica Estruturada

**Tipo:** documentary | **Engine:** manual + LLM-assisted

---

## Quando usar

- "Faz sentido construir X?" — antes de estimar
- "O que falta pra chegar em Y?" — gap analysis
- "Vale o ROI?" — avaliação de opções antes de comprometer
- "Precisamos documentar essa decisão" — ADR pós-conversa
- Spike técnico que precisa virar artefato rastreável

## Quando NÃO usar

- Quando a decisão já foi tomada e a execução já começou → use `review` ou `backlog`
- Quando é um incidente ativo → use `incident` + `ops-response`
- Quando é uma reunião de calibração → use `1on1`

## Estrutura do documento

```
NNN-<topic>-YYYY-MM-DD.md

---
# frontmatter (para ChainResolver)
topic: <topic>
question: <pergunta central>
status: draft | concluded
recommendation: <uma linha>
handoff: <use-case> | none
date: YYYY-MM-DD
---

## Contexto
Por que essa discovery foi iniciada.

## Diagnóstico
Estado atual — o que já existe, o que foi explorado.

## Gap / O que falta
O que está ausente em relação ao objetivo.

## Opções
Tabela ou lista de alternativas com trade-offs.

## ROI Scoring
Esforço × valor × risco por opção.

## Recomendação
Resposta direta à pergunta central.
Próximo passo concreto.

## Handoff
→ <use-case>: <o que passa adiante>
```

## Execução típica

```
1. Conversa exploratória (pergunta → análise → achados)
2. wtb new discovery <topic> --repo <path>   ← scaffolding do artefato
3. Preencher seções com os achados
4. Opcional: HTML de apresentação em docs/<topic>-discovery.html
5. Definir handoff → próximo use-case ou "terminal"
```

## Primitivas envolvidas

| Primitiva | Onde aplica |
|-----------|-------------|
| **Socratic Probe** | Clarifica a pergunta antes de analisar — "por que esse topic agora?" |
| **Options Scoring** | ROI scoring das alternativas levantadas |
| **ADR Capture** | Registra a decisão como artefato persistente e rastreável |

## Handoffs possíveis

- `discovery` → `backlog` — achados geram histórias de produto
- `discovery` → `review` — achados precisam de validação com stakeholders
- `discovery` → `incident` — discovery de protocolo vira runbook
- `discovery` → nenhum — decisão foi "não construir agora"

## Artefatos gerados

- `docs/workflow/discovery/NNN-<topic>-YYYY-MM-DD.md` — análise estruturada
- `docs/<topic>-discovery.html` — apresentação visual (opcional)
