import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiClient, fromSnake, toSnake } from './client';
import type { ConvertRequest, JobResponse } from './types';

describe('toSnake / fromSnake', () => {
  it('round-trips nested objects', () => {
    const camel = {
      jobId: 'job_1',
      currentItem: 'photo.png',
      nested: { fooBar: 1, list: [{ aB: 'x' }] },
    };
    const wire = toSnake(camel);
    expect(wire).toEqual({
      job_id: 'job_1',
      current_item: 'photo.png',
      nested: { foo_bar: 1, list: [{ a_b: 'x' }] },
    });
    expect(fromSnake(wire)).toEqual(camel);
  });

  it('leaves primitives and nulls alone', () => {
    expect(toSnake(null)).toBeNull();
    expect(toSnake(42)).toBe(42);
    expect(toSnake('hello')).toBe('hello');
    expect(fromSnake(null)).toBeNull();
  });
});

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

describe('ApiClient.requestJSON happy path', () => {
  it('serializes camelCase request to snake_case and parses back', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input.url;
      expect(url).toBe('/v1/image/convert');
      expect(init?.method).toBe('POST');
      const body = JSON.parse(String(init?.body));
      expect(body).toEqual({
        source: { path: '/in/photo.png' },
        target_format: 'JPEG',
        options: {
          quality: 88,
          max_width: 0,
          max_height: 0,
          strip_metadata: false,
        },
        output: { directory: '/out' },
      });
      return jsonResponse(202, { job_id: 'job_abc' });
    });

    const client = new ApiClient({ fetch: fetchMock as typeof fetch });
    const req: ConvertRequest = {
      source: { path: '/in/photo.png' },
      targetFormat: 'JPEG',
      options: { quality: 88, maxWidth: 0, maxHeight: 0, stripMetadata: false },
      output: { directory: '/out' },
    };
    const res: JobResponse = await client.imageConvert(req);
    expect(res).toEqual({ jobId: 'job_abc' });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

describe('ApiClient error envelope', () => {
  it('throws ApiError carrying the unwrapped envelope on non-2xx', async () => {
    const fetchMock = vi.fn(
      async () =>
        jsonResponse(503, {
          error: {
            code: 'MISSING_BINARY',
            message: 'pdftoppm not found',
            detail: 'brew install poppler',
          },
        }) as Response,
    );

    const client = new ApiClient({ fetch: fetchMock as typeof fetch });

    await expect(
      client.pdfToImage({
        source: { path: '/x.pdf' },
        pages: { from: 1, to: 1 },
        dpi: 150,
        targetFormat: 'PNG',
        output: { directory: '/out' },
      }),
    ).rejects.toMatchObject({
      name: 'ApiError',
      status: 503,
      message: 'pdftoppm not found',
      envelope: {
        code: 'MISSING_BINARY',
        message: 'pdftoppm not found',
        detail: 'brew install poppler',
      },
    });
  });

  it('falls back to a synthetic message when the body is not JSON', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response('upstream timeout', {
          status: 502,
          headers: { 'content-type': 'text/plain' },
        }),
    );
    const client = new ApiClient({ fetch: fetchMock as typeof fetch });
    await expect(client.fetchSysdep()).rejects.toMatchObject({
      name: 'ApiError',
      status: 502,
      envelope: null,
    });
  });
});

describe('ApiClient.fetchSysdep camelCase mapping', () => {
  it('maps install_hint, used_alias on each result', async () => {
    const fetchMock = vi.fn(
      async () =>
        jsonResponse(200, [
          {
            name: 'pdftoppm',
            found: false,
            used_alias: '',
            description: 'render PDF pages',
            install_hint: { darwin: 'brew install poppler' },
          },
        ]) as Response,
    );
    const client = new ApiClient({ fetch: fetchMock as typeof fetch });
    const out = await client.fetchSysdep();
    expect(out).toEqual([
      {
        name: 'pdftoppm',
        found: false,
        usedAlias: '',
        description: 'render PDF pages',
        installHint: { darwin: 'brew install poppler' },
      },
    ]);
  });
});

describe('ApiClient.fetchJobs', () => {
  it('maps the snake_case job list into camelCase JobSummary objects', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo) => {
      const url = typeof input === 'string' ? input : input.url;
      expect(url).toBe('/v1/jobs');
      return jsonResponse(200, {
        jobs: [
          {
            job_id: 'job_1',
            tool: 'image',
            action: 'convert',
            status: 'done',
            completed: true,
            fraction: 1,
            current_item: 'a.png',
            started_unix_ms: 1700000000000,
          },
        ],
      }) as Response;
    });
    const client = new ApiClient({ fetch: fetchMock as typeof fetch });
    const res = await client.fetchJobs();
    expect(res.jobs).toEqual([
      {
        jobId: 'job_1',
        tool: 'image',
        action: 'convert',
        status: 'done',
        completed: true,
        fraction: 1,
        currentItem: 'a.png',
        startedUnixMs: 1700000000000,
      },
    ]);
  });
});

describe('ApiClient.subscribeJobs', () => {
  class FakeEventSource {
    static last: FakeEventSource | null = null;
    url: string;
    onmessage: ((e: MessageEvent<string>) => void) | null = null;
    onerror: (() => void) | null = null;
    closed = false;
    constructor(url: string) {
      this.url = url;
      FakeEventSource.last = this;
    }
    close(): void {
      this.closed = true;
    }
  }

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('parses each SSE frame into a JobSummary and closes on abort', () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    const client = new ApiClient();
    const seen: unknown[] = [];
    const ac = client.subscribeJobs((s) => seen.push(s));

    const es = FakeEventSource.last;
    expect(es?.url).toBe('/v1/jobs/events');
    es?.onmessage?.({
      data: JSON.stringify({ job_id: 'j', tool: 'pdf', action: 'merge', status: 'running', completed: false }),
    } as MessageEvent<string>);
    expect(seen).toEqual([
      { jobId: 'j', tool: 'pdf', action: 'merge', status: 'running', completed: false },
    ]);

    ac.abort();
    expect(es?.closed).toBe(true);
  });
});
