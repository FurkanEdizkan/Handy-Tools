import { writable, type Writable } from 'svelte/store';

/* The theme cycle (forge / snow / ember) was retired alongside the TUI in
 * #205. The CSS variables that drive the surviving forge palette still live
 * in styles/theme.css; this store now only carries client-only display and
 * mascot preferences. */

/* ---- Density ----------------------------------------------------------
 * Density adjusts header/dock heights via the `data-density` attribute (see
 * styles/theme.css). Local-only preference — no backend field. */

export type Density = 'compact' | 'regular' | 'comfy';

export const DENSITIES: Density[] = ['compact', 'regular', 'comfy'];

const DENSITY_KEY = 'handy.density';
const DEFAULT_DENSITY: Density = 'regular';

function isDensity(value: unknown): value is Density {
  return typeof value === 'string' && (DENSITIES as string[]).includes(value);
}

function readStoredDensity(): Density | null {
  try {
    const raw = localStorage.getItem(DENSITY_KEY);
    return isDensity(raw) ? raw : null;
  } catch {
    return null;
  }
}

export const currentDensity: Writable<Density> = writable(readStoredDensity() ?? DEFAULT_DENSITY);

currentDensity.subscribe((value) => {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-density', value);
  }
  try {
    localStorage.setItem(DENSITY_KEY, value);
  } catch {
    /* localStorage unavailable — the attribute is still applied */
  }
});

export function setDensity(density: Density): void {
  currentDensity.set(density);
}

/* ---- Mascot preferences ----------------------------------------------
 * Character (Wrenly / Hopper) and sidebar visibility. Local-only prefs. */

export type MascotCharacter = 'wrenly' | 'hopper';

const MASCOT_CHAR_KEY = 'handy.mascot.character';
const MASCOT_ON_KEY = 'handy.mascot.enabled';

function readStoredString<T extends string>(key: string, allowed: readonly T[]): T | null {
  try {
    const raw = localStorage.getItem(key);
    return raw !== null && (allowed as readonly string[]).includes(raw) ? (raw as T) : null;
  } catch {
    return null;
  }
}

export const mascotCharacter: Writable<MascotCharacter> = writable(
  readStoredString<MascotCharacter>(MASCOT_CHAR_KEY, ['wrenly', 'hopper']) ?? 'wrenly',
);
mascotCharacter.subscribe((v) => {
  try {
    localStorage.setItem(MASCOT_CHAR_KEY, v);
  } catch {
    /* localStorage unavailable */
  }
});

export const mascotEnabled: Writable<boolean> = writable(
  readStoredString(MASCOT_ON_KEY, ['true', 'false']) !== 'false',
);
mascotEnabled.subscribe((v) => {
  try {
    localStorage.setItem(MASCOT_ON_KEY, String(v));
  } catch {
    /* localStorage unavailable */
  }
});
