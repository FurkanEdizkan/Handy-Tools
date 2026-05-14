#!/usr/bin/env sh
# Handy Tools installer — downloads the latest (or pinned) release from GitHub,
# verifies the checksum, drops `htools` and `htoolsd` into an install dir, and
# optionally installs the small set of optional system tools Handy Tools uses.
#
#   curl -fsSL https://raw.githubusercontent.com/furkandedizkan/handy-tools/main/install.sh | sh
#
# Knobs (env or flag):
#   HANDY_TOOLS_VERSION=0.2.0          # pin a specific version (default: latest)
#   HANDY_TOOLS_INSTALL_DIR=$HOME/...  # where to put binaries (default: $HOME/.local/bin)
#   HANDY_TOOLS_INSTALL_DEPS=1         # also install optional system tools (apt/dnf/pacman/brew)
#   NO_COLOR=1                         # disable ANSI colors
#   --version 0.2.0 / --dir PATH / --install-deps / --yes / --no-color / --help
#
# This script is POSIX sh. It targets Linux and macOS. Windows is not yet
# supported.

set -eu

REPO="furkandedizkan/handy-tools"
PROJECT_NAME="handy-tools"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"

VERSION="${HANDY_TOOLS_VERSION:-}"
INSTALL_DIR="${HANDY_TOOLS_INSTALL_DIR:-}"
INSTALL_DEPS="${HANDY_TOOLS_INSTALL_DEPS:-0}"
ASSUME_YES=0
USE_COLOR=1

# ---- color helpers --------------------------------------------------------
# Orange-and-black mascot palette; gated on TTY + NO_COLOR + TERM.
if [ -n "${NO_COLOR:-}" ] || [ ! -t 1 ] || [ "${TERM:-dumb}" = "dumb" ]; then
  USE_COLOR=0
fi

color() {
  # $1 = ansi code, $2... = text
  code=$1; shift
  if [ "$USE_COLOR" = "1" ]; then
    printf '\033[%sm%s\033[0m' "$code" "$*"
  else
    printf '%s' "$*"
  fi
}
orange() { color '38;5;208' "$*"; }
amber()  { color '38;5;214' "$*"; }
dim()    { color '2'        "$*"; }
bold()   { color '1'        "$*"; }
green()  { color '32'       "$*"; }
red()    { color '31'       "$*"; }

# ---- banner ---------------------------------------------------------------
banner() {
  [ "$USE_COLOR" = "1" ] || { printf 'Handy Tools installer\n\n'; return; }
  cat <<EOF

$(orange '   /\___/\')      $(bold "$(orange 'Handy Tools')")  $(dim 'one-line installer')
$(orange '  ( ')$(amber 'o')$(orange ' . ')$(amber 'o')$(orange ' )')    $(dim "$REPO")
$(orange '   \  ')$(amber 'v')$(orange '  /')
$(orange "    '---'")

EOF
}

usage() {
  banner
  cat <<'EOF'
Handy Tools installer — downloads the latest (or pinned) release from GitHub,
verifies the checksum, drops `htools` and `htoolsd` into an install dir, and
optionally installs the small set of optional system tools Handy Tools uses.

  curl -fsSL https://raw.githubusercontent.com/furkandedizkan/handy-tools/main/install.sh | sh

Knobs (env or flag):
  HANDY_TOOLS_VERSION=0.2.0          # pin a specific version (default: latest)
  HANDY_TOOLS_INSTALL_DIR=$HOME/...  # where to put binaries (default: $HOME/.local/bin)
  HANDY_TOOLS_INSTALL_DEPS=1         # also install optional system tools (apt/dnf/pacman/brew)
  NO_COLOR=1                         # disable ANSI colors
  --version 0.2.0 / --dir PATH / --install-deps / --yes / --no-color / --help

Targets Linux and macOS. Windows is not yet supported.
EOF
  exit 0
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --version=*) VERSION="${1#--version=}"; shift ;;
    --dir) INSTALL_DIR="$2"; shift 2 ;;
    --dir=*) INSTALL_DIR="${1#--dir=}"; shift ;;
    --install-deps) INSTALL_DEPS=1; shift ;;
    --yes|-y) ASSUME_YES=1; shift ;;
    --no-color) USE_COLOR=0; shift ;;
    --help|-h) usage ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

