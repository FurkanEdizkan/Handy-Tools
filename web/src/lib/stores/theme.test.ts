// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';

import { currentDensity, setDensity, mascotCharacter, mascotEnabled } from './theme';

beforeEach(() => {
  localStorage.clear();
  currentDensity.set('regular');
  mascotCharacter.set('wrenly');
  mascotEnabled.set(true);
});

describe('setDensity', () => {
  it('updates the live store and writes the data-density attribute', () => {
    setDensity('compact');
    expect(get(currentDensity)).toBe('compact');
    expect(document.documentElement.dataset.density).toBe('compact');
  });

  it('persists the choice to localStorage', () => {
    setDensity('comfy');
    expect(localStorage.getItem('handy.density')).toBe('comfy');
  });
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
