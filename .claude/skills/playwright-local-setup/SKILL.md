---
name: playwright-local-setup
description: >
  Cria os arquivos locais de E2E do herbie-dashboard que não são commitados no repo:
  playwright.local.config.ts, global.setup.local.ts e fixtures/local.fixture.ts.
  Ativar quando esses arquivos sumirem ou precisarem ser recriados.
user-invocable: true
---

# Playwright Local Setup — herbie-dashboard

Cria 3 arquivos em `~/Cobliteam/herbie-dashboard/e2e/playwright/` que **não são commitados** (estão no `.gitignore`). Eles permitem rodar smoke tests localmente com auth Cognito headful.

## Arquivos a criar

### 1. `playwright.local.config.ts`

Config sem `globalTeardown` (token persiste entre runs) e `reuseExistingServer: true`.

```typescript
import { defineConfig, devices } from '@playwright/test';
import * as dotenv from 'dotenv';
import * as path from 'path';

dotenv.config({ path: path.resolve(__dirname, '.env') });

const WAIT_TIMEOUT = 30 * 1000;

export default defineConfig({
  globalSetup: path.resolve(__dirname, './global.setup.local'),
  // Sem globalTeardown: auth persiste entre runs locais
  testDir: './tests/',
  fullyParallel: false,
  forbidOnly: false,
  retries: 0,
  workers: 1,
  timeout: WAIT_TIMEOUT,
  expect: { timeout: WAIT_TIMEOUT },
  reporter: [['list']],
  use: {
    baseURL: 'http://localhost:3000',
    headless: true,
    ignoreHTTPSErrors: true,
    storageState: path.resolve(__dirname, '.auth/user.json'),
    screenshot: 'only-on-failure',
    actionTimeout: WAIT_TIMEOUT,
    navigationTimeout: 60000,
  },
  webServer: {
    command: `cd ${path.resolve(__dirname, '../../apps/dashboard')} && npm run dev`,
    url: 'http://localhost:3000',
    timeout: 5 * 60 * 1000,
    ignoreHTTPSErrors: true,
    reuseExistingServer: true,
    stdout: 'ignore',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        locale: 'pt-BR',
        headless: true,
        serviceWorkers: 'block',
        timezoneId: 'America/Sao_Paulo',
        viewport: { width: 1460, height: 920 },
        ignoreHTTPSErrors: true,
      },
    },
  ],
});
```

### 2. `global.setup.local.ts`

Login headful (browser visível) na primeira vez; reutiliza token nas seguintes.

```typescript
import { chromium } from '@playwright/test';
import * as dotenv from 'dotenv';
import * as fs from 'fs';
import jwtDecode from 'jwt-decode';
import * as path from 'path';

dotenv.config({ path: path.resolve(__dirname, '.env') });

const authDir = path.join(__dirname, '.auth');
const authFile = path.join(authDir, 'user.json');

if (!fs.existsSync(authDir)) {
  fs.mkdirSync(authDir, { recursive: true });
}
if (!fs.existsSync(authFile)) {
  fs.writeFileSync(authFile, '{}');
}

async function globalSetupLocal() {
  // Verifica se já existe um token válido — pula login se sim
  if (fs.existsSync(authFile)) {
    const authData = JSON.parse(fs.readFileSync(authFile, 'utf8'));
    const cobliAccessToken = authData.origins?.[0]?.localStorage?.find(
      (item: any) => item.name === 'cobli-access-token'
    );

    if (cobliAccessToken) {
      try {
        const tokenExpiration = jwtDecode<any>(cobliAccessToken.value).exp;
        const isTokenValid = tokenExpiration > Date.now() / 1000;
        const minutesLeft = Math.floor((tokenExpiration * 1000 - Date.now()) / 60000);

        if (isTokenValid) {
          console.info(`✅ Token válido — expira em ${minutesLeft} min. Pulando login.`);
          return;
        }
        console.info('⚠️  Token expirado. Iniciando login headful...');
      } catch {
        console.info('⚠️  Token inválido. Iniciando login headful...');
      }
    }
  }

  if (!process.env.E2E_USERNAME || !process.env.E2E_PASSWORD) {
    throw new Error('E2E_USERNAME e E2E_PASSWORD devem estar no .env');
  }

  console.info('🌐 Abrindo browser para login com verificação Cognito...');
  console.info('   Preencha o código que chegará no seu e-mail.');

  const browser = await chromium.launch({ headless: false, slowMo: 200 });

  try {
    const context = await browser.newContext();
    const page = await context.newPage();

    await page.goto('http://localhost:3000');
    await page.getByPlaceholder('nome@empresa.com').fill(process.env.E2E_USERNAME || '');
    await page.getByPlaceholder('Insira sua senha').fill(process.env.E2E_PASSWORD || '');
    await page.getByRole('button', { name: 'Entrar' }).click();

    console.info('⏳ Aguardando conclusão do login (incluindo código de e-mail se solicitado)...');

    // Aguarda navegação para fora da tela de login (5 min para digitar MFA)
    await page.waitForURL(
      (url) => !url.toString().includes('/login') && url.toString() !== 'http://localhost:3000/',
      { timeout: 5 * 60 * 1000, waitUntil: 'domcontentloaded' }
    );
    // Aguarda token aparecer no localStorage após redirect
    await page.waitForFunction(() => !!localStorage.getItem('cobli-access-token'), { timeout: 30000 });

    const storageState = await context.storageState();
    fs.writeFileSync(authFile, JSON.stringify(storageState));
    console.info('✅ Login concluído. Token salvo em .auth/user.json');
    console.info('   Próximas runs vão reutilizar este token sem precisar logar novamente.');
  } finally {
    await browser.close();
  }
}

export default globalSetupLocal;
```

### 3. `fixtures/local.fixture.ts`

Fixture opcional com interceptor de 401 (para casos onde ainda ocorra reload loop).

```typescript
import { test as base, expect } from '@playwright/test';

export { expect };

export const test = base.extend<{ preventAuthReload: void }>({
  preventAuthReload: [
    async ({ page }, use) => {
      await page.route('**/*', async (route) => {
        const response = await route.fetch().catch(() => null);
        if (response && response.status() === 401) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({}),
          });
        } else if (response) {
          await route.fulfill({ response });
        } else {
          await route.continue();
        }
      });
      await use();
    },
    { auto: true },
  ],
});
```

## Como usar após criar os arquivos

```bash
# Build uma vez antes de rodar
cd ~/Cobliteam/herbie-dashboard/apps/dashboard && nvm use 22 && bun run build && npm run preview &

# Rodar testes (primeira vez: browser abre para MFA)
cd ~/Cobliteam/herbie-dashboard
npx playwright test \
  --config=e2e/playwright/playwright.local.config.ts \
  e2e/playwright/tests/smoke/identifiers/identifiers-page.test.ts \
  --reporter=list --workers=1

# Token expirou? Delete e repita:
rm e2e/playwright/.auth/user.json
```

## .gitignore

Confirmar que estes patterns estão no `.gitignore` do repo:
```
e2e/playwright/playwright.local.config.ts
e2e/playwright/global.setup.local.ts
e2e/playwright/fixtures/local.fixture.ts
e2e/playwright/.auth/
e2e/playwright/.env.local
```
