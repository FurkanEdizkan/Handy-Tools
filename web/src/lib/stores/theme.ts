import { writable, get, type Writable } from 'svelte/store';
import { api } from '../api';

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

/**
 * persistToBackend syncs the choice into the server config (the `theme.name`
 * field) via PATCH /v1/config. It is fire-and-forget: localStorage already
 * holds the selection, so an unreachable backend degrades gracefully and the
 * active session is unaffected.
 */
async function persistToBackend(theme: ThemeName): Promise<void> {
  try {
    await api.patchConfig({ theme });
  } catch {
    /* backend unreachable — the localStorage fallback already applied */
  }
}

/**
 * setTheme is the entry point for an explicit user choice. It updates the live
 * store (which applies `data-theme` and writes localStorage via the subscriber)
 * and persists the choice to the server config so it survives a fresh browser.
 */
export function setTheme(theme: ThemeName): void {
  currentTheme.set(theme);
  void persistToBackend(theme);
}

export function cycleTheme(): void {
  const idx = THEMES.indexOf(get(currentTheme));
  setTheme(THEMES[(idx + 1) % THEMES.length]);
}

/**
 * Initialise the theme from `GET /v1/config` if reachable. A theme already
 * chosen on this device (localStorage) wins over the server default, so the
 * fetch is skipped in that case. On an unreachable backend the stored / default
 * value is kept.
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
