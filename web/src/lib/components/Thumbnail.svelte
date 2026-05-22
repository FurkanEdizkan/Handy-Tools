<script lang="ts">
  /**
   * A small preview thumbnail for an input file. Files carry a real absolute
   * `path`, rendered through the backend GET /v1/preview endpoint (#59).
   *
   * Anything that can't be previewed (non-image type, missing pdftoppm) falls
   * back to a neutral placeholder glyph.
   */

  interface Props {
    /** Absolute file path, resolved by the backend preview endpoint. */
    path?: string;
    size?: number;
  }
  let { path, size = 44 }: Props = $props();

  let failed = $state(false);

  // Request server previews at 2× for crisp thumbnails on hi-dpi displays.
  const src = $derived(
    path
      ? `/v1/preview?path=${encodeURIComponent(path)}&w=${size * 2}&h=${size * 2}`
      : '',
  );
</script>

{#if src && !failed}
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
