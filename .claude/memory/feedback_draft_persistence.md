---
name: feedback_draft_persistence
description: Regra de persistência de rascunhos de comunicação externa produzidos em sessão (Slack, email, Jira, anúncios)
type: feedback
---

Todo texto produzido em sessão destinado a comunicação externa (anúncio Slack, email, post, comentário Jira, release note) deve ser salvo imediatamente no docs.db como `draft` — nunca deixar no clipboard ou marcar como "não persistido".

**Por que:** Rascunho de anúncio de novos event types de câmera (30/03/2026) ficou só no clipboard. Na sessão seguinte foi necessário reconstruir do zero — custo desnecessário, risco de divergência.

**Como aplicar:**
```bash
wtb doc add --type draft \
  --title "<contexto do comunicado — canal/destino/assunto>" \
  --date YYYY-MM-DD \
  --tag "draft,slack|email|jira,<contexto>" \
  --content "<texto completo do rascunho>"
```

Regras de vinculação na cadeia de docs:
- Se o draft originou de um discovery → adicionar tag com ID do discovery
- Se o draft foi postado → registrar no savepoint: `draft: <id> → postado em #canal em YYYY-MM-DD`
- Se o draft foi descartado → `wtb doc delete <id>` com nota no savepoint

**Não é necessário guardar:** respostas de code review inline curtas (≤3 linhas), variáveis de ambiente, IDs temporários de debug.
