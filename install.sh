#!/usr/bin/env sh
# Handy Tools installer — downloads the latest (or pinned) release from GitHub,
# verifies the checksum, drops `handy` + `htools` + `htoolsd` + `htools-mcp`
# into an install dir, and optionally installs the small set of optional
# system tools Handy Tools uses.
#
#   curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh
#
# Symmetric uninstall path:
#   curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh -s -- --uninstall
#
# Knobs (env or flag):
#   HANDY_TOOLS_VERSION=0.2.0          # pin a specific version (default: latest)
#   HANDY_TOOLS_INSTALL_DIR=$HOME/...  # where to put binaries (default: $HOME/.local/bin)
#   HANDY_TOOLS_INSTALL_DEPS=1         # also install optional system tools (apt/dnf/pacman/brew)
#   HANDY_TOOLS_NO_GUI=1               # skip the desktop GUI tarball even on linux/amd64
#   HANDY_TOOLS_UNINSTALL=1            # remove binaries + config + cache and exit
#   NO_COLOR=1                         # disable ANSI colors
#   --version 0.2.0 / --dir PATH / --install-deps / --no-gui / --uninstall / --yes / --no-color / --help
#
# This script is POSIX sh. It targets Linux and macOS. Windows is not yet
# supported.

set -eu

REPO="FurkanEdizkan/Handy-Tools"
PROJECT_NAME="handy-tools"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"

VERSION="${HANDY_TOOLS_VERSION:-}"
INSTALL_DIR="${HANDY_TOOLS_INSTALL_DIR:-}"
INSTALL_DEPS="${HANDY_TOOLS_INSTALL_DEPS:-0}"
NO_GUI="${HANDY_TOOLS_NO_GUI:-0}"
UNINSTALL="${HANDY_TOOLS_UNINSTALL:-0}"
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
  if [ "$USE_COLOR" = "1" ]; then
    printf '\n%s  %s\n  %s\n\n' \
      "$(bold "$(orange 'Handy Tools')")" \
      "$(dim 'one-line installer')" \
      "$(dim "$REPO")"
  else
    printf '\nHandy Tools — one-line installer\n  %s\n\n' "$REPO"
  fi
}

usage() {
  banner
  cat <<'EOF'
Handy Tools installer — downloads the latest (or pinned) release from GitHub,
verifies the checksum, drops `handy` + `htools` + `htoolsd` + `htools-mcp`
(plus `htools-gui` on linux/amd64) into an install dir, and optionally installs the small set of optional
system tools Handy Tools uses.

  curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh
  curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh -s -- --uninstall

Knobs (env or flag):
  HANDY_TOOLS_VERSION=0.2.0          # pin a specific version (default: latest)
  HANDY_TOOLS_INSTALL_DIR=$HOME/...  # where to put binaries (default: $HOME/.local/bin)
  HANDY_TOOLS_INSTALL_DEPS=1         # also install optional system tools (apt/dnf/pacman/brew)
  HANDY_TOOLS_NO_GUI=1               # skip the desktop GUI tarball even on linux/amd64
  HANDY_TOOLS_UNINSTALL=1            # remove binaries + config + cache, then exit
  NO_COLOR=1                         # disable ANSI colors
  --version 0.2.0 / --dir PATH / --install-deps / --no-gui / --uninstall / --yes / --no-color / --help

The desktop GUI ships only on linux/amd64 (Wails + libwebkit2gtk); on other
platforms or with --no-gui the script installs only the four CLI binaries.

Uninstall removes the handy, htools, htoolsd, htools-mcp, htools-gui binaries
from the install dir (default $HOME/.local/bin or --dir), the config dir
($HANDY_TOOLS_CONFIG parent or $XDG_CONFIG_HOME/handy-tools or
~/.config/handy-tools), and the cache dir ($XDG_CACHE_HOME/handy-tools or
~/.cache/handy-tools). User-created output files are never touched. Prompts
before deleting unless --yes is set.

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
    --no-gui) NO_GUI=1; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
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

