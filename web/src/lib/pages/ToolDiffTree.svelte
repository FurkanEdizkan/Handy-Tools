<script lang="ts">
  import Toast from '../components/Toast.svelte';
  import FolderPicker from '../components/FolderPicker.svelte';
  import UnifiedDiff from '../components/UnifiedDiff.svelte';
  import { ApiError, api, type DiffEntry, type DiffLine, type DiffTreeMode } from '../api';

  let pathA = $state('');
  let pathB = $state('');
  let mode = $state<DiffTreeMode>('mtime');
  let comparing = $state(false);
  let results = $state<DiffEntry[] | null>(null);
  let summary = $state<{ added: number; removed: number; changed: number } | null>(null);

  // Per-row diff cache. Key = "path" of the changed entry; value is the
  // loaded structured diff or 'loading' / an error string. Persists across
  // expand/collapse so reopening a row is instant.
  type RowDiff =
    | { state: 'loading' }
    | { state: 'error'; message: string }
    | {
        state: 'ready';
        binary: boolean;
        truncated: boolean;
        identical: boolean;
        lines: DiffLine[];
      };
  let rowDiffs = $state<Record<string, RowDiff>>({});
  let expanded = $state<Set<string>>(new Set());

  let toastVisible = $state(false);
  let toastMsg = $state('');
  let toastTone = $state<'info' | 'error'>('info');

  const ready = $derived(pathA.trim() !== '' && pathB.trim() !== '');

  async function compare(): Promise<void> {
    if (!ready || comparing) return;
    comparing = true;
    results = null;
    summary = null;
    rowDiffs = {};
    expanded = new Set();
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

  function joinPath(root: string, rel: string): string {
    const trimmed = root.replace(/[/\\]+$/, '');
    return `${trimmed}/${rel}`;
  }

  async function loadRowDiff(path: string): Promise<void> {
    rowDiffs = { ...rowDiffs, [path]: { state: 'loading' } };
    try {
      const res = await api.diffTreeFile({
        a: { path: joinPath(pathA, path) },
        b: { path: joinPath(pathB, path) },
      });
      rowDiffs = {
        ...rowDiffs,
        [path]: {
          state: 'ready',
          binary: res.binary,
          truncated: res.truncated,
          identical: res.identical,
          lines: res.lines,
        },
      };
    } catch (e) {
      rowDiffs = {
        ...rowDiffs,
        [path]: { state: 'error', message: e instanceof ApiError ? e.message : 'Diff failed.' },
      };
    }
  }

  function toggleRow(entry: DiffEntry): void {
    if (entry.status !== 'changed') return;
    const next = new Set(expanded);
    if (next.has(entry.path)) {
      next.delete(entry.path);
    } else {
      next.add(entry.path);
      if (!(entry.path in rowDiffs)) {
        void loadRowDiff(entry.path);
      }
    }
    expanded = next;
  }
</script>

<div class="page-header">
  <div class="icon-block">⇆</div>
  <div style="flex:1">
    <h1>Diff two folders</h1>
    <div class="desc">Compare two directory trees. Click a changed row to inspect its content diff.</div>
  </div>
</div>

<div class="conv-grid">
  <div class="panel">
    <div class="panel-head"><span>Folders</span></div>
    <div class="panel-body">
      <div class="opt-label">Folder A (baseline)</div>
      <FolderPicker bind:value={pathA} placeholder="/path/to/old" />
      <div class="opt-label" style="margin-top:12px">Folder B (candidate)</div>
      <FolderPicker bind:value={pathB} placeholder="/path/to/new" />
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
          {@const open = expanded.has(d.path)}
          {@const diff = rowDiffs[d.path]}
          <div class="dt-card">
            {#if d.status === 'changed'}
              <div
                class="file-row clickable"
                role="button"
                tabindex="0"
                onclick={() => toggleRow(d)}
                onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && toggleRow(d)}
              >
                <div class="thumb">~</div>
                <div class="file-name">
                  {d.path}
                  {#if d.reason}<span class="dim">{d.reason}</span>{/if}
                </div>
                <span></span>
                <span></span>
                <span></span>
                <span class="dt-caret">{open ? '▾' : '▸'}</span>
              </div>
            {:else}
              <div class="file-row">
                <div class="thumb">{d.status === 'added' ? '+' : '−'}</div>
                <div class="file-name">
                  {d.path}
                  {#if d.reason}<span class="dim">{d.reason}</span>{/if}
                </div>
                <span></span>
                <span></span>
                <span></span>
                <span></span>
              </div>
            {/if}
            {#if open}
              <div class="dt-diff">
                {#if !diff || diff.state === 'loading'}
                  <div class="dt-loading">loading diff…</div>
                {:else if diff.state === 'error'}
                  <div class="dt-err">{diff.message}</div>
                {:else}
                  <UnifiedDiff
                    lines={diff.lines}
                    binary={diff.binary}
                    truncated={diff.truncated}
                    identical={diff.identical}
                  />
                {/if}
              </div>
            {/if}
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

<style>
  .file-row.clickable {
    cursor: pointer;
  }
  .file-row.clickable:hover {
    background: var(--row-hover, rgba(255, 255, 255, 0.04));
  }
  .dt-caret {
    color: var(--text-dim);
    user-select: none;
  }
  .dt-diff {
    padding: 0 12px 12px 44px;
  }
  .dt-loading,
  .dt-err {
    padding: 8px 12px;
    color: var(--text-dim);
    font-size: 12px;
    font-style: italic;
  }
  .dt-err {
    color: var(--err, #c64d4d);
  }
</style>
