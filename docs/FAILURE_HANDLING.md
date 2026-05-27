# Failure handling

How Handy Tools behaves when a multi-file operation can't process every
file — and how the API surfaces "which file failed, and why."

## The contract in one paragraph

Every multi-file tool (hash, rename, image batch convert, strip-meta) is
**partial-success-by-default**: a per-file error doesn't abort the batch.
The streaming progress channel emits one `SeverityError` event per failed
file with a classified code on its `Err`, and the terminal event sets
`Progress.Failures []tools.Failure` to a structured list of every failure.
Archive pack is single-archive and atomic: it writes to `<output>.partial`
and renames on success; on any failure the `.partial` file is removed.

## Error codes

Defined in [internal/tools/tools.go](../internal/tools/tools.go). Tools
classify filesystem errors via `tools.ClassifyFSError(err)` so the codes
mean the same thing everywhere.

| Code | When | HTTP | CLI exit |
|---|---|---|---|
| `BAD_REQUEST` | Invalid arguments (empty sources, malformed regex). | 400 | 1 |
| `UNSUPPORTED_INPUT` | Format isn't supported (e.g. RAR creation, unknown image ext). | 415 | 1 |
| `MISSING_BINARY` | Required system binary not on PATH (`unrar`, `7z`, `pdftoppm`, `magick`). | 503 | 1 |
| `PERMISSION_DENIED` | EACCES — current process can't read/write the path. | 403 | 2 |
| `NOT_FOUND` | ENOENT — path doesn't exist. | 404 | 2 |
| `IO_ERROR` | Anything else (disk error, truncated read, etc.). | 500 | 2 |
| `ABORTED` | Caller cancellation (Ctrl+C, request timeout). | 499 | 2 |
| `ROLLBACK_FAILED` | An opt-in rollback step itself failed (best-effort). | n/a (per-Failure code) | n/a |

**Coalescing.** When every per-file failure in a batch shares the same
`Code`, the terminal `Err.Code` surfaces that code (via
`tools.CoalesceFailureCode`) instead of collapsing into `IO_ERROR`. So
`htools hash /missing-1 /missing-2` exits with `NOT_FOUND` on the wire,
not `IO_ERROR`.

## The `Failures` field

`tools.Progress.Failures` is set **only on the terminal event** (the one
with `Completed: true`). Each entry is a `tools.Failure { Path, Code,
Message }`. proto3 omits empty repeated fields, so a fully-successful
batch costs nothing extra on the wire.

The same shape crosses every transport:

| Transport | Field |
|---|---|
| Go in-process | `tools.Progress.Failures []tools.Failure` |
| gRPC / proto | `handytools.v1.Progress.failures` (repeated `Failure { path, code, message }`) |
| HTTP SSE | `progressEvent.failures` (`[{path, code, message}]`) on the terminal `data:` frame |
| MCP `tools/call` result | `structuredContent.failures` (same shape) |
| `htools` CLI `--json` | `progressJSON.failures` on the terminal line |
| `htools` CLI text mode | One `  <path>: <CODE> <message>` line under the summary |

## Preflight: `Inspect()` + `--strict`

Every multi-file tool exposes `Inspect(req)` (or `InspectCompress` for
archive pack, `InspectBatch` for image batch). The result carries an
`Issues []tools.PathIssue` slice populated from `tools.StatInputs(paths)`
and (where applicable) `tools.CheckOutputDirWritable(dir)`. `Issues` is
informational — Inspect never refuses to run the tool. The CLI exposes
`--strict` to abort with exit 2 when any preflight issue is present, and
`--dry-run` to report issues without acting.

MCP exposes preflight via dedicated tools:

- `archive_pack_inspect`
- `hash_inspect`
- `image_batch_inspect`
- `rename_inspect` (returns `plans` + `issues`)

HTTP exposes it via `POST /v1/rename/inspect` (response: `{plans,
issues}`).

## Opt-in rollback (rename + strip-meta)

