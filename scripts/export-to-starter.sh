#!/usr/bin/env bash
# export-to-starter.sh — exporta workflow pessoal → workflow-starter (repo público)
#
# Copia apenas os arquivos permitidos (allowlist), stripa referências Cobli,
# e faz push para o repo público.
#
# Usage:
#   ./scripts/export-to-starter.sh [--target <path>] [--push] [--dry-run]
#   --target  : path local do workflow-starter (default: ~/workflow-starter)
#   --push    : faz git push após exportar
#   --dry-run : mostra o que seria copiado sem fazer nada
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${WORKFLOW_STARTER_PATH:-$HOME/workflow-starter}"
DRY_RUN=false
DO_PUSH=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --target)   TARGET="$2"; shift 2 ;;
    --push)     DO_PUSH=true; shift ;;
    --dry-run)  DRY_RUN=true; shift ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

log() { echo "[export] $*"; }

copy() {
  local src="$REPO_ROOT/$1" dst="$TARGET/$1"
  if [[ ! -e "$src" ]]; then
    log "SKIP (não existe): $1"
    return
  fi
  if $DRY_RUN; then
    log "DRY: $1"
    return
  fi
  mkdir -p "$(dirname "$dst")"
  if [[ -d "$src" ]]; then
    rsync -a --delete "$src/" "$dst/"
  else
    cp "$src" "$dst"
  fi
  log "OK: $1"
}

# ── Sanitização — remove referências Cobli/pessoais ──────────────────────────
sanitize() {
  local file="$TARGET/$1"
  [[ -f "$file" ]] || return 0
  $DRY_RUN && return 0
  sed -i '' \
    -e 's/geraldo\.thuler@cobli\.co/<your-email>/g' \
    -e 's/geraldo\.thuler/<your-username>/g' \
    -e 's/<Your Name>/<Your Name>/g' \
    -e 's/<Your Company>/<Your Company>/g' \
    -e 's|||g' \
    -e 's|~/|~/projects/|g' \
    -e 's/<your-slack-team-id>/<your-slack-team-id>/g' \
    -e 's|911383825788\.dkr\.ecr\.us-east-1\.amazonaws\.com|<your-ecr-registry>|g' \
    "$file"
}

# ── Validações ────────────────────────────────────────────────────────────────
if [[ ! -d "$TARGET" ]]; then
  log "Target não existe: $TARGET"
  log "Clone o repo público primeiro: git clone git@github.com:<org>/workflow-starter $TARGET"
  exit 1
fi

if [[ ! -d "$TARGET/.git" ]]; then
  log "Target não é um repositório git: $TARGET"
  exit 1
fi

log "Exportando $REPO_ROOT → $TARGET"
$DRY_RUN && log "(dry-run mode — nada será alterado)"

# ── ALLOWLIST — o que vai para o starter ─────────────────────────────────────

# Skills genéricas (sem contexto Cobli)
SKILLS_GENERIC=(
  ai-radar
  agent-browser
  chrome-devtools
  daily
  estimate
  grill-me
  health-check
  onboarding-doc
  pdf
  playwright
  playwright-local-setup
  pod-cleanup
  postmortem
  pr-comment
  promote-skill
  rds-pg-eval
  repo-doc
  repo-guide
  savepoint
  tdd
  workflow-conventions
)

for skill in "${SKILLS_GENERIC[@]}"; do
  copy ".claude/skills/$skill"
done

# CLAUDE.md sanitizado
copy "CLAUDE.md"
sanitize "CLAUDE.md"

# Memory — apenas arquivos de metodologia (sem dados operacionais)
MEMORY_GENERIC=(
  feedback_autonomy_gates.md
  feedback_check_memory_before_asking.md
  feedback_code_comments_english.md
  feedback_communication_language.md
  feedback_grill_me.md
  feedback_no_preexisting_excuse.md
  feedback_pdf_skill_mandatory.md
  feedback_pdf_tables.md
  feedback_savepoint_cli.md
  feedback_search_protocol.md
  feedback_slack_draft_clipboard.md
  feedback_socratic_standard.md
  feedback_draft_persistence.md
  socratic_example.md
  git-worktrees.md
)

for f in "${MEMORY_GENERIC[@]}"; do
  copy ".claude/memory/$f"
  sanitize ".claude/memory/$f"
done

# Docs de metodologia (sem discoveries operacionais)
copy "docs/workflow/platform"
sanitize "docs/workflow/platform/REFERENCE.md"

# Scripts genéricos
SCRIPTS_GENERIC=(
  memory-observer.sh
  db-backup.sh
  export-to-starter.sh
)

for s in "${SCRIPTS_GENERIC[@]}"; do
  copy "scripts/$s"
  sanitize "scripts/$s"
done

# ── Código-fonte do wtb CLI ───────────────────────────────────────────────────
if $DRY_RUN; then
  log "DRY: cmd/ pkg/ go.mod go.sum doc-types.yml"
else
  rsync -a --delete \
    --exclude='*.db' \
    --exclude='*.duckdb' \
    --exclude='session.yml' \
    "$REPO_ROOT/cmd/" "$TARGET/cmd/"
  rsync -a --delete \
    --exclude='*.db' \
    --exclude='*.duckdb' \
    "$REPO_ROOT/pkg/" "$TARGET/pkg/"
  cp "$REPO_ROOT/go.mod"      "$TARGET/go.mod"
  cp "$REPO_ROOT/go.sum"      "$TARGET/go.sum"
  cp "$REPO_ROOT/pkg/docstore/doc-types.yml" "$TARGET/pkg/docstore/doc-types.yml" 2>/dev/null || true
  log "OK: cmd/ pkg/ go.mod go.sum"
