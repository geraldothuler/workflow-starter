---
name: chrome-devtools
description: >
  Debug de apps Cobli via Chrome DevTools Protocol: inspecionar network, console,
  DOM, performance e screenshots. Usa chrome-devtools-mcp (headed — browser visível).
  Ativar quando o usuário pedir "inspeciona a request", "vê o network", "debug no browser",
  "captura o console", "analisa a performance", "abre o herbie e inspeciona", ou qualquer
  investigação de comportamento de browser com visibilidade.
user-invocable: true
---

# Chrome DevTools MCP — Debug Cobli

**MCP:** `chrome-devtools` (headed Chrome, via `mcpServers` em settings.json)
**Versão:** `chrome-devtools-mcp@0.20.3`
**Modo:** headed — browser visível. Para testes headless usar skill `playwright`.

---

## Fluxo 1 — Inspecionar request do batch-create (SS-2306)

Quando o fluxo de importação falha e precisa entender o que foi enviado/recebido.

```
1. navigate → http://localhost:4200
2. Fazer login
3. Abrir painel de Network (DevTools)
4. Filtrar por: /identification-tokens/batch-create  (ou GraphQL /graphql)
5. Executar o fluxo de importação
6. Capturar request payload e response
7. Comparar com schema esperado (fusca-api-openapi.yaml)
```

**O que procurar:**
- Status 422: erros de validação por item (checar `extensions.index` nos erros GraphQL)
- Status 403/401: problema de auth/permissão
- Payload malformado: verificar se `{"items": [...]}` está correto

---

## Fluxo 2 — Capturar erros de console

Para investigar erros JS silenciosos que não aparecem na UI.

```
1. navigate → URL do ambiente
2. Abrir console listener
3. Executar a ação que suspeita estar falhando
4. Capturar todos os logs de console (error, warn)
5. Identificar stack trace e origem do erro
```

---

## Fluxo 3 — Audit de performance (Lighthouse)

Para avaliar bundle size e performance após mudanças grandes no herbie-dashboard.

```
1. navigate → http://localhost:4200
2. Executar Lighthouse audit
3. Verificar: FCP, LCP, TBT, bundle size
4. Comparar com baseline anterior
```

---

## Fluxo 4 — Screenshot com anotações

Para evidência em postmortem, review ou report de bug.

```
1. navigate → URL com o estado a ser capturado
2. screenshot (com --annotate se disponível)
3. Salvar em /tmp/ para referência
```

---

## Gotchas Cobli

- **Headed vs headless:** este MCP abre uma janela Chrome visível. Normal — é intencional para debug.
- **GraphQL:** o herbie-dashboard usa GraphQL via Janus (`/graphql`). No Network filter, usar `graphql` e inspecionar o campo `operationName` no payload para identificar a mutation correta (`BatchCreateIdentificationTokens`, `GetDrivers`, etc.).
- **CORS local:** em dev, o herbie pode ter CORS configurado apenas para `localhost:4200`. Se mudar a porta, as requests para Janus podem falhar.
- **LaunchDarkly flags:** algumas features ficam ocultas por flag. Se um componente não aparecer, verificar se a flag está ativa via skill `launchdarkly` antes de debugar no browser.
