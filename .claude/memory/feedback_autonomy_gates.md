---
name: feedback_autonomy_gates
description: Quais ações requerem confirmação explícita — apenas gates humanos externos; tudo interno é autônomo
type: feedback
---

Prosseguir sem pedir confirmação para qualquer ação que não gere efeito externo irreversível:
- Edições de arquivo (Edit, Write)
- go build, go test, npm build, compilações
- Loops de re-index, re-index --force
- Buscas, leituras, queries SQL de leitura
- DELETE/UPDATE no SQLite/DuckDB local (backlog.db, docs.db, repos.duckdb)

**Why:** Confirmações desnecessárias interrompem o fluxo e não acrescentam segurança quando o impacto é local e reversível.

**How to apply:** Só pausar e pedir confirmação explícita para:
- `gh pr create` / `git push` / `git merge`
- Deploy em produção
- Postar mensagem Slack, comentar em PR, criar/fechar issue Jira
- Qualquer escrita em sistema externo (DD monitor, Airbyte, LaunchDarkly)
