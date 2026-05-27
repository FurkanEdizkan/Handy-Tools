/**
 * extract-grouping.ts — pure folding of per-file Inspect results into the
 * minimum set of extract jobs.
 *
 * The Extract page asks `archiveInspect` for every dropped file, then calls
 * `groupArchives` to:
 *   - reject files whose multi-part set is incomplete (missing volumes),
 *   - coalesce sibling volumes that belong to the same archive into ONE
 *     extract job (using the backend-supplied `detectedParts` as the group
 *     key — overlap on any part path means same archive),
 *   - leave independent archives as their own job.
 *
 * Kept dependency-free so the grouping rules can be unit-tested without a
 * component harness.
 */

import type { InspectResponse } from '../api/types';
import { computeExtractDestDir, type ExtractDestMode, type ResolvedSource } from './run';

export interface InspectedSource {
  src: ResolvedSource;
  ins: InspectResponse;
}

/** One coalesced extract job: a primary entry-point + its sibling volumes. */
export interface ExtractGroup {
  format: string;
  /** Path passed as `source` in the extract request. */
  primary: string;
  /** All volume paths (incl. primary). Passed as `parts`. */
  parts: string[];
  /** Final per-archive output directory. */
  destDir: string;
  /** User-picked paths that folded into this group. */
  members: string[];
}

export interface GroupingResult {
  groups: ExtractGroup[];
  /** Files that can't be extracted (missing peer volumes). */
  errors: InspectedSource[];
}

export function groupArchives(
  items: InspectedSource[],
  mode: ExtractDestMode,
  destination: string,
): GroupingResult {
  const seen = new Map<string, ExtractGroup>();
  const errors: InspectedSource[] = [];

  for (const { src, ins } of items) {
    // Go's JSON encoder serializes nil slices as null. The backend was
    // patched to normalize these to []; the `?? []` here defends older
    // backends and any other endpoint that hasn't been normalized.
    const missing = ins.missingParts ?? [];
    const detected = ins.detectedParts ?? [];
    if (ins.multiPart && missing.length > 0) {
      errors.push({ src, ins });
      continue;
    }
    const parts = detected.length > 0 ? detected : [src.path];
    const existing = parts.map((p) => seen.get(p)).find((g): g is ExtractGroup => !!g);
    if (existing) {
      if (!existing.members.includes(src.path)) existing.members.push(src.path);
      continue;
    }
    const primary = parts[0];
    const g: ExtractGroup = {
      format: ins.format,
      primary,
      parts,
      destDir: computeExtractDestDir(primary, mode, destination),
      members: [src.path],
    };
    for (const p of parts) seen.set(p, g);
  }
  return { groups: [...new Set(seen.values())], errors };
}
