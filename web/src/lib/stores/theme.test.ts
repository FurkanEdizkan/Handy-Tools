// @vitest-environment jsdom
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';

// Mock the API client so setTheme's backend sync is observable without a
// server. vi.hoisted keeps `patchConfig` initialised before the hoisted
// vi.mock factory runs.
const { patchConfig } = vi.hoisted(() => ({
  patchConfig: vi.fn().mockResolvedValue({}),
}));
vi.mock('../api', () => ({ api: { patchConfig } }));

import { currentTheme, setTheme, cycleTheme, hydrateFromConfig } from './theme';

beforeEach(() => {
  patchConfig.mockClear();
  localStorage.clear();
  currentTheme.set('forge');
});

describe('setTheme', () => {
  it('updates the live store', () => {
    setTheme('snow');
    expect(get(currentTheme)).toBe('snow');
  });

  it('persists the choice to the backend config', () => {
    setTheme('ember');
    expect(patchConfig).toHaveBeenCalledWith({ theme: 'ember' });
  });
});

describe('cycleTheme', () => {
  it('advances forge -> snow -> ember -> forge', () => {
    currentTheme.set('forge');
    cycleTheme();
    expect(get(currentTheme)).toBe('snow');
    cycleTheme();
    expect(get(currentTheme)).toBe('ember');
    cycleTheme();
    expect(get(currentTheme)).toBe('forge');
  });
});

describe('hydrateFromConfig', () => {
  it('applies the server theme when nothing is stored on this device', async () => {
    currentTheme.set('forge');
    localStorage.clear(); // ensure no stored choice — the subscriber writes one
    const fetchFn = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ theme: 'ember' }),
    });
    await hydrateFromConfig(fetchFn as unknown as typeof fetch);
    expect(get(currentTheme)).toBe('ember');
  });

  it('keeps a locally-chosen theme over the server value', async () => {
    setTheme('snow'); // writes localStorage via the subscriber
    const fetchFn = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ theme: 'ember' }),
    });
    await hydrateFromConfig(fetchFn as unknown as typeof fetch);
    expect(get(currentTheme)).toBe('snow');
    expect(fetchFn).not.toHaveBeenCalled();
  });
});
