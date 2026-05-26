<script lang="ts">
  import Dropzone from '../components/Dropzone.svelte';
  import Toast from '../components/Toast.svelte';
  import { ApiError, api, type HashAlgo, type HashVerifyEntry } from '../api';
  import { runJob, resolveSources } from './run';
  import type { PickedFile } from './toolform';

  const ALGOS: HashAlgo[] = ['sha256', 'md5', 'blake3'];

  let mode = $state<'hash' | 'verify'>('hash');
  let algo = $state<HashAlgo>('sha256');

  // Hash mode state
  let files = $state<PickedFile[]>([]);
  let hashing = $state(false);

  // Verify mode state
  let manifestPath = $state('');
  let manifestPicked = $state<PickedFile | null>(null);
  let verifying = $state(false);
  let verifyResults = $state<HashVerifyEntry[] | null>(null);
  let verifySummary = $state<{ ok: number; failed: number; missing: number } | null>(null);

  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const hashReady = $derived(mode === 'hash' && files.length > 0);
  const verifyReady = $derived(mode === 'verify' && manifestPath !== '');

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

  function pickManifest(dropped: File[] | string[]): void {
    const first = dropped[0];
    if (first === undefined) return;
    if (typeof first === 'string') {
      manifestPath = first;
      manifestPicked = { name: first.split(/[/\\]/).pop() ?? first, path: first };
    } else {
      manifestPath = first.name;
      manifestPicked = { name: first.name, file: first };
    }
  }

  async function runHash(): Promise<void> {
    if (!hashReady || hashing) return;
    let resolved;
    try {
      resolved = resolveSources(files);
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Could not start the job.';
      toastTone = 'error';
      toastVisible = true;
      return;
    }
    hashing = true;
    const outcome = await runJob(() =>
      api.hash({
        sources: resolved.sources.map((s) => ({ path: s.path })),
        algo,
      }),
    );
    hashing = false;
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }

  async function runVerify(): Promise<void> {
    if (!verifyReady || verifying || !manifestPicked) return;
    verifying = true;
    verifyResults = null;
    verifySummary = null;
    try {
      const res = await api.hashVerify({
        manifest: { path: manifestPicked.path ?? manifestPath },
        algo,
      });
      verifyResults = res.entries;
      verifySummary = { ok: res.ok, failed: res.failed, missing: res.missing };
      toastMsg = `Verified: ${res.ok} ok, ${res.failed} failed, ${res.missing} missing.`;
      toastTone = res.failed === 0 && res.missing === 0 ? 'info' : 'error';
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Verify failed.';
      toastTone = 'error';
    }
    verifying = false;
    toastVisible = true;
  }
</script>

<div class="page-header">
  <div class="icon-block">#</div>
  <div style="flex:1">
    <h1>Hash files</h1>
    <div class="desc">Compute or verify file digests — MD5, SHA-256, BLAKE3.</div>
  </div>
</div>

<div class="conv-grid">
  <div class="panel">
    <div class="panel-head">
      <span>Mode</span>
    </div>
    <div class="panel-body">
      <div class="seg wide">
        <button class={mode === 'hash' ? 'on' : ''} onclick={() => (mode = 'hash')}>Hash files</button>
        <button class={mode === 'verify' ? 'on' : ''} onclick={() => (mode = 'verify')}>Verify manifest</button>
      </div>
    </div>

    {#if mode === 'hash'}
      <div class="panel-head" style="margin-top:12px"><span>Input</span></div>
      <div class="panel-body">
        <Dropzone label="Drop files to hash" hint="Browse files" onfiles={addFiles} />
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
            <div class="thumb">#</div>
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
    {:else}
      <div class="panel-head" style="margin-top:12px"><span>Manifest</span></div>
      <div class="panel-body">
        <Dropzone multiple={false} label="Drop a checksums manifest" hint="Browse files" onfiles={pickManifest} />
        {#if manifestPath !== ''}
          <div class="file-row" style="border-bottom:none;padding:12px 0 0">
            <div class="thumb">✓</div>
            <div class="file-name">
              {manifestPicked?.name ?? manifestPath}
              {#if manifestPicked?.path}<span class="dim">{manifestPicked.path}</span>{/if}
            </div>
            <span></span>
            <span></span>
            <span></span>
            <span></span>
          </div>
        {/if}
      </div>

      {#if verifyResults !== null}
        <div class="files-head">
          <span class="ttl">Results</span>
          {#if verifySummary}
            <span class="meta">({verifySummary.ok} ok · {verifySummary.failed} failed · {verifySummary.missing} missing)</span>
          {/if}
        </div>
        {#each verifyResults as e (e.path)}
          <div class="file-row">
            <div class="thumb">{e.ok ? '✓' : '✗'}</div>
            <div class="file-name">
              {e.path}
              {#if e.err}<span class="dim">{e.err}</span>{:else if !e.ok && e.got}<span class="dim">got {e.got}</span>{/if}
            </div>
            <span></span>
            <span></span>
            <span></span>
            <span></span>
          </div>
        {/each}
      {/if}
    {/if}
  </div>

  <div class="conv-col">
    <div class="panel">
      <div class="panel-head"><span>Algorithm</span></div>
      <div class="panel-body">
        <div class="seg wide">
          {#each ALGOS as a (a)}
            <button class={algo === a ? 'on' : ''} onclick={() => (algo = a)}>{a}</button>
          {/each}
        </div>
      </div>
    </div>

    <div class="run-btn-block">
      {#if mode === 'hash'}
        <button class="btn primary" disabled={!hashReady || hashing} onclick={runHash}>
          {hashing ? 'Running…' : '▸ Hash files'}
        </button>
      {:else}
        <button class="btn primary" disabled={!verifyReady || verifying} onclick={runVerify}>
          {verifying ? 'Verifying…' : '▸ Verify manifest'}
        </button>
      {/if}
    </div>
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
