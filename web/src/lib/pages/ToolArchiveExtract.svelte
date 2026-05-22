<script lang="ts">
  import { Dropzone, OptionRow, Toast, DownloadResult } from '../components';
  import { archiveExtractReady, archiveExtractSummary, type PickedFile } from './toolform';
  import { ApiError, api } from '../api';
  import { isDesktop } from '../native';
  import { runJob, resolveSources } from './run';

  let source = $state('');
  let sourcePicked = $state<PickedFile | null>(null);
  let destination = $state('');
  let overwrite = $state(false);
  let autoMultiPart = $state(false);
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  // Set after a successful browser run so the result can be downloaded.
  let lastUploadId = $state('');
  let lastJobIds = $state<string[]>([]);

  // A plain browser has no destination folder to type — the extracted files
  // land in the upload workspace and come back as a zip. The desktop build
  // keeps the folder field.
  const desktop = isDesktop();

  let summary = $derived(
    desktop
      ? archiveExtractSummary(source, destination)
      : source === ''
        ? 'choose an archive to extract'
        : `ready: extract ${source}`,
  );
  let ready = $derived(
    desktop ? archiveExtractReady(source, destination) : source !== '',
  );

  function pickSource(dropped: File[] | string[]): void {
    const first = dropped[0];
    if (first === undefined) return;
    if (typeof first === 'string') {
      source = first;
      sourcePicked = { name: first, path: first };
    } else {
      source = first.name;
      sourcePicked = { name: first.name, file: first };
    }
  }

  async function run(): Promise<void> {
    if (!ready || !sourcePicked) return;
    let resolved;
    try {
      resolved = await resolveSources([sourcePicked]);
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Upload failed.';
      toastTone = 'error';
      toastVisible = true;
      return;
    }
    const outcome = await runJob(() =>
      api.archiveExtract({
        source: { path: resolved.sources[0].path },
        parts: [],
        destinationDir: desktop ? destination : resolved.outputDir,
        password: '',
        overwrite,
        autoAcceptMultiPart: autoMultiPart,
      }),
    );
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
    if (outcome.ok && resolved.uploadId) {
      lastUploadId = resolved.uploadId;
      lastJobIds = outcome.jobIds;
    }
  }
</script>

<div class="space-y-6">
  <Dropzone
    multiple={false}
    label="Drop an archive to extract"
    hint="or click to browse — zip · 7z · rar · tar · gz · bz2 · zst"
    onfiles={pickSource}
  />

  {#if source !== ''}
    <p class="rounded-md border border-border bg-surface px-3 py-2 text-sm">
      <span class="text-text-dim">Archive:</span>
      <span class="font-mono">{source}</span>
    </p>
  {/if}

  {#if desktop}
    <section>
      <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">Destination</h2>
      <label class="flex items-center gap-3 text-sm">
        <span class="w-28 text-text-dim">Folder</span>
        <input
          bind:value={destination}
          class="flex-1 rounded border border-border bg-surface text-sm px-2 py-1"
          placeholder="/path/to/output"
          aria-label="Destination folder"
        />
      </label>
    </section>
  {/if}

  <section class="space-y-1">
    <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">Options</h2>
    <OptionRow type="checkbox" label="Overwrite existing files" bind:value={overwrite} />
    <OptionRow
      type="checkbox"
      label="Auto-accept multi-part archives"
      hint="extract .partN / .7z.NNN volumes without a confirmation hop"
      bind:value={autoMultiPart}
    />
  </section>

  <div class="flex items-center justify-between border-t border-border pt-4">
    <span class="text-xs text-text-dim">{summary}</span>
    <button
      type="button"
      class="rounded-md bg-accent px-4 py-2 text-sm font-semibold text-bg disabled:opacity-40 disabled:cursor-not-allowed"
      disabled={!ready}
      onclick={run}
    >Run</button>
  </div>

  {#if lastUploadId}
    <div class="flex justify-end">
      <DownloadResult uploadId={lastUploadId} jobIds={lastJobIds} />
    </div>
  {/if}
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
