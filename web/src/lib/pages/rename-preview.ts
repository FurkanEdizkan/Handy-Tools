/**
 * rename-preview.ts — the Batch rename live preview, driven by the server's
 * dry run (POST /v1/rename/inspect) so it shows exactly what Apply will do:
 * Go RE2 syntax, ReplaceAllString (every match, not just the first) and Go's
 * `$name` template rules (`$1_x` means group "1_x"; write `${1}_x`).
 *
 * Kept free of Svelte so the debounce / stale-response rules are unit-testable.
 */

import { ApiError, type RenameInspectResponse, type RenamePlan, type RenameRequest } from '../api';
import { basenameOf, resolveSources } from './run';
import type { PickedFile } from './toolform';

export const PREVIEW_DEBOUNCE_MS = 250;

export interface RenamePreview {
  /** renamePreviewKey of the inputs this result was computed for. */
  key: string;
  /** Server plans in source order, or null when there is nothing to show. */
  plans: RenamePlan[] | null;
  /** User-facing reason the preview could not be computed ('' when fine). */
  error: string;
}

/** renamePreviewKey identifies one (files, pattern, replacement) input set. */
export function renamePreviewKey(files: PickedFile[], pattern: string, replace: string): string {
  return JSON.stringify([pattern, replace, files.map((f) => f.path ?? `\0${f.name}`)]);
}

/** planChanges reports whether a plan row renames its file (basename differs). */
export function planChanges(p: RenamePlan): boolean {
  return basenameOf(p.from) !== basenameOf(p.to);
}

function errorText(e: unknown): string {
  if (e instanceof ApiError) {
    const detail = e.envelope?.detail;
    return detail ? `${e.message}: ${detail}` : e.message;
  }
  return 'Preview request failed.';
}

/**
 * createRenamePreview returns a debounced previewer. Each request() supersedes
 * the previous one: its timer is cancelled and any in-flight response for an
 * older request is dropped, so onUpdate only ever sees the latest inputs.
 */
export function createRenamePreview(
  inspect: (req: RenameRequest) => Promise<RenameInspectResponse>,
  onUpdate: (p: RenamePreview) => void,
  delayMs: number = PREVIEW_DEBOUNCE_MS,
): { request(files: PickedFile[], pattern: string, replace: string): void; dispose(): void } {
  let seq = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;

  return {
    request(files, pattern, replace) {
      const mine = ++seq;
      clearTimeout(timer);
      const key = renamePreviewKey(files, pattern, replace);
      if (!pattern || files.length === 0) {
        onUpdate({ key, plans: null, error: '' });
        return;
      }
      let sources: { path: string }[];
      try {
        sources = resolveSources(files).sources.map((s) => ({ path: s.path }));
      } catch (e) {
        onUpdate({ key, plans: null, error: e instanceof Error ? e.message : String(e) });
        return;
      }
      timer = setTimeout(async () => {
        try {
          const res = await inspect({ sources, pattern, replace });
          if (mine === seq) onUpdate({ key, plans: res.plans ?? [], error: '' });
        } catch (e) {
          if (mine === seq) onUpdate({ key, plans: null, error: errorText(e) });
        }
      }, delayMs);
    },
    dispose() {
      seq++;
      clearTimeout(timer);
    },
  };
}
