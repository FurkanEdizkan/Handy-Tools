<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type SysdepResult } from '../api';
  import { health, formatUptime } from '../stores/health';

  let results = $state<SysdepResult[]>([]);
  let loading = $state(true);
  let error = $state('');
  let expanded = $state<string | null>(null);

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

  const found = $derived(results.filter((r) => r.found).length);
  const missing = $derived(results.length - found);

  function hints(dep: SysdepResult): { os: string; cmd: string }[] {
    return Object.entries(dep.installHint ?? {}).map(([os, cmd]) => ({ os, cmd }));
  }
  function toggle(name: string): void {
    expanded = expanded === name ? null : name;
  }
</script>

<div class="page-header">
  <div class="icon-block">◊</div>
  <div style="flex:1">
    <h1>Doctor</h1>
    <div class="desc">Optional system dependencies that unlock specific tools.</div>
  </div>
  <div class="actions">
    <button class="btn" onclick={load}>⟳ Re-check</button>
  </div>
</div>

{#if loading}
  <div class="empty-note">Checking optional system tools…</div>
{:else if error}
  <div class="panel" style="padding:24px;text-align:center">
    <div style="font-size:13px;color:var(--error)">Could not reach /v1/sysdep — {error}</div>
    <button class="btn" style="margin-top:12px" onclick={load}>Retry</button>
  </div>
{:else}
  <div class="doctor-summary">
    <div class="stat-card">
      <div class="label">Found</div>
      <div class="value good">{found} <span class="unit">/ {results.length}</span></div>
      <div class="hint">on PATH</div>
    </div>
    <div class="stat-card">
      <div class="label">Missing</div>
      <div class="value {missing > 0 ? 'warn' : 'good'}">{missing}</div>
      <div class="hint">{missing > 0 ? 'features disabled' : 'all features available'}</div>
    </div>
    <div class="stat-card">
      <div class="label">Backend</div>
      <div class="value mono {$health.level === 'online' ? 'good' : ''}">{$health.level}</div>
      <div class="hint">
        {$health.version ?? '—'}{$health.commit ? ` · ${$health.commit}` : ''} · up {formatUptime($health.uptimeSeconds)}
      </div>
    </div>
  </div>

  <div class="doctor-table">
    {#each results as dep (dep.name)}
      <div
        class="dr-row {dep.found ? 'ok' : 'miss'}"
        role="button"
        tabindex="0"
        onclick={() => toggle(dep.name)}
        onkeydown={(e) => e.key === 'Enter' && toggle(dep.name)}
      >
        <div class="ico">{dep.found ? '✓' : '⚠'}</div>
        <div class="name">{dep.name}</div>
        <div class="ft">{dep.description ?? ''}</div>
        <div class="ver">{dep.found ? 'on PATH' : '— not installed —'}</div>
        <div class="st">{dep.found ? 'Detected' : 'Missing'}</div>
      </div>
      {#if expanded === dep.name}
        <div class="dr-cmd">
          {#if dep.features && dep.features.length > 0}
            <div class="line"><span class="os">unlocks: {dep.features.join(' · ')}</span></div>
          {/if}
          {#each hints(dep) as h (h.os)}
            <div class="line">
              <span class="prompt">$</span>
              <span class="cmd">{h.cmd}</span>
              <span class="os"># {h.os}</span>
            </div>
          {/each}
          {#if dep.found && hints(dep).length === 0}
            <div class="line"><span class="os">Installed and ready.</span></div>
          {/if}
        </div>
      {/if}
    {/each}
  </div>
{/if}