`rename --rollback-on-error` and `htools strip-meta --rollback-on-error`
flip the per-file behaviour from "skip and continue" to "stop and undo."
A `tools.RollbackStack` accumulates reversible steps as the batch runs;
on the first failure the stack is replayed in reverse. Each rollback
step's failure (if any) becomes a `Failure` entry tagged
`ROLLBACK_FAILED`, but doesn't stop the rest of the rollback.

Strip-meta in-place mode uses a `<source>.handy-bak` sidecar per file.
On batch success these are removed; on rollback they're renamed back.
**Hard-kill caveat:** if the process is SIGKILLed mid-batch the `.bak`
sidecars are left next to processed files. The naming is stable so a
follow-up sweep can collect them.

Archive pack is always atomic — writes to `<output>.partial` then renames
on success, removes on failure. No `--rollback` flag because there's
nothing to opt into.

## How to repro the canonical scenarios

The three situations that drove this design:

### 1. Multi-file batch where one file is unreadable

```sh
mkdir /tmp/demo
echo data > /tmp/demo/a.txt
echo data > /tmp/demo/blocked.txt && chmod 0 /tmp/demo/blocked.txt
./bin/htools hash /tmp/demo/a.txt /tmp/demo/blocked.txt /tmp/demo/ghost.txt
# Exit 0 (a.txt succeeded). stderr lists per-file failures with codes;
# the terminal event's Progress.Failures (visible via --json or MCP/HTTP)
# enumerates the blocked and missing paths.
```

### 2. Aborting before a destructive operation when something looks wrong

```sh
./bin/htools rename --strict --dry-run \
  --pattern '\.JPG$' --replace '.jpg' /tmp/some/dir
# Exit 2 if any source can't be stat'd or any destination dir isn't
# writable. No filesystem changes happen.
```

### 3. Bulk rename that's all-or-nothing

```sh
./bin/htools rename --rollback-on-error \
  --pattern 'IMG_(\d+)\.JPG' --replace 'photo-$1.jpg' /tmp/some/dir
# On first failure, every rename already done is reversed. The terminal
# event carries Failures including both the original failure and any
# ROLLBACK_FAILED entries if a reversal step itself errored.
```

## Test coverage

These scenarios are exercised by automated tests so the contract can be
regression-tested.

| Layer | Test |
|---|---|
| Tool package — hash | [TestHashRunMixedBatchScenario, TestHashRunUnanimousNotFoundCoalesces](../internal/tools/hash/hash_test.go) |
| Tool package — rename | [TestRunMixedBatchScenario, TestRunPermissionDeniedPerFile, TestRunRollbackUndoesEarlierRenames](../internal/tools/rename/rename_test.go) |
| Tool package — image batch | [TestBatchConvertMixedBatchScenario, TestBatchConvertAllFail](../internal/tools/image/image_test.go) |
| Tool package — strip-meta | [TestRunMixedBatchScenario, TestRunInPlaceRollbackRestoresOriginal, TestRunNonInPlaceRollbackDeletesWrittenOutputs](../internal/tools/stripmeta/stripmeta_test.go) |
| Tool package — archive pack | [TestCompressMixedSourceScenario, TestCompressNoPartialOnSuccess, TestCompressRemovesPartialOnFailure](../internal/tools/archive/compress_test.go) |
| Shared helpers | [TestStatInputs, TestCheckOutputDirWritable, TestClassifyFSError, TestCoalesceFailureCode](../internal/tools/tools_test.go), [TestRollbackStackReplaysInReverse](../internal/tools/rollback_test.go) |
| HTTP wire | [TestHashRunFailuresCrossTheWire, TestHashRunFailuresOmittedWhenAllSucceed](../internal/api/http/handlers_failures_test.go) |
| HTTP error codes | [TestStatusForCode](../internal/api/http/errors_test.go) |
| MCP wire | [TestE2E_HashFailuresInRunResult, TestE2E_RenameInspectIssuesPresent](../cmd/htools-mcp/e2e_test.go) |

Run the full suite with:

```sh
make test
```

Run just the scenario tests with:

```sh
go test ./internal/tools/... ./internal/api/http/... ./cmd/htools-mcp/... \
  -run 'MixedBatch|MixedSource|Failures|Rollback|Coalesce|Issues|Permission|NotFound'
```
