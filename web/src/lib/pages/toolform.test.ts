import { describe, it, expect } from 'vitest';
import {
  imageSummary,
  imageFormReady,
  pdfFormReady,
  pdfSummary,
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
