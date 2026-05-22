<script lang="ts">
  /**
   * Storybook-style gallery for the #70 component library. Dev-only route
   * (`/dev/components`) — renders every component in isolation so visual
   * regressions and theme behavior are easy to eyeball. Not linked from the
   * primary nav.
   */
  import {
    Dropzone,
    Mascot,
    OptionRow,
    ProgressBar,
    Toast,
    ToolCard,
    MASCOT_STATES,
    type RadioOption,
  } from '../components';

  // Toast demo
  let toastVisible = $state(false);
  let toastTone = $state<'info' | 'success' | 'error'>('success');

  // Dropzone demo
  let dropped = $state<string[]>([]);
  function onfiles(files: File[] | string[]): void {
    dropped = files.map((f) => (typeof f === 'string' ? f : f.name));
  }

  // OptionRow demo
  let stripMeta = $state(true);
  let quality = $state(82);
  let format = $state('jpeg');
  const formatOptions: RadioOption[] = [
    { label: 'PNG', value: 'png' },
    { label: 'JPEG', value: 'jpeg' },
    { label: 'WebP', value: 'webp' },
  ];

  // ProgressBar demo
  let fraction = $state(0.62);
</script>

<section class="gallery">
  <header class="page-head">
    <h1>Component gallery</h1>
    <p>Dev-only showcase of the core component library (issue #70). Cycle the
      theme from the header to check both palettes.</p>
  </header>

  <article class="demo">
    <h2>Mascot</h2>
    <p class="note">All seven <code>mascot.State</code> expressions, animated.</p>
    <div class="mascot-row">
      {#each MASCOT_STATES as pose}
        <Mascot {pose} />
      {/each}
    </div>
    <div class="mascot-row">
      <Mascot character="hopper" pose="happy" />
      <Mascot character="hopper" pose="thinking" />
      <Mascot character="hopper" pose="worried" />
    </div>
  </article>

  <article class="demo">
    <h2>ToolCard</h2>
    <div class="stack narrow">
      <ToolCard glyph="⇄" title="Convert images" description="PNG · JPEG · HEIC · WebP" href="#/tool/convert" />
      <ToolCard glyph="🗜" title="Archive" description="pack or extract zip / tar / 7z" href="#/tool/archive" />
      <ToolCard glyph="✚" title="PDF utilities" description="merge · split · render · extract" disabled />
    </div>
  </article>

  <article class="demo">
    <h2>Dropzone</h2>
    <div class="narrow">
      <Dropzone {onfiles} hint="or click to browse — images only" />
    </div>
    {#if dropped.length > 0}
      <p class="note">Received: <code>{dropped.join(', ')}</code></p>
    {/if}
  </article>

  <article class="demo">
    <h2>OptionRow</h2>
    <div class="stack narrow">
      <OptionRow type="checkbox" label="Strip metadata" hint="remove EXIF / XMP" bind:value={stripMeta} />
      <OptionRow type="slider" label="JPEG quality" min={1} max={100} bind:value={quality} />
      <OptionRow type="radio" label="Output format" options={formatOptions} bind:value={format} />
    </div>
    <p class="note">
      <code>{`{ stripMeta: ${stripMeta}, quality: ${quality}, format: "${format}" }`}</code>
    </p>
  </article>

  <article class="demo">
    <h2>ProgressBar</h2>
    <div class="stack narrow">
      <ProgressBar fraction={fraction} label="Converting photo_4.png" />
      <ProgressBar fraction={fraction} label="blocks variant" variant="blocks" />
      <ProgressBar fraction={1} label="done" tone="success" />
      <ProgressBar fraction={0.4} label="failed" tone="error" />
      <input
        type="range"
        min="0"
        max="1"
        step="0.01"
        value={fraction}
        oninput={(e) => (fraction = Number((e.currentTarget as HTMLInputElement).value))}
        aria-label="Demo progress"
      />
    </div>
  </article>

  <article class="demo">
    <h2>Toast</h2>
    <div class="btn-row">
      {#each ['info', 'success', 'error'] as const as t}
        <button
          type="button"
          class="trigger"
          onclick={() => {
            toastTone = t;
            toastVisible = true;
          }}
        >
          Show {t}
        </button>
      {/each}
    </div>
  </article>
</section>

<Toast
  message={`This is a ${toastTone} toast — auto-dismisses in 3s.`}
  tone={toastTone}
  duration={3000}
  bind:visible={toastVisible}
/>

<style>
  .gallery {
    display: flex;
    flex-direction: column;
    gap: 22px;
  }

  .page-head h1 {
    font-size: 20px;
    font-weight: 600;
    letter-spacing: -0.01em;
  }
  .page-head p {
    font-size: 13px;
    color: var(--color-text-dim);
    max-width: 52ch;
    margin-top: 4px;
  }

  .demo {
    padding: 16px;
    border: 1px solid var(--color-border);
    border-radius: 10px;
    background: var(--color-surface);
  }
  .demo h2 {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 10px;
  }

  .note {
    margin-top: 10px;
    font-size: 11px;
    color: var(--color-text-dim);
  }
  .note code,
  .demo code {
    font-family: ui-monospace, monospace;
    color: var(--color-text);
  }

  .mascot-row {
    display: flex;
    flex-wrap: wrap;
    gap: 18px;
    align-items: flex-end;
  }
  .mascot-row + .mascot-row {
    margin-top: 16px;
  }

  .stack {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .narrow {
    max-width: 420px;
  }

  .btn-row {
    display: flex;
    gap: 8px;
  }
  .trigger {
    padding: 6px 12px;
    border: 1px solid var(--color-border);
    border-radius: 6px;
    background: var(--color-bg);
    color: var(--color-text);
    font-size: 12px;
    cursor: pointer;
    transition: border-color 0.12s;
  }
  .trigger:hover {
    border-color: var(--color-accent);
  }

  input[type='range'] {
    accent-color: var(--color-accent);
  }
</style>
