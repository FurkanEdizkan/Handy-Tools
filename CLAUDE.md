# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project shape

Handy Tools is a single Go module that produces two binaries from one core:

- `cmd/htools` — Bubble Tea TUI (`make tui` / `go run ./cmd/htools`)
- `cmd/htoolsd` — gRPC server exposing the same tools (`make serve` / `go run ./cmd/htoolsd`)

Both depend on `internal/tools/<feature>/` (image, archive, pdf), which is the **only** layer allowed to touch files, run external binaries, or know about formats. `internal/ui/` and `internal/server/` are thin adapters — never put tool logic in either. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

A third entry point, `cmd/snapshot`, is a developer-only helper that renders the TUI views into `docs/screenshots/htools-*.txt` so the README previews stay in sync. It is **excluded** from release builds (not listed under `builds:` in `.goreleaser.yaml`). Re-run `go run ./cmd/snapshot` after any UI-shaping change.

## Common commands

```sh
make proto      # regenerate Go bindings into gen/ from api/proto (run once after clone)
make build      # bin/htools + bin/htoolsd
make test       # go test -race -count=1 ./...
make fuzz       # short fuzz over the YAML mini-parser
make lint       # golangci-lint + buf lint
make cover      # coverage.out + coverage.html
```

Run a single test: `go test -race -run TestName ./internal/tools/archive`.

CI uses Go 1.22 and `golangci-lint v1.59` — match locally or lint output may diverge.

## Generated code

`gen/` holds the protobuf/gRPC bindings produced by `buf generate`. They are checked in so contributors don't need `protoc`. After editing anything under `api/proto/v1/`, run `make proto` and commit the regenerated files. The `go_package_prefix` in [buf.gen.yaml](buf.gen.yaml) pins them under `github.com/furkandedizkan/handy-tools/gen`. The proto package is `handytools.v1` (output dir `gen/handytools/v1/`).

## Tool/feature contract

Every tool package under `internal/tools/<feature>/` exposes:

- request/result structs mirroring its proto,
- a function returning a progress channel of `tools.Progress` (see [internal/tools/tools.go](internal/tools/tools.go)),
- an `Inspect()` for preflight (e.g. multi-part archive detection) so callers can confirm before destructive work.

Errors are structured `*tools.Error` with stable codes (`MISSING_BINARY`, `UNSUPPORTED_INPUT`, `BAD_REQUEST`, `IO_ERROR`, `ABORTED`). The TUI and server both translate these — don't invent new codes without updating both layers.

Optional system binaries (`unrar`, `7z`, `pdftoppm`, `pdftotext`, `magick`) are detected at request time via [internal/tools/sysdep](internal/tools/sysdep/sysdep.go). Missing binaries must surface a `MISSING_BINARY` error with an install hint — never panic, never crash the TUI. New optional tools must be added to `sysdep.Known` so `htools doctor` lists them.

## Server invariant: AllowRoots

`htoolsd` refuses to start without `server.allow_roots` (config or `--allow-roots` flag). Every `FileRef.path` is run through `Options.CheckPath` before any tool is called. The default behavior on empty roots is **fail closed** (reject everything), not "serve cwd" — preserve this when editing [internal/server/server.go](internal/server/server.go).

## Config

Settings live at `$HANDY_TOOLS_CONFIG` → `$XDG_CONFIG_HOME/handy-tools/config.yaml` → `~/.config/handy-tools/config.yaml` (in that order). The on-disk YAML is parsed by a tiny hand-rolled reader in [internal/config/yaml_min.go](internal/config/yaml_min.go) — only flat scalars + the `recent`/`allow_roots` lists are supported. If you need richer YAML, swap to `gopkg.in/yaml.v3` rather than extending the hand-rolled parser. `Defaults()` must always return a complete `Config` so partial files round-trip safely.

## Installer script

[install.sh](install.sh) at repo root is the supported install path for end users (`curl -fsSL ... | sh`). It detects OS/arch, downloads the matching tarball from the latest GitHub release, verifies it against `checksums.txt`, and lists missing optional system tools — with a `--install-deps` flag to run the appropriate `apt`/`dnf`/`pacman`/`brew` command. The installer renders an orange/black mascot banner when stdout is a TTY, `NO_COLOR` is unset, and `TERM` isn't `dumb`.

Two duplications to keep in sync:

- The list of optional tools and the brief feature blurbs in `install.sh` mirror [internal/tools/sysdep/sysdep.go](internal/tools/sysdep/sysdep.go).
- The `pkg_for` mapping in `install.sh` (handy-tool → distro package name) is its own source of truth; update it when adding new optional tools.

The asset name template (`handy-tools_${VERSION}_${OS}_${ARCH}.tar.gz`) must stay aligned with `archives.name_template` and `project_name` in [.goreleaser.yaml](.goreleaser.yaml).

## Versioning & releases

