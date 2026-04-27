---
name: savepoint
description: Encerra um ciclo de trabalho — limpa pods efêmeros, atualiza memória, faz backup dos DBs, grava savepoint técnico e cria savepoint rico no docs.db. Invocar no fim de sessão ou ao concluir um ciclo estável.
argument-hint: [--repo path]
allowed-tools: Bash, Read, Edit, Write
disable-model-invocation: false
user-invocable: true
---

# Savepoint — Encerramento de Ciclo

**Regra fundamental:** savepoints vivem em `docs.db` via `wtb doc add`. Nunca criar arquivo .md de savepoint.

## Current state
- Date: !`date +%Y-%m-%d`
- Git changes: !`cd ~/workflow && git status --short`
- Last savepoint: !`wtb doc list --type savepoint --since $(date -v-7d +%Y-%m-%d) 2>/dev/null | head -5 || echo "none found"`

## Instructions

1. Parse `$ARGUMENTS` for `--repo <path>`. Default: `~/workflow`.

2. Executar os 9 passos do Session Exit Rule em ordem:

   **0. `/pod-cleanup`** — deletar pods efêmeros Completed (cqlsh-*, *-probe, kubectl-run-*, evicted), sinalizar suspeitos de outros times.

   **1. Memory observer:**
   ```bash
   bash ~/workflow/scripts/memory-observer.sh --repo <repo>
   ```
   Analisar propostas → executar os `wtb memory set` aprovados.

   **2–5. Topic files** — atualizar se houver padrões novos confirmados, IDs de conexão, regras de processo ou skills com comportamento novo.

   **6. Custo do dia:**
   ```bash
   bash ~/workflow/monitors/cost-report.sh
   ```
   Registrar o valor no savepoint rico (passo 9).

   **7. Backup dos DBs:**
   ```bash
   bash ~/workflow/scripts/db-backup.sh
   ```
   Se falhar → **não prosseguir**.

   **8. Savepoint técnico (sinais):**
   ```bash
   wtb cycle-check --save --repo <repo>
   ```
   Persiste em `docs.db` automaticamente. Exibe score e ID.

   **9. Savepoint rico (narrativo):**
   ```bash
   wtb doc add --type savepoint \
     --title "Savepoint YYYY-MM-DD — <contexto>" \
     --date YYYY-MM-DD \
     --content "$(cat <<'EOF'
   ## O que foi feito
   <bullet list — o que mudou neste ciclo>

   ## Pendências
   <tarefas abertas, PRs aguardando, próximos passos>

   ## Custo
   <valor do dia em USD>

   ## Artefatos
   <IDs de docs relevantes, PRs, tickets>
   EOF
   )"
   ```

3. Após concluir, exibir:
   - ID do savepoint técnico (cycle-check)
   - ID do savepoint rico (doc add)
   - Pendências que sobram para a próxima sessão
