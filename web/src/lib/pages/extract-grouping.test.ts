import { describe, expect, it } from 'vitest';
import { groupArchives, type InspectedSource } from './extract-grouping';
import type { InspectResponse } from '../api/types';

function ins(overrides: Partial<InspectResponse> = {}): InspectResponse {
  return {
    format: 'ZIP',
    multiPart: false,
    detectedParts: [],
    missingParts: [],
    uncompressedSizeBytes: 0,
    entryCount: 0,
    requiresPassword: false,
    requiresBinary: '',
    binaryAvailable: true,
    ...overrides,
  };
}

function item(path: string, i: Partial<InspectResponse> = {}): InspectedSource {
  return { src: { name: path.split('/').pop()!, path }, ins: ins(i) };
}

describe('groupArchives', () => {
  it('returns one group per independent archive in "into" mode', () => {
    const { groups, errors } = groupArchives(
      [
        item('/in/a.zip'),
        item('/in/b.tar.gz', { format: 'TAR_GZ' }),
        item('/in/c.7z', { format: 'SEVENZ' }),
      ],
      'into',
      '/out',
    );
    expect(errors).toHaveLength(0);
    expect(groups.map((g) => g.primary)).toEqual(['/in/a.zip', '/in/b.tar.gz', '/in/c.7z']);
    expect(groups.map((g) => g.destDir)).toEqual(['/out/a', '/out/b', '/out/c']);
  });

  it('coalesces every volume of a multi-part RAR set into one group', () => {
    const parts = [
      '/in/foo.part01.rar',
      '/in/foo.part02.rar',
      '/in/foo.part03.rar',
      '/in/foo.part04.rar',
    ];
    const detected = parts;
    const { groups, errors } = groupArchives(
      parts.map((p) => item(p, { format: 'RAR', multiPart: true, detectedParts: detected })),
      'into',
      '/out',
    );
    expect(errors).toHaveLength(0);
    expect(groups).toHaveLength(1);
    expect(groups[0].primary).toBe('/in/foo.part01.rar');
    expect(groups[0].parts).toEqual(detected);
    expect(groups[0].members.sort()).toEqual(parts.slice().sort());
    expect(groups[0].destDir).toBe('/out/foo');
  });

  it('still produces a single job when the user drops only one volume of a complete set', () => {
    const detected = ['/in/big.part01.rar', '/in/big.part02.rar'];
    const { groups, errors } = groupArchives(
      [item('/in/big.part02.rar', { format: 'RAR', multiPart: true, detectedParts: detected })],
      'into',
      '/out',
    );
    expect(errors).toHaveLength(0);
    expect(groups).toHaveLength(1);
    expect(groups[0].primary).toBe('/in/big.part01.rar');
    expect(groups[0].parts).toEqual(detected);
    expect(groups[0].destDir).toBe('/out/big');
  });

  it('rejects a multi-part archive when Inspect reports missing volumes', () => {
    const { groups, errors } = groupArchives(
      [
        item('/in/lonely.part01.rar', {
          format: 'RAR',
          multiPart: true,
          detectedParts: ['/in/lonely.part01.rar'],
          missingParts: ['lonely.part02.rar', 'lonely.part03.rar'],
        }),
      ],
      'into',
      '/out',
    );
    expect(groups).toHaveLength(0);
    expect(errors).toHaveLength(1);
    expect(errors[0].ins.missingParts).toHaveLength(2);
  });

  it('mixes independent + grouped + rejected entries in one pass', () => {
    const detected = ['/in/set.part01.rar', '/in/set.part02.rar'];
    const { groups, errors } = groupArchives(
      [
        item('/in/a.zip'),
        item('/in/set.part01.rar', { format: 'RAR', multiPart: true, detectedParts: detected }),
        item('/in/set.part02.rar', { format: 'RAR', multiPart: true, detectedParts: detected }),
        item('/in/broken.part01.rar', {
          format: 'RAR',
          multiPart: true,
          detectedParts: ['/in/broken.part01.rar'],
          missingParts: ['broken.part02.rar'],
        }),
        item('/in/b.zip'),
      ],
      'into',
      '/out',
    );
    expect(errors).toHaveLength(1);
    expect(groups.map((g) => g.primary)).toEqual([
      '/in/a.zip',
      '/in/set.part01.rar',
      '/in/b.zip',
    ]);
    expect(groups.map((g) => g.destDir)).toEqual(['/out/a', '/out/set', '/out/b']);
  });

  it('honours "alongside" destination mode', () => {
    const { groups } = groupArchives([item('/downloads/x.zip')], 'alongside', '/ignored');
    expect(groups[0].destDir).toBe('/downloads/x');
  });

  it('dedupes member entries when the user drops the same path twice', () => {
    const { groups } = groupArchives(
      [item('/in/a.zip'), item('/in/a.zip')],
      'into',
      '/out',
    );
    expect(groups).toHaveLength(1);
    expect(groups[0].members).toEqual(['/in/a.zip']);
  });
});
