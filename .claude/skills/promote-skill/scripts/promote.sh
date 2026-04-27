#!/usr/bin/env bash
# promote.sh — promove uma skill local para o agents-marketplace da Cobli
# Uso: promote.sh <skill-name> [--plan|--execute]

set -euo pipefail

SKILL_NAME="${1:-}"
MODE="${2:---plan}"
SKILLS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SKILL_DIR="${SKILLS_DIR}/${SKILL_NAME}"
MARKETPLACE_DIR="${HOME}/Cobliteam/agents-marketplace"
PLUGIN_DIR="${MARKETPLACE_DIR}/plugins/${SKILL_NAME}"

# ── Validações ──────────────────────────────────────────────────────────────

if [[ -z "$SKILL_NAME" ]]; then
  echo "Erro: nome da skill é obrigatório." >&2
  echo "Uso: promote.sh <skill-name> [--plan|--execute]" >&2
  exit 1
fi

if [[ ! -d "$SKILL_DIR" ]]; then
  echo "Erro: skill '${SKILL_NAME}' não encontrada em ${SKILLS_DIR}/" >&2
  exit 1
fi

SKILL_MD="${SKILL_DIR}/SKILL.md"
if [[ ! -f "$SKILL_MD" ]]; then
  echo "Erro: ${SKILL_MD} não existe." >&2
  exit 1
fi

# Verifica gate marketplace-ready
if ! grep -q "^marketplace-ready: true" "$SKILL_MD"; then
  echo "Gate não passado: 'marketplace-ready: true' ausente no frontmatter de ${SKILL_NAME}/SKILL.md" >&2
  echo "" >&2
  echo "Adicione ao frontmatter quando a skill estiver validada e sem dependências locais:" >&2
  echo "  marketplace-ready: true" >&2
  exit 1
fi

# Verifica que agents-marketplace existe
if [[ ! -d "$MARKETPLACE_DIR/.git" ]]; then
  echo "Erro: agents-marketplace não encontrado em ${MARKETPLACE_DIR}" >&2
  echo "Clone primeiro: gh repo clone Cobliteam/agents-marketplace ${MARKETPLACE_DIR}" >&2
  exit 1
fi

# ── Coleta metadados da skill ────────────────────────────────────────────────

SKILL_DESC=$(grep "^description:" "$SKILL_MD" | head -1 | sed 's/^description: *//' | tr -d '"')
BRANCH_NAME="feat/add-skill-${SKILL_NAME}"
HAS_SCRIPTS=$([[ -d "${SKILL_DIR}/scripts" ]] && echo "true" || echo "false")
EXISTING=$([[ -d "$PLUGIN_DIR" ]] && echo "true" || echo "false")

# ── Modo PLAN ───────────────────────────────────────────────────────────────

if [[ "$MODE" == "--plan" ]]; then
  echo "Plano de promoção: ${SKILL_NAME}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "Origem:  ${SKILL_DIR}/"
  echo "Destino: ${MARKETPLACE_DIR}/plugins/${SKILL_NAME}/"
  echo "Branch:  ${BRANCH_NAME}"
  echo "PR em:   Cobliteam/agents-marketplace"
  echo ""
  echo "Estrutura que será criada:"
  echo "  plugins/${SKILL_NAME}/"
  echo "  ├── .claude-plugin/plugin.json"
  echo "  ├── skills/${SKILL_NAME}/SKILL.md   (SKILL.md sem campos locais)"
  if [[ "$HAS_SCRIPTS" == "true" ]]; then
    echo "  ├── scripts/                       (copiados de ${SKILL_NAME}/scripts/)"
  fi
  echo "  └── README.md                       (gerado a partir do SKILL.md)"
  echo ""
  echo "marketplace.json: +1 entrada para '${SKILL_NAME}'"
  echo ""
  if [[ "$EXISTING" == "true" ]]; then
    echo "⚠ Plugin '${SKILL_NAME}' já existe no marketplace — será atualizado."
  fi
  echo "marketplace-ready: true ✓"
  exit 0
fi

# ── Modo EXECUTE ─────────────────────────────────────────────────────────────

