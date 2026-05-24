# Pull Request Guidelines

This is the checklist a PR must satisfy to get merged into `test`. Maintainers
will use it during review.

## Targeting

- [ ] PR is opened against **`test`**, not `main`.
- [ ] The branch name has a meaningful prefix: `feat/`, `fix/`, `docs/`,
  `refactor/`, `chore/`, `ci/`, `test/`.

## Title & description

- [ ] PR title follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
  Example: `feat(archive): detect multi-part RAR volumes`.
- [ ] Description explains **what** changed and **why**, not just the diff.
- [ ] Linked issue if one exists (`Closes #123`).

## Scope

- [ ] One logical change per PR. Don't bundle unrelated refactors.
- [ ] Public API changes (proto, tool package signatures) are called out
  explicitly in the description.
- [ ] Breaking changes use `!` in the title and a `BREAKING CHANGE:` footer.

## Code

- [ ] Follows the architecture: tool logic lives in `internal/tools/<x>/`, the
  CLI, gRPC server, and HTTP transport are thin layers on top.
- [ ] No tool logic in `cmd/htools/`, `internal/server/`, or `internal/api/http/`.
- [ ] No new direct dependencies without a one-line justification in the PR.
- [ ] System binaries (`unrar`, `7z`, `pdftoppm`, etc.) are detected at runtime
  and produce a clear error when missing — never panic.

## Tests

- [ ] New behavior has unit tests next to the code.
- [ ] Tool changes include small fixtures under `testdata/`.
- [ ] `make lint test` passes locally.

## Docs

- [ ] README updated if user-facing behavior or system deps changed.
- [ ] `htools doctor` output reflects any new optional system tool.
- [ ] If a new tool was added, `docs/ARCHITECTURE.md` mentions it.
- [ ] If a new top-level CLI subcommand was added, the `usage` string in
      [cmd/htools/usage.go](../cmd/htools/usage.go) lists it.

## CI

- [ ] All required checks are green.
- [ ] No skipped jobs without an explanation in the PR.

## After merge

- Maintainers merge with a merge commit (not squash). The PR title becomes
  the merge commit message (so it must be a valid Conventional Commit), and
  every commit on the branch is preserved on `test`.
- The automated `test -> main` PR will pick the change up on the next green run.

## Branch protection (maintainer setup)

The `test -> main` promotion automation only matters if direct pushes to `main`
are blocked. These settings must be configured in the GitHub UI under
**Settings → Branches → Branch protection rules** for `main` (they can't be
checked in to the repo without admin-scoped API tokens):

- [ ] **Require a pull request before merging.**
- [ ] **Require status checks to pass before merging.** Add as required checks
  (use the check *display names*, not the job ids):
  - `Lint (Go)`, `Lint (proto)`
  - `Web (Svelte / Vite)`
  - `Test + fuzz (fast lane, Linux)`
  - `Build`
  - `Validate PR title` (from the commitlint workflow)
- [ ] **Allowed merge methods: Merge commit only** — disable Squash and
  Rebase, so every PR lands as a merge commit and history stays fully traceable.
- [ ] **Do not require linear history** — the workflow relies on merge commits.
- [ ] **Restrict who can push to matching branches.** Allow only
  `github-actions[bot]` so the `test -> main` promotion automation can push;
  everyone else is PR-only.
- [ ] **Do not** require approvals from a separate reviewer while the project
  has a single maintainer — that would block your own promotion PRs.

The same settings on `test` are optional but recommended (require PR + CI green)
so that direct pushes can't bypass the `test -> main` chain.
