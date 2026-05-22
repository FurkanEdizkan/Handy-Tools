<script lang="ts">
  import {
    Dropzone,
    OptionRow,
    Toast,
    Thumbnail,
    DownloadResult,
    type RadioOption,
  } from '../components';
  import { IMAGE_FORMATS, imageSummary, imageFormReady, type ImageFile } from './toolform';
  import { ApiError, api, type ImageTargetFormat } from '../api';
  import { runJob, resolveSources } from './run';

  let files = $state<ImageFile[]>([]);
  let quality = $state(90);
  let overwrite = $state(false);
  let stripMetadata = $state(false);
  let recurse = $state(true);
  let outDest = $state('default');
  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  // Set after a successful browser run so the result can be downloaded.
  let lastUploadId = $state('');
  let lastJobIds = $state<string[]>([]);

  const outOptions: RadioOption[] = [
    { label: 'Default folder', value: 'default' },
    { label: 'Alongside source', value: 'alongside' },
    { label: 'Custom…', value: 'custom' },
  ];

  let summary = $derived(imageSummary(files));
  let ready = $derived(imageFormReady(files));

  function addFiles(dropped: File[] | string[]): void {
    const picked: ImageFile[] = dropped.map((d) =>
      typeof d === 'string'
        ? { name: d, path: d, target: 'JPEG' }
        : { name: d.name, file: d, target: 'JPEG' },
    );
    files = [...files, ...picked];
  }

  function removeFile(index: number): void {
    files = files.filter((_, i) => i !== index);
  }

  // One job per file so each row's target format is honored. In a browser the
  // files are uploaded first; either way output lands in resolved.outputDir.
  async function run(): Promise<void> {
    if (!ready) return;
    let resolved;
    try {
      resolved = await resolveSources(files);
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Upload failed.';
      toastTone = 'error';
      toastVisible = true;
      return;
    }
    const outcome = await runJob(() =>
      Promise.all(
        resolved.sources.map((src, i) =>
          api.imageConvert({
            source: { path: src.path },
            targetFormat: files[i].target.toUpperCase() as ImageTargetFormat,
            options: { quality, maxWidth: 0, maxHeight: 0, stripMetadata },
            output: { directory: resolved.outputDir, overwrite },
          }),
        ),
      ),
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
  <Dropzone accept="image/*" label="Drop images here" hint="or click to browse" onfiles={addFiles} />

  {#if files.length > 0}
    <section>
      <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">
        Files · convert each to
      </h2>
      <ul class="space-y-1.5">
        {#each files as f, i (f.name + i)}
          <li class="flex items-center gap-3 rounded-md border border-border bg-surface px-3 py-2">
            <Thumbnail path={f.path} file={f.file} />
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
