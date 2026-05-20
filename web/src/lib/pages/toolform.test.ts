import { describe, it, expect } from 'vitest';
import {
  imageSummary,
  imageFormReady,
  pdfFormReady,
  pdfSummary,
  archivePackReady,
  archivePackSummary,
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
});

describe('archiveExtract', () => {
  it('needs both a source and a destination', () => {
    expect(archiveExtractReady('', '/out')).toBe(false);
    expect(archiveExtractReady('a.zip', '  ')).toBe(false);
    expect(archiveExtractReady('a.zip', '/out')).toBe(true);
  });

  it('summarises the disabled and ready states', () => {
    expect(archiveExtractSummary('', '/out')).toBe('choose an archive to extract');
    expect(archiveExtractSummary('a.zip', '')).toBe('choose a destination folder');
    expect(archiveExtractSummary('a.zip', '/out')).toBe('ready: extract a.zip → /out');
  });
});
