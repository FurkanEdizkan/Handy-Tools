<script lang="ts">
  import type { Snippet } from 'svelte';
  import Header from './Header.svelte';
  import LeftPane from './LeftPane.svelte';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();
</script>

<div class="app-shell">
  <Header />
  <div class="shell-grid">
    <LeftPane />
    <main class="content-pane">
      {@render children?.()}
    </main>
  </div>
</div>

<style>
  .app-shell {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    background: var(--color-bg);
    color: var(--color-text);
  }

  .shell-grid {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 320px 1fr;
  }

  .content-pane {
    min-width: 0;
    overflow-y: auto;
    padding: 24px 28px 32px;
  }

  /* Below 900px: left pane drops to a sticky bottom drawer.
     This is the layout pain the TUI couldn't solve — the raison d'être
     of the web pivot per #56 / #68. */
  @media (max-width: 899px) {
    .shell-grid {
      grid-template-columns: 1fr;
      grid-template-rows: 1fr auto;
    }
    .shell-grid :global(.left-pane) {
      order: 2;
      border-right: none;
      border-top: 1px solid var(--color-border);
      position: sticky;
      bottom: 0;
      max-height: 38vh;
      overflow-y: auto;
    }
    .content-pane {
      order: 1;
    }
  }
</style>
