import { writable, type Writable } from 'svelte/store';
import { api, ApiClient } from '../api/client';
import type { JobStatusWire, JobSummary } from '../api/types';

export type JobStatus = 'wait' | 'running' | 'done' | 'fail';

export type LogLevel = 'INFO' | 'DEBUG' | 'WARN' | 'ERROR' | 'HINT' | 'DONE' | 'FAIL';

export interface JobLogLine {
  ts: string;
  level: LogLevel;
  message: string;
}

export interface Job {
  id: string;
  tool: string;
  currentItem: string | null;
  /** Number between 0 and 1. */
  progress: number;
  status: JobStatus;
  logs: JobLogLine[];
}

/** The live job list, folded from GET /v1/jobs + the /v1/jobs/events stream. */
export const jobs: Writable<Job[]> = writable([]);

export const expandedJobs: Writable<Set<string>> = writable(new Set());

/**
 * focusedJobId is set by deep-link sources (e.g. the failure popup in
 * NotificationStack) to ask the Jobs page to scroll the matching card into
 * view. The page reads it once on mount and clears it.
 */
export const focusedJobId: Writable<string | null> = writable(null);

export function toggleJobExpanded(id: string): void {
  expandedJobs.update((set) => {
    const next = new Set(set);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  });
}

/** statusFromWire maps the backend's job status onto the panel's JobStatus. */
function statusFromWire(s: JobStatusWire): JobStatus {
  switch (s) {
    case 'queued':
      return 'wait';
    case 'failed':
      return 'fail';
    case 'done':
      return 'done';
    default:
      return 'running';
  }
}

/** formatTs renders a start timestamp as a local HH:MM:SS string. */
function formatTs(unixMs?: number): string {
  if (!unixMs) return '';
  return new Date(unixMs).toLocaleTimeString('en-GB', { hour12: false });
}

/** displayTool joins a job's tool and action into one label. */
function displayTool(s: JobSummary): string {
  return s.action ? `${s.tool} · ${s.action}` : s.tool;
}

function emptyJob(id: string): Job {
  return { id, tool: '', currentItem: null, progress: 0, status: 'wait', logs: [] };
}

/**
 * applyJobSummary folds one /v1/jobs(/events) snapshot into the jobs store.
 * The row is updated in place (or prepended when new); a log line is appended
 * for each distinct progress message — a snapshot only carries the latest one,
 * so the live stream accumulates the history.
 */
export function applyJobSummary(s: JobSummary): void {
  jobs.update((list) => {
    const next = [...list];
    let idx = next.findIndex((j) => j.id === s.jobId);
    if (idx < 0) {
      next.unshift(emptyJob(s.jobId));
      idx = 0;
    }
    const prev = next[idx];
    const logs = [...prev.logs];
    const msg = s.error?.message ?? s.message;
    if (msg && (logs.length === 0 || logs[logs.length - 1].message !== msg)) {
      logs.push({
        ts: formatTs(s.startedUnixMs),
        level: s.error ? 'ERROR' : s.completed ? 'DONE' : 'INFO',
        message: msg,
      });
    }
    next[idx] = {
      id: s.jobId,
      tool: displayTool(s) || prev.tool,
      currentItem: s.currentItem ?? prev.currentItem,
      progress: s.fraction ?? prev.progress,
      status: statusFromWire(s.status),
      logs,
    };
    return next;
  });
}

/** loadJobs folds the current GET /v1/jobs snapshot into the store. */
export async function loadJobs(client: ApiClient = api): Promise<void> {
  const res = await client.fetchJobs();
  for (const s of res.jobs) applyJobSummary(s);
}

/**
 * startJobsFeed wires the dock and Jobs page to the live backend: it opens
 * the all-jobs SSE stream, then loads the initial /v1/jobs snapshot. Subscribing
 * first means no lifecycle event is missed in the gap between the two calls —
 * both paths fold by job id, so an overlap is deduped. Returns an
 * AbortController; the caller aborts it on teardown.
 */
export function startJobsFeed(client: ApiClient = api): AbortController {
  const ac = client.subscribeJobs((s) => applyJobSummary(s));
  // A failed initial load (htoolsd offline) leaves the panel empty; the SSE
  // stream still backfills rows as jobs start.
  void loadJobs(client).catch(() => {});
  return ac;
}
