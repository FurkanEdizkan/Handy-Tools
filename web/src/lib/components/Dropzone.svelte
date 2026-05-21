<script lang="ts">
  /**
   * Drag-and-drop file input. In the Wails desktop build clicking the zone
   * opens the native OS picker (#80/#81), which yields real absolute paths as
   * `string[]`; in a plain browser it falls back to a `<input type=file>`,
   * which yields `File[]`. Both arrive through the same `onfiles` callback.
   */
  import { isDesktop, pickFiles } from '../native';

  interface Props {
    /** `accept` attribute for the file picker (e.g. "image/*,.zip"). */
    accept?: string;
    multiple?: boolean;
    disabled?: boolean;
    label?: string;
    hint?: string;
    onfiles?: (files: File[] | string[]) => void;
  }

  let {
    accept,
    multiple = true,
    disabled = false,
    label = 'Drop files here',
    hint = 'or click to browse',
    onfiles,
  }: Props = $props();

  let dragging = $state(false);
  let input: HTMLInputElement;

  function emit(list: FileList | null): void {
    if (!list || list.length === 0) return;
    onfiles?.(Array.from(list));
  }

  function onDragOver(ev: DragEvent): void {
    if (disabled) return;
    ev.preventDefault();
    dragging = true;
  }

  function onDragLeave(): void {
    dragging = false;
  }

  function onDrop(ev: DragEvent): void {
    if (disabled) return;
    ev.preventDefault();
    dragging = false;
    emit(ev.dataTransfer?.files ?? null);
  }

  async function browse(): Promise<void> {
    if (disabled) return;
    // Desktop: the native picker returns real absolute paths.
    if (isDesktop()) {
      const paths = await pickFiles();
      if (paths.length > 0) onfiles?.(multiple ? paths : paths.slice(0, 1));
      return;
    }
    input.click();
  }

  function onKey(ev: KeyboardEvent): void {
    if (ev.key === 'Enter' || ev.key === ' ') {
      ev.preventDefault();
      browse();
    }
  }
</script>

<div
  class="dropzone"
  class:dragging
  class:disabled
  role="button"
  tabindex={disabled ? -1 : 0}
  aria-disabled={disabled}
  aria-label={label}
  ondragover={onDragOver}
  ondragleave={onDragLeave}
  ondrop={onDrop}
  onclick={browse}
  onkeydown={onKey}
>
  <span class="icon" aria-hidden="true">⇪</span>
  <span class="label">{label}</span>
  <span class="hint">{hint}</span>

  <input
    bind:this={input}
    type="file"
    {accept}
    {multiple}
    {disabled}
    class="sr-only"
    tabindex="-1"
    onchange={(ev) => emit((ev.currentTarget as HTMLInputElement).files)}
  />
</div>

<style>
  .dropzone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    padding: 26px 18px;
    border: 1.5px dashed var(--color-border);
    border-radius: 10px;
    background: var(--color-bg);
    color: var(--color-text-dim);
    cursor: pointer;
    transition: border-color 0.12s, background 0.12s, color 0.12s;
  }
  .dropzone:hover:not(.disabled),
  .dropzone:focus-visible {
    border-color: var(--color-accent);
    color: var(--color-text);
    outline: none;
  }
  .dropzone.dragging {
    border-color: var(--color-accent);
    border-style: solid;
    background: color-mix(in oklch, var(--color-accent) 10%, var(--color-bg));
    color: var(--color-text);
  }
  .dropzone.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .icon {
    font-size: 22px;
    color: var(--color-accent);
  }
  .label {
    font-size: 13px;
    font-weight: 600;
    color: var(--color-text);
  }
  .hint {
    font-size: 11px;
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
