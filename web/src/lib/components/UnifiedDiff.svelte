<script lang="ts">
  /**
   * Renders a unified diff as a monospace table with line numbers in two
   * left columns and red/green/grey row tinting. The diff comes pre-parsed
   * from the backend (kind=context|add|remove|hunk) so this is a pure
   * presentation component — no string parsing here.
   */
  import type { DiffLine } from '../api';

  interface Props {
    lines: DiffLine[];
    binary?: boolean;
    truncated?: boolean;
    identical?: boolean;
  }

  let { lines, binary = false, truncated = false, identical = false }: Props = $props();
</script>

<div class="udiff">
  {#if binary}
    <div class="udiff-msg">(binary file — not displayed)</div>
  {:else if identical}
    <div class="udiff-msg">(files are identical within the read window)</div>
  {:else}
    <div class="udiff-grid" role="table" aria-label="Unified diff">
      {#each lines as l, i (i)}
        <div class="udiff-row kind-{l.kind}" role="row">
          <span class="ln">{l.kind === 'remove' || l.kind === 'context' ? l.aOld : ''}</span>
          <span class="ln">{l.kind === 'add' || l.kind === 'context' ? l.bNew : ''}</span>
          <span class="marker">
            {#if l.kind === 'add'}+{:else if l.kind === 'remove'}−{:else if l.kind === 'hunk'}@{:else}&nbsp;{/if}
          </span>
          <span class="content">{l.text}</span>
        </div>
      {/each}
    </div>
    {#if truncated}
      <div class="udiff-msg">(diff truncated at 1 MiB)</div>
    {/if}
  {/if}
</div>

<style>
  .udiff {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12px;
    line-height: 1.5;
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--panel, #1a1d24);
    overflow-x: auto;
  }
  .udiff-msg {
    padding: 8px 12px;
    color: var(--text-dim);
    font-style: italic;
  }
  .udiff-grid {
    display: flex;
    flex-direction: column;
  }
  .udiff-row {
    display: grid;
    grid-template-columns: 48px 48px 16px 1fr;
    align-items: baseline;
    gap: 8px;
    padding: 0 8px;
    white-space: pre;
  }
  .udiff-row.kind-add {
    background: rgba(70, 200, 120, 0.12);
  }
  .udiff-row.kind-remove {
    background: rgba(220, 80, 80, 0.12);
  }
  .udiff-row.kind-hunk {
    background: rgba(240, 200, 80, 0.16);
    color: var(--text-dim);
    margin-top: 4px;
  }
  .udiff-row.kind-context {
    color: var(--text-dim);
  }
  .ln {
    color: var(--text-dim);
    text-align: right;
    user-select: none;
    font-size: 11px;
  }
  .marker {
    text-align: center;
    user-select: none;
  }
  .content {
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
