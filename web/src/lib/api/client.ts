/**
 * Typed wrapper around fetch + EventSource for talking to htoolsd.
 *
 * - All endpoints return camelCase TS objects; the wire format is snake_case,
 *   converted at the boundary by toSnake / fromSnake.
 * - postJSON throws ApiError on non-2xx; the envelope is preserved.
 * - subscribeJob wraps EventSource and exposes an AbortController for
 *   teardown (Svelte components call .abort() from onDestroy).
 */

import type {
  ConfigPatch,
  ConfigResponse,
  ConvertRequest,
  ErrorEnvelope,
  ExtractRequest,
  HealthResponse,
  CompressRequest,
  InspectRequest,
  InspectResponse,
  JobResponse,
  JobsResponse,
  JobSummary,
  PdfMergeRequest,
  PdfSplitRequest,
  PdfToImageRequest,
  PdfToTextRequest,
  ProgressEvent,
  SysdepResult,
  UploadCreateResponse,
} from './types';

const DEFAULT_BASE = '';

export class ApiError extends Error {
  readonly status: number;
  readonly envelope: ErrorEnvelope | null;

  constructor(status: number, message: string, envelope: ErrorEnvelope | null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.envelope = envelope;
  }
}

export interface ClientOptions {
  baseUrl?: string;
  fetch?: typeof fetch;
}

export class ApiClient {
  private readonly baseUrl: string;
  private readonly fetchFn: typeof fetch;

  constructor(opts: ClientOptions = {}) {
    this.baseUrl = opts.baseUrl ?? DEFAULT_BASE;
    this.fetchFn = opts.fetch ?? globalThis.fetch.bind(globalThis);
  }

  // ---- Image ----------------------------------------------------------------

  imageConvert(req: ConvertRequest): Promise<JobResponse> {
    return this.postJSON('/v1/image/convert', req);
  }

  // ---- Archive --------------------------------------------------------------

  archiveInspect(req: InspectRequest): Promise<InspectResponse> {
    return this.postJSON('/v1/archive/inspect', req);
  }

  archiveExtract(req: ExtractRequest): Promise<JobResponse> {
    return this.postJSON('/v1/archive/extract', req);
  }

  archiveCompress(req: CompressRequest): Promise<JobResponse> {
    return this.postJSON('/v1/archive/compress', req);
  }

  // ---- PDF ------------------------------------------------------------------

  pdfToImage(req: PdfToImageRequest): Promise<JobResponse> {
    return this.postJSON('/v1/pdf/to-image', req);
  }

  pdfToText(req: PdfToTextRequest): Promise<JobResponse> {
    return this.postJSON('/v1/pdf/to-text', req);
  }

  pdfMerge(req: PdfMergeRequest): Promise<JobResponse> {
    return this.postJSON('/v1/pdf/merge', req);
  }

  pdfSplit(req: PdfSplitRequest): Promise<JobResponse> {
    return this.postJSON('/v1/pdf/split', req);
  }

  // ---- Uploads --------------------------------------------------------------

  /**
   * Upload one or more files to a fresh server-side workspace (POST
   * /v1/uploads). The browser uses this when it has no native filesystem
   * paths to give the tool endpoints — the response hands back server-side
   * paths to feed into imageConvert / pdf* / archive* as FileRef.path.
   */
  uploadFiles(files: File[]): Promise<UploadCreateResponse> {
    const form = new FormData();
    for (const f of files) form.append('files', f, f.name);
    return this.requestForm('/v1/uploads', form);
  }

  /**
   * Absolute URL of an upload's download endpoint — suitable for an
   * `<a href download>` once the job that wrote into the workspace completes.
   */
  uploadDownloadUrl(uploadId: string): string {
    return `${this.baseUrl}/v1/uploads/${encodeURIComponent(uploadId)}/download`;
  }

  /** Reap an upload workspace (DELETE /v1/uploads/{id}). Best-effort. */
  async deleteUpload(uploadId: string): Promise<void> {
    const url = `${this.baseUrl}/v1/uploads/${encodeURIComponent(uploadId)}`;
    await this.fetchFn(url, { method: 'DELETE' });
  }

  // ---- Jobs -----------------------------------------------------------------

  /** GET /v1/jobs — a snapshot of every job the shared queue knows about. */
  fetchJobs(): Promise<JobsResponse> {
    return this.getJSON('/v1/jobs');
  }

  // ---- Sysdep / Health / Config --------------------------------------------

  fetchSysdep(): Promise<SysdepResult[]> {
    return this.getJSON('/v1/sysdep');
  }

  fetchHealth(): Promise<HealthResponse> {
    return this.getJSON('/v1/health');
  }

  fetchConfig(): Promise<ConfigResponse> {
    return this.getJSON('/v1/config');
  }

  patchConfig(patch: ConfigPatch): Promise<ConfigResponse> {
    return this.requestJSON('PATCH', '/v1/config', patch);
  }

  // ---- SSE ------------------------------------------------------------------

