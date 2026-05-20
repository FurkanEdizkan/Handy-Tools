<script lang="ts">
  /**
   * Clickable tool catalog card — icon glyph + title + description. Mirrors a
   * row of the TUI's Home tool catalog. Renders as an anchor when `href` is
   * given, otherwise a button that fires `onselect`.
   */
  interface Props {
    glyph: string;
    title: string;
    description: string;
    href?: string;
    onselect?: () => void;
    /** Dim + disable interaction (e.g. tool needs a missing system binary). */
    disabled?: boolean;
  }

  let { glyph, title, description, href, onselect, disabled = false }: Props = $props();
</script>

{#if href && !disabled}
  <a class="tool-card" {href} onclick={() => onselect?.()}>
    <span class="glyph" aria-hidden="true">{glyph}</span>
    <span class="body">
      <span class="title">{title}</span>
      <span class="desc">{description}</span>
    </span>
    <span class="chev" aria-hidden="true">›</span>
  </a>
{:else}
  <button
    type="button"
    class="tool-card"
    {disabled}
    onclick={() => onselect?.()}
  >
    <span class="glyph" aria-hidden="true">{glyph}</span>
    <span class="body">
      <span class="title">{title}</span>
      <span class="desc">{description}</span>
    </span>
    <span class="chev" aria-hidden="true">›</span>
  </button>
{/if}

<style>
  .tool-card {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 12px 14px;
    text-align: left;
    text-decoration: none;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    color: var(--color-text);
    cursor: pointer;
    transition: border-color 0.12s, background 0.12s, transform 0.06s;
  }
  .tool-card:hover:not(:disabled) {
    border-color: var(--color-accent);
    background: color-mix(in oklch, var(--color-accent) 6%, var(--color-surface));
  }
  .tool-card:active:not(:disabled) {
    transform: translateY(1px);
  }
  .tool-card:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }
  .tool-card:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .glyph {
    flex-shrink: 0;
    width: 34px;
    height: 34px;
    display: grid;
    place-items: center;
    border-radius: 7px;
    background: color-mix(in oklch, var(--color-accent) 14%, transparent);
    color: var(--color-accent);
    font-size: 16px;
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    flex: 1;
  }
  .title {
    font-size: 13px;
    font-weight: 600;
  }
  .desc {
    font-size: 11px;
    color: var(--color-text-dim);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chev {
    flex-shrink: 0;
    color: var(--color-text-dim);
    font-size: 16px;
    transition: transform 0.12s, color 0.12s;
  }
  .tool-card:hover:not(:disabled) .chev {
    color: var(--color-accent);
    transform: translateX(2px);
  }
</style>
