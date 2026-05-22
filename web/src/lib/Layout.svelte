<script lang="ts">
  /* App shell — sidebar + main column (topbar · scrolling page · dock).
     The htools-gui design; the OS / browser provides the window chrome. */
  import { onMount, type Snippet } from 'svelte';
  import Sidebar from './Sidebar.svelte';
  import Topbar from './Topbar.svelte';
  import Dock from './Dock.svelte';
  import { startJobsFeed } from './stores/jobs';

  interface Props {
    children?: Snippet;
  }
  let { children }: Props = $props();

  // One live job feed for the whole app — the dock, sidebar badge, Jobs page
  // and mascot all read the shared `jobs` store.
  onMount(() => {
    const ac = startJobsFeed();
    return () => ac.abort();
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
</div>
