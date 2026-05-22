<script lang="ts">
  /**
   * Drag-and-drop file input. Clicking opens the native OS picker (#80/#81),
   * which yields real absolute paths as `string[]`; a drag-and-drop yields
   * `File[]`. Both arrive through the same `onfiles` callback. Styled by the
   * design system's `.dropzone` rules in styles/gui.css.
   */
  import { pickFiles } from '../native';

  interface Props {
    multiple?: boolean;
    disabled?: boolean;
    label?: string;
    hint?: string;
    onfiles?: (files: File[] | string[]) => void;
  }

  let {
    multiple = true,
    disabled = false,
    label = 'Drop files or a folder here',
    hint = 'Browse files',
    onfiles,
  }: Props = $props();

  let over = $state(false);

  function emit(list: FileList | null): void {
    if (!list || list.length === 0) return;
    onfiles?.(Array.from(list));
  }

  function onDragOver(ev: DragEvent): void {
    if (disabled) return;
    ev.preventDefault();
    over = true;
  }

  function onDrop(ev: DragEvent): void {
    if (disabled) return;
    ev.preventDefault();
    over = false;
    emit(ev.dataTransfer?.files ?? null);
  }

  async function browse(): Promise<void> {
    if (disabled) return;
    const paths = await pickFiles();
    if (paths.length > 0) onfiles?.(multiple ? paths : paths.slice(0, 1));
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
  class:over
  role="button"
  tabindex={disabled ? -1 : 0}
  aria-disabled={disabled}
  aria-label={label}
  ondragover={onDragOver}
  ondragleave={() => (over = false)}
  ondrop={onDrop}
  onclick={browse}
  onkeydown={onKey}
>
  <div class="ico" aria-hidden="true">↓</div>
  <div class="big">{label}</div>
  <div class="sub"><span class="accent">{hint}</span></div>
</div>
