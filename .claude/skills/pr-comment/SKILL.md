---
name: pr-comment
description: Responde a findings de PR (CodeRabbit, review inline, issue comment) sem duplicar. Sempre verifica histórico antes de postar. Usar sempre que for comentar em PR — nunca chamar `gh pr comment` diretamente.
user-invocable: true
---

# pr-comment — Protocolo anti-duplicata

## Regra fundamental

**Nunca chamar `gh pr comment` sem antes executar o passo 1 (dedup check).**
Comentários duplicados aparecem com meu nome no PR — isso é unprofessional e confunde os revisores.

---

## Passo 1 — Inventário de comentários existentes

Sempre executar os dois comandos antes de qualquer post:

```bash
OWNER=<org>  REPO=<repo>  PR=<número>

# Issue-level comments (flat — inclui nossas respostas a reviews de body)
gh api "repos/$OWNER/$REPO/issues/$PR/comments" \
  --jq '.[] | {id: .id, author: .user.login, snippet: .body[0:120]}'

# Inline review comments (diff-level — CodeRabbit findings inline no código)
gh api "repos/$OWNER/$REPO/pulls/$PR/comments" \
  --jq '.[] | {id: .id, author: .user.login, path: .path, snippet: .body[0:120]}'
```

**Critério de duplicata:** se qualquer comentário existente cita o mesmo trecho
(`> ...`) ou menciona o mesmo arquivo/linha/tema, **não postar**. Encerrar aqui.

---

## Passo 2 — Identificar o tipo de finding e o canal correto

| Tipo de finding | Como identificar | Canal de resposta |
|----------------|-----------------|-------------------|
| **Review comment inline** (no diff) | Aparece em `pulls/$PR/comments` com `path` e `line` | Reply inline: `gh api repos/$OWNER/$REPO/pulls/$PR/comments/$COMMENT_ID/replies -f body="..."` |
| **Review body** (bloco de texto na review) | Aparece em `pulls/$PR/reviews` sem `path` | Issue comment: `gh pr comment $PR --body "..."` |
| **Issue comment** (PR comment plano) | Aparece em `issues/$PR/comments` | Reply como novo issue comment (não há threading) |

Para descobrir o `COMMENT_ID` de um finding inline:

```bash
gh api "repos/$OWNER/$REPO/pulls/$PR/comments" \
  --jq '.[] | select(.user.login | test("coderabbit")) | {id: .id, path: .path, body: .body[0:200]}'
```

---

## Passo 3 — Formatar a resposta

### Resposta a finding CodeRabbit (nitpick/suggestion)

```
> **[título do finding]**

[Decisão: aceito / não aceito + justificativa em 2-3 linhas]

[Se não aceito: explicar *por que* a situação atual é intencional]
```

Regras de tom:
- Não usar "Com certeza" / "Entendido" / "Concordamos"
- Direto ao ponto — decisão primeiro, justificativa depois
- Máximo 5 linhas

### Resposta a CI failure

```
Fix em [commit hash curto]: [o que foi corrigido e por quê era o problema]
```

---

## Passo 4 — Executar o post (após dedup confirmado)

### Reply inline em review comment:
```bash
gh api "repos/$OWNER/$REPO/pulls/$PR/comments/$COMMENT_ID/replies" \
  -f body="$(cat <<'EOF'
[conteúdo da resposta]
EOF
)"
```

### Issue comment (resposta a review body ou comentário plano):
```bash
gh pr comment $PR --repo $OWNER/$REPO --body "$(cat <<'EOF'
[conteúdo da resposta]
EOF
)"
```

---

## Checklist rápido

- [ ] Executei o inventário (Passo 1)?
- [ ] Não encontrei duplicata com mesmo tema/arquivo?
- [ ] Identifiquei o canal correto (inline reply vs issue comment)?
- [ ] Resposta está em ≤5 linhas, sem filler words?
- [ ] Usei heredoc para evitar problemas de escape?

---

## Casos especiais

**Multiple findings na mesma review:** responder em um único comment com seções por finding. Não postar um comment por finding.

**Finding já respondido por humano:** não postar. Verificar se a resposta humana cobre o ponto antes de adicionar.

**Erro de "comment not found":** o `pulls/$PR/comments` e `issues/$PR/comments` são endpoints distintos — conferir qual foi usado na busca antes de concluir que não há duplicata.
