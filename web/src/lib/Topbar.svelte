<script lang="ts">
  /* Topbar — breadcrumb + host pill + Jobs/Settings shortcuts. */
  import { router, push } from 'svelte-spa-router';
  import { pageTitle } from './routes';
  import { jobs } from './stores/jobs';
  import HostPill from './HostPill.svelte';

  const atHome = $derived(router.location === '/' || router.location === '');
  const title = $derived(pageTitle(router.location));
  const hasFail = $derived($jobs.some((j) => j.status === 'fail'));
</script>

<div class="topbar">
  <div class="crumbs">
    {#if !atHome}
      <a class="home" href="#/">Home</a>
      <span class="sep">/</span>
    {/if}
    <span class="page-title">{title}</span>
  </div>

  <div class="topbar-right">
    <HostPill />
    <button
      class="icon-btn {hasFail ? 'has-badge' : ''}"
      title="Jobs"
      aria-label="Jobs"
      onclick={() => push('/jobs')}
    >
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
        <rect x="2" y="3" width="12" height="3" rx="1" />
        <rect x="2" y="9" width="12" height="3" rx="1" />
      </svg>
    </button>
    <button class="icon-btn" title="Settings" aria-label="Settings" onclick={() => push('/settings')}>
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
        <circle cx="8" cy="8" r="2.4" />
        <path
          d="M8 1.5v2M8 12.5v2M14.5 8h-2M3.5 8h-2M12.6 3.4l-1.4 1.4M4.8 11.2l-1.4 1.4M12.6 12.6l-1.4-1.4M4.8 4.8L3.4 3.4"
        />
      </svg>
    </button>
  </div>
</div>
