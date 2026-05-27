# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project shape

Handy Tools is a single Go module that produces five binaries from one core:

- `cmd/handy` — user-facing front door. Bare `handy` launches the desktop app; `handy <verb>` re-execs into the right backend (`htools` / `htoolsd` / `htools-mcp` / `htools-gui`). Thin dispatcher, ~200 LOC, zero tool logic — never add behavior here, only routing. See [cmd/handy/main.go](cmd/handy/main.go).
- `cmd/htools` — non-interactive subcommand CLI (`make build && ./bin/htools --help`). Each invocation runs exactly one operation (`convert`, `pack`, `extract`, `inspect`, `pdf merge|split|render|text`, `hash`, `diff-tree`, `rename`, `strip-meta`, `doctor`, `version`) using stdlib `flag`.
- `cmd/htoolsd` — gRPC + HTTP/SSE server exposing the same tools (`make serve` / `go run ./cmd/htoolsd`).
- `cmd/htools-mcp` — Model Context Protocol server over stdio (`make mcp` / `go run ./cmd/htools-mcp`). Lets an MCP-capable client (Claude Code, Claude Desktop, Cursor) drive every tool. CGO-free, ships on linux/darwin × amd64/arm64 like `htools` and `htoolsd`.
- `cmd/htools-gui` — Wails v2 desktop app (`make gui` / `make gui-build`), gated behind the `wails` build tag (CGO + webkit2gtk, linux/amd64); without the tag a stub stands in so the other jobs stay CGO-free. `make gui` also adds the `webkit2_41` tag when `pkg-config` finds webkit2gtk-4.1 (Ubuntu 24.04+), so one command builds against either webkit 4.0 or 4.1. The release pipeline builds **both** ABIs (4.0 + 4.1) on the Ubuntu 22.04 runner and publishes two GUI tarballs (`handy-tools-gui_*` for 4.0, `handy-tools-gui-webkit41_*` for 4.1); `install.sh` ldconfig-probes and picks the matching one.

The four backends depend on `internal/tools/<feature>/` (image, archive, pdf, hash, difftree, rename, ...), which is the **only** layer allowed to touch files, run external binaries, or know about formats. `cmd/htools/`, `cmd/htools-mcp/`, `internal/server/`, and `internal/api/http/` are thin adapters — never put tool logic in any of them. The MCP server reuses the same `internal/server/*Handler` types as the gRPC/HTTP transports. `cmd/handy/` doesn't import any of `internal/tools` or `internal/server` — it only routes. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

### Wiring htools-mcp into Claude Code

After `make build` produces `bin/htools-mcp`, add it to `~/.claude.json` (or a project `.mcp.json`):

```json
{
  "mcpServers": {
    "handy-tools": {
      "command": "/absolute/path/to/handy-tools/bin/htools-mcp",
      "args": []
    }
  }
}
```

The binary defaults to `--allow-roots=/` because it runs as a subprocess of the local user, matching the `htools-gui` sandbox posture. Pass `--allow-roots=/path1,/path2` to narrow it.

## Common commands

```sh
make proto      # regenerate Go bindings into gen/ from api/proto (run once after clone)
make build      # bin/handy + bin/htools + bin/htoolsd + bin/htools-mcp
make handy      # print the handy front-door help (sanity check)
make cli        # print the htools CLI help (sanity check)
make mcp        # run the MCP server on stdio (or `make mcp-build` for the binary)
make gui        # build the web UI + run the Wails desktop app (needs GTK/webkit dev headers)
make test       # go test -race -count=1 ./...
make fuzz       # short fuzz over the config YAML decoder
make lint       # golangci-lint + buf lint
make cover      # coverage.out + coverage.html
```

Run a single test: `go test -race -run TestName ./internal/tools/archive`.

CI uses Go 1.25 and `golangci-lint v2.12.2` — match locally or lint output may diverge. The lint config (`.golangci.yml`) is golangci-lint v2 schema; `gofmt`/`goimports` are configured as v2 *formatters*.

## Generated code

