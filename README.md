<div align="center">

# Handy Tools

**A friendly toolbox for everyday file work**
*Image conversion · Archive extraction · PDF utilities · and counting*

[![CI](https://github.com/FurkanEdizkan/Handy-Tools/actions/workflows/ci.yml/badge.svg)](https://github.com/FurkanEdizkan/Handy-Tools/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/furkandedizkan/handy-tools)](https://goreportcard.com/report/github.com/furkandedizkan/handy-tools)
[![Go Reference](https://pkg.go.dev/badge/github.com/furkandedizkan/handy-tools.svg)](https://pkg.go.dev/github.com/furkandedizkan/handy-tools)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/conventional%20commits-1.0.0-orange.svg)](https://www.conventionalcommits.org/en/v1.0.0/)

</div>

---

Handy Tools is a small toolbox for the file work you do every day — converting
images, extracting odd archive formats, slicing PDFs apart. One front door
(`handy`) over four backends.

- **`handy`** — the user-facing command. Bare `handy` launches the desktop
  app; `handy convert in.png --format jpeg --out out.jpg` runs the CLI;
  `handy serve` / `handy mcp` start the daemons. Thin dispatcher; the
  backends below do the real work.
- **`htools`** — a non-interactive subcommand CLI. One run, one operation,
  scriptable: `htools convert in.png --format jpeg --out out.jpg`. Also
  reachable via `handy convert ...`.
- **`htoolsd`** — the same tools exposed over **gRPC** and **HTTP + SSE**, so
  you can run Handy Tools as a service and call its features from anywhere
  (web, CI, scripts). It also embeds and serves the Svelte web UI. Reachable
  via `handy serve`.
- **`htools-mcp`** — the same tools exposed over the **Model Context Protocol**
  on stdio, so an MCP-capable client (Claude Code, Claude Desktop, Cursor) can
  call them directly in a conversation. Reachable via `handy mcp`.
  See [MCP server](#mcp-server) below.
- **`htools-gui`** — a Wails desktop app that wraps the same web UI in a
  native window with file dialogs. Built behind the `wails` build tag
  (CGO + webkit2gtk, linux/amd64). Reachable via bare `handy` or
  `handy gui`.

`handy` is the friendly UX; the four backends remain installed and can
still be called directly if you want to skip a hop. All five share one
core: every tool is a plain Go package, used identically by the CLI, the
server, the MCP bridge, and the desktop app. The architecture is on one page in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

> **Status:** pre-1.0, calver pre-releases. The CLI, the `htoolsd` server
> (gRPC + HTTP/SSE), the embedded Svelte web UI, and the Wails desktop app all
> run real image/archive/PDF jobs through a shared `internal/queue/`.
> See [Project #14](https://github.com/users/FurkanEdizkan/projects/14) for
> live status and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#roadmap) for the
> longer-horizon roadmap.

## Features

### What it can do today

| Domain       | What you get                                                                               | Pure Go?             |
| ------------ | ------------------------------------------------------------------------------------------ | -------------------- |
| **Images**   | Convert PNG / JPEG / GIF / BMP / TIFF, decode WebP.                                        | Yes                  |
| **Images**   | WebP / HEIC encoding, HEIC / HEIF decoding.                                                | Needs `magick`       |
| **Archives** | Pack & extract & inspect zip, tar, gz, bz2, zst.                                           | Yes                  |
| **Archives** | RAR (incl. multi-part `.partN.rar`) and 7z (incl. `.7z.001` parts).                        | Needs `unrar` / `7z` |
| **PDF**      | Merge, split, metadata.                                                                    | Yes                  |
| **PDF**      | Render pages to images, extract text, multi-page previews.                                 | Needs Poppler        |
| **CLI**      | Subcommand dispatch with `--quiet` / `--json` for scripting; stdlib `flag`, zero deps.     | Yes                  |
| **gRPC**     | Streaming progress, allow-rooted path sandbox, reflection enabled.                         | —                    |
| **HTTP/SSE** | REST endpoints mirroring gRPC, `tools.Progress` streamed as Server-Sent Events.            | —                    |
| **Web UI**   | Svelte + Tailwind SPA embedded into `htoolsd`; thumbnails, PDF previews, drag-and-drop.    | —                    |
| **Desktop**  | `htools-gui` — the web UI in a native Wails window with file dialogs.                      | —                    |

### What's coming

- **`pdfcpu` for the remaining shelled-out PDF ops** — merge and split are
  already pure-Go; page render still uses Poppler.

## Install

### One-liner (Linux & macOS, amd64 & arm64)

```sh
curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh
```

The installer detects your OS/arch, downloads the matching release tarball,
verifies it against `checksums.txt`, and drops `handy`, `htools`, `htoolsd`,
and `htools-mcp` into `$HOME/.local/bin`.

After install it lists missing optional system tools. Pass `--install-deps`
(and optionally `--yes`) to have it run the matching `apt-get` / `dnf` /
`pacman` / `brew` command.

### Uninstall

Same script, `--uninstall` flag:

```sh
curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh -s -- --uninstall
```

The uninstaller removes the `handy`, `htools`, `htoolsd`, `htools-mcp`, and
`htools-gui` binaries from the install dir; the config dir (`$HANDY_TOOLS_CONFIG` parent, or
`$XDG_CONFIG_HOME/handy-tools`, or `~/.config/handy-tools`); and the cache
dir (`$XDG_CACHE_HOME/handy-tools` or `~/.cache/handy-tools`). It prompts
once before deleting; pass `--yes` to skip the prompt. User-created output
files are never touched. `--dir PATH` overrides the binary location the same
way it does for install.

### Tuning the installer

| Flag / env var                                | Effect                                                |
| --------------------------------------------- | ----------------------------------------------------- |
| `--version 0.2.0` / `HANDY_TOOLS_VERSION`     | Pin a specific version (default: latest).             |
| `--dir PATH` / `HANDY_TOOLS_INSTALL_DIR`      | Override the install/uninstall directory.             |
| `--install-deps` / `HANDY_TOOLS_INSTALL_DEPS` | Also install the optional system tools.               |
| `--uninstall` / `HANDY_TOOLS_UNINSTALL`       | Remove binaries + config + cache, then exit.          |
| `--yes`                                       | Skip the `[y/N]` prompt (deps install and uninstall). |

### Manual install

Pick the archive for your OS/arch from the [releases page] (it includes
`LICENSE`, `README.md`, and `docs/`), extract, put both binaries on your PATH.
Each release also publishes a `*_source.tar.gz` and a `checksums.txt`.

[releases page]: https://github.com/FurkanEdizkan/Handy-Tools/releases

### Optional system dependencies

The Handy Tools binaries are self-contained Go programs. A few features shell
out to small external programs when present; without them, the affected actions
fail with a structured `MISSING_BINARY` error and a clear install hint instead
of a crash. Run `htools doctor` to see exactly which binaries are installed and
what each unlocks.

| Feature                | Required tool        | Debian/Ubuntu                | macOS (Homebrew)           |
| ---------------------- | -------------------- | ---------------------------- | -------------------------- |
| RAR (incl. multi-part) | `unrar`              | `apt install unrar`          | `brew install unrar`       |
| 7z multi-part          | `7z` (p7zip)         | `apt install p7zip-full`     | `brew install p7zip`       |
| PDF → image            | `pdftoppm` (Poppler) | `apt install poppler-utils`  | `brew install poppler`     |
| PDF → text             | `pdftotext`          | `apt install poppler-utils`  | `brew install poppler`     |
| WebP / HEIC encoding   | `magick`             | `apt install imagemagick`    | `brew install imagemagick` |

CI exercises these tools on Linux, and each published release is smoke-tested
on Ubuntu 22.04/24.04 and macOS before it goes out.

## Quick tour

Everything below works two ways: through `handy` (the friendly front door)
or by calling the underlying binary directly. Pick whichever you like.

```sh
# Open the desktop app:
handy                                  # ↔ htools-gui

# Image conversion (single source, single output file):
handy convert photo.png --format jpeg --quality 80 --out photo.jpg
# ↔ htools convert photo.png --format jpeg --quality 80 --out photo.jpg

# Batch convert into a directory:
handy convert a.png b.png c.png --format webp --out ./converted

# Pack a zip:
handy pack ./project --format zip --output project.zip

# Extract any archive (zip / tar.gz / 7z / rar / …):
handy extract bundle.tar.gz --out ./extracted

# PDF operations:
handy pdf merge a.pdf b.pdf --out merged.pdf
handy pdf split big.pdf --pages 1-20 --out ./parts
handy pdf render report.pdf --pages 1-3 --dpi 200 --out ./pages
handy pdf text report.pdf --layout --out report.txt

# Doctor: which optional tools are present, and what each one unlocks
handy doctor

# Version: semver, short commit, build date, GOOS/GOARCH
handy --version

# Help:
handy --help

# Service mode:
handy serve --listen :7777 --allow-roots /srv/uploads,/srv/output
# ↔ htoolsd --listen :7777 --allow-roots /srv/uploads,/srv/output

# MCP server (wire it into Claude Code / Cursor / Claude Desktop):
handy mcp                              # ↔ htools-mcp

# Probe the running service with grpcurl:
grpcurl -plaintext localhost:7777 list
grpcurl -plaintext localhost:7777 \
  handytools.v1.ArchiveService/Inspect \
  <<<'{"source":{"path":"/srv/uploads/foo.7z.001"}}'
```

Every subcommand accepts `--quiet` (suppress per-event progress lines) and
`--json` (emit one JSON object per progress event on stdout, for piping into
other tooling).

`htoolsd` refuses to start without `--allow-roots` (or `server.allow_roots`
in the config). Every `FileRef.path` is run through `Options.CheckPath`
before any tool is called — paths outside an allow-root, or that try to
escape via `..`, are rejected. See the test suite at
[internal/server/server_test.go](internal/server/server_test.go) for the
exact contract.

## MCP server

`htools-mcp` exposes the same toolbox over the [Model Context Protocol](https://modelcontextprotocol.io)
on stdio. An MCP-capable client launches it as a subprocess and can then call
`pdf_merge`, `pdf_split`, `pdf_render`, `pdf_text`, `image_convert`,
`image_batch_convert`, `image_strip_meta`, `archive_inspect`,
`archive_extract`, `archive_compress`, `hash`, `hash_verify`, `diff_tree`,
`rename_inspect`, `rename_run`, and `doctor` as ordinary tools in a
conversation.

To wire it into Claude Code, add an entry to `~/.claude.json` (or a per-project
`.mcp.json`). Either binary works — `handy mcp` re-execs `htools-mcp` and
preserves stdin/stdout, so the wire behavior is identical:

```json
{
  "mcpServers": {
    "handy-tools": {
      "command": "/absolute/path/to/handy-tools/bin/handy",
      "args": ["mcp"]
    }
  }
}
```

`htools-mcp` defaults to `--allow-roots=/` because it runs as a subprocess of
the local user (the same sandbox posture as `htools-gui`). Pass
`--allow-roots=/path/a,/path/b` if you want to narrow it. Path validation,
error codes, and progress messages flow through the same `internal/server/*Handler`
adapters that the gRPC and HTTP transports use, so behavior is identical
across surfaces.

## Configuration

Settings live at:

```text
$HANDY_TOOLS_CONFIG                          (explicit override)
$XDG_CONFIG_HOME/handy-tools/config.yaml     (XDG, when set)
~/.config/handy-tools/config.yaml            (default)
```

The on-disk YAML is decoded with [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3);
the canonical shape and defaults live in
[internal/config/config.go](internal/config/config.go). Unknown keys are
silently ignored so configs stay forward-compatible.

A minimal config looks like:

```yaml
image:
  default_jpeg_quality: 90
pdf:
  default_dpi: 150
archive:
  auto_extract_multi_part: false
  overwrite_by_default: false
server:
  listen: ":7777"
  allow_roots:
    - /srv/uploads
    - /srv/output
recent: []
```

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

CI uses Go 1.25 and `golangci-lint v2.12.2` — match locally or lint output may
diverge.

### Desktop app (`htools-gui`)

The Wails desktop build needs CGO and the GTK/webkit dev headers. Install them
once, then `make gui`:

```sh
# Ubuntu 24.04+ (webkit2gtk-4.1):
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
# Ubuntu 22.04 (webkit2gtk-4.0):
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev

make gui          # build the embedded UI + run the app
```

`make gui` detects the installed webkit version with `pkg-config` and selects
the matching Wails build tag (`webkit2_41` for 4.1), so the same command works
on both. The other Go binaries stay CGO-free — only `htools-gui` needs this
toolchain.

## Releasing

Releases are **automatic**: every CI-green commit on `main` gets a
calver pre-release tag and a published GitHub release. There is no
manual version bump.

The tag scheme is `v{YYYY}.{M}.{D}-beta.{N}` where `N` is one more
than the highest `-beta` counter already published for that UTC date.
For example, the third release on 2026-05-14 becomes
`v2026.5.14-beta.3` and ships as a GitHub **Pre-release**.

End-to-end flow:

1. Push a commit to `main`.
2. CI runs (lint, tests, fuzz, build).
3. On CI success,
   [`.github/workflows/auto-tag.yml`](.github/workflows/auto-tag.yml)
   fires via `workflow_run`, computes the next calver tag, and pushes
   it.
4. The tag push triggers
   [`.github/workflows/release.yml`](.github/workflows/release.yml),
   which runs GoReleaser. Release notes are grouped into **Changes**
   (`feat:`) and **Fixes** (`fix:`) sections; `docs/test/chore/ci:`
   commits are filtered out.

`internal/buildinfo/version.txt` (`0.0.0-dev`) is a placeholder for
local development only. `htools --version` shows it; the
release binaries have the real calver version baked in via
`-ldflags -X buildinfo.Version=…` at release time.

When we eventually cut a `v1.0.0` we'll switch the scheme back to
semver and drop the per-day beta counter; until then the calver +
pre-release model keeps the cadence honest about the project's
pre-alpha status.

## Contributing

We love contributions. Read [CONTRIBUTING.md](CONTRIBUTING.md) and the
[PR guidelines](docs/PR_GUIDELINES.md) before opening a pull request.

In short:

- Open PRs against the `test` branch, **never** `main`.
- Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
- CI must be green.

## Acknowledgements

Handy Tools binds several third-party libraries directly — pdfcpu for PDF
operations, klauspost/compress and dsnet/compress for archives,
`golang.org/x/image`, Wails for the desktop shell, gRPC, and Svelte/Tailwind
for the web UI. Every one is credited with its copyright and license in
[NOTICE](NOTICE).

## License

[MIT](LICENSE).
