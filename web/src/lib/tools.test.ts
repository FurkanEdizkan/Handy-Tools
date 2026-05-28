import { describe, it, expect } from 'vitest';
import { defaultTools, toolById } from './tools';

describe('defaultTools', () => {
  it('lists exactly the catalog tools', () => {
    expect(defaultTools.map((t) => t.id)).toEqual([
      'convert-image',
      'zip-pack',
      'archive-extract',
      'pdf',
      'diff-tree',
      'rename',
      'doctor',
    ]);
  });

  it('gives every tool a non-empty glyph, label and description', () => {
    for (const t of defaultTools) {
      expect(t.glyph, `${t.id} glyph`).not.toBe('');
      expect(t.label, `${t.id} label`).not.toBe('');
      expect(t.desc, `${t.id} desc`).not.toBe('');
    }
  });
});

describe('toolById', () => {
  it('resolves every known id', () => {
    for (const t of defaultTools) {
      expect(toolById(t.id)).toEqual(t);
    }
  });

  it('returns undefined for an unknown id', () => {
    expect(toolById('nope')).toBeUndefined();
    expect(toolById('')).toBeUndefined();
  });
});
