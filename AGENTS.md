# AGENTS.md — Handy Tools for chatbots

> If you're an MCP-capable chatbot (Claude Code, Claude Desktop, Cursor,
> or any other client that speaks the [Model Context Protocol]) and a
> user has wired Handy Tools into your session, this file is for you.
> Read it once at the start of a session; it's everything you need to
> drive the toolbox responsibly.
>
> If you're a human, this file doubles as the canonical MCP reference.
> The user-facing intro lives in [README.md](README.md); the
> implementation lives under [cmd/htools-mcp/](cmd/htools-mcp/) and
> [internal/tools/](internal/tools/).

[Model Context Protocol]: https://modelcontextprotocol.io

## What Handy Tools is, from a chatbot's perspective

Handy Tools is a **local-first** file toolbox: image conversion, archive
extraction/packing, PDF utilities, hashing, directory diffing, and
regex rename. It runs entirely on the user's machine — nothing is
uploaded, nothing phones home. When wired into your session, the
binary `htools-mcp` is launched as a subprocess by the MCP client and
speaks JSON-RPC over stdio. You call its tools the same way you call
any other MCP tool.

The same core ships four other surfaces too — a CLI, a gRPC + HTTP
server with an embedded web UI, and a Wails desktop app — but for you,
only the MCP surface matters.

## Wiring (for the user, in case they ask)

Drop one of these into `~/.claude.json` or a per-project `.mcp.json`:

```json
{
  "mcpServers": {
    "handy-tools": {
      "command": "/absolute/path/to/handy-tools/bin/htools-mcp",
      "args": ["--allow-roots=/absolute/path/to/work-tree"]
    }
  }
}
```

`handy mcp` and `htools-mcp` are interchangeable — `handy mcp` re-execs
into `htools-mcp` preserving stdin/stdout.

`htools-mcp` defaults to `--allow-roots=/` because it already runs as
the local user. Tighten it (`--allow-roots=/path/a,/path/b`) when you
want to sandbox the chatbot to a specific tree.

## The local-first contract

A few invariants you can rely on, and rules you must follow:

1. **All paths are absolute.** Tildes (`~`) and relative paths are
   rejected by the path sandbox before a tool ever sees them. If the
   user gives you a relative path, resolve it first.
2. **Stay inside `--allow-roots`.** Any `source`, `sources[]`,
   `destination`, `output_file`, `output_dir`, `manifest`, `a`, or
   `b` argument is checked against `Options.CheckPath`. Paths that
   escape via `..` or land outside the allow-list return a structured
   error — they will never silently succeed.
3. **No internet, no telemetry.** The binary opens no outbound
   sockets. Don't assume URLs are fetchable; download what you need
   into the allow-rooted tree first and pass the absolute path.
4. **Tools never panic.** Failures arrive as structured `*tools.Error`
   values with stable codes — handle them, don't retry blindly.
5. **Progress events are advisory.** A streamed `Progress` event is
   useful narration for the user, but the terminal result struct (or
   structured error) is the authoritative outcome.

The error codes, defined in
[internal/tools/tools.go](internal/tools/tools.go):

| Code                | Meaning                                                                | What to do                                                                  |
| ------------------- | ---------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| `MISSING_BINARY`    | An optional system tool isn't on PATH (`unrar`, `7z`, `pdftoppm`, …). | Surface the install hint to the user; offer to call `doctor` for the list. |
| `UNSUPPORTED_INPUT` | The file is in a format the tool can't handle.                         | Explain the format mismatch; suggest a conversion path if one exists.       |
| `BAD_REQUEST`       | Inputs are malformed (e.g. missing required field, conflicting opts).  | Re-read your arguments against the schema below; don't retry as-is.         |
| `IO_ERROR`          | Filesystem trouble — permission denied, disk full, source missing.     | Check the path exists and is readable; ask the user before retrying.       |
| `ABORTED`           | The user (or the MCP client) cancelled the call.                       | Stop. Don't restart without explicit confirmation.                          |

## Behavioral guidance

These are the habits that make a chatbot a good citizen of the
toolbox:

1. **Call `doctor` first.** Once per session is plenty. It tells you
   which optional binaries are available, so you don't propose
   `pdf_render` only to fail with `MISSING_BINARY`.
