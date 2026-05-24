<script lang="ts">
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import { archiveExtractReady, archiveExtractSummary, type PickedFile } from './toolform';
  import { ApiError, api } from '../api';
  import { runJob, resolveSources } from './run';

  let source = $state('');
  let sourcePicked = $state<PickedFile | null>(null);
  let destination = $state('');
  let overwrite = $state(false);
  let autoMultiPart = $state(false);
  let running = $state(false);
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const summary = $derived(archiveExtractSummary(source, destination));
  const ready = $derived(archiveExtractReady(source, destination));

  function pickSource(dropped: File[] | string[]): void {
    const first = dropped[0];
    if (first === undefined) return;
    if (typeof first === 'string') {
      source = first;
      sourcePicked = { name: first.split(/[/\\]/).pop() ?? first, path: first };
    } else {
      source = first.name;
      sourcePicked = { name: first.name, file: first };
    }
  }

  async function run(): Promise<void> {
    if (!ready || !sourcePicked || running) return;
    let resolved;
    try {
      resolved = resolveSources([sourcePicked]);
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Could not start the job.';
      toastTone = 'error';
      toastVisible = true;
      return;
    }
    running = true;
    const outcome = await runJob(() =>
      api.archiveExtract({
        source: { path: resolved.sources[0].path },
        parts: [],
        destinationDir: destination,
        password: '',
        overwrite,
        autoAcceptMultiPart: autoMultiPart,
      }),
    );
    running = false;
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
      <span>Archive</span>
      <span class="right">zip · 7z · rar · tar · gz · bz2 · zst</span>
    </div>
    <div class="panel-body">
      <Dropzone multiple={false} label="Drop an archive to extract" hint="Browse files" onfiles={pickSource} />
      {#if source !== ''}
        <div class="file-row" style="border-bottom:none;padding:12px 0 0">
          <div class="thumb archive">ZIP</div>
          <div class="file-name">
            {sourcePicked?.name ?? source}
            {#if sourcePicked?.path}<span class="dim">{sourcePicked.path}</span>{/if}
          </div>
          <span></span>
          <span></span>
          <span></span>
          <span></span>
        </div>
      {/if}
    </div>
  </div>

  <div class="conv-col">
    <div class="panel">
      <div class="panel-head"><span>Destination</span></div>
      <div class="panel-body">
        <div class="opt-label">Extract into</div>
        <input class="text-input" style="width:100%" bind:value={destination} placeholder="/path/to/output" />
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><span>Options</span></div>
      <div class="panel-body">
        <div class="toggle-row">
          <div class="lbl"><span>Overwrite existing</span><span class="sub">Replace files already at the destination</span></div>
          <button class="toggle {overwrite ? 'on' : ''}" aria-label="Overwrite existing" onclick={() => (overwrite = !overwrite)}></button>
        </div>
        <div class="toggle-row">
          <div class="lbl">
            <span>Auto-accept multi-part</span>
            <span class="sub">Extract .partN / .7z.NNN volumes without confirming</span>
          </div>
          <button class="toggle {autoMultiPart ? 'on' : ''}" aria-label="Auto-accept multi-part" onclick={() => (autoMultiPart = !autoMultiPart)}></button>
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
