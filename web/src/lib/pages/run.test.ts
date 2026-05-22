// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { dirOf, resolveSources, runJob, stemOf } from './run';
import type { PickedFile } from './toolform';

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
  it('maps native paths onto tool sources', () => {
    const files: PickedFile[] = [
      { name: '/home/me/a.png', path: '/home/me/a.png' },
      { name: '/home/me/b.png', path: '/home/me/b.png' },
    ];
    const r = resolveSources(files);
    expect(r.sources.map((s) => s.path)).toEqual(['/home/me/a.png', '/home/me/b.png']);
    expect(r.outputDir).toBe('/home/me');
  });

  it('rejects an empty selection', () => {
    expect(() => resolveSources([])).toThrow();
  });
});