log()  { printf '%s %s\n' "$(orange '==>')" "$*"; }
warn() { printf '%s %s\n' "$(amber 'warn:')" "$*" >&2; }
die()  { printf '%s %s\n' "$(red 'error:')" "$*" >&2; exit 1; }
ok()   { printf '%s %s\n' "$(green ' ok ')" "$*"; }

banner

require() {
  command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"
}

require uname
require tar
# curl OR wget will work for downloads; we pick whichever is present.
if command -v curl >/dev/null 2>&1; then
  DL='curl -fsSL'
  DLO='curl -fsSL -o'
elif command -v wget >/dev/null 2>&1; then
  DL='wget -qO-'
  DLO='wget -qO'
else
  die "neither curl nor wget is installed"
fi

# ---- detect OS / arch ------------------------------------------------------
uname_s=$(uname -s)
uname_m=$(uname -m)

case "$uname_s" in
  Linux)  OS=linux ;;
  Darwin) OS=darwin ;;
  *)      die "unsupported OS: $uname_s (Handy Tools currently supports Linux and macOS)" ;;
esac

case "$uname_m" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) die "unsupported arch: $uname_m" ;;
esac

# ---- resolve version -------------------------------------------------------
if [ -z "$VERSION" ]; then
  log "looking up latest release of $REPO"
  api="https://api.github.com/repos/${REPO}/releases/latest"
  # parse tag_name without jq: grep + sed for the first occurrence.
  tag=$($DL "$api" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' || true)
  [ -n "$tag" ] || die "could not determine latest release (rate limited? set HANDY_TOOLS_VERSION explicitly)"
  VERSION="${tag#v}"
fi

case "$VERSION" in
  v*) VERSION="${VERSION#v}" ;;
esac

log "installing $(bold "$(orange "Handy Tools v${VERSION}")") for $(amber "${OS}/${ARCH}")"

# ---- download & verify -----------------------------------------------------
asset="${PROJECT_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
base="https://github.com/${REPO}/releases/download/v${VERSION}"
asset_url="${base}/${asset}"
sums_url="${base}/checksums.txt"

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t handy-tools-install)
trap 'rm -rf "$tmp"' EXIT INT TERM

log "downloading $asset"
$DLO "$tmp/$asset" "$asset_url" || die "download failed: $asset_url"

log "downloading checksums.txt"
$DLO "$tmp/checksums.txt" "$sums_url" || die "checksums download failed"

# Pick whichever sha256 tool exists. macOS ships shasum; most Linux ships sha256sum.
if command -v sha256sum >/dev/null 2>&1; then
  SHA="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA="shasum -a 256"
else
  die "no sha256 tool found (need sha256sum or shasum)"
fi

expected=$(grep "  ${asset}\$" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$expected" ] || die "no checksum entry for $asset in checksums.txt"
got=$(cd "$tmp" && $SHA "$asset" | awk '{print $1}')
[ "$expected" = "$got" ] || die "checksum mismatch: expected $expected, got $got"
ok "checksum verified"

# ---- install ---------------------------------------------------------------
mkdir -p "$INSTALL_DIR"
( cd "$tmp" && tar -xzf "$asset" )

# The tar produced by goreleaser puts htools / htoolsd at the archive root.
for bin in htools htoolsd; do
  [ -f "$tmp/$bin" ] || die "expected $bin in archive but it was missing"
  install -m 0755 "$tmp/$bin" "$INSTALL_DIR/$bin"
  ok "installed $(amber "$INSTALL_DIR/$bin")"
done

# Warn if the install dir is not on PATH.
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) warn "$INSTALL_DIR is not on your PATH; add it to your shell rc:
     export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac

# ---- optional system tool detection / install -----------------------------
# Source of truth for these names is internal/tools/sysdep/sysdep.go.
# Keep this list in sync when adding new optional tools.
needed_for() {
  case "$1" in
    unrar)     echo "RAR archive extraction (incl. multi-part .partN.rar)" ;;
    7z)        echo "7z multi-part extraction" ;;
    pdftoppm)  echo "render PDF pages to images" ;;
    pdftotext) echo "extract text from PDFs" ;;
    magick)    echo "decode HEIC/HEIF images" ;;
    *)         echo "" ;;
  esac
}

