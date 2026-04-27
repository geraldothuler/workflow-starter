---
name: ai-radar
description: Monitora canais Slack de AI/ML e engenharia da Cobli, avalia descobertas e propõe incorporação ao ecossistema workflow. Ativar quando o usuário pedir "novidades de AI", "o que tem nos canais de AI", "avalia as descobertas", ou em revisões periódicas de sessão.
user-invocable: true
---

# AI Radar — Monitor de Descobertas AI

Lê os canais Slack de AI/ML e engenharia da Cobli, filtra o que é acionável, avalia relevância para o ecossistema workflow e propõe com justificativa. O usuário sempre decide.

## Setup (one-time)

**Canais monitorados:**
- `#sig-ai` (privado) — ferramentas e descobertas AI/ML
- `#sig-agentic-coding` (privado) — agentic coding, Claude Code, padrões
- `#engineering` (público) — discussões de engenharia com relevância cross-team
- `#colib-design-system` (privado) — componentes, padrões de design, design system

**Passo 1 — Adicionar scopes ao bot Slack** (`api.slack.com/apps`):
- `groups:read` — listar canais privados dos quais o bot é membro
- `groups:history` — ler histórico de canais privados

**Passo 2 — Convidar bot aos canais privados:**
```
/invite @workflow_toolbox   (em #sig-ai e #sig-agentic-coding)
```

**Passo 3 — Salvar no Keychain:**
```bash
security add-generic-password -U -s "workflow-slack-channel-sig-ai" -a geraldothuler -w "<ID>"
security add-generic-password -U -s "workflow-slack-channel-sig-agentic" -a geraldothuler -w "<ID>"
security add-generic-password -U -s "workflow-slack-channel-engineering" -a geraldothuler -w "<ID>"
security add-generic-password -U -s "workflow-slack-channel-colib-design-system" -a geraldothuler -w "<ID>"
```

IDs atuais: sig-ai=`C072CR23K32`, sig-agentic=`C08MVTM0RDM`, engineering=`C03AVHBFGBG`, colib-design-system=`C070ERVN2TU`

## Fluxo de execução

### 0. Carregar histórico de decisões (OBRIGATÓRIO — antes de qualquer avaliação)

```bash
# 1. Índice rápido — todas as ferramentas com status e condição de ativação
wtb doc get ai-radar-index

# 2. Detalhes de ferramenta específica sob demanda
wtb doc get ai-radar-<slug>   # ex: ai-radar-grill-me, ai-radar-litellm

# 3. Listar todas por status (tag filtering)
wtb doc list --type reference --tag "ai-radar,installed"
wtb doc list --type reference --tag "ai-radar,monitored"
wtb doc list --type reference --tag "ai-radar,rejected"
```

**Regras de uso do histórico:**
- Status `installed` → reportar apenas **novidades/atualizações** se a ferramenta aparecer de novo. Não recomendar adoção.
- Status `monitored` → verificar se a **condição de ativação** foi atingida. Se sim: elevar para ADOTAR. Se não: omitir do relatório ou uma linha no rodapé.
- Status `rejected` → **ignorar completamente** — não re-avaliar, não mencionar.
- **Não presente** → avaliar normalmente.

---

### 1. Leitura dos canais (últimos 7 dias por padrão)

```bash
TOKEN=$(security find-generic-password -s "workflow-slack-token" -w)
CH1=$(security find-generic-password -s "workflow-slack-channel-sig-ai" -w)
CH2=$(security find-generic-password -s "workflow-slack-channel-sig-agentic" -w)
CH3=$(security find-generic-password -s "workflow-slack-channel-engineering" -w)
CH4=$(security find-generic-password -s "workflow-slack-channel-colib-design-system" -w)
SINCE=$(python3 -c "import time; print(int(time.time() - 7*86400))")

for CH in $CH1 $CH2 $CH3 $CH4; do
  curl -sf "https://slack.com/api/conversations.history?channel=$CH&oldest=$SINCE&limit=100" \
    -H "Authorization: Bearer $TOKEN"
done
```

### 2. Filtro de relevância

**#sig-ai e #sig-agentic-coding** — ignorar automaticamente:
- Anúncios de versão de modelos sem integração clara (ex: "GPT-5 lançado")
- Tutoriais genéricos sem link para ferramenta/lib específica
- Notícias de mercado (valuation, acquisitions)
- Discussões internas sem artefato concreto

**#engineering** — ignorar automaticamente:
- Avisos operacionais pontuais (deploys, incidentes já resolvidos)
- Discussões de processo sem impacto técnico
- Threads de onboarding / organização de equipe

Surfacar para avaliação (todos os canais):
- **MCP servers** novos com função relevante para ops/dev
- **Padrões de context management** (MEMORY, RAG, chunking, tool use)
- **Ferramentas de code review / CI automation** AI-driven
- **Agent frameworks** (Mastra, LangGraph, CrewAI, AutoGen, etc.)
- **Prompting patterns** estruturados com ganho demonstrado
- **Integrações CLI/IDE** que reduzem atrito
- **Decisões arquiteturais de times** que impactam o ecossistema workflow (ex: nova stack, mudança de provider, novo padrão de API)

