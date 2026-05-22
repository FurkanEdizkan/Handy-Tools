<script lang="ts" module>
  /* Compact dot-grid mascot for the GUI sidebar — ported from the design
     handoff (project/gui-mascot.jsx). Two characters, one sprite renderer. */
  import type { MascotState } from '../stores/mascot';

  export type MascotCharacter = 'wrenly' | 'hopper';

  const SPRITE_PANDA = `.O...........O.
OOO.........OOO
OwwO.......OwwO
OOOOOOOOOOOOOOO
OOOOOOOOOOOOOOO
OwwOOOOOOOOOwwO
OwwbEbwOwbEbwwO
OwbwwwwOwwwwbwO
OwwwwwwwwwwwwwO
OwwwwwNNNwwwwwO
OwwwwwbMbwwwwwO
.OwwwwwwwwwwwO.
..OOwwwwwwwOO..
.....OOOOO.....`;

  const SPRITE_BUNNY = `.O...........O.
OwO.........OwO
OwO.........OwO
OwO.........OwO
OwO.........OwO
OOO.........OOO
.OOOOOOOOOOOOO.
OOOOOOOOOOOOOOO
OOOEOOOOOOOEOOO
OwwwwwwwwwwwwwO
OwwwwwbNbwwwwwO
OwwwwwwMwwwwwwO
.OwwwwwwwwwwwO.
..OOwwwwwwwOO..
....OOOOOOO....`;

  interface Palette {
    sprite: string;
    fur: string;
    cream: string;
    stripe: string;
    pupil: string;
    nose: string;
  }

  const CHARACTERS: Record<MascotCharacter, Palette> = {
    wrenly: { sprite: SPRITE_PANDA, fur: '#ff8c1a', cream: '#fff4e6', stripe: '#2a0f06', pupil: '#060403', nose: '#060403' },
    hopper: { sprite: SPRITE_BUNNY, fur: '#cdb9ed', cream: '#ffe6ef', stripe: '#6b4a8a', pupil: '#1a0f2e', nose: '#ff8fb8' },
  };

  /* Per-state fur hue override (fur dots only). */
  const STATE_FUR: Partial<Record<MascotState, string>> = {
    happy: 'oklch(0.78 0.17 145)',
    worried: 'oklch(0.62 0.20 25)',
    stressed: 'oklch(0.74 0.22 38)',
    tired: 'oklch(0.55 0.06 65)',
  };

  const RADIUS: Record<string, number> = { O: 0.42, b: 0.38, w: 0.38, E: 0.3, N: 0.26, M: 0.2 };

  interface Dot {
    x: number;
    y: number;
    c: string;
  }

  function parseSprite(s: string): { dots: Dot[]; cols: number; rows: number } {
    const rows = s.split('\n');
    const dots: Dot[] = [];
    rows.forEach((row, y) => {
      for (let x = 0; x < row.length; x++) {
        const c = row[x];
        if (c === '.' || c === ' ') continue;
        dots.push({ x, y, c });
      }
    });
    return { dots, cols: Math.max(...rows.map((r) => r.length)), rows: rows.length };
  }

  function dotColor(c: string, p: Palette): string {
    switch (c) {
      case 'O':
        return p.fur;
      case 'w':
        return p.cream;
      case 'b':
      case 'M':
        return p.stripe;
      case 'E':
        return p.pupil;
      case 'N':
        return p.nose;
      default:
        return p.fur;
    }
  }
</script>

<script lang="ts">
  interface Props {
    character?: MascotCharacter;
    state?: MascotState;
    size?: number;
    showBadge?: boolean;
    /** bare = just the SVG sprite, no container/ring/badge (brand mark). */
    bare?: boolean;
  }
  let { character = 'wrenly', state = 'idle', size = 40, showBadge = true, bare = false }: Props =
    $props();

  const chr = $derived(CHARACTERS[character] ?? CHARACTERS.wrenly);
  const sprite = $derived(parseSprite(chr.sprite));
  const palette = $derived<Palette>({ ...chr, fur: STATE_FUR[state] ?? chr.fur });
  const pad = $derived(bare ? 0.4 : 0.6);

  const badge = $derived(
    (
      {
        thinking: { color: 'var(--accent)', glyph: '…' },
        watching: { color: 'var(--accent)', glyph: '◐' },
        stressed: { color: 'var(--warning)', glyph: '!' },
        tired: { color: 'var(--text-dim)', glyph: 'z' },
        happy: { color: 'var(--success)', glyph: '✓' },
        worried: { color: 'var(--error)', glyph: '!' },
      } as Record<string, { color: string; glyph: string }>
    )[state],
  );

  const ringColor = $derived(
    state === 'happy'
      ? 'var(--success)'
      : state === 'worried'
        ? 'var(--error)'
        : state === 'stressed'
          ? 'var(--warning)'
          : 'var(--border)',
  );
</script>

{#snippet spr()}
  <svg
    viewBox={`${-pad} ${-pad} ${sprite.cols + pad * 2} ${sprite.rows + pad * 2}`}
    width={bare ? size : size - 6}
    height={bare ? size : size - 6}
    style="display:block"
  >
    {#each sprite.dots as d, i (i)}
      {@const cx = d.x + 0.5}
      {@const cy = d.y + 0.5}
      {#if d.c === 'E' && state === 'happy'}
        <path
          d={`M ${cx - 0.45} ${cy + 0.1} Q ${cx} ${cy - 0.4} ${cx + 0.45} ${cy + 0.1}`}
          stroke={palette.pupil}
          stroke-width="0.22"
          stroke-linecap="round"
          fill="none"
        />
      {:else if d.c === 'E' && state === 'worried'}
        <path
          d={`M ${cx - 0.4} ${cy - 0.15} Q ${cx} ${cy + 0.25} ${cx + 0.4} ${cy - 0.15}`}
          stroke={palette.pupil}
          stroke-width="0.2"
          stroke-linecap="round"
          fill="none"
        />
      {:else if d.c === 'E' && state === 'tired'}
        <line
          x1={cx - 0.45}
          y1={cy}
          x2={cx + 0.45}
          y2={cy}
          stroke={palette.pupil}
          stroke-width="0.22"
          stroke-linecap="round"
        />
      {:else}
        <circle {cx} {cy} r={RADIUS[d.c] ?? 0.4} fill={dotColor(d.c, palette)} />
      {/if}
    {/each}
  </svg>
{/snippet}

{#if bare}
  {@render spr()}
{:else}
  <div
    class="m-av"
    style={`width:${size}px;height:${size}px;box-shadow:inset 0 0 0 1px ${ringColor}`}
  >
    {@render spr()}
    {#if showBadge && badge}
      <span class="m-badge" style={`background:${badge.color}`}>{badge.glyph}</span>
    {/if}
  </div>
{/if}

<style>
  .m-av {
    position: relative;
    border-radius: 8px;
    background: var(--bg);
    display: grid;
    place-items: center;
    overflow: hidden;
    transition: box-shadow 0.25s;
  }
  .m-badge {
    position: absolute;
    right: -2px;
    bottom: -2px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    color: #0c0d10;
    display: grid;
    place-items: center;
    font-size: 9px;
    font-weight: 700;
    font-family: var(--mono);
    border: 2px solid var(--surface);
  }
</style>
