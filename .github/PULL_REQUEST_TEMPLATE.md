<!--
  PR title MUST follow Conventional Commits, e.g.
    feat(archive): detect multi-part RAR volumes
    fix(ui): home page crashes on empty recent files list
  See docs/PR_GUIDELINES.md for the full checklist.
-->

## Summary

<!-- What changed and why? Describe the user-visible effect. -->

## Type of change

- [ ] feat — new feature
- [ ] fix — bug fix
- [ ] docs — documentation only
- [ ] refactor — no behavior change
- [ ] perf — performance
- [ ] test — tests only
- [ ] build / ci — tooling
- [ ] chore — other

## Linked issues

Closes #

## Test plan

<!-- Commands you ran, fixtures you used, manual verification steps. -->

- [ ] `make lint test` passes locally
- [ ] Added or updated tests
- [ ] Updated README / docs if user-facing

## Checklist

- [ ] PR targets `test`, not `main`
- [ ] Title follows Conventional Commits
- [ ] One logical change per PR
- [ ] No tool logic in `internal/ui` or `internal/server`
- [ ] System binary requirements (if any) handled gracefully when missing
