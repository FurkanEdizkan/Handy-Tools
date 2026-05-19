<script lang="ts">
  import { navItems } from './routes';
  import { router } from 'svelte-spa-router';
  import QueuePanel from './QueuePanel.svelte';
</script>

<aside class="left-pane flex flex-col gap-3 p-3 bg-surface border-r border-border min-h-0">
  <section
    class="rounded-lg border border-border bg-bg p-3 flex items-center gap-3"
    aria-label="Mascot"
  >
    <div
      class="w-10 h-10 rounded-md grid place-items-center font-mono text-lg"
      style="background: color-mix(in oklch, var(--color-accent) 14%, transparent); color: var(--color-accent);"
      aria-hidden="true"
    >
      ●
    </div>
    <div class="min-w-0">
      <div class="text-xs font-semibold">Wrenly</div>
      <div class="text-[11px] text-text-dim leading-snug line-clamp-2">
        Ready when you are.
      </div>
    </div>
  </section>

  <nav class="flex flex-col gap-0.5" aria-label="Primary">
    {#each navItems as item}
      {@const active = item.matches(router.location)}
      <a
        href={item.href}
        class="flex items-center gap-2.5 px-2.5 py-2 rounded-md text-sm transition-colors"
        class:active
        aria-current={active ? 'page' : undefined}
      >
        <span class="w-4 text-center text-base" aria-hidden="true">{item.glyph}</span>
        <span class="flex-1">{item.label}</span>
      </a>
    {/each}
  </nav>

  <div class="mt-auto min-h-0 flex flex-col">
    <QueuePanel />
  </div>
</aside>

<style>
  .left-pane {
    width: 100%;
  }
  a {
    color: var(--color-text-dim);
  }
  a:hover {
    background: color-mix(in oklch, var(--color-text) 6%, transparent);
    color: var(--color-text);
  }
  a.active {
    background: color-mix(in oklch, var(--color-accent) 14%, transparent);
    color: var(--color-accent);
    font-weight: 500;
  }
</style>
