<script lang="ts">
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import RunStatus from '../components/RunStatus.svelte';
  import FolderPicker from '../components/FolderPicker.svelte';
  import { IMAGE_FORMATS, type ImageFile, type ImageFormat } from './toolform';
  import { ApiError, api, type ImageTargetFormat } from '../api';
  import { runJob, resolveSources } from './run';

  let files = $state<ImageFile[]>([]);
  let defaultFmt = $state<ImageFormat>('JPEG');
  let outMode = $state<'default' | 'alongside' | 'custom'>('default');
  let customPath = $state('');
  let quality = $state(90);
  let overwrite = $state(false);
  let stripMetadata = $state(false);
  let recurse = $state(true);
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');
  let running = $state(false);
  let activeJobIds = $state<string[]>([]);

  const summary = $derived.by(() => {
    const counts = new Map<string, number>();
    for (const f of files) counts.set(f.target, (counts.get(f.target) ?? 0) + 1);
    return {
      count: files.length,
      parts: IMAGE_FORMATS.filter((fmt) => counts.has(fmt)).map((fmt) => `${counts.get(fmt)} → ${fmt}`),
    };
  });
  const ready = $derived(files.length > 0);

  function extOf(name: string): string {
    const dot = name.lastIndexOf('.');
    return dot >= 0 ? name.slice(dot + 1).toLowerCase() : '';
  }

  function addFiles(dropped: File[] | string[]): void {
    const picked: ImageFile[] = dropped.map((d) =>
      typeof d === 'string'
        ? { name: d.split(/[/\\]/).pop() ?? d, path: d, target: defaultFmt }
        : { name: d.name, file: d, target: defaultFmt },
    );
    files = [...files, ...picked];
  }

  function removeFile(index: number): void {
    files = files.filter((_, i) => i !== index);
  }

  function cycleFormat(index: number): void {
    files = files.map((f, i) => {
      if (i !== index) return f;
      const next = IMAGE_FORMATS[(IMAGE_FORMATS.indexOf(f.target) + 1) % IMAGE_FORMATS.length];
      return { ...f, target: next };
    });
  }

  function applyDefaultToAll(): void {
    files = files.map((f) => ({ ...f, target: defaultFmt }));
  }

  function changeDefault(fmt: ImageFormat): void {
    defaultFmt = fmt;
    applyDefaultToAll();
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
    const outDir = outMode === 'custom' && customPath.trim() ? customPath.trim() : resolved.outputDir;
    running = true;
    const outcome = await runJob(() =>
      Promise.all(
        resolved.sources.map((src, i) =>
          api.imageConvert({
            source: { path: src.path },
            targetFormat: files[i].target.toUpperCase() as ImageTargetFormat,
            options: { quality, maxWidth: 0, maxHeight: 0, stripMetadata },
            output: { directory: outDir, overwrite },
          }),
        ),
      ),
    );
    running = false;
    if (outcome.ok) activeJobIds = outcome.jobIds;
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }
</script>

<div class="page-header">
  <div class="icon-block">◇</div>
  <div style="flex:1">
    <h1>Convert images</h1>
    <div class="desc">Reencode between PNG · JPEG · WebP · GIF · BMP · TIFF · HEIC.</div>
  </div>
</div>

