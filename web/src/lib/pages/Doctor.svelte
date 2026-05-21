<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type SysdepResult } from '../api';
  import { sysdepSummary } from './doctor';
  import { health, formatUptime } from '../stores/health';

  let results = $state<SysdepResult[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function load(): Promise<void> {
    loading = true;
    error = '';
    try {
      results = await api.fetchSysdep();
    } catch (e) {
      error = e instanceof Error ? e.message : 'failed to load /v1/sysdep';
    } finally {
      loading = false;
    }
  }

  onMount(load);

  // installHint is a per-OS map (linux / darwin / …); show every entry so the
  // page is useful regardless of which OS the server runs on.
  function hints(dep: SysdepResult): { os: string; cmd: string }[] {
    return Object.entries(dep.installHint ?? {}).map(([os, cmd]) => ({ os, cmd }));
  }
</script>

<section>
  <header class="mb-2 flex items-baseline gap-3">
    <h1 class="text-xl font-semibold tracking-tight">Doctor</h1>
    {#if !loading && !error}
      <span class="text-sm text-text-dim">{sysdepSummary(results)}</span>
    {/if}
  </header>

  <p class="mb-6 text-xs text-text-dim">
    Backend
    <span class="font-mono" class:text-success={$health.level === 'online'} class:text-error={$health.level === 'offline'}
      >{$health.level}</span>
    · version <code class="font-mono text-accent">{$health.version ?? '—'}</code>
    · uptime {formatUptime($health.uptimeSeconds)}
  </p>

  {#if loading}
    <p class="text-sm text-text-dim">Checking optional system tools…</p>
  {:else if error}
    <p class="text-sm text-error">
      Could not reach <code class="font-mono">/v1/sysdep</code>: {error}
    </p>
    <button
      type="button"
      class="mt-3 rounded-md border border-border px-3 py-1.5 text-sm hover:border-accent"
      onclick={load}
    >Retry</button>
  {:else}
    <ul class="space-y-2">
      {#each results as dep (dep.name)}
        <li class="rounded-lg border border-border bg-surface p-4">
          <div class="flex items-center gap-2">
            <span
              class="font-mono"
              class:text-success={dep.found}
              class:text-error={!dep.found}
              aria-hidden="true"
            >{dep.found ? '✓' : '✗'}</span>
            <span class="text-sm font-semibold">{dep.name}</span>
            <span
              class="text-xs"
              class:text-success={dep.found}
              class:text-text-dim={!dep.found}
            >{dep.found ? 'found' : 'missing'}</span>
          </div>

          {#if dep.description}
            <p class="mt-1 text-xs text-text-dim">{dep.description}</p>
          {/if}

          {#if dep.features && dep.features.length > 0}
            <ul class="mt-1 text-xs text-text-dim">
              {#each dep.features as feat (feat)}
                <li>· {feat}</li>
              {/each}
            </ul>
          {/if}

          {#if !dep.found}
            {#each hints(dep) as h (h.os)}
              <p class="mt-1 text-xs">
                <span class="text-text-dim">install ({h.os}):</span>
                <code class="font-mono text-accent">{h.cmd}</code>
              </p>
            {/each}
          {/if}
        </li>
      {/each}
    </ul>

    <button
      type="button"
      class="mt-4 rounded-md border border-border px-3 py-1.5 text-sm hover:border-accent"
      onclick={load}
    >Re-scan</button>
  {/if}
</section>
