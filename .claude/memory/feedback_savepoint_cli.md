---
name: feedback_savepoint_cli
description: Savepoint via wtb doc add ainda falha — continuar criando .md em ~/.workflow/savepoints/
type: feedback
---

Savepoints vivem exclusivamente no `docs.db` via CLI. Nunca criar arquivos .md para savepoints.

**Por que:** CLAUDE.md é explícito — "Não há arquivo .md a commitar — savepoints vivem exclusivamente no docs.db."

**How to apply:**
- `wtb cycle-check --save --repo .` — sinal técnico
- `wtb doc add --type savepoint --title "..." --date YYYY-MM-DD --content "..."` — savepoint rico
- Flag correta: `--content` (não `--body`, não `--file` para inline)
- Nunca criar .md em `~/.workflow/savepoints/`
