import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';
import {
  dismissAll,
  dismissPopup,
  popups,
  pushPopup,
  subscribeJobsForFailures,
} from './notifications';
import { applyJobSummary, jobs, startJobsFeed, type Job } from './jobs';
import type { ApiClient } from '../api/client';
import type { JobSummary } from '../api/types';

function job(over: Partial<Job> = {}): Job {
  return {
    id: 'j1',
    tool: 'archive · extract',
    currentItem: null,
    progress: 0,
    estimated: false,
    status: 'wait',
    logs: [],
    ...over,
  };
}

beforeEach(() => {
  dismissAll();
  jobs.set([]);
});

describe('pushPopup / dismissPopup', () => {
  it('adds entries and dedupes by id', () => {
    pushPopup({ id: 'a', tone: 'info', sticky: false, title: 't', message: 'm' });
    pushPopup({ id: 'a', tone: 'info', sticky: false, title: 't', message: 'm' }); // dup
    pushPopup({ id: 'b', tone: 'error', sticky: true, title: 't', message: 'm' });
    expect(get(popups).map((p) => p.id)).toEqual(['a', 'b']);
  });

  it('dismissPopup removes by id', () => {
    pushPopup({ id: 'a', tone: 'info', sticky: false, title: 't', message: 'm' });
    pushPopup({ id: 'b', tone: 'info', sticky: false, title: 't', message: 'm' });
    dismissPopup('a');
    expect(get(popups).map((p) => p.id)).toEqual(['b']);
  });

  it('stamps createdAt when not supplied', () => {
    const before = Date.now();
    pushPopup({ id: 'x', tone: 'info', sticky: false, title: 't', message: 'm' });
    const p = get(popups)[0];
    expect(p.createdAt).toBeGreaterThanOrEqual(before);
  });
});

describe('subscribeJobsForFailures with the live feed (Layout wiring)', () => {
  let emitLive: (s: JobSummary) => void = () => {};

  /** Fake client: the GET /v1/jobs snapshot resolves asynchronously, like fetch. */
  function fakeClient(snapshot: JobSummary[]): ApiClient {
    return {
      subscribeJobs: (onEvent: (s: JobSummary) => void) => {
        emitLive = onEvent;
        return new AbortController();
      },
      fetchJobs: () => Promise.resolve({ jobs: snapshot }),
    } as unknown as ApiClient;
  }

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('does not pop for jobs that had already failed before the page loaded', async () => {
    const feed = startJobsFeed(
      fakeClient([
        {
          jobId: 'old',
          tool: 'archive',
          action: 'extract',
          status: 'failed',
          completed: true,
          error: { code: 'IO_ERROR', message: 'failed in an earlier session' },
        },
      ]),
    );
    const unsub = subscribeJobsForFailures(feed.loaded);
    await feed.loaded;
    await Promise.resolve();

    expect(get(jobs).map((j) => [j.id, j.status])).toEqual([['old', 'fail']]);
    expect(get(popups)).toHaveLength(0);

    unsub();
    feed.abort();
  });

  it('pops exactly once for a job that fails after the page loaded', async () => {
    const feed = startJobsFeed(fakeClient([]));
    const unsub = subscribeJobsForFailures(feed.loaded);
    await feed.loaded;
    await Promise.resolve();

    const s = { jobId: 'new', tool: 'image', action: 'convert', completed: false } as const;
    emitLive({ ...s, status: 'queued' });
    emitLive({ ...s, status: 'running' });
    emitLive({ ...s, status: 'failed', completed: true, error: { code: 'IO_ERROR', message: 'disk full' } });
    emitLive({ ...s, status: 'failed', completed: true, error: { code: 'IO_ERROR', message: 'disk full' } });
    expect(get(popups).map((p) => [p.id, p.message])).toEqual([['job:new', 'disk full']]);

    // A job first seen already failed after load (its earlier frames were
    // missed) still happened during this session, so it pops too.
    applyJobSummary({ jobId: 'fast', tool: 'pdf', action: 'merge', status: 'failed', completed: true });
    expect(get(popups).map((p) => p.id)).toEqual(['job:new', 'job:fast']);

    unsub();
    feed.abort();
  });
});

describe('subscribeJobsForFailures', () => {
  it('does not emit popups for jobs already failed at subscribe time', () => {
    jobs.set([job({ id: 'old', status: 'fail' })]);
    const unsub = subscribeJobsForFailures();
    expect(get(popups)).toHaveLength(0);
    unsub();
  });

  it('emits a sticky popup when a job newly transitions to fail', () => {
    const unsub = subscribeJobsForFailures();

    jobs.set([job({ id: 'j2', status: 'running' })]);
    expect(get(popups)).toHaveLength(0);

    jobs.update((list) =>
      list.map((j) => ({
        ...j,
        status: 'fail',
        logs: [{ ts: '00:00', level: 'ERROR', message: 'disk full' }],
      })),
    );

    const list = get(popups);
    expect(list).toHaveLength(1);
    expect(list[0]).toMatchObject({
      id: 'job:j2',
      tone: 'error',
      sticky: true,
      jobId: 'j2',
      message: 'disk full',
    });
    expect(list[0].title).toContain('failed');

    unsub();
  });

  it('does not re-emit a popup for the same job staying in fail across snapshots', () => {
    const unsub = subscribeJobsForFailures();
    const failed = job({
      id: 'j3',
      status: 'fail',
      logs: [{ ts: '00:00', level: 'ERROR', message: 'boom' }],
    });
    jobs.set([failed]);
    jobs.set([failed]); // a fresh snapshot with no status change
    expect(get(popups)).toHaveLength(1);
    unsub();
  });

  it('falls back to a generic message when the job has no ERROR log', () => {
    const unsub = subscribeJobsForFailures();
    jobs.set([job({ id: 'j4', status: 'running' })]);
    jobs.update((list) => list.map((j) => ({ ...j, status: 'fail', logs: [] })));
    const p = get(popups)[0];
    expect(p.message).toMatch(/Jobs page/i);
    unsub();
  });
});
