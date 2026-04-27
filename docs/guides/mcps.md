> 📍 [README](../../README.md) > Guides > MCPs Recomendados

# MCPs Recomendados

MCPs (Model Context Protocol servers) são o maior multiplicador de produtividade do stack. Sem eles, o Claude usa subprocess/CLI e faz parse de texto — com eles, opera diretamente sobre dados estruturados com segurança e contexto completo.

**Regra:** sempre tentar MCP antes de `gh`/curl/subprocess. Output estruturado, sem parse de texto, sem risco de rate limit de terminal.

Use `/mcp-setup` para verificar o que está instalado e guiar a configuração do que falta.

---

## MCPs essenciais

### GitHub
**Por que:** PR review, CI checks, issues, deployments — sem precisar de `gh` CLI nem parse de markdown.

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_PERSONAL_ACCESS_TOKEN": "<token>" }
    }
  }
}
```

Ferramentas chave: `list_check_runs`, `get_pull_request`, `list_workflow_runs`, `create_pull_request`, `get_issue`, `list_commits`.

Gerar token: github.com → Settings → Developer settings → Personal access tokens → Fine-grained.
Escopos mínimos: `repo`, `pull_requests`, `checks`, `actions`.

---

### Atlassian (Jira + Confluence)
**Por que:** criar, buscar, transicionar tickets Jira e ler/escrever Confluence sem browser.

Disponível via claude.ai MCP integrations (sem config manual — habilitar no painel Claude.ai).

Ferramentas chave: `getJiraIssue`, `searchJiraIssuesUsingJql`, `createJiraIssue`, `transitionJiraIssue`, `getConfluencePage`, `createConfluencePage`.

---

### Datadog
**Por que:** RCA, métricas APM, logs, traces, dependências de serviço — sem precisar de curl com API key.

Disponível via claude.ai MCP integrations (OAuth SSO — sem API key manual no config).

Ferramentas chave: `search_datadog_logs`, `get_datadog_metric`, `search_datadog_traces`, `analyze_datadog_logs`, `search_datadog_service_dependencies`.

> Fallback para escrita (criar monitor, editar dashboard): `curl` com `DD-API-KEY` + `DD-APP-KEY` do Keychain.

---

### Slack
**Por que:** ler canais, buscar mensagens, enviar drafts — sem copiar/colar manualmente.

Disponível via claude.ai MCP integrations.

Ferramentas chave: `slack_read_channel`, `slack_search_public_and_private`, `slack_send_message_draft`, `slack_read_thread`.

> Padrão: sempre apresentar rascunho no chat + `pbcopy` antes de enviar. Nunca enviar diretamente sem confirmação.

---

### Browser (claude-in-chrome)
**Por que:** automação, scraping, testes E2E, debug de rede/console — Claude controla o Chrome diretamente.

Instalar extensão Claude in Chrome → habilitar no Claude Code Desktop.

Ferramentas chave: `navigate`, `read_page`, `find`, `form_input`, `javascript_tool`, `read_console_messages`, `gif_creator`.

---

### Notion
**Por que:** criar, ler e atualizar páginas Notion sem precisar abrir browser.

Disponível via claude.ai MCP integrations.

Ferramentas chave: `notion-fetch`, `notion-create-pages`, `notion-search`, `notion-update-page`.

---

### Figma
**Por que:** ler designs, gerar código a partir de componentes, criar diagramas FigJam.

Disponível via claude.ai MCP integrations.

Ferramentas chave: `get_design_context`, `get_screenshot`, `generate_diagram`, `get_code_connect_suggestions`.

---

## Configuração no Claude Code

MCPs ficam em `~/.claude/settings.json` (global) ou `.claude/settings.json` (projeto).

```json
{
  "mcpServers": {
    "github": { ... }
  }
}
```

MCPs via claude.ai (Atlassian, Datadog, Slack, Notion, Figma) são habilitados em:
claude.ai → Settings → Integrations → MCP Servers

Verificar MCPs ativos na sessão: o Claude lista automaticamente as ferramentas disponíveis ao iniciar.

---

## Precedência de uso

| Tarefa | MCP preferido | Fallback |
|--------|--------------|---------|
| PR, CI, issues | GitHub MCP | `gh` CLI |
| Jira, Confluence | Atlassian MCP | `curl` Jira REST |
| Logs, métricas, RCA | Datadog MCP | `curl` DD API |
| Slack mensagens | Slack MCP | `curl` Slack API |
| Browser, scraping | claude-in-chrome | Playwright skill |
| Documentação | Notion MCP | markdown + `wtb doc` |

---

## Keychain para fallbacks

Credenciais de fallback ficam no macOS Keychain, nunca em arquivos commitados:

```bash
# Gravar
security add-generic-password -s "workflow-dd-api-key" -a $(whoami) -w "<valor>"

# Ler
security find-generic-password -s "workflow-dd-api-key" -a $(whoami) -w
```

Serviços recomendados para registrar: `workflow-dd-api-key`, `workflow-dd-app-key`, `workflow-slack-token`, `workflow-github-token`.
