#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

REPO="${REM_UPDATE_REPO:-crnobog69/rem}"
REF="${REM_UPDATE_REF:-master}"

if [[ -n "${BASH_SOURCE[0]:-}" && -f "${BASH_SOURCE[0]}" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  if [[ -f "${SCRIPT_DIR}/install.sh" ]]; then
    "${SCRIPT_DIR}/install.sh"
    exit 0
  fi
fi

curl -fsSL "https://raw.githubusercontent.com/${REPO}/${REF}/scripts/install.sh" | bash
