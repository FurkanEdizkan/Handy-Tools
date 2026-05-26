<script lang="ts">
  import Toast from '../components/Toast.svelte';
  import { ApiError, api, type DiffEntry, type DiffTreeMode } from '../api';

  let pathA = $state('');
  let pathB = $state('');
  let mode = $state<DiffTreeMode>('mtime');
  let comparing = $state(false);
  let results = $state<DiffEntry[] | null>(null);
  let summary = $state<{ added: number; removed: number; changed: number } | null>(null);

  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const ready = $derived(pathA.trim() !== '' && pathB.trim() !== '');

  async function compare(): Promise<void> {
    if (!ready || comparing) return;
    comparing = true;
    results = null;
    summary = null;
    try {
      const res = await api.diffTreeInspect({
        a: { path: pathA.trim() },
        b: { path: pathB.trim() },
        mode,
      });
      results = res.entries;
      const s = { added: 0, removed: 0, changed: 0 };
      for (const e of res.entries) {
        if (e.status === 'added') s.added++;
        else if (e.status === 'removed') s.removed++;
        else if (e.status === 'changed') s.changed++;
      }
      summary = s;
      toastMsg = `Diff: ${s.added} added, ${s.removed} removed, ${s.changed} changed.`;
      toastTone = 'info';
    } catch (e) {
      toastMsg = e instanceof ApiError ? e.message : 'Diff failed.';
      toastTone = 'error';
    }
    comparing = false;
    toastVisible = true;
  }
</script>

<div class="page-header">
  <div class="icon-block">⇆</div>
  <div style="flex:1">
    <h1>Diff two folders</h1>
    <div class="desc">Compare two directory trees and list what's added, removed, or changed.</div>
  </div>
</div>

<div class="conv-grid">
  <div class="panel">
    <div class="panel-head"><span>Folders</span></div>
    <div class="panel-body">
      <div class="opt-label">Path A (the baseline)</div>
      <input class="text-input" style="width:100%" bind:value={pathA} placeholder="/path/to/old" />
      <div class="opt-label" style="margin-top:12px">Path B (the candidate)</div>
      <input class="text-input" style="width:100%" bind:value={pathB} placeholder="/path/to/new" />
    </div>

    {#if results !== null}
      <div class="files-head">
        <span class="ttl">Results</span>
        {#if summary}
          <span class="meta">({summary.added} added · {summary.removed} removed · {summary.changed} changed)</span>
        {/if}
      </div>
      {#if results.length === 0}
        <div class="empty-note">No differences found.</div>
      {:else}
        {#each results as d (d.status + d.path)}
          <div class="file-row">
            <div class="thumb">{d.status === 'added' ? '+' : d.status === 'removed' ? '−' : '~'}</div>
            <div class="file-name">
              {d.path}
              {#if d.reason}<span class="dim">{d.reason}</span>{/if}
            </div>
            <span></span>
            <span></span>
            <span></span>
            <span></span>
          </div>
        {/each}
      {/if}
    {/if}
  </div>

  <div class="conv-col">
    <div class="panel">
      <div class="panel-head"><span>Compare strategy</span></div>
      <div class="panel-body">
        <div class="seg wide">
          <button class={mode === 'mtime' ? 'on' : ''} onclick={() => (mode = 'mtime')}>mtime</button>
          <button class={mode === 'hash' ? 'on' : ''} onclick={() => (mode = 'hash')}>hash</button>
        </div>
        <div class="dim" style="margin-top:8px;font-size:12px">
          {#if mode === 'mtime'}
            Fast — one stat per file. Misses rewrites with identical size + mtime.
          {:else}
            Slow but authoritative — reads every byte of every pair.
          {/if}
        </div>
      </div>
    </div>

    <div class="run-btn-block">
      <button class="btn primary" disabled={!ready || comparing} onclick={compare}>
        {comparing ? 'Comparing…' : '▸ Compare folders'}
      </button>
    </div>
  </div>
</div>

<Toast message={toastMsg} tone={toastTone} bind:visible={toastVisible} duration={2600} />
