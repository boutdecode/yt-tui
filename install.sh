#!/usr/bin/env bash
# Installateur de yt-tui :  curl -fsSL https://raw.githubusercontent.com/boutdecode/yt-tui/main/install.sh | bash
#
# Variables :
#   YT_TUI_VERSION   version à installer, ex. 0.1.0 ou v0.1.0 (défaut : la dernière)
#   YT_TUI_PREFIX    préfixe d'installation (défaut : ~/.local → ~/.local/bin/yt-tui)
#   YT_TUI_BASE_URL  surcharge l'URL de téléchargement (tests / miroir)
# Option : --uninstall  supprime yt-tui du préfixe.
set -euo pipefail

REPO="boutdecode/yt-tui"
PREFIX="${YT_TUI_PREFIX:-$HOME/.local}"
VERSION="${YT_TUI_VERSION:-latest}"

say() { printf '\033[1;35m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die() { printf '\033[1;31mErreur :\033[0m %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

if [ "${1:-}" = "--uninstall" ]; then
	rm -f "$PREFIX/bin/yt-tui" "$PREFIX/share/applications/yt-tui.desktop"
	say "yt-tui supprimé de $PREFIX (config et bibliothèque conservées)."
	exit 0
fi

[ "$(uname -s)" = "Linux" ] || die "seul Linux est pris en charge."
case "$(uname -m)" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) die "architecture non prise en charge : $(uname -m) (amd64 et arm64 seulement)." ;;
esac

if have curl; then
	fetch() { curl -fsSL --retry 3 -o "$2" "$1"; }
elif have wget; then
	fetch() { wget -q -O "$2" "$1"; }
else
	die "curl ou wget est requis."
fi

if have sha256sum; then
	sha() { sha256sum "$1" | cut -d' ' -f1; }
elif have shasum; then
	sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	die "sha256sum (ou shasum) est requis pour vérifier le téléchargement."
fi

if [ -n "${YT_TUI_BASE_URL:-}" ]; then
	BASE="${YT_TUI_BASE_URL%/}"
elif [ "$VERSION" = "latest" ]; then
	BASE="https://github.com/$REPO/releases/latest/download"
else
	BASE="https://github.com/$REPO/releases/download/v${VERSION#v}"
fi

# Le nom de l'archive contient la version : on la lit dans checksums.txt si « latest ».
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

say "Téléchargement de la somme de contrôle…"
fetch "$BASE/checksums.txt" "$TMP/checksums.txt" || die "impossible de télécharger $BASE/checksums.txt"
ARCHIVE="$(awk -v a="_linux_${ARCH}.tar.gz" 'index($2, a) { print $2; exit }' "$TMP/checksums.txt")"
[ -n "$ARCHIVE" ] || die "aucune archive linux/$ARCH dans cette version."
WANT="$(awk -v f="$ARCHIVE" '$2 == f { print $1; exit }' "$TMP/checksums.txt")"

say "Téléchargement de $ARCHIVE…"
fetch "$BASE/$ARCHIVE" "$TMP/$ARCHIVE" || die "impossible de télécharger $BASE/$ARCHIVE"
[ "$(sha "$TMP/$ARCHIVE")" = "$WANT" ] || die "somme SHA256 incorrecte : téléchargement corrompu ou altéré, rien n'a été installé."

tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
[ -f "$TMP/yt-tui" ] || die "archive invalide (binaire absent)."
install -Dm755 "$TMP/yt-tui" "$PREFIX/bin/yt-tui"
if [ -f "$TMP/yt-tui.desktop" ]; then
	install -Dm644 "$TMP/yt-tui.desktop" "$PREFIX/share/applications/yt-tui.desktop"
fi
say "Installé : $PREFIX/bin/yt-tui ($("$PREFIX/bin/yt-tui" --version 2>/dev/null || echo '?'))"

case ":$PATH:" in
*":$PREFIX/bin:"*) ;;
*) warn "$PREFIX/bin n'est pas dans votre PATH. Ajoutez à ~/.bashrc ou ~/.zshrc :  export PATH=\"$PREFIX/bin:\$PATH\"" ;;
esac

# Dépendances : on indique seulement ce qui manque, sans rien installer en root.
missing=()
for c in mpv ffmpeg yt-dlp; do have "$c" || missing+=("$c"); done
if ! have deno && ! have node && ! have bun; then missing+=("un runtime JS (deno, node ou bun)"); fi
if [ "${#missing[@]}" -gt 0 ]; then
	warn "Dépendances manquantes : ${missing[*]}"
	if have apt-get; then
		warn "  Debian/Ubuntu : sudo apt install mpv ffmpeg nodejs && pipx install yt-dlp"
	elif have dnf; then
		warn "  Fedora : sudo dnf install mpv ffmpeg nodejs yt-dlp"
	elif have pacman; then
		warn "  Arch : sudo pacman -S mpv ffmpeg nodejs yt-dlp"
	fi
fi
say "Terminé. Lancez : yt-tui"