`gen/` holds the protobuf/gRPC bindings produced by `buf generate`. They are checked in so contributors don't need `protoc`. After editing anything under `api/proto/v1/`, run `make proto` and commit the regenerated files. The `go_package_prefix` in [buf.gen.yaml](buf.gen.yaml) pins them under `github.com/furkandedizkan/handy-tools/gen`. The proto package is `handytools.v1` (output dir `gen/handytools/v1/`).

## Tool/feature contract

Every tool package under `internal/tools/<feature>/` exposes:

- request/result structs mirroring its proto,
- a function returning a progress channel of `tools.Progress` (see [internal/tools/tools.go](internal/tools/tools.go)),
- an `Inspect()` for preflight (e.g. multi-part archive detection, source-readability check) so callers can confirm before destructive work. Every `Inspection` carries an `Issues []tools.PathIssue` slice — populated via `tools.StatInputs` and `tools.CheckOutputDirWritable` — for missing/unreadable inputs and unwritable outputs. Issues are informational by default; the CLI's `--strict` flag aborts with exit 2 if any issue is present, and `--dry-run` reports issues then exits without acting.

Errors are structured `*tools.Error` with stable codes (`MISSING_BINARY`, `UNSUPPORTED_INPUT`, `BAD_REQUEST`, `IO_ERROR`, `PERMISSION_DENIED`, `NOT_FOUND`, `ABORTED`, `ROLLBACK_FAILED`). Filesystem error sites should classify via `tools.ClassifyFSError(err)` in [internal/tools/tools.go](internal/tools/tools.go) so EACCES surfaces as `PERMISSION_DENIED` and ENOENT as `NOT_FOUND` instead of being swallowed into `IO_ERROR`. The CLI maps these to process exit codes (`progress.go:exitCode`); the server translates them to gRPC/HTTP status — don't invent new codes without updating both layers.

Multi-file ops emit a terminal `tools.Progress.Failures []tools.Failure` so callers can list which specific files failed (not just a count). When every per-file failure shares the same Code (e.g. all `PERMISSION_DENIED`), the terminal `Err.Code` is coalesced via `tools.CoalesceFailureCode` to surface that instead of `IO_ERROR`. Rename, hash, image batch, archive pack, and strip-meta all populate this. Rename + strip-meta accept an opt-in `Rollback bool` (CLI flag `--rollback-on-error`) that aborts on first failure and replays a `tools.RollbackStack` to undo successful steps; failures during the undo itself are surfaced as Failure entries tagged `ROLLBACK_FAILED`. Archive pack writes to `<output>.partial` and atomically renames on success (no opt-in needed — never leaves half-written archives behind). Strip-meta `--in-place --rollback-on-error` keeps a `<source>.handy-bak` sidecar per file until the batch succeeds; a SIGKILL mid-batch leaves these orphans by stable name so a janitor pass can collect them.

The end-to-end story (codes, preflight Issues, rollback, atomic pack) is documented in [docs/FAILURE_HANDLING.md](docs/FAILURE_HANDLING.md), which also lists the scenario tests across the tool packages and the HTTP / MCP wire layers. When changing this surface, update the doc's coverage table.

Optional system binaries (`unrar`, `7z`, `pdftoppm`, `pdftotext`, `magick`) are detected at request time via [internal/tools/sysdep](internal/tools/sysdep/sysdep.go). Missing binaries must surface a `MISSING_BINARY` error with an install hint — never panic. New optional tools must be added to `sysdep.Known` so `htools doctor` lists them.

## Server invariant: AllowRoots

`htoolsd` **refuses to start without `server.allow_roots`** (config or `--allow-roots` flag) — with no roots there is nothing it can safely act on. Every `FileRef.path` is run through `Options.CheckPath` before any tool is called. The default behavior on empty roots is **fail closed** (reject everything), not "serve cwd" — preserve this when editing [internal/server/server.go](internal/server/server.go). The desktop (Wails) build runs on the user's own machine and so passes `AllowRoots: ["/"]`; the CLI calls the tool packages directly and has no path sandbox at all.

## Config

Settings live at `$HANDY_TOOLS_CONFIG` → `$XDG_CONFIG_HOME/handy-tools/config.yaml` → `~/.config/handy-tools/config.yaml` (in that order). The on-disk YAML is decoded with `gopkg.in/yaml.v3` via the thin `decode`/`writeYAML` helpers in [internal/config/config.go](internal/config/config.go); every `Config` field carries a `yaml:` struct tag. `loadFile` unmarshals over a `Defaults()` value, so `Defaults()` must always return a complete `Config` for partial files to round-trip safely.

