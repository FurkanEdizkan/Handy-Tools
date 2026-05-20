/**
 * Pure helpers for the Doctor page. Kept dependency-free (only a type import)
 * so the summary rule can be unit-tested without a component harness.
 */

import type { SysdepResult } from '../api';

/**
 * sysdepSummary mirrors the TUI Doctor page's count line — e.g. `3 / 5 found`.
 */
export function sysdepSummary(results: SysdepResult[]): string {
  const found = results.filter((r) => r.found).length;
  return `${found} / ${results.length} found`;
}
