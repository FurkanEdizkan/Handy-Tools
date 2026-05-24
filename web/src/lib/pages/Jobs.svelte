<script lang="ts">
  /* Jobs — the full live queue, streamed over SSE from htoolsd. */
  import { jobs, expandedJobs, toggleJobExpanded, type JobStatus } from '../stores/jobs';

  type Filter = 'all' | JobStatus;
  let filter = $state<Filter>('all');

  const counts = $derived.by(() => {
    const c: Record<JobStatus, number> = { wait: 0, running: 0, done: 0, fail: 0 };
    for (const j of $jobs) c[j.status]++;
    return c;
  });
  const filtered = $derived(filter === 'all' ? $jobs : $jobs.filter((j) => j.status === filter));

  const chips: { id: Filter; label: string }[] = [
    { id: 'all', label: 'All' },
    { id: 'running', label: 'Running' },
    { id: 'done', label: 'Done' },
    { id: 'fail', label: 'Failed' },
    { id: 'wait', label: 'Queued' },
  ];

  const ICO: Record<JobStatus, string> = { running: '◐', done: '✓', fail: '✕', wait: '•' };
  const STATE_LABEL: Record<JobStatus, string> = {
    running: 'running',
    done: 'done',
    fail: 'failed',
    wait: 'queued',
  };

  function chipCount(id: Filter): number {
    return id === 'all' ? $jobs.length : counts[id];
  }

  function clearCompleted(): void {
    jobs.update((list) => list.filter((j) => j.status !== 'done'));
  }

  async function copyLogs(text: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      /* clipboard unavailable — best effort */
    }
  }
</script>

<div class="page-header">
  <div class="icon-block">⌗</div>
  <div style="flex:1">
    <h1>Jobs</h1>
    <div class="desc">Live queue across this session — streamed over SSE from htoolsd.</div>
  </div>
  <div class="actions">
    <button class="btn ghost" onclick={clearCompleted}>Clear completed</button>
  </div>
</div>

<div class="jobs-filter">
  {#each chips as c (c.id)}
    <button class="filter-chip {filter === c.id ? 'on' : ''}" onclick={() => (filter = c.id)}>
      {c.label} <span class="ct">{chipCount(c.id)}</span>
    </button>
  {/each}
</div>

<div class="jobs-list">
  {#if filtered.length === 0}
    <div class="empty-note">No jobs match that filter.</div>
  {:else}
    {#each filtered as job (job.id)}
      {@const open = $expandedJobs.has(job.id)}
      <div class="job-card {job.status}">
        <div
          class="job-row {job.status}"
          role="button"
          tabindex="0"
          onclick={() => toggleJobExpanded(job.id)}
          onkeydown={(e) => e.key === 'Enter' && toggleJobExpanded(job.id)}
        >
          <span class="job-ico">{ICO[job.status]}</span>
          <div style="min-width:0">
            <div class="job-label">{job.tool || 'job'}</div>
            <div class="job-meta">{job.currentItem ?? job.id}</div>
          </div>
          {#if job.status === 'running'}
            <div class="job-prog-mini"><i style={`width:${Math.round(job.progress * 100)}%`}></i></div>
          {:else}
            <span></span>
          {/if}
          <span class="job-time">{job.logs[0]?.ts ?? ''}</span>
          <span class="job-state">{STATE_LABEL[job.status]}</span>
          <span class="job-caret">{open ? '▾' : '▸'}</span>
        </div>
        {#if open && job.logs.length > 0}
          <div class="job-logs">
            <div class="job-logs-toolbar">
              <span class="stderr">stderr</span>
              <span>· {job.id}</span>
              <span class="grow"></span>
              <span>{job.logs.length} lines</span>
              <button
                class="btn-mini"
                onclick={() => copyLogs(job.logs.map((l) => `[${l.ts}] ${l.level} ${l.message}`).join('\n'))}
              >⎘ copy</button>
            </div>
            {#each job.logs as l, i (i)}
              <div class="ll lvl-{l.level}">
                <span class="ts">[{l.ts}]</span>
                <span class="lv">{l.level}</span>
                <span class="ms">{l.message}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/each}
  {/if}
</div>
