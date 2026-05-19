import { writable, type Writable } from 'svelte/store';

export type ThemeName = 'forge' | 'snow' | 'ember';

export const THEMES: ThemeName[] = ['forge', 'snow', 'ember'];

const STORAGE_KEY = 'handy.theme';
const DEFAULT_THEME: ThemeName = 'forge';

function isTheme(value: unknown): value is ThemeName {
  return typeof value === 'string' && (THEMES as string[]).includes(value);
}

function readStored(): ThemeName | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return isTheme(raw) ? raw : null;
  } catch {
    return null;
  }
}

function applyToDocument(theme: ThemeName): void {
  if (typeof document === 'undefined') return;
  document.documentElement.setAttribute('data-theme', theme);
}

function persist(theme: ThemeName): void {
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    /* localStorage may be unavailable (private mode, embed); the data-theme
       attribute is still set, so the active session works. */
  }
}

export const currentTheme: Writable<ThemeName> = writable(readStored() ?? DEFAULT_THEME);

currentTheme.subscribe((value) => {
  applyToDocument(value);
  persist(value);
});

export function setTheme(theme: ThemeName): void {
  currentTheme.set(theme);
}

export function cycleTheme(): void {
  currentTheme.update((current) => {
    const idx = THEMES.indexOf(current);
    return THEMES[(idx + 1) % THEMES.length];
  });
}

/**
 * Initialise the theme from `GET /v1/config` if reachable, otherwise keep the
 * stored / default value. Backend endpoint lands in #65; until then the fetch
 * fails silently and the local fallback is used.
 */
export async function hydrateFromConfig(fetchFn: typeof fetch = fetch): Promise<void> {
  if (readStored()) return; // user choice wins over server default
  try {
    const res = await fetchFn('/v1/config', { headers: { accept: 'application/json' } });
    if (!res.ok) return;
    const body = (await res.json()) as { theme?: unknown };
    if (isTheme(body.theme)) {
      currentTheme.set(body.theme);
    }
  } catch {
    /* unreachable backend — keep forge default */
  }
}
