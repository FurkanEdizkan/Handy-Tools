import { describe, it, expect } from 'vitest';
import {
  imageSummary,
  imageFormReady,
  pdfFormReady,
  pdfSummary,
  archivePackReady,
  archivePackSummary,
  archiveWireFormat,
  archiveFormatFromName,
  withArchiveExt,
  ARCHIVE_FORMATS,
  archiveExtractReady,
  archiveExtractSummary,
  type ImageFile,
} from './toolform';

const img = (name: string, target: ImageFile['target']): ImageFile => ({ name, target });

describe('imageSummary', () => {
  it('prompts when there are no files', () => {
    expect(imageSummary([])).toBe('add at least one image to convert');
  });

  it('mirrors the TUI summary line, grouped by target format', () => {
    const files = [
      img('a.png', 'JPEG'),
      img('b.png', 'JPEG'),
      img('c.png', 'JPEG'),
      img('d.png', 'WebP'),
    ];
    expect(imageSummary(files)).toBe('ready: 4 inputs · 3 → JPEG · 1 → WebP');
  });

  it('uses the singular noun for one file', () => {
    expect(imageSummary([img('only.png', 'PNG')])).toBe('ready: 1 input · 1 → PNG');
  });
});

describe('imageFormReady', () => {
  it('is false with no files, true with at least one', () => {
    expect(imageFormReady([])).toBe(false);
    expect(imageFormReady([img('x.png', 'JPEG')])).toBe(true);
  });
});

describe('pdfFormReady', () => {
  it('requires two documents for merge, one for everything else', () => {
    expect(pdfFormReady('merge', 1)).toBe(false);
    expect(pdfFormReady('merge', 2)).toBe(true);
    expect(pdfFormReady('split', 1)).toBe(true);
    expect(pdfFormReady('render', 0)).toBe(false);
    expect(pdfFormReady('text', 1)).toBe(true);
  });
});

describe('pdfSummary', () => {
  it('explains the disabled state per operation', () => {
    expect(pdfSummary('merge', 1)).toBe('merge needs at least 2 PDFs');
    expect(pdfSummary('split', 0)).toBe('add at least one PDF');
  });

  it('summarises a ready form', () => {
    expect(pdfSummary('merge', 3)).toBe('ready: merge · 3 documents');
    expect(pdfSummary('text', 1)).toBe('ready: extract text · 1 document');
  });
});

describe('archivePack', () => {
  it('needs at least one file and a non-blank output name', () => {
    expect(archivePackReady(0, 'out.zip')).toBe(false);
    expect(archivePackReady(2, '   ')).toBe(false);
    expect(archivePackReady(2, 'out.zip')).toBe(true);
  });

  it('summarises the disabled and ready states', () => {
    expect(archivePackSummary(0, 'zip', 'out.zip')).toBe('add files to pack');
    expect(archivePackSummary(2, 'zip', '')).toBe('name the output archive');
    expect(archivePackSummary(1, 'tar.gz', 'a.tgz')).toBe('ready: 1 item → a.tgz (tar.gz)');
    expect(archivePackSummary(3, 'zip', 'a.zip')).toBe('ready: 3 items → a.zip (zip)');
  });

  it('maps every selectable format onto the server\'s wire enum', () => {
    // Must match parseArchiveFormat in internal/api/http/handlers_archive.go.
    expect(ARCHIVE_FORMATS.map(archiveWireFormat)).toEqual([
      'ZIP',
      'TAR_GZ',
      'TAR_BZ2',
      'TAR_ZST',
      'TAR_XZ',
      'SEVENZ',
    ]);
  });

  it('swaps the output extension when the format changes, incl. tar.xz', () => {
    let out = 'archive.zip';
    out = withArchiveExt(out, 'tar.xz');
    expect(out).toBe('archive.tar.xz');
    out = withArchiveExt(out, 'zip');
    expect(out).toBe('archive.zip');
    out = withArchiveExt(out, 'tar.gz');
    expect(out).toBe('archive.tar.gz');
    expect(withArchiveExt('backup.tgz', '7z')).toBe('backup.7z');
    expect(withArchiveExt('backup.TXZ', 'zip')).toBe('backup.zip');
    expect(withArchiveExt('backup.tar', 'tar.zst')).toBe('backup.tar.zst');
    expect(withArchiveExt('my.photos', 'zip')).toBe('my.photos.zip');
    expect(withArchiveExt('.zip', 'tar.gz')).toBe('archive.tar.gz');
  });

  it('reads the selectable format back from a typed output name', () => {
    // The selector follows a typed extension so the explicitly-sent format
    // never contradicts the filename.
    expect(archiveFormatFromName('photos.tar.gz')).toBe('tar.gz');
    expect(archiveFormatFromName('photos.TGZ')).toBe('tar.gz');
    expect(archiveFormatFromName('photos.txz')).toBe('tar.xz');
    expect(archiveFormatFromName('photos.tar.xz')).toBe('tar.xz');
    expect(archiveFormatFromName('photos.7z')).toBe('7z');
    expect(archiveFormatFromName('photos.zip')).toBe('zip');
    expect(archiveFormatFromName('photos')).toBeNull();
    expect(archiveFormatFromName('photos.tar')).toBeNull();
  });
});

describe('archiveExtract', () => {
  it('needs at least one source, and a destination in "into" mode', () => {
    expect(archiveExtractReady(0, 'into', '/out')).toBe(false);
    expect(archiveExtractReady(1, 'into', '  ')).toBe(false);
    expect(archiveExtractReady(1, 'into', '/out')).toBe(true);
  });

  it('ignores the destination input in "alongside" mode', () => {
    expect(archiveExtractReady(0, 'alongside', '')).toBe(false);
    expect(archiveExtractReady(1, 'alongside', '')).toBe(true);
    expect(archiveExtractReady(3, 'alongside', 'ignored')).toBe(true);
  });

  it('summarises the disabled and ready states', () => {
    expect(archiveExtractSummary(0, 'into', '/out')).toBe('choose one or more archives to extract');
    expect(archiveExtractSummary(2, 'into', '')).toBe('choose a destination folder');
    expect(archiveExtractSummary(1, 'into', '/out')).toBe('ready: extract 1 archive → /out');
    expect(archiveExtractSummary(3, 'into', '/out')).toBe('ready: extract 3 archives → /out');
    expect(archiveExtractSummary(1, 'alongside', '')).toBe(
      'ready: extract 1 archive alongside each source',
    );
  });
});
