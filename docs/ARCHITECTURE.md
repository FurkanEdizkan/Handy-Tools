# Architecture

Handy Tools is split into three layers, each depending only on the layer above
it:

```text
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
       cmd/htools (TUI)                  cmd/htoolsd (server)
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

Bubble Tea models, arranged as a two-pane layout:

```text
+--------------------+----------------------------------+
| mascot (Wrenly)    |                                  |
+--------------------+   home menu  OR  tool page       |
| state + progress   |   (input · files · output ·      |
+--------------------+    options · run)                |
| queue (jobs+logs)  |                                  |
+--------------------+----------------------------------+
```

- **Left column (fixed)**: `mascot.Model` (idle / thinking / working /
  success / error states), the state block (current task + progress bar),
  and the queue panel with expandable per-job stderr-style logs.
- **Right column**: the **Home** tool catalog or a **Tool detail page**
  (`toolpage.go`) with mode-specific UI for image conversion, archive
  pack/extract, PDF utilities, and `htools doctor`.

The router (`router.go`) owns the shared state (mascot, queue, progress,
toast) and dispatches cross-page messages (`OpenTool`, `GoHome`, `RunJob`,
`MascotMsg`). Pages don't reach into each other directly — every nav or
state change is a `tea.Msg`.

### `internal/server`

Thin gRPC handlers. Each one calls into `internal/tools/<x>` and maps progress
channels onto streaming RPC responses. Handlers contain no business logic.

### `internal/config`

Loads and persists user settings from the XDG config directory
(`$XDG_CONFIG_HOME/handy-tools/config.yaml` or `~/.config/handy-tools/config.yaml`,
overridable via `$HANDY_TOOLS_CONFIG`).

## API contract

Protos in `api/proto/v1/` are the source of truth for both clients and the TUI.
We generate Go bindings into `gen/` and check them in so contributors don't
need `protoc` to build.

## System dependencies

Handy Tools is a single Go binary, but some features call out to system tools
when present:

- `unrar` — RAR extraction (single & multi-part)
- `7z` — 7z multi-part extraction
- `pdftoppm`, `pdftotext` — PDF rasterization & text extraction
- `magick` — HEIC decoding

`htools doctor` enumerates which are installed and which features they unlock.
A missing tool disables the corresponding action with an inline hint, never a
crash.

## Roadmap

The eight initial phases are now landed on `main`:

1. Repo scaffolding & governance — done
2. CI/CD baseline — done
3. gRPC API contract first — done
4. Core tool packages (image, archive, pdf) — done
5. Settings/config layer — done
6. TUI shell with Bubble Tea — done
7. gRPC server — done
8. Release & docs polish — done

### Stretch / next up

- **Plugin system**: tool registry so contributors can ship a new tool by
  adding one package + one proto file, no UI changes.
- **WebP encoder**: currently disabled (no pure-Go encoder); add via
  `chai2010/webp` once we accept its CGO requirement.
- **Real YAML lib**: swap the hand-rolled config parser for
  `gopkg.in/yaml.v3` once we have other reasons to take that dep.
- **`pdfcpu` library import**: pull merge/split into pure Go instead of
  shelling out to the CLI.
- **Web UI**: a thin gRPC-Web or REST gateway over `htoolsd` for the
  "deploy as a service" use case.
- **Chrome extension**: a popup that calls into a local or hosted `htoolsd`
  for quick file conversions from the browser.
