# Architecture

Handy is split into three layers, each depending only on the layer above it:

```
                +----------------------------+
                |   internal/tools/<x>       |   pure Go API per feature
                |   (image, archive, pdf)    |   no UI, no network
                +-------------+--------------+
                              ^
              +---------------+---------------+
              |                               |
     +--------+--------+              +-------+--------+
     |  internal/ui    |              | internal/server |
     |  (Bubble Tea)   |              | (gRPC handlers) |
     +-----------------+              +-----------------+
              ^                               ^
              |                               |
       cmd/handy (TUI)                  cmd/handyd (server)
```

## Packages

### `internal/tools/<feature>`

Each feature is a plain Go package with:

- A request/result struct that mirrors the proto.
- A function that performs the work and returns a progress channel.
- An `Inspect()` function where preflight matters (e.g. detecting multi-part
  archives) so callers can confirm before destructive work.

These packages are the **only** place that touches files, runs external
binaries, or knows anything about formats.

### `internal/ui`

Bubble Tea models. The router stacks pages: Home, FileBrowser, ToolView,
Settings. The mascot is a reusable component with idle / working / success /
error / thinking states; each page can `mascot.Set(state)` via a `tea.Cmd`.

### `internal/server`

Thin gRPC handlers. Each one calls into `internal/tools/<x>` and maps progress
channels onto streaming RPC responses. Handlers contain no business logic.

### `internal/config`

Loads and persists user settings from the XDG config directory
(`$XDG_CONFIG_HOME/handy/config.yaml` or `~/.config/handy/config.yaml`).

## API contract

Protos in `api/proto/v1/` are the source of truth for both clients and the TUI.
We generate Go bindings into `gen/` and check them in so contributors don't
need `protoc` to build.

## System dependencies

Handy is a single Go binary, but some features call out to system tools when
present:

- `unrar` — RAR extraction (single & multi-part)
- `7z` — 7z multi-part extraction
- `pdftoppm`, `pdftotext` — PDF rasterization & text extraction
- `magick` — HEIC decoding

`handy doctor` enumerates which are installed and which features they unlock.
A missing tool disables the corresponding action with an inline hint, never a
crash.

## Roadmap

See the phases listed in the project plan:

1. Repo scaffolding & governance ............ this PR
2. CI/CD baseline
3. gRPC API contract first
4. Core tool packages (image, archive, pdf)
5. Settings/config layer
6. TUI shell with Bubble Tea
7. gRPC server
8. Release & docs polish
9. Stretch: plugin system, web UI
