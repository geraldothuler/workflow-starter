#!/usr/bin/env bash
# secret-set.sh — cross-platform credential store
# Usage: bash scripts/secret-set.sh <service-key> <value> [account]
#
# Stores in the first available backend:
#   1. macOS Keychain (security)  — account = $USER
#   2. Linux Secret Service (secret-tool / GNOME Keyring / KDE Wallet)
#   3. GNU pass  — path: wtb/<key-without-workflow->
#
# Exit 0 on success; exit 1 if no backend available.
set -euo pipefail

KEY="${1:?Usage: secret-set.sh <key> <value> [account]}"
VALUE="${2:?secret-set.sh requires a value}"
ACCOUNT="${3:-${USER:-workflow}}"

# 1. macOS Keychain
if command -v security &>/dev/null; then
  security add-generic-password -U -s "$KEY" -a "$ACCOUNT" -w "$VALUE" 2>/dev/null \
    && echo "Stored in macOS Keychain: $KEY" && exit 0
fi

# 2. Linux Secret Service
if command -v secret-tool &>/dev/null; then
  echo -n "$VALUE" | secret-tool store --label "$KEY" service "$KEY" account workflow 2>/dev/null \
    && echo "Stored in GNOME Keyring: $KEY" && exit 0
fi

# 3. GNU pass
if command -v pass &>/dev/null; then
  PASS_PATH="wtb/${KEY#workflow-}"
  printf '%s\n%s\n' "$VALUE" "$VALUE" | pass insert -f "$PASS_PATH" &>/dev/null \
    && echo "Stored in pass: $PASS_PATH" && exit 0
fi

echo "secret-set: no backend available (tried: security, secret-tool, pass)" >&2
echo "Hint — install one of:" >&2
echo "  macOS: built-in (security command missing — unexpected)" >&2
echo "  Ubuntu: apt install libsecret-tools   # secret-tool" >&2
echo "  Any:   apt install pass && pass init <gpg-id>   # GNU pass" >&2
echo "Or set env var: WORKFLOW_SECRET_${KEY//-/_}" >&2
exit 1
