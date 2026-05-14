<div align="center">

# Handy Tools

```text
   /\___/\
  ( o . o )
   \  v  /
    `---`
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

| Domain       | What you get                                                                               | Pure Go?             |
| ------------ | ------------------------------------------------------------------------------------------ | -------------------- |
| **Images**   | Convert PNG / JPEG / GIF / BMP / TIFF / WebP (decode-only).                                | Yes                  |
| **Images**   | HEIC / HEIF decoding.                                                                      | Needs `magick`       |
| **Archives** | Extract & inspect zip, tar, gz, bz2, zst.                                                  | Yes                  |
| **Archives** | RAR (incl. multi-part `.partN.rar`) and 7z (incl. `.7z.001` parts).                        | Needs `unrar` / `7z` |
| **PDF**      | Merge, split, metadata.                                                                    | Yes                  |
| **PDF**      | Render pages to images, extract text.                                                      | Needs Poppler        |
| **TUI**      | Home menu + per-tool detail page, live queue with expandable logs, three themes, mascot.   | —                    |
| **gRPC**     | Streaming progress, allow-rooted path sandbox, reflection enabled.                         | —                    |

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
# TUI:
#   ↑↓ or j/k     move through the tool menu  (or focus rows on the tool page)
#   enter         open the highlighted tool
#   esc           back to home
#   tab           cycle themes (forge / snow / ember)
#   ,             toggle the settings popover
#   1-5           jump to a tool by number
#   r             run the configured tool (on the tool page)
#   q             quit
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

### What it looks like

Two-pane terminal layout: a fixed left column with the **Wrenly** mascot, a
state block (current task + progress), and a live queue panel; a right column
that swaps between the home menu and a per-tool detail page. Full plain-text
snapshots live under [`docs/screenshots/`](docs/screenshots/) — here is the
home view abridged:

```text
  ⚙ settings    ◤ HANDY TOOLS / htools › Home                                                                       v0.1.0 ●

╭─────────────────────────────────╮    Welcome to Handy Tools — a friendly toolbox for everyday file work.
│ wrenly · IDLE                   │    Pick a tool to set up an input, output and options.  ↑↓  /  ENTER
│                                 │
│    /\___/\                      │    AVAILABLE TOOLS  ────────────────────────────────────────────  1 / 5
│   ( o . o )                     │
│    \  v  /                      │    ╭──────────────────────────────────────────────────────────────────╮
│     `---`                       │    │ ▸ ◇  Convert images — PNG · JPEG · WebP · GIF · BMP · TIFF    ↵ │
│                                 │    ╰──────────────────────────────────────────────────────────────────╯
│ Hi! I'm Wrenly.                 │       ▢  Pack into archive — zip · tar.gz · tar.bz2 · zstd · 7z
│ Throw any image at me — I'll    │       ◰  Extract archive — zip · 7z · rar · tar · gz · bz2 · zst
│ re-encode it.                   │       ◫  PDF utilities — merge · split · pages → image · text
╰─────────────────────────────────╯       ◊  Doctor — check optional system tools

╭───────────────────────────────────╮
│ STATE  idle                       │
│ CURRENT  — no active task —       │
│ ░░░░░░░░░░░░░░░░░░░░░░░░░  0%     │
╰───────────────────────────────────╯

╭───────────────────────────────────╮
│ QUEUE  0 run · 1 done · 1 fail    │
│   ✓ invoice-2026-04.png  DONE     │
│   ✕ manual.pdf → 32 pgs  FAIL  ▾  │
│     STDERR · q3  10 lines         │
│     [14:08] ERROR  MISSING_BINARY │
│     [14:08] HINT   brew install   │
│            poppler  # macOS       │
│   • photos.zip          WAIT      │
│   • big-batch (24 PNGs) WAIT      │
╰───────────────────────────────────╯
```

And the per-tool detail page (Convert images), with the **WebP** override on
row 3 visibly diverging from the JPEG default and the run summary on the
bottom reflecting the mixed targets:

```text
  ⚙ settings    ◤ HANDY TOOLS / htools › Convert images                                                              v0.1.0 ●

  ← back    Convert images
            Reencode between PNG · JPEG · WebP · GIF · BMP · TIFF

  INPUT  ────────────────────────────────────  accepts PNG · JPEG · WebP · GIF · BMP · TIFF · HEIC
  ╭──────────────────────────────────────────────────────────────────────────╮
  │                       Drop files or a folder here                         │
  │                    ▸ Browse files     ▸ Browse folder                    │
  │                       — or —  press  b  to browse                        │
  ╰──────────────────────────────────────────────────────────────────────────╯

  FILES (4)  ────────────────────────  default → JPEG   (f) cycle row · (F) apply to all
    ▪ screenshot-2026-05-14.png                          PNG → [ JPEG ▾ ]   2.1 MB
    ▪ logo-mark.png                                      PNG → [ JPEG ▾ ]   184 KB
    ▪ export@2x.png                                      PNG → [ WebP ▾ ]   5.4 MB
    ▪ cover-shot.jpg                                    JPEG → [ JPEG ▾ ]   1.8 MB

  OUTPUT DESTINATION  ─────────────────────────────────────────────────────────────────
    (●)  Default location — ./out                                  RECOMMENDED
    ( )  Alongside input — write next to each source file
    ( )  Custom path — [ /Users/me/converted ]

  OPTIONS  ──────────────────────────────────────────────────────────────
    JPEG/WebP quality    ▰▰▰▰▰▰▰▰▰▰▰▰▱▱▱▱  90
    Overwrite existing    [ ]
    Preserve mtime        [●]
    Recurse subfolders    [●]

  ready: 4 inputs  ·  3 → JPEG · 1 → WebP        [   ▸ RUN   ]   press  r  or  ENTER
```

Re-generate the full-width previews after any TUI change with:

```sh
go run ./cmd/snapshot                 # writes docs/screenshots/htools-*.txt
go run ./cmd/snapshot -stdout         # print to stdout instead
go run ./cmd/snapshot -width 200      # render at a different terminal width
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
4. CI runs on `main`. When it goes green,
   [`.github/workflows/auto-tag.yml`](.github/workflows/auto-tag.yml) fires
   via `workflow_run`, reads `version.txt`, and pushes a `vX.Y.Z` tag if
   one doesn't already exist.
5. The tag push triggers
   [`.github/workflows/release.yml`](.github/workflows/release.yml), which
   runs GoReleaser.

Auto-tag is idempotent: if `vX.Y.Z` already exists, the workflow logs
"already published" and exits cleanly — so a no-op CI run on the same
version is harmless. To cut another release, bump `version.txt`.

## Contributing

We love contributions. Read [CONTRIBUTING.md](CONTRIBUTING.md) and the
[PR guidelines](docs/PR_GUIDELINES.md) before opening a pull request.

In short:

- Open PRs against the `test` branch, **never** `main`.
- Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
- CI must be green.

## License

[MIT](LICENSE).
