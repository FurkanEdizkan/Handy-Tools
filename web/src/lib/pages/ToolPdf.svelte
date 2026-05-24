<script lang="ts">
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import { PDF_OPS, pdfFormReady, pdfSummary, type PdfOp, type PickedFile } from './toolform';
  import { ApiError, api } from '../api';
  import { runJob, resolveSources, stemOf } from './run';

  let op = $state<PdfOp>('merge');
  let files = $state<PickedFile[]>([]);
  let splitEvery = $state(10);
  let dpi = $state(150);
  let renderFormat = $state<'PNG' | 'JPEG'>('PNG');
  let keepLayout = $state(true);
  let running = $state(false);
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const summary = $derived(pdfSummary(op, files.length));
  const ready = $derived(pdfFormReady(op, files.length));
  const opLabel = $derived(PDF_OPS.find((o) => o.value === op)?.label ?? op);

  function addFiles(dropped: File[] | string[]): void {
    files = [
      ...files,
      ...dropped.map((d) =>
        typeof d === 'string'
          ? { name: d.split(/[/\\]/).pop() ?? d, path: d }
          : { name: d.name, file: d },
      ),
    ];
  }
  function removeFile(index: number): void {
    files = files.filter((_, i) => i !== index);
  }

  async function run(): Promise<void> {
    if (!ready || running) return;
    let resolved;
    try {
      resolved = resolveSources(files);
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Could not start the job.';
      toastTone = 'error';
      toastVisible = true;
      return;
    }
    const first = resolved.sources[0].path;
    const dir = resolved.outputDir;
    running = true;
    const outcome = await runJob(() => {
      switch (op) {
        case 'split':
          return api.pdfSplit({ source: { path: first }, pageRanges: [], everyN: splitEvery, output: { directory: dir } });
        case 'render':
          return api.pdfToImage({
            source: { path: first },
            pages: { from: 0, to: 0 },
            dpi,
            targetFormat: renderFormat,
            output: { directory: dir },
          });
        case 'text':
          return api.pdfToText({
            source: { path: first },
            pages: { from: 0, to: 0 },
            layout: keepLayout,
            output: { file: `${dir}/${stemOf(first)}.txt` },
          });
        default:
          return api.pdfMerge({
            sources: resolved.sources.map((s) => ({ path: s.path })),
            output: { file: `${dir}/merged.pdf` },
          });
      }
    });
    running = false;
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }
</script>

<div class="page-header">
  <div class="icon-block">◫</div>
  <div style="flex:1">
    <h1>PDF utilities</h1>
    <div class="desc">Merge, split, render pages to images, extract text.</div>
  </div>
</div>

<div class="conv-grid">
  <div class="panel">
    <div class="panel-head">
      <span>Input</span>
      <span class="right">accepts PDF</span>
    </div>
    <div class="panel-body">
      <Dropzone label="Drop PDF files here" hint="Browse files" onfiles={addFiles} />
    </div>
    <div class="files-head">
      <span class="ttl">Documents</span>
      <span class="meta">({files.length})</span>
      {#if op === 'merge'}<span class="meta">· merged in this order</span>{/if}
    </div>
    {#if files.length === 0}
      <div class="empty-note">No documents yet. Drop PDFs into the box above.</div>
    {:else}
      {#each files as f, i (f.name + i)}
        <div class="file-row">
          <div class="thumb pdf">{i + 1}</div>
          <div class="file-name">
            {f.name}
            {#if f.path}<span class="dim">{f.path}</span>{/if}
          </div>
          <span></span>
          <span></span>
          <span></span>
          <button class="file-x" onclick={() => removeFile(i)} title="remove" aria-label="Remove {f.name}">×</button>
        </div>
      {/each}
    {/if}
  </div>

  <div class="conv-col">
    <div class="panel">
      <div class="panel-head"><span>Operation</span></div>
      <div class="panel-body">
        <div class="seg wide">
          {#each PDF_OPS as o (o.value)}
            <button class={op === o.value ? 'on' : ''} onclick={() => (op = o.value)}>{o.label}</button>
          {/each}
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><span>{opLabel} options</span></div>
      <div class="panel-body">
        {#if op === 'merge'}
          <p style="font-size:12px;color:var(--text-dim);margin:0">
            Merge concatenates the documents into one PDF.
          </p>
        {:else if op === 'split'}
          <div class="slider-row">
            <span class="lbl">New file every N pages</span>
            <span class="val">{splitEvery}</span>
            <input type="range" min="1" max="100" bind:value={splitEvery} />
          </div>
        {:else if op === 'render'}
          <div class="slider-row">
            <span class="lbl">Resolution (DPI)</span>
            <span class="val">{dpi}</span>
            <input type="range" min="72" max="600" bind:value={dpi} />
          </div>
          <div class="toggle-row" style="padding-top:10px">
            <span class="lbl">Image format</span>
            <div class="seg">
              <button class={renderFormat === 'PNG' ? 'on' : ''} onclick={() => (renderFormat = 'PNG')}>PNG</button>
              <button class={renderFormat === 'JPEG' ? 'on' : ''} onclick={() => (renderFormat = 'JPEG')}>JPEG</button>
            </div>
          </div>
        {:else}
          <div class="toggle-row">
            <div class="lbl"><span>Preserve visual layout</span><span class="sub">Keep columns and spacing</span></div>
            <button class="toggle {keepLayout ? 'on' : ''}" aria-label="Preserve visual layout" onclick={() => (keepLayout = !keepLayout)}></button>
          </div>
        {/if}
      </div>
    </div>

    <div class="run-summary">{summary}</div>

    <div class="run-btn-block">
      <button class="btn primary" disabled={!ready || running} onclick={run}>
        {running ? 'Running…' : `▸ Run ${opLabel.toLowerCase()}`}
      </button>
    </div>
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
