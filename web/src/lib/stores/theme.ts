import { writable, type Writable } from 'svelte/store';

/* The theme cycle (forge / snow / ember) was retired alongside the TUI in
 * #205, and the density picker was retired in favour of automatic responsive
 * layout. This store now only carries the mascot preferences. */

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
