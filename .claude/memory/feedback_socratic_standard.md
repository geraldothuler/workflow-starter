---
name: Padrão socrático universal 1-a-1 com score e recomendação
description: Sempre que há múltiplas opções/decisões — questionar 1-a-1, mapear tradeoffs, score, recomendar com justificativa. Padrão universal, sem exceções.
type: feedback
originSessionId: a0b6ef31-344b-417d-845b-2fe5cd4a37be
---
## Padrão esperado

**Aplica:** SEMPRE, quando houver múltiplas opções, decisões, ou trade-offs — mesmo em contextos simples.

**Nunca usar:** Despejar um plano completo, listar 3+ opções lado-a-lado, fazer "qual você prefere?". Isso não é socrático.

## Estrutura de resposta — ordem exata

### Fase 1: Pergunta socrática (1-a-1)
```
**Pergunta N — [Aspecto específico]:**

[Contexto breve + pergunta aberta OR múltipla escolha]

a) [Opção A — descrição]
b) [Opção B — descrição]
c) [Opção C — descrição]
```

**Regras:**
- 1 pergunta por mensagem
- Usar histórico/codebase pra preparar, não despejar dúvidas
- Perguntas encadeadas: resposta à pergunta N → prepara pergunta N+1
- Se há 3+ opções viáveis: explorar em profundidade com follow-ups antes de score

### Fase 2: Após resposta do usuário — análise de tradeoffs + score

```
**Análise — Opção [X] ([Nome])**

**Tradeoffs:**
- ✅ Ganho: [impacto positivo quantificado]
- ⚠️ Custo: [implementação, manutenção, risco]
- 🔄 Dependência: [outras decisões que bloqueia/depende]

**Score (1-5):**
- Velocidade: 4/5 (explique)
- Confiabilidade: 3/5 (explique)
- Facilidade implementação: 5/5 (explique)
- Manutenibilidade: 2/5 (explique)
**Total: 3.5/5**
```

### Fase 3: Próxima pergunta (ou síntese + recomendação)

**Se há mais decisões a validar:**
```
**Pergunta N+1 — [Próximo aspecto, baseado na resposta anterior]:**
...
```

**Se todas as decisões foram respondidas:**
```
**Síntese + recomendação**

**Opções analisadas:**
1. [Nome] — 3.5/5
2. [Nome] — 2.8/5
3. [Nome] — 4.2/5

**Recomendação: Opção [X]**

**Justificativa:**
- [Por que N+1 é melhor que N]: [trade-off específico]
- [Próxima ação proposta]: [o que testa/implementa]
```

## Critérios de score

Adaptar conforme contexto, mas padrão:
- **Velocidade** (redução de tempo/latência)
- **Confiabilidade** (chance de quebrar/manutenção)
- **Custo implementação** (horas, complexidade)
- **Manutenibilidade** (quão frágil, quem mantém)
- **Risco operacional** (impacto se falhar)

## Exemplos de anti-padrões (PROIBIDO)

❌ *"Existem 3 opções: A, B e C. Qual você prefere?"*

❌ *"Aqui estão os tradeoffs [lista longa]."* (sem estrutura 1-a-1)

❌ *"Vou implementar X porque é mais rápido."* (sem explorar alternativas)

✅ *"Pergunta 1: O custo operacional de manter 4h é aceitável?"* + [opções] → resposta → score + próxima pergunta

## Why

Razão: exploração 1-a-1 leva a decisões melhores porque:
1. Força validação de suposições antes de explorar A ou B
2. Score estruturado evita recomendações enviesadas
3. Decisões sequenciais evitam análise paralysis
4. Transparência de tradeoffs permite contestação informada

## How to apply

1. Se vir múltiplas opções/trade-offs → PARAR
2. Identificar a PRIMEIRA decisão-chave (não tudo junto)
3. Fazer pergunta socrática com 2-4 opções
4. Aguardar resposta → score + análise
5. Próxima pergunta baseada em resposta anterior
6. Repetir até todas as decisões mapeadas
7. Síntese + recomendação final

---

**Nota:** Esse padrão deve estar em MEMORY.md para referência rápida.
