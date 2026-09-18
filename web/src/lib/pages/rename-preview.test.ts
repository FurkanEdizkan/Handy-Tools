import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type RenameInspectResponse, type RenameRequest } from '../api';
import {
  PREVIEW_DEBOUNCE_MS,
  createRenamePreview,
  planChanges,
  renamePreviewKey,
  type RenamePreview,
} from './rename-preview';
import type { PickedFile } from './toolform';

const files: PickedFile[] = [
  { name: 'notes.txt', path: '/home/u/notes.txt' },
  { name: 'IMG_001.jpg', path: '/home/u/IMG_001.jpg' },
];

/** Mirrors the server: From == To for rows whose basename does not change. */
function plansFor(req: RenameRequest, rename: (base: string) => string): RenameInspectResponse {
  return {
    plans: req.sources.map((s) => {
      const i = s.path.lastIndexOf('/');
      return { from: s.path, to: s.path.slice(0, i + 1) + rename(s.path.slice(i + 1)) };
    }),
  };
}

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('planChanges', () => {
  it('compares basenames, so unmatched natively-picked files are unchanged', () => {
    expect(
      planChanges({ from: '/home/u/notes.txt', to: '/home/u/notes.txt' }),
    ).toBe(false);
    expect(
      planChanges({ from: '/home/u/IMG_001.jpg', to: '/home/u/001_old.jpg' }),
    ).toBe(true);
  });
});

describe('createRenamePreview', () => {
  it('debounces keystrokes into one server dry-run and reports its plans', async () => {
    const inspect = vi.fn(async (req: RenameRequest) =>
      plansFor(req, (b) => (b.startsWith('IMG_') ? '001_old.jpg' : b)),
    );
    const updates: RenamePreview[] = [];
    const p = createRenamePreview(inspect, (u) => updates.push(u));

    p.request(files, '^IMG', 'x');
    p.request(files, '^IMG_', 'x');
    p.request(files, '^IMG_(\\d+)', '${1}_old');
    expect(inspect).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(PREVIEW_DEBOUNCE_MS);
    expect(inspect).toHaveBeenCalledOnce();
    expect(inspect.mock.calls[0][0]).toEqual({
      sources: [{ path: '/home/u/notes.txt' }, { path: '/home/u/IMG_001.jpg' }],
      pattern: '^IMG_(\\d+)',
      replace: '${1}_old',
    });

    const last = updates.at(-1)!;
    expect(last.key).toBe(renamePreviewKey(files, '^IMG_(\\d+)', '${1}_old'));
    expect(last.plans?.map(planChanges)).toEqual([false, true]);
    expect(last.error).toBe('');
  });

  it('reports no changes when the pattern matches nothing', async () => {
    const inspect = vi.fn(async (req: RenameRequest) => plansFor(req, (b) => b));
    const updates: RenamePreview[] = [];
    createRenamePreview(inspect, (u) => updates.push(u)).request(files, 'zzz', 'y');
    await vi.advanceTimersByTimeAsync(PREVIEW_DEBOUNCE_MS);
    expect(updates.at(-1)!.plans!.some(planChanges)).toBe(false);
  });

  it('ignores a stale response that resolves after a newer request', async () => {
    let resolveSlow: (r: RenameInspectResponse) => void = () => {};
    const inspect = vi
      .fn<(req: RenameRequest) => Promise<RenameInspectResponse>>()
      .mockImplementationOnce(
        (req) =>
          new Promise((r) => {
            resolveSlow = () => r(plansFor(req, () => 'STALE.txt'));
          }),
      )
      .mockImplementationOnce(async (req) => plansFor(req, (b) => `new-${b}`));
    const updates: RenamePreview[] = [];
    const p = createRenamePreview(inspect, (u) => updates.push(u));

    p.request(files, 'a', 'b');
    await vi.advanceTimersByTimeAsync(PREVIEW_DEBOUNCE_MS); // slow request in flight
    p.request(files, 'c', 'd');
    await vi.advanceTimersByTimeAsync(PREVIEW_DEBOUNCE_MS); // fast request done
    resolveSlow({ plans: [] });
    await vi.runAllTimersAsync();

    const last = updates.at(-1)!;
    expect(last.key).toBe(renamePreviewKey(files, 'c', 'd'));
    expect(last.plans?.[0].to).toBe('/home/u/new-notes.txt');
    expect(updates.some((u) => u.plans?.[0]?.to.includes('STALE'))).toBe(false);
  });

  it('surfaces the server error (e.g. an RE2-invalid pattern) instead of plans', async () => {
    const inspect = vi.fn(async () => {
      throw new ApiError(400, 'invalid regex pattern', {
        code: 'BAD_REQUEST',
        message: 'invalid regex pattern',
        detail: 'invalid or unsupported Perl syntax: `(?<`',
      });
    });
    const updates: RenamePreview[] = [];
    createRenamePreview(inspect, (u) => updates.push(u)).request(files, '(?<=a)b', '');
    await vi.advanceTimersByTimeAsync(PREVIEW_DEBOUNCE_MS);
    expect(updates.at(-1)).toMatchObject({ plans: null });
    expect(updates.at(-1)!.error).toMatch(/invalid regex pattern.*Perl syntax/);
  });

  it('does not call the server for dropped files without a path', async () => {
    const inspect = vi.fn(async (req: RenameRequest) => plansFor(req, (b) => b));
    const updates: RenamePreview[] = [];
    createRenamePreview(inspect, (u) => updates.push(u)).request(
      [{ name: 'dropped.txt', file: new File(['x'], 'dropped.txt') }],
      'd',
      'D',
    );
    await vi.advanceTimersByTimeAsync(PREVIEW_DEBOUNCE_MS);
    expect(inspect).not.toHaveBeenCalled();
    expect(updates.at(-1)!.error).toMatch(/no filesystem path/);
  });

  it('clears the preview for an empty pattern and stops after dispose', async () => {
    const inspect = vi.fn(async (req: RenameRequest) => plansFor(req, (b) => b));
    const updates: RenamePreview[] = [];
    const p = createRenamePreview(inspect, (u) => updates.push(u));
    p.request(files, '', 'x');
    expect(updates.at(-1)).toMatchObject({ plans: null, error: '' });

    p.request(files, 'a', 'b');
    p.dispose();
    await vi.advanceTimersByTimeAsync(PREVIEW_DEBOUNCE_MS);
    expect(inspect).not.toHaveBeenCalled();
  });
});
