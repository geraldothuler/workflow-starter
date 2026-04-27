---
name: playwright
description: >
  Testes E2E e automação de browser no herbie-dashboard e outros apps Cobli.
  Usa @playwright/mcp (headless Chromium). Ativar quando o usuário pedir
  "testa o fluxo", "abre o herbie", "valida a tela", "roda E2E", "testa a importação",
  "verifica o comportamento no browser", ou qualquer validação visual de UI.
user-invocable: true
---

# Playwright MCP — Testes E2E Cobli

**MCP:** `playwright` (headless Chromium, via `mcpServers` em settings.json)
**Versão:** `@playwright/mcp@0.0.68`
**Modo:** headless — sem janela visível. Para debug visual usar skill `chrome-devtools`.

---

## Ambientes

| App | URL local | URL staging |
|-----|-----------|-------------|
| herbie-dashboard | `http://localhost:3000` | verificar com equipe |
| janus (GraphQL) | `http://localhost:4000` | — |
| Mastra AI | `http://localhost:4111` | — |

---

## Fluxo 1 — Testar importação de identificadores (SS-2306)

Testa o fluxo completo: upload CSV → validação IA → criação em lote.

```
1. navigate → http://localhost:4200
2. Fazer login (verificar se há sessão ativa)
3. Navegar para a página de motoristas / identificadores
4. Abrir o dialog "Importar identificadores"
5. Upload do arquivo CSV de teste
6. Aguardar validação pela IA (ProcessingScreen)
7. Verificar ValidationScreen — checar erros e válidos
8. Confirmar importação
9. Verificar SuccessScreen ou ErrorScreen
```

**Seletores úteis** (accessibility tree — preferir role/label sobre CSS):
- Dialog de importação: `role=dialog name="Importar identificadores"`
- Botão de upload: `role=button name="Upload"` ou input `type=file`
- Botão continuar: `role=button name="Continuar"`
- Tela de erro: `role=heading name="Falha na importação"`

---

## Fluxo 2 — Smoke test pós-deploy

Verificação rápida após deploy em prod/staging.

```
1. navigate → URL do ambiente
2. Verificar se a página carrega (sem 500/crash)
3. Checar elementos críticos: navbar, menu principal, rota /drivers
4. Verificar console: ausência de erros críticos (JS errors)
5. Screenshot para evidência
```

---

## Fluxo 3 — Verificar tela específica após PR review

Quando revisar um PR com nova UI (ex: PR #10844 — ErrorScreen):

```
1. navigate → http://localhost:4200 (branch do PR rodando local)
2. Navegar até o componente alterado
3. Forçar o estado de erro (mock ou payload inválido)
4. Screenshot da tela
5. Comparar com o design Figma se disponível
```

---

## Rodar smoke tests localmente (herbie-dashboard)

**Pré-requisito: Node 22** (`nvm use 22`). Vite 7 exige Node 20+.

### Setup de auth local (única vez por sessão/token)

Os arquivos abaixo são **locais, não commitados** (`.gitignore`). Para recriar se sumirem:

```bash
# playwright.local.config.ts  →  config sem teardown, usa global.setup.local
# global.setup.local.ts       →  login headful (abre browser para o usuário autenticar)
# fixtures/local.fixture.ts   →  fixture opcional com interceptor de 401
```

Criar via: `/playwright-local-setup` (skill que gera esses 3 arquivos)

### Fluxo completo

```bash
# 1. Build + preview (match exato do CI — evita 401 loop do dev server)
cd ~/Cobliteam/herbie-dashboard/apps/dashboard
nvm use 22
bun run build          # ~1 min; só necessário após mudanças de código
npm run preview &      # serve em :3000, manter rodando

# 2. Rodar smoke tests
cd ~/Cobliteam/herbie-dashboard
npx playwright test \
  --config=e2e/playwright/playwright.local.config.ts \
  e2e/playwright/tests/smoke/identifiers/identifiers-page.test.ts \
  --reporter=list --workers=1
```

**Primeira execução:** browser abre em modo headful. Fazer login normalmente + digitar código MFA do e-mail se solicitado. Token salvo em `.auth/user.json` — próximas runs pulam o login.

**Token expirou?** `rm e2e/playwright/.auth/user.json` e rodar novamente.

**Rodar todos os @smoke:**
```bash
npx playwright test --config=e2e/playwright/playwright.local.config.ts --grep "@smoke"
```

### Por que preview e não dev server

`CobliAxios.datadogCatch()` chama `window.location.reload()` em qualquer 401. Com dev server, chamadas de background (notificações, geofences) retornam 401 antes da página estabilizar → loop de reload → tela de login.
Com `npm run preview` (build estático), esse comportamento não ocorre.

**CI:** CircleCI faz o mesmo — `npm run preview` + `CI=true`. Os configs são idênticos em comportamento.

---

## Gotchas Cobli

- **i18n:** o app usa pt-BR por padrão. Labels nos seletores devem usar o texto PT ("Importar", "Continuar", "Voltar") não EN.
- **Dialog z-index:** componentes Dialog têm `z-dropdown` aplicado. Se um elemento não for clicável, verificar se há overlay de dialog por cima.
- **Mastra workflow:** o fluxo de importação faz chamadas para `localhost:4111`. Garantir que `bun dev` está rodando em `~/Cobliteam/ai/` antes de testar o fluxo completo.
- **Auth Cognito MFA:** primeira run em device novo pede código de e-mail. Cognito lembra o device após isso — runs seguintes sem MFA.
