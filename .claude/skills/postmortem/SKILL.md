---
name: postmortem
description: >
  Gera postmortem no Notion seguindo o formato Cobli. Dado um incidente (descrição livre
  ou ID do docs.db), busca evidências automaticamente (docs.db, Slack, Datadog), faz
  perguntas socráticas 1-a-1 para preencher lacunas e cria página privada no Notion.
  MVP: página privada no workspace pessoal. Próxima iteração: banco de dados compartilhado.
argument-hint: "<descrição do incidente | doc-id>"
user-invocable: true
---

# postmortem — Gerador de Postmortem Cobli

> ⚠️ **Escopo MVP:** cria página **privada** no workspace Notion (sem parent = workspace-level).
> **NÃO** escrever na coleção compartilhada `collection://b23ad769-4280-4845-b1c7-d2ec1fb2d502`.
> Isso muda só quando o usuário explicitamente aprovar a migração para produção.

---

## Fase 0 — Reconhecimento automático (sem perguntar ao usuário)

Parse `$ARGUMENTS` como descrição do incidente. Execute em paralelo:

```bash
# 1. Busca primária no docs.db
cd ~/workflow && wtb doc search "$ARGUMENTS" 2>/dev/null | head -20

# 2. Sinônimos se busca vazia
cd ~/workflow && wtb doc list --since $(date -v-7d +%Y-%m-%d) 2>/dev/null | head -20

# 3. Git log — repos potencialmente afetados (inferir do contexto)
# Ex: se args menciona herbie-dashboard → cd ~/Cobliteam/herbie-dashboard
git log --oneline --since="7 days ago" 2>/dev/null \
  | grep -i "fix\|hotfix\|revert\|rollback\|incident" | head -15
```

Se encontrou doc relevante, carregue com `wtb doc get <id>` e extraia:
- Datas/horários já documentados
- Causa raiz
- Timeline existente
- Repos/serviços envolvidos
- Autores mencionados

Registre o que encontrou e o que ainda está em branco antes de avançar.

---

## Fase 1 — Campos mínimos (1 pergunta por vez)

Só pergunte o que não foi encontrado na Fase 0.
Quando encontrar algo, confirme antes de usar: *"Encontrei X no docs.db — confirma?"*

### M1 — Título
Formato Cobli: `Postmortem - <descrição concisa>`
Sugira com base no contexto. Ex: `Postmortem - Tela de Motoristas — crash e exibição incorreta`

### M2 — Início do Impacto
Datetime em que o problema começou a afetar usuários.
Dica: geralmente é o horário do deploy problemático ou primeira ocorrência.
**Formato:** `DD/MM/YYYY HH:MM`

### M3 — Data da detecção
Quando a equipe soube do problema.
MTTD será calculado automaticamente: M3 − M2.
**Pergunta:** *"Quando foi detectado? (data + hora se souber)"*

### M4 — Data da resolução
Quando o serviço foi restaurado / fix entrou em produção.
**Pergunta:** *"Quando foi resolvido?"*

### M5 — Sumário
Texto curto que qualquer pessoa não-técnica entenda: o que houve, gravidade, solução.
Sugira baseado no contexto. Exemplo de tom:
> "Uma regressão no deploy do PR #10830 causou crash e exibição incorreta na tela de
> Motoristas por ~20h para frotas com tokens RFID externos."

### M6 — Causa raiz
A falha fundamental que permitiu o incidente ocorrer.
Sugira se encontrou evidência. Diferente do gatilho: é o *porquê estrutural*, não o evento.

---

## Fase 2 — Reconstrução da timeline (proativa, antes das seções restantes)

Tente reconstruir automaticamente antes de perguntar.

### 2.1 — docs.db
Se o doc de incidente carregado na Fase 0 tem seção de timeline, use-a.

### 2.2 — Slack
```
Use: mcp__claude_ai_Slack__slack_search_public_and_private
query: "<nome do serviço/incidente> <data do incidente>"
```
Extraia eventos com horário de threads relevantes.

### 2.3 — Datadog
```
Use: mcp__datadog__search_datadog_events
filtro: serviço afetado + janela temporal M2→M4
```

### 2.4 — Draft da timeline
Monte tabela com o que encontrou e apresente:

*"Montei esta timeline a partir de [fontes]. Confirma ou complementa?"*

| Data / Hora | Descrição |
|-------------|-----------|
| HH:MM | Evento X |

Aceite complementos em formato livre e incorpore.

---

## Fase 3 — Seções restantes (1 pergunta por vez, sugere + confirma)

### R1 — Gatilho
O evento específico que *acionou* o incidente (deploy, pico de tráfego, mudança de config).
Diferente da causa raiz: é o gatilho imediato, não o problema estrutural.

### R2 — Resolução
Passos detalhados tomados para mitigar e restaurar.
Tente extrair de: PR de fix (descrição), git log, docs.db.

### R3 — Detecção
Como a equipe soube? (alerta Datadog, reclamação via suporte, monitoramento manual…)
MTTD já calculado com M2 e M3 — inclua no campo.

