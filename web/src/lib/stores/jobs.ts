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
  /** True when progress is a time/size ETA estimate, not a measured byte count. */
  estimated: boolean;
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
  return { id, tool: '', currentItem: null, progress: 0, estimated: false, status: 'wait', logs: [] };
}

/**
 * foldSummary merges one snapshot onto a job's previous row. A log line is
 * appended for each distinct progress message — a snapshot only carries the
 * latest one, so the live stream accumulates the history.
 */
function foldSummary(prev: Job, s: JobSummary): Job {
  const logs = [...prev.logs];
  const msg = s.error?.message ?? s.message;
  if (msg && (logs.length === 0 || logs[logs.length - 1].message !== msg)) {
    logs.push({
      ts: formatTs(s.startedUnixMs),
      level: s.error ? 'ERROR' : s.completed ? 'DONE' : 'INFO',
      message: msg,
    });
  }
  return {
    id: s.jobId,
    tool: displayTool(s) || prev.tool,
    currentItem: s.currentItem ?? prev.currentItem,
    progress: s.fraction ?? prev.progress,
    estimated: s.estimated ?? (s.fraction === undefined ? prev.estimated : false),
    status: statusFromWire(s.status),
    logs,
  };
}

/**
 * applyJobSummary folds one /v1/jobs(/events) snapshot into the jobs store.
 * The row is updated in place (or prepended when new).
 */
export function applyJobSummary(s: JobSummary): void {
  jobs.update((list) => {
    const next = [...list];
    let idx = next.findIndex((j) => j.id === s.jobId);
    if (idx < 0) {
      next.unshift(emptyJob(s.jobId));
      idx = 0;
    }
    next[idx] = foldSummary(next[idx], s);
    return next;
  });
}

/**
 * replaceJobs swaps the store's contents for a full GET /v1/jobs snapshot
 * (oldest first), rather than merging: rows the server no longer reports
 * (e.g. after an htoolsd restart) are dropped, and rows whose final frames
 * were missed get corrected. Rows still present keep their log history.
 * `liveIds` are jobs the SSE stream updated after the snapshot was requested
 * — their current row is newer than the snapshot, so it is kept as-is.
 */
export function replaceJobs(snapshot: JobSummary[], liveIds: ReadonlySet<string> = new Set()): void {
  jobs.update((list) => {
    const current = new Map(list.map((j) => [j.id, j]));
    const inSnapshot = new Set(snapshot.map((s) => s.jobId));
    const next: Job[] = [];
    for (const s of snapshot) {
      const cur = current.get(s.jobId);
      next.unshift(cur && liveIds.has(s.jobId) ? cur : foldSummary(cur ?? emptyJob(s.jobId), s));
    }
    const newer = list.filter((j) => liveIds.has(j.id) && !inSnapshot.has(j.id));
    return [...newer, ...next];
  });
}

/** loadJobs folds the current GET /v1/jobs snapshot into the store. */
export async function loadJobs(client: ApiClient = api): Promise<void> {
  const res = await client.fetchJobs();
  for (const s of res.jobs) applyJobSummary(s);
}

export interface JobsFeed {
  /** Settles once the initial /v1/jobs snapshot is in the store (or failed). */
  loaded: Promise<void>;
  /** Closes the SSE stream and ignores any snapshot still in flight. */
  abort(): void;
}

/**
 * startJobsFeed wires the dock and Jobs page to the live backend: it opens
 * the all-jobs SSE stream, then loads the initial /v1/jobs snapshot.
 * Subscribing first means no lifecycle event is missed in the gap between the
 * two calls. The stream does not replay history, so whenever the EventSource
 * reconnects after an error (htoolsd restart, dropped connection) the
 * snapshot is fetched again and replaces the store — otherwise a job whose
 * completion frame fell into the gap would show "running" forever. The
 * caller aborts the returned feed on teardown.
 */
export function startJobsFeed(client: ApiClient = api): JobsFeed {
  let stopped = false;
  let disconnected = false;
  let latestSync = 0;
  // Ids the stream updated while each snapshot fetch was in flight.
  const pending = new Set<Set<string>>();

  async function resync(): Promise<void> {
    const mine = ++latestSync;
    const liveIds = new Set<string>();
    pending.add(liveIds);
    try {
      const res = await client.fetchJobs();
      // Drop a snapshot superseded by a newer resync (quick reconnect cycles).
      if (!stopped && mine === latestSync) replaceJobs(res.jobs, liveIds);
    } catch {
      // htoolsd offline: the panel keeps what it has; the next reconnect retries.
    } finally {
      pending.delete(liveIds);
    }
  }

  const ac = client.subscribeJobs(
    (s) => {
      for (const ids of pending) ids.add(s.jobId);
      applyJobSummary(s);
    },
    () => {
      disconnected = true;
    },
    () => {
      if (!disconnected) return; // the first connection is covered by `loaded`
      disconnected = false;
      void resync();
    },
  );
  const loaded = resync();
  return {
    loaded,
    abort() {
      stopped = true;
      ac.abort();
    },
  };
}
