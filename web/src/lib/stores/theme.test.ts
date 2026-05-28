// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';

import { mascotCharacter, mascotEnabled } from './theme';

beforeEach(() => {
  localStorage.clear();
  mascotCharacter.set('wrenly');
  mascotEnabled.set(true);
});

describe('mascot preferences', () => {
  it('cycles mascotCharacter between wrenly and hopper', () => {
    mascotCharacter.set('hopper');
    expect(get(mascotCharacter)).toBe('hopper');
    expect(localStorage.getItem('handy.mascot.character')).toBe('hopper');
  });

  it('persists mascotEnabled to localStorage', () => {
    mascotEnabled.set(false);
    expect(get(mascotEnabled)).toBe(false);
    expect(localStorage.getItem('handy.mascot.enabled')).toBe('false');
  });
});
