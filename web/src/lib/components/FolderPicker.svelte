<script lang="ts">
  /**
   * A folder path input with a "📁 Browse" button that opens the Wails
   * native picker. In a plain browser the button is hidden so the text
   * input still works as the fallback.
   */
  import { pickFolder, isDesktop } from '../native';

  interface Props {
    value: string;
    placeholder?: string;
    onchange?: (value: string) => void;
  }

  let { value = $bindable(''), placeholder = '/path/to/output', onchange }: Props = $props();
  const desktop = isDesktop();

  async function browse(): Promise<void> {
    const picked = await pickFolder();
    if (picked) {
      value = picked;
      onchange?.(picked);
    }
  }
</script>

<div class="folder-picker">
  <input
    class="text-input fp-input"
    type="text"
    bind:value
    {placeholder}
    oninput={() => onchange?.(value)}
  />
  {#if desktop}
    <button class="btn ghost fp-btn" onclick={browse} title="Pick a folder">📁 Browse</button>
  {/if}
</div>

<style>
  .folder-picker {
    display: flex;
    gap: 6px;
    align-items: stretch;
  }
  .fp-input {
    flex: 1;
    min-width: 0;
  }
  .fp-btn {
    flex-shrink: 0;
    font-size: 12px;
    white-space: nowrap;
  }
</style>
