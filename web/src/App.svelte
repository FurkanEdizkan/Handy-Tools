<script lang="ts">
  import { onMount } from 'svelte';
  import Router from 'svelte-spa-router';
  import { hydrateFromConfig } from './lib/stores/theme';
  import Layout from './lib/Layout.svelte';
  import { routes } from './lib/routes';

  onMount(() => {
    void hydrateFromConfig();
  });
</script>

<Layout>
  <Router {routes} />
</Layout>
