<script lang="ts">
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import {
    archiveExtractReady,
    archiveExtractSummary,
    type ExtractDestMode,
    type PickedFile,
  } from './toolform';
  import { ApiError, api } from '../api';
  import type { InspectResponse } from '../api/types';
  import { basenameOf, resolveSources, runJob, type ResolvedSource } from './run';
  import { groupArchives } from './extract-grouping';
  import { pushPopup } from '../stores/notifications';

  let sources = $state<PickedFile[]>([]);
  let destMode = $state<ExtractDestMode>('into');
  let destination = $state('');
  let overwrite = $state(false);
  let autoMultiPart = $state(true);
  let running = $state(false);
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const summary = $derived(archiveExtractSummary(sources.length, destMode, destination));
  const ready = $derived(archiveExtractReady(sources.length, destMode, destination));

  function addSources(dropped: File[] | string[]): void {
    sources = [
      ...sources,
      ...dropped.map((d) =>
        typeof d === 'string'
          ? { name: d.split(/[/\\]/).pop() ?? d, path: d }
          : { name: d.name, file: d },
      ),
    ];
  }
  function removeSource(index: number): void {
    sources = sources.filter((_, i) => i !== index);
  }
  function clearSources(): void {
    sources = [];
  }

  async function run(): Promise<void> {
    if (!ready || running) return;
    let resolved;
    try {
      resolved = resolveSources(sources);
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Could not start the job.';
      toastTone = 'error';
      toastVisible = true;
      return;
    }
    running = true;

    // 1. Inspect every picked file in parallel. A failed inspect (e.g. server
    //    sandbox rejection) surfaces as a sticky popup; we keep going for the
    //    rest so one bad file doesn't sink the whole batch.
    type InspectResult =
      | { kind: 'ok'; src: ResolvedSource; ins: InspectResponse }
      | { kind: 'err'; src: ResolvedSource; err: unknown };
    const inspected: InspectResult[] = await Promise.all(
      resolved.sources.map(async (src) => {
        try {
          const ins = await api.archiveInspect({ source: { path: src.path } });
          return { kind: 'ok', src, ins } as InspectResult;
        } catch (err) {
          return { kind: 'err', src, err } as InspectResult;
        }
      }),
    );

    const okItems: { src: ResolvedSource; ins: InspectResponse }[] = [];
    for (const r of inspected) {
      if (r.kind === 'ok') {
        okItems.push({ src: r.src, ins: r.ins });
      } else {
        pushPopup({
          id: `preflight:${r.src.path}`,
          tone: 'error',
          sticky: true,
          title: `Cannot inspect ${basenameOf(r.src.path)}`,
          message: r.err instanceof ApiError ? r.err.message : 'Inspect request failed.',
        });
      }
    }

    const { groups, errors } = groupArchives(okItems, destMode, destination);

    for (const e of errors) {
      pushPopup({
        id: `preflight:${e.src.path}`,
        tone: 'error',
        sticky: true,
        title: `Missing volumes for ${basenameOf(e.src.path)}`,
        message: `Multi-part archive needs ${e.ins.missingParts.length} more volume(s): ${e.ins.missingParts.join(', ')}`,
      });
    }

    if (groups.length === 0) {
      running = false;
      toastMsg = errors.length > 0 ? 'Nothing extracted — see the popup at the top.' : 'No archives to extract.';
      toastTone = 'error';
      toastVisible = true;
      return;
    }

    // 2. One extract POST per group. runJob's array form gives us the
    //    "N jobs queued" toast for free.
    const outcome = await runJob(() =>
      Promise.all(
        groups.map((g) =>
          api.archiveExtract({
            source: { path: g.primary },
            parts: g.parts.map((p) => ({ path: p })),
            destinationDir: g.destDir,
            password: '',
            overwrite,
            autoAcceptMultiPart: autoMultiPart,
          }),
        ),
      ),
    );

    running = false;
    if (outcome.ok) clearSources();
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }
</script>

<div class="page-header">
  <div class="icon-block">◰</div>
  <div style="flex:1">
    <h1>Extract archive</h1>
    <div class="desc">Unpack any common archive — including multi-part RAR and 7z.</div>
  </div>
</div>

<div class="conv-grid">
  <div class="panel">
    <div class="panel-head">
      <span>Archives</span>
      <span class="right">zip · 7z · rar · tar · gz · bz2 · zst</span>
    </div>
    <div class="panel-body">
      <Dropzone
        multiple={true}
        label="Drop one or more archives to extract"
        hint="Browse files"
        onfiles={addSources}
      />
    </div>
    <div class="files-head">
      <span class="ttl">Files</span>
      <span class="meta">({sources.length})</span>
    </div>
    {#if sources.length === 0}
      <div class="empty-note">No archives yet. Drop one or more into the box above.</div>
    {:else}
      {#each sources as f, i (f.name + i)}
        <div class="file-row">
          <div class="thumb archive">{i + 1}</div>
          <div class="file-name">
            {f.name}
            {#if f.path}<span class="dim">{f.path}</span>{/if}
          </div>
          <span></span>
          <span></span>
          <span></span>
          <button
            class="file-x"
            onclick={() => removeSource(i)}
            title="remove"
            aria-label="Remove {f.name}"
          >×</button>
        </div>
      {/each}
    {/if}
  </div>

  <div class="conv-col">
    <div class="panel">
      <div class="panel-head"><span>Destination</span></div>
      <div class="panel-body">
        <div class="opt-label">Where to extract</div>
        <div class="seg wide">
          <button class={destMode === 'into' ? 'on' : ''} onclick={() => (destMode = 'into')}>
            Single folder
          </button>
          <button
            class={destMode === 'alongside' ? 'on' : ''}
            onclick={() => (destMode = 'alongside')}
          >
            Alongside source
          </button>
        </div>
        <div class="opt-label" style="margin-top:12px">Extract into</div>
        <input
          class="text-input"
          style="width:100%"
          bind:value={destination}
          placeholder={destMode === 'alongside'
            ? 'using each archive’s parent folder'
            : '/path/to/output'}
          disabled={destMode === 'alongside'}
        />
        <div class="sub" style="margin-top:6px">
          {#if destMode === 'alongside'}
            Each archive extracts into a sibling folder named after itself.
          {:else}
            Each archive gets its own subfolder under this path to avoid collisions.
          {/if}
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><span>Options</span></div>
      <div class="panel-body">
        <div class="toggle-row">
          <div class="lbl">
            <span>Overwrite existing</span>
            <span class="sub">Replace files already at the destination</span>
          </div>
          <button
            class="toggle {overwrite ? 'on' : ''}"
            aria-label="Overwrite existing"
            onclick={() => (overwrite = !overwrite)}
          ></button>
        </div>
        <div class="toggle-row">
          <div class="lbl">
            <span>Auto-accept multi-part</span>
            <span class="sub">Extract .partN / .7z.NNN volumes without confirming</span>
          </div>
          <button
            class="toggle {autoMultiPart ? 'on' : ''}"
            aria-label="Auto-accept multi-part"
            onclick={() => (autoMultiPart = !autoMultiPart)}
          ></button>
        </div>
      </div>
    </div>

    <div class="run-summary">{summary}</div>

    <div class="run-btn-block">
      <button class="btn primary" disabled={!ready || running} onclick={run}>
        {running ? 'Running…' : '▸ Extract archive'}
      </button>
    </div>
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
