# TODO

main test

A living list of concrete, in-repo improvements — the kind a code agent
(or human) can pick up and verify. For the longer-horizon roadmap (plugin
registry, web/Chrome surfaces, pure-Go PDF), see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#stretch--next-up).

## TUI — wire the design to real tools

The TUI ([internal/ui/](internal/ui/)) now matches the design from
[`htools-tui.html`][design], but the **Run** button still drives a simulated
stepper. Below is the prioritized path from "good demo" → "real product."

[design]: https://api.anthropic.com/v1/design/h/mgeSoB1YWUmcvuZSld6AGQ

- [ ] **Replace the run simulator with real tool calls.** Today
      [`router.go`](internal/ui/router.go) `startRun` / `stepRun` ticks
      progress by hand and synthesizes `processed <file>` log lines. Each
      tool should instead call its real `internal/tools/<x>` API and pipe
      the `tools.Progress` channel into the queue entry (status, percent,
      `LogLine{Lvl:"INFO", Msg: ...}`). Failures become
      `JobFailed` with the structured `*tools.Error` code surfaced on the
      queue row (matches the design's `pdftoppm` error trace).
- [ ] **Real file picker.** The dropzone in
      [`toolpage.go`](internal/ui/toolpage.go) renders Browse buttons but
      they just call `addPlaceholderFile()`. Either embed an inline file
      browser (similar to what `internal/ui/files.go` used to do — its
      logic is in `git log`) or shell out to `$EDITOR`-style picking.
- [ ] **Editable Custom path.** The `outCustom` row shows the path as a
      static box. Hook keyboard input so typing edits `customPath` when the
      row is focused.
- [ ] **Doctor page reads real sysdep.** `renderDoctor()` in
      `toolpage.go` hardcodes 5 deps with hardcoded ok/missing. Call into
      [`internal/tools/sysdep`](internal/tools/sysdep/sysdep.go) `Known` +
      `Detect` so the page reflects the user's actual PATH, the same way
      the `htools doctor` subcommand does.
- [ ] **Settings overlay popover.** The design has a top-left popover
      anchored to the cog with theme/scanlines/mascot chips. Today the
      `,` key switches to a separate Settings *page*. Build the overlay
      variant for keyboard + mouse users.
- [ ] **Persist theme on Tab.** Pressing `tab` cycles theme but doesn't
      write `theme.name` to `~/.config/handy-tools/config.yaml`. Call
      `config.Save` on change (or batch on quit).
- [ ] **Reflect `htoolsd` reachability.** The status bar always shows
      `htoolsd offline`. Add a periodic gRPC `grpc.health.v1.Health/Check`
      ping (or dial test) and flip the dot.

## TUI — polish

- [ ] **Truncate queue labels.** Long labels like
      `invoice-2026-04.png → JPEG` wrap to 4 lines inside the 36-col queue
      column. Truncate with `…` at the column boundary so each item is
      one row tall.
- [ ] **File-row layout collapse at narrow widths.** The per-file row in
      the tool detail page uses a bordered `[ JPEG ▾ ]` select that wraps
      to a second line under the file name when the right column is < 90
      cols. Either drop the border or render `JPEG ▾` inline.
- [ ] **Single-column fallback for terminals < 100 cols.** Below ~100 cols
      the fixed 28-col left column eats half the screen. Detect width in
      `Model.View()` and switch to a single-pane layout (mascot collapsed
      into the header, queue moved to a `Q` modal).
- [ ] **ASCII fallback for tool glyphs.** `◇ ▢ ◰ ◫ ◊` in `defaultTools`
      can render as zero-width in older terminals. Detect via
      `$TERM`/`LANG` and substitute `o # x + ?`.
- [ ] **Floating toast like the design.** Current toast is appended below
      the status bar. Design has it absolutely positioned bottom-center
      with a border. Use `lipgloss.Place` over the frame.
- [ ] **Tweaks panel.** The design's right-edge "Tweaks" sliding panel
      (tick-ms slider, auto-run demo, scanlines toggle) is not wired up.
      Optional — for demos / screenshots.

## TUI — tests

- [ ] Unit-test `toolpage` focus cycling (`cycleFocus` across all modes
      including Doctor) — currently only covered transitively by the
      router smoke test.
- [ ] Unit-test `summary()` for each mode + per-file overrides → matches
      the design's `3 → JPEG · 1 → WebP` rule.
- [ ] Snapshot-test `View()` output by recording golden `.txt` under
      `internal/ui/testdata/` so future layout drifts trip CI rather than
      humans noticing in the README preview.
- [ ] `cmd/snapshot` CI guard: run it in a job and fail if
      `docs/screenshots/htools-*.txt` is out of sync with the committed
      copy (same pattern as `make proto` would have).

## Installer / release

- [ ] **Fix the install-doc URL.** The README example points at
      `FurkanEdizkan/Handy-Tools` (mixed case); `install.sh` defines
      `REPO="furkandedizkan/handy-tools"` (lowercase). GitHub redirects
      most operations, but **the GitHub API is case-sensitive** —
      `api.github.com/repos/.../releases/latest` 404s if the case is wrong
      *and* there is no published release yet. Confirm the canonical
      casing of the repo, then make the README curl line and `install.sh`
      use it verbatim.
- [ ] **Surface a clearer "no release yet" error.** Right now the user
      sees `curl: (22) The requested URL returned error: 404` and an
      "ambiguous rate-limited?" hint. Distinguish 404 (no release / bad
      slug) from 403 (rate limited) and print which one happened.
- [x] ~~Ship a first release.~~ — done in `v0.2.0`. Subsequent releases
      now ship automatically on every CI-green commit to `main` as
      `v{YYYY}.{M}.{D}-beta.{N}` pre-releases (see
      [auto-tag.yml](.github/workflows/auto-tag.yml)); no manual bump
      needed.
- [ ] **Cut the first stable `v1.0.0`** when the TUI is wired to real
      tool calls (see "Replace the run simulator" at the top of this
      file). Switching to `v1.0.0` is the signal to drop calver and
      return to plain semver — update
      [`auto-tag.yml`](.github/workflows/auto-tag.yml) and the
      `name_template` in [`.goreleaser.yaml`](.goreleaser.yaml).
- [ ] **Test the installer mascot under `NO_COLOR=1`.** The new face
      mascot in [`install.sh`](install.sh#L53) was only verified in
      color mode. Make sure the no-color branch (line 54) still prints
      the bare `Handy Tools installer` message.

## DX / build

- [ ] `make screenshots` target that wraps `go run ./cmd/snapshot`.
- [ ] `make snapshots-check` for CI — exits non-zero if running the
      generator would change the committed files. Mirrors the
      "regenerate previews" PR-checklist item in
      [docs/PR_GUIDELINES.md](docs/PR_GUIDELINES.md).
- [ ] Tighten the theme color constants. golangci-lint flags
      [`internal/ui/theme/theme.go`](internal/ui/theme/theme.go) for
      duplicating `#9bdc8a`, `#f0c674`, `#e57373` across all three
      palettes (Success/Warning/Error are deliberately shared across
      Forge/Snow/Ember). Extract to package-level constants.
- [ ] Drop the unused `stringForState` / `stateStrings` helpers in
      [`internal/ui/components.go`](internal/ui/components.go) now that
      `mascot.State` has a `String()` method.

## Already-known roadmap items

Reproduced here so the file is a single index — but the canonical home
for these is [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#stretch--next-up).

- [ ] WebP encoder (needs a pure-Go encoder, or accepting CGO).
- [ ] `pdfcpu` library import to replace shelled-out PDF ops.
- [ ] Plugin registry — one proto + one Go package per new tool, no
      router edits.
- [ ] gRPC-Web / REST gateway over `htoolsd` for the web UI / Chrome
      extension surfaces.
- [ ] Swap the hand-rolled YAML mini-parser for `gopkg.in/yaml.v3`
      once we have another reason to take that dep.
