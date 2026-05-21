// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { dirOf, runJob, stemOf } from './run';

afterEach(() => {
  delete (window as unknown as { go?: unknown }).go;
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
  it('refuses in a plain browser — no Wails runtime', async () => {
    let started = false;
    const res = await runJob(async () => {
      started = true;
      return { jobId: 'x' };
    });
    expect(res.ok).toBe(false);
    expect(res.message).toMatch(/desktop app/i);
    expect(started).toBe(false); // never POSTs without the desktop runtime
  });

  it('reports queued jobs when the desktop runtime is present', async () => {
    (window as unknown as { go: unknown }).go = { main: { App: {} } };
    const res = await runJob(async () => [{ jobId: 'a' }, { jobId: 'b' }]);
    expect(res.ok).toBe(true);
    expect(res.message).toMatch(/2 jobs/);
  });

  it('surfaces a thrown error as a failed outcome', async () => {
    (window as unknown as { go: unknown }).go = { main: { App: {} } };
    const res = await runJob(async () => {
      throw new Error('boom');
    });
    expect(res.ok).toBe(false);
  });
});