# resolve_config_dir prints the directory Handy Tools persists settings into,
# using the same lookup order as internal/config/config.go Path():
#   1. dirname($HANDY_TOOLS_CONFIG) if set
#   2. $XDG_CONFIG_HOME/handy-tools if set
#   3. $HOME/.config/handy-tools
resolve_config_dir() {
  if [ -n "${HANDY_TOOLS_CONFIG:-}" ]; then
    dirname "$HANDY_TOOLS_CONFIG"
  elif [ -n "${XDG_CONFIG_HOME:-}" ]; then
    printf '%s/handy-tools' "$XDG_CONFIG_HOME"
  else
    printf '%s/.config/handy-tools' "$HOME"
  fi
}

# resolve_cache_dir prints the directory Handy Tools may keep transient state
# in. Currently nothing writes here but we clean it defensively so a future
# cache addition has nothing stale to inherit on re-install.
resolve_cache_dir() {
  if [ -n "${XDG_CACHE_HOME:-}" ]; then
    printf '%s/handy-tools' "$XDG_CACHE_HOME"
  else
    printf '%s/.cache/handy-tools' "$HOME"
  fi
}

# do_uninstall removes installed binaries + config + cache and exits. It is
# called before the network-touching install path so an uninstall never tries
# to look up the latest release. User-created output files (./out, ./converted,
# etc.) are never touched.
do_uninstall() {
  cfg_dir=$(resolve_config_dir)
  cache_dir=$(resolve_cache_dir)
  bins="$INSTALL_DIR/handy $INSTALL_DIR/htools $INSTALL_DIR/htoolsd $INSTALL_DIR/htools-mcp $INSTALL_DIR/htools-gui"

  log "uninstall plan"
  for b in $bins; do
    if [ -e "$b" ]; then
      printf '  %s remove %s\n' "$(dim '-')" "$b"
    else
      printf '  %s skip   %s %s\n' "$(dim '-')" "$b" "$(dim '(not present)')"
    fi
  done
  if [ -d "$cfg_dir" ]; then
    printf '  %s remove %s\n' "$(dim '-')" "$cfg_dir"
  else
    printf '  %s skip   %s %s\n' "$(dim '-')" "$cfg_dir" "$(dim '(not present)')"
  fi
  if [ -d "$cache_dir" ]; then
    printf '  %s remove %s\n' "$(dim '-')" "$cache_dir"
  else
    printf '  %s skip   %s %s\n' "$(dim '-')" "$cache_dir" "$(dim '(not present)')"
  fi

  if [ "$ASSUME_YES" != "1" ]; then
    printf '%s ' "$(amber 'Proceed? [y/N]')"
    read -r reply || reply=""
    case "$reply" in
      y|Y|yes|YES) ;;
      *) die "aborted by user" ;;
    esac
  fi

  removed=0
  for b in $bins; do
    if [ -e "$b" ]; then
      rm -f "$b" && { ok "removed $b"; removed=$((removed+1)); }
    fi
  done
  if [ -d "$cfg_dir" ]; then
    rm -rf "$cfg_dir" && { ok "removed $cfg_dir"; removed=$((removed+1)); }
  fi
  if [ -d "$cache_dir" ]; then
    rm -rf "$cache_dir" && { ok "removed $cache_dir"; removed=$((removed+1)); }
  fi
  log "done. removed $removed path(s)."
}

banner

if [ "$UNINSTALL" = "1" ]; then
  do_uninstall
  exit 0
fi

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

