<script lang="ts">
  /* App shell — sidebar + main column (topbar · scrolling page · dock).
     The htools-gui design; the OS / browser provides the window chrome. */
  import { onMount, type Snippet } from 'svelte';
  import Sidebar from './Sidebar.svelte';
  import Topbar from './Topbar.svelte';
  import Dock from './Dock.svelte';
  import NotificationStack from './components/NotificationStack.svelte';
  import { startJobsFeed } from './stores/jobs';
  import { subscribeJobsForFailures } from './stores/notifications';

  interface Props {
    children?: Snippet;
  }
  let { children }: Props = $props();

  // One live job feed for the whole app — the dock, sidebar badge, Jobs page
  // and mascot all read the shared `jobs` store.
  onMount(() => {
    const ac = startJobsFeed();
    // Subscribe AFTER the feed so the initial snapshot replay seeds the
    // popup subscriber's "last seen status" map without firing spurious
    // failure popups for jobs that already failed before this session.
    const unsubFailures = subscribeJobsForFailures();
    return () => {
      ac.abort();
      unsubFailures();
    };
  });
</script>

<div class="shell">
  <Sidebar />
  <div class="main-col">
    <Topbar />
    <div class="main-scroll">
      {@render children?.()}
    </div>
    <Dock />
  </div>
  <NotificationStack />
</div>
