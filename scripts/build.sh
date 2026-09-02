#!/usr/bin/env bash
# Construit l'executable Windows depuis n'importe quel systeme (macOS, Linux).
#
# C'est un des avantages de Go : la compilation croisee est integree, aucun
# outil supplementaire n'est necessaire.
#
#   ./scripts/build.sh            -> dist/CryptoBulle.exe (Windows 64 bits)
#   ./scripts/build.sh arm64      -> dist/CryptoBulle-arm64.exe (Windows sur ARM)
set -euo pipefail

cd "$(dirname "$0")/.."
arch="${1:-amd64}"
output="dist/CryptoBulle.exe"
[ "$arch" = "amd64" ] || output="dist/CryptoBulle-$arch.exe"

mkdir -p dist
GOOS=windows GOARCH="$arch" go build -trimpath \
	-ldflags="-s -w -H windowsgui" \
	-o "$output" ./cmd/cryptobulle

printf 'Termine : %s (%s Ko)\n' "$output" "$(( $(wc -c < "$output") / 1024 ))"
