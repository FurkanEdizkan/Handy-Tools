/**
 * run.ts — shared Run-button plumbing for the ToolPage forms.
 *
 * Running a tool needs real filesystem paths, which only the Wails desktop
 * build can supply (via the native picker, #80/#81). In a plain browser
 * runJob refuses up front; ToolPages also disable the Run button there.
 */

import { ApiError, type JobResponse } from '../api';
import { isDesktop } from '../native';

/** dirOf returns the directory component of an absolute path. */
export function dirOf(path: string): string {
  const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
  return i > 0 ? path.slice(0, i) : '.';
}

/** stemOf returns a path's basename without its final extension. */
export function stemOf(path: string): string {
  const base = path.slice(Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1);
  const dot = base.lastIndexOf('.');
  return dot > 0 ? base.slice(0, dot) : base;
}

export interface RunOutcome {
  ok: boolean;
  message: string;
}

/**
 * runJob executes start() — one or more job-start POSTs — and maps the result
 * to a Toast outcome. The started jobs surface live in the Queue panel, which
 * is already wired to the /v1/jobs/events stream (#156).
 */
export async function runJob(
  start: () => Promise<JobResponse | JobResponse[]>,
): Promise<RunOutcome> {
  if (!isDesktop()) {
    return { ok: false, message: 'Running tools needs the Handy Tools desktop app.' };
  }
  try {
    const res = await start();
    const n = Array.isArray(res) ? res.length : 1;
    return {
      ok: true,
      message:
        n === 1
          ? 'Job queued — watch the Queue panel.'
          : `${n} jobs queued — watch the Queue panel.`,
    };
  } catch (e) {
    return { ok: false, message: e instanceof ApiError ? e.message : 'Request failed.' };
  }
}
