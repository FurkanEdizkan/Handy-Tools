<script lang="ts">
  import { Dropzone, OptionRow, Toast, type RadioOption } from '../components';
  import { PDF_OPS, pdfFormReady, pdfSummary, type PdfOp } from './toolform';

  // `op` is a plain string so it binds cleanly to OptionRow's value prop;
  // it only ever holds a PdfOp (the radio options are the PdfOp values).
  let op = $state('merge');
  let files = $state<string[]>([]);
  let toastVisible = $state(false);

  // Per-operation options.
  let splitEvery = $state(10); // Split: start a new file every N pages.
  let dpi = $state(150); // Render: rasterisation resolution.
  let renderFormat = $state('PNG'); // Render: output image format.
  let keepLayout = $state(true); // Extract text: preserve visual layout.

  const opOptions: RadioOption[] = PDF_OPS.map((o) => ({ label: o.label, value: o.value }));
  const renderFormats: RadioOption[] = [
    { label: 'PNG', value: 'PNG' },
    { label: 'JPEG', value: 'JPEG' },
  ];

  let summary = $derived(pdfSummary(op as PdfOp, files.length));
  let ready = $derived(pdfFormReady(op as PdfOp, files.length));

  function addFiles(dropped: File[] | string[]): void {
    const names = dropped.map((f) => (typeof f === 'string' ? f : f.name));
    files = [...files, ...names];
  }

  function removeFile(index: number): void {
    files = files.filter((_, i) => i !== index);
  }

  function run(): void {
    if (!ready) return;
    // Track C wires this to the real queue.
    toastVisible = true;
  }
</script>

<div class="space-y-6">
  <section>
    <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">Operation</h2>
    <OptionRow type="radio" label="What to do with the PDFs" bind:value={op} options={opOptions} />
  </section>

  <Dropzone
    accept="application/pdf,.pdf"
    label="Drop PDF files here"
    hint="or click to browse — PDFs only"
    onfiles={addFiles}
  />

  {#if files.length > 0}
    <section>
      <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">
        Documents{op === 'merge' ? ' · merged in this order' : ''}
      </h2>
      <ul class="space-y-1.5">
        {#each files as name, i (name + i)}
          <li class="flex items-center gap-3 rounded-md border border-border bg-surface px-3 py-2">
            <span class="text-text-dim text-xs tabular-nums">{i + 1}</span>
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

  <section class="space-y-1">
    <h2 class="text-xs font-semibold uppercase tracking-wide text-text-dim mb-2">
      {PDF_OPS.find((o) => o.value === op)?.label} options
    </h2>
    {#if op === 'merge'}
      <p class="text-xs text-text-dim">Merge concatenates the documents above into one PDF.</p>
    {:else if op === 'split'}
      <OptionRow type="slider" label="Start a new file every N pages" bind:value={splitEvery} min={1} max={100} />
    {:else if op === 'render'}
      <OptionRow type="slider" label="Resolution (DPI)" bind:value={dpi} min={72} max={600} />
      <OptionRow type="radio" label="Image format" bind:value={renderFormat} options={renderFormats} />
    {:else}
      <OptionRow type="checkbox" label="Preserve visual layout" bind:value={keepLayout} />
    {/if}
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
