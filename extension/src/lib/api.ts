/**
 * HtoolsClient — a typed wrapper around fetch + EventSource for the htoolsd
 * HTTP API, used by the Chrome extension.
 *
 * It is the cross-origin sibling of web/src/lib/api/client.ts: the snake_case
 * conversion helpers and the ApiError shape are copied from there, but here
 * baseUrl is mandatory and configurable (the extension talks to a separate
 * origin, never same-origin).
 */

import type {
  BatchConvertRequest,
  CompressRequest,
  ErrorEnvelope,
  ExtractRequest,
  HealthResponse,
  JobResponse,
  JobsResponse,
  PdfMergeRequest,
  PdfSplitRequest,
  PdfToImageRequest,
  PdfToTextRequest,
  ProgressEvent,
  UploadCreateResponse,
} from './types';

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

/** joinUrl combines a base origin with an API path, tolerating a trailing
 *  slash on the base so "http://h:8080/" + "/v1/x" === "http://h:8080/v1/x". */
export function joinUrl(base: string, path: string): string {
  return base.replace(/\/+$/, '') + path;
}

export interface HtoolsClientOptions {
  baseUrl: string;
  fetch?: typeof fetch;
}

export class HtoolsClient {
  private readonly baseUrl: string;
  private readonly fetchFn: typeof fetch;

  constructor(opts: HtoolsClientOptions) {
    this.baseUrl = opts.baseUrl;
    this.fetchFn = opts.fetch ?? globalThis.fetch.bind(globalThis);
  }

  // ---- Uploads --------------------------------------------------------------

  uploadFiles(files: File[]): Promise<UploadCreateResponse> {
    const form = new FormData();
    for (const f of files) form.append('files', f, f.name);
    return this.requestForm('/v1/uploads', form);
  }

  uploadDownloadUrl(uploadId: string): string {
    return joinUrl(this.baseUrl, `/v1/uploads/${encodeURIComponent(uploadId)}/download`);
  }

  async deleteUpload(uploadId: string): Promise<void> {
    await this.fetchFn(joinUrl(this.baseUrl, `/v1/uploads/${encodeURIComponent(uploadId)}`), {
      method: 'DELETE',
    });
  }

  // ---- Tools ----------------------------------------------------------------

  imageBatchConvert(req: BatchConvertRequest): Promise<JobResponse> {
    return this.requestJSON('POST', '/v1/image/batch-convert', req);
  }

  pdfToImage(req: PdfToImageRequest): Promise<JobResponse> {
    return this.requestJSON('POST', '/v1/pdf/to-image', req);
  }

  pdfToText(req: PdfToTextRequest): Promise<JobResponse> {
    return this.requestJSON('POST', '/v1/pdf/to-text', req);
  }

  pdfMerge(req: PdfMergeRequest): Promise<JobResponse> {
    return this.requestJSON('POST', '/v1/pdf/merge', req);
  }

  pdfSplit(req: PdfSplitRequest): Promise<JobResponse> {
    return this.requestJSON('POST', '/v1/pdf/split', req);
  }

  archiveCompress(req: CompressRequest): Promise<JobResponse> {
    return this.requestJSON('POST', '/v1/archive/compress', req);
  }

  archiveExtract(req: ExtractRequest): Promise<JobResponse> {
    return this.requestJSON('POST', '/v1/archive/extract', req);
  }

  // ---- Jobs / health --------------------------------------------------------

  fetchJobs(): Promise<JobsResponse> {
    return this.requestJSON('GET', '/v1/jobs', undefined);
  }

  fetchHealth(): Promise<HealthResponse> {
    return this.requestJSON('GET', '/v1/health', undefined);
  }

  /**
   * Subscribe to a job's SSE progress stream. The returned AbortController
   * closes the EventSource when aborted. The stream replays history, so it is
   * safe to subscribe after the job has already finished.
   */
  subscribeJob(
    jobId: string,
    onEvent: (e: ProgressEvent) => void,
    onError?: () => void,
  ): AbortController {
    const url = joinUrl(this.baseUrl, `/v1/jobs/${encodeURIComponent(jobId)}/events`);
    const es = new EventSource(url);
    const ac = new AbortController();
    es.onmessage = (msg: MessageEvent<string>) => {
      try {
        onEvent(fromSnake(JSON.parse(msg.data)) as ProgressEvent);
      } catch {
        /* malformed frame — drop */
      }
    };
    if (onError) es.onerror = onError;
    ac.signal.addEventListener('abort', () => es.close(), { once: true });
    return ac;
  }

  // ---- internals ------------------------------------------------------------

  private async requestJSON<TReq, TRes>(
    method: 'GET' | 'POST',
    path: string,
    body: TReq | undefined,
  ): Promise<TRes> {
    const init: RequestInit = { method, headers: { accept: 'application/json' } };
    if (body !== undefined) {
      init.headers = { ...init.headers, 'content-type': 'application/json' };
      init.body = JSON.stringify(toSnake(body));
    }
    return this.parse(await this.fetchFn(joinUrl(this.baseUrl, path), init), `${method} ${path}`);
  }

  private async requestForm<TRes>(path: string, body: FormData): Promise<TRes> {
    // No content-type header — the browser sets the multipart boundary.
    const res = await this.fetchFn(joinUrl(this.baseUrl, path), {
      method: 'POST',
      headers: { accept: 'application/json' },
      body,
    });
    return this.parse(res, `POST ${path}`);
  }

  private async parse<TRes>(res: Response, label: string): Promise<TRes> {
    const text = await res.text();
    const parsed: unknown = text ? safeParseJSON(text) : null;
    if (!res.ok) {
      const env = extractError(parsed);
      throw new ApiError(res.status, env?.message ?? `${label} → ${res.status}`, env);
    }
    return (parsed === null ? (null as TRes) : (fromSnake(parsed) as TRes));
  }
}

// ---- helpers (copied from web/src/lib/api/client.ts) ------------------------

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
  if ('code' in obj && 'message' in obj) return fromSnake(obj) as ErrorEnvelope;
  return null;
}

/** Convert camelCase keys to snake_case for the wire. */
export function toSnake(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(toSnake);
  if (value === null || typeof value !== 'object') return value;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    out[k.replace(/[A-Z]/g, (m) => '_' + m.toLowerCase())] = toSnake(v);
  }
  return out;
}

/** Convert snake_case keys to camelCase coming back from the wire. */
export function fromSnake(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(fromSnake);
  if (value === null || typeof value !== 'object') return value;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    out[k.replace(/_([a-z0-9])/g, (_, c: string) => c.toUpperCase())] = fromSnake(v);
  }
  return out;
}
