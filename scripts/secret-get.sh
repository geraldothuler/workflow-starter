#!/usr/bin/env bash
# secret-get.sh — cross-platform credential lookup
# Usage: bash scripts/secret-get.sh <service-key> [account]
#
# Resolution chain (stops at first hit):
#   1. macOS Keychain (security)  — account = $USER
#   2. Linux Secret Service (secret-tool / GNOME Keyring / KDE Wallet)
#   3. GNU pass  — path: wtb/<key-without-workflow->
#   4. Environment variable  — WORKFLOW_SECRET_<KEY_UPPER>  or  <KEY_UPPER>
#
# Exit 0 on success (prints value); exit 1 if not found.
set -euo pipefail

KEY="${1:?Usage: secret-get.sh <key> [account]}"
ACCOUNT="${2:-${USER:-workflow}}"

# 1. macOS Keychain
if command -v security &>/dev/null; then
  val=$(security find-generic-password -s "$KEY" -a "$ACCOUNT" -w 2>/dev/null) \
    && [[ -n "$val" ]] && echo "$val" && exit 0
fi

# 2. Linux Secret Service (GNOME Keyring / KDE Wallet)
if command -v secret-tool &>/dev/null; then
  val=$(secret-tool lookup service "$KEY" account workflow 2>/dev/null) \
    && [[ -n "$val" ]] && echo "$val" && exit 0
fi

# 3. GNU pass
if command -v pass &>/dev/null; then
  PASS_PATH="wtb/${KEY#workflow-}"
  val=$(pass show "$PASS_PATH" 2>/dev/null | head -1) \
    && [[ -n "$val" ]] && echo "$val" && exit 0
fi

# 4. Environment variable
KEY_UPPER="${KEY//-/_}"
KEY_UPPER="${KEY_UPPER^^}"
for VARNAME in "WORKFLOW_SECRET_${KEY_UPPER}" "$KEY_UPPER"; do
  if [[ -n "${!VARNAME:-}" ]]; then
    echo "${!VARNAME}"
    exit 0
  fi
done

echo "secret-get: '$KEY' not found (tried: security, secret-tool, pass, \$WORKFLOW_SECRET_${KEY_UPPER})" >&2
exit 1
