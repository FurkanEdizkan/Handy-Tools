# Install

This page covers the supported install paths for Handy Tools and the optional
system tools that unlock formats Go alone can't handle. For a quick start
(install one-liner + `handy`), see the [README](../README.md).

> The optional-system-tools table below mirrors
> [internal/tools/sysdep/sysdep.go](../internal/tools/sysdep/sysdep.go) and the
> `pkg_for` mapping in [install.sh](../install.sh). Keep all three in sync when
> adding or renaming an optional dependency.

## One-liner (Linux & macOS, amd64 & arm64)

```sh
curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh
```

The installer detects your OS/arch, downloads the matching release tarball,
verifies it against `checksums.txt`, and drops `handy`, `htools`, `htoolsd`,
and `htools-mcp` into `$HOME/.local/bin`.

On **linux/amd64** it also pulls a second tarball and installs `htools-gui`
— the Wails desktop app — alongside the CLI binaries. Pass `--no-gui`
(or `HANDY_TOOLS_NO_GUI=1`) to skip it on headless servers and container
builds. macOS and linux/arm64 don't get the GUI today; the installer
prints a one-line note and continues with the CLI binaries.

After install it lists missing optional system tools (and `libwebkit2gtk`
when the GUI was installed but the runtime library isn't on the system).
Pass `--install-deps` (and optionally `--yes`) to have it run the
matching `apt-get` / `dnf` / `pacman` / `brew` command.

## Installer flags

| Flag / env var                                | Effect                                                |
| --------------------------------------------- | ----------------------------------------------------- |
| `--version 0.2.0` / `HANDY_TOOLS_VERSION`     | Pin a specific version (default: latest).             |
| `--dir PATH` / `HANDY_TOOLS_INSTALL_DIR`      | Override the install/uninstall directory.             |
| `--install-deps` / `HANDY_TOOLS_INSTALL_DEPS` | Also install the optional system tools + `libwebkit2gtk`. |
| `--no-gui` / `HANDY_TOOLS_NO_GUI`             | Skip the desktop GUI tarball even on linux/amd64.     |
| `--uninstall` / `HANDY_TOOLS_UNINSTALL`       | Remove binaries + config + cache, then exit.          |
| `--yes`                                       | Skip the `[y/N]` prompt (deps install and uninstall). |

## Manual install

Pick the `handy-tools_VERSION_OS_ARCH.tar.gz` archive for your OS/arch from
the [releases page] (it includes `LICENSE`, `README.md`, and `docs/` plus
the four CLI binaries), extract it, and put the binaries on your PATH.
For the desktop app on linux/amd64, also grab
`handy-tools-gui_VERSION_linux_amd64.tar.gz` from the same release and
extract `htools-gui` next to the others. Each release publishes a
`*_source.tar.gz` and a `checksums.txt`.

[releases page]: https://github.com/FurkanEdizkan/Handy-Tools/releases

## Uninstall

Same script, `--uninstall` flag:

```sh
curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh -s -- --uninstall
```

The uninstaller removes the `handy`, `htools`, `htoolsd`, `htools-mcp`, and
`htools-gui` binaries from the install dir; the config dir
(`$HANDY_TOOLS_CONFIG` parent, or `$XDG_CONFIG_HOME/handy-tools`, or
`~/.config/handy-tools`); and the cache dir (`$XDG_CACHE_HOME/handy-tools`
or `~/.cache/handy-tools`). It prompts once before deleting; pass `--yes`
to skip the prompt. User-created output files are never touched. `--dir
PATH` overrides the binary location the same way it does for install.

## Optional system dependencies

The Handy Tools binaries are self-contained Go programs. A few features
shell out to small external programs when present; without them, the
affected actions fail with a structured `MISSING_BINARY` error and a clear
install hint instead of a crash. Run `handy doctor` to see exactly which
binaries are installed and what each unlocks.

| Feature                | Required tool        | Debian/Ubuntu                | macOS (Homebrew)           |
| ---------------------- | -------------------- | ---------------------------- | -------------------------- |
| RAR (incl. multi-part) | `unrar`              | `apt install unrar`          | `brew install unrar`       |
| 7z multi-part          | `7z` (p7zip)         | `apt install p7zip-full`     | `brew install p7zip`       |
| PDF → image            | `pdftoppm` (Poppler) | `apt install poppler-utils`  | `brew install poppler`     |
| PDF → text             | `pdftotext`          | `apt install poppler-utils`  | `brew install poppler`     |
| WebP / HEIC encoding   | `magick`             | `apt install imagemagick`    | `brew install imagemagick` |

CI exercises these tools on Linux, and each published release is
smoke-tested on Ubuntu 22.04/24.04 and macOS before it goes out.

## Build from source

```sh
make proto       # generate Go bindings under gen/ from api/proto (run once after clone)
make build       # builds bin/handy + bin/htools + bin/htoolsd + bin/htools-mcp
make handy       # prints the handy front-door help (sanity check)
make cli         # prints the htools CLI help (sanity check)
make serve       # runs the gRPC server on the address from config (default :7777)
make mcp         # runs the MCP server on stdio (or `make mcp-build` for the binary)
make gui         # builds the web UI and runs the Wails desktop app
make gui-build   # builds bin/htools-gui
make test        # go test -race -count=1 ./...
make fuzz        # 20s fuzz pass over the config YAML decoder
make lint        # golangci-lint + buf lint
make cover       # coverage.out + coverage.html
```

CI uses Go 1.25 and `golangci-lint v2.12.2` — match locally or lint output
may diverge.

### Desktop app (`htools-gui`)

The Wails desktop build needs CGO and the GTK/webkit dev headers. Install
them once, then `make gui`:

```sh
# Ubuntu 24.04+ (webkit2gtk-4.1):
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
# Ubuntu 22.04 (webkit2gtk-4.0):
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev

make gui          # build the embedded UI + run the app
```

`make gui` detects the installed webkit version with `pkg-config` and selects
the matching Wails build tag (`webkit2_41` for 4.1), so the same command
works on both. The other Go binaries stay CGO-free — only `htools-gui`
needs this toolchain.
