---
name: Exemplo prático — Padrão socrático 1-a-1 com score e recomendação
description: Exemplo real de como aplicar o padrão universal. Não é memória — é template de referência.
type: reference
originSessionId: a0b6ef31-344b-417d-845b-2fe5cd4a37be
---
# Exemplo: Análise de alternativas para otimização de crawler

## ❌ ANTI-PADRÃO (proibido)

```
Existem 3 formas de resolver isso:

1. Paralelismo (aiohttp 5 workers)
   - Velocidade: 50-80% redução
   - Confiabilidade: média (throttling)
   - Custo: baixo
   
2. Caching incremental
   - Velocidade: 90% redução
   - Confiabilidade: alta
   - Custo: médio
   
3. API reversa
   - Velocidade: 95% redução
   - Confiabilidade: média (frágil a mudanças)
   - Custo: médio-alto

Qual você prefere?
```

**Por que está errado:**
- Despeja 3 opções de uma vez
- Não é socrático (não questiona suposições)
- Não há score estruturado
- "Qual você prefere?" força decisão sem contexto

---

## ✅ PADRÃO CORRETO (obrigatório)

### Pergunta 1 — Impacto operacional

**Pergunta 1 — Custo real de 4 horas:**

O crawler leva 4 horas hoje. Como ele é usado?

a) **Manual** — roda quando pedido (Pedro Lion, ~1x/trimestre)
b) **Automação** — roda mensalmente via CI/CD
c) **Ad-hoc** — quando há bug report de modelo não encontrado
d) **Desconhecido** — preciso investigar

---

### Resposta + Análise (Pergunta 1)

[Usuário responde: "Manual"]

**Análise — Opção A (Manual)**

**Tradeoffs:**
- ✅ Ganho: nenhuma pressão para reduzir (problema cosmético)
- ✅ Ganho: permite explorar API reversa sem urgência (há tempo)
- ⚠️ Custo: bloqueio humano (alguém tem que rodar 4h)
- ⚠️ Custo: sem cobertura automática (modelos 2026 demoraram descobrir)
- 🔄 Dependência: próxima decisão = "por quanto tempo é aceitável esperar?"

**Score (se mantiver manual):**
- Velocidade necessária: 1/5 (baixa)
- Impacto de falha: 2/5 (clientes contornam com placa)
- ROI de refactor: 1/5 (ganho é conforto, não negócio)
**Total: 1.3/5** ← indica: refactor não é urgente, explore conforme tempo livre

---

### Pergunta 2 — Frequência mínima aceitável

**Pergunta 2 — Com execução manual, qual é frequência aceitável?**

a) **1x/ano** — basta atualizar versões novas de modelos uma vez por ano
b) **1x/trimestre** — mantém atualizado com ciclos reais de lançamento
c) **1x/mês** — cobertura semanal é nice-to-have mas impraticável com 4h
d) **Sem restrição** — quanto mais frequente melhor

---

### Resposta + Análise (Pergunta 2)

[Usuário responde: "1x/trimestre"]

**Análise — Opção B (Trimestral)**

**Tradeoffs:**
- ✅ Ganho: captura ciclos reais de lançamento automotivo (Jan/Set = principais)
- ✅ Ganho: modelo 2026 seria descoberto em ~3 meses (aceitável)
- ⚠️ Custo: ainda demanda bloquei humano (1 dia a cada trimestre)
- ⚠️ Custo: não viável automação mensal (4h × 12 = 48h/ano, muito)
- 🔄 Dependência: próxima decisão = "qual é o mínimo gain pra justificar refactor?"

**Score (se trimestral manual):**
- Alinhamento com ciclo automotivo: 4/5 (coincide com fases reais)
- Facilidade de manutenção: 4/5 (não requer automação)
- Impacto no negócio: 2/5 (bug report → descobrem 2 semanas depois)
**Total: 3.3/5** ← indica: aceitável, mas há espaço para melhoria sem urgência

---

### Pergunta 3 — Qual redução de tempo justifica refactor?

**Pergunta 3 — Assumindo trimestral, qual redução de 4h justificaria refactor?**

a) **30-50min** (paralelismo) — poupa ~3.5h, permitiria automação (CI/CD)
b) **15-30min** (API reversa) — poupa ~3.75h, idem
c) **Qualquer redução < 1h** — ainda impraticável para automação
d) **Nenhuma** — manual 4h trimestral é aceitável indefinidamente

---

### Resposta + Análise (Pergunta 3)

[Usuário responde: "30-50min viabilizaria automação CI/CD"]

**Análise — Limiar de refactor identificado**

**Critério estabelecido:**
- Se conseguir ≤ 50min → viável automação (reduz a load manual)
- Se ficar em 2-4h → manual continua aceitável
- ROI: apenas se paralelismo (5-8 workers) não disparar throttling

---

### Pergunta 4 — Explorar alternativas

**Pergunta 4 — Antes de implementar paralelismo, qual fonte explorar primeiro?**

a) **Engenharia reversa (DevTools)** — 30min investigação, pode revelar API JSON
b) **Dataset público (FIPE/ABVE)** — 30min busca, pode substituir scraping completamente
c) **Contato direto (Molicar)** — email, aguardar resposta (1-2 semanas)
d) **Paralelismo local** — começar pela implementação (6-8h)

---

### Resposta + Síntese final

[Usuário responde: "Explorar A (DevTools) + B (públicos) em paralelo"]

**Síntese + Recomendação**

**Decisões validadas:**
1. **Execução manual trimestral é aceitável** — score 3.3/5
2. **Refactor justificado apenas se ≤ 50min** — criterion definido
3. **Paralelismo é riscado** — throttling confirmado como bloqueador

**Recomendação: Testar 2 hipóteses em paralelo (esta semana)**

| Hipótese | Ação | Ganho esperado | Risco | Prazo |
|----------|------|---------------|-------|-------|
| **A — API reversa** | DevTools Network inspection | 95% (15-30min) | Médio (frágil a mudanças) | 30min |
| **B — Dataset público** | Buscar FIPE/ABVE/Detran | 100% (substitui scraping) | Alto (pode não existir) | 1h |

**Por que essa ordem:**
- DevTools é rápido (30min) e tem alto ganho (se API existir)
- Se API não existir → FIPE/ABVE pode estar disponível (100% substituição)
- Se ambos falharem → paralelismo local (6-8h) é fallback aceitável
- Nenhuma pressão de urgência (trimestral é viável indefinidamente)

**Próximo passo:** Rodar DevTools + busca FIPE em paralelo, reportar achados em 1h.

---

## Estrutura de resposta — checklist

- [ ] 1 pergunta por mensagem (nunca 2+)
- [ ] Após resposta: análise de tradeoffs
- [ ] Score estruturado (1-5, com justificativa)
- [ ] Próxima pergunta ou síntese
- [ ] Recomendação com justificativa explícita
- [ ] Nunca "qual você prefere?" sem contexto
