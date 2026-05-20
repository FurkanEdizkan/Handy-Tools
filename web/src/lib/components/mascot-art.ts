/**
 * Wrenly / Hopper sprite data, ported once from `internal/ui/mascot/mascot.go`.
 *
 * The Go TUI is the source of truth for the dot-grid art and the per-state
 * expressions; this module mirrors `wrenlySprite` / `hopperSprite`, the
 * `State` enum, `eyeGlyph`, `overlayLine` and `furColor` so the web mascot
 * doesn't drift. Keep the grids and the glyph/overlay tables in sync if the
 * Go sprite ever changes.
 */

export type MascotCharacter = 'wrenly' | 'hopper';

/** Mirrors the Go `mascot.State` enum (order is stable). */
export type MascotState =
  | 'idle'
  | 'thinking'
  | 'watching'
  | 'stressed'
  | 'tired'
  | 'happy'
  | 'worried';

export const MASCOT_STATES: readonly MascotState[] = [
  'idle',
  'thinking',
  'watching',
  'stressed',
  'tired',
  'happy',
  'worried',
];

/** A single sprite cell. Mirrors the Go token legend (O/b/w/E/N/M + empty). */
export type SpriteToken = 'fur' | 'stripe' | 'cream' | 'eye' | 'nose' | 'mouth' | 'empty';

// Wrenly — 15×14 panda with cheek mask + tear stripes + 3-dot nose.
const WRENLY_GRID = `.O...........O.
OOO.........OOO
OwwO.......OwwO
OOOOOOOOOOOOOOO
OOOOOOOOOOOOOOO
OwwOOOOOOOOOwwO
OwwbEbwOwbEbwwO
OwbwwwwOwwwwbwO
OwwwwwwwwwwwwwO
OwwwwwNNNwwwwwO
OwwwwwbMbwwwwwO
.OwwwwwwwwwwwO.
..OOwwwwwwwOO..
.....OOOOO.....`;

// Hopper — 15×15 rabbit with tall pink-inner ears, bead eyes, pink nose.
const HOPPER_GRID = `.O...........O.
OwO.........OwO
OwO.........OwO
OwO.........OwO
OwO.........OwO
OOO.........OOO
.OOOOOOOOOOOOO.
OOOOOOOOOOOOOOO
OOOEOOOOOOOEOOO
OwwwwwwwwwwwwwO
OwwwwwbNbwwwwwO
OwwwwwwMwwwwwwO
.OwwwwwwwwwwwO.
..OOwwwwwwwOO..
....OOOOOOO....`;

const TOKEN_OF: Record<string, SpriteToken> = {
  O: 'fur',
  b: 'stripe',
  w: 'cream',
  E: 'eye',
  N: 'nose',
  M: 'mouth',
};

/** Parses a character's sprite into a row-major grid of {@link SpriteToken}s. */
export function spriteGrid(character: MascotCharacter): SpriteToken[][] {
  const raw = character === 'hopper' ? HOPPER_GRID : WRENLY_GRID;
  return raw.split('\n').map((row) => [...row].map((ch) => TOKEN_OF[ch] ?? 'empty'));
}

/**
 * Per-state eye glyph. Mirrors Go `eyeGlyph` — idle blinks every ~7th frame,
 * stressed quivers each frame.
 */
export function eyeGlyph(state: MascotState, frame: number): string {
  switch (state) {
    case 'happy':
      return '‿';
    case 'worried':
      return '⌒';
    case 'stressed':
      return frame % 2 === 0 ? '◉' : '●';
    case 'tired':
      return '–';
    case 'thinking':
      return '˙';
    case 'idle':
      return frame % 7 === 6 ? '–' : '•';
    default:
      return '•';
  }
}

/** Short uppercase status tag. Mirrors Go `StatusLabel` (tired → FRUSTRATED). */
export function statusLabel(state: MascotState): string {
  switch (state) {
    case 'thinking':
      return 'THINKING';
    case 'watching':
      return 'WATCHING';
    case 'stressed':
      return 'STRESSED';
    case 'tired':
      return 'FRUSTRATED';
    case 'happy':
      return 'HAPPY';
    case 'worried':
      return 'WORRIED';
    default:
      return 'IDLE';
  }
}

/**
 * CSS custom-property name for the per-state fur tint. Mirrors Go `furColor`:
 * stress/tired/happy/worried borrow theme hues; calm states keep mascot fur.
 */
export function furVar(state: MascotState): string {
  switch (state) {
    case 'stressed':
      return '--color-accent';
    case 'tired':
      return '--color-text-dim';
    case 'happy':
      return '--color-success';
    case 'worried':
      return '--color-error';
    default:
      return '--color-mascot-fur';
  }
}

export type OverlayTone = 'dim' | 'accent' | 'success' | 'error';
export type OverlayAlign = 'left' | 'center' | 'right';

export interface MascotOverlay {
  glyphs: string;
  tone: OverlayTone;
  align: OverlayAlign;
}

/**
 * One-row decoration floating above the sprite. Mirrors Go `overlayLine`:
 * thought dots when thinking, sparkles when happy, ticks when stressed,
 * a sweat drop when worried, huff marks when tired. `null` = blank row.
 */
export function overlayFrame(state: MascotState, frame: number): MascotOverlay | null {
  switch (state) {
    case 'thinking': {
      const cycle = ['·  °  ●', '·  ●  ∘', '●  ∘'];
      return { glyphs: cycle[frame % 3], tone: 'accent', align: 'right' };
    }
    case 'happy': {
      const cycle = ['✦   ✧   ✦', '✧  ✦  ✧', '✦  ✧   ✦'];
      return { glyphs: cycle[frame % 3], tone: 'success', align: 'center' };
    }
    case 'stressed':
      return { glyphs: '╱       ╲', tone: 'accent', align: 'center' };
    case 'worried':
      return { glyphs: '💧', tone: 'error', align: 'right' };
    case 'tired':
      return { glyphs: '~ ~', tone: 'dim', align: 'right' };
    default:
      return null;
  }
}
