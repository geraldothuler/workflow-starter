#!/usr/bin/env bash
# db-backup.sh — health check dos SQLite + backup antes do savepoint
#
# Uso:
#   bash ~/workflow/scripts/db-backup.sh
#
# Backup é sobrescrito a cada savepoint (ponto de rollback de sessão).
# Se health check falhar: imprime erro e sai com código 1 — NÃO prosseguir.
#
# Chamado pelo Session Exit Rule antes de wtb cycle-check --save.

set -euo pipefail

WORKFLOW_PATH="${WORKFLOW_PATH:-$HOME/workflow}"
BACKUP_DIR="$WORKFLOW_PATH/savepoints/backups"
DBS=("backlog.db" "docs.db")

mkdir -p "$BACKUP_DIR"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  DB Health Check + Backup                                    ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

FAILED=0

for DB in "${DBS[@]}"; do
    DB_PATH="$WORKFLOW_PATH/$DB"
    BAK_PATH="$BACKUP_DIR/${DB}.bak"

    if [[ ! -f "$DB_PATH" ]]; then
        echo "  ⚠  $DB — não encontrado, ignorando"
        continue
    fi

    # Health check via PRAGMA integrity_check
    printf "  %-14s integrity_check ... " "$DB"
    RESULT=$(sqlite3 "$DB_PATH" "PRAGMA integrity_check;" 2>&1 || true)

    if [[ "$RESULT" == "ok" ]]; then
        echo "✔ ok"
    else
        echo "✗ FALHOU"
        echo "     └─ $RESULT"
        FAILED=1
    fi

    # Backup apenas se saudável
    if [[ $FAILED -eq 0 ]]; then
        # sqlite3 .backup usa a API de backup do SQLite (consistente, sem lock)
        sqlite3 "$DB_PATH" ".backup '$BAK_PATH'" 2>/dev/null \
            || cp "$DB_PATH" "$BAK_PATH"
        SIZE=$(du -sh "$BAK_PATH" 2>/dev/null | cut -f1)
        echo "             backup  → savepoints/backups/${DB}.bak  ($SIZE)"
    else
        echo "             backup  → CANCELADO (DB corrompido)"
    fi
done

echo ""

if [[ $FAILED -ne 0 ]]; then
    echo "❌  Health check falhou — NÃO prosseguir com savepoint."
    echo "    Investigar corrupção antes de sobrescrever backup."
    exit 1
fi

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
echo "✓  Backup concluído — $TIMESTAMP"
echo "   Para restaurar: cp savepoints/backups/<db>.bak <db>"
