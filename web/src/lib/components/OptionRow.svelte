<script module lang="ts">
  /** A single choice in a radio-group OptionRow. */
  export interface RadioOption {
    label: string;
    value: string;
  }
</script>

<script lang="ts">
  /**
   * One labeled control in a tool's option grid. Renders a checkbox, a slider,
   * or a segmented radio group depending on `type`. `value` is bindable so the
   * parent ToolPage form owns the state.
   */
  interface Props {
    type: 'checkbox' | 'slider' | 'radio';
    label: string;
    hint?: string;
    disabled?: boolean;
    value: boolean | number | string;
    /** Radio choices — required when `type` is "radio". */
    options?: RadioOption[];
    /** Slider bounds — used when `type` is "slider". */
    min?: number;
    max?: number;
    step?: number;
  }

  let {
    type,
    label,
    hint,
    disabled = false,
    value = $bindable(),
    options = [],
    min = 0,
    max = 100,
    step = 1,
  }: Props = $props();

  const uid = `opt-${Math.random().toString(36).slice(2, 9)}`;
</script>

<div class="option-row" class:disabled aria-disabled={disabled}>
  <div class="text">
    <span class="label" id={`${uid}-label`}>{label}</span>
    {#if hint}<span class="hint">{hint}</span>{/if}
  </div>

  <div class="control">
    {#if type === 'checkbox'}
      <label class="switch">
        <input
          type="checkbox"
          {disabled}
          checked={value === true}
          onchange={(ev) => (value = (ev.currentTarget as HTMLInputElement).checked)}
        />
        <span class="track" aria-hidden="true"><span class="thumb"></span></span>
      </label>
    {:else if type === 'slider'}
      <div class="slider">
        <input
          type="range"
          {min}
          {max}
          {step}
          {disabled}
          aria-labelledby={`${uid}-label`}
          value={Number(value)}
          oninput={(ev) => (value = Number((ev.currentTarget as HTMLInputElement).value))}
        />
        <output class="readout">{value}</output>
      </div>
    {:else}
      <div class="radio" role="radiogroup" aria-labelledby={`${uid}-label`}>
        {#each options as opt}
          <button
            type="button"
            class="seg"
            class:selected={value === opt.value}
            role="radio"
            aria-checked={value === opt.value}
            {disabled}
            onclick={() => (value = opt.value)}
          >
            {opt.label}
          </button>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .option-row {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 9px 12px;
    border: 1px solid var(--color-border);
    border-radius: 8px;
    background: var(--color-surface);
  }
  .option-row.disabled {
    opacity: 0.55;
  }

  .text {
    display: flex;
    flex-direction: column;
    gap: 1px;
    flex: 1;
    min-width: 0;
  }
  .label {
    font-size: 12px;
    font-weight: 500;
    color: var(--color-text);
  }
  .hint {
    font-size: 10px;
    color: var(--color-text-dim);
  }

  .control {
    flex-shrink: 0;
  }

  /* checkbox — pill switch */
  .switch {
    display: inline-flex;
    cursor: pointer;
  }
  .switch input {
    position: absolute;
    opacity: 0;
    width: 0;
    height: 0;
  }
  .track {
    width: 34px;
    height: 18px;
    border-radius: 999px;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    display: flex;
    align-items: center;
    padding: 2px;
    transition: background 0.12s, border-color 0.12s;
  }
  .thumb {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--color-text-dim);
    transition: transform 0.14s ease, background 0.12s;
  }
  .switch input:checked + .track {
    background: color-mix(in oklch, var(--color-accent) 30%, transparent);
    border-color: var(--color-accent);
  }
  .switch input:checked + .track .thumb {
    transform: translateX(16px);
    background: var(--color-accent);
  }
  .switch input:focus-visible + .track {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* slider */
  .slider {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .slider input[type='range'] {
    width: 120px;
    accent-color: var(--color-accent);
  }
  .readout {
    min-width: 32px;
    text-align: right;
    font-family: ui-monospace, monospace;
    font-size: 11px;
    color: var(--color-text-dim);
  }

  /* radio — segmented buttons */
  .radio {
    display: inline-flex;
    border: 1px solid var(--color-border);
    border-radius: 6px;
    overflow: hidden;
  }
  .seg {
    padding: 5px 11px;
    background: var(--color-bg);
    border: none;
    border-right: 1px solid var(--color-border);
    color: var(--color-text-dim);
    font-size: 11px;
    cursor: pointer;
    transition: background 0.1s, color 0.1s;
  }
  .seg:last-child {
    border-right: none;
  }
  .seg:hover:not(:disabled) {
    color: var(--color-text);
  }
  .seg.selected {
    background: color-mix(in oklch, var(--color-accent) 18%, transparent);
    color: var(--color-accent);
    font-weight: 600;
  }
  .seg:disabled {
    cursor: not-allowed;
  }
  .seg:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }
</style>
