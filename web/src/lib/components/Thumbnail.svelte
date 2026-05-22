<script lang="ts">
  /**
   * A small preview thumbnail for an input file.
   *
   * - Desktop build: files carry a real absolute `path`, rendered through the
   *   backend GET /v1/preview endpoint (#59).
   * - Plain browser (#191): files carry a `File` blob; an image is previewed
   *   client-side via URL.createObjectURL — no server round-trip, and it works
   *   before the file is ever uploaded.
   *
   * Anything that can't be previewed (non-image type, missing pdftoppm, a
   * browser non-image) falls back to a neutral placeholder glyph.
   */
  import { isDesktop } from '../native';

  interface Props {
    /** Absolute file path — only resolvable in the desktop build. */
    path?: string;
    /** Picked File blob — used for client-side previews in a browser. */
    file?: File;
    size?: number;
  }
  let { path, file, size = 44 }: Props = $props();

  let failed = $state(false);

  // Client-side object URL for a browser-picked image; revoked on change /
  // unmount by the effect cleanup so blobs don't leak.
  let objectUrl = $state<string | null>(null);
  $effect(() => {
    if (file && file.type.startsWith('image/')) {
      const url = URL.createObjectURL(file);
      objectUrl = url;
      return () => {
        URL.revokeObjectURL(url);
        objectUrl = null;
      };
    }
    objectUrl = null;
    return undefined;
  });

  // Request server previews at 2× for crisp thumbnails on hi-dpi displays.
  const serverSrc = $derived(
    path
      ? `/v1/preview?path=${encodeURIComponent(path)}&w=${size * 2}&h=${size * 2}`
      : '',
  );
  const src = $derived(objectUrl ?? (isDesktop() && path ? serverSrc : ''));
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