<div class="conv-grid">
  <!-- LEFT: input + files -->
  <div class="panel">
    <div class="panel-head">
      <span>Input</span>
      <span class="right">accepts PNG · JPEG · GIF · BMP · TIFF · WebP · HEIC</span>
    </div>
    <div class="panel-body">
      <Dropzone label="Drop images or a folder here" hint="Browse files" onfiles={addFiles} />
    </div>

    <div class="files-head">
      <span class="ttl">Files</span>
      <span class="meta">({files.length})</span>
      <span class="meta">· default → {defaultFmt}</span>
      <span class="spacer"></span>
      <span class="hint"><span class="kbd">click</span> a format to cycle</span>
    </div>

    {#if files.length === 0}
      <div class="empty-note">No files yet. Drop something into the box above.</div>
    {:else}
      {#each files as f, i (f.name + i)}
        <div class="file-row {f.target !== defaultFmt ? 'diverged' : ''}">
          <div class="thumb {extOf(f.name)}">{extOf(f.name).toUpperCase() || 'IMG'}</div>
          <div class="file-name">
            {f.name}
            {#if f.path}<span class="dim">{f.path}</span>{/if}
          </div>
          <span class="fmt-pill from">{extOf(f.name).toUpperCase() || '—'}</span>
          <span style="color:var(--text-dim)">→</span>
          <button class="fmt-pill" onclick={() => cycleFormat(i)} title="click to cycle format">
            {f.target} <span class="arrow">▾</span>
          </button>
          <button class="file-x" onclick={() => removeFile(i)} title="remove" aria-label="Remove {f.name}">×</button>
        </div>
      {/each}
    {/if}
  </div>

  <!-- RIGHT: format default + output + options + run -->
  <div class="conv-col">
    <div class="panel">
      <div class="panel-head"><span>Format default</span></div>
      <div class="panel-body">
        <div class="seg wide">
          {#each IMAGE_FORMATS as fmt (fmt)}
            <button class={defaultFmt === fmt ? 'on' : ''} onclick={() => changeDefault(fmt)}>{fmt}</button>
          {/each}
        </div>
        <button
          class="btn ghost"
          style="margin-top:10px;width:100%;justify-content:center;font-size:12px"
          onclick={applyDefaultToAll}
        >
          Apply to all files
        </button>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><span>Output destination</span></div>
      <div class="panel-body">
        <div class="radio-list">
          <div
            class="radio-opt {outMode === 'default' ? 'on' : ''}"
            role="radio"
            aria-checked={outMode === 'default'}
            tabindex="0"
            onclick={() => (outMode = 'default')}
            onkeydown={(e) => e.key === 'Enter' && (outMode = 'default')}
          >
            <span class="ring"></span>
            <span class="lbl">Default location<span class="sub mono">./out</span></span>
            <span class="tag reco">RECO</span>
          </div>
          <div
            class="radio-opt {outMode === 'alongside' ? 'on' : ''}"
            role="radio"
            aria-checked={outMode === 'alongside'}
            tabindex="0"
            onclick={() => (outMode = 'alongside')}
            onkeydown={(e) => e.key === 'Enter' && (outMode = 'alongside')}
          >
            <span class="ring"></span>
            <span class="lbl">Alongside input<span class="sub">Write next to each source file</span></span>
            <span></span>
          </div>
          <div
            class="radio-opt {outMode === 'custom' ? 'on' : ''}"
            role="radio"
            aria-checked={outMode === 'custom'}
            tabindex="0"
            onclick={() => (outMode = 'custom')}
            onkeydown={(e) => e.key === 'Enter' && (outMode = 'custom')}
          >
            <span class="ring"></span>
            <span class="lbl">Custom path</span>
            <span></span>
            {#if outMode === 'custom'}
              <FolderPicker bind:value={customPath} placeholder="/path/to/output" />
            {/if}
          </div>
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><span>Options</span></div>
      <div class="panel-body">
        <div class="slider-row">
          <span class="lbl">JPEG / WebP quality</span>
          <span class="val">{quality}</span>
          <input type="range" min="1" max="100" bind:value={quality} />
        </div>
        <div class="toggle-row">
          <div class="lbl"><span>Overwrite existing</span><span class="sub">Replace files at the output path</span></div>
          <button class="toggle {overwrite ? 'on' : ''}" aria-label="Overwrite existing" onclick={() => (overwrite = !overwrite)}></button>
        </div>
        <div class="toggle-row">
          <div class="lbl"><span>Strip metadata</span><span class="sub">Drop EXIF and colour profiles</span></div>
          <button class="toggle {stripMetadata ? 'on' : ''}" aria-label="Strip metadata" onclick={() => (stripMetadata = !stripMetadata)}></button>
        </div>
        <div class="toggle-row">
          <div class="lbl"><span>Recurse subfolders</span><span class="sub">When the input is a folder</span></div>
          <button class="toggle {recurse ? 'on' : ''}" aria-label="Recurse subfolders" onclick={() => (recurse = !recurse)}></button>
        </div>
      </div>
    </div>

    <div class="run-summary">
      <b>{summary.count}</b> input{summary.count === 1 ? '' : 's'}
      {#each summary.parts as p (p)}
        · <span class="accent">{p}</span>
      {/each}
    </div>

    <div class="run-btn-block">
      <button class="btn primary" disabled={!ready || running} onclick={run}>
        {running ? 'Running…' : '▸ Run conversion'}
      </button>
    </div>

    <RunStatus jobIds={activeJobIds} />
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
