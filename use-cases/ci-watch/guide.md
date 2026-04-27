# Use Case: CI Watch — Sentinel e Auto-Fix

**Tipo:** technical | **Engine:** shell + plan template

---

## Quando usar

- CI falhou em um PR e você quer monitorar sem ficar abrindo o browser
- Falha rápida (< 8 minutos) em projeto Kotlin → suspeita de ktlint
- Quer aplicar o fix automático sem loop manual de push → esperar → checar → editar

## Quando NÃO usar

- Falha de infraestrutura de CI (runner sem memória, rede do CircleCI caindo)
- Falha claramente de teste funcional — vá direto ao log e diagnostique
- Quando não tem `local_path` disponível — o sentinel funciona, o auto-fix não

---

## 1. Sentinel — Poll até CI completar {#sentinel}

Substitui ficar recarregando o browser. Cole no terminal após um push:

```bash
# Por PR (resolve o commit automaticamente)
REPO="Cobliteam/webhook"
PR=63

COMMIT=$(gh api repos/$REPO/pulls/$PR --jq '.head.sha')
echo "Monitorando commit ${COMMIT:0:8} no PR #$PR..."

until STATUS=$(gh api repos/$REPO/commits/$COMMIT/check-runs \
  --jq '[.check_runs[] | select(.name | test("test|check|build"))] | map(.conclusion) | unique | .[]' 2>/dev/null); \
  [[ "$STATUS" == "success" ]] || [[ "$STATUS" == "failure" ]]; do
  echo "[$(date +%H:%M:%S)] aguardando... ($STATUS)"
  sleep 30
done
echo "[$(date +%H:%M:%S)] CI: $STATUS"
```

```bash
# Por commit direto
REPO="Cobliteam/webhook"
COMMIT="6c4085c"

until STATUS=$(gh api repos/$REPO/commits/$COMMIT/check-runs \
  --jq '[.check_runs[] | select(.name=="webhook-builder-test")] | last | .conclusion // .status' \
  2>/dev/null); \
  [[ "$STATUS" == "success" ]] || [[ "$STATUS" == "failure" ]]; do
  echo "[$(date +%H:%M:%S)] $STATUS"
  sleep 30
done
echo "Resultado: $STATUS"
```

---

## 2. Detectar tipo de falha {#detect}

Heurística por duração — consultável via `gh api`:

```bash
gh api repos/$REPO/commits/$COMMIT/check-runs \
  --jq '.check_runs[] | select(.conclusion=="failure") |
    {
      name,
      duration_sec: ((.completed_at | fromdateiso8601) - (.started_at | fromdateiso8601)),
      conclusion
    }'
```

**Regra:**
| Duração | Tipo provável | Ação |
|---------|--------------|------|
| < 480s  | ktlint ou compile | Tentar auto-fix |
| ≥ 480s  | falha de teste | Diagnosticar manualmente |

---

## 3. Auto-fix ktlint {#autofix}

Se a falha for ktlint, executar o plan template:

```bash
wtb ops plan-new --template ci-fix-ktlint \
  --var local_path=~/Cobliteam/webhook \
  --var module=webhook-builder
```

Ou manualmente (o plan faz exatamente isso):

```bash
cd ~/Cobliteam/webhook

# 1. Formatar
./gradlew ktlintFormat --quiet

# 2. Verificar (o check report em build/reports/ktlint/ é a fonte de verdade)
./gradlew ktlintCheck --quiet && echo "✓ limpo" || echo "✗ violações restantes"

# 3. Ver o que sobrou (se ktlintCheck falhou)
find . -path "*/build/reports/ktlint/*Check*.txt" -exec cat {} \;

# 4. Commitar (só se ktlintCheck passou)
git add -A
git commit -m "fix: ktlint auto-format"
git push
```

---

## 4. Verificar violações residuais {#residual}

Quando `ktlintFormat` não resolve tudo (regras que requerem intervenção manual):

```bash
# Lê todos os relatórios de Check (não Format) e filtra por violações reais
find . -path "*/build/reports/ktlint/*Check*.txt" ! -empty -exec cat {} \; \
  | grep -v "^Summary" | grep -v "^$"
```

Regras que o format **não resolve automaticamente**:
- `trailing-comma-on-call-site` — argumento posicional na chamada de função
- `no-wildcard-imports` — import com `*`
- Qualquer regra custom do projeto

---

## 5. Diagnosticar falha de teste {#test-diagnose}

Quando a falha **não** é ktlint (duração ≥ 8min ou ktlintCheck local passa), diagnosticar via JUnit XML.

### Pré-condição: infra Docker Compose

O CI usa `docker compose up --wait -d` antes de rodar testes. Localmente, a mesma infra
precisa estar up — caso contrário os testes de integração falham com `AllNodesFailedException`.

