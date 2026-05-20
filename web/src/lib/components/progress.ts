/**
 * Progress math shared by ProgressBar.svelte. Kept as a plain module so the
 * clamping / block-rendering logic is unit-testable without a DOM.
 */

const FILLED_BLOCK = '▰';
const EMPTY_BLOCK = '▱';

/** Clamps an arbitrary fraction into [0, 1]; NaN collapses to 0. */
export function clampFraction(fraction: number): number {
  if (Number.isNaN(fraction)) return 0;
  return Math.min(1, Math.max(0, fraction));
}

/**
 * Renders a fraction as `▰`/`▱` blocks — the same glyphs the TUI uses, so the
 * web progress readout stays visually consistent with `internal/ui`.
 */
export function progressBlocks(fraction: number, width: number): string {
  const cells = Math.max(0, Math.floor(width));
  const filled = Math.round(clampFraction(fraction) * cells);
  return FILLED_BLOCK.repeat(filled) + EMPTY_BLOCK.repeat(cells - filled);
}

/** Whole-percent label, e.g. 0.624 → "62%". */
export function percentLabel(fraction: number): string {
  return `${Math.round(clampFraction(fraction) * 100)}%`;
}
