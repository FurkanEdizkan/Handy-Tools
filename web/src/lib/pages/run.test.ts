// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import {
  archiveStem,
  basenameOf,
  buildPdfRequests,
  computeExtractDestDir,
  dirOf,
  imageOutputDir,
  resolveSources,
  runJob,
  stemOf,
  type PdfRunOptions,
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

  it('refuses a dropped file with no path instead of sending its bare name', () => {
    // A drag-and-drop File carries only a name. Sending "photo.png" as a
    // server path would resolve against htoolsd's cwd — possibly a different
    // file with the same name.
    const dropped: PickedFile = { name: 'photo.png', file: new File(['x'], 'photo.png') };
    expect(() => resolveSources([dropped])).toThrow(/photo\.png.*Browse/);
    expect(() =>
      resolveSources([{ name: 'a.png', path: '/home/me/a.png' }, dropped]),
    ).toThrow(/no filesystem path/);
  });
});

describe('imageOutputDir', () => {
  const first = '/home/me/A/one.png';

  it('writes next to each source in "alongside" mode', () => {
    expect(imageOutputDir('alongside', '/home/me/A/one.png', first, '')).toBe('/home/me/A');
    expect(imageOutputDir('alongside', '/home/me/B/two.png', first, '')).toBe('/home/me/B');
  });

  it('uses the first source\'s folder in "default" mode', () => {
    expect(imageOutputDir('default', '/home/me/B/two.png', first, '')).toBe('/home/me/A');
  });

  it('uses the trimmed custom path, falling back to default when blank', () => {
    expect(imageOutputDir('custom', '/home/me/B/two.png', first, ' /out ')).toBe('/out');
    expect(imageOutputDir('custom', '/home/me/B/two.png', first, '  ')).toBe('/home/me/A');
  });
});

describe('buildPdfRequests', () => {
  const opts: PdfRunOptions = { splitEvery: 5, dpi: 200, renderFormat: 'JPEG', keepLayout: false };
  const sources = [
    { name: 'a.pdf', path: '/docs/x/a.pdf' },
    { name: 'b.pdf', path: '/docs/y/b.pdf' },
    { name: 'c.pdf', path: '/docs/y/c.pdf' },
  ];

  it('extracts text from every document, each next to its own source', () => {
    const reqs = buildPdfRequests('text', sources, '', opts);
    expect(reqs.map((r) => r.kind)).toEqual(['text', 'text', 'text']);
    expect(reqs.map((r) => (r.kind === 'text' ? r.req.output.file : ''))).toEqual([
      '/docs/x/a.txt',
      '/docs/y/b.txt',
      '/docs/y/c.txt',
    ]);
    expect(reqs.map((r) => (r.kind === 'merge' ? '' : r.req.source.path))).toEqual(
      sources.map((s) => s.path),
    );
  });

  it('splits and renders every document, honouring an output-folder override', () => {
    const split = buildPdfRequests('split', sources, ' /out/ ', opts);
    expect(split).toHaveLength(3);
    for (const r of split) {
      expect(r).toMatchObject({ kind: 'split', req: { everyN: 5, pageRanges: [], output: { directory: '/out' } } });
    }
    const render = buildPdfRequests('render', sources, '', opts);
    expect(render.map((r) => (r.kind === 'render' ? r.req.output.directory : ''))).toEqual([
      '/docs/x',
      '/docs/y',
      '/docs/y',
    ]);
    expect(render[0]).toMatchObject({ req: { dpi: 200, targetFormat: 'JPEG' } });
  });

  it('keeps merge as a single job into the first source\'s folder', () => {
    const reqs = buildPdfRequests('merge', sources, '', opts);
    expect(reqs).toHaveLength(1);
    expect(reqs[0]).toMatchObject({
      kind: 'merge',
      req: { sources: sources.map((s) => ({ path: s.path })), output: { file: '/docs/x/merged.pdf' } },
    });
  });
});
