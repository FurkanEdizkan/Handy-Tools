<script lang="ts">
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import { ARCHIVE_FORMATS, archivePackReady, archivePackSummary, type ArchiveFormat, type PickedFile } from './toolform';
  import { ApiError, api } from '../api';
  import { runJob, resolveSources } from './run';

  let files = $state<PickedFile[]>([]);
  let format = $state<ArchiveFormat>('zip');
  let output = $state('archive.zip');
  let level = $state(6);
  let overwrite = $state(false);
  let running = $state(false);
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const summary = $derived(archivePackSummary(files.length, format, output));
  const ready = $derived(archivePackReady(files.length, output));

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

  function pickFormat(fmt: ArchiveFormat): void {
    format = fmt;
    // Keep the output filename's extension aligned with the chosen format.
    const stem = output.replace(/\.(zip|tar\.gz|tar\.bz2|tar\.zst|7z)$/i, '') || 'archive';
    output = `${stem}.${fmt}`;
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
    running = true;
    const outcome = await runJob(() =>
      api.archiveCompress({
        sources: resolved.sources.map((s) => ({ path: s.path })),
        destination: { file: `${resolved.outputDir}/${output}`, overwrite },
        format: '',
        compressionLevel: level,
      }),
    );
    running = false;
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }
</script>

<div class="page-header">
  <div class="icon-block">▢</div>
  <div style="flex:1">
    <h1>Pack into archive</h1>
    <div class="desc">Bundle files and folders into a single archive.</div>
  </div>
</div>

<div class="conv-grid">
  <div class="panel">
    <div class="panel-head"><span>Input</span></div>
    <div class="panel-body">
      <Dropzone label="Drop files or a folder to pack" hint="Browse files" onfiles={addFiles} />
    </div>
    <div class="files-head">
      <span class="ttl">Files</span>
      <span class="meta">({files.length})</span>
    </div>
    {#if files.length === 0}
      <div class="empty-note">No files yet. Drop something into the box above.</div>
    {:else}
      {#each files as f, i (f.name + i)}
        <div class="file-row">
          <div class="thumb archive">{i + 1}</div>
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
      <div class="panel-head"><span>Output</span></div>
      <div class="panel-body">
        <div class="opt-label">Archive format</div>
        <div class="seg wide">
          {#each ARCHIVE_FORMATS as fmt (fmt)}
            <button class={format === fmt ? 'on' : ''} onclick={() => pickFormat(fmt)}>{fmt}</button>
          {/each}
        </div>
        <div class="opt-label" style="margin-top:12px">Output file</div>
        <input class="text-input" style="width:100%" bind:value={output} placeholder="archive.zip" />
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><span>Options</span></div>
      <div class="panel-body">
        <div class="slider-row">
          <span class="lbl">Compression level</span>
          <span class="val">{level}</span>
          <input type="range" min="0" max="9" bind:value={level} />
        </div>
        <div class="toggle-row">
          <div class="lbl"><span>Overwrite existing</span><span class="sub">Replace the archive if it exists</span></div>
          <button class="toggle {overwrite ? 'on' : ''}" aria-label="Overwrite existing" onclick={() => (overwrite = !overwrite)}></button>
        </div>
      </div>
    </div>

    <div class="run-summary">{summary}</div>

    <div class="run-btn-block">
      <button class="btn primary" disabled={!ready || running} onclick={run}>
        {running ? 'Running…' : '▸ Pack archive'}
      </button>
    </div>
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
