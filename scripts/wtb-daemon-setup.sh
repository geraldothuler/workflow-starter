#!/usr/bin/env bash
# wtb-daemon-setup.sh — registra wtb serve como daemon launchd + configura MCP global
#
# Após rodar este script, o wtb estará disponível como MCP em qualquer sessão
# do Claude Code, em qualquer diretório.
#
# Usage:
#   ./scripts/wtb-daemon-setup.sh [--wtb-bin <path>] [--repo-root <path>] [--port <port>]
#   ./scripts/wtb-daemon-setup.sh --uninstall

set -euo pipefail

WTB_BIN="${WTB_BIN:-$(command -v wtb 2>/dev/null || echo "")}"
REPO_ROOT="${WTB_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
MCP_PORT="7654"
PLIST_LABEL="com.workflow.wtb-daemon"
PLIST_PATH="$HOME/Library/LaunchAgents/$PLIST_LABEL.plist"
CLAUDE_SETTINGS="$HOME/.claude/settings.json"
UNINSTALL=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --wtb-bin)    WTB_BIN="$2"; shift 2 ;;
    --repo-root)  REPO_ROOT="$2"; shift 2 ;;
    --port)       MCP_PORT="$2"; shift 2 ;;
    --uninstall)  UNINSTALL=true; shift ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

log()  { echo "[wtb-setup] $*"; }
ok()   { echo "[wtb-setup] ✓ $*"; }
warn() { echo "[wtb-setup] ⚠ $*" >&2; }

# ── Uninstall ─────────────────────────────────────────────────────────────────
if $UNINSTALL; then
  if launchctl list | grep -q "$PLIST_LABEL" 2>/dev/null; then
    launchctl unload "$PLIST_PATH" 2>/dev/null || true
    ok "daemon descarregado"
  fi
  rm -f "$PLIST_PATH"
  ok "plist removido: $PLIST_PATH"
  log "MCP config em $CLAUDE_SETTINGS não foi alterada — remova manualmente a entrada \"workflow\" se desejar."
  exit 0
fi

# ── Validações ────────────────────────────────────────────────────────────────
if [[ -z "$WTB_BIN" ]]; then
  warn "wtb não encontrado no PATH. Compile primeiro:"
  warn "  go build -o ~/bin/wtb ./cmd/wtb"
  warn "Ou informe o path: --wtb-bin /caminho/para/wtb"
  exit 1
fi

if [[ ! -x "$WTB_BIN" ]]; then
  warn "Binário não executável: $WTB_BIN"
  exit 1
fi

if [[ ! -d "$REPO_ROOT" ]]; then
  warn "Repo root não existe: $REPO_ROOT"
  warn "Informe com --repo-root ou exporte WTB_REPO_ROOT"
  exit 1
fi

log "Configurando wtb daemon"
log "  Binário  : $WTB_BIN"
log "  Repo root: $REPO_ROOT"
log "  MCP port : $MCP_PORT"

# ── Criar log dir ─────────────────────────────────────────────────────────────
mkdir -p "$HOME/.wtb"

# ── Criar plist ───────────────────────────────────────────────────────────────
cat > "$PLIST_PATH" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>$PLIST_LABEL</string>

    <key>ProgramArguments</key>
    <array>
        <string>$WTB_BIN</string>
        <string>serve</string>
        <string>--daemon</string>
        <string>--mcp-port</string>
        <string>$MCP_PORT</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
        <key>WTB_REPO_ROOT</key>
        <string>$REPO_ROOT</string>
        <key>HOME</key>
        <string>$HOME</string>
        <key>PATH</key>
        <string>$(dirname "$WTB_BIN"):/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>

    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>$HOME/.wtb/daemon.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/.wtb/daemon.log</string>

    <key>ThrottleInterval</key>
    <integer>10</integer>
</dict>
</plist>
PLIST

ok "plist criado: $PLIST_PATH"

# ── Carregar daemon ───────────────────────────────────────────────────────────
if launchctl list | grep -q "$PLIST_LABEL" 2>/dev/null; then
  launchctl unload "$PLIST_PATH" 2>/dev/null || true
  log "daemon anterior descarregado"
fi

launchctl load "$PLIST_PATH"
sleep 1

if launchctl list | grep -q "$PLIST_LABEL"; then
  ok "daemon iniciado (launchd)"
else
  warn "daemon pode não ter iniciado — verifique: tail -f ~/.wtb/daemon.log"
fi

# ── Testar MCP ────────────────────────────────────────────────────────────────
MCP_URL="http://localhost:$MCP_PORT/mcp"
sleep 1
if curl -sf --max-time 3 "$MCP_URL" -o /dev/null 2>/dev/null; then
  ok "MCP HTTP respondendo em $MCP_URL"
else
  warn "MCP HTTP ainda não responde em $MCP_URL"
  warn "Aguarde alguns segundos e teste: curl -s $MCP_URL"
fi

# ── Atualizar ~/.claude/settings.json ────────────────────────────────────────
if [[ ! -f "$CLAUDE_SETTINGS" ]]; then
  mkdir -p "$(dirname "$CLAUDE_SETTINGS")"
  echo '{}' > "$CLAUDE_SETTINGS"
fi

# Verificar se já existe a entrada "workflow"
if python3 -c "
import json, sys
with open('$CLAUDE_SETTINGS') as f:
    s = json.load(f)
sys.exit(0 if 'workflow' in s.get('mcpServers', {}) else 1)
" 2>/dev/null; then
  ok "entrada \"workflow\" já existe em $CLAUDE_SETTINGS"
else
  python3 - <<PYEOF
import json

with open('$CLAUDE_SETTINGS', 'r') as f:
    settings = json.load(f)

settings.setdefault('mcpServers', {})['workflow'] = {
    'url': 'http://localhost:$MCP_PORT/mcp'
}

with open('$CLAUDE_SETTINGS', 'w') as f:
    json.dump(settings, f, indent=2)
    f.write('\n')

print('[wtb-setup] ✓ MCP registrado em $CLAUDE_SETTINGS')
PYEOF
fi

# ── Resumo ────────────────────────────────────────────────────────────────────
echo ""
echo "Setup concluído."
echo ""
echo "  Daemon  : launchctl list | grep $PLIST_LABEL"
echo "  MCP URL : $MCP_URL"
echo "  Logs    : tail -f ~/.wtb/daemon.log"
echo ""
echo "Reinicie o Claude Code para carregar o MCP na sessão."
echo ""
echo "Para desinstalar: ./scripts/wtb-daemon-setup.sh --uninstall"
