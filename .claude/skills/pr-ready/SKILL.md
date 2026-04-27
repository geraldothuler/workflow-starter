# Skill: pr-ready

Ciclo autônomo completo de PR: rebase → push → aguarda CodeRabbit → aplica findings → responde comments → monitora CI → repete até verde e sem findings críticos.

**Padrão: sem confirmações.** Todas as ações (commit, push, force-push) são executadas automaticamente.

## Quando usar

- "cria o PR e itera com o rabbit"
- "pr-ready"
- "sobe o PR e monitora"
- "resolve os findings e monitora CI"
- Após qualquer `gh pr create` ou rebase que precisa do ciclo completo

## Protocolo completo

O ciclo é um **loop**, não uma sequência linear. Todo push reinicia o Rabbit.

```
LOOP:
  push → aguarda Rabbit → lê findings → aplica → responde → aguarda CI
  se CI falhou  → corrige → push → LOOP (reinicia do topo)
  se novos findings → push → LOOP (reinicia do topo)
  se CI verde + Rabbit concluído + sem findings críticos → ENCERRAR
```

### Fase 0 — contexto (uma vez só)

```bash
cd <repo_path>
PR=$(gh pr view --json number -q .number)
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
BASE=$(gh pr view --json baseRefName -q .baseRefName)
CIRCLE_CI_API_KEY=$(bash ~/workflow/scripts/secret-get.sh workflow-circleci-token)
```

### Fase 1 — rebase e push (primeira vez ou após conflito)

```bash
git fetch origin
git rebase origin/$BASE
```

**Resolução de conflitos:**
- Commit idêntico ao upstream: `git rebase --skip`
- Conflito real: resolver, `git add`, `git rebase --continue`

```bash
git push --force-with-lease origin <branch>
```

---

### ↺ INÍCIO DO LOOP (repetir após qualquer push)

### Fase 2 — aguardar CodeRabbit terminar

**Obrigatório após todo push** — o Rabbit roda novamente e pode abrir novos findings.

```bash
# Aguardar Rabbit sair de pending (pode demorar vários minutos)
while true; do
  STATUS=$(gh pr checks $PR --repo $REPO 2>&1 | grep "CodeRabbit" | awk '{print $2}')
  echo "$(date +%H:%M:%S) CodeRabbit: $STATUS"
  [ "$STATUS" != "pending" ] && break
  sleep 30
done
```

Se CodeRabbit não aparecer (PR muito novo): aguardar 60s e re-checar.

### Fase 3 — ler inline comments sem reply

```bash
# Todos os comments do Rabbit
RABBIT_COMMENTS=$(gh api "repos/$REPO/pulls/$PR/comments" \
  --jq '[.[] | select(.body | contains("auto-generated comment by CodeRabbit")) | {id: .id, path: .path, line: .line, body: .body}]')

# Replies já postadas
REPLIED_IDS=$(gh api "repos/$REPO/pulls/$PR/comments" \
  --jq '[.[] | select(.in_reply_to_id != null) | .in_reply_to_id]')

# Pendentes = RABBIT_COMMENTS cujo id não está em REPLIED_IDS
```

**Nota:** `resolved` do GitHub nem sempre é preenchido — filtrar por ausência de reply, não por esse campo.

Classificar por severidade:
- `🔴 Critical` / `⚠️ Potential issue` → **aplicar obrigatoriamente**
- `🛠️ Refactor suggestion` / `🟠 Major` → aplicar se não introduzir risco
- `🟡 Minor` → aplicar se simples e sem side effects
- `ℹ️ Nitpick` → opcional, aplicar se trivial

Se não há findings pendentes → pular para Fase 6 (CI).

### Fase 4 — aplicar fixes

Para cada finding a aplicar:
1. Editar arquivo conforme sugestão
2. Validar: `./gradlew :<module>:ktlintCheck :<module>:test`
3. Commit por finding (não agrupar):
   ```bash
   git commit -m "fix: <descrição do finding>

   Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
   ```
4. Após todos os fixes: `git push --force-with-lease origin <branch>`
5. **→ Voltar ao início do loop** (Fase 2) — push disparou novo review do Rabbit

### Fase 5 — responder inline comments

**Sempre verificar antes de postar** — não há idempotência na API.