2. **Prefer dry-runs for destructive work.** `archive_inspect` before
   `archive_extract`; `rename_inspect` before `rename_run`. For
   batch image converts that might overwrite outputs, default
   `overwrite` to `false` and ask the user before flipping it.
3. **One absolute path per field.** Don't pack multiple paths into a
   single string. The schema is explicit about which fields are
   arrays.
4. **Read structured results, not text.** Each tool returns both a
   human-readable header and a JSON `structuredContent`. Parse the
   structured form when you need to compose multi-step plans.
5. **Don't speculate about formats.** If you don't know the input
   format, call `archive_inspect` or look at the file extension —
   don't guess from filename heuristics and pass `format` to
   `archive_compress`.

## Tool catalog

Source of truth: [cmd/htools-mcp/tools_image.go](cmd/htools-mcp/tools_image.go),
[cmd/htools-mcp/tools_archive.go](cmd/htools-mcp/tools_archive.go),
[cmd/htools-mcp/tools_pdf.go](cmd/htools-mcp/tools_pdf.go),
[cmd/htools-mcp/tools_misc.go](cmd/htools-mcp/tools_misc.go).

Field types follow JSON conventions: `string`, `int`, `bool`,
`string[]`. Optional fields are marked. When two fields are mutually
exclusive, the description says so; passing both surfaces a
`BAD_REQUEST`.

### Images

#### `image_convert`

Convert a single image between formats. WebP / HEIC **encoding**
require the `magick` binary; decoding WebP is pure-Go.

| Field            | Type     | Required | Notes                                                                       |
| ---------------- | -------- | -------- | --------------------------------------------------------------------------- |
| `source`         | string   | yes      | Absolute path of the source image.                                          |
| `target_format`  | string   | yes      | `png`, `jpeg`, `gif`, `bmp`, `tiff`, `webp`, or `heic`.                     |
| `output_file`    | string   | one of   | Absolute output file path. Mutually exclusive with `output_dir`.            |
| `output_dir`     | string   | one of   | Absolute directory; the tool picks the basename.                            |
| `quality`        | int      | no       | JPEG quality 1..100; ignored for lossless formats.                          |
| `max_width`      | int      | no       | Resize so width ≤ `max_width`, preserving aspect ratio. `0` = keep.        |
| `max_height`     | int      | no       | Resize so height ≤ `max_height`, preserving aspect ratio. `0` = keep.      |
| `overwrite`      | bool     | no       | Replace an existing output instead of disambiguating with `-1`, `-2`, …    |
| `strip_metadata` | bool     | no       | Best-effort: drop EXIF/IPTC/XMP on encode.                                  |

*Call this when:* the user wants a single image converted, resized, or
both. For "convert this whole folder", use `image_batch_convert`.

#### `image_batch_convert`

Same as `image_convert`, but for many sources into one `output_dir`.
Emits one progress event per file plus a terminal summary.

| Field            | Type     | Required | Notes                                                                     |
| ---------------- | -------- | -------- | ------------------------------------------------------------------------- |
| `sources`        | string[] | yes      | Absolute paths of source images.                                          |
| `target_format`  | string   | yes      | Same set as `image_convert`.                                              |
| `output_dir`     | string   | yes      | Absolute directory all converted files are written into.                  |
| `quality`        | int      | no       | JPEG quality 1..100.                                                      |
| `max_width`      | int      | no       | Resize cap, `0` = keep.                                                   |
| `max_height`     | int      | no       | Resize cap, `0` = keep.                                                   |
| `overwrite`      | bool     | no       | Replace existing outputs instead of suffixing.                            |
| `strip_metadata` | bool     | no       | Drop metadata on re-encode.                                               |
| `parallelism`    | int      | no       | Worker pool size; `0` (default) auto-sizes to the host's GOMAXPROCS.      |

#### `image_strip_meta`

Re-encode images into `output_dir` with EXIF/IPTC/XMP stripped; **keeps
each source in its original format**. Use this when the user's intent
is privacy, not format conversion.

| Field        | Type     | Required | Notes                                                                  |
| ------------ | -------- | -------- | ---------------------------------------------------------------------- |
| `sources`    | string[] | yes      | Absolute paths of source images.                                       |
| `output_dir` | string   | yes      | Absolute directory the cleaned copies are written into.                |
| `quality`    | int      | no       | JPEG quality 1..100 for the re-encode.                                 |
| `overwrite`  | bool     | no       | Replace an existing output instead of suffixing.                       |

