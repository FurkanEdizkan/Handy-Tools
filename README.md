<div align="center">

<img src="docs/brand/wrenly.svg" alt="Handy Tools logo" width="160" />

# Handy Tools

**A local-first toolbox for everyday file work**
*Convert images · Slice PDFs · Crack open weird archives*

[![CI](https://github.com/FurkanEdizkan/Handy-Tools/actions/workflows/ci.yml/badge.svg)](https://github.com/FurkanEdizkan/Handy-Tools/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FurkanEdizkan/Handy-Tools)](https://goreportcard.com/report/github.com/FurkanEdizkan/Handy-Tools)
[![Go Reference](https://pkg.go.dev/badge/github.com/furkandedizkan/handy-tools.svg)](https://pkg.go.dev/github.com/furkandedizkan/handy-tools)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/conventional%20commits-1.0.0-orange.svg)](https://www.conventionalcommits.org/en/v1.0.0/)

</div>

---

**Personal Note:** I really don't like uploading stuff to random websites in order to convert file types, also I deal with a lot of multi part archived files, multi part renames and other stuff. So we (me and claude) decided to build something simple which can be used locally both as app and in terminal, so we protect our privacy and I don't have to remember all of the complex terminal commands for each file conversion type.

Hope it will be helpful for someone, open for any feedback or fix.

---

Handy Tools is the small toolbox you reach for when a file is in the wrong
format, an archive won't open, or a PDF needs cutting up. The web UI runs
on your own machine, so **your files never leave it**. Same toolbox is
also a CLI, a local web service, and an MCP server for AI assistants —
pick whichever surface fits the moment.

## What it can do

- **Images** — Convert between PNG, JPEG, GIF, BMP, TIFF, and (with
  ImageMagick) WebP / HEIC. Resize, strip metadata, batch a whole
  folder.
- **Archives** — Pack and extract zip, tar, gz, bz2, zst pure-Go;
  multi-part RAR and 7z when the matching binary is installed.
- **PDFs** — Merge, split, extract text, render pages to images.
- **Integrity** — Hash files (MD5 / SHA-256 / BLAKE3), verify a
  sha256sum manifest, diff two directory trees.
- **Rename** — Regex-based batch rename with a dry-run preview before
  anything moves.
- **Diagnostics** — One `handy doctor` call tells you which optional
  system tools are present and what each one unlocks.

Full command reference: [docs/CLI.md](docs/CLI.md). Architecture
overview: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Your files stay on your machine

- The web UI is served by a process you run locally — it binds to a
  port on your own host, no SaaS, no account.
- The `htoolsd` service refuses to start without an explicit
  `--allow-roots` list, and every path argument is validated against
  that list before any tool runs.
- The binaries make **no outbound network calls** at runtime. No
  telemetry, no update pings.
- The MCP server runs as a subprocess of your AI client (Claude Code,
  Cursor, Claude Desktop), so the chatbot calls the same sandbox the
  service does.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and
[internal/server/server.go](internal/server/server.go) for the exact
path-safety contract.

## Get it running

```sh
curl -fsSL https://raw.githubusercontent.com/FurkanEdizkan/Handy-Tools/main/install.sh | sh
handy                # opens the desktop app
```

That's it. Tarball-only / manual / Windows-via-WSL paths and the full
installer flag list live in [docs/INSTALL.md](docs/INSTALL.md).

## Four ways to use it

Whichever surface you pick, you're calling the same Go core
underneath — behavior and error codes are identical across them.

- **Desktop app.** Bare `handy` launches the Wails GUI (linux/amd64)
  with file dialogs, drag-and-drop, and live previews.
- **CLI.** `handy convert in.png --format jpeg --out out.jpg` (also
  reachable as `htools convert ...`). Subcommand dispatch, `--quiet`
  and `--json` for scripts. See [docs/CLI.md](docs/CLI.md).
- **Local service + web UI.** `handy serve --listen :7777 --allow-roots
  /srv/uploads` runs the gRPC + HTTP/SSE server with the embedded Svelte
  web UI. Reachable in a browser at `http://localhost:7777`.
- **AI assistants (MCP).** `handy mcp` runs the Model Context Protocol
  server on stdio. Wire it into Claude Code / Cursor / Claude Desktop
  with a small `.mcp.json` and your chatbot can drive every tool —
  [AGENTS.md](AGENTS.md) has a copy-paste example.

## For AI assistants

Handy Tools ships a Model Context Protocol server. Wire `htools-mcp`
(or `handy mcp`) into your client and the chatbot gets sixteen tools:
image / archive / PDF / hash / rename / diff-tree / doctor.

The chatbot's reference is **[AGENTS.md](AGENTS.md)** — a single read
covering the local-first contract, every tool's input schema, the
error codes, and behavioral guidance. Point your assistant there at
the start of a session.

## Status

Pre-1.0, on calver pre-releases (`v{YYYY}.{M}.{D}-beta.{N}`). The CLI,
the `htoolsd` server, the embedded web UI, the MCP server, and the Wails
desktop app all run real image / archive / PDF jobs through a shared
`internal/queue/`. See [Project #14] for live status and
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#roadmap) for the
longer-horizon roadmap.

[Project #14]: https://github.com/users/FurkanEdizkan/projects/14

## Where to go next

- **[docs/INSTALL.md](docs/INSTALL.md)** — installer flags, manual
  install, optional system deps, build from source.
- **[docs/CLI.md](docs/CLI.md)** — every subcommand and flag.
- **[docs/CONFIG.md](docs/CONFIG.md)** — the YAML config file, paths,
  and defaults.
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** — the layered design
  and how the four surfaces share one core.
- **[docs/FAILURE_HANDLING.md](docs/FAILURE_HANDLING.md)** — what
  happens when one file in a batch can't be processed: error codes,
  the structured `failures` field, preflight `--strict`, and opt-in
  rollback.
- **[AGENTS.md](AGENTS.md)** — the MCP / chatbot guide.
- **[docs/RELEASES.md](docs/RELEASES.md)** — calver scheme and the
  automated release pipeline.
- **[CONTRIBUTING.md](CONTRIBUTING.md)** and
  **[docs/PR_GUIDELINES.md](docs/PR_GUIDELINES.md)** — branch flow,
  Conventional Commits, the `test` → `main` promotion.

## Acknowledgements

Handy Tools binds several third-party libraries directly — pdfcpu for
PDF operations, klauspost/compress and dsnet/compress for archives,
`golang.org/x/image`, Wails for the desktop shell, gRPC, and
Svelte/Tailwind for the web UI. Every one is credited with its
copyright and license in [NOTICE](NOTICE).

## License

[MIT](LICENSE).
