---
name: agent-browser
description: >
  Scraping, automação autônoma e tarefas de browser que não precisam de visibilidade.
  Usa CLI agent-browser (Rust nativo, instalado globalmente). Ativar quando o usuário
  pedir "scraping", "extrai dados da página", "automatiza esse fluxo no browser",
  "acessa e coleta", "verifica o status em", ou qualquer tarefa autônoma de browser
  sem necessidade de debug visual.
user-invocable: true
---

# Agent-Browser — Automação e Scraping Cobli

**CLI:** `agent-browser` (global, Rust nativo)
**Chrome:** `~/.agent-browser/browsers/chrome-147.0.7727.24`
**Modo:** headless por padrão. Para headed: `agent-browser --headed <cmd>`
**Instalação:** `npm install -g agent-browser && agent-browser install`

> **Diferença de playwright e chrome-devtools:**
> - `playwright` → testes E2E estruturados, accessibility tree, assertions
> - `chrome-devtools` → debug visual, network inspector, console
> - `agent-browser` → automação rápida, scraping, tarefas encadeadas sem overhead de MCP

---

## Padrão de uso — encadeamento via daemon

O agent-browser mantém um daemon entre chamadas. Encadear com `&&` reutiliza o browser:

```bash
agent-browser open <url> && \
agent-browser wait --load networkidle && \
agent-browser snapshot -i
```

---

## Fluxo 1 — Extrair dados de uma página (scraping)

```bash
# 1. Abrir e aguardar carregamento
agent-browser open "https://url-alvo.com" && \
agent-browser wait --load networkidle

# 2. Tirar snapshot da árvore de acessibilidade (retorna refs @e1, @e2...)
agent-browser snapshot -i

# 3. Extrair texto de elemento específico por ref
agent-browser get text @e1

# 4. Ou via JavaScript
agent-browser eval "document.querySelector('.dado-alvo').innerText"
```

---

## Fluxo 2 — Automação de login + coleta

Para páginas que requerem autenticação:

```bash
# Login
agent-browser open "https://app.cobli.co/login" && \
agent-browser wait --load networkidle && \
agent-browser snapshot -i
# [usar ref do campo email e senha retornados pelo snapshot]
agent-browser fill @e1 "email@cobli.co" && \
agent-browser fill @e2 "$(security find-generic-password -s <service> -w)" && \
agent-browser click @e3

# Persistir sessão para próximas chamadas
agent-browser --session-name cobli-session open "https://app.cobli.co/dados"
```

---

## Fluxo 3 — Screenshot para evidência

```bash
# Screenshot simples
agent-browser open "https://url" && \
agent-browser wait --load networkidle && \
agent-browser screenshot /tmp/evidencia-$(date +%Y%m%d).png

# Screenshot com anotações visuais (útil para reports)
agent-browser screenshot --annotate /tmp/evidencia-anotada.png

# Screenshot full page
agent-browser screenshot --full /tmp/pagina-completa.png
```

---

## Fluxo 4 — Verificar status de URL (monitoramento pontual)

```bash
agent-browser open "https://url-alvo" && \
agent-browser wait --load networkidle && \
agent-browser eval "document.title" && \
agent-browser snapshot -i | head -20
```

---

## Referência rápida de comandos

| Comando | Uso |
|---------|-----|
| `open <url>` | Navegar para URL |
| `snapshot -i` | Árvore de acessibilidade (só interativos) — retorna refs @e1, @e2 |
| `snapshot` | Árvore completa |
| `click @eN` | Clicar por ref do snapshot |
| `fill @eN "texto"` | Preencher campo |
| `get text @eN` | Ler texto de elemento |
| `eval "<js>"` | Executar JavaScript |
| `screenshot [path]` | Capturar tela |
| `wait --load networkidle` | Aguardar carregamento completo |
| `--session-name <n>` | Persistir cookies/estado entre sessões |
| `--headed` | Modo visível (debug) |
| `--auto-connect` | Conectar ao Chrome já aberto |

---

## Gotchas Cobli

- **Credenciais:** nunca hardcodar — usar `$(security find-generic-password -s <key> -w)` inline
- **SPAs (herbie-dashboard, app Cobli):** sempre usar `wait --load networkidle` após `open` — React precisa hidratar antes dos elementos estarem disponíveis
- **Refs são efêmeras:** `@e1` do snapshot anterior invalida após qualquer navegação. Sempre tirar novo snapshot após mudança de página
- **Sessão persistida:** `--session-name` salva cookies em `~/.agent-browser/sessions/<name>`. Útil para não fazer login a cada vez em automações recorrentes
