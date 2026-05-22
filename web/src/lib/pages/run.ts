/**
 * run.ts — shared Run-button plumbing for the ToolPage forms.
 *
 * Running a tool needs real server-side filesystem paths. The Wails desktop
 * build supplies them directly via the native picker (#80/#81); resolveSources
 * maps the picked files onto FileRef paths the /v1/* tool endpoints consume.
 */

import { ApiError, type JobResponse } from '../api';
import type { PickedFile } from './toolform';

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
  /** Job IDs of the started jobs — empty when the run failed to start. */
  jobIds: string[];
}

/**
 * runJob executes start() — one or more job-start POSTs — and maps the result
 * to a Toast outcome. The started jobs surface live in the Queue panel, which
 * is already wired to the /v1/jobs/events stream (#156); jobIds lets a caller
 * also wait on completion (e.g. to reveal a download link, #191).
 */
export async function runJob(
  start: () => Promise<JobResponse | JobResponse[]>,
): Promise<RunOutcome> {
  try {
    const res = await start();
    const jobs = Array.isArray(res) ? res : [res];
    const n = jobs.length;
    return {
      ok: true,
      message:
        n === 1
          ? 'Job queued — watch the Queue panel.'
          : `${n} jobs queued — watch the Queue panel.`,
      jobIds: jobs.map((j) => j.jobId),
    };
  } catch (e) {
    return {
      ok: false,
      message: e instanceof ApiError ? e.message : 'Request failed.',
      jobIds: [],
    };
  }
}

/** One source ready to feed a tool endpoint as a FileRef.path. */
export interface ResolvedSource {
  name: string;
  path: string;
}

export interface ResolvedSources {
  sources: ResolvedSource[];
  /** Server-side directory the tool should write its output into. */
  outputDir: string;
}

/**
 * resolveSources turns the user's picked files into server-side paths a tool
 * endpoint can run against. The Wails native picker hands back real absolute
 * paths, so the mapping is direct.
 */
export function resolveSources(files: PickedFile[]): ResolvedSources {
  if (files.length === 0) {
    throw new Error('no files selected');
  }
  const sources = files.map((f) => ({ name: f.name, path: f.path ?? f.name }));
  return { sources, outputDir: dirOf(sources[0].path) };
}
