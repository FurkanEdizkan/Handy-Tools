<script lang="ts">
  /**
   * Inline per-page job status. Subscribes to the shared $jobs store (fed by
   * the all-jobs SSE stream) and renders one compact card per matching id,
   * so each tool page gives the user immediate feedback after Run — without
   * forcing a trip to the Jobs panel. Cards expose "Open folder" (Wails-only)
   * and "View logs" (deep-links to the Jobs page) for completed runs.
   */
  import { push } from 'svelte-spa-router';
  import { jobs, focusedJobId, type Job, type JobStatus } from '../stores/jobs';
  import { reveal, isDesktop } from '../native';

  interface Props {
    jobIds: string[];
  }
  let { jobIds }: Props = $props();

  const myJobs = $derived($jobs.filter((j) => jobIds.includes(j.id)));
  const desktop = isDesktop();

  const ICO: Record<JobStatus, string> = { wait: '•', running: '◐', done: '✓', fail: '✕' };

  // TODO(progress-output-path): every tool currently encodes the result path
  // in the terminal Message (`wrote <path>` for a single file, `into <dir>` /
  // `rendered to <dir>` for a directory). Parsing is brittle vs. message
  // changes — a structured Progress.OutputPath field would be cleaner; see
  // the plan's "What's NOT in scope" section.
  const WROTE_FILE_RE = /(?:wrote|saved to)\s+(.+?)\s*$/;
  const WROTE_DIR_RE = /(?:into|rendered to|split .* into)\s+(.+?)\s*$/;

  function outputFromLogs(job: Job): { kind: 'file' | 'dir' | 'none'; path: string } {
    if (job.status !== 'done') return { kind: 'none', path: '' };
    for (let i = job.logs.length - 1; i >= 0; i--) {
      const m = job.logs[i].message.match(WROTE_FILE_RE);
      if (m) return { kind: 'file', path: m[1] };
      const d = job.logs[i].message.match(WROTE_DIR_RE);
      if (d) return { kind: 'dir', path: d[1] };
    }
    return { kind: 'none', path: '' };
  }

  function openLogs(id: string): void {
    focusedJobId.set(id);
    push('/jobs');
  }
</script>

{#if myJobs.length > 0}
  <div class="run-status">
    {#each myJobs as job (job.id)}
      {@const out = outputFromLogs(job)}
      <div class="rs-card {job.status}">
        <span class="rs-ico">{ICO[job.status]}</span>
        <div class="rs-body">
          <div class="rs-label">{job.tool || 'job'}{job.currentItem ? ` · ${job.currentItem}` : ''}</div>
          {#if job.status === 'running'}
            <div class="rs-bar"><i style={`width:${Math.round(job.progress * 100)}%`}></i></div>
          {/if}
          {#if out.kind !== 'none'}
            <div class="rs-out">{out.kind === 'file' ? '→ wrote' : '→'} <code>{out.path}</code></div>
          {:else if job.status === 'fail' && job.logs.length > 0}
            <div class="rs-out err">{job.logs[job.logs.length - 1].message}</div>
          {/if}
        </div>
        <div class="rs-actions">
          {#if desktop && out.kind !== 'none'}
            <button class="btn ghost rs-btn" onclick={() => reveal(out.path)} title="Open in file manager">📂 Open folder</button>
          {/if}
          <button class="btn ghost rs-btn" onclick={() => openLogs(job.id)} title="View full logs">⌗ Logs</button>
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  .run-status {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 12px;
  }
  .rs-card {
    display: grid;
    grid-template-columns: auto 1fr auto;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--panel);
    font-size: 13px;
  }
  .rs-card.running {
    border-color: var(--accent, #4a9eff);
  }
  .rs-card.done {
    border-color: var(--ok, #4a9e6c);
  }
  .rs-card.fail {
    border-color: var(--err, #c64d4d);
  }
  .rs-ico {
    font-size: 18px;
    line-height: 1;
  }
  .rs-body {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .rs-label {
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rs-bar {
    height: 4px;
    background: var(--border);
    border-radius: 2px;
    overflow: hidden;
  }
  .rs-bar i {
    display: block;
    height: 100%;
    background: var(--accent, #4a9eff);
    transition: width 0.2s;
  }
  .rs-out {
    font-size: 12px;
    color: var(--text-dim);
  }
  .rs-out.err {
    color: var(--err, #c64d4d);
  }
  .rs-out code {
    background: transparent;
    padding: 0;
  }
  .rs-actions {
    display: flex;
    gap: 6px;
  }
  .rs-btn {
    font-size: 11px;
    padding: 4px 8px;
  }
</style>
