# Handy Tools

A friendly terminal toolbox for everyday file work — image conversion, archive
extraction (zip / 7z / rar / tar), PDF utilities, and more — guided by a small
red-panda companion called Wrenly.

Handy Tools is two things in one binary set:

- **`htools`** — an interactive TUI (built with [Bubble Tea]) that lets you pick
  files, choose an action, confirm, and watch progress.
- **`htoolsd`** — the same tools exposed over **gRPC**, so you can run Handy
  Tools as a service and call its features from anywhere.

Both share one core: every tool is a plain Go package, used identically by the
TUI and the server.

[Bubble Tea]: https://github.com/charmbracelet/bubbletea

## Status

Pre-alpha. See the roadmap in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Features (planned)

- **Images**: convert between PNG, JPEG, GIF, BMP, TIFF, WebP. HEIC via system
  `magick`.
- **Archives**: extract & inspect zip, tar, gz, bz2, zst natively; rar and 7z
  (including multi-part `.part1.rar` / `.7z.001`) via system `unrar` / `7z`.
- **PDF**: merge, split, metadata (pure Go); render pages to images and extract
  text via system Poppler (`pdftoppm`, `pdftotext`).
- **Home page** with tools list and a recent-files browser.
- **File browser** that surfaces the actions available for each selected file.
- **Settings** for theme, mascot, and per-tool defaults (e.g. *always extract
  multi-part archives without asking*).
- **gRPC API** mirroring every TUI feature.

## System dependencies (optional)

Handy Tools ships as a single Go binary. A few features use external tools when
present; without them, the affected actions are disabled with a clear hint.
Run `htools doctor` to see what's installed.

| Feature                | Required tool        | Debian/Ubuntu                | macOS (Homebrew)           |
| ---------------------- | -------------------- | ---------------------------- | -------------------------- |
| RAR (incl. multi-part) | `unrar`              | `apt install unrar`          | `brew install unrar`       |
| 7z multi-part          | `7z` (p7zip)         | `apt install p7zip-full`     | `brew install p7zip`       |
| PDF -> image           | `pdftoppm` (Poppler) | `apt install poppler-utils`  | `brew install poppler`     |
| PDF -> text            | `pdftotext`          | `apt install poppler-utils`  | `brew install poppler`     |
| HEIC images            | `magick`             | `apt install imagemagick`    | `brew install imagemagick` |

## Install

One-liner (Linux & macOS, amd64 & arm64):

```sh
curl -fsSL https://raw.githubusercontent.com/furkandedizkan/handy-tools/main/install.sh | sh
```

The script detects your OS/arch, downloads the latest release tarball published
by [GoReleaser], verifies the checksum against `checksums.txt`, and installs
`htools` and `htoolsd` into `$HOME/.local/bin` (override with `--dir` or
`HANDY_TOOLS_INSTALL_DIR`). Pin a version with `HANDY_TOOLS_VERSION=0.2.0` or
`--version 0.2.0`. The installer renders an orange/black mascot banner in
color-capable terminals; set `NO_COLOR=1` (or pass `--no-color`) to disable.

After install it lists the optional system tools that aren't on your PATH yet.
Pass `--install-deps` (and optionally `--yes`) to have it run the matching
`apt-get` / `dnf` / `pacman` / `brew` command for you.

Manual install: pick the archive for your OS/arch from the [releases page]
(it includes `LICENSE`, `README.md`, and `docs/`), extract, put both binaries
on your PATH. Each release also publishes a `*_source.tar.gz` and a
`checksums.txt`.

[GoReleaser]: https://goreleaser.com
[releases page]: https://github.com/furkandedizkan/handy-tools/releases

## Build from source

```sh
make proto       # generate Go bindings under gen/ (run once after clone)
make build       # builds bin/htools and bin/htoolsd
make tui         # runs the TUI
make serve       # runs the gRPC server on the address from config (default :7777)
make test        # unit tests
make fuzz        # short fuzz pass over the YAML mini-parser
make lint        # golangci-lint + buf lint
```

## Quick tour

```sh
htools                 # launch the TUI; tab cycles Home / Files / Settings
htools doctor          # show which optional system tools are installed
htools --version       # print version, commit, build date, GOOS/GOARCH

# Service mode:
htoolsd --listen :7777 --allow-roots /srv/uploads,/srv/output
# Use grpcurl in another terminal:
grpcurl -plaintext localhost:7777 list
grpcurl -plaintext localhost:7777 handytools.v1.ArchiveService/Inspect <<<'{"source":{"path":"/srv/uploads/foo.7z.001"}}'
```

The gRPC server refuses to start without `allow_roots` — every `FileRef.path`
is checked against that list before the underlying tool is called.

## Releasing

The project version lives in [`internal/buildinfo/version.txt`](internal/buildinfo/version.txt) — one line, plain semver, no `v` prefix. To cut a release:

1. Open a PR against `test` that bumps `version.txt` (e.g. `0.1.0` → `0.2.0`).
2. CI on `test` validates the value (semver check lives in `internal/buildinfo/buildinfo_test.go`).
3. The promotion PR carries the bump into `main`.
4. On push to `main`, [`.github/workflows/auto-tag.yml`](.github/workflows/auto-tag.yml) waits for CI to go green, sees that `version.txt` changed, and pushes a `vX.Y.Z` tag.
5. The tag push triggers [`.github/workflows/release.yml`](.github/workflows/release.yml), which runs GoReleaser.

Reusing an existing version is rejected by the auto-tag workflow — bump again to recover.

## Contributing

We love contributions. Read [CONTRIBUTING.md](CONTRIBUTING.md) and the
[PR guidelines](docs/PR_GUIDELINES.md) before opening a pull request.

In short:

- Open PRs against the `test` branch, never `main`.
- Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
- CI must be green.

## License

[MIT](LICENSE).
