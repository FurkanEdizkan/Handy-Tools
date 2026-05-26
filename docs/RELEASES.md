# Releases

Releases are **automatic**: every CI-green commit on `main` gets a
calver pre-release tag and a published GitHub release. There is no
manual version bump.

## Versioning scheme

While the project is pre-1.0 the tag scheme is:

```text
v{YYYY}.{M}.{D}-beta.{N}
```

`N` is one more than the highest `-beta` counter already published for
that UTC date. For example, the third release on 2026-05-14 becomes
`v2026.5.14-beta.3` and ships as a GitHub **Pre-release**.

When `v1.0.0` is eventually cut we'll switch back to plain semver and
drop the per-day counter; until then the calver + pre-release model
keeps the cadence honest about the project's pre-alpha status.

## End-to-end flow

1. Push a commit to `main`.
2. CI runs (lint, tests, fuzz, build).
3. On CI success,
   [`.github/workflows/auto-tag.yml`](../.github/workflows/auto-tag.yml)
   fires via `workflow_run`, computes the next calver tag, and pushes
   it. It then invokes
   [`.github/workflows/release.yml`](../.github/workflows/release.yml)
   directly via `workflow_call` (a tag-push trigger wouldn't fire
   downstream workflows because the tag was pushed with
   `GITHUB_TOKEN`).
4. GoReleaser builds linux/darwin × amd64/arm64 archives, computes
   `checksums.txt`, and publishes the GitHub release.
5. The `release: published` event triggers
   [`.github/workflows/verify-install.yml`](../.github/workflows/verify-install.yml),
   which runs `install.sh` against the new release on Ubuntu
   22.04/24.04 and macOS 14 and asserts `htools --version` matches.
   This catches installer/release-format drift before users do.

## Release-note grouping

Release notes use GoReleaser's commit-message grouping:

- `feat:` → **Changes**
- `fix:`  → **Fixes**
- everything else under **Other**

`docs/test/chore/ci:` commits are filtered out. The release title is
`Handy Tools {version}`.

## `version.txt`

[internal/buildinfo/version.txt](../internal/buildinfo/version.txt)
(`0.0.0-dev`) is a placeholder used by local builds only. Release
binaries get the real calver version baked in via `-ldflags -X
buildinfo.Version=…` from
[.goreleaser.yaml](../.goreleaser.yaml).
`TestEmbeddedVersionIsSemver` in
`internal/buildinfo/buildinfo_test.go` validates that `version.txt`
parses as semver — `0.0.0-dev` is valid, so the gate stays green.

## Installer gotcha

Every calver build is marked Pre-release on GitHub (`prerelease: auto`
in `.goreleaser.yaml`). GitHub's `/releases/latest` API endpoint
deliberately skips prereleases, so [install.sh](../install.sh) lists
`/releases?per_page=10` and picks the newest non-draft entry instead.
Don't "simplify" the installer back to `/releases/latest` without first
flipping releases to non-prerelease — and that won't be appropriate
until v1.0.0.