### 3. Avaliação de cada item

Para cada item relevante, perguntar:

1. **Substitui algo que fazemos com subprocess/CLI hoje?** → alto valor
2. **Melhora context.json / MEMORY.md / topic files?** → avaliar
3. **Reduziria código Go no wtb?** → avaliar (YAML-first gate)
4. **É um MCP server que cobre fonte que ainda usamos via curl?** → alta prioridade
5. **Adiciona observabilidade ou reduz drift?** → avaliar
6. **Requer infraestrutura nova ou é drop-in?** → impacta recomendação
7. **Vindo do #engineering: afeta arquitetura de serviços que o workflow monitora?** → atualizar heurísticas/topic files se sim

### 4. Formato de output

Para cada descoberta relevante:

```
## [Nome da ferramenta / padrão]

**Fonte:** #canal | @autor | data
**Link:** [URL se disponível]

**O que faz:**
[1-3 linhas, sem marketing — o que concretamente resolve]

**Integração potencial:**
[Onde se encaixaria: substituiria X, complementaria Y, adicionaria Z]

**Tradeoffs:**
+ [ganho concreto e mensurável]
+ [...]
- [custo: setup, manutenção, lock-in, risco]
- [...]

**Recomendação:** ADOTAR | POC (30min) | MONITORAR | IGNORAR

**Condição de ativação:** [apenas para MONITORAR]
[Gatilho concreto que tornaria adoção relevante — ex: "quando o primeiro serviço Cobli fizer chamada LLM direta", "quando worktrees acumuladas virarem gargalo recorrente"]

**Por quê:**
[1-2 linhas de justificativa — não repetir os tradeoffs, ir direto ao ponto de decisão]
```

Ao final do relatório, incluir:
- **Sem recomendação desta rodada:** lista de itens lidos mas descartados (nome + motivo em 5 palavras)
- **Próxima verificação sugerida:** data (padrão: +7 dias)

### 5. Persistência de decisões

**Storage:** `docs.db` via `wtb doc`. Dois níveis:
- **Índice** (`ai-radar-index`) — carrega rápido, tem tabela de status + slug de cada ferramenta
- **Por ferramenta** (`ai-radar-<slug>`) — detalhes completos, carregado sob demanda

```bash
# Adicionar nova ferramenta
cat > /tmp/ai-radar-<slug>.md << 'EOF'
# AI Radar — <nome>
**Status:** installed | monitored | rejected
**Versão:** <versão>
**Fonte:** #canal | YYYY-MM-DD
**Link:** <url>
## Histórico
- YYYY-MM-DD: <evento>
EOF
wtb doc add --type reference --title "ai-radar-<slug>" \
  --tag "ai-radar,<status>" --date YYYY-MM-DD --file /tmp/ai-radar-<slug>.md

# Atualizar status (ex: monitored → installed)
wtb doc delete ai-radar-<slug>
# editar /tmp/ai-radar-<slug>.md com novo status + linha no Histórico
wtb doc add --type reference --title "ai-radar-<slug>" \
  --tag "ai-radar,<novo-status>" --date YYYY-MM-DD --file /tmp/ai-radar-<slug>.md

# Atualizar índice após qualquer mudança
wtb doc delete ai-radar-index
# editar /tmp/ai-radar-index.md
wtb doc add --type reference --title "ai-radar-index" \
  --tag "ai-radar,index" --date YYYY-MM-DD --file /tmp/ai-radar-index.md
```

**Regra:** sempre atualizar o índice quando status mudar. O índice é o ponto de entrada do passo 0.

## Dimensões de avaliação do ecossistema atual

| Área | Como é feita hoje | O que melhoraria |
|------|-------------------|-----------------|
| GitHub CI/PR | `gh` CLI subprocess + GitHub MCP | MCP com mais cobertura de eventos |
| Jira | Atlassian MCP | — já bom |
| Context management | MEMORY.md + context.json + topic files | RAG local, chunking automático |
| K8s troubleshooting | kubectl subprocess | MCP k8s com multi-context |
| Snowflake queries | Python script SSO + snowsql subprocess | MCP oficial Snowflake |
| Code review | CodeRabbit + review manual | — já bom |
| Ops probes | wtb Go + YAML heuristics | Heurísticas com ML adaptativo |

## Cadência e state file

**Execução automática:** 1x/dia, no início de sessão via CLAUDE.md §6.
State file: `~/workflow/.ai-radar-last-run` (contém `YYYY-MM-DD` da última execução).

**Após rodar** (sempre, independente de achados):
```bash
echo $(date +%Y-%m-%d) > ~/workflow/.ai-radar-last-run
```

**Comportamento:**
- Achados relevantes → apresentar relatório completo antes de prosseguir
- Sem achados → silencioso (`— AI Radar: sem novidades relevantes.` em uma linha)
- Scopes ausentes no bot → silencioso + nota única: `— AI Radar: pendente setup groups:history` (não repetir a cada sessão — verificar state file)

**Execução manual:** `wtb memory get ai-radar` carrega a skill; pedir "roda o ai-radar" ou "novidades AI".
