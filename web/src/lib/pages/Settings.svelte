<script lang="ts">
  import { currentTheme, setTheme, THEMES, type ThemeName } from '../stores/theme';
</script>

<section>
  <header class="mb-6">
    <h1 class="text-xl font-semibold tracking-tight">Settings</h1>
    <p class="text-sm text-text-dim">Appearance and runtime preferences.</p>
  </header>

  <div class="rounded-lg border border-border bg-surface p-4 max-w-xl">
    <div class="text-[10px] font-semibold uppercase tracking-wider text-text-dim mb-2">
      Theme
    </div>
    <div class="flex gap-1">
      {#each THEMES as t}
        <button
          type="button"
          class="px-3 py-1.5 rounded-md text-xs font-mono border transition-colors"
          class:active={$currentTheme === t}
          on:click={() => setTheme(t as ThemeName)}
        >
          {t}
        </button>
      {/each}
    </div>
  </div>
</section>

<style>
  button {
    color: var(--color-text-dim);
    border-color: var(--color-border);
    background: var(--color-bg);
  }
  button:hover {
    color: var(--color-text);
    border-color: var(--color-accent);
  }
  button.active {
    color: var(--color-accent);
    border-color: var(--color-accent);
    background: color-mix(in oklch, var(--color-accent) 12%, transparent);
  }
</style>
