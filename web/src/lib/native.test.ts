// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { isDesktop, pickFiles, pickFolder, reveal } from './native';

afterEach(() => {
  delete (window as unknown as { go?: unknown }).go;
});

/** installApp stubs the Wails-injected window.go.main.App. */
function installApp(app: Record<string, unknown>): void {
  (window as unknown as { go: unknown }).go = { main: { App: app } };
}

describe('native — plain browser (no Wails runtime)', () => {
  it('isDesktop is false and the pickers resolve empty', async () => {
    expect(isDesktop()).toBe(false);
    expect(await pickFiles()).toEqual([]);
    expect(await pickFolder()).toBe('');
    await expect(reveal('/x')).resolves.toBeUndefined();
  });
});

describe('native — desktop (Wails runtime present)', () => {
  it('isDesktop is true and the pickers call through to the bindings', async () => {
    installApp({
      PickFiles: vi.fn(async () => ['/abs/a.png', '/abs/b.png']),
      PickFolder: vi.fn(async () => '/abs/out'),
      Reveal: vi.fn(async () => undefined),
    });
    expect(isDesktop()).toBe(true);
    expect(await pickFiles()).toEqual(['/abs/a.png', '/abs/b.png']);
    expect(await pickFolder()).toBe('/abs/out');
    await expect(reveal('/abs/a.png')).resolves.toBeUndefined();
  });

  it('a rejected binding degrades to the empty result', async () => {
    installApp({
      PickFiles: vi.fn(async () => {
        throw new Error('dialog failed');
      }),
      PickFolder: vi.fn(async () => {
        throw new Error('dialog failed');
      }),
      Reveal: vi.fn(async () => undefined),
    });
    expect(await pickFiles()).toEqual([]);
    expect(await pickFolder()).toBe('');
  });
});
