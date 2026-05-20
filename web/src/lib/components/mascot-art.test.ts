import { describe, expect, it } from 'vitest';
import {
  MASCOT_STATES,
  eyeGlyph,
  furVar,
  overlayFrame,
  spriteGrid,
  statusLabel,
} from './mascot-art';

describe('spriteGrid', () => {
  it('parses Wrenly as a 14×15 token grid', () => {
    const grid = spriteGrid('wrenly');
    expect(grid).toHaveLength(14);
    expect(grid.every((row) => row.length === 15)).toBe(true);
  });

  it('parses Hopper as a 15×15 token grid', () => {
    const grid = spriteGrid('hopper');
    expect(grid).toHaveLength(15);
    expect(grid.every((row) => row.length === 15)).toBe(true);
  });

  it('places exactly two eye cells on each character', () => {
    for (const character of ['wrenly', 'hopper'] as const) {
      const eyes = spriteGrid(character)
        .flat()
        .filter((t) => t === 'eye');
      expect(eyes).toHaveLength(2);
    }
  });

  it('maps unknown glyphs to empty', () => {
    expect(spriteGrid('wrenly')[0][0]).toBe('empty');
  });
});

describe('eyeGlyph', () => {
  it('blinks idle on every 7th frame', () => {
    expect(eyeGlyph('idle', 0)).toBe('•');
    expect(eyeGlyph('idle', 6)).toBe('–');
  });

  it('quivers the stressed eye per frame', () => {
    expect(eyeGlyph('stressed', 0)).not.toBe(eyeGlyph('stressed', 1));
  });

  it('uses a fixed glyph for happy and worried', () => {
    expect(eyeGlyph('happy', 3)).toBe('‿');
    expect(eyeGlyph('worried', 9)).toBe('⌒');
  });
});

describe('statusLabel', () => {
  it('renders tired as FRUSTRATED (artist choice, mirrors the TUI)', () => {
    expect(statusLabel('tired')).toBe('FRUSTRATED');
  });

  it('covers every state with a non-empty uppercase tag', () => {
    for (const state of MASCOT_STATES) {
      const label = statusLabel(state);
      expect(label).toBe(label.toUpperCase());
      expect(label.length).toBeGreaterThan(0);
    }
  });
});

describe('furVar', () => {
  it('borrows theme hues for mood states', () => {
    expect(furVar('stressed')).toBe('--color-accent');
    expect(furVar('happy')).toBe('--color-success');
    expect(furVar('worried')).toBe('--color-error');
    expect(furVar('tired')).toBe('--color-text-dim');
  });

  it('keeps mascot fur for calm states', () => {
    expect(furVar('idle')).toBe('--color-mascot-fur');
    expect(furVar('thinking')).toBe('--color-mascot-fur');
    expect(furVar('watching')).toBe('--color-mascot-fur');
  });
});

describe('overlayFrame', () => {
  it('has no overlay for idle / watching', () => {
    expect(overlayFrame('idle', 0)).toBeNull();
    expect(overlayFrame('watching', 0)).toBeNull();
  });

  it('animates the thinking overlay across frames', () => {
    const a = overlayFrame('thinking', 0);
    const b = overlayFrame('thinking', 1);
    expect(a?.glyphs).not.toBe(b?.glyphs);
    expect(a?.tone).toBe('accent');
  });

  it('tones the worried overlay as an error', () => {
    expect(overlayFrame('worried', 0)?.tone).toBe('error');
  });
});
