<script lang="ts">
  /**
   * Bottom-center floating notification, mirroring the TUI's toast. `visible`
   * is bindable; when `duration` is set the toast auto-dismisses and flips it
   * back to false. `onclose` fires on auto-dismiss or manual close.
   */
  import { fly } from 'svelte/transition';

  interface Props {
    message: string;
    tone?: 'info' | 'success' | 'error';
    visible?: boolean;
    /** Auto-dismiss delay in ms. 0 (default) keeps the toast until closed. */
    duration?: number;
    onclose?: () => void;
  }

  let {
    message,
    tone = 'info',
    visible = $bindable(false),
    duration = 0,
    onclose,
  }: Props = $props();

  const GLYPH = { info: 'ⓘ', success: '✓', error: '✕' } as const;

  function dismiss(): void {
    visible = false;
    onclose?.();
  }

  $effect(() => {
    if (!visible || duration <= 0) return;
    const handle = setTimeout(dismiss, duration);
    return () => clearTimeout(handle);
  });
</script>

{#if visible}
  <div class="toast-wrap" role="status" aria-live="polite">
    <div class="toast" data-tone={tone} transition:fly={{ y: 16, duration: 160 }}>
      <span class="glyph" aria-hidden="true">{GLYPH[tone]}</span>
      <span class="message">{message}</span>
      <button type="button" class="close" aria-label="Dismiss" onclick={dismiss}>✕</button>
    </div>
  </div>
{/if}

<style>
  .toast-wrap {
    position: fixed;
    bottom: 22px;
    left: 0;
    right: 0;
    display: flex;
    justify-content: center;
    z-index: 50;
    pointer-events: none;
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 10px;
    max-width: min(420px, calc(100vw - 32px));
    padding: 9px 10px 9px 13px;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-left-width: 3px;
    border-radius: 8px;
    box-shadow: 0 16px 40px -18px rgba(0, 0, 0, 0.7);
    font-size: 12px;
    color: var(--color-text);
  }
  .toast[data-tone='info'] { border-left-color: var(--color-accent); }
  .toast[data-tone='success'] { border-left-color: var(--color-success); }
  .toast[data-tone='error'] { border-left-color: var(--color-error); }

  .glyph {
    flex-shrink: 0;
    font-size: 13px;
  }
  .toast[data-tone='info'] .glyph { color: var(--color-accent); }
  .toast[data-tone='success'] .glyph { color: var(--color-success); }
  .toast[data-tone='error'] .glyph { color: var(--color-error); }

  .message {
    flex: 1;
    min-width: 0;
  }

  .close {
    flex-shrink: 0;
    width: 20px;
    height: 20px;
    display: grid;
    place-items: center;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--color-text-dim);
    font-size: 10px;
    cursor: pointer;
    transition: background 0.1s, color 0.1s;
  }
  .close:hover {
    background: var(--color-bg);
    color: var(--color-text);
  }
  .close:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 1px;
  }
</style>
