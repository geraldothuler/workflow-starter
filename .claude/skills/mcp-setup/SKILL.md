# Skill: mcp-setup

Wizard interativo que detecta MCPs instalados, identifica gaps e guia a configuração do que falta.

## Quando usar

- `/mcp-setup` — ao começar a usar o workflow-starter pela primeira vez
- `/mcp-setup` — quando uma ferramenta MCP falha e quer diagnosticar a causa
- `/mcp-setup --check` — só verificar, sem guiar configuração

## Execução

### 1. Detectar MCPs ativos na sessão

Inspecione as ferramentas disponíveis na sessão atual. MCPs ativos ficam como ferramentas com prefixo `mcp__<servidor>__*`.

Catalogue quais estão presentes:

| MCP | Prefixo esperado | Status |
|-----|-----------------|--------|
| GitHub | `github__*` ou tool nativa | ✓ / ✗ |
| Atlassian | `mcp__claude_ai_Atlassian__*` | ✓ / ✗ |
| Datadog | `mcp__datadog__*` | ✓ / ✗ |
| Slack | `mcp__claude_ai_Slack__*` | ✓ / ✗ |
| Notion | `mcp__claude_ai_Notion__*` | ✓ / ✗ |
| Figma | `mcp__claude_ai_Figma__*` | ✓ / ✗ |
| Browser (claude-in-chrome) | `mcp__claude-in-chrome__*` | ✓ / ✗ |

### 2. Ler configuração atual

```bash
cat ~/.claude/settings.json 2>/dev/null || echo "{}"
cat .claude/settings.json 2>/dev/null || echo "{}"
```

### 3. Apresentar diagnóstico

Mostre ao usuário uma tabela clara:

```
MCPs detectados nesta sessão:
  ✓ GitHub
  ✓ Atlassian (Jira + Confluence)
  ✗ Datadog — não configurado
  ✗ Slack — não configurado
  ✓ Browser (claude-in-chrome)
  ✗ Notion — não configurado
  ✗ Figma — não configurado
```

### 4. Perguntar quais o usuário usa

Pergunte (1 pergunta por vez, padrão socrático):

> "Qual dessas ferramentas faltantes você usa no dia a dia? (Datadog, Slack, Notion, Figma — ou todas)"

### 5. Guiar configuração de cada um

Para cada MCP faltante que o usuário confirmar, forneça as instruções exatas:

#### GitHub MCP

```bash
# Gerar token em: github.com → Settings → Developer settings → Personal access tokens → Fine-grained
# Escopos: repo, pull_requests, checks, actions
```

Configuração em `~/.claude/settings.json`:
```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "<cole o token aqui>"
      }
    }
  }
}
```

Salvar token no Keychain:
```bash
security add-generic-password -s "workflow-github-token" -a $(whoami) -w "<token>"
```

#### Atlassian, Datadog, Slack, Notion, Figma

Todos via claude.ai integrations — sem config manual de JSON:

1. Acesse: claude.ai → Settings → Integrations
2. Encontre o serviço na lista
3. Clique "Connect" e faça OAuth
4. Reinicie o Claude Code para carregar as ferramentas

#### Browser (claude-in-chrome)

1. Instale a extensão "Claude in Chrome" na Chrome Web Store
2. No Claude Code Desktop: Settings → Extensions → Claude in Chrome → Enable
3. Abra o Chrome antes de usar ferramentas de browser

### 6. Verificar após configuração

Após o usuário confirmar que configurou, teste cada MCP com uma query simples:

- **GitHub**: tente listar os repositórios do usuário
- **Atlassian**: tente buscar um projeto Jira recente
- **Datadog**: tente listar serviços monitorados
- **Slack**: tente listar os canais disponíveis
- **Browser**: navegue para `about:blank` e confirme resposta

### 7. Registrar Keychain para fallbacks

Ao final, pergunte se o usuário quer registrar API keys de fallback no Keychain:

```bash
# Datadog
security add-generic-password -s "workflow-dd-api-key" -a $(whoami) -w "<DD API Key>"
security add-generic-password -s "workflow-dd-app-key" -a $(whoami) -w "<DD App Key>"

# Slack
security add-generic-password -s "workflow-slack-token" -a $(whoami) -w "<Bot Token>"
```

## Referência

Guia completo: `docs/guides/mcps.md`

## Flags

- `--check` — só exibe diagnóstico, não guia configuração
- `--mcp <nome>` — foca em um MCP específico (ex: `/mcp-setup --mcp datadog`)
