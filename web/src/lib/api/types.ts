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
 * /v1/health response. Not yet on the wire (#64); the field set is the agreed
 * shape so the client and Header badge can both compile against it now.
 */
export interface HealthResponse {
  status: 'ok' | 'serving' | 'degraded' | 'offline';
  version: string;
  uptimeSeconds: number;
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
