<div align="center">

# Handy Tools

```text
   /\___/\
  ( o   o )
   ( v )--[o]
  /~~~~~~~\
   \_____/
```

**A friendly terminal toolbox for everyday file work**
*Image conversion · Archive extraction · PDF utilities · and counting*

[![CI](https://github.com/FurkanEdizkan/Handy-Tools/actions/workflows/ci.yml/badge.svg)](https://github.com/FurkanEdizkan/Handy-Tools/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/furkandedizkan/handy-tools)](https://goreportcard.com/report/github.com/furkandedizkan/handy-tools)
[![Go Reference](https://pkg.go.dev/badge/github.com/furkandedizkan/handy-tools.svg)](https://pkg.go.dev/github.com/furkandedizkan/handy-tools)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/conventional%20commits-1.0.0-orange.svg)](https://www.conventionalcommits.org/en/v1.0.0/)

</div>

---

Handy Tools is a small toolbox for the file work you do every day — converting
images, extracting odd archive formats, slicing PDFs apart — without leaving
the terminal. Two binaries, one core, one mascot named **Wrenly**.

- **`htools`** — an interactive TUI (built with [Bubble Tea]) that lets you
  pick files, choose an action, confirm, and watch progress live.
- **`htoolsd`** — the same tools exposed over **gRPC**, so you can run Handy
  Tools as a service and call its features from anywhere (web, CI, scripts).

Both share one core: every tool is a plain Go package, used identically by the
TUI and the server. The architecture is on one page in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

[Bubble Tea]: https://github.com/charmbracelet/bubbletea

> **Status:** pre-alpha. The eight initial milestones (scaffolding through
> release polish) have landed on `main`; the next chapters are a plugin
> registry, a web/Chrome-extension surface over `htoolsd`, and a pure-Go PDF
> path. See the roadmap in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#roadmap).

## Features

### What it can do today

| Domain       | What you get                                                        | Pure Go?               |
| ------------ | ------------------------------------------------------------------- | ---------------------- |
| **Images**   | Convert PNG / JPEG / GIF / BMP / TIFF / WebP (decode-only).         | Yes                    |
| **Images**   | HEIC / HEIF decoding.                                               | Needs `magick`         |
| **Archives** | Extract & inspect zip, tar, gz, bz2, zst.                           | Yes                    |
| **Archives** | RAR (incl. multi-part `.partN.rar`) and 7z (incl. `.7z.001` parts). | Needs `unrar` / `7z`   |
| **PDF**      | Merge, split, metadata.                                             | Yes                    |
| **PDF**      | Render pages to images, extract text.                               | Needs Poppler          |
| **TUI**      | Home / Files / Settings pages, orange-black theme, mascot animation.| —                      |
| **gRPC**     | Streaming progress, allow-rooted path sandbox, reflection enabled.  | —                      |

### What's coming

- **Plugin registry** — add a tool by dropping one proto + one Go package; no
  TUI changes.
- **Web UI / Chrome extension** — a thin gRPC-Web gateway over `htoolsd`.
- **WebP encoding** — re-enabled once a pure-Go encoder is available (or CGO
  is accepted).
- **`pdfcpu` import** — replace shelled-out PDF ops with the pure-Go library.

## Install

### One-liner (Linux & macOS, amd64 & arm64)

```sh
curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh
```

The installer detects your OS/arch, downloads the matching release tarball,
verifies it against `checksums.txt`, and drops `htools` and `htoolsd` into
`$HOME/.local/bin`. In a color-capable terminal it renders an orange-and-black
ASCII mascot banner; set `NO_COLOR=1` (or pass `--no-color`) to disable.

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
| `--no-color` / `NO_COLOR`                     | Disable the ANSI banner.                        |

### Manual install

Pick the archive for your OS/arch from the [releases page] (it includes
`LICENSE`, `README.md`, and `docs/`), extract, put both binaries on your PATH.
Each release also publishes a `*_source.tar.gz` and a `checksums.txt`.

[releases page]: https://github.com/FurkanEdizkan/Handy-Tools/releases

### Optional system dependencies

Handy Tools is a single Go binary. A few features shell out to small external
programs when present; without them, the affected actions are disabled with a
clear inline hint instead of a crash. Run `htools doctor` to see exactly which
binaries are installed and what each unlocks.

| Feature                | Required tool        | Debian/Ubuntu                | macOS (Homebrew)           |
| ---------------------- | -------------------- | ---------------------------- | -------------------------- |
| RAR (incl. multi-part) | `unrar`              | `apt install unrar`          | `brew install unrar`       |
| 7z multi-part          | `7z` (p7zip)         | `apt install p7zip-full`     | `brew install p7zip`       |
| PDF → image            | `pdftoppm` (Poppler) | `apt install poppler-utils`  | `brew install poppler`     |
| PDF → text             | `pdftotext`          | `apt install poppler-utils`  | `brew install poppler`     |
| HEIC images            | `magick`             | `apt install imagemagick`    | `brew install imagemagick` |

Tested matrix is recorded in [COMPATIBILITY.md](COMPATIBILITY.md), rewritten
on every push to `main` from CI artifacts.

## Quick tour

```sh
# TUI: tab cycles Home / Files / Settings
htools

# Doctor: which optional tools are present, and what each one unlocks
htools doctor

# Version: semver, short commit, build date, GOOS/GOARCH
htools --version

# Service mode:
htoolsd --listen :7777 --allow-roots /srv/uploads,/srv/output

# Probe the running service with grpcurl:
grpcurl -plaintext localhost:7777 list
grpcurl -plaintext localhost:7777 \
  handytools.v1.ArchiveService/Inspect \
  <<<'{"source":{"path":"/srv/uploads/foo.7z.001"}}'
```

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

The on-disk YAML is parsed by a tiny hand-rolled reader in
[internal/config/yaml_min.go](internal/config/yaml_min.go); the canonical
shape and defaults live in [internal/config/config.go](internal/config/config.go).
Unknown keys are silently ignored so configs stay forward-compatible.

A minimal config looks like:

```yaml
theme:
  name: forge        # forge (default), snow, ember
mascot:
  enabled: true
  style: wrenly
image:
  default_jpeg_quality: 90
pdf:
  default_dpi: 150
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
make tui         # runs the TUI
make serve       # runs the gRPC server on the address from config (default :7777)
make test        # go test -race -count=1 ./...
make fuzz        # 20s fuzz pass over the YAML mini-parser
make lint        # golangci-lint + buf lint
make cover       # coverage.out + coverage.html
```

CI uses Go 1.22 and `golangci-lint v1.59` — match locally or lint output may
diverge.

## Releasing

The project version lives in
[`internal/buildinfo/version.txt`](internal/buildinfo/version.txt) — one line,
plain semver, no `v` prefix. To cut a release:

1. Open a PR against `test` that bumps `version.txt` (e.g. `0.1.0` → `0.2.0`).
2. CI on `test` validates the value (semver check lives in
   `internal/buildinfo/buildinfo_test.go`).
3. The promotion PR carries the bump into `main`.
4. On push to `main`,
   [`.github/workflows/auto-tag.yml`](.github/workflows/auto-tag.yml) waits
   for CI to go green, sees that `version.txt` changed, and pushes a
   `vX.Y.Z` tag.
5. The tag push triggers
   [`.github/workflows/release.yml`](.github/workflows/release.yml), which
   runs GoReleaser.

Reusing an existing version is rejected by the auto-tag workflow — bump
again to recover.

## Contributing

We love contributions. Read [CONTRIBUTING.md](CONTRIBUTING.md) and the
[PR guidelines](docs/PR_GUIDELINES.md) before opening a pull request.

In short:

- Open PRs against the `test` branch, **never** `main`.
- Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
- CI must be green.

## License

[MIT](LICENSE).