echo "Executando promoção de '${SKILL_NAME}'..."

# Sincroniza agents-marketplace
cd "$MARKETPLACE_DIR"
git fetch origin
git checkout -b "$BRANCH_NAME" origin/main 2>/dev/null || git checkout "$BRANCH_NAME"

# Cria estrutura do plugin
mkdir -p "${PLUGIN_DIR}/.claude-plugin"
mkdir -p "${PLUGIN_DIR}/skills/${SKILL_NAME}"
[[ "$HAS_SCRIPTS" == "true" ]] && mkdir -p "${PLUGIN_DIR}/scripts"

# plugin.json
cat > "${PLUGIN_DIR}/.claude-plugin/plugin.json" <<EOF
{
  "name": "${SKILL_NAME}",
  "version": "1.0.0",
  "description": "${SKILL_DESC}"
}
EOF

# SKILL.md — copia sem campos exclusivamente locais
sed '/^marketplace-ready:/d' "$SKILL_MD" > "${PLUGIN_DIR}/skills/${SKILL_NAME}/SKILL.md"

# Scripts
if [[ "$HAS_SCRIPTS" == "true" ]]; then
  cp -r "${SKILL_DIR}/scripts/." "${PLUGIN_DIR}/scripts/"
  chmod +x "${PLUGIN_DIR}/scripts/"*.sh 2>/dev/null || true
fi

# README.md — extrai do primeiro bloco H1 + descrição do SKILL.md
SKILL_TITLE=$(grep "^# " "${PLUGIN_DIR}/skills/${SKILL_NAME}/SKILL.md" | head -1 | sed 's/^# //')
cat > "${PLUGIN_DIR}/README.md" <<EOF
# ${SKILL_TITLE}

${SKILL_DESC}

## Instalação

\`\`\`bash
/plugin install Cobliteam/agents-marketplace#${SKILL_NAME}
\`\`\`

## Uso

Skill registrada como \`/${SKILL_NAME}\` no Claude Code.
EOF

# marketplace.json — adiciona entrada se não existir
MARKETPLACE_JSON="${MARKETPLACE_DIR}/.claude-plugin/marketplace.json"
if ! python3 -c "
import json, sys
with open('${MARKETPLACE_JSON}') as f: d = json.load(f)
names = [p['name'] for p in d.get('plugins', [])]
sys.exit(0 if '${SKILL_NAME}' in names else 1)
" 2>/dev/null; then
  python3 -c "
import json
with open('${MARKETPLACE_JSON}') as f: d = json.load(f)
d['plugins'].append({
  'name': '${SKILL_NAME}',
  'source': './plugins/${SKILL_NAME}',
  'description': '${SKILL_DESC}'
})
with open('${MARKETPLACE_JSON}', 'w') as f: json.dump(d, f, indent=2, ensure_ascii=False)
print('marketplace.json atualizado')
"
else
  echo "marketplace.json: entrada '${SKILL_NAME}' já existe — mantida."
fi

# Commit
git add "plugins/${SKILL_NAME}" ".claude-plugin/marketplace.json"
git commit -m "$(cat <<COMMIT
feat(${SKILL_NAME}): add skill from workflow-toolkit

${SKILL_DESC}

Promovido de: Cobliteam/workflow-toolkit/.claude/skills/${SKILL_NAME}/

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
COMMIT
)"

# PR
git push -u origin "$BRANCH_NAME"
gh pr create \
  --repo Cobliteam/agents-marketplace \
  --title "feat(${SKILL_NAME}): add skill" \
  --body "$(cat <<PR
## O que muda

Adiciona skill \`${SKILL_NAME}\` ao marketplace.

${SKILL_DESC}

## Origem

Promovido de \`Cobliteam/workflow-toolkit/.claude/skills/${SKILL_NAME}/\` após validação em sessões reais.

## Como testar

\`\`\`bash
claude --plugin-dir ./plugins/${SKILL_NAME}
\`\`\`

## Rastreabilidade

- Origem: Cobliteam/workflow-toolkit
PR
)"

echo ""
echo "PR criado em Cobliteam/agents-marketplace."
