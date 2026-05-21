<script lang="ts">
  import { Dropzone, OptionRow, Toast, type RadioOption } from '../components';
  import { IMAGE_FORMATS, imageSummary, imageFormReady, type ImageFile } from './toolform';
  import { api, type ImageTargetFormat } from '../api';
  import { isDesktop } from '../native';
  import { runJob, dirOf } from './run';

  let files = $state<ImageFile[]>([]);
  let quality = $state(90);
  let overwrite = $state(false);
  let stripMetadata = $state(false);
  let recurse = $state(true);
  let outDest = $state('default');
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const desktop = isDesktop();
  const outOptions: RadioOption[] = [
    { label: 'Default folder', value: 'default' },
    { label: 'Alongside source', value: 'alongside' },
    { label: 'Custom…', value: 'custom' },
  ];

  let summary = $derived(imageSummary(files));
  let ready = $derived(imageFormReady(files));

  function addFiles(dropped: File[] | string[]): void {
    const names = dropped.map((f) => (typeof f === 'string' ? f : f.name));
    files = [...files, ...names.map((name) => ({ name, target: 'JPEG' as const }))];
  }

  function removeFile(index: number): void {
    files = files.filter((_, i) => i !== index);
  }

  // One job per file so each row's target format is honored. Output lands
  // alongside the source.
  async function run(): Promise<void> {
    if (!ready || !desktop) return;
    const outcome = await runJob(() =>
      Promise.all(
        files.map((f) =>
          api.imageConvert({
            source: { path: f.name },
            targetFormat: f.target.toUpperCase() as ImageTargetFormat,
            options: { quality, maxWidth: 0, maxHeight: 0, stripMetadata },
            output: { directory: dirOf(f.name), overwrite },
          }),
        ),
      ),
    );
    toastMsg = outcome.message;
    toastTone = outcome.ok ? 'info' : 'error';
    toastVisible = true;
  }
</script>

<div class="space-y-6">
  <Dropzone accept="image/*" label="Drop images here" hint="or click to browse" onfiles={addFiles} />

  {#if files.length > 0}
    <section>
      <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">
        Files · convert each to
      </h2>
      <ul class="space-y-1.5">
        {#each files as f, i (f.name + i)}
          <li class="flex items-center gap-3 rounded-md border border-border bg-surface px-3 py-2">
            <span class="flex-1 truncate text-sm">{f.name}</span>
            <select
              bind:value={f.target}
              class="rounded border border-border bg-surface text-xs px-2 py-1"
              aria-label={`Target format for ${f.name}`}
            >
              {#each IMAGE_FORMATS as fmt (fmt)}
                <option value={fmt}>{fmt}</option>
              {/each}
            </select>
            <button
              type="button"
              class="text-text-dim hover:text-error text-sm"
              aria-label={`Remove ${f.name}`}
              onclick={() => removeFile(i)}
            >✕</button>
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  <section>
    <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">
      Output destination
    </h2>
    <OptionRow type="radio" label="Where to write converted images" bind:value={outDest} options={outOptions} />
  </section>

  <section class="space-y-1">
    <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">Options</h2>
    <OptionRow type="slider" label="JPEG / WebP quality" bind:value={quality} min={1} max={100} />
    <OptionRow type="checkbox" label="Overwrite existing files" bind:value={overwrite} />
    <OptionRow type="checkbox" label="Strip metadata (EXIF)" bind:value={stripMetadata} />
    <OptionRow type="checkbox" label="Recurse into dropped folders" bind:value={recurse} />
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
