<script lang="ts">
  import { jobs, expandedJobs, toggleJobExpanded, type JobStatus } from './stores/jobs';

  const STATUS_GLYPH: Record<JobStatus, string> = {
    wait: '•',
    running: '⏳',
    done: '✓',
    fail: '✕',
  };
</script>

<section class="queue-panel flex flex-col min-h-0">
  <header class="flex items-center gap-2 px-3 py-2 border-b border-border">
    <span class="text-[10px] font-semibold uppercase tracking-wider text-text-dim">
      Queue
    </span>
    <span class="ml-auto text-[11px] font-mono text-text-dim">
      {$jobs.length} {$jobs.length === 1 ? 'job' : 'jobs'}
    </span>
  </header>

  <ul class="job-list flex-1 overflow-y-auto" role="list">
    {#each $jobs as job (job.id)}
      {@const expanded = $expandedJobs.has(job.id)}
      <li class="job border-b border-border last:border-b-0" data-status={job.status}>
        <button
          type="button"
          class="row w-full flex items-center gap-2 px-3 py-2 text-left"
          aria-expanded={expanded}
          onclick={() => toggleJobExpanded(job.id)}
        >
          <span class="status-ico font-mono text-sm" aria-hidden="true">
            {STATUS_GLYPH[job.status]}
          </span>
          <span class="min-w-0 flex-1">
            <span class="block text-xs font-medium truncate">{job.tool}</span>
            <span class="block text-[10px] text-text-dim font-mono truncate">
              {job.currentItem ?? 'queued'}
            </span>
          </span>
          <span class="text-[10px] font-mono text-text-dim w-9 text-right">
            {Math.round(job.progress * 100)}%
          </span>
        </button>

        <div class="progress h-[3px] bg-bg" aria-hidden="true">
          <i class="block h-full" style={`width: ${Math.round(job.progress * 100)}%`}></i>
        </div>

        {#if expanded}
          <div class="logs">
            {#if job.logs.length === 0}
              <p class="text-[11px] font-mono text-text-dim px-3 py-2">
                No log lines yet.
              </p>
            {:else}
              <ol role="list">
                {#each job.logs as line, i (job.id + ':' + i)}
                  <li class={`log-line lvl-${line.level}`}>
                    <span class="ts">{line.ts}</span>
                    <span class="lv">{line.level}</span>
                    <span class="ms">{line.message}</span>
                  </li>
                {/each}
              </ol>
            {/if}
          </div>
        {/if}
      </li>
    {/each}
  </ul>
</section>

<style>
  .queue-panel {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    overflow: hidden;
  }

  .job-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .row {
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--color-text);
  }
  .row:hover {
    background: color-mix(in oklch, var(--color-text) 5%, transparent);
  }

  .status-ico { width: 14px; text-align: center; }
  .job[data-status='running'] .status-ico { color: var(--color-accent); }
  .job[data-status='done']    .status-ico { color: var(--color-success); }
  .job[data-status='fail']    .status-ico { color: var(--color-error); }
  .job[data-status='wait']    .status-ico { color: var(--color-text-dim); }

  .progress i {
    background: var(--color-accent);
    transition: width 0.25s ease;
  }
  .job[data-status='done'] .progress i { background: var(--color-success); }
  .job[data-status='fail'] .progress i { background: var(--color-error); }

  .logs {
    background: var(--color-bg);
    max-height: 220px;
    overflow-y: auto;
    border-top: 1px solid var(--color-border);
  }
  .logs ol {
    list-style: none;
    margin: 0;
    padding: 6px 0;
  }
  .log-line {
    display: grid;
    grid-template-columns: 70px 46px 1fr;
    gap: 8px;
    padding: 1px 12px;
    font-family: var(--font-mono, ui-monospace, monospace);
    font-size: 11px;
    line-height: 1.5;
    color: var(--color-text);
  }
  .log-line .ts { color: var(--color-text-dim); }
  .log-line .lv { font-weight: 700; font-size: 10px; padding-top: 1px; }
  .log-line .ms { white-space: pre-wrap; word-break: break-word; }
  .log-line.lvl-DEBUG .lv, .log-line.lvl-DEBUG .ms { color: var(--color-text-dim); }
  .log-line.lvl-WARN  .lv, .log-line.lvl-WARN  .ms { color: var(--color-warning); }
  .log-line.lvl-ERROR .lv, .log-line.lvl-ERROR .ms { color: var(--color-error); }
  .log-line.lvl-FAIL  .lv, .log-line.lvl-FAIL  .ms { color: var(--color-error); font-weight: 600; }
  .log-line.lvl-DONE  .lv, .log-line.lvl-DONE  .ms { color: var(--color-success); }
  .log-line.lvl-HINT  .lv, .log-line.lvl-HINT  .ms { color: var(--color-accent); }
</style>
