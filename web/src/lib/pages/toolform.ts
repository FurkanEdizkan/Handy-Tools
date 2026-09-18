/**
 * Pure form-state helpers shared by the per-tool pages (ToolImage, ToolPdf,
 * and later the archive forms). Kept dependency-free so the validity and
 * summary-line rules can be unit-tested without a component harness.
 */

import type { ArchiveWireFormat } from '../api/types';

export const IMAGE_FORMATS = ['JPEG', 'PNG', 'WebP', 'GIF', 'BMP', 'TIFF'] as const;
export type ImageFormat = (typeof IMAGE_FORMATS)[number];

/**
 * A file the user picked for a tool. The native picker supplies a real
 * absolute `path`; a drag-and-drop supplies the `File` object itself. `name`
 * is always the display label.
 */
export interface PickedFile {
  name: string;
  /** Drag-and-drop — the dropped File blob. */
  file?: File;
  /** Native picker — the absolute path. */
  path?: string;
}

/** One image queued for conversion, with its chosen target format. */
export interface ImageFile extends PickedFile {
  target: ImageFormat;
}

/**
 * imageSummary mirrors the TUI's summary line, e.g.
 * `ready: 4 inputs · 3 → JPEG · 1 → WebP`. With no files it returns the
 * prompt that explains why the Run button is disabled.
 */
export function imageSummary(files: ImageFile[]): string {
  if (files.length === 0) return 'add at least one image to convert';
  const counts = new Map<ImageFormat, number>();
  for (const f of files) counts.set(f.target, (counts.get(f.target) ?? 0) + 1);
  // Preserve IMAGE_FORMATS order so the summary is stable.
  const parts = IMAGE_FORMATS.filter((fmt) => counts.has(fmt)).map(
    (fmt) => `${counts.get(fmt)} → ${fmt}`,
  );
  const noun = files.length === 1 ? 'input' : 'inputs';
  return `ready: ${files.length} ${noun} · ${parts.join(' · ')}`;
}

/** imageFormReady reports whether ToolImage's Run button should be enabled. */
export function imageFormReady(files: ImageFile[]): boolean {
  return files.length > 0;
}

export type PdfOp = 'merge' | 'split' | 'render' | 'text';

export const PDF_OPS: { value: PdfOp; label: string }[] = [
  { value: 'merge', label: 'Merge' },
  { value: 'split', label: 'Split' },
  { value: 'render', label: 'Pages → image' },
  { value: 'text', label: 'Extract text' },
];

/**
 * pdfFormReady reports whether ToolPdf's Run button should be enabled. Merge
 * needs at least two documents; every other operation needs at least one.
 */
export function pdfFormReady(op: PdfOp, fileCount: number): boolean {
  return op === 'merge' ? fileCount >= 2 : fileCount >= 1;
}

/** pdfSummary is ToolPdf's summary line, mirroring imageSummary's shape. */
export function pdfSummary(op: PdfOp, fileCount: number): string {
  if (!pdfFormReady(op, fileCount)) {
    return op === 'merge'
      ? 'merge needs at least 2 PDFs'
      : 'add at least one PDF';
  }
  const label = PDF_OPS.find((o) => o.value === op)?.label ?? op;
  const noun = fileCount === 1 ? 'document' : 'documents';
  return `ready: ${label.toLowerCase()} · ${fileCount} ${noun}`;
}

export const ARCHIVE_FORMATS = ['zip', 'tar.gz', 'tar.bz2', 'tar.zst', 'tar.xz', '7z'] as const;
export type ArchiveFormat = (typeof ARCHIVE_FORMATS)[number];

/** Wire enum for POST /v1/archive/compress (see parseArchiveFormat server-side). */
const ARCHIVE_WIRE_FORMATS: Record<ArchiveFormat, ArchiveWireFormat> = {
  zip: 'ZIP',
  'tar.gz': 'TAR_GZ',
  'tar.bz2': 'TAR_BZ2',
  'tar.zst': 'TAR_ZST',
  'tar.xz': 'TAR_XZ',
  '7z': 'SEVENZ',
};

