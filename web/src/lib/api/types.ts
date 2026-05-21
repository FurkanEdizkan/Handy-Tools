/**
 * TypeScript types mirroring the JSON shapes defined in
 * internal/api/http/types.go. Names use camelCase here but the wire format
 * stays snake_case — the client layer is responsible for the mapping at the
 * boundary (see client.ts).
 *
 * Keep this file hand-mirrored for now; a proto→TS generator is a later
 * optimization (#89 / Track D).
 */

export interface FileRef {
  path: string;
}

export interface OutputRef {
  file?: string;
  directory?: string;
  overwrite?: boolean;
}

export interface PageRange {
  from: number;
  to: number;
}

export interface ImageOptions {
  quality: number;
  maxWidth: number;
  maxHeight: number;
  stripMetadata: boolean;
}

export type ImageTargetFormat =
  | 'PNG'
  | 'JPEG'
  | 'GIF'
  | 'BMP'
  | 'TIFF'
  | 'WEBP'
  | 'HEIC';

export interface ConvertRequest {
  source: FileRef;
  targetFormat: ImageTargetFormat;
  options: ImageOptions;
  output: OutputRef;
}

export interface InspectRequest {
  source: FileRef;
}

export interface InspectResponse {
  format: string;
  multiPart: boolean;
  detectedParts: string[];
  missingParts: string[];
  uncompressedSizeBytes: number;
  entryCount: number;
  requiresPassword: boolean;
  requiresBinary: string;
  binaryAvailable: boolean;
}

export interface ExtractRequest {
  source: FileRef;
  parts: FileRef[];
  destinationDir: string;
  password: string;
  overwrite: boolean;
  autoAcceptMultiPart: boolean;
}

export interface PdfToImageRequest {
  source: FileRef;
  pages: PageRange;
  dpi: number;
  targetFormat: 'PNG' | 'JPEG';
  output: OutputRef;
}

export interface PdfToTextRequest {
  source: FileRef;
  pages: PageRange;
  layout: boolean;
  output: OutputRef;
}

export interface PdfMergeRequest {
  sources: FileRef[];
  output: OutputRef;
}

export interface JobResponse {
  jobId: string;
}

export type JobStatusWire = 'queued' | 'running' | 'done' | 'failed';

/**
 * One job in GET /v1/jobs and each GET /v1/jobs/events SSE frame. Mirrors the
 * jobSummary shape in internal/api/http/types.go.
 */
export interface JobSummary {
  jobId: string;
  tool: string;
  action: string;
  startedUnixMs?: number;
  status: JobStatusWire;
  fraction?: number;
  currentItem?: string;
  message?: string;
  completed: boolean;
  error?: ErrorEnvelope;
}

/** GET /v1/jobs response. */
export interface JobsResponse {
  jobs: JobSummary[];
}

export type ProgressLevel = 'INFO' | 'WARNING' | 'ERROR';

export interface ProgressEvent {
  jobId: string;
  tool: string;
  action: string;
  startedUnixMs?: number;
  currentItem?: string;
  bytesDone?: number;
  bytesTotal?: number;
  fraction?: number;
  level?: ProgressLevel;
  message?: string;
  completed: boolean;
  error?: ErrorEnvelope;
}

export interface SysdepResult {
  name: string;
  found: boolean;
  path?: string;
  usedAlias?: string;
  description?: string;
  features?: string[];
  installHint?: Record<string, string>;
}

export interface ErrorEnvelope {
  code: string;
  message: string;
  detail?: string;
}

/**
 * GET /v1/health response (#64). The endpoint has no status field — a
 * successful HTTP response *is* the liveness signal; reachability decides the
 * badge level.
 */
export interface HealthResponse {
  version: string;
  uptimeSeconds: number;
  transports: string[];
  toolsAvailable: string[];
}

/**
 * /v1/config response. Not yet on the wire (#65). Optional fields cover the
 * forward-compat case where the server omits keys we don't care about yet.
 */
export interface ConfigResponse {
  theme?: 'forge' | 'snow' | 'ember';
  allowRoots?: string[];
  defaultJpegQuality?: number;
  defaultPdfDpi?: number;
}

export type ConfigPatch = Partial<ConfigResponse>;