### R4 — Impacto
Sob perspectiva do cliente e do negócio. Use métricas se disponíveis.
Ex: "X frotas afetadas, Y usuários não conseguiram acessar a tela por Z horas."

### R5 — Autores
Infira do contexto:
```bash
# Commits no período do incidente
git log --oneline --after="<M2>" --before="<M4 + 1h>" 2>/dev/null | head -20

# Autor do PR de fix (se identificado na Fase 0)
gh pr view <PR#> --json author,reviews 2>/dev/null \
  | python3 -c "import json,sys; d=json.load(sys.stdin); \
    print('Autor:', d['author']['login']); \
    [print('Reviewer:', r['author']['login']) for r in d.get('reviews',[])]"
```
Apresente: *"Autores inferidos: [lista]. Confirma ou adiciona alguém?"*

### R6 — Times Impactados
Infira do repo/serviço:
- `herbie-dashboard`, `blueprint-api`, `trigger-action-api`, `trigger-engine` → `GF-Core`
- `fusca`, `iris`, `cerberus`, `alexstrasza` → consulte squad em `memory/reference_devs_monitoring_squad.md`
- Dúvida → pergunte

Confirme antes de usar.

### R7 — Lições: O que deu certo
Sugira com base no que funcionou durante a resolução.
Ex: "Rollback foi rápido porque havia PR atômico com contexto claro."

### R8 — Lições: O que deu errado
Sugira com base nas causas identificadas.
Ex: "Fields deprecated não foram sinalizados no code review do PR #10830."

### R9 — Lições: No que demos sorte
Sugira se houver evidência de sorte não planejada.
Ex: "Bug afetava apenas frotas com RFID externo (isExternal=true) — impacto limitado."

### R10 — Pontos de ação
*"Quais são os pontos de ação corretivos/preventivos?"*
Aceite lista livre. Formatará como checkboxes no Notion.

---

## Fase 4 — Criação da página privada no Notion

> ⚠️ **parent omitido** = página privada no workspace do usuário. Não passar `data_source_id`.

**4.1** Faça fetch do enhanced markdown spec antes de montar o conteúdo:
```
Use: ReadMcpResourceTool com URI notion://docs/enhanced-markdown-spec
```

**4.2** Monte o conteúdo seguindo exatamente o template Cobli:

```markdown
# Autores
- [R5 — um por linha]

- [ ] ⬆️Preencher no topo do documento os **horários** e **datas** de introdução do problema,
detecção do problema, início do incidente e resolução do problema.

## Status
Em desenvolvimento.

## Sumário
[M5]

## Impacto
[R4]

## Causa raiz
[M6]

## Gatilho
[R1]

## Resolução
[R2]

## Detecção
- **Meio de detecção:** [R3]
- **Tempo para detecção (MTTD):** [M3 − M2, em horas/minutos]

## Pontos de ação
[R10 — um checkbox por linha: "- [ ] Ação — Responsável"]

## Lições aprendidas

### O que deu certo
[R7]

### O que deu errado
[R8]

### No que demos sorte
[R9]

## Linha do tempo
[tabela da Fase 2]

## Informações de apoio
[links relevantes: PR de fix, docs.db, Datadog dashboard, Slack thread]
```

**4.3** Criar com `mcp__claude_ai_Notion__notion-create-pages`:
```json
{
  "pages": [{
    "icon": "👻",
    "properties": { "title": "<M1>" },
    "content": "<conteúdo montado acima>"
  }]
}
```
**Não incluir `parent`** — página privada.

---

## Fase 5 — Persistir referência no docs.db

```bash
wtb doc add --type reference \
  --title "<M1>" \
  --date "<data de M2 no formato YYYY-MM-DD>" \
  --content "Postmortem Notion (privado MVP)
Título: <M1>
Início do Impacto: <M2>
Data da resolução: <M4>
MTTD: <calculado>
Notion URL: <URL retornada pela criação>

Para migrar para o banco de produção, usar:
  data_source_id: b23ad769-4280-4845-b1c7-d2ec1fb2d502"
```

**Retorne ao usuário:**
1. Link da página Notion criada
2. Confirmação de referência salva no docs.db

---

## Iteração futura — migrar para banco de produção

Quando o usuário aprovar, trocar o `parent` para:
```json
{
  "parent": {
    "type": "data_source_id",
    "data_source_id": "b23ad769-4280-4845-b1c7-d2ec1fb2d502"
  }
}
```

Propriedades adicionais do banco a preencher:
| Propriedade Notion | Valor |
|--------------------|-------|
| `Name` | M1 |
| `Início do Impacto` | M2 (datetime ISO-8601) |
| `Início do tratamento` | M3 (datetime ISO-8601) |
| `Data da detecção` | M3 (date) |
| `Data da resolução` | M4 (date) |
| `Data de criação do documento` | hoje |
| `Times Impactados` | R6 (JSON array, ex: `["GF-Core"]`) |
