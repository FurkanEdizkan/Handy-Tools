<script lang="ts">
  import { onMount } from 'svelte';
  import { currentTheme, cycleTheme, hydrateFromConfig, THEMES } from './lib/stores/theme';

  const version = '0.0.0-dev';

  onMount(() => {
    void hydrateFromConfig();
  });
</script>

<main class="min-h-screen grid place-items-center p-6 bg-bg text-text">
  <section class="text-center space-y-4">
    <header class="space-y-1">
      <h1 class="text-2xl font-semibold tracking-tight">Handy Tools</h1>
      <p class="text-sm text-text-dim">web frontend scaffold · v{version}</p>
    </header>

    <button
      type="button"
      class="px-3 py-1.5 rounded-md border border-border bg-surface text-sm text-text hover:border-accent hover:text-accent transition-colors"
      on:click={cycleTheme}
    >
      theme: <span class="text-accent font-mono">{$currentTheme}</span>
    </button>

    <p class="text-xs text-text-dim font-mono">
      {THEMES.join(' · ')}
    </p>
  </section>
</main>