  /**
   * Subscribe to a job's progress stream. The returned AbortController closes
   * the underlying EventSource when its signal is aborted — Svelte components
   * typically call .abort() from onDestroy / on navigation away.
   *
   * onError fires on transport-level failures (EventSource error event). The
   * EventSource API auto-reconnects after a transient drop; pass a non-null
   * onError if you want to react to the disconnect (e.g. flip the connection
   * badge to "offline").
   */
  subscribeJob(
    jobId: string,
    onEvent: (event: ProgressEvent) => void,
    onError?: () => void,
  ): AbortController {
    const url = `${this.baseUrl}/v1/jobs/${encodeURIComponent(jobId)}/events`;
    const es = new EventSource(url);
    const ac = new AbortController();

    es.onmessage = (msg: MessageEvent<string>) => {
      try {
        const raw = JSON.parse(msg.data) as Record<string, unknown>;
        onEvent(fromSnake(raw) as ProgressEvent);
      } catch {
        /* malformed frame — drop, the server should never emit these */
      }
    };

    if (onError) {
      es.onerror = onError;
    }

    ac.signal.addEventListener(
      'abort',
      () => {
        es.close();
      },
      { once: true },
    );

    return ac;
  }

  /**
   * Subscribe to the all-jobs lifecycle stream (GET /v1/jobs/events). Each
   * frame is a JobSummary snapshot. Unlike subscribeJob this stream does not
   * replay history — pair it with fetchJobs for the initial state. Returns an
   * AbortController that closes the EventSource when aborted.
   */
  subscribeJobs(
    onEvent: (event: JobSummary) => void,
    onError?: () => void,
  ): AbortController {
    const es = new EventSource(`${this.baseUrl}/v1/jobs/events`);
    const ac = new AbortController();

    es.onmessage = (msg: MessageEvent<string>) => {
      try {
        const raw = JSON.parse(msg.data) as Record<string, unknown>;
        onEvent(fromSnake(raw) as JobSummary);
      } catch {
        /* malformed frame — drop, the server should never emit these */
      }
    };

    if (onError) {
      es.onerror = onError;
    }

    ac.signal.addEventListener('abort', () => es.close(), { once: true });

    return ac;
  }

  // ---- internals ------------------------------------------------------------

  private postJSON<TReq, TRes>(path: string, body: TReq): Promise<TRes> {
    return this.requestJSON('POST', path, body);
  }

  private getJSON<TRes>(path: string): Promise<TRes> {
    return this.requestJSON('GET', path, undefined);
  }

  private async requestJSON<TReq, TRes>(
    method: 'GET' | 'POST' | 'PATCH',
    path: string,
    body: TReq | undefined,
  ): Promise<TRes> {
    const init: RequestInit = {
      method,
      headers: { accept: 'application/json' },
    };
    if (body !== undefined) {
      init.headers = { ...init.headers, 'content-type': 'application/json' };
      init.body = JSON.stringify(toSnake(body));
    }

    const res = await this.fetchFn(`${this.baseUrl}${path}`, init);
    const text = await res.text();
    const parsed = text ? safeParseJSON(text) : null;

    if (!res.ok) {
      const env = extractError(parsed);
      throw new ApiError(
        res.status,
        env?.message ?? `${method} ${path} → ${res.status}`,
        env,
      );
    }

    return (parsed === null ? (null as TRes) : (fromSnake(parsed) as TRes));
  }

  /**
   * POST a multipart/form-data body. Unlike requestJSON this must NOT set a
   * content-type header — the browser fills in the multipart boundary — and
   * the body is sent as-is rather than JSON-encoded. The response is still
   * snake_case JSON, decoded the same way.
   */
  private async requestForm<TRes>(path: string, body: FormData): Promise<TRes> {
    const res = await this.fetchFn(`${this.baseUrl}${path}`, {
      method: 'POST',
      headers: { accept: 'application/json' },
      body,
    });
    const text = await res.text();
    const parsed = text ? safeParseJSON(text) : null;

    if (!res.ok) {
      const env = extractError(parsed);
      throw new ApiError(
        res.status,
        env?.message ?? `POST ${path} → ${res.status}`,
        env,
      );
    }

    return (parsed === null ? (null as TRes) : (fromSnake(parsed) as TRes));
  }
}

/** Module-level default client — mounts on the same origin htoolsd serves. */
export const api = new ApiClient();

// ---- helpers ----------------------------------------------------------------

function safeParseJSON(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

function extractError(parsed: unknown): ErrorEnvelope | null {
  if (!parsed || typeof parsed !== 'object') return null;
  const obj = parsed as Record<string, unknown>;
  const wrapped = obj.error;
  if (wrapped && typeof wrapped === 'object') {
    return fromSnake(wrapped as Record<string, unknown>) as ErrorEnvelope;
  }
  if ('code' in obj && 'message' in obj) {
    return fromSnake(obj) as ErrorEnvelope;
  }
  return null;
}

/**
 * Convert camelCase keys to snake_case for the wire. Walks objects and arrays
 * recursively. Leaves Date / primitives alone.
 */
export function toSnake(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(toSnake);
  if (value === null || typeof value !== 'object') return value;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    out[camelToSnake(k)] = toSnake(v);
  }
  return out;
}

/**
 * Convert snake_case keys to camelCase coming back from the wire.
 */
export function fromSnake(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(fromSnake);
  if (value === null || typeof value !== 'object') return value;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    out[snakeToCamel(k)] = fromSnake(v);
  }
  return out;
}

function camelToSnake(s: string): string {
  return s.replace(/[A-Z]/g, (m) => '_' + m.toLowerCase());
}

function snakeToCamel(s: string): string {
  return s.replace(/_([a-z0-9])/g, (_, c: string) => c.toUpperCase());
}
