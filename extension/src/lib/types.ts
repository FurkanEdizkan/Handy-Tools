/**
 * Wire shapes for the htoolsd HTTP API, mirrored from
 * web/src/lib/api/types.ts. Names are camelCase here; the client converts to
 * and from the snake_case wire format at the boundary (see api.ts).
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

export type ImageTargetFormat = 'PNG' | 'JPEG' | 'WEBP' | 'GIF' | 'BMP' | 'TIFF';

export interface BatchConvertRequest {
  sources: FileRef[];
  targetFormat: ImageTargetFormat;
  options: ImageOptions;
  output: OutputRef;
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

export interface PdfSplitRequest {
  source: FileRef;
  pageRanges: PageRange[];
  everyN: number;
  output: OutputRef;
}

export interface CompressRequest {
  sources: FileRef[];
  destination: OutputRef;
  /** "" infers from the destination file extension. */
  format: string;
  compressionLevel: number;
  password?: string;
}

export interface ExtractRequest {
  source: FileRef;
  parts: FileRef[];
  destinationDir: string;
  password: string;
  overwrite: boolean;
  autoAcceptMultiPart: boolean;
}

export interface JobResponse {
  jobId: string;
}

export interface ErrorEnvelope {
  code: string;
  message: string;
  detail?: string;
}

export interface ProgressEvent {
  jobId: string;
  fraction?: number;
  message?: string;
  level?: 'INFO' | 'WARNING' | 'ERROR';
  completed: boolean;
  error?: ErrorEnvelope;
}

export interface JobSummary {
  jobId: string;
  status: 'queued' | 'running' | 'done' | 'failed';
  fraction?: number;
  completed: boolean;
  error?: ErrorEnvelope;
}

export interface JobsResponse {
  jobs: JobSummary[];
}

export interface UploadedFile {
  name: string;
  path: string;
}

export interface UploadCreateResponse {
  uploadId: string;
  files: UploadedFile[];
  outputDir: string;
}

export interface HealthResponse {
  version: string;
  uptimeSeconds: number;
  transports: string[];
  toolsAvailable: string[];
}
