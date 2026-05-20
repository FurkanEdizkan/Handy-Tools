import { describe, expect, it } from 'vitest';
import { clampFraction, percentLabel, progressBlocks } from './progress';

describe('clampFraction', () => {
  it('passes through values already in range', () => {
    expect(clampFraction(0)).toBe(0);
    expect(clampFraction(0.5)).toBe(0.5);
    expect(clampFraction(1)).toBe(1);
  });

  it('clamps out-of-range values', () => {
    expect(clampFraction(-0.3)).toBe(0);
    expect(clampFraction(1.7)).toBe(1);
  });

  it('collapses NaN to 0', () => {
    expect(clampFraction(NaN)).toBe(0);
  });
});

describe('progressBlocks', () => {
  it('renders an empty bar at 0', () => {
    expect(progressBlocks(0, 8)).toBe('▱▱▱▱▱▱▱▱');
  });

  it('renders a full bar at 1', () => {
    expect(progressBlocks(1, 8)).toBe('▰▰▰▰▰▰▰▰');
  });

  it('rounds the fill to the nearest cell and keeps total width', () => {
    const bar = progressBlocks(0.5, 8);
    expect(bar).toBe('▰▰▰▰▱▱▱▱');
    expect(bar).toHaveLength(8);
  });

  it('clamps over-range fractions instead of overflowing', () => {
    expect(progressBlocks(2, 4)).toBe('▰▰▰▰');
    expect(progressBlocks(-1, 4)).toBe('▱▱▱▱');
  });
});

describe('percentLabel', () => {
  it('rounds to a whole percent', () => {
    expect(percentLabel(0.624)).toBe('62%');
    expect(percentLabel(0)).toBe('0%');
    expect(percentLabel(1)).toBe('100%');
  });
});
