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

## CI

- [ ] All required checks are green.
- [ ] No skipped jobs without an explanation in the PR.

## After merge

- Maintainers squash-merge into `test`. The squash commit message is the PR
  title (so the title must be a valid Conventional Commit).
- The automated `test -> main` PR will pick the change up on the next green run.
