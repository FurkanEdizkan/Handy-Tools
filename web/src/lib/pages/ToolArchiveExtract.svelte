<script lang="ts">
  import { Dropzone, OptionRow, Toast } from '../components';
  import { archiveExtractReady, archiveExtractSummary } from './toolform';

  let source = $state('');
  let destination = $state('');
  let overwrite = $state(false);
  let autoMultiPart = $state(false);
  let toastVisible = $state(false);

  let summary = $derived(archiveExtractSummary(source, destination));
  let ready = $derived(archiveExtractReady(source, destination));

  function pickSource(dropped: File[] | string[]): void {
    const first = dropped[0];
    if (first === undefined) return;
    source = typeof first === 'string' ? first : first.name;
  }

  function run(): void {
    if (!ready) return;
    // Track C wires this to the real queue.
    toastVisible = true;
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
</div>

<Toast message="queue not wired yet" tone="info" bind:visible={toastVisible} duration={2600} />
