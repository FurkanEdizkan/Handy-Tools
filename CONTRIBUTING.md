# Contributing to Handy Tools

Thanks for your interest in Handy Tools! This document explains how to set up
your environment, the branching model, and the rules we ask contributors to
follow.

## Code of conduct

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).
Be kind. Assume good faith.

## Prerequisites

- Go 1.22+
- `make`
- `buf` (for protobuf) — see [installation](https://buf.build/docs/installation)
- `golangci-lint` — see [installation](https://golangci-lint.run/welcome/install/)
- Optional system tools to exercise all features locally: `unrar`, `7z`,
  `poppler-utils`, `imagemagick` (see the table in [README.md](README.md)).

## Getting started

```sh
git clone https://github.com/furkandedizkan/handy-tools.git
cd handy-tools
git checkout test          # always branch off test, never main
git switch -c feat/my-thing
make proto                 # generates Go bindings under gen/ from api/proto
make build
make test
```

## Branching model

Handy Tools uses a two-trunk model.

```text
contributors  ->  feature/*  ->  PR into test  ->  CI green  ->  merged into test
                                                                       |
                                                          automated PR test -> main
                                                                       |
                                                                  CI green
                                                                       v
                                                                main (stable)
```

- **`main`** is the stable branch. Releases are tagged from `main`. **Never**
  open a PR directly into `main`.
- **`test`** is the integration branch. All contributor PRs target `test`.
- A scheduled GitHub workflow opens (or updates) a PR from `test` into `main`
  whenever `test` is green. Maintainers review and merge that PR to ship.

## Pull request workflow

1. Create a topic branch off `test`. Use prefixes: `feat/`, `fix/`,
   `docs/`, `refactor/`, `chore/`, `ci/`, `test/`.
2. Make your changes. Add or update tests.
3. Ensure `make lint test` passes locally.
4. Push and open a PR **into `test`**. Fill out the PR template.
5. Address review feedback. CI must be green before merge.
6. Maintainers will squash-merge. Your PR title becomes the commit message,
   so it must follow Conventional Commits.

Read [docs/PR_GUIDELINES.md](docs/PR_GUIDELINES.md) for the full checklist.

## Commit messages

We follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).

Format:

```text
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

Allowed `type`s: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
`build`, `ci`, `chore`, `revert`.

Common scopes: `image`, `archive`, `pdf`, `ui`, `server`, `api`, `config`,
`mascot`, `ci`, `release`.

Examples:

- `feat(archive): detect multi-part RAR volumes`
- `fix(ui): home page crashes on empty recent files list`
- `docs: clarify system dependency table`

Breaking changes use `!` and a `BREAKING CHANGE:` footer:

```text
feat(api)!: rename ExtractRequest.path to ExtractRequest.source

BREAKING CHANGE: clients must update field name.
```

## Testing

- Unit tests live next to the code (`foo_test.go`).
- Tool packages must include small fixture files under `testdata/`. Keep them
  tiny — a 4-pixel PNG, a 3-byte text file in a zip, etc.
- Use real fixtures, not mocks, when verifying file format behavior.

## Adding a new tool

1. Add the proto in `api/proto/v1/<tool>.proto`. Run `make proto`.
2. Implement `internal/tools/<tool>/` with a clean Go API mirroring the proto.
3. Wire it into `internal/server/` as a thin adapter.
4. Add a TUI view under `internal/ui/<tool>/` that calls the same package.
5. Document required system binaries (if any) in the README table and in
   `htools doctor`.

## Reporting bugs / requesting features

Use the GitHub issue templates. Include OS, Go version, and reproduction steps.