fi

# ── repos.duckdb — índice completo de repos ───────────────────────────────────
if $DRY_RUN; then
  log "DRY: repos.duckdb"
elif [[ -f "$REPO_ROOT/repos.duckdb" ]]; then
  cp "$REPO_ROOT/repos.duckdb" "$TARGET/repos.duckdb"
  log "OK: repos.duckdb"
fi

# ── docs.db — apenas templates (sem dados operacionais pessoais) ──────────────
if $DRY_RUN; then
  log "DRY: docs.db (somente type=template)"
elif [[ -f "$REPO_ROOT/docs.db" ]]; then
  STARTER_DOCS="$TARGET/docs.db"
  # Cria nova DB com apenas os templates
  rm -f "$STARTER_DOCS"
  sqlite3 "$REPO_ROOT/docs.db" \
    "ATTACH '$STARTER_DOCS' AS starter;
     CREATE TABLE starter.documents AS SELECT * FROM documents WHERE type='template' AND deleted_at='';
     CREATE INDEX starter.idx_documents_type     ON documents(type);
     CREATE INDEX starter.idx_documents_doc_date ON documents(doc_date);
     CREATE INDEX starter.idx_documents_repo     ON documents(repo);
     CREATE VIRTUAL TABLE starter.documents_fts USING fts5(id, title, content, tags, content=documents, content_rowid=rowid);
     INSERT INTO starter.documents_fts(rowid, id, title, content, tags)
       SELECT rowid, id, title, content, tags FROM starter.documents;
     DETACH starter;" 2>/dev/null || {
    # Fallback: sqlite3 pode não ter suporte a fts5 com ATTACH — usar export/import
    log "WARN: ATTACH não suportou fts5 — usando dump/restore"
    sqlite3 "$REPO_ROOT/docs.db" \
      "SELECT 'INSERT INTO documents VALUES(' ||
        quote(id)||','||quote(type)||','||quote(title)||','||quote(doc_date)||','||
        quote(repo)||','||quote(tags)||','||quote(content)||','||quote(deleted_at)||','||
        quote(created_at)||','||quote(updated_at)||');'
       FROM documents WHERE type='template' AND deleted_at='';" > /tmp/wtb-templates.sql
    # Schema mínimo para o starter
    cat > /tmp/wtb-docs-schema.sql <<'SCHEMA'
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS documents (
  id TEXT PRIMARY KEY, type TEXT NOT NULL, title TEXT NOT NULL,
  doc_date TEXT NOT NULL DEFAULT '', repo TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '',
  deleted_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_documents_type     ON documents(type);
CREATE INDEX IF NOT EXISTS idx_documents_doc_date ON documents(doc_date);
CREATE INDEX IF NOT EXISTS idx_documents_repo     ON documents(repo);
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
  id, title, content, tags, content=documents, content_rowid=rowid
);
CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
  INSERT INTO documents_fts(rowid, id, title, content, tags) VALUES (new.rowid, new.id, new.title, new.content, new.tags);
END;
CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
  INSERT INTO documents_fts(documents_fts, rowid, id, title, content, tags) VALUES ('delete', old.rowid, old.id, old.title, old.content, old.tags);
END;
CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
  INSERT INTO documents_fts(documents_fts, rowid, id, title, content, tags) VALUES ('delete', old.rowid, old.id, old.title, old.content, old.tags);
  INSERT INTO documents_fts(rowid, id, title, content, tags) VALUES (new.rowid, new.id, new.title, new.content, new.tags);
END;
SCHEMA
    sqlite3 "$STARTER_DOCS" < /tmp/wtb-docs-schema.sql
    sqlite3 "$STARTER_DOCS" < /tmp/wtb-templates.sql
    rm -f /tmp/wtb-templates.sql /tmp/wtb-docs-schema.sql
  }
  log "OK: docs.db ($(sqlite3 "$STARTER_DOCS" 'SELECT COUNT(*) FROM documents;') templates)"
fi

# ── backlog.db — scaffold vazio ───────────────────────────────────────────────
if $DRY_RUN; then
  log "DRY: backlog.db (vazio)"
else
  STARTER_BACKLOG="$TARGET/backlog.db"
  # Exporta só o schema, sem dados
  sqlite3 "$REPO_ROOT/backlog.db" ".schema" | sqlite3 "$STARTER_BACKLOG" 2>/dev/null || true
  log "OK: backlog.db (scaffold vazio)"
fi

# ── Git commit no target ──────────────────────────────────────────────────────
if ! $DRY_RUN; then
  cd "$TARGET"
  if git status --porcelain | grep -q .; then
    EXPORT_DATE=$(date +%Y-%m-%d)
    SOURCE_SHA=$(git -C "$REPO_ROOT" rev-parse --short HEAD)
    git add -A
    git commit -m "chore: sync from workflow pessoal @ $SOURCE_SHA ($EXPORT_DATE)"
    log "Commit criado no target."

    if $DO_PUSH; then
      git push origin main
      log "Push feito."
    else
      log "Rode com --push para fazer push, ou: cd $TARGET && git push origin main"
    fi
  else
    log "Nenhuma mudança detectada no target."
  fi
fi

log "Concluído."
