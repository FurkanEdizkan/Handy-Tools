<script lang="ts">
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import { ApiError, api, type RenameCollision, type RenamePlan } from '../api';
  import { runJob, resolveSources } from './run';
  import type { PickedFile } from './toolform';

  const COLLISIONS: RenameCollision[] = ['error', 'skip', 'suffix'];

  let files = $state<PickedFile[]>([]);
  let pattern = $state('');
  let replace = $state('');
  let onCollision = $state<RenameCollision>('error');
  let inspecting = $state(false);
  let running = $state(false);
  let preview = $state<RenamePlan[] | null>(null);

  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const previewReady = $derived(files.length > 0 && pattern !== '');
  // Apply is gated behind a successful preview — rename is destructive.
  const applyReady = $derived(preview !== null && preview.some((p) => p.from !== p.to));

  function addFiles(dropped: File[] | string[]): void {
    files = [
      ...files,
      ...dropped.map((d) =>
        typeof d === 'string'
          ? { name: d.split(/[/\\]/).pop() ?? d, path: d }
          : { name: d.name, file: d },
      ),
    ];
    // Picking new files invalidates the previous preview.
    preview = null;
  }

  function removeFile(index: number): void {
    files = files.filter((_, i) => i !== index);
    preview = null;
  }

  function invalidatePreview(): void {
    preview = null;
  }

  async function runPreview(): Promise<void> {
    if (!previewReady || inspecting) return;
    let resolved;
    try {
      resolved = resolveSources(files);
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Could not preview.';
      toastTone = 'error';
      toastVisible = true;
      return;
    }
    inspecting = true;
    try {
      const res = await api.renameInspect({
        sources: resolved.sources.map((s) => ({ path: s.path })),
        pattern,
        replace,
        onCollision,
      });
      preview = res.plans;
      const renames = res.plans.filter((p) => p.from !== p.to).length;
      toastMsg = `Preview: ${renames} of ${res.plans.length} file(s) would be renamed.`;
      toastTone = 'info';
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Preview failed.';
      toastTone = 'error';
    }
    inspecting = false;
    toastVisible = true;
  }

  async function runApply(): Promise<void> {
    if (!applyReady || running) return;
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
      api.renameRun({
        sources: resolved.sources.map((s) => ({ path: s.path })),
        pattern,
        replace,
        onCollision,
      }),
    );
    running = false;
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
    // Force another preview before another apply.
    if (outcome.ok) preview = null;
  }

  function basename(path: string): string {
    return path.split(/[/\\]/).pop() ?? path;
  }
</script>

<div class="page-header">
  <div class="icon-block">✎</div>
  <div style="flex:1">
    <h1>Batch rename</h1>
    <div class="desc">Apply a regex pattern + replacement across many files. Always previews first.</div>
  </div>
</div>

<div class="conv-grid">
  <div class="panel">
    <div class="panel-head"><span>Input</span></div>
    <div class="panel-body">
      <Dropzone label="Drop files to rename" hint="Browse files" onfiles={addFiles} />
    </div>
    <div class="files-head">
      <span class="ttl">Files</span>
      <span class="meta">({files.length})</span>
    </div>
    {#if files.length === 0}
      <div class="empty-note">No files yet. Drop something into the box above.</div>
    {:else if preview === null}
      {#each files as f, i (f.name + i)}
        <div class="file-row">
          <div class="thumb">✎</div>
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
    {:else}
      {#each preview as p (p.from)}
        <div class="file-row {p.from === p.to ? '' : 'diverged'}">
          <div class="thumb">{p.from === p.to ? '·' : '→'}</div>
          <div class="file-name">
            {basename(p.from)}
            {#if p.from !== p.to}<span class="dim">→ {basename(p.to)}</span>{:else}<span class="dim">unchanged</span>{/if}
          </div>
          <span></span>
          <span></span>
          <span></span>
          <span></span>
        </div>
      {/each}
    {/if}
  </div>

  <div class="conv-col">
    <div class="panel">
      <div class="panel-head"><span>Rule</span></div>
      <div class="panel-body">
        <div class="opt-label">Pattern (Go regexp)</div>
        <input
          class="text-input"
          style="width:100%"
          bind:value={pattern}
          oninput={invalidatePreview}
          placeholder="^IMG_(\d+)"
        />
        <div class="opt-label" style="margin-top:12px">Replacement</div>
        <input
          class="text-input"
          style="width:100%"
          bind:value={replace}
          oninput={invalidatePreview}
          placeholder="photo-$1"
        />
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><span>On collision</span></div>
      <div class="panel-body">
        <div class="seg wide">
          {#each COLLISIONS as c (c)}
            <button class={onCollision === c ? 'on' : ''} onclick={() => { onCollision = c; invalidatePreview(); }}>{c}</button>
          {/each}
        </div>
        <div class="dim" style="margin-top:8px;font-size:12px">
          {#if onCollision === 'error'}
            Abort the whole batch if any target already exists.
          {:else if onCollision === 'skip'}
            Leave colliding files untouched and continue.
          {:else}
            Append -1, -2, … to colliding targets.
          {/if}
        </div>
      </div>
    </div>

    <div class="run-btn-block" style="gap:8px;display:flex;flex-direction:column">
      <button class="btn ghost" disabled={!previewReady || inspecting} onclick={runPreview}>
        {inspecting ? 'Previewing…' : '▸ Preview rename'}
      </button>
      <button class="btn primary" disabled={!applyReady || running} onclick={runApply}>
        {running ? 'Running…' : '▸ Apply rename'}
      </button>
    </div>
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
