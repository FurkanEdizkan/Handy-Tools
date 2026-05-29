/**
 * TypeScript types mirroring the JSON shapes defined in
 * internal/api/http/types.go. Names use camelCase here but the wire format
 * stays snake_case — the client layer is responsible for the mapping at the
 * boundary (see client.ts).
 *
 * Keep this file hand-mirrored for now; a proto→TS generator is a possible
 * later optimization.
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
  /** Backend normalizes nil → [], but treat as possibly-null for safety
   *  against older binaries or other endpoints that haven't been normalized. */
  detectedParts: string[] | null;
  missingParts: string[] | null;
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

/** POST /v1/pdf/split — page_ranges and every_n are mutually exclusive. */
export interface PdfSplitRequest {
  source: FileRef;
  pageRanges: PageRange[];
  everyN: number;
  output: OutputRef;
}

/** POST /v1/archive/compress. */
export interface CompressRequest {
  sources: FileRef[];
  destination: OutputRef;
  /** ZIP|TAR|TAR_GZ|TAR_BZ2|TAR_ZST|SEVENZ; "" infers from the file extension. */
  format: string;
  compressionLevel: number;
  password?: string;
}

export type HashAlgo = 'md5' | 'sha256' | 'blake3';

/** POST /v1/hash. */
export interface HashRunRequest {
  sources: FileRef[];
  algo: HashAlgo;
}

/** POST /v1/hash/verify. */
export interface HashVerifyRequest {
  manifest: FileRef;
  algo: HashAlgo;
}

export interface HashVerifyEntry {
  path: string;
  expected: string;
  got?: string;
  ok: boolean;
  err?: string;
}

export interface HashVerifyResponse {
  entries: HashVerifyEntry[];
  ok: number;
  failed: number;
  missing: number;
}

export type DiffTreeMode = 'mtime' | 'hash';

/** POST /v1/diff-tree/inspect. */
export interface DiffTreeInspectRequest {
  a: FileRef;
  b: FileRef;
  mode: DiffTreeMode;
}

export type DiffStatus = 'added' | 'removed' | 'changed';

export interface DiffEntry {
  path: string;
  status: DiffStatus;
  reason?: string;
}

export interface DiffTreeInspectResponse {
  entries: DiffEntry[];
}

/** POST /v1/diff-tree/file — unified diff between two single files. */
export interface DiffTreeFileRequest {
  a: FileRef;
  b: FileRef;
}

export type DiffLineKind = 'context' | 'add' | 'remove' | 'hunk';

export interface DiffLine {
  kind: DiffLineKind;
  text: string;
  aOld?: number;
  bNew?: number;
}

export interface DiffTreeFileResponse {
  binary: boolean;
  truncated: boolean;
  identical: boolean;
  lines: DiffLine[];
}

export type RenameCollision = 'error' | 'skip' | 'suffix';

/** POST /v1/rename/inspect and POST /v1/rename/run. */
export interface RenameRequest {
  sources: FileRef[];
  pattern: string;
  replace: string;
  onCollision?: RenameCollision;
}

export interface RenamePlan {
  from: string;
  to: string;
}

export interface RenameInspectResponse {
  plans: RenamePlan[];
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
  /** True when fraction is a time/size ETA estimate, not a measured byte count. */
  estimated?: boolean;
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
  /** Short git commit hash (7 chars, may have "-dirty" suffix). Empty in
   *  release-cut builds with no VCS metadata baked in. */
  commit?: string;
  /** RFC3339 build timestamp. Empty when no VCS metadata is available. */
  buildDate?: string;
  uptimeSeconds: number;
  transports: string[];
  toolsAvailable: string[];
}

/**
 * GET /v1/config response. Optional fields cover the forward-compat case
 * where the server omits keys we don't care about yet.
 */
export interface ConfigResponse {
  allowRoots?: string[];
  defaultJpegQuality?: number;
  defaultPdfDpi?: number;
}

export type ConfigPatch = Partial<ConfigResponse>;
