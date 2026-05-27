# Command reference

Everything below works two ways: through `handy` (the friendly front door)
or by calling the underlying binary directly. Pick whichever you like.

For an architectural overview of how the front door re-execs into each
backend, see [docs/ARCHITECTURE.md](ARCHITECTURE.md).

## Desktop app

```sh
handy                                  # ↔ htools-gui
```

Bare `handy` launches the desktop app (Wails, linux/amd64). On platforms
without the desktop build it prints a friendly note.

## Images

```sh
# Image conversion (single source, single output file):
handy convert photo.png --format jpeg --quality 80 --out photo.jpg
# ↔ htools convert photo.png --format jpeg --quality 80 --out photo.jpg

# Batch convert into a directory:
handy convert a.png b.png c.png --format webp --out ./converted
```

## Archives

```sh
# Pack a zip:
handy pack ./project --format zip --output project.zip

# Extract any archive (zip / tar.gz / 7z / rar / …):
handy extract bundle.tar.gz --out ./extracted

# Inspect first (good for multi-part RAR / 7z):
handy inspect bundle.part1.rar
```

## PDFs

```sh
handy pdf merge a.pdf b.pdf --out merged.pdf
handy pdf split big.pdf --pages 1-20 --out ./parts
handy pdf render report.pdf --pages 1-3 --dpi 200 --out ./pages
handy pdf text report.pdf --layout --out report.txt
```

## Integrity & rename

```sh
# Hash a tree:
handy hash *.tar.gz --algo sha256

# Verify a manifest:
handy hash --verify checksums.txt

# Compare two trees:
handy diff-tree ./before ./after --mode mtime

# Regex rename (dry-run first):
handy rename --inspect '*.jpeg' --pattern '\.jpeg$' --replace '.jpg'
handy rename '*.jpeg' --pattern '\.jpeg$' --replace '.jpg'

# Preflight + abort on any unreadable/missing source or unwritable target:
handy rename --strict --pattern '\.JPG$' --replace '.jpg' /photos
handy hash   --strict /backup/*

# All-or-nothing rename: stop on first failure and undo every move so far.
handy rename --rollback-on-error --pattern 'IMG_(\d+)\.JPG' --replace 'photo-$1.jpg' /photos

# Strip metadata with rollback (in-place + .handy-bak snapshots):
handy strip-meta --in-place --rollback-on-error /photos/*.jpg
```

See [FAILURE_HANDLING.md](FAILURE_HANDLING.md) for the full contract:
when a batch partially fails, the terminal Progress event carries a
structured `failures` list (visible in `--json`, MCP `structuredContent`,
and HTTP SSE), and the exit code reflects the unanimous failure code
(`PERMISSION_DENIED` and `NOT_FOUND` both exit 2; `BAD_REQUEST`,
`UNSUPPORTED_INPUT`, `MISSING_BINARY` exit 1).

## Service mode

```sh
# Run the gRPC + HTTP/SSE server + embedded web UI:
handy serve --listen :7777 --allow-roots /srv/uploads,/srv/output
# ↔ htoolsd --listen :7777 --allow-roots /srv/uploads,/srv/output

# MCP server (wire it into Claude Code / Cursor / Claude Desktop):
handy mcp                              # ↔ htools-mcp

# Probe the running gRPC service with grpcurl:
grpcurl -plaintext localhost:7777 list
grpcurl -plaintext localhost:7777 \
  handytools.v1.ArchiveService/Inspect \
  <<<'{"source":{"path":"/srv/uploads/foo.7z.001"}}'
```

`handy serve` (and `htoolsd`) refuses to start without `--allow-roots`
(or `server.allow_roots` in [config](CONFIG.md)). Every `FileRef.path`
is run through `Options.CheckPath` before any tool is called — paths
outside an allow-root, or that try to escape via `..`, are rejected.

## Diagnostics & version

```sh
# Doctor: which optional tools are present, and what each one unlocks
handy doctor

# Version: semver, short commit, build date, GOOS/GOARCH
handy --version

# Help:
handy --help
```

## Common flags

Every subcommand accepts:

- `--quiet` — suppress per-event progress lines.
- `--json`  — emit one JSON object per progress event on stdout (good
  for piping into other tooling).
