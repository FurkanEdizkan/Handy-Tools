import { writable, type Writable } from 'svelte/store';

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

/**
 * Placeholder fixtures. Real wiring to /v1/jobs + SSE lands with #78 / Track C
 * once the backend queue (#74) exposes the stream.
 */
const seed: Job[] = [
  {
    id: 'job_001',
    tool: 'Convert images',
    currentItem: 'photo_4.png',
    progress: 0.62,
    status: 'running',
    logs: [
      { ts: '00:12.041', level: 'INFO', message: 'opening batch of 8 files' },
      { ts: '00:12.214', level: 'DEBUG', message: 'decode photo_1.png → 1024x768 RGBA' },
      { ts: '00:12.301', level: 'DONE', message: 'photo_1.png → photo_1.jpg (188 KB)' },
      { ts: '00:13.420', level: 'DONE', message: 'photo_2.png → photo_2.jpg (203 KB)' },
      { ts: '00:14.812', level: 'DONE', message: 'photo_3.png → photo_3.jpg (191 KB)' },
      { ts: '00:15.001', level: 'INFO', message: 'encoding photo_4.png' },
    ],
  },
  {
    id: 'job_002',
    tool: 'PDF utilities',
    currentItem: 'manual.pdf',
    progress: 0,
    status: 'fail',
    logs: [
      { ts: '00:00.012', level: 'INFO', message: 'opening manual.pdf' },
      { ts: '00:00.118', level: 'DEBUG', message: 'sysdep.find: looking for "pdftoppm"' },
      { ts: '00:00.220', level: 'WARN', message: 'tried /usr/local/bin, /opt/homebrew/bin' },
      { ts: '00:00.318', level: 'ERROR', message: 'sysdep.MISSING_BINARY: pdftoppm' },
      { ts: '00:00.318', level: 'HINT', message: 'brew install poppler' },
      { ts: '00:00.319', level: 'FAIL', message: 'job aborted after 318ms' },
    ],
  },
  {
    id: 'job_003',
    tool: 'Archive',
    currentItem: 'release-2026-05.zip',
    progress: 1,
    status: 'done',
    logs: [
      { ts: '00:00.000', level: 'INFO', message: 'packing 12 files' },
      { ts: '00:01.842', level: 'DONE', message: 'wrote release-2026-05.zip (4.8 MB)' },
    ],
  },
  {
    id: 'job_004',
    tool: 'Convert images',
    currentItem: null,
    progress: 0,
    status: 'wait',
    logs: [],
  },
];

export const jobs: Writable<Job[]> = writable(seed);

export const expandedJobs: Writable<Set<string>> = writable(new Set([seed[1].id]));

export function toggleJobExpanded(id: string): void {
  expandedJobs.update((set) => {
    const next = new Set(set);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  });
}