```bash
cd ~/Cobliteam/webhook

# 1. Verificar status da infra
docker compose ps

# 2. Se scylla/kafka não estiver healthy — autenticar no ECR (imagem privada)
aws ecr get-login-password --region us-east-1 \
  | docker login --username AWS --password-stdin \
    911383825788.dkr.ecr.us-east-1.amazonaws.com

# 3. Subir infra e aguardar healthchecks (mesmo comportamento do CI)
docker compose up --wait -d
```

**Sinal de infra ausente no JUnit XML:** `AllNodesFailedException` ou `Connection refused` em `build/test-results/` — não é falha de código, é pré-condição.

### Via plan template (recomendado)

```bash
# Inclui check de infra + diagnóstico de XML em um único fluxo
wtb ops plan-new --template test-diagnose \
  --var local_path=~/Cobliteam/webhook \
  --var module=webhook-sender
```

O plan classifica automaticamente: `infra-unavailable` vs `test-failure` (falha de código real).

### Manualmente

```bash
cd ~/Cobliteam/webhook

# 1. Rodar testes
./gradlew :webhook-sender:test 2>&1; echo "exit: $?"

# 2. Se falhou — sumário por classe (classificado)
find webhook-sender/build/test-results -name "*.xml" | while read f; do
  CLASS=$(grep -oP '(?<=classname=")[^"]+' "$f" | head -1)
  FAILS=$(grep -oP '(?<=failures=")[^"]+' "$f" | head -1)
  ERRS=$(grep -oP '(?<=errors=")[^"]+' "$f" | head -1)
  INFRA=$(grep -c "AllNodesFailedException\|Connection refused" "$f" 2>/dev/null || echo 0)
  TYPE=$( [ "$INFRA" -gt 0 ] && echo "infra" || echo "code" )
  echo "[$TYPE] $CLASS — failures:${FAILS:-0} errors:${ERRS:-0}"
done | sort

# 3. Detalhe das falhas de CÓDIGO (ignorar infra)
find webhook-sender/build/test-results -name "*.xml" | while read f; do
  IS_INFRA=$(grep -c "AllNodesFailedException\|Connection refused" "$f" 2>/dev/null || echo 0)
  HAS_FAIL=$(grep -c "<failure" "$f" 2>/dev/null || echo 0)
  if [ "$HAS_FAIL" -gt 0 ] && [ "$IS_INFRA" -eq 0 ]; then
    echo "=== $(basename $f | sed 's/TEST-//;s/.xml//') ==="
    grep -A 6 "<failure" "$f" | sed 's/<[^>]*>//g' | grep -v '^$' | head -10
  fi
done
```

**Por que não usar `--info | grep`:**

| Abordagem | Problema |
|-----------|---------|
| `--quiet \| grep "FAILED"` | `--quiet` suprime output de testes inteiramente |
| `--info \| grep "Exception"` | `--info` despeja centenas de linhas de setup; grep em stream ao vivo é frágil |
| `find reports/tests -name "*.xml"` | Caminho errado — HTML em `build/reports/`, JUnit XML em `build/test-results/` |

**Fonte de verdade: `build/test-results/test/*.xml`** — gerado por qualquer runner JUnit-compatível.

### Extensão por stack

| Stack | Infra local | Comando de teste | JUnit XML path |
|-------|-------------|-----------------|----------------|
| Kotlin + Gradle | `docker compose up --wait -d` | `./gradlew :module:test` | `module/build/test-results/test/*.xml` |
| Java + Maven | `docker compose up --wait -d` | `mvn test -pl module` | `module/target/surefire-reports/*.xml` |
| Python + pytest | variável por projeto | `pytest --junit-xml=results.xml` | `results.xml` |
| JavaScript + Jest | variável por projeto | `jest --reporters=jest-junit` | `junit.xml` (configurável) |

---

## 6. Ciclo completo (referência rápida)

```
push
  └─► sentinel poll (até success/failure)
          └─► failure + duração < 8min?
                  └─► ktlintFormat → ktlintCheck
                          ├─► limpo → commit + push → sentinel novamente
                          └─► violações restantes → fix manual → commit + push
```

---

## Padrão de sentinel para outros projetos

Este mesmo padrão de poll se aplica para qualquer CI baseado em GitHub check-runs.
Para projetos não-Kotlin, substituir o step de auto-fix pelo equivalente:

| Stack | Format command |
|-------|---------------|
| Kotlin + Gradle | `./gradlew ktlintFormat` |
| JavaScript/TypeScript | `npm run lint:fix` ou `npx eslint --fix` |
| Python | `ruff format .` ou `black .` |
| Go | `gofmt -w .` |
| Swift | `swiftformat .` |

O sentinel de poll e a heurística de duração são universais — só o step de format muda.
