import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';
import { ApiClient } from '../api/client';
import type { JobSummary } from '../api/types';
import { applyJobSummary, jobs, loadJobs, replaceJobs, startJobsFeed } from './jobs';

function summary(over: Partial<JobSummary> = {}): JobSummary {
  return {
    jobId: 'j1',
    tool: 'image',
    action: 'convert',
    status: 'running',
    completed: false,
    ...over,
  };
}

/** Minimal EventSource stand-in — jsdom does not implement EventSource. */
class FakeEventSource {
  static instances: FakeEventSource[] = [];
  url: string;
  onmessage: ((e: MessageEvent<string>) => void) | null = null;
  onerror: (() => void) | null = null;
  onopen: (() => void) | null = null;
  closed = false;

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }

  close(): void {
    this.closed = true;
  }

  emit(data: unknown): void {
    this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent<string>);
  }
}

beforeEach(() => {
  jobs.set([]);
  FakeEventSource.instances = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('applyJobSummary', () => {
  it('inserts a new row, mapping status, progress and tool label', () => {
    applyJobSummary(summary({ status: 'running', fraction: 0.5, currentItem: 'a.png' }));
    const list = get(jobs);
    expect(list).toHaveLength(1);
    expect(list[0]).toMatchObject({
      id: 'j1',
      tool: 'image · convert',
      status: 'running',
      progress: 0.5,
      currentItem: 'a.png',
    });
  });

  it('maps queued → wait and failed → fail', () => {
    applyJobSummary(summary({ jobId: 'q', status: 'queued', completed: false }));
    applyJobSummary(summary({ jobId: 'f', status: 'failed', completed: true }));
    const byId = Object.fromEntries(get(jobs).map((j) => [j.id, j.status]));
    expect(byId.q).toBe('wait');
    expect(byId.f).toBe('fail');
  });

  it('updates a job in place and accumulates distinct log messages', () => {
    applyJobSummary(summary({ message: 'processed a.png', fraction: 0.5 }));
    applyJobSummary(summary({ message: 'processed a.png', fraction: 0.5 })); // dup
    applyJobSummary(summary({ message: 'processed b.png', fraction: 1, status: 'done', completed: true }));
    const list = get(jobs);
    expect(list).toHaveLength(1); // no duplicate row
    expect(list[0].status).toBe('done');
    expect(list[0].progress).toBe(1);
    expect(list[0].logs.map((l) => l.message)).toEqual(['processed a.png', 'processed b.png']);
  });

  it('surfaces a terminal error as an ERROR log line', () => {
    applyJobSummary(
      summary({ status: 'failed', completed: true, error: { code: 'IO_ERROR', message: 'disk full' } }),
    );
    const job = get(jobs)[0];
    expect(job.status).toBe('fail');
    expect(job.logs.at(-1)).toMatchObject({ level: 'ERROR', message: 'disk full' });
  });
});

describe('loadJobs', () => {
  it('folds the GET /v1/jobs snapshot into the store', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            jobs: [
              { job_id: 'a', tool: 'pdf', action: 'merge', status: 'done', completed: true, fraction: 1 },
              { job_id: 'b', tool: 'image', action: 'convert', status: 'running', completed: false },
            ],
          }),
          { status: 200, headers: { 'content-type': 'application/json' } },
        ),
    );
    const client = new ApiClient({ fetch: fetchMock as typeof fetch });

    await loadJobs(client);

    const ids = get(jobs)
      .map((j) => j.id)
      .sort();
    expect(ids).toEqual(['a', 'b']);
    expect(fetchMock).toHaveBeenCalledOnce();
  });
});

describe('startJobsFeed', () => {
  it('folds events from the live stream and closes the source on abort', async () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    const fetchMock = vi.fn(
      async () =>
        new Response(JSON.stringify({ jobs: [] }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        }),
    );
    const client = new ApiClient({ fetch: fetchMock as typeof fetch });

    const feed = startJobsFeed(client);
    const es = FakeEventSource.instances[0];
    expect(es.url).toContain('/v1/jobs/events');

    es.emit({ job_id: 'live', tool: 'archive', action: 'extract', status: 'running', completed: false });
    expect(get(jobs).map((j) => j.id)).toContain('live');

    feed.abort();
    expect(es.closed).toBe(true);
  });
});