### Archives

#### `archive_inspect`

Report format, multi-part status, entry count, and whether a required
binary (`unrar` / `7z`) is on PATH. **Call this before
`archive_extract`** when you don't already know the archive shape.

| Field    | Type   | Required | Notes                              |
| -------- | ------ | -------- | ---------------------------------- |
| `source` | string | yes      | Absolute path of the archive.      |

#### `archive_extract`

Extract an archive into `destination`. zip / tar variants are pure-Go;
RAR and 7z need the `unrar` / `7z` binaries.

| Field             | Type     | Required | Notes                                                                                      |
| ----------------- | -------- | -------- | ------------------------------------------------------------------------------------------ |
| `source`          | string   | yes      | Absolute path of the archive to extract.                                                   |
| `destination`     | string   | yes      | Absolute directory to extract into.                                                        |
| `parts`           | string[] | no       | Multi-part volume paths (`.partN.rar`, `.7z.00N`). Use `archive_inspect` first to enumerate. |
| `password`        | string   | no       | Password for encrypted archives.                                                           |
| `overwrite`       | bool     | no       | Overwrite existing files in `destination`.                                                 |
| `auto_multi_part` | bool     | no       | Proceed with multi-part archives without prompting.                                        |

#### `archive_compress`

Pack `sources` into a single archive. zip / tar variants are pure-Go; 7z
needs `7z`; RAR creation is **not supported**.

| Field               | Type     | Required | Notes                                                                                   |
| ------------------- | -------- | -------- | --------------------------------------------------------------------------------------- |
| `sources`           | string[] | yes      | Absolute paths to pack (files or directories; directories are added recursively).       |
| `output`            | string   | yes      | Absolute path of the archive to write.                                                  |
| `format`            | string   | no       | `zip`, `tar`, `tar.gz`, `tar.bz2`, `tar.zst`, `7z`. Empty = infer from output extension.|
| `password`          | string   | no       | Only honored by `7z`.                                                                   |
| `compression_level` | int      | no       | `0`–`9` where the format supports it.                                                   |

### PDFs

#### `pdf_merge`

Concatenate two or more PDFs into a single output (pure-Go, no external
binary).

| Field         | Type     | Required | Notes                                                  |
| ------------- | -------- | -------- | ------------------------------------------------------ |
| `sources`     | string[] | yes      | Absolute paths of two or more PDFs to concatenate.     |
| `output_file` | string   | yes      | Absolute path of the merged PDF to write.              |

#### `pdf_split`

Split a PDF by either an explicit page range (`from`/`to`) or every N
pages. Pure-Go.

| Field        | Type   | Required | Notes                                                                                |
| ------------ | ------ | -------- | ------------------------------------------------------------------------------------ |
| `source`     | string | yes      | Absolute path of the PDF to split.                                                   |
| `output_dir` | string | yes      | Absolute directory the per-chunk PDFs are written to.                                |
| `from`       | int    | no       | 1-based first page of one explicit range. Use `0` with `every_n` for chunk mode.     |
| `to`         | int    | no       | 1-based last page (inclusive); `0` = last page.                                      |
| `every_n`    | int    | no       | Chunk every N pages instead of a range. Mutually exclusive with `from`/`to`.         |

#### `pdf_render`

Rasterise PDF pages to PNG (default) or JPEG. **Requires `pdftoppm`**
(`poppler-utils`).

| Field        | Type   | Required | Notes                                                          |
| ------------ | ------ | -------- | -------------------------------------------------------------- |
| `source`     | string | yes      | Absolute path of the PDF to rasterise.                         |
| `output_dir` | string | yes      | Absolute directory the page images are written to.             |
| `from`       | int    | no       | 1-based first page; `0` = start.                               |
| `to`         | int    | no       | 1-based last page (inclusive); `0` = end.                      |
| `dpi`        | int    | no       | Render DPI. `0` lets the tool pick a sensible default (~150).  |
| `jpeg`       | bool   | no       | Emit JPEG instead of the PNG default.                          |

#### `pdf_text`

Extract plain text from a PDF. **Requires `pdftotext`** (`poppler-utils`).

