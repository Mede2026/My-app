#!/usr/bin/env bash
# Construit l'exécutable Windows depuis n'importe quel système (macOS, Linux).
#
# C'est un des avantages de Go : la compilation croisée est intégrée, aucun
# outil supplémentaire n'est nécessaire.
#
#   ./scripts/build.sh                 -> dist/CryptoBulle.exe (Windows 64 bits)
#   ./scripts/build.sh arm64           -> dist/CryptoBulle-arm64.exe (Windows sur ARM)
#   VERSION=3.0.1 ./scripts/build.sh   -> numéro de version inscrit dans le binaire
set -euo pipefail

cd "$(dirname "$0")/.."
arch="${1:-amd64}"
version="${VERSION:-}"
# Variantes de mise à jour :
#   (rien)            l'application prévient et ouvre la page de téléchargement
#   TAGS=autoupdate   elle télécharge et se remplace toute seule
#   TAGS=noupdate     elle ne vérifie rien du tout
tags="${TAGS:-}"

output="dist/CryptoBulle.exe"
[ "$arch" = "amd64" ] || output="dist/CryptoBulle-$arch.exe"
case "$tags" in
	autoupdate) output="dist/CryptoBulle-maj-auto.exe" ;;
	noupdate) output="dist/CryptoBulle-sans-maj.exe" ;;
esac

# -H windowsgui : pas de fenêtre noire au lancement.
# -s -w         : on retire les tables de débogage, l'exécutable est plus petit.
flags="-s -w -H windowsgui"
if [ -n "$version" ]; then
	flags="$flags -X github.com/mede2026/cryptobulle/internal/app.appVersion=$version"
fi

mkdir -p dist
GOOS=windows GOARCH="$arch" go build -trimpath -tags="$tags" -ldflags="$flags" \
	-o "$output" ./cmd/cryptobulle

printf 'Terminé : %s (%s Ko)\n' "$output" "$(( $(wc -c < "$output") / 1024 ))"
