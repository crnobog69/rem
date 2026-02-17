#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

if ! command -v tar >/dev/null 2>&1; then
  echo "tar is required" >&2
  exit 1
fi

REPO="${REM_UPDATE_REPO:-}"
if [[ -z "${REPO}" ]]; then
  echo "Set REM_UPDATE_REPO=owner/repo before running install script." >&2
  exit 1
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported arch: ${ARCH}" >&2
    exit 1
    ;;
esac

case "${OS}" in
  linux) BIN_NAME="rem-linux-${ARCH}" ;;
  *)
    echo "Unsupported OS: ${OS}" >&2
    exit 1
    ;;
esac

URL="https://github.com/${REPO}/releases/latest/download/${BIN_NAME}"
DEST_DIR="${HOME}/.local/bin"
DEST="${DEST_DIR}/rem"

mkdir -p "${DEST_DIR}"
curl -fsSL "${URL}" -o "${DEST}"
chmod +x "${DEST}"

echo "Installed rem to ${DEST}"

