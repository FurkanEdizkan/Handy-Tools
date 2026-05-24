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
images, extracting odd archive formats, slicing PDFs apart. Three binaries, one
core.

- **`htools`** — a non-interactive subcommand CLI. One run, one operation,
  scriptable: `htools convert in.png --format jpeg --out out.jpg`.
- **`htoolsd`** — the same tools exposed over **gRPC** and **HTTP + SSE**, so
  you can run Handy Tools as a service and call its features from anywhere
  (web, CI, scripts). It also embeds and serves the Svelte web UI.
- **`htools-gui`** — a Wails desktop app that wraps the same web UI in a
  native window with file dialogs. Built behind the `wails` build tag
  (CGO + webkit2gtk, linux/amd64).

All three share one core: every tool is a plain Go package, used identically by
the CLI, the server, and the desktop app. The architecture is on one page in
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
verifies it against `checksums.txt`, and drops `htools` and `htoolsd` into
`$HOME/.local/bin`.

After install it lists missing optional system tools. Pass `--install-deps`
(and optionally `--yes`) to have it run the matching `apt-get` / `dnf` /
`pacman` / `brew` command.

### Tuning the installer

| Flag / env var                                | Effect                                          |
| --------------------------------------------- | ----------------------------------------------- |
| `--version 0.2.0` / `HANDY_TOOLS_VERSION`     | Pin a specific version (default: latest).       |
| `--dir PATH` / `HANDY_TOOLS_INSTALL_DIR`      | Override the install directory.                 |
| `--install-deps` / `HANDY_TOOLS_INSTALL_DEPS` | Also install the optional system tools.         |
| `--yes`                                       | Skip the `[y/N]` prompt before installing deps. |

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

```sh
# Image conversion (single source, single output file):
htools convert photo.png --format jpeg --quality 80 --out photo.jpg

# Batch convert into a directory:
htools convert a.png b.png c.png --format webp --out ./converted

# Pack a zip:
htools pack ./project --format zip --output project.zip

# Extract any archive (zip / tar.gz / 7z / rar / …):
htools extract bundle.tar.gz --out ./extracted

# PDF operations:
htools pdf merge a.pdf b.pdf --out merged.pdf
htools pdf split big.pdf --pages 1-20 --out ./parts
htools pdf render report.pdf --pages 1-3 --dpi 200 --out ./pages
htools pdf text report.pdf --layout --out report.txt

# Doctor: which optional tools are present, and what each one unlocks
htools doctor

# Version: semver, short commit, build date, GOOS/GOARCH
htools --version

# Help:
htools --help

# Service mode:
htoolsd --listen :7777 --allow-roots /srv/uploads,/srv/output

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
make build       # builds bin/htools and bin/htoolsd
make cli         # prints the CLI help (sanity check)
make serve       # runs the gRPC server on the address from config (default :7777)
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
