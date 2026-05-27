/**
 * Global notification stack for app-wide popups (failed jobs, preflight
 * errors, ad-hoc info). Renders via NotificationStack mounted in Layout.
 *
 * Two ingress paths:
 * 1. Direct: pushPopup({...}) from a tool page that wants to surface a
 *    synchronous error (e.g. "missing volumes detected before submit").
 * 2. Subscribed: subscribeJobsForFailures() watches the live `jobs` store
 *    and pushes a sticky error popup whenever any job transitions to 'fail'.
 *    The popup carries the job's last ERROR log line as its message and
 *    deep-links to the Jobs page on click.
 */

import { writable, type Writable } from 'svelte/store';
import { jobs, type Job, type JobStatus } from './jobs';

export type PopupTone = 'error' | 'info';

export interface Popup {
  id: string;
  tone: PopupTone;
  title: string;
  message: string;
  /** When set, clicking the popup deep-links to the Jobs page with this id. */
  jobId?: string;
  /** When true, the popup stays until the user dismisses it. */
  sticky: boolean;
  createdAt: number;
}

export const popups: Writable<Popup[]> = writable([]);

/**
 * pushPopup adds a popup, deduping on `id` — re-pushing an existing id is a
 * no-op so the failure subscriber can't double-emit when a snapshot replays.
 */
export function pushPopup(p: Omit<Popup, 'createdAt'> & { createdAt?: number }): void {
  const entry: Popup = { createdAt: p.createdAt ?? Date.now(), ...p };
  popups.update((list) => {
    if (list.some((x) => x.id === entry.id)) return list;
    return [...list, entry];
  });
}

export function dismissPopup(id: string): void {
  popups.update((list) => list.filter((p) => p.id !== id));
}

export function dismissAll(): void {
  popups.set([]);
}

/**
 * subscribeJobsForFailures wires the popup stack to the live job feed: every
 * job that transitions into 'fail' produces a single sticky error popup. The
 * message is the last ERROR-level log line on the job, falling back to a
 * generic note when the log is empty. Returns the unsubscribe function.
 */
export function subscribeJobsForFailures(): () => void {
  const seen = new Map<string, JobStatus>();
  let primed = false;
  return jobs.subscribe((list) => {
    for (const j of list) {
      const prev = seen.get(j.id);
      // First subscriber invocation just seeds the map — Svelte fires the
      // subscriber synchronously with the current snapshot, which may already
      // contain previously-failed jobs from the initial /v1/jobs replay. We
      // only want popups for transitions observed *during this session*.
      if (primed && j.status === 'fail' && prev !== 'fail') {
        pushPopup({
          id: `job:${j.id}`,
          tone: 'error',
          sticky: true,
          jobId: j.id,
          title: `${j.tool || 'Job'} failed`,
          message: lastErrorMessage(j),
        });
      }
      seen.set(j.id, j.status);
    }
    primed = true;
  });
}

function lastErrorMessage(j: Job): string {
  for (let i = j.logs.length - 1; i >= 0; i--) {
    const l = j.logs[i];
    if (l.level === 'ERROR' || l.level === 'FAIL') return l.message;
  }
  return 'Job failed — open the Jobs page for full logs.';
}
