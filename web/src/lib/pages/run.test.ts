// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import {
  archiveStem,
  basenameOf,
  computeExtractDestDir,
  dirOf,
  resolveSources,
  runJob,
  stemOf,
} from './run';
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

describe('basenameOf', () => {
  it('returns the trailing component', () => {
    expect(basenameOf('/a/b/c.zip')).toBe('c.zip');
    expect(basenameOf(String.raw`c:\a\b\c.zip`)).toBe('c.zip');
    expect(basenameOf('c.zip')).toBe('c.zip');
  });
});

describe('archiveStem', () => {
  it('strips compound tar extensions', () => {
    expect(archiveStem('/x/y/foo.tar.gz')).toBe('foo');
    expect(archiveStem('foo.TAR.BZ2')).toBe('foo');
    expect(archiveStem('foo.tar.zst')).toBe('foo');
    expect(archiveStem('foo.tar.xz')).toBe('foo');
  });

  it('strips RAR multi-part volume suffixes', () => {
    expect(archiveStem('/x/Crusade.part01.rar')).toBe('Crusade');
    expect(archiveStem('Crusade.PART9.rar')).toBe('Crusade');
  });

  it('strips 7z multi-part volume suffixes', () => {
    expect(archiveStem('foo.7z.001')).toBe('foo');
    expect(archiveStem('/x/foo.7z.123')).toBe('foo');
  });

  it('strips legacy RAR continuation suffixes', () => {
    expect(archiveStem('foo.r00')).toBe('foo');
    expect(archiveStem('foo.r99')).toBe('foo');
  });

  it('falls back to a single extension strip', () => {
    expect(archiveStem('foo.zip')).toBe('foo');
    expect(archiveStem('/x/y/foo.7z')).toBe('foo');
    expect(archiveStem('plain')).toBe('plain');
  });
});

describe('computeExtractDestDir', () => {
  it('appends archive stem to the user-chosen dir in "into" mode', () => {
    expect(computeExtractDestDir('/in/foo.zip', 'into', '/out')).toBe('/out/foo');
    expect(computeExtractDestDir('/in/big.tar.gz', 'into', '/out/')).toBe('/out/big');
    expect(computeExtractDestDir('/in/set.part01.rar', 'into', '/out')).toBe('/out/set');
  });

  it('anchors next to the source in "alongside" mode', () => {
    expect(computeExtractDestDir('/in/foo.zip', 'alongside', '')).toBe('/in/foo');
    expect(computeExtractDestDir('/in/sub/big.tar.gz', 'alongside', '/other')).toBe('/in/sub/big');
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
