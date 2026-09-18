<script lang="ts">
  import { onDestroy } from 'svelte';
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import RunStatus from '../components/RunStatus.svelte';
  import { api, type RenameCollision } from '../api';
  import { basenameOf, runJob, resolveSources } from './run';
  import {
    createRenamePreview,
    planChanges,
    renamePreviewKey,
    type RenamePreview,
  } from './rename-preview';
  import type { PickedFile } from './toolform';

  const COLLISIONS: RenameCollision[] = ['error', 'skip', 'suffix'];

  let files = $state<PickedFile[]>([]);
  let pattern = $state('');
  let replace = $state('');
  let onCollision = $state<RenameCollision>('error');
  let running = $state(false);
  let activeJobIds = $state<string[]>([]);

  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  // Live preview: a debounced server dry run (POST /v1/rename/inspect), so
  // the rows show exactly what Apply will do under Go's regexp semantics.
  let preview = $state<RenamePreview>({ key: '', plans: null, error: '' });
  const previewer = createRenamePreview((req) => api.renameInspect(req), (p) => (preview = p));
  $effect(() => {
    previewer.request(files, pattern, replace);
  });
  onDestroy(() => previewer.dispose());

  const currentKey = $derived(renamePreviewKey(files, pattern, replace));
  // Rows keep showing the latest plan while it still describes the current
  // file list (no flicker while typing); Apply waits for a plan computed for
  // exactly the current inputs.
  const rows = $derived.by(() => {
    const plans = preview.plans;
    if (!plans || plans.length !== files.length) return null;
    return plans.every((p, i) => basenameOf(p.from) === basenameOf(files[i].path ?? '')) ? plans : null;
  });
  const applyReady = $derived(
    preview.key === currentKey && preview.plans !== null && preview.plans.some(planChanges),
  );

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

  async function runApply(): Promise<void> {
    if (!applyReady || running) return;
    let resolved;
    try {
      resolved = resolveSources(files);
    } catch (e) {
      toastMsg = e instanceof Error ? e.message : 'Could not start the job.';
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
    if (outcome.ok) activeJobIds = outcome.jobIds;
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }
</script>

<div class="page-header">
  <div class="icon-block">✎</div>
  <div style="flex:1">
    <h1>Batch rename</h1>
    <div class="desc">Apply a regex pattern + replacement across many files. Live preview as you type.</div>
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
    {:else if rows === null}
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
      {#each rows as p, i (p.from + i)}
        {@const changed = planChanges(p)}
        <div class="file-row {changed ? 'diverged' : ''}">
          <div class="thumb">{changed ? '→' : '·'}</div>
          <div class="file-name">
            {basenameOf(p.from)}
            {#if changed}<span class="dim">→ {basenameOf(p.to)}</span>{:else}<span class="dim">unchanged</span>{/if}
          </div>
          <span></span>
          <span></span>
          <span></span>
          <button class="file-x" onclick={() => removeFile(i)} title="remove" aria-label="Remove {basenameOf(p.from)}">×</button>
        </div>
      {/each}
    {/if}
  </div>

  <div class="conv-col">
    <div class="panel">
      <div class="panel-head"><span>Rule</span></div>
      <div class="panel-body">
        <div class="opt-label">Pattern (regex)</div>
        <input
          class="text-input"
          style="width:100%"
          bind:value={pattern}
          placeholder="^IMG_(\d+)"
        />
        <div class="opt-label" style="margin-top:12px">Replacement</div>
        <input
          class="text-input"
          style="width:100%"
          bind:value={replace}
          placeholder="photo-$1"
        />
        {#if preview.error && preview.key === currentKey}
          <div style="margin-top:8px;font-size:12px;color:var(--error)">{preview.error}</div>
        {/if}
        <div class="dim" style="margin-top:8px;font-size:11px">
          Preview is a server-side dry run using Go's RE2 regexp — every match is replaced.
          Lookbehinds aren't supported; write <code>{'${1}_x'}</code>, not <code>$1_x</code>, when
          text follows a group reference.
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><span>On collision</span></div>
      <div class="panel-body">
        <div class="seg wide">
          {#each COLLISIONS as c (c)}
            <button class={onCollision === c ? 'on' : ''} onclick={() => (onCollision = c)}>{c}</button>
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

    <div class="run-btn-block">
      <button class="btn primary" disabled={!applyReady || running} onclick={runApply}>
        {running ? 'Running…' : '▸ Apply rename'}
      </button>
    </div>

    <RunStatus jobIds={activeJobIds} />
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
