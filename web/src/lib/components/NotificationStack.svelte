<script lang="ts">
  /**
   * Top-anchored global popup stack. Reads `popups` from
   * stores/notifications; sticky entries persist until the user dismisses
   * them. Each entry is clickable: when a `jobId` is attached, clicking
   * deep-links to the Jobs page with that card expanded and scrolled into
   * view (see Jobs.svelte's focusedJobId handling).
   */
  import { push } from 'svelte-spa-router';
  import { popups, dismissPopup, type Popup } from '../stores/notifications';
  import { expandedJobs, focusedJobId } from '../stores/jobs';

  function openJob(p: Popup): void {
    if (p.jobId) {
      expandedJobs.update((s) => {
        const next = new Set(s);
        next.add(p.jobId!);
        return next;
      });
      focusedJobId.set(p.jobId);
      push('/jobs');
    }
    dismissPopup(p.id);
  }

  function onKeyOpen(ev: KeyboardEvent, p: Popup): void {
    if (ev.key === 'Enter' || ev.key === ' ') {
      ev.preventDefault();
      openJob(p);
    }
  }

  function close(ev: Event, id: string): void {
    ev.stopPropagation();
    dismissPopup(id);
  }
</script>

<div class="popup-stack" aria-live="polite">
  {#each $popups as p (p.id)}
    {#if p.jobId}
      <div
        class="popup {p.tone} clickable"
        role="button"
        tabindex="0"
        onclick={() => openJob(p)}
        onkeydown={(e) => onKeyOpen(e, p)}
      >
        <div class="popup-body">
          <div class="popup-title">{p.title}</div>
          <div class="popup-msg">{p.message}</div>
          <div class="popup-hint">click to view in Jobs</div>
        </div>
        <button
          class="popup-x"
          type="button"
          aria-label="Dismiss notification"
          onclick={(e) => close(e, p.id)}
        >×</button>
      </div>
    {:else}
      <div class="popup {p.tone}" role="status">
        <div class="popup-body">
          <div class="popup-title">{p.title}</div>
          <div class="popup-msg">{p.message}</div>
        </div>
        <button
          class="popup-x"
          type="button"
          aria-label="Dismiss notification"
          onclick={(e) => close(e, p.id)}
        >×</button>
      </div>
    {/if}
  {/each}
</div>