/** jobsResponse builds a GET /v1/jobs Response from snake_case job rows. */
function jobsResponse(rows: Record<string, unknown>[]): Response {
  return new Response(JSON.stringify({ jobs: rows }), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  });
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe('replaceJobs', () => {
  it('drops rows missing from the snapshot and keeps log history for the rest', () => {
    applyJobSummary(summary({ jobId: 'gone', status: 'running' }));
    applyJobSummary(summary({ jobId: 'kept', status: 'running', message: 'processed a.png' }));

    replaceJobs([summary({ jobId: 'kept', status: 'done', completed: true, message: 'wrote /x/a.jpg' })]);

    const list = get(jobs);
    expect(list.map((j) => j.id)).toEqual(['kept']);
    expect(list[0].status).toBe('done');
    expect(list[0].logs.map((l) => l.message)).toEqual(['processed a.png', 'wrote /x/a.jpg']);
  });

  it('prefers the live row for ids the stream updated after the snapshot was taken', () => {
    applyJobSummary(summary({ jobId: 'x', status: 'done', completed: true }));
    applyJobSummary(summary({ jobId: 'fresh', status: 'running' }));
    // The snapshot predates both: x still running, fresh not yet created.
    replaceJobs([summary({ jobId: 'x', status: 'running' })], new Set(['x', 'fresh']));
    const byId = Object.fromEntries(get(jobs).map((j) => [j.id, j.status]));
    expect(byId).toEqual({ x: 'done', fresh: 'running' });
  });

  it('orders rows newest first like the incremental fold', () => {
    replaceJobs([summary({ jobId: 'old' }), summary({ jobId: 'new' })]);
    expect(get(jobs).map((j) => j.id)).toEqual(['new', 'old']);
  });
});

describe('startJobsFeed reconnect', () => {
  it('re-fetches /v1/jobs after the stream reconnects and replaces stuck rows', async () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jobsResponse([
          { job_id: 'stuck', tool: 'archive', action: 'extract', status: 'running', completed: false },
          { job_id: 'ok', tool: 'pdf', action: 'merge', status: 'running', completed: false },
        ]),
      )
      // After htoolsd restarts only jobs it knows about come back.
      .mockResolvedValueOnce(
        jobsResponse([{ job_id: 'ok', tool: 'pdf', action: 'merge', status: 'done', completed: true }]),
      );
    const client = new ApiClient({ fetch: fetchMock as unknown as typeof fetch });

    const feed = startJobsFeed(client);
    await feed.loaded;
    const es = FakeEventSource.instances[0];
    es.onopen?.(); // the first connection is not a reconnect
    await flush();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(get(jobs).map((j) => [j.id, j.status])).toEqual([
      ['ok', 'running'],
      ['stuck', 'running'],
    ]);

    es.onerror?.(); // stream dropped — the completion frames were missed
    es.onopen?.(); // EventSource reconnected on its own
    await flush();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(get(jobs).map((j) => [j.id, j.status])).toEqual([['ok', 'done']]);
    feed.abort();
  });

  it('does not apply a resync that resolves after abort', async () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    let release: (r: Response) => void = () => {};
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jobsResponse([]))
      .mockImplementationOnce(() => new Promise<Response>((r) => (release = r)));
    const client = new ApiClient({ fetch: fetchMock as unknown as typeof fetch });

    const feed = startJobsFeed(client);
    await feed.loaded;
    const es = FakeEventSource.instances[0];
    es.onerror?.();
    es.onopen?.();
    feed.abort();
    release(jobsResponse([{ job_id: 'late', tool: 'x', action: 'y', status: 'running', completed: false }]));
    await flush();
    expect(get(jobs)).toEqual([]);
  });

  it('ignores an older resync that resolves after a newer one', async () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    let releaseOld: (r: Response) => void = () => {};
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jobsResponse([]))
      .mockImplementationOnce(() => new Promise<Response>((r) => (releaseOld = r)))
      .mockResolvedValueOnce(
        jobsResponse([{ job_id: 'j', tool: 'pdf', action: 'merge', status: 'done', completed: true }]),
      );
    const client = new ApiClient({ fetch: fetchMock as unknown as typeof fetch });

    const feed = startJobsFeed(client);
    await feed.loaded;
    const es = FakeEventSource.instances[0];
    es.onerror?.();
    es.onopen?.(); // resync #1 — slow
    es.onerror?.();
    es.onopen?.(); // resync #2 — fast, newer
    await flush();
    expect(get(jobs).map((j) => [j.id, j.status])).toEqual([['j', 'done']]);

    releaseOld(
      jobsResponse([{ job_id: 'j', tool: 'pdf', action: 'merge', status: 'running', completed: false }]),
    );
    await flush();
    expect(get(jobs).map((j) => [j.id, j.status])).toEqual([['j', 'done']]);
    feed.abort();
  });
});