```bash
reply_if_no_reply() {
  local COMMENT_ID=$1
  local BODY=$2
  EXISTING=$(gh api "repos/$REPO/pulls/$PR/comments" \
    --jq "[.[] | select(.in_reply_to_id == $COMMENT_ID)] | length")
  if [ "$EXISTING" -gt 0 ]; then
    echo "Skipping $COMMENT_ID — reply already exists"
    return
  fi
  gh api "repos/$REPO/pulls/$PR/comments/$COMMENT_ID/replies" \
    -f body="$BODY" --silent && echo "Replied to $COMMENT_ID"
}

reply_if_no_reply $COMMENT_ID "Addressed in <sha>: <o que foi feito>"
reply_if_no_reply $COMMENT_ID "Not applied: <justificativa objetiva>"
```

**Endpoint correto:** `repos/{owner}/{repo}/pulls/{PR}/comments/{id}/replies`
(não `pulls/comments/{id}/replies` — esse retorna 404)

### Fase 6 — monitorar CI (CircleCI)

```bash
# Obter workflow ID do run mais recente
WORKFLOW_URL=$(gh pr checks $PR --repo $REPO 2>&1 | grep "circleci.com/pipelines" | head -1 | awk '{print $3}')
WORKFLOW_ID=$(echo $WORKFLOW_URL | grep -oP 'workflows/\K[^?]+')

# Poll até concluir
while true; do
  STATUS=$(curl -s -H "Circle-Token: $CIRCLE_CI_API_KEY" \
    "https://circleci.com/api/v2/workflow/$WORKFLOW_ID" | jq -r '.status')
  echo "$(date +%H:%M:%S) CI: $STATUS"
  [ "$STATUS" != "running" ] && [ "$STATUS" != "on_hold" ] && break
  sleep 30
done
```

**Se CI falhou:**
```bash
# Jobs com falha
curl -s -H "Circle-Token: $CIRCLE_CI_API_KEY" \
  "https://circleci.com/api/v2/workflow/$WORKFLOW_ID/job" \
  | jq '.items[] | select(.status=="failed") | {name, job_number}'
```
Diagnosticar, corrigir, commitar, `git push` → **voltar ao início do loop** (Fase 2).

**Se CI verde → verificar critério de encerramento abaixo.**

### Critério de encerramento

Sair do loop somente quando **todos** forem verdadeiros:
1. CI verde (todos os checks `pass`)
2. CodeRabbit completou (não está `pending`)
3. Sem findings críticos/major sem reply

### Fase 7 — reportar

```
PR #<N> pronto:
- CI: verde (pipeline <X>)
- CodeRabbit: <N> findings aplicados, <M> descartados com justificativa
- Iterações: <quantos loops foram necessários>
- Commits adicionados: <lista>
```

## Variáveis de ambiente e tokens

| Recurso | Como obter |
|---------|-----------|
| CircleCI token | `bash ~/workflow/scripts/secret-get.sh workflow-circleci-token` |
| GitHub | `gh` CLI autenticado (sem token manual) |

## Heurísticas de rebase

| Situação | Ação |
|----------|------|
| `patch contents already upstream` no log do rebase | `git rebase --skip` — commit duplicado |
| Conflito em arquivo que mudou upstream e no branch | Resolver manualmente — inspecionar `git diff` de ambos os lados |
| Branch muito divergido (>10 commits divergindo) | Avaliar se merge é melhor que rebase — preferir rebase para PRs ativos |

## Armadilhas conhecidas

- `gh run watch` **não funciona** — CI é CircleCI, não GitHub Actions. Sempre usar CircleCI API.
- **Agents em background não têm permissão de Bash** por padrão — monitorar CI diretamente no contexto principal com poll loop.
- CodeRabbit pode abrir comments **depois** que o CI já passou — sempre aguardar CodeRabbit (fase 7) antes de declarar concluído.
- `force-with-lease` falha se o remote avançou — fazer `git fetch` antes.
- Commit de fix do CodeRabbit não deve agrupar múltiplos findings — um commit por finding facilita o review e o revert se necessário.
- **Verificar duplicatas antes de postar replies** — `gh api` não impede double-post. Usar `reply_if_no_reply()` (fase 5).
- `resolved` nos comments do GitHub pode ser `null` mesmo para comments tratados — não filtrar por esse campo.
- Replies ficam em `GET /pulls/{PR}/comments` com `in_reply_to_id != null`, não em endpoint separado.