| Field         | Type   | Required | Notes                                                            |
| ------------- | ------ | -------- | ---------------------------------------------------------------- |
| `source`      | string | yes      | Absolute path of the PDF.                                        |
| `output_file` | string | no       | Absolute path of the `.txt` to write. Empty defaults to `{source}.txt`. |
| `from`        | int    | no       | 1-based first page; `0` = start.                                 |
| `to`          | int    | no       | 1-based last page (inclusive); `0` = end.                        |
| `layout`      | bool   | no       | Preserve original column layout (`pdftotext -layout`).           |

### Integrity

#### `hash`

Compute MD5 / SHA-256 / BLAKE3 digests of one or more files. The
per-file progress message is in sha256sum format (`DIGEST␣␣PATH`), so
you can pipe the output into `hash_verify` later.

| Field         | Type     | Required | Notes                                                                   |
| ------------- | -------- | -------- | ----------------------------------------------------------------------- |
| `sources`     | string[] | yes      | Absolute paths of files to hash.                                        |
| `algo`        | string   | no       | `md5`, `sha256` (default), or `blake3`.                                 |
| `parallelism` | int      | no       | Worker pool size; `0` (default) auto-sizes to GOMAXPROCS.               |

#### `hash_verify`

Verify a sha256sum-format manifest: recomputes each entry's digest and
reports OK / failed / missing counts plus per-row detail.

| Field      | Type   | Required | Notes                                                              |
| ---------- | ------ | -------- | ------------------------------------------------------------------ |
| `manifest` | string | yes      | Absolute path of a `sha256sum`-format manifest (`DIGEST␣␣PATH`).   |
| `algo`     | string | no       | `md5`, `sha256` (default), or `blake3`. Must match the manifest.   |

#### `diff_tree`

Compare two directory trees and report added / removed / changed paths.
Mode `mtime` is fast; `hash` reads every byte but is authoritative.

| Field         | Type   | Required | Notes                                                                                 |
| ------------- | ------ | -------- | ------------------------------------------------------------------------------------- |
| `a`           | string | yes      | Absolute path of the first directory tree.                                            |
| `b`           | string | yes      | Absolute path of the second directory tree.                                           |
| `mode`        | string | no       | `mtime` (default, fast) or `hash` (slow, authoritative).                              |
| `parallelism` | int    | no       | Worker pool size for `hash` mode; ignored in `mtime`. `0` auto-sizes.                 |

### Rename

#### `rename_inspect`

Dry-run a regex-based rename batch and return the plan. **No
filesystem mutation.** Always call this first when the user is about to
rename anything they can't trivially undo.

| Field          | Type     | Required | Notes                                                                          |
| -------------- | -------- | -------- | ------------------------------------------------------------------------------ |
| `sources`      | string[] | yes      | Absolute paths to consider for renaming.                                       |
| `pattern`      | string   | yes      | Go regex matched against each basename.                                        |
| `replace`      | string   | yes      | Replacement template (regex backrefs allowed).                                 |
| `on_collision` | string   | no       | `error` (default), `skip`, or `suffix` (appends `-1`, `-2`, …).                |

The result has a `plans` array; each entry says what would move
from where to where.

#### `rename_run`

Execute the same regex-based rename batch. Same input shape as
`rename_inspect`.

### Diagnostics

#### `doctor`

Report which optional system binaries (`unrar`, `7z`, `pdftoppm`,
`pdftotext`, `magick`) are available on PATH and what features they
unlock. Takes no input.

Call this **once at the start of a session** to learn which downstream
tools will work without surprises. If a later tool returns
`MISSING_BINARY`, re-call `doctor` and surface the install hint from
its result.

## Cross-links

- [README.md](README.md) — the user-facing intro and link hub.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — the layered design;
  read this if you want to understand why the MCP / HTTP / gRPC / CLI
  surfaces never drift from one another.
- [docs/CLI.md](docs/CLI.md) — the same toolbox via shell, useful if
  the user wants to script something you've prototyped.
- [docs/CONFIG.md](docs/CONFIG.md) — the YAML config, including
  `server.allow_roots`.
- [CLAUDE.md](CLAUDE.md) — read this *only* if you're editing the
  repo's source code (not just calling its tools).
- [.mcp.json](.mcp.json) — a working wiring example.
