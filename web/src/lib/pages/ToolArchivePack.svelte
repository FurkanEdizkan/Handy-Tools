<script lang="ts">
  import { Dropzone, OptionRow, Toast } from '../components';
  import { ARCHIVE_FORMATS, archivePackReady, archivePackSummary } from './toolform';
  import { api } from '../api';
  import { isDesktop } from '../native';
  import { runJob, dirOf } from './run';

  let files = $state<string[]>([]);
  let format = $state<(typeof ARCHIVE_FORMATS)[number]>('zip');
  let output = $state('archive.zip');
  let level = $state(6);
  let overwrite = $state(false);
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const desktop = isDesktop();

  let summary = $derived(archivePackSummary(files.length, format, output));
  let ready = $derived(archivePackReady(files.length, output));

  function addFiles(dropped: File[] | string[]): void {
    const names = dropped.map((f) => (typeof f === 'string' ? f : f.name));
    files = [...files, ...names];
  }

  function removeFile(index: number): void {
    files = files.filter((_, i) => i !== index);
  }

  // The archive is written next to the first input; format is inferred from
  // the output filename's extension.
  async function run(): Promise<void> {
    if (!ready || !desktop) return;
    const outcome = await runJob(() =>
      api.archiveCompress({
        sources: files.map((p) => ({ path: p })),
        destination: { file: `${dirOf(files[0])}/${output}`, overwrite },
        format: '',
        compressionLevel: level,
      }),
    );
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }
</script>

<div class="space-y-6">
  <Dropzone label="Drop files to pack" hint="or click to browse" onfiles={addFiles} />

  {#if files.length > 0}
    <section>
      <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">
        Files · {files.length} to pack
      </h2>
      <ul class="space-y-1.5">
        {#each files as name, i (name + i)}
          <li class="flex items-center gap-3 rounded-md border border-border bg-surface px-3 py-2">
            <span class="flex-1 truncate text-sm">{name}</span>
            <button
              type="button"
              class="text-text-dim hover:text-error text-sm"
              aria-label={`Remove ${name}`}
              onclick={() => removeFile(i)}
            >✕</button>
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  <section class="space-y-3">
    <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim">Output</h2>
    <label class="flex items-center gap-3 text-sm">
      <span class="w-28 text-text-dim">Archive format</span>
      <select
        bind:value={format}
        class="rounded border border-border bg-surface text-sm px-2 py-1"
        aria-label="Archive format"
      >
        {#each ARCHIVE_FORMATS as fmt (fmt)}
          <option value={fmt}>{fmt}</option>
        {/each}
      </select>
    </label>
    <label class="flex items-center gap-3 text-sm">
      <span class="w-28 text-text-dim">Output file</span>
      <input
        bind:value={output}
        class="flex-1 rounded border border-border bg-surface text-sm px-2 py-1"
        placeholder="archive.zip"
        aria-label="Output file name"
      />
    </label>
  </section>

  <section class="space-y-1">
    <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">
      Compression options
    </h2>
    <OptionRow type="slider" label="Compression level (0 = store, 9 = max)" bind:value={level} min={0} max={9} />
    <OptionRow type="checkbox" label="Overwrite if the archive exists" bind:value={overwrite} />
  </section>

  <div class="flex items-center justify-between border-t border-border pt-4">
    <span class="text-xs text-text-dim">
      {summary}{#if !desktop} · running needs the desktop app{/if}
    </span>
    <button
      type="button"
      class="rounded-md bg-accent px-4 py-2 text-sm font-semibold text-bg disabled:opacity-40 disabled:cursor-not-allowed"
      disabled={!ready || !desktop}
      onclick={run}
    >Run</button>
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
