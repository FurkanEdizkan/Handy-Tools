// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { dirOf, resolveSources, runJob, stemOf } from './run';
import { api } from '../api';
import type { PickedFile } from './toolform';

function setDesktop(on: boolean): void {
  if (on) {
    (window as unknown as { go: unknown }).go = { main: { App: {} } };
  } else {
    delete (window as unknown as { go?: unknown }).go;
  }
}

afterEach(() => {
  setDesktop(false);
  vi.restoreAllMocks();
});

describe('dirOf / stemOf', () => {
  it('splits a posix path', () => {
    expect(dirOf('/home/me/pics/a.png')).toBe('/home/me/pics');
    expect(stemOf('/home/me/pics/a.png')).toBe('a');
  });

  it('falls back to "." for a bare filename', () => {
    expect(dirOf('a.png')).toBe('.');
    expect(stemOf('a.png')).toBe('a');
  });
});

describe('runJob', () => {
  it('reports queued jobs and collects their ids', async () => {
    const res = await runJob(async () => [{ jobId: 'a' }, { jobId: 'b' }]);
    expect(res.ok).toBe(true);
    expect(res.message).toMatch(/2 jobs/);
    expect(res.jobIds).toEqual(['a', 'b']);
  });

  it('handles a single job response', async () => {
    const res = await runJob(async () => ({ jobId: 'solo' }));
    expect(res.ok).toBe(true);
    expect(res.jobIds).toEqual(['solo']);
  });

  it('surfaces a thrown error as a failed outcome', async () => {
    const res = await runJob(async () => {
      throw new Error('boom');
    });
    expect(res.ok).toBe(false);
    expect(res.jobIds).toEqual([]);
  });
});

describe('resolveSources', () => {
  it('uses native paths in the desktop build — no upload', async () => {
    setDesktop(true);
    const upload = vi.spyOn(api, 'uploadFiles');
    const files: PickedFile[] = [
      { name: '/home/me/a.png', path: '/home/me/a.png' },
      { name: '/home/me/b.png', path: '/home/me/b.png' },
    ];
    const r = await resolveSources(files);
    expect(upload).not.toHaveBeenCalled();
    expect(r.sources.map((s) => s.path)).toEqual(['/home/me/a.png', '/home/me/b.png']);
    expect(r.outputDir).toBe('/home/me');
    expect(r.uploadId).toBeUndefined();
  });

  it('uploads File blobs in a plain browser and uses the staged paths', async () => {
    setDesktop(false);
    const upload = vi.spyOn(api, 'uploadFiles').mockResolvedValue({
      uploadId: 'up_9',
      files: [{ name: 'a.png', path: '/ws/up_9/in/a.png' }],
      outputDir: '/ws/up_9/out',
    });
    const blob = new File(['x'], 'a.png', { type: 'image/png' });
    const r = await resolveSources([{ name: 'a.png', file: blob }]);
    expect(upload).toHaveBeenCalledOnce();
    expect(r.sources[0].path).toBe('/ws/up_9/in/a.png');
    expect(r.outputDir).toBe('/ws/up_9/out');
    expect(r.uploadId).toBe('up_9');
  });

  it('rejects an empty selection', async () => {
    await expect(resolveSources([])).rejects.toThrow();
  });
});
