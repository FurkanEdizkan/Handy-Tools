<script lang="ts">
  /* Bottom dock — a collapsed peek at the job queue. Clicking opens Jobs. */
  import { push } from 'svelte-spa-router';
  import { jobs, type Job } from './stores/jobs';
  import { percentLabel } from './components/progress';

  const counts = $derived.by(() => {
    const c = { running: 0, done: 0, fail: 0, wait: 0 };
    for (const j of $jobs) c[j.status]++;
    return c;
  });
  const active = $derived<Job | undefined>($jobs.find((j) => j.status === 'running'));
</script>

<button class="dock" onclick={() => push('/jobs')}>
  <span class="ttl">Queue</span>
  <span class="counts">
    <span><b class="run">{counts.running}</b> running</span>
    <span><b class="done">{counts.done}</b> done</span>
    <span><b class="fail">{counts.fail}</b> failed</span>
    <span>{counts.wait} queued</span>
  </span>
  <span class="grow"></span>
  {#if active}
    <span class="preview">
      <span class="dot"></span>
      {active.currentItem ?? active.tool}
      <span class="mini-bar"><i style={`width:${Math.round(active.progress * 100)}%`}></i></span>
      <span class="pct">{percentLabel(active.progress)}</span>
    </span>
  {:else if counts.fail > 0}
    <span class="preview error">✕ {counts.fail} failed job{counts.fail === 1 ? '' : 's'}</span>
  {:else}
    <span class="preview">All clear · idle</span>
  {/if}
  <span class="open-icon">↗ Jobs</span>
</button>
