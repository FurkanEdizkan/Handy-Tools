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
  TUI and gRPC server are thin layers on top.
- [ ] No tool logic in `internal/ui/` or `internal/server/`.
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
- [ ] If the TUI layout, keymap, home menu, or tool detail page changed,
      re-run `go run ./cmd/snapshot` and commit the updated
      `docs/screenshots/htools-*.txt` previews referenced from the README.

## CI

- [ ] All required checks are green.
- [ ] No skipped jobs without an explanation in the PR.

## After merge

- Maintainers squash-merge into `test`. The squash commit message is the PR
  title (so the title must be a valid Conventional Commit).
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
- [ ] **Require linear history** (matches the squash-merge convention).
- [ ] **Restrict who can push to matching branches.** Allow only
  `github-actions[bot]` so the `test -> main` promotion automation can push;
  everyone else is PR-only.
- [ ] **Do not** require approvals from a separate reviewer while the project
  has a single maintainer — that would block your own promotion PRs.

The same settings on `test` are optional but recommended (require PR + CI green)
so that direct pushes can't bypass the `test -> main` chain.
