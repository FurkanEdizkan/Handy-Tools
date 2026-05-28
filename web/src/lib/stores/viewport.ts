/**
 * viewport.ts — viewport-size + sidebar-drawer state.
 *
 * The desktop GUI used to render at one fixed shape (232 px sidebar always
 * visible). To accommodate small windows / phone-shaped viewports, this
 * store flips a boolean when the window crosses a media-query threshold,
 * and the layout CSS reacts: sidebar leaves the grid and becomes a
 * slide-out drawer triggered by the hamburger button in the topbar.
 *
 * `sidebarOpen` is per-session (not persisted) — the drawer always starts
 * closed and the close-on-navigation effect in Layout.svelte keeps it from
 * lingering after the user picks a tool.
 */

import { readable, writable, type Readable, type Writable } from 'svelte/store';

/** Threshold below which the layout switches into drawer mode. */
export const COMPACT_BREAKPOINT_PX = 768;

/**
 * isCompactViewport tracks whether the viewport is narrower than the
 * compact breakpoint. Driven by matchMedia so updates are debounced by the
 * browser and we don't have to listen to every resize event.
 */
export const isCompactViewport: Readable<boolean> = readable(
  typeof window !== 'undefined'
    ? window.matchMedia(`(max-width: ${COMPACT_BREAKPOINT_PX}px)`).matches
    : false,
  (set) => {
    if (typeof window === 'undefined') return;
    const mq = window.matchMedia(`(max-width: ${COMPACT_BREAKPOINT_PX}px)`);
    set(mq.matches);
    const handler = (e: MediaQueryListEvent): void => set(e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  },
);

/** Drawer state for the compact-viewport sidebar. Ignored above the breakpoint. */
export const sidebarOpen: Writable<boolean> = writable(false);

export function toggleSidebar(): void {
  sidebarOpen.update((open) => !open);
}

export function closeSidebar(): void {
  sidebarOpen.set(false);
}
