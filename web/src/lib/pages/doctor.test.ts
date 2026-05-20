import { describe, it, expect } from 'vitest';
import type { SysdepResult } from '../api';
import { sysdepSummary } from './doctor';

const dep = (name: string, found: boolean): SysdepResult => ({ name, found });

describe('sysdepSummary', () => {
  it('counts found tools out of the total', () => {
    expect(
      sysdepSummary([
        dep('unrar', true),
        dep('7z', true),
        dep('pdftoppm', false),
        dep('pdftotext', true),
        dep('magick', false),
      ]),
    ).toBe('3 / 5 found');
  });

  it('handles an all-found and an empty list', () => {
    expect(sysdepSummary([dep('unrar', true)])).toBe('1 / 1 found');
    expect(sysdepSummary([])).toBe('0 / 0 found');
  });
});
