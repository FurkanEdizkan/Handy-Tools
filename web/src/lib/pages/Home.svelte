<script lang="ts">
  /* Home — quick actions + the tool catalog. */
  import { jobs } from '../stores/jobs';

  interface ToolCardDef {
    id: string;
    glyph: string;
    label: string;
    desc: string;
    formats: { name: string; req?: string }[];
  }

  // Catalog for the Home grid. Format chips marked with `req` need an optional
  // system binary (Doctor lists them); they render as yellow `.req` chips.
  const tools: ToolCardDef[] = [
    {
      id: 'convert-image',
      glyph: '◇',
      label: 'Convert images',
      desc: 'Reencode between PNG · JPEG · WebP · GIF · BMP · TIFF · HEIC.',
      formats: [
        { name: 'PNG' },
        { name: 'JPEG' },
        { name: 'GIF' },
        { name: 'BMP' },
        { name: 'TIFF' },
        { name: 'WebP' },
        { name: 'HEIC', req: 'magick' },
      ],
    },
    {
      id: 'zip-pack',
      glyph: '▢',
      label: 'Pack into archive',
      desc: 'Bundle files and folders into a single archive.',
      formats: [
        { name: 'zip' },
        { name: 'tar.gz' },
        { name: 'tar.bz2' },
        { name: 'tar.zst' },
        { name: 'tar.xz' },
        { name: '7z', req: '7z' },
      ],
    },
    {
      id: 'archive-extract',
      glyph: '◰',
      label: 'Extract archive',
      desc: 'Unpack any common archive — including multi-part RAR and 7z.',
      formats: [
        { name: 'zip' },
        { name: 'tar' },
        { name: 'gz' },
        { name: 'bz2' },
        { name: 'zst' },
        { name: 'rar', req: 'unrar' },
        { name: '7z', req: '7z' },
      ],
    },
    {
      id: 'pdf',
      glyph: '◫',
      label: 'PDF utilities',
      desc: 'Merge, split, render pages to images, extract text.',
      formats: [
        { name: 'merge' },
        { name: 'split' },
        { name: '→ image', req: 'pdftoppm' },
        { name: '→ text', req: 'pdftotext' },
      ],
    },
  ];

  const counts = $derived.by(() => {
    let done = 0;
    let fail = 0;
    for (const j of $jobs) {
      if (j.status === 'done') done++;
      else if (j.status === 'fail') fail++;
    }
    return { done, fail };
  });
</script>

<div class="page-header">
  <div>
    <h1>Welcome back</h1>
    <div class="desc">A toolbox for everyday file work — pick a tool to get going.</div>
  </div>
</div>

<div class="quick-row">
  <a class="quick-card" href="#/tool/convert-image">
    <div class="qi">↓</div>
    <div>
      <div class="ql">Convert images</div>
      <div class="qs">PNG · JPEG · WebP and more</div>
    </div>
  </a>
  <a class="quick-card" href="#/jobs">
    <div class="qi">⌗</div>
    <div>
      <div class="ql">Recent jobs</div>
      <div class="qs">{counts.done} done · {counts.fail} failed</div>
    </div>
  </a>
  <a class="quick-card" href="#/doctor">
    <div class="qi">◊</div>
    <div>
      <div class="ql">System check</div>
      <div class="qs">Optional tools &amp; install hints</div>
    </div>
  </a>
</div>

<div class="section-head">
  <h2>Tools</h2>
  <span class="meta">{tools.length} available</span>
</div>
<div class="tool-grid">
  {#each tools as t (t.id)}
    <a class="tool-card" href="#/tool/{t.id}">
      <div class="top">
        <div class="glyph">{t.glyph}</div>
        <div style="flex:1">
          <div class="title">{t.label}</div>
          <div class="desc">{t.desc}</div>
        </div>
      </div>
      <div class="formats">
        {#each t.formats as f (f.name)}
          <span class="fmt {f.req ? 'req' : ''}" title={f.req ? `Requires ${f.req}` : undefined}>
            {f.name}{f.req ? ' *' : ''}
          </span>
        {/each}
      </div>
      <div class="footer">
        <span>Handy Tools</span>
        <span class="open">Open <span style="font-size:11px">→</span></span>
      </div>
    </a>
  {/each}
</div>