## Installer script

[install.sh](install.sh) at repo root is the supported install path for end users (`curl -fsSL ... | sh`). It detects OS/arch, downloads the matching tarball from the latest GitHub release, verifies it against `checksums.txt`, and lists missing optional system tools — with a `--install-deps` flag to run the appropriate `apt`/`dnf`/`pacman`/`brew` command.

Two duplications to keep in sync:

- The list of optional tools and the brief feature blurbs in `install.sh` mirror [internal/tools/sysdep/sysdep.go](internal/tools/sysdep/sysdep.go).
- The `pkg_for` mapping in `install.sh` (handy-tool → distro package name) is its own source of truth; update it when adding new optional tools.

The asset name template (`handy-tools_${VERSION}_${OS}_${ARCH}.tar.gz`) must stay aligned with `archives.name_template` and `project_name` in [.goreleaser.yaml](.goreleaser.yaml).

## Versioning & releases

Handy Tools uses **calver pre-releases** while pre-1.0: every CI-green commit on `main` gets tagged `v{YYYY}.{M}.{D}-beta.{N}` (UTC date, unpadded month/day, per-day counter) and published as a GitHub pre-release. There is no manual version bump.

`internal/buildinfo/version.txt` (`0.0.0-dev`) is a placeholder used by local builds only — `go run ./cmd/htools --version` shows it. Release binaries get the real calver version baked in via `-ldflags -X buildinfo.Version=…` from [.goreleaser.yaml](.goreleaser.yaml), so the user-visible version on a real install always matches the tag. `TestEmbeddedVersionIsSemver` in `internal/buildinfo/buildinfo_test.go` validates that `version.txt` parses as semver — `0.0.0-dev` is valid, so the gate stays green.

End-to-end flow on every push to `main`:

1. CI runs (lint, tests, fuzz, build).
2. On CI success, [auto-tag.yml](.github/workflows/auto-tag.yml) fires via `workflow_run`, computes the next available `v{YYYY}.{M}.{D}-beta.{N}` (one more than the highest counter for today, or 1 if none), and pushes the tag.
3. auto-tag.yml then invokes [release.yml](.github/workflows/release.yml) directly via `workflow_call` (a tag-push trigger wouldn't fire downstream workflows because the tag was pushed with `GITHUB_TOKEN`) → GoReleaser builds linux/darwin × amd64/arm64 archives, computes `checksums.txt`, publishes the GitHub release.
4. The `release: published` event triggers [verify-install.yml](.github/workflows/verify-install.yml), which runs `install.sh` against the new release on Ubuntu 22.04/24.04 and macOS 14 and asserts `htools --version` matches. This catches installer/release-format drift before users do.

Release notes use GoReleaser's commit-message grouping: `feat:` → "Changes", `fix:` → "Fixes", everything else under "Other"; `docs/test/chore/ci:` commits are filtered out. The release title is "Handy Tools {version}".

Don't tag manually; the workflow owns the tag/release surface. When we eventually cut `v1.0.0` we'll switch back to plain semver and drop the per-day counter.

**Installer gotcha:** every calver build is marked Pre-release on GitHub (`prerelease: auto` in `.goreleaser.yaml`). GitHub's `/releases/latest` API endpoint deliberately skips prereleases, so `install.sh` lists `/releases?per_page=10` and picks the newest non-draft entry instead. Don't "simplify" the installer back to `/releases/latest` without first flipping releases to non-prerelease — and that won't be appropriate until v1.0.0.

## CI jobs

CI ([ci.yml](.github/workflows/ci.yml)) is Linux-only and runs on every push/PR to `main` and `test` — no path filter, so required status checks always report (even on docs-only PRs):

- `lint-go` — golangci-lint v2.12.2, runs `buf generate` first so generated code is in scope.
- `lint-proto` — `buf lint` against `api/proto/`.
- `web-build` — installs/builds/tests the Svelte frontend, uploads the `web-dist` artifact.
- `test-quick` — `go test -race` plus a 20-second fuzz pass over the config YAML decoder (`FuzzDecodeYAML`); consumes `web-dist`.
- `build` — final assembly check for the Linux binaries.

There is no OS matrix while the project is pre-1.0; non-Linux coverage is release-time only, via [verify-install.yml](.github/workflows/verify-install.yml). Restoring an ubuntu × macos (+ Windows compile) matrix is tracked for ~v1.0.0.

## Branding

Display brand is **Handy Tools**. Binary names are `htools`, `htoolsd`, and `htools-gui`. The proto package is `handytools.v1`. The vector mark at [docs/brand/wrenly.svg](docs/brand/wrenly.svg) is the project logo; rasterize to PNG for web surfaces. (The earlier ASCII mascot banner and the per-state TUI sprites were retired with the TUI.)

## CLI structure

The `cmd/htools/` binary is a stdlib-`flag` subcommand dispatcher:

- [main.go](cmd/htools/main.go) — entry + `dispatch()` switch over the verb.
- [convert.go](cmd/htools/convert.go), [pack.go](cmd/htools/pack.go), [extract.go](cmd/htools/extract.go), [inspect.go](cmd/htools/inspect.go), [pdf.go](cmd/htools/pdf.go), [hash.go](cmd/htools/hash.go), [difftree.go](cmd/htools/difftree.go), [rename.go](cmd/htools/rename.go), [stripmeta.go](cmd/htools/stripmeta.go), [doctor.go](cmd/htools/doctor.go) — one file per top-level verb; each owns its own `flag.FlagSet`.
- [progress.go](cmd/htools/progress.go) — `streamProgress(ch, opts)` drains a tool's `<-chan tools.Progress`, handles `--quiet` / `--json`, and maps the terminal `tools.Error.Code` to a process exit code.
- [usage.go](cmd/htools/usage.go) — `printUsage(w)` for `--help` / unknown-command paths.

Adding a new top-level subcommand means: new `cmd/htools/<verb>.go` with a `cmd<Verb>(ctx, cfg, args) int` function, a case in `dispatch()` in main.go, and a block in the `usage` string. Tool logic still lives under `internal/tools/<feature>/` — the verb file is a thin flag-parser + request-builder.

## Branching & PR workflow

- **Never open PRs against `main`.** All contributor PRs target `test`. A scheduled workflow promotes `test` → `main` automatically when CI is green.
- Branch prefixes: `feat/`, `fix/`, `docs/`, `refactor/`, `chore/`, `ci/`, `test/`.
- PR titles **must** be Conventional Commits (validated by the commitlint workflow). Common scopes: `image`, `archive`, `pdf`, `cli`, `server`, `api`, `config`, `web`, `gui`, `ci`, `release`.
- PRs are merged with **merge commits** — never squash — so every commit is
  preserved on `test` and `main` and the branch graph stays intact. The PR
  title (a Conventional Commit) becomes the merge commit message; keep the
  individual commits on your branch clean and Conventional too, since they
  land in history and feed the release changelog.

Full checklist in [docs/PR_GUIDELINES.md](docs/PR_GUIDELINES.md).

## Gotchas

- The `.gitignore` ignores `/htools` and `/htoolsd` at repo root (ad-hoc `go build` outputs). It used to be too broad and accidentally excluded `cmd/handy/` sources — if you add new top-level files or directories whose name starts with `htools` or `handy`, double-check they aren't ignored.
- Tests should use real fixtures under `testdata/`, not mocks, when verifying file-format behavior. Keep fixtures tiny (a 4-pixel PNG, a 3-byte file in a zip).
- There is no plugin system — each tool is in-house code or a directly-bound library. Any new third-party dependency must be credited in [NOTICE](NOTICE) with its copyright holder and license.
- WebP and HEIC encoding have no pure-Go encoder (and the project hasn't accepted CGO), so both are delegated to the optional `magick` binary via `encodeViaMagick` in [internal/tools/image/image.go](internal/tools/image/image.go). Without `magick` on `PATH` a WebP/HEIC convert surfaces a `MISSING_BINARY` error — it never panics. Decoding WebP is pure-Go (`golang.org/x/image/webp`).
