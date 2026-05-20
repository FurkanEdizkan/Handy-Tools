# Architecture

Handy Tools is one Go module that produces several binaries from one shared
tool core. Every UI/transport is a thin adapter over `internal/tools/`.

```text
                       +----------------------------+
                       |   internal/tools/<x>       |   pure Go API per feature
                       |   (image, archive, pdf)    |   no UI, no network
                       +-------------+--------------+
                                     ^
              +----------+-----------+-----------+-----------+
              |          |                       |           |
     +--------+------+ +-+-------------+  +------+--------+  +-------+--------+
     | internal/ui   | | internal/api/ |  | internal/queue|  | internal/server |
     | (Bubble Tea)  | | http (REST+SSE)|  | (planned ph3)|  | (gRPC handlers) |
     +---------------+ +---------------+  +---------------+  +-----------------+
              ^             ^   ^                   ^             ^
              |             |   |                   |             |
       cmd/htools       cmd/htoolsd            (consumed by    cmd/htoolsd
       (TUI)            (HTTP+SSE)              every adapter)  (gRPC)
                              ^
                              |  via embedded webview
                              |
                       cmd/htools-gui
                       (Wails desktop, planned phase 4)

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

### `internal/ui` (Bubble Tea TUI)

The TUI is a sibling adapter, not the primary surface. Layout:

```text
+--------------------+----------------------------------+
| mascot (Wrenly)    |                                  |
+--------------------+   home menu  OR  tool page       |
| state + progress   |   (input · files · output ·      |
+--------------------+    options · run)                |
| queue (jobs+logs)  |                                  |
+--------------------+----------------------------------+
```

Pages don't reach into each other directly; the router (`router.go`) holds
shared state and dispatches `tea.Msg` events.

### `internal/server` (gRPC)

Thin gRPC handlers under `cmd/htoolsd`. Each one calls into
`internal/tools/<x>` and maps the progress channel onto a streaming RPC
response. Path safety lives here as `Options.CheckPath`, reused by every
transport.

### `internal/api/http` (HTTP + Server-Sent Events)

Sibling transport added in pivot Phase 1 (issue #55). Mirrors the gRPC
services with JSON bodies; the streaming response is rendered as SSE so
browsers consume it natively. Reuses the same
`internal/server.{Image,Archive,PDF}Handler` types and `Options.CheckPath`
as gRPC — there is no second copy of the path-safety logic. Endpoints:

```text
POST /v1/image/convert            → 202 {"job_id": "..."}
POST /v1/archive/inspect          → 200 {Inspection}
POST /v1/archive/extract          → 202 {"job_id": "..."}
POST /v1/pdf/to-image             → 202 {"job_id": "..."}
POST /v1/pdf/to-text              → 202 {"job_id": "..."}
POST /v1/pdf/merge                → 202 {"job_id": "..."}
GET  /v1/jobs/{id}/events         → text/event-stream of Progress
GET  /v1/sysdep                   → 200 [SysdepResult, ...]
```

The Phase-1 job tracker is a minimal in-package store; Phase 3 replaces it
with the shared `internal/queue/` package below.

### `internal/queue` *(planned — pivot Phase 3, issue #57)*

Owns the live `Job` list and per-job `tools.Progress` fan-out. Both the
TUI and the HTTP transport subscribe to it, so a job enqueued via either
surface shows up in both. Deletes the simulated `seedQueue()` /
`stepRun()` in `internal/ui/router.go`.

### `internal/config`

Loads and persists user settings from
`$XDG_CONFIG_HOME/handy-tools/config.yaml` (overridable via
`$HANDY_TOOLS_CONFIG`). Hand-rolled minimal YAML parser; replace with
`gopkg.in/yaml.v3` if richer YAML becomes needed.

## Front-end (`web/`) — planned, pivot Phase 2 (issue #56)

A Svelte + Vite + TypeScript + Tailwind project. Built once into
`web/dist/` and embedded into the `htoolsd` binary (and the Wails
`htools-gui` binary) via `go:embed`. The same bundle runs in two hosts:

- **Server mode**: `htoolsd` serves the static SPA on its HTTP port; the
  frontend calls `/v1/*` endpoints on its origin.
- **Desktop mode**: `cmd/htools-gui` (Wails, Phase 4 / issue #58) starts a
  loopback HTTP server and opens a webview to it.

The frontend never has a "Wails-only" branch — feature detection of
`window.runtime` lets it use native file dialogs when present.

## Binary entry points

| Binary | Source | Purpose |
| --- | --- | --- |
| `htools` | `cmd/htools/` | Bubble Tea TUI (secondary surface) |
| `htoolsd` | `cmd/htoolsd/` | gRPC + HTTP/SSE server; serves `web/dist/` when built |
| `htools-gui` | `cmd/htools-gui/` *(planned)* | Wails desktop app; bundles `web/dist/` |
| `snapshot` | `cmd/snapshot/` | Dev tool that regenerates README + brand mascot art; excluded from `.goreleaser.yaml` builds |

## API contract

Protos in `api/proto/v1/` are the source of truth for the gRPC services and
are checked-in generated under `gen/`. The HTTP transport mirrors the same
shapes in snake_case JSON, currently hand-mirrored in
[`internal/api/http/types.go`](../internal/api/http/types.go); a
`protoc-gen-ts` step that emits matching TypeScript bindings is a later
optimization.

## Path safety

`htoolsd` refuses to start without `--allow-roots` (or `server.allow_roots`
in the config). Every incoming path goes through `Options.CheckPath` before
any tool is invoked. The check fails closed: an empty roots list rejects
everything, not "serve CWD". This invariant is identical across the gRPC
and HTTP transports because both reuse the same `server.Options`.

## System dependencies

Handy Tools is a single Go binary, but some features call out to system
tools when present:

- `unrar` — RAR extraction (single & multi-part)
- `7z` — 7z multi-part extraction
- `pdftoppm`, `pdftotext` — PDF rasterization & text extraction
- `magick` — HEIC decoding
- *(planned)* `webkit2gtk-4.0` / `-4.1` on Linux — required by the Wails
  desktop shell (Phase 4)

`htools doctor` (and the HTTP `GET /v1/sysdep` endpoint) enumerate which
are installed and which features they unlock. A missing tool disables the
corresponding action with an inline hint, never a crash.

## Roadmap

The eight initial phases landed on `main`:

1. Repo scaffolding & governance — done
2. CI/CD baseline — done
3. gRPC API contract first — done
4. Core tool packages (image, archive, pdf) — done
5. Settings/config layer — done
6. TUI shell with Bubble Tea — done
7. gRPC server — done
8. Release & docs polish — done

### Current focus: Web GUI pivot

The active multi-phase work is the **Web GUI pivot** — see
[Project #14](https://github.com/users/FurkanEdizkan/projects/14) for live
status and the staged phases. Issues #55–#59 track them:

- Phase 1 — HTTP + SSE transport (#55, landed via #86)
- Phase 2 — Svelte/Vite/TS/Tailwind frontend scaffold (#56)
- Phase 3 — Shared `internal/queue/` package; wire all tools end-to-end (#57)
- Phase 4 — Wails desktop shell `cmd/htools-gui` (#58)
- Phase 5 — Image/PDF preview endpoints for web gallery (#59)

### Stretch / next up

- **Plugin system**: tool registry so contributors can ship a new tool by
  adding one package + one proto file, no UI changes.
- **WebP encoder**: currently disabled (no pure-Go encoder); add via
  `chai2010/webp` once we accept its CGO requirement.
- **Real YAML lib**: swap the hand-rolled config parser for
  `gopkg.in/yaml.v3` once we have other reasons to take that dep.
- **`pdfcpu` library import**: pull merge/split into pure Go instead of
  shelling out to the CLI.