# api_get URL — fetch a GitHub API URL and print its body to stdout. Unlike a
# bare `curl -f`, it inspects the HTTP status so the failure message names what
# actually went wrong: 404 (no release published, or the repo slug is wrong)
# versus 403 (API rate limited). Requires $tmp to exist. Dies on any non-2xx.
api_get() {
  _url=$1
  _body="$tmp/api_body"
  if [ "${DL%% *}" = "curl" ]; then
    # -f is deliberately omitted here: with -f, curl exits 22 on a 4xx and
    # discards both the body and the status code — that is the exact ambiguity
    # this function exists to remove. -w prints the numeric status instead.
    _code=$(curl -sSL -o "$_body" -w '%{http_code}' "$_url" 2>/dev/null) || _code=000
  else
    # wget -S writes the status line(s) to stderr; keep the last one so a
    # redirect chain reports the final code.
    if wget -S -O "$_body" "$_url" 2>"$tmp/api_hdr"; then
      _code=200
    else
      _code=$(sed -n 's#^[[:space:]]*HTTP/[0-9.]*[[:space:]]*\([0-9][0-9][0-9]\).*#\1#p' \
                "$tmp/api_hdr" | tail -n1)
      [ -n "$_code" ] || _code=000
    fi
  fi
  case "$_code" in
    2??) cat "$_body" ;;
    403) die "GitHub API returned 403 (rate limited). Wait a few minutes, or set HANDY_TOOLS_VERSION to install a specific version without the API lookup." ;;
    404) die "GitHub API returned 404 — no release found for $REPO. The repository may have no published release yet, or the repo slug is wrong. Set HANDY_TOOLS_VERSION to pin a version." ;;
    000) die "could not reach the GitHub API ($_url) — check your network connection." ;;
    *)   die "GitHub API request to $_url failed with HTTP $_code." ;;
  esac
}

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

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t handy-tools-install)
trap 'rm -rf "$tmp"' EXIT INT TERM

# ---- resolve version -------------------------------------------------------
if [ -z "$VERSION" ]; then
  log "looking up latest release of $REPO"
  # Handy Tools ships every CI-green commit as a calver pre-release
  # (v{YYYY}.{M}.{D}-beta.{N}). GitHub's /releases/latest endpoint
  # deliberately skips pre-releases, so we list /releases and pick the
  # newest non-draft entry instead (pre-releases are fine — they're
  # the norm pre-1.0).
  api="https://api.github.com/repos/${REPO}/releases?per_page=10"
  # GitHub's JSON emits "tag_name" before "draft" for each release entry.
  # Buffer the most recent tag_name and print it when the following
  # "draft": false is seen. POSIX-compatible, no jq.
  # api_get distinguishes 403/404/etc and dies with a specific message; here
  # we only have to handle a 2xx body that simply contained no usable tag.
  releases_json=$(api_get "$api")
  tag=$(printf '%s\n' "$releases_json" \
    | grep -E '"(tag_name|draft)"' \
    | awk '
        /"tag_name":/ { tag=$0; next }
        /"draft":/ {
          if ($0 ~ /false/ && tag != "") { print tag; exit }
          tag=""
        }' \
    | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' || true)
  [ -n "$tag" ] || die "GitHub API returned a release list with no published (non-draft) release for $REPO. Set HANDY_TOOLS_VERSION explicitly."
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

# The tar produced by goreleaser puts handy / htools / htoolsd / htools-mcp
# at the archive root. htools-gui ships in a separate archive (Wails desktop
# is linux/amd64-only with CGO, so it can't share the linux+darwin tarball).
for bin in handy htools htoolsd htools-mcp; do
  [ -f "$tmp/$bin" ] || die "expected $bin in archive but it was missing"
  install -m 0755 "$tmp/$bin" "$INSTALL_DIR/$bin"
  ok "installed $(amber "$INSTALL_DIR/$bin")"
done

# ---- desktop GUI (linux/amd64 only) ---------------------------------------
# Pull the separate GUI tarball when the platform supports it and the user
# didn't opt out. Soft-fail throughout — a missing or mismatching GUI
# tarball must not block the CLI install (the four binaries above are
# already on disk and useful).
GUI_INSTALLED=0
if [ "$NO_GUI" = "1" ]; then
  log "skipping desktop GUI install (--no-gui set)"
elif [ "$OS" != "linux" ] || [ "$ARCH" != "amd64" ]; then
  log "desktop GUI is currently linux/amd64 only — skipping htools-gui"
