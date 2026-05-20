<script lang="ts">
  /**
   * Renders the Wrenly / Hopper dot-grid sprite with per-state expressions,
   * mirroring `internal/ui/mascot/mascot.go`. Sprite data and the glyph /
   * overlay tables live in `mascot-art.ts` so they stay testable.
   */
  import {
    eyeGlyph,
    furVar,
    overlayFrame,
    spriteGrid,
    statusLabel,
    type MascotCharacter,
    type MascotState,
  } from './mascot-art';

  interface Props {
    character?: MascotCharacter;
    /** Expression / animation state. Named `pose` to avoid the `$state` rune. */
    pose?: MascotState;
    /** Edge length of one sprite cell, in px. */
    cell?: number;
    /** Animate blink / overlay frames. Disable for static snapshots. */
    animate?: boolean;
  }

  let {
    character = 'wrenly',
    pose = 'idle',
    cell = 13,
    animate = true,
  }: Props = $props();

  let frame = $state(0);

  $effect(() => {
    if (!animate) return;
    const handle = setInterval(() => {
      frame += 1;
    }, 550);
    return () => clearInterval(handle);
  });

  const grid = $derived(spriteGrid(character));
  const overlay = $derived(overlayFrame(pose, frame));
  const eye = $derived(eyeGlyph(pose, frame));

  // Per-state fur tint. Calm Hopper keeps its lilac base; mood states borrow
  // theme hues for both characters (mirrors Go `furColor`).
  const furColor = $derived.by(() => {
    const v = furVar(pose);
    if (v === '--color-mascot-fur' && character === 'hopper') return '#cdb9ed';
    return `var(${v})`;
  });
  const noseColor = $derived(character === 'hopper' ? '#ff8fb8' : 'var(--color-accent)');
</script>

<figure
  class="mascot"
  style:--fur={furColor}
  style:--nose={noseColor}
  style:--cell={`${cell}px`}
  aria-label={`${character} mascot — ${statusLabel(pose).toLowerCase()}`}
>
  <div
    class="overlay"
    data-tone={overlay?.tone ?? 'none'}
    style:justify-content={overlay
      ? overlay.align === 'left'
        ? 'flex-start'
        : overlay.align === 'right'
          ? 'flex-end'
          : 'center'
      : 'center'}
    aria-hidden="true"
  >
    {overlay?.glyphs ?? ''}
  </div>

  <div class="grid" aria-hidden="true">
    {#each grid as row}
      <div class="row">
        {#each row as token}
          {#if token === 'eye'}
            <span class="dot eye">{eye}</span>
          {:else}
            <span class="dot {token}"></span>
          {/if}
        {/each}
      </div>
    {/each}
  </div>

  <figcaption class="badge">
    <span class="who">{character}</span>
    <span class="sep">·</span>
    <span class="state">{statusLabel(pose)}</span>
  </figcaption>
</figure>

<style>
  .mascot {
    margin: 0;
    display: inline-flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .overlay {
    display: flex;
    align-items: flex-end;
    width: calc(var(--cell) * 15);
    height: calc(var(--cell) * 1.4);
    padding: 0 calc(var(--cell) * 0.5);
    font-family: ui-monospace, monospace;
    font-size: calc(var(--cell) * 1.1);
    line-height: 1;
    letter-spacing: 1px;
    white-space: pre;
  }
  .overlay[data-tone='dim'] { color: var(--color-text-dim); }
  .overlay[data-tone='accent'] { color: var(--color-accent); }
  .overlay[data-tone='success'] { color: var(--color-success); }
  .overlay[data-tone='error'] { color: var(--color-error); }

  .grid {
    display: flex;
    flex-direction: column;
  }
  .row {
    display: flex;
  }

  .dot {
    width: var(--cell);
    height: var(--cell);
    box-sizing: border-box;
  }
  /* Filled cells leave a hairline gap so the grid reads as dots, not a slab. */
  .dot.fur,
  .dot.stripe,
  .dot.cream,
  .dot.nose,
  .dot.mouth,
  .dot.eye {
    border-radius: 2px;
    transform: scale(0.86);
  }
  .dot.fur { background: var(--fur); }
  .dot.stripe { background: var(--color-border); }
  .dot.cream { background: var(--color-text); }
  .dot.nose { background: var(--nose); }
  .dot.mouth {
    background: var(--color-text-dim);
    transform: scale(0.6);
  }
  .dot.eye {
    background: var(--fur);
    display: grid;
    place-items: center;
    color: var(--color-bg);
    font-family: ui-monospace, monospace;
    font-size: calc(var(--cell) * 0.82);
    line-height: 1;
  }

  .badge {
    margin-top: 4px;
    font-family: ui-monospace, monospace;
    font-size: 10px;
    letter-spacing: 0.5px;
    color: var(--color-text-dim);
  }
  .badge .who { text-transform: capitalize; }
  .badge .state { color: var(--color-accent); font-weight: 700; }
  .badge .sep { margin: 0 4px; }
</style>
