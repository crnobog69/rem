#!/usr/bin/env bash
set -euo pipefail

VERSION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "${VERSION}" ]]; then
  echo "Usage: ./scripts/release.sh --version vX.Y.Z" >&2
  exit 1
fi

mkdir -p dist
rm -f dist/rem-* dist/checksums.txt

LDFLAGS="-s -w -X main.version=${VERSION}"

build() {
  local goos="$1"
  local goarch="$2"
  local out="$3"
  echo "Building ${out}"
  GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 \
    go build -trimpath -ldflags "${LDFLAGS}" -o "${out}" ./cmd/rem
}

build linux amd64 dist/rem-linux-amd64
build linux arm64 dist/rem-linux-arm64
build windows amd64 dist/rem-windows-amd64.exe
build windows arm64 dist/rem-windows-arm64.exe

(
  cd dist
  sha256sum rem-* > checksums.txt
)

echo "Done. Artifacts are in dist/"