Version lives in [internal/buildinfo/version.txt](internal/buildinfo/version.txt) — one line, plain semver, no `v` prefix. The buildinfo package embeds it via `//go:embed` so `htools --version` and `htoolsd --version` always reflect what's checked in. Release builds override `buildinfo.Version` (and `Commit`, `Date`) via `-ldflags "-X"`; see [.goreleaser.yaml](.goreleaser.yaml).

Release flow: bump `version.txt` in a PR to `test` → promote to `main` → [auto-tag.yml](.github/workflows/auto-tag.yml) waits for CI, then pushes a `vX.Y.Z` tag → tag triggers [release.yml](.github/workflows/release.yml). Reusing an existing version causes the auto-tag step to fail loudly. Don't tag manually; let the workflow do it.

`TestEmbeddedVersionIsSemver` in `internal/buildinfo/buildinfo_test.go` is the canonical semver gate — if the file is malformed, `make test` fails before anything is tagged.

## Compatibility matrix

Tested OS/arch combinations are recorded in [COMPATIBILITY.md](COMPATIBILITY.md), generated by the `update-compatibility` job in [ci.yml](.github/workflows/ci.yml). The job runs only on push to `main`, downloads `compat-*` artifacts emitted by each test matrix entry, and rewrites the table — overwriting rows for the current version, preserving rows for older versions. The commit uses `[skip ci]` to avoid a feedback loop.

If you add a new OS to the test matrix, also update the runner-list section in [README.md](README.md#system-dependencies-optional) so users know which environments are validated.

## CI jobs

- `lint-go` — golangci-lint v1.59, runs `buf generate` first so generated code is in scope.
- `lint-proto` — `buf lint` against `api/proto/`.
- `test` — matrix on Ubuntu 22/24 and macOS 13/14/15 with system deps installed.
- `build-windows` — compile-only on `windows-latest`; ensures Go sources remain portable even though releases don't target Windows yet.
- `fuzz` — 20-second fuzz pass over the YAML mini-parser (`FuzzDecodeMinimalYAML`).
- `build` — final assembly check for the Linux binaries.
- `update-compatibility` — main-only, rewrites COMPATIBILITY.md from CI artifacts.

## Branding

Display brand is **Handy Tools**. Binary names are `htools` and `htoolsd`. The proto package is `handytools.v1`. The companion mascot is **Wrenly** — a small red panda rendered as a face-only ASCII silhouette (`/\___/\` ears, `( o . o )` eyes, `\  v  /` mouth, `` `---` `` chin). Different per-state frames (idle blink, thinking `?`, working, success sparkles, error `x x`) cover the animation hooks — the wrench-bearing body was retired on 2026-05-14 in favor of the cleaner face after design iteration. The default theme `forge` is orange-and-black; `snow` (cyan) and `ember` (warm orange) remain as alternative palettes.

## TUI layout

The TUI is a two-pane layout owned by [internal/ui/router.go](internal/ui/router.go):

- **Left column (fixed)**: `mascot.Model` → state block (current task + progress) → queue panel with expandable per-job stderr logs.
- **Right column**: either the **Home** menu (tool catalog in [internal/ui/home.go](internal/ui/home.go)) or a **Tool detail page** ([internal/ui/toolpage.go](internal/ui/toolpage.go)) with input dropzone, file list with per-file format override, output destination radio, options grid, and a run button.

Pages don't reach into each other — every navigation/state change is a `tea.Msg` (`OpenTool`, `GoHome`, `RunJob`, `MascotMsg`). The router holds the shared state (mascot, queue, progress, toast) and re-renders the active page.

Adding a tool to the TUI means: append to `defaultTools` in `home.go`, add a `speechFor` case, and extend `toolpage.go` to handle the new `toolMode`. The queue, state block, and mascot already work for every tool — no changes there.

When you change the TUI in any visible way, regenerate the README previews:

```sh
go run ./cmd/snapshot
```

## Branching & PR workflow

- **Never open PRs against `main`.** All contributor PRs target `test`. A scheduled workflow promotes `test` → `main` automatically when CI is green.
- Branch prefixes: `feat/`, `fix/`, `docs/`, `refactor/`, `chore/`, `ci/`, `test/`.
- PR titles **must** be Conventional Commits (validated by the commitlint workflow). Common scopes: `image`, `archive`, `pdf`, `ui`, `server`, `api`, `config`, `mascot`, `ci`, `release`.
- Squash-merge is the default; the PR title becomes the commit message.

Full checklist in [docs/PR_GUIDELINES.md](docs/PR_GUIDELINES.md).

## Gotchas

- The `.gitignore` ignores `/htools` and `/htoolsd` at repo root (ad-hoc `go build` outputs). It used to be too broad and accidentally excluded `cmd/handy/` sources — if you add new top-level files or directories whose name starts with `htools` or `handy`, double-check they aren't ignored.
- Tests should use real fixtures under `testdata/`, not mocks, when verifying file-format behavior. Keep fixtures tiny (a 4-pixel PNG, a 3-byte file in a zip).
- WebP encoding is intentionally disabled — there is no pure-Go encoder and the project hasn't accepted CGO yet.
