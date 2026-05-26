# Configuration

Handy Tools reads a single YAML file at startup. The shape and defaults
live in [internal/config/config.go](../internal/config/config.go); this
page is a reader-friendly summary.

## Where the config lives

Resolved in this order; the first existing file wins:

```text
$HANDY_TOOLS_CONFIG                          (explicit override)
$XDG_CONFIG_HOME/handy-tools/config.yaml     (XDG, when set)
~/.config/handy-tools/config.yaml            (default)
```

The on-disk YAML is decoded with
[`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3). Unknown keys are
silently ignored so configs stay forward-compatible — and partial files
round-trip safely against `Defaults()`.

## Minimal example

```yaml
image:
  default_jpeg_quality: 90
pdf:
  default_dpi: 150
archive:
  auto_extract_multi_part: false
  overwrite_by_default: false
server:
  listen: ":7777"
  allow_roots:
    - /srv/uploads
    - /srv/output
recent: []
```

## The `server.allow_roots` key

`htoolsd` and `htools-mcp` refuse to act on paths outside this list.
Every `FileRef.path` is run through `Options.CheckPath` before any
tool sees it, and paths that try to escape via `..` are rejected.

`htoolsd` won't even start with an empty `allow_roots` (or
`--allow-roots`) — the fail-closed default means "no roots, no
service." `htools-mcp` defaults to `--allow-roots=/` because it runs as
a subprocess of the local user, the same posture `htools-gui` uses.

See [internal/server/server.go](../internal/server/server.go) and the
test suite at
[internal/server/server_test.go](../internal/server/server_test.go) for
the exact contract.
