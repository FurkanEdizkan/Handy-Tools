/**
 * Pure form-state helpers shared by the per-tool pages (ToolImage, ToolPdf,
 * and later the archive forms). Kept dependency-free so the validity and
 * summary-line rules can be unit-tested without a component harness.
 */

export const IMAGE_FORMATS = ['JPEG', 'PNG', 'WebP', 'GIF', 'BMP', 'TIFF'] as const;
export type ImageFormat = (typeof IMAGE_FORMATS)[number];

/** One image queued for conversion, with its chosen target format. */
export interface ImageFile {
  name: string;
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

export const ARCHIVE_FORMATS = ['zip', 'tar.gz', 'tar.bz2', 'tar.zst', '7z'] as const;
export type ArchiveFormat = (typeof ARCHIVE_FORMATS)[number];

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

/** archiveExtractReady enables ToolArchiveExtract's Run. */
export function archiveExtractReady(source: string, destination: string): boolean {
  return source.trim() !== '' && destination.trim() !== '';
}

/** archiveExtractSummary is ToolArchiveExtract's summary line. */
export function archiveExtractSummary(source: string, destination: string): string {
  if (source.trim() === '') return 'choose an archive to extract';
  if (destination.trim() === '') return 'choose a destination folder';
  return `ready: extract ${source.trim()} → ${destination.trim()}`;
}
