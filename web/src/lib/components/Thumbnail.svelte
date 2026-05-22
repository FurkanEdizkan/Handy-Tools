<script lang="ts">
  /**
   * A small preview thumbnail for an input file, sourced from the backend
   * GET /v1/preview endpoint (#59). It only renders an image in the Wails
   * desktop build, where files carry real absolute paths; in a plain browser,
   * or if the preview can't be rendered (non-previewable type, pdftoppm
   * missing), it falls back to a neutral placeholder glyph.
   */
  import { isDesktop } from '../native';

  interface Props {
    /** Absolute file path — only resolvable in the desktop build. */
    path: string;
    size?: number;
  }
  let { path, size = 44 }: Props = $props();

  let failed = $state(false);
  // Request at 2× for crisp thumbnails on hi-dpi displays.
  const src = $derived(
    `/v1/preview?path=${encodeURIComponent(path)}&w=${size * 2}&h=${size * 2}`,
  );
</script>

{#if isDesktop() && !failed}
  <img
    {src}
    alt=""
    width={size}
    height={size}
    class="thumb"
    loading="lazy"
    onerror={() => (failed = true)}
  />
{:else}
  <span
    class="thumb placeholder"
    style={`width:${size}px;height:${size}px`}
    aria-hidden="true">▤</span>
{/if}

<style>
  .thumb {
    border-radius: 4px;
    object-fit: cover;
    flex-shrink: 0;
    background: var(--color-bg);
  }
  .placeholder {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-text-dim);
    border: 1px solid var(--color-border);
    font-size: 18px;
  }
</style>