/** archiveWireFormat maps a selectable format onto the compress request enum. */
export function archiveWireFormat(fmt: ArchiveFormat): ArchiveWireFormat {
  return ARCHIVE_WIRE_FORMATS[fmt];
}

/** Every archive extension the backend's detectFormat recognises (compound first). */
const ARCHIVE_EXT_RE = /\.(tar\.gz|tgz|tar\.bz2|tbz2|tar\.zst|tzst|tar\.xz|txz|tar|zip|7z|rar)$/i;

/**
 * withArchiveExt swaps an output filename's archive extension for `fmt`'s,
 * e.g. `archive.tar.xz` → `archive.zip`. Names without a known archive
 * extension keep their full text as the stem.
 */
export function withArchiveExt(output: string, fmt: ArchiveFormat): string {
  const stem = output.trim().replace(ARCHIVE_EXT_RE, '') || 'archive';
  return `${stem}.${fmt}`;
}

const EXT_TO_FORMAT: Record<string, ArchiveFormat> = {
  zip: 'zip',
  'tar.gz': 'tar.gz',
  tgz: 'tar.gz',
  'tar.bz2': 'tar.bz2',
  tbz2: 'tar.bz2',
  'tar.zst': 'tar.zst',
  tzst: 'tar.zst',
  'tar.xz': 'tar.xz',
  txz: 'tar.xz',
  '7z': '7z',
};

/**
 * archiveFormatFromName returns the selectable format a typed output name's
 * extension implies (e.g. `a.tgz` → `tar.gz`), or null when it has none.
 */
export function archiveFormatFromName(output: string): ArchiveFormat | null {
  const ext = ARCHIVE_EXT_RE.exec(output.trim())?.[1]?.toLowerCase();
  return (ext && EXT_TO_FORMAT[ext]) || null;
}

/** archivePackReady enables ToolArchivePack's Run: needs files and a name. */
export function archivePackReady(fileCount: number, output: string): boolean {
  return fileCount > 0 && output.trim() !== '';
}

/** archivePackSummary is ToolArchivePack's summary line. */
export function archivePackSummary(
  fileCount: number,
  format: ArchiveFormat,
  output: string,
): string {
  if (fileCount === 0) return 'add files to pack';
  if (output.trim() === '') return 'name the output archive';
  const noun = fileCount === 1 ? 'item' : 'items';
  return `ready: ${fileCount} ${noun} → ${output.trim()} (${format})`;
}

/** Destination mode for ToolArchiveExtract — kept here so toolform stays
 *  the single source of truth for the page's form-state shape. */
export type ExtractDestMode = 'into' | 'alongside';

/**
 * archiveExtractReady enables ToolArchiveExtract's Run. Needs at least one
 * picked archive; in `into` mode also needs a destination path. In
 * `alongside` mode the destination is derived from each archive's parent
 * folder, so the text input is irrelevant.
 */
export function archiveExtractReady(
  sourceCount: number,
  mode: ExtractDestMode,
  destination: string,
): boolean {
  if (sourceCount <= 0) return false;
  if (mode === 'alongside') return true;
  return destination.trim() !== '';
}

/** archiveExtractSummary is ToolArchiveExtract's summary line. */
export function archiveExtractSummary(
  sourceCount: number,
  mode: ExtractDestMode,
  destination: string,
): string {
  if (sourceCount <= 0) return 'choose one or more archives to extract';
  if (mode === 'into' && destination.trim() === '') return 'choose a destination folder';
  const noun = sourceCount === 1 ? 'archive' : 'archives';
  if (mode === 'alongside') return `ready: extract ${sourceCount} ${noun} alongside each source`;
  return `ready: extract ${sourceCount} ${noun} → ${destination.trim()}`;
}
