# Handy

A friendly terminal toolbox for everyday file work — image conversion, archive
extraction (zip / 7z / rar / tar), PDF utilities, and more — guided by a small
snow-fox companion.

Handy is two things in one binary set:

- **`handy`** — an interactive TUI (built with [Bubble Tea]) that lets you pick
  files, choose an action, confirm, and watch progress.
- **`handyd`** — the same tools exposed over **gRPC**, so you can run Handy as
  a service and call its features from anywhere.

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

Handy ships as a single Go binary. A few features use external tools when
present; without them, the affected actions are disabled with a clear hint.
Run `handy doctor` to see what's installed.

| Feature                | Required tool        | Debian/Ubuntu                | macOS (Homebrew)           |
| ---------------------- | -------------------- | ---------------------------- | -------------------------- |
| RAR (incl. multi-part) | `unrar`              | `apt install unrar`          | `brew install unrar`       |
| 7z multi-part          | `7z` (p7zip)         | `apt install p7zip-full`     | `brew install p7zip`       |
| PDF -> image           | `pdftoppm` (Poppler) | `apt install poppler-utils`  | `brew install poppler`     |
| PDF -> text            | `pdftotext`          | `apt install poppler-utils`  | `brew install poppler`     |
| HEIC images            | `magick`             | `apt install imagemagick`    | `brew install imagemagick` |

## Install

Coming soon. Releases will be published via [GoReleaser] for linux & macOS,
amd64 & arm64.

[GoReleaser]: https://goreleaser.com

## Build from source

```sh
make proto       # generate Go bindings under gen/ (run once after clone)
make build       # builds bin/handy and bin/handyd
make tui         # runs the TUI
make serve       # runs the gRPC server on the address from config (default :7777)
make test        # unit tests
make lint        # golangci-lint + buf lint
```

## Quick tour

```sh
handy                 # launch the TUI; tab cycles Home / Files / Settings
handy doctor          # show which optional system tools are installed

# Service mode:
handyd --listen :7777 --allow-roots /srv/uploads,/srv/output
# Use grpcurl in another terminal:
grpcurl -plaintext localhost:7777 list
grpcurl -plaintext localhost:7777 handy.v1.ArchiveService/Inspect <<<'{"source":{"path":"/srv/uploads/foo.7z.001"}}'
```

The gRPC server refuses to start without `allow_roots` — every `FileRef.path`
is checked against that list before the underlying tool is called.

## Contributing

We love contributions. Read [CONTRIBUTING.md](CONTRIBUTING.md) and the
[PR guidelines](docs/PR_GUIDELINES.md) before opening a pull request.

In short:

- Open PRs against the `test` branch, never `main`.
- Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
- CI must be green.

## License

[MIT](LICENSE).
