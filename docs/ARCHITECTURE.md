# Architecture

Handy Tools is one Go module that produces five binaries from one shared
tool core. Every UI/transport is a thin adapter over `internal/tools/`.

```text
                       +----------------------------+
                       |   internal/tools/<x>       |   pure Go API per feature
                       |   (image, archive, pdf,    |   no UI, no network
                       |    hash, difftree, rename) |
                       +-------------+--------------+
                                     ^
              +----------+-----------+-----------+
              |          |                       |
     +--------+------+ +-+-------------+  +------+--------+
     |  cmd/htools   | | internal/api/ |  | internal/queue|
     |  (CLI)        | | http (REST+SSE)|  | (shared)     |
     +---------------+ +---------------+  +---------------+
                            ^   ^                 ^
                            |   |                 |
                       cmd/htoolsd        consumed by every
                       (gRPC + HTTP+SSE)  server-side surface
                       cmd/htools-mcp     (htoolsd / htools-gui)
                       (MCP over stdio;
                        wraps internal/server/*Handler)
                              ^
                              |  via embedded webview
                              |
                       cmd/htools-gui
                       (Wails desktop, linux/amd64 + CGO)

       cmd/handy  front-door dispatcher; re-execs the right backend
                  (htools | htoolsd | htools-mcp | htools-gui)

       web/  Svelte + Vite + TS + Tailwind — built into web/dist/ and
             embedded into htoolsd / htools-gui via go:embed
```

## Packages

### `internal/tools/<feature>`

Each feature is a plain Go package with:

- A request/result struct.
- A function that performs the work and returns a `<-chan tools.Progress`.
- An `Inspect()` function where preflight matters (e.g. detecting multi-part
  archives) so callers can confirm before destructive work.

These packages are the **only** place that touches files, runs external
binaries, or knows anything about formats. They are deliberately UI- and
transport-agnostic and are imported as-is by every adapter.

### `cmd/htools` (CLI)

The CLI is a non-interactive subcommand dispatcher built on stdlib `flag`. One
run = one operation; nothing is persistent. Each subcommand parses its own
flag set, builds the matching request struct in `internal/tools/<feature>`,
streams the progress channel to stderr (or JSON to stdout via `--json`), and
exits with a code derived from the terminal `tools.Error.Code`.

Subcommands: `convert`, `pack`, `extract`, `inspect`, `pdf
merge|split|render|text`, `hash`, `diff-tree`, `rename`, `strip-meta`,
`doctor`, `version`. See `htools --help` for the canonical list of flags.

### `internal/server` (gRPC)

Thin gRPC handlers under `cmd/htoolsd`. Each one calls into
`internal/tools/<x>` and maps the progress channel onto a streaming RPC
response. Path safety lives here as `Options.CheckPath`, reused by every
transport.

### `internal/api/http` (HTTP + Server-Sent Events)

Sibling transport. Mirrors the gRPC services with JSON bodies; the streaming
response is rendered as SSE so browsers consume it natively. Reuses the same
`internal/server.{Image,Archive,PDF}Handler` types and `Options.CheckPath`
as gRPC — there is no second copy of the path-safety logic. Endpoints:

```text
POST /v1/image/convert            → 202 {"job_id": "..."}
POST /v1/archive/inspect          → 200 {Inspection}
POST /v1/archive/extract          → 202 {"job_id": "..."}
POST /v1/pdf/to-image             → 202 {"job_id": "..."}
POST /v1/pdf/to-text              → 202 {"job_id": "..."}
POST /v1/pdf/merge                → 202 {"job_id": "..."}
GET  /v1/jobs                     → 200 {"jobs": [JobSummary, ...]}
GET  /v1/jobs/events              → text/event-stream of JobSummary (all jobs)
GET  /v1/jobs/{id}/events         → text/event-stream of Progress
GET  /v1/sysdep                   → 200 [SysdepResult, ...]
```

### `internal/queue`

Owns the live `Job` list and per-job `tools.Progress` fan-out. Consumed by
`htoolsd` (and `htools-gui` via the embedded server) — the CLI does not use it
because each CLI run executes exactly one synchronous job.

### `internal/config`

Loads and persists user settings from
`$XDG_CONFIG_HOME/handy-tools/config.yaml` (overridable via
`$HANDY_TOOLS_CONFIG`). YAML is parsed via `gopkg.in/yaml.v3`. Unknown keys are
ignored so stale config keys from older releases parse without error.

## Front-end (`web/`)

A Svelte + Vite + TypeScript + Tailwind project. Built once into
`web/dist/` and embedded into the `htoolsd` binary (and the Wails
`htools-gui` binary) via `go:embed`. The same bundle runs in two hosts:

- **Server mode**: `htoolsd` serves the static SPA on its HTTP port; the
  frontend calls `/v1/*` endpoints on its origin.
- **Desktop mode**: `cmd/htools-gui` (Wails) starts a loopback HTTP server
  and opens a webview to it.

The frontend never has a "Wails-only" branch — feature detection of
`window.runtime` lets it use native file dialogs when present.

## Binary entry points

| Binary | Source | Purpose |
| --- | --- | --- |
| `handy` | `cmd/handy/` | User-facing front door. Bare `handy` opens the desktop app; `handy <verb>` re-execs into the right backend. ~200 LOC, no tool logic, no `internal/tools` import — just a router |
| `htools` | `cmd/htools/` | Non-interactive subcommand CLI (image/archive/pdf/doctor) |
| `htoolsd` | `cmd/htoolsd/` | gRPC + HTTP/SSE server; serves `web/dist/` |
| `htools-mcp` | `cmd/htools-mcp/` | Model Context Protocol server over stdio; wraps every `internal/server/*Handler` so an MCP client (Claude, Cursor) drives the same tools |
| `htools-gui` | `cmd/htools-gui/` | Wails desktop app; bundles `web/dist/` (linux/amd64 + CGO) |

`handy` is a thin dispatcher: it parses the first positional argument, locates
the right backend binary (same directory as `handy` first, then `$PATH`), and
`syscall.Exec`s it so the backend takes over the process — stdin/stdout/stderr/
signals/exit codes flow through unchanged. The four standalone binaries
remain installed and callable directly; `handy` is layered on top.

`htools-mcp` shares the gRPC/HTTP server-side layer rather than re-implementing
it: every MCP tool calls into an `internal/server/*Handler` method, which in
turn delegates to `internal/tools/<feature>/`. Three tool packages (`hash`,
`difftree`, `rename`) that had no `internal/server` handler before that binary
landed got one each, following the existing PDF/Image/Archive pattern. Path
safety uses the same `Options.CheckPath`; the MCP binary's default
`--allow-roots=/` matches the `htools-gui` posture because both run as a
subprocess of the local user.

## API contract

Protos in `api/proto/v1/` are the source of truth for the gRPC services and
are checked-in generated under `gen/`. The HTTP transport mirrors the same
shapes in snake_case JSON, currently hand-mirrored in
[`internal/api/http/types.go`](../internal/api/http/types.go); a
`protoc-gen-ts` step that emits matching TypeScript bindings is a later
optimization.

### Multi-file failure surface

Streaming tools report per-file failures via `tools.Progress.Failures` on
the terminal event; `Inspect()` returns a preflight `Issues` list; rename
and strip-meta support opt-in rollback; archive pack writes atomically via
a `.partial` staging file. The codes, wire shapes, and CLI/MCP/HTTP
projections are documented in
[../docs/FAILURE_HANDLING.md](FAILURE_HANDLING.md).

## Path safety

`htoolsd` refuses to start without `--allow-roots` (or `server.allow_roots`
in the config). Every incoming path goes through `Options.CheckPath` before
any tool is invoked. The check fails closed: an empty roots list rejects
everything, not "serve CWD". This invariant is identical across the gRPC
and HTTP transports because both reuse the same `server.Options`.

The CLI bypasses `CheckPath` entirely — it runs on the user's own machine
against arbitrary paths, the same as any other shell utility.

## System dependencies

Handy Tools is a single Go binary, but some features call out to system
tools when present:

- `unrar` — RAR extraction (single & multi-part)
- `7z` — 7z multi-part extraction
- `pdftoppm`, `pdftotext` — PDF rasterization & text extraction
- `magick` — HEIC decoding, WebP / HEIC encoding

`htools doctor` (and the HTTP `GET /v1/sysdep` endpoint) enumerate which
are installed and which features they unlock. A missing tool surfaces a
structured `MISSING_BINARY` error with an install hint, never a crash.

## Roadmap

The early phases landed on `main`:

1. Repo scaffolding & governance — done
2. CI/CD baseline — done
3. gRPC API contract first — done
4. Core tool packages (image, archive, pdf) — done
5. Settings/config layer — done
6. Non-interactive CLI (`cmd/htools`) — done
7. gRPC server — done
8. HTTP + SSE transport — done
9. Shared `internal/queue/` package — done
10. Wails desktop shell `cmd/htools-gui` — done
11. Release & docs polish — done

### Stretch / next up

- **Plugin system**: tool registry so contributors can ship a new tool by
  adding one package + one proto file, no UI changes.
- **`pdfcpu` library import**: pull page-render into pure Go instead of
  shelling out to Poppler.
- **OS matrix in CI**: restore an ubuntu × macos × windows compile matrix
  ahead of v1.0.0 (currently CI is Linux-only).