ALL_TOOLS="unrar 7z pdftoppm pdftotext magick"
missing=""
for t in $ALL_TOOLS; do
  if ! command -v "$t" >/dev/null 2>&1; then
    # 7z has aliases (7zz, 7za); unrar has unrar-free.
    case "$t" in
      7z)    command -v 7zz >/dev/null 2>&1 || command -v 7za >/dev/null 2>&1 && continue ;;
      unrar) command -v unrar-free >/dev/null 2>&1 && continue ;;
    esac
    missing="$missing $t"
  fi
done
missing=${missing# }

if [ -z "$missing" ]; then
  ok "all optional system tools are already installed"
else
  echo
  echo "$(amber 'Optional system tools NOT installed') (Handy Tools will work; affected features will be disabled):"
  for t in $missing; do
    printf '  %s %-10s %s\n' "$(dim '-')" "$(orange "$t")" "$(needed_for "$t")"
  done
fi

# Detect package manager for install hints / auto-install.
PM=""
PM_INSTALL=""
PM_PKGS=""
if [ "$OS" = "darwin" ] && command -v brew >/dev/null 2>&1; then
  PM=brew; PM_INSTALL="brew install"
elif command -v apt-get >/dev/null 2>&1; then
  PM=apt;  PM_INSTALL="sudo apt-get install -y"
elif command -v dnf >/dev/null 2>&1; then
  PM=dnf;  PM_INSTALL="sudo dnf install -y"
elif command -v pacman >/dev/null 2>&1; then
  PM=pacman; PM_INSTALL="sudo pacman -S --noconfirm"
fi

# Map handy-tool -> package name per package manager.
pkg_for() {
  pm=$1; tool=$2
  case "$pm:$tool" in
    apt:unrar)         echo "unrar-free" ;;
    apt:7z)            echo "p7zip-full" ;;
    apt:pdftoppm|apt:pdftotext) echo "poppler-utils" ;;
    apt:magick)        echo "imagemagick" ;;
    dnf:unrar)         echo "unrar" ;;
    dnf:7z)            echo "p7zip p7zip-plugins" ;;
    dnf:pdftoppm|dnf:pdftotext) echo "poppler-utils" ;;
    dnf:magick)        echo "ImageMagick" ;;
    pacman:unrar)      echo "unrar" ;;
    pacman:7z)         echo "p7zip" ;;
    pacman:pdftoppm|pacman:pdftotext) echo "poppler" ;;
    pacman:magick)     echo "imagemagick" ;;
    brew:unrar)        echo "unrar" ;;
    brew:7z)           echo "p7zip" ;;
    brew:pdftoppm|brew:pdftotext) echo "poppler" ;;
    brew:magick)       echo "imagemagick" ;;
    *) echo "" ;;
  esac
}

if [ -n "$missing" ]; then
  if [ -n "$PM" ]; then
    seen=""
    for t in $missing; do
      pkgs=$(pkg_for "$PM" "$t")
      [ -z "$pkgs" ] && continue
      for p in $pkgs; do
        case " $seen " in *" $p "*) ;; *) seen="$seen $p" ;; esac
      done
    done
    seen=${seen# }
    cmd="$PM_INSTALL $seen"
    PM_PKGS="$seen"

    echo
    echo "Detected package manager: $(amber "$PM")"
    echo "To install everything missing in one go:"
    echo "    $(orange "$cmd")"

    if [ "$INSTALL_DEPS" = "1" ]; then
      run=0
      if [ "$ASSUME_YES" = "1" ]; then
        run=1
      else
        printf '%s ' "$(amber 'Run it now? [y/N]')"
        read -r ans </dev/tty || ans=""
        case "$ans" in y|Y|yes|YES) run=1 ;; esac
      fi
      if [ "$run" = "1" ]; then
        log "running: $cmd"
        # shellcheck disable=SC2086
        sh -c "$cmd"
      else
        echo "Skipped — run the command above when you're ready."
      fi
    fi
  else
    echo
    echo "No supported package manager detected (apt/dnf/pacman/brew). Install the missing tools manually."
  fi
fi

echo
log "done. Try: $(orange "$INSTALL_DIR/htools --version")"