else
  gui_asset="${PROJECT_NAME}-gui_${VERSION}_${OS}_${ARCH}.tar.gz"
  gui_url="${base}/${gui_asset}"
  log "downloading $gui_asset"
  if $DLO "$tmp/$gui_asset" "$gui_url" 2>/dev/null; then
    gui_expected=$(grep "  ${gui_asset}\$" "$tmp/checksums.txt" | awk '{print $1}')
    if [ -z "$gui_expected" ]; then
      warn "no checksum entry for $gui_asset — skipping GUI install"
    else
      gui_got=$(cd "$tmp" && $SHA "$gui_asset" | awk '{print $1}')
      if [ "$gui_expected" != "$gui_got" ]; then
        warn "GUI checksum mismatch: expected $gui_expected, got $gui_got — skipping GUI install"
      else
        ok "GUI checksum verified"
        ( cd "$tmp" && tar -xzf "$gui_asset" )
        if [ -f "$tmp/htools-gui" ]; then
          install -m 0755 "$tmp/htools-gui" "$INSTALL_DIR/htools-gui"
          ok "installed $(amber "$INSTALL_DIR/htools-gui")"
          GUI_INSTALLED=1
        else
          warn "htools-gui missing from GUI tarball — skipping"
        fi
      fi
    fi
  else
    warn "GUI tarball not found at $gui_url — skipping (this release may not include a desktop build)"
  fi
fi

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
    unrar)         echo "RAR archive extraction (incl. multi-part .partN.rar)" ;;
    7z)            echo "7z multi-part extraction" ;;
    pdftoppm)      echo "render PDF pages to images" ;;
    pdftotext)     echo "extract text from PDFs" ;;
    magick)        echo "decode HEIC/HEIF images; encode HEIC and WebP" ;;
    libwebkit2gtk) echo "desktop app runtime — required to launch htools-gui" ;;
    *)             echo "" ;;
  esac
}

ALL_TOOLS="unrar 7z pdftoppm pdftotext magick"
# Only check for the libwebkit2gtk runtime when we actually installed the
# desktop GUI; macOS users and --no-gui callers shouldn't see a GTK prompt.
if [ "$GUI_INSTALLED" = "1" ]; then
  ALL_TOOLS="$ALL_TOOLS libwebkit2gtk"
fi
missing=""
for t in $ALL_TOOLS; do
  # libwebkit2gtk is a shared library, not a command — detect via
  # ldconfig (Linux). 4.0 (Ubuntu 22.04) and 4.1 (Ubuntu 24.04+) are both
  # acceptable; Wails resolves whichever is present at startup.
  if [ "$t" = "libwebkit2gtk" ]; then
    if ldconfig -p 2>/dev/null | grep -qE 'libwebkit2gtk-4\.(0|1)\.so'; then
      continue
    fi
    missing="$missing $t"
    continue
  fi
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
    apt:libwebkit2gtk) echo "libwebkit2gtk-4.1-0" ;;
    dnf:unrar)         echo "unrar" ;;
    dnf:7z)            echo "p7zip p7zip-plugins" ;;
    dnf:pdftoppm|dnf:pdftotext) echo "poppler-utils" ;;
    dnf:magick)        echo "ImageMagick" ;;
    dnf:libwebkit2gtk) echo "webkit2gtk4.1" ;;
    pacman:unrar)      echo "unrar" ;;
    pacman:7z)         echo "p7zip" ;;
    pacman:pdftoppm|pacman:pdftotext) echo "poppler" ;;
    pacman:magick)     echo "imagemagick" ;;
    pacman:libwebkit2gtk) echo "webkit2gtk-4.1" ;;
    brew:unrar)        echo "unrar" ;;
    brew:7z)           echo "p7zip" ;;
    brew:pdftoppm|brew:pdftotext) echo "poppler" ;;
    brew:magick)       echo "imagemagick" ;;
    # brew:libwebkit2gtk intentionally omitted — we don't ship GUI on darwin.
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
log "done. Try: $(orange "$INSTALL_DIR/handy --help")"
if [ "$GUI_INSTALLED" = "1" ]; then
  log "      or:  $(orange "$INSTALL_DIR/handy")  $(dim '# opens the desktop app')"
fi
log "      or:  $(orange "$INSTALL_DIR/htools --version")  $(dim '# the low-level CLI')"
