/**
 * native.ts — feature-detects the Wails desktop runtime.
 *
 * In the desktop build (`cmd/htools-gui`) Wails injects `window.go`, exposing
 * the Go bindings from #80 — native file pickers that return real filesystem
 * paths. In a plain browser those bindings are absent: `isDesktop()` is false
 * and the pick helpers resolve empty, so callers (the ToolPage Run buttons,
 * #155) can degrade to a "desktop app required" state.
 */

/** The Go methods bound on the App struct in cmd/htools-gui (#80). */
interface WailsApp {
  PickFiles(): Promise<string[]>;
  PickFolder(): Promise<string>;
  Reveal(path: string): Promise<void>;
}

interface WailsWindow {
  go?: { main?: { App?: WailsApp } };
}

/** wailsApp returns the bound App, or null in a plain browser. */
function wailsApp(): WailsApp | null {
  if (typeof window === 'undefined') return null;
  return (window as unknown as WailsWindow).go?.main?.App ?? null;
}

/** isDesktop reports whether the app is running inside the Wails desktop shell. */
export function isDesktop(): boolean {
  return wailsApp() !== null;
}

/**
 * pickFiles opens the native multi-file picker and resolves to the chosen
 * absolute paths. In a browser — or on cancel/error — it resolves to [].
 */
export async function pickFiles(): Promise<string[]> {
  const app = wailsApp();
  if (!app) return [];
  try {
    return (await app.PickFiles()) ?? [];
  } catch {
    return [];
  }
}

/**
 * pickFolder opens the native directory picker and resolves to the chosen
 * absolute path. In a browser — or on cancel/error — it resolves to "".
 */
export async function pickFolder(): Promise<string> {
  const app = wailsApp();
  if (!app) return '';
  try {
    return (await app.PickFolder()) ?? '';
  } catch {
    return '';
  }
}

/**
 * reveal opens the host file manager showing path. A no-op (best-effort) in a
 * browser or on error.
 */
export async function reveal(path: string): Promise<void> {
  const app = wailsApp();
  if (!app) return;
  try {
    await app.Reveal(path);
  } catch {
    /* best-effort — a failed reveal must not surface as an error */
  }
}
