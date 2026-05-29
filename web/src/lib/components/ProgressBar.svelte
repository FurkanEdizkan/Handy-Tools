<script lang="ts">
  /**
   * Progress indicator. `variant="bar"` is a styled CSS fill (matches the
   * QueuePanel rows); `variant="blocks"` renders `▰`/`▱` glyphs for parity
   * with the TUI. `tone` recolors the fill for done / failed jobs.
   */
  import { clampFraction, percentLabel, progressBlocks } from './progress';

  interface Props {
    /** Progress as a fraction in [0, 1]. */
    fraction: number;
    label?: string;
    variant?: 'bar' | 'blocks';
    tone?: 'accent' | 'success' | 'error';
    /** Block count when `variant="blocks"`. */
    blocks?: number;
    /** Hide the trailing percent readout. */
    hidePercent?: boolean;
    /** Marks an ETA estimate (opaque op) rather than a measured fraction. */
    estimated?: boolean;
  }

  let {
    fraction,
    label,
    variant = 'bar',
    tone = 'accent',
    blocks = 16,
    hidePercent = false,
    estimated = false,
  }: Props = $props();

  const clamped = $derived(clampFraction(fraction));
  const percent = $derived(percentLabel(fraction));
</script>

<div class="progress" data-tone={tone}>
  {#if label || !hidePercent}
    <div class="caption">
      {#if label}<span class="label">{label}</span>{/if}
      {#if !hidePercent}<span class="percent">{percent}</span>{/if}
    </div>
  {/if}

  {#if variant === 'blocks'}
    <div
      class="blocks"
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(clamped * 100)}
      aria-label={label ?? 'progress'}
    >
      {progressBlocks(fraction, blocks)}
    </div>
  {:else}
    <div
      class="track"
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(clamped * 100)}
      aria-label={label ?? 'progress'}
    >
      <div class="fill" class:estimated style:width={`${clamped * 100}%`}></div>
    </div>
  {/if}
</div>

<style>
  .progress {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 100%;
  }

  .caption {
    display: flex;
    align-items: baseline;
    gap: 8px;
    font-size: 11px;
  }
  .label {
    color: var(--color-text);
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .percent {
    font-family: ui-monospace, monospace;
    color: var(--color-text-dim);
  }

  .track {
    height: 6px;
    border-radius: 999px;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    background: var(--color-accent);
    transition: width 0.25s ease;
  }
  .progress[data-tone='success'] .fill { background: var(--color-success); }
  .progress[data-tone='error'] .fill { background: var(--color-error); }

  /* Estimated (ETA) progress: overlay a moving diagonal stripe so it reads as
     "working, but this percentage is a guess" rather than a measured value. */
  .fill.estimated {
    background-image: linear-gradient(
      45deg,
      rgba(255, 255, 255, 0.18) 25%,
      transparent 25%,
      transparent 50%,
      rgba(255, 255, 255, 0.18) 50%,
      rgba(255, 255, 255, 0.18) 75%,
      transparent 75%,
      transparent
    );
    background-size: 12px 12px;
    animation: progress-stripe 0.8s linear infinite;
  }
  @keyframes progress-stripe {
    from { background-position: 0 0; }
    to { background-position: 12px 0; }
  }
  @media (prefers-reduced-motion: reduce) {
    .fill.estimated { animation: none; }
  }

  .blocks {
    font-family: ui-monospace, monospace;
    font-size: 13px;
    letter-spacing: 1px;
    color: var(--color-accent);
    line-height: 1;
  }
  .progress[data-tone='success'] .blocks { color: var(--color-success); }
  .progress[data-tone='error'] .blocks { color: var(--color-error); }
</style>
