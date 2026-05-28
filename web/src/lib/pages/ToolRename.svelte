<script lang="ts">
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import RunStatus from '../components/RunStatus.svelte';
  import { ApiError, api, type RenameCollision, type RenamePlan } from '../api';
  import { runJob, resolveSources } from './run';
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

  // Live preview: JS regex over the file list, recomputed on every keystroke.
  // The actual rename on Apply goes through the server-side Go regexp, which
  // is the source of truth. Most syntax matches between the two engines —
  // character classes, $1 backrefs, alternation. JS-only features (named
  // groups via (?<name>...), lookbehinds) won't run server-side; the hint
  // below tells the user.
  const preview = $derived.by<RenamePlan[] | null>(() => {
    if (!pattern || files.length === 0) return null;
    let re: RegExp;
    try {
      re = new RegExp(pattern);
    } catch {
      return null;
    }
    return files.map((f) => ({
      from: f.path ?? f.name,
      to: f.name.replace(re, replace),
    }));
  });
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
    if (outcome.ok) activeJobIds = outcome.jobIds;
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }

  function basename(path: string): string {
    return path.split(/[/\\]/).pop() ?? path;
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
      {#each preview as p, i (p.from + i)}
        <div class="file-row {p.from === p.to ? '' : 'diverged'}">
          <div class="thumb">{p.from === p.to ? '·' : '→'}</div>
          <div class="file-name">
            {basename(p.from)}
            {#if p.from !== p.to}<span class="dim">→ {basename(p.to)}</span>{:else}<span class="dim">unchanged</span>{/if}
          </div>
          <span></span>
          <span></span>
          <span></span>
          <button class="file-x" onclick={() => removeFile(i)} title="remove" aria-label="Remove {basename(p.from)}">×</button>
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
        <div class="dim" style="margin-top:8px;font-size:11px">
          Preview uses JavaScript regex; the actual rename uses Go's RE2 server-side.
          Most syntax matches (char classes, <code>$1</code> backrefs) — JS-only features
          like lookbehinds and named groups won't run on Apply.
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
